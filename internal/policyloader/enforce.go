// Package policyloader — enforce.go
//
// ApplyPolicyOverlay evaluates package_allowlist and sensitive_path rules from
// loaded policy files against a ToolCall and combines the result with the base
// Decision from policy.Evaluate via most-restrictive-wins (with an explicit
// allowlist escape hatch for package_allowlist allow rules).
//
// Architecture constraints:
//   - This function is PURE: no I/O, no goroutines, no globals mutation.
//   - internal/policy is NOT modified; the overlay lives exclusively here.
//   - corroboration_threshold rules are NOT handled here — they already flow
//     through thresholdsFromPolicyFile into policy.Evaluate. Double-applying
//     them would double-count corroboration.
//   - release_age and lifecycle_script_allowlist require package-age and
//     lifecycle data NOT present in a pure ToolCall. They are therefore NOT
//     enforced by this overlay in v1. The engine's built-in release-age /
//     lifecycle policies (catalog/config-driven) remain the enforcement path.
//     See docs/THREAT-MODEL.md §8 for the documented overlay limitation.
//
// T-09-30: Overlay reads only validated declarative fields (no eval, no URL
//          fetch). ValidateSchema already rejects unknown/exec fields at load.
// T-09-31: A package_allowlist allow rule CAN override a catalog-corroborated
//          block for that exact package. This is intentional and documented in
//          docs/THREAT-MODEL.md §1 (Policy file injection surface). The
//          override is recorded in the decision reason so it is forensically
//          visible in the audit log.
// T-09-33: A malformed individual policy file must not crash beekeeper check.
//          LoadPolicyDir skips invalid files with a logged warning.
package policyloader

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mzansi-agentive/beekeeper/internal/policy"
)

// ApplyPolicyOverlay evaluates package_allowlist and sensitive_path rules from
// files against tc, then combines the result with base by most-restrictive-wins,
// with an explicit allowlist escape hatch:
//
//  1. If any matched rule action is "block" → block (reason cites rule ID + file name).
//  2. Else if an "allow" package_allowlist rule matches the EXACT package AND
//     base is warn or block → downgrade to allow (version-controlled user-trust
//     override). Reason cites the rule. See T-09-31.
//  3. Else final = most-restrictive of {base, any matched "warn"}.
//  4. Else → base unchanged.
//
// corroboration_threshold, release_age, and lifecycle_script_allowlist rules
// are silently skipped — see file comment for rationale.
func ApplyPolicyOverlay(files []PolicyFile, tc policy.ToolCall, base policy.Decision) policy.Decision {
	// Extract the package and ecosystem from the tool call for allowlist matching.
	// We reuse the same extract logic as the engine: direct-shape or command-shape.
	eco, pkg := extractEcoPackage(tc)
	targetPath := extractTargetPath(tc)

	// Track the most-restrictive overlay result.
	blockRuleID := ""
	blockFile := ""
	warnRuleID := ""
	warnFile := ""
	allowRuleID := ""
	allowFile := ""

	for _, pf := range files {
		for _, r := range pf.Rules {
			switch r.RuleType {
			case "package_allowlist":
				if eco == "" || pkg == "" {
					// Cannot evaluate package rules without package context.
					continue
				}
				if !matchesEcosystem(r, eco) {
					continue
				}
				if !matchesPackage(r.Packages, pkg) {
					continue
				}
				// This rule matches.
				switch r.Action {
				case "block":
					if blockRuleID == "" {
						blockRuleID = r.ID
						blockFile = pf.Name
					}
				case "warn":
					if warnRuleID == "" {
						warnRuleID = r.ID
						warnFile = pf.Name
					}
				case "allow", "":
					// "allow" is the escape hatch (T-09-31). Empty action on a
					// package_allowlist is treated as allow.
					if allowRuleID == "" {
						allowRuleID = r.ID
						allowFile = pf.Name
					}
				}

			case "sensitive_path":
				if targetPath == "" {
					// No path in this tool call — sensitive_path rules do not apply.
					continue
				}
				if !matchesSensitivePath(r.PathPatterns, targetPath) {
					continue
				}
				// Sensitive path matched.
				switch r.Action {
				case "block", "":
					// Default action for sensitive_path is block (if empty).
					if blockRuleID == "" {
						blockRuleID = r.ID
						blockFile = pf.Name
					}
				case "warn":
					if warnRuleID == "" {
						warnRuleID = r.ID
						warnFile = pf.Name
					}
				}

			// corroboration_threshold: already handled via thresholdsFromPolicyFile
			// in the engine; skipped here to prevent double-application.
			case "corroboration_threshold":
				continue

			// release_age, lifecycle_script_allowlist: require package-age / lifecycle
			// data not present in a pure ToolCall. NOT enforced by this overlay in v1.
			// The engine's built-in release-age/lifecycle catalog-driven enforcement
			// remains the only enforcement path. See docs/THREAT-MODEL.md §8.
			case "release_age", "lifecycle_script_allowlist":
				continue
			}
		}
	}

	// Step 1: Any block match → block regardless of base.
	if blockRuleID != "" {
		fileHint := ""
		if blockFile != "" {
			fileHint = fmt.Sprintf(" (policy: %s)", blockFile)
		}
		return policy.Decision{
			Allow:   false,
			Level:   "block",
			Reason:  fmt.Sprintf("policy overlay: rule %q%s blocks this action", blockRuleID, fileHint),
			RuleIDs: append([]string{blockRuleID}, base.RuleIDs...),
		}
	}

	// Step 2: Explicit "allow" package_allowlist escape hatch overrides a warn/block base.
	if allowRuleID != "" && (base.Level == "warn" || base.Level == "block") {
		fileHint := ""
		if allowFile != "" {
			fileHint = fmt.Sprintf(" (policy: %s)", allowFile)
		}
		return policy.Decision{
			Allow:  true,
			Level:  "allow",
			Reason: fmt.Sprintf("policy overlay: rule %q%s allowlists this package (user-trust override — recorded for audit)", allowRuleID, fileHint),
			RuleIDs: []string{allowRuleID},
		}
	}

	// Step 3: Most-restrictive of base and any matched warn.
	if warnRuleID != "" && base.Level == "allow" {
		fileHint := ""
		if warnFile != "" {
			fileHint = fmt.Sprintf(" (policy: %s)", warnFile)
		}
		return policy.Decision{
			Allow:   true,
			Level:   "warn",
			Reason:  fmt.Sprintf("policy overlay: rule %q%s warns on this action", warnRuleID, fileHint),
			RuleIDs: append([]string{warnRuleID}, base.RuleIDs...),
		}
	}

	// Step 4: No overlay change — return base unchanged.
	return base
}

