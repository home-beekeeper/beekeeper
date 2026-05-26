package policy

import (
	"strings"

	"github.com/mzansi-agentive/beekeeper/internal/catalog"
)

// ruleBumblebeeCatalogMatch is the rule ID surfaced when a tool call matches a
// Bumblebee threat-intel catalog entry.
const ruleBumblebeeCatalogMatch = "bumblebee-catalog-match"

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

// Evaluate is the pure policy entry point. Given a tool call and a catalog
// lookup it returns a Decision without any I/O, goroutines, globals mutation,
// or wall-clock access. Phase 1 implements Bumblebee single-source matching:
// a catalog hit produces a warn (Allow stays true); everything else allows.
func Evaluate(tc ToolCall, idx CatalogLookup) Decision {
	ecosystem, pkg, version, ok := extract(tc.ToolInput)
	if !ok {
		return Decision{
			Allow:  true,
			Level:  "allow",
			Reason: "no package identified",
		}
	}

	entry, found := idx.Lookup(ecosystem, pkg)
	if !found || !versionMatches(entry, version) {
		return Decision{
			Allow:  true,
			Level:  "allow",
			Reason: "no catalog match",
		}
	}

	signed := catalog.VerifySignature(entry)
	match := CatalogMatch{
		CatalogSource: entry.CatalogSource,
		EntryID:       entry.ID,
		Ecosystem:     ecosystem,
		Package:       pkg,
		Version:       version,
		Severity:      entry.Severity,
		Signed:        signed,
	}

	// Phase 1: single source => warn. Warn never blocks (Allow stays true);
	// unsigned entries are warn-only too (CTLG-07). Corroboration-based block
	// escalation is Phase 2 (PLCY-01).
	return Decision{
		Allow:          true,
		Level:          "warn",
		Reason:         "bumblebee catalog match: " + entry.ID,
		RuleIDs:        []string{ruleBumblebeeCatalogMatch},
		CatalogMatches: []CatalogMatch{match},
	}
}

// versionMatches reports whether the extracted version is covered by the entry.
// An entry with no Versions applies to all versions (defense-favoring), and an
// extracted version of "" (no explicit @version) also matches defensively.
func versionMatches(e catalog.Entry, version string) bool {
	if len(e.Versions) == 0 || version == "" {
		return true
	}
	for _, v := range e.Versions {
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
