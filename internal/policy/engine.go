package policy

import "strings"

// Rule ID constants for catalog-match policy rules.
const (
	ruleBumblebeeCatalogMatch = "bumblebee-catalog-match"
	ruleOSVCatalogMatch       = "osv-catalog-match"
	ruleSocketCatalogMatch    = "socket-catalog-match"
)

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
// (which returns matches from all configured catalog sources), and corroboration
// thresholds, it returns a Decision without any I/O, goroutines, globals mutation,
// or wall-clock access.
//
// Phase 2 corroboration semantics (PLCY-01):
//   - 1 signed source  → warn  (Allow true)
//   - 2 signed sources → block (Allow false)
//   - 3 signed sources → block + Quarantine true
//   - unsigned sources → warn-only (never block alone; require ≥1 signed)
//
// NOTE: the internal/check call site (plan 08) is the consumer of this new
// signature. Until Plan 08 rewires handler.go, go build ./... will report a
// type mismatch at the call site in internal/check/handler.go. This is an
// expected transient break — go build ./internal/policy/... and
// go test ./internal/policy/... both pass.
func Evaluate(tc ToolCall, idx MultiCatalogLookup, t CorroborationThresholds) Decision {
	ecosystem, pkg, version, ok := extract(tc.ToolInput)
	if !ok {
		return Decision{
			Allow:  true,
			Level:  "allow",
			Reason: "no package identified",
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

	// Command shape.
	if cmd, ok2 := input["command"].(string); ok2 {
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
