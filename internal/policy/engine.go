package policy

import (
	"fmt"
	"strings"
)

// Rule ID constants for catalog-match policy rules.
const (
	ruleBumblebeeCatalogMatch = "bumblebee-catalog-match"
	ruleOSVCatalogMatch       = "osv-catalog-match"
	ruleSocketCatalogMatch    = "socket-catalog-match"
)

// maxAgentDepth is the maximum allowed agent nesting depth. A subagent at
// depth > maxAgentDepth is immediately blocked with rule INTG-07, before any
// corroboration logic runs. This enforces that arbitrarily deep agent trees
// cannot escape parent policy permissions (INTG-07).
const maxAgentDepth = 10

// editorInstallPatterns are the lowercase CLI substrings that indicate an
// agent-initiated editor-extension install. Trailing space is intentional so
// that the extension ID is always the next token.
var editorInstallPatterns = []string{
	"code --install-extension ",
	"code-insiders --install-extension ",
	"cursor --install-extension ",
	"windsurf --install-extension ",
}

// installPrefixes maps an install-command prefix to its package ecosystem.
// Longest/most-specific prefixes are listed so the first match wins; "cargo
// install" and "cargo add" both resolve to cargo.
var installPrefixes = []struct {
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

// Evaluate is the pure policy entry point. Given a tool call, a MultiCatalogLookup
// (which returns matches from all configured catalog sources), corroboration
// thresholds, and an AgentContext, it returns a Decision without any I/O,
// goroutines, globals mutation, or wall-clock access.
//
// Phase 2 corroboration semantics (PLCY-01):
//   - 1 signed source  → warn  (Allow true)
//   - 2 signed sources → block (Allow false)
//   - 3 signed sources → block + Quarantine true
//   - unsigned sources → warn-only (never block alone; require ≥1 signed)
//
// Phase 4 additions (INTG-07):
//   - Negative ac.Depth is normalized to 0 (root) before any check.
//   - ac.Depth > maxAgentDepth → immediate block with rule INTG-07, before
//     any corroboration logic. This is the first check in Evaluate.
func Evaluate(tc ToolCall, idx MultiCatalogLookup, t CorroborationThresholds, ac AgentContext) Decision {
	// INTG-07: normalize negative depth to 0 (root), then enforce max depth.
	if ac.Depth < 0 {
		ac.Depth = 0
	}
	if ac.Depth > maxAgentDepth {
		return Decision{
			Allow:              false,
			Level:              "block",
			Reason:             fmt.Sprintf("agent depth %d exceeds maximum %d", ac.Depth, maxAgentDepth),
			RuleIDs:            []string{"INTG-07"},
			CorroborationCount: 0,
			SourcesAgreed:      []string{},
			SourcesDissented:   []string{},
		}
	}

	ecosystem, pkg, version, ok := extract(tc.ToolInput)
	if !ok {
		return Decision{
			Allow:  true,
			Level:  "allow",
			Reason: "no package identified",
		}
	}

	// Bulk editor-extension install: if a single command installs 2+ extensions,
	// evaluate each one and return the worst decision (block > warn > allow).
	if ecosystem == "editor-extension" {
		if cmd, cmdOK := tc.ToolInput["command"].(string); cmdOK {
			if strings.Count(strings.ToLower(cmd), "--install-extension ") >= 2 {
				ids := extractAllExtensionInstalls(cmd)
				var worst Decision
				worst = Decision{Allow: true, Level: "allow", Reason: "no catalog match"}
				levelRank := map[string]int{"allow": 0, "warn": 1, "block": 2}
				for _, id := range ids {
					sub := ToolCall{
						AgentName: tc.AgentName,
						ToolName:  tc.ToolName,
						ToolInput: map[string]any{
							"ecosystem": "editor-extension",
							"package":   id,
						},
					}
					d := Evaluate(sub, idx, t, ac)
					if levelRank[d.Level] > levelRank[worst.Level] {
						worst = d
					}
				}
				return worst
			}
		}
	}

	allMatches := idx.LookupAll(ecosystem, pkg)
	// Filter matches by version: if the extracted version is non-empty and the
	// match carries version information, only include matches where the version
	// is listed. Matches with no version constraint apply to all versions.
	// This preserves Phase 1 version-matching semantics in the Phase 2 engine.
	matches := make([]CatalogMatch, 0, len(allMatches))
	for _, m := range allMatches {
		if m.Version == "" || version == "" || m.Version == version {
			matches = append(matches, m)
		}
	}
	if len(matches) == 0 {
		return Decision{
			Allow:  true,
			Level:  "allow",
			Reason: "no catalog match",
		}
	}

	// Apply corroboration logic (pure function — no I/O).
	level, quarantine, count, agreed, dissented := corroborate(matches, t)

	// Mark each match's Corroborated flag based on decision level.
	annotated := make([]CatalogMatch, len(matches))
	for i, m := range matches {
		m.Corroborated = level == "block"
		annotated[i] = m
	}

	// Build RuleIDs from agreed sources.
	ruleIDs := buildRuleIDs(agreed)

	// Construct the reason string.
	var reason string
	switch level {
	case "block":
		reason = "corroborated catalog match: " + strings.Join(agreed, ",")
	case "warn":
		if len(agreed) > 0 {
			reason = "single-source catalog match: " + agreed[0]
		} else {
			reason = "catalog match: warn"
		}
	default:
		reason = "no catalog match"
	}

	return Decision{
		Allow:              level != "block",
		Level:              level,
		Reason:             reason,
		RuleIDs:            ruleIDs,
		CatalogMatches:     annotated,
		CorroborationCount: count,
		SourcesAgreed:      agreed,
		SourcesDissented:   dissented,
		Quarantine:         quarantine,
	}
}

// buildRuleIDs returns the rule IDs corresponding to the agreed source names.
func buildRuleIDs(agreed []string) []string {
	ruleMap := map[string]string{
		"bumblebee": ruleBumblebeeCatalogMatch,
		"osv":       ruleOSVCatalogMatch,
		"socket":    ruleSocketCatalogMatch,
	}
	seen := make(map[string]bool)
	var ids []string
	for _, src := range agreed {
		if rule, ok := ruleMap[src]; ok && !seen[rule] {
			ids = append(ids, rule)
			seen[rule] = true
		}
	}
	// If no known source matched, include bumblebee rule as fallback.
	if len(ids) == 0 && len(agreed) > 0 {
		ids = []string{ruleBumblebeeCatalogMatch}
	}
	return ids
}

// versionMatches reports whether the extracted version is covered by the entry.
// An entry with no Versions applies to all versions (defense-favoring), and an
// extracted version of "" (no explicit @version) also matches defensively.
// NOTE: this helper is retained for compatibility — the multi-source path
// delegates version matching to the catalog adapter.
func versionMatches(versions []string, version string) bool {
	if len(versions) == 0 || version == "" {
		return true
	}
	for _, v := range versions {
		if v == version {
			return true
		}
	}
	return false
}

// extract pulls (ecosystem, package, version) from a tool call's input. It
// supports two shapes:
//
//   - direct: ToolInput has string keys "ecosystem", "package", and (optional)
//     "version" taken verbatim (covers the editor-extension corpus case).
//   - command: ToolInput["command"] is an install command like
//     "npm install <pkg>@<version>"; the prefix selects the ecosystem.
//
// Package names are lowercased and trimmed to match the index key
// normalization. ok is false when no package can be identified.
func extract(input map[string]any) (ecosystem, pkg, version string, ok bool) {
	if input == nil {
		return "", "", "", false
	}

	// Direct shape.
	if eco, pkgRaw, ok2 := directPackage(input); ok2 {
		ver, _ := input["version"].(string)
		return eco, normalize(pkgRaw), strings.TrimSpace(ver), true
	}

	// Command shape: check for editor-extension installs first (higher priority),
	// then fall back to the generic install-prefix table.
	if cmd, ok2 := input["command"].(string); ok2 {
		if eco, p, v, ok3 := extractExtensionInstall(cmd); ok3 {
			return eco, p, v, true
		}
		return extractFromCommand(cmd)
	}

	return "", "", "", false
}

// directPackage returns the ecosystem and raw package name when the input
// carries both as non-empty strings.
func directPackage(input map[string]any) (ecosystem, pkg string, ok bool) {
	eco, ecoOK := input["ecosystem"].(string)
	pkgRaw, pkgOK := input["package"].(string)
	if !ecoOK || !pkgOK {
		return "", "", false
	}
	eco = strings.TrimSpace(eco)
	if eco == "" || strings.TrimSpace(pkgRaw) == "" {
		return "", "", false
	}
	return eco, pkgRaw, true
}

// extractExtensionInstall recognises agent-initiated editor-extension installs
// of the form "<editor> --install-extension <publisher.name[@version]>".
// Pattern matching is case-insensitive; the extension ID is taken from the
// original (non-lowered) cmd to preserve case for display purposes, then
// lowercased by normalize() before return.
// Returns ("editor-extension", pkg, version, true) on success.
func extractExtensionInstall(cmd string) (ecosystem, pkg, version string, ok bool) {
	trimmed := strings.TrimSpace(cmd)
	lower := strings.ToLower(trimmed)

	for _, pattern := range editorInstallPatterns {
		idx := strings.Index(lower, pattern)
		if idx == -1 {
			continue
		}
		// Take the text after the matched pattern from the original cmd.
		after := strings.TrimSpace(trimmed[idx+len(pattern):])
		token := firstPackageToken(after)
		if token == "" {
			return "", "", "", false
		}
		name, ver := splitVersion(token)
		if name == "" {
			return "", "", "", false
		}
		return "editor-extension", normalize(name), strings.TrimSpace(ver), true
	}
	return "", "", "", false
}

// extractAllExtensionInstalls returns every publisher.name (normalized) found
// after each "--install-extension " occurrence in cmd. It is used for bulk
// multi-flag commands such as:
//
//	code --install-extension a.b@1 --install-extension c.d@2
func extractAllExtensionInstalls(cmd string) []string {
	const marker = "--install-extension "
	lower := strings.ToLower(cmd)
	var result []string
	offset := 0
	for {
		idx := strings.Index(lower[offset:], marker)
		if idx == -1 {
			break
		}
		abs := offset + idx + len(marker)
		after := strings.TrimSpace(cmd[abs:])
		token := firstPackageToken(after)
		if token != "" {
			name, _ := splitVersion(token)
			if name != "" {
				result = append(result, normalize(name))
			}
		}
		offset = abs
	}
	return result
}

// extractFromCommand parses an install command into (ecosystem, package,
// version). The package token is the first non-flag argument after the install
// prefix; a trailing "@version" is split off.
func extractFromCommand(cmd string) (ecosystem, pkg, version string, ok bool) {
	trimmed := strings.TrimSpace(cmd)
	lower := strings.ToLower(trimmed)

	for _, p := range installPrefixes {
		if !strings.HasPrefix(lower, p.prefix) {
			continue
		}
		rest := strings.TrimSpace(trimmed[len(p.prefix):])
		token := firstPackageToken(rest)
		if token == "" {
			return "", "", "", false
		}
		name, ver := splitVersion(token)
		if name == "" {
			return "", "", "", false
		}
		return p.ecosystem, normalize(name), strings.TrimSpace(ver), true
	}

	return "", "", "", false
}

// firstPackageToken returns the first whitespace-delimited token in rest that
// is not a flag (does not start with "-"). Returns "" if none.
func firstPackageToken(rest string) string {
	for _, tok := range strings.Fields(rest) {
		if strings.HasPrefix(tok, "-") {
			continue
		}
		return tok
	}
	return ""
}

// splitVersion splits a "name@version" token. Scoped npm packages start with
// "@" (e.g. "@scope/pkg@1.0.0"), so the version separator is the LAST "@".
func splitVersion(token string) (name, version string) {
	at := strings.LastIndex(token, "@")
	if at <= 0 { // no "@", or leading "@" only (scoped name with no version)
		return token, ""
	}
	return token[:at], token[at+1:]
}

// normalize lowercases and trims a package name to match index key
// normalization (catalog index lowercases the package on build).
func normalize(pkg string) string {
	return strings.ToLower(strings.TrimSpace(pkg))
}