// LoadPolicyDir loads all valid *.json files from dir as PolicyFiles, skipping
// individually invalid files with a logged warning (T-09-33). A missing
// directory is treated as empty (not an error) — mirrors ListPolicyFiles /
// Pitfall 3 (beekeeper init may not have created ~/.beekeeper/policies/ yet).
//
// Returns an error only when the directory exists but cannot be read at all
// (e.g., permission denied). Callers must honor fail_mode on a non-nil error.
func LoadPolicyDir(dir string) ([]PolicyFile, error) {
	summaries, err := ListPolicyFiles(dir)
	if err != nil {
		return nil, fmt.Errorf("load policy dir %q: %w", dir, err)
	}

	files := make([]PolicyFile, 0, len(summaries))
	for _, s := range summaries {
		pf, errs := LoadPolicyFile(s.Path)
		if len(errs) > 0 {
			// Skip individually invalid policy files with a warning (T-09-33).
			// This prevents a single malformed file from crashing beekeeper check.
			fmt.Printf("beekeeper: WARNING: skipping invalid policy file %q: %v\n", s.Path, errs[0])
			continue
		}
		files = append(files, pf)
	}

	return files, nil
}

// extractEcoPackage extracts (ecosystem, package) from a ToolCall using the same
// two-shape logic as the policy engine (direct-shape and command-shape). Returns
// empty strings when no package is identifiable.
func extractEcoPackage(tc policy.ToolCall) (eco, pkg string) {
	if tc.ToolInput == nil {
		return "", ""
	}

	// Direct shape: explicit "ecosystem" and "package" keys.
	if e, ok := tc.ToolInput["ecosystem"].(string); ok && e != "" {
		if p, ok := tc.ToolInput["package"].(string); ok && p != "" {
			return strings.ToLower(strings.TrimSpace(e)),
				strings.ToLower(strings.TrimSpace(p))
		}
	}

	// Command shape: extract from install command prefix.
	if cmd, ok := tc.ToolInput["command"].(string); ok && cmd != "" {
		eco, pkg = extractEcoPackageFromCommand(cmd)
	}

	return eco, pkg
}

// installPrefixesOverlay is the same prefix table as in engine.go — duplicated
// here to keep internal/policy untouched (the overlay must not import engine
// internals, and internal/policy is not allowed to export this table).
var installPrefixesOverlay = []struct {
	prefix    string
	ecosystem string
}{
	{"npm install", "npm"},
	{"npm i ", "npm"},
	{"pip install", "pypi"},
	{"pip3 install", "pypi"},
	{"go get", "go"},
	{"gem install", "rubygems"},
	{"cargo add", "cargo"},
	{"cargo install", "cargo"},
	{"composer require", "packagist"},
}

// extractEcoPackageFromCommand parses an install command into (ecosystem, pkg).
func extractEcoPackageFromCommand(cmd string) (eco, pkg string) {
	lower := strings.ToLower(strings.TrimSpace(cmd))
	for _, p := range installPrefixesOverlay {
		if !strings.HasPrefix(lower, p.prefix) {
			continue
		}
		rest := strings.TrimSpace(cmd[len(p.prefix):])
		token := firstNonFlagToken(rest)
		if token == "" {
			return "", ""
		}
		// Strip version suffix (last "@").
		name := stripVersionSuffix(token)
		if name == "" {
			return "", ""
		}
		return p.ecosystem, strings.ToLower(strings.TrimSpace(name))
	}
	return "", ""
}

// extractTargetPath extracts a filesystem path from a ToolCall. Looks for a
// "path" key (WriteFile, ReadFile, etc.) or falls back to an empty string.
func extractTargetPath(tc policy.ToolCall) string {
	if tc.ToolInput == nil {
		return ""
	}
	if p, ok := tc.ToolInput["path"].(string); ok {
		return p
	}
	return ""
}

// matchesEcosystem returns true when the rule applies to the given ecosystem.
// A rule with both Ecosystem and Ecosystems empty applies to all ecosystems.
func matchesEcosystem(r PolicyRule, eco string) bool {
	// Single-ecosystem field.
	if r.Ecosystem != "" {
		return strings.EqualFold(r.Ecosystem, eco)
	}
	// Multi-ecosystem field.
	for _, e := range r.Ecosystems {
		if strings.EqualFold(e, eco) {
			return true
		}
	}
	// Neither set — applies to all ecosystems.
	return len(r.Ecosystems) == 0 && r.Ecosystem == ""
}

// matchesPackage returns true when pkg is in the packages list (case-insensitive).
func matchesPackage(packages []string, pkg string) bool {
	for _, p := range packages {
		if strings.EqualFold(p, pkg) {
			return true
		}
	}
	return false
}

// matchesSensitivePath returns true when targetPath matches any of the given
// patterns. Patterns use the same semantics as internal/policy/path.go:
//   - Patterns containing "/" or "\" → fragment / strings.Contains match.
//   - Patterns ending in ".*" → basename prefix match.
//   - All others → exact basename match.
func matchesSensitivePath(patterns []string, targetPath string) bool {
	for _, pattern := range patterns {
		if matchesSensitivePathPattern(targetPath, pattern) {
			return true
		}
	}
	return false
}

// matchesSensitivePathPattern mirrors the matching logic in
// internal/policy/path.go matchesBlockPattern to keep semantics consistent.
func matchesSensitivePathPattern(resolvedPath, pattern string) bool {
	// filepath.Match for glob-style patterns (fallback to Contains for path fragments).
	if strings.Contains(pattern, "/") || strings.Contains(pattern, "\\") {
		// Fragment pattern.
		return strings.Contains(resolvedPath, pattern) ||
			strings.Contains(normalizePathSlashes(resolvedPath), pattern)
	}

	// Basename pattern — extract last segment.
	seg := lastPathSegment(resolvedPath)

	// Glob: ".env.*" matches any segment with prefix ".env."
	if strings.HasSuffix(pattern, ".*") {
		prefix := pattern[:len(pattern)-1] // strip trailing "*", keep "."
		return strings.HasPrefix(seg, prefix)
	}

	// filepath.Match for patterns without path separators (handles * glob).
	if strings.ContainsAny(pattern, "*?[") {
		matched, err := filepath.Match(pattern, seg)
		return err == nil && matched
	}

	// Exact basename match.
	return seg == pattern
}

// lastPathSegment returns the last component of a path, splitting on "/" and "\".
func lastPathSegment(p string) string {
	i := strings.LastIndexAny(p, "/\\")
	if i < 0 {
		return p
	}
	return p[i+1:]
}

// normalizePathSlashes converts backslashes to forward slashes for cross-platform
// pattern comparison (mirrors internal/policy/path.go normalizeSlashes).
func normalizePathSlashes(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}

// firstNonFlagToken returns the first whitespace-delimited token in s that
// does not start with "-". Returns "" if none.
func firstNonFlagToken(s string) string {
	for _, tok := range strings.Fields(s) {
		if !strings.HasPrefix(tok, "-") {
			return tok
		}
	}
	return ""
}

// stripVersionSuffix removes a trailing "@version" from a package token.
// Scoped npm packages start with "@" so we look for the LAST "@".
func stripVersionSuffix(token string) string {
	at := strings.LastIndex(token, "@")
	if at <= 0 {
		return token
	}
	return token[:at]
}
