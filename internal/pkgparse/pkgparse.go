package pkgparse

// PURE — imports only "strings". No os/net/io/time/sync/context.
// Enforced by TestPkgparseImportsArePure in pkgparse_test.go.

import "strings"

// ParsedCommand is the result of parsing an agent-issued install command.
//
// Manager is the literal command word ("npm", "pnpm", "bun", "yarn", "npx",
// "pip", "pip3", "go", "gem", "cargo", "composer"). Ecosystem is the catalog
// lookup key (pnpm/bun/yarn/npx map to "npm" so LookupAll("npm", pkg) matches
// — this closes F3/SC1). Package is normalized (lowercase + trimmed). Version
// is the "@version" suffix if present; "" means unpinned.
//
// IsInstall is true for all install-class verbs (install, i, add, get, require,
// dlx, x). IsExec is true for exec verbs (npx, pnpm dlx, bun x) which also set
// IsInstall (§10-9). Sudo is true when the command was prefixed with "sudo".
//
// Unpinned (NUDGE-05): true when Version ends with "latest", when there is no
// "@version" at all (bare name), or when Version starts with "^" or "~"; false
// for an exact pinned version (e.g. "5.4.0").
type ParsedCommand struct {
	Raw       string // original command verbatim
	Manager   string // "npm" | "pnpm" | "bun" | "yarn" | "npx" | "pip" | ...
	Ecosystem string // catalog key: "npm" / "pypi" / "go" / "rubygems" / "cargo" / "packagist"
	Verb      string // "install" | "i" | "add" | "get" | "require" | "dlx" | "x" | ""
	Package   string // normalized (lowercase + trimmed); "" for no-arg install
	Version   string // from trailing @version; "" if absent
	IsInstall bool   // true for install-class verbs
	IsExec    bool   // true for exec verbs (npx, pnpm dlx, bun x)
	Sudo      bool   // true when "sudo " prefix was stripped
	Unpinned  bool   // true when no exact version pin (NUDGE-05)
}

// installEntry is one row in the prefix dispatch table.
type installEntry struct {
	// prefix is matched (case-insensitively) against the trimmed, lowercased command.
	prefix string
	// manager is the literal package-manager word.
	manager string
	// ecosystem is the catalog lookup key.
	ecosystem string
	// verb is the sub-command (install / i / add / dlx / x / get / etc.).
	verb string
	// isExec flags exec-style verbs (npx, pnpm dlx, bun x).
	isExec bool
}

// installTable maps install-command prefixes to their metadata. Entries are
// sorted so that more-specific prefixes appear before shorter ones that share a
// leading word (e.g. "cargo add" before "cargo install").
//
// pnpm/bun/yarn entries map to Ecosystem "npm" because those managers install
// from the npm registry — without this mapping, a pnpm-installed malicious
// package would not be caught by LookupAll("npm", pkg) (F3/SC1).
//
// "npm add" is included per PRD §6.4 — the live engine.go table lacks it,
// causing a silent §10-7/§10-9 hole.
var installTable = []installEntry{
	// npm — exact verb variants
	{"npm install", "npm", "npm", "install", false},
	{"npm i ", "npm", "npm", "i", false},
	{"npm add ", "npm", "npm", "add", false},

	// npx — always exec + install (§10-9)
	{"npx ", "npx", "npm", "", true},

	// pnpm (npm registry → ecosystem "npm")
	{"pnpm add ", "pnpm", "npm", "add", false},
	{"pnpm install", "pnpm", "npm", "install", false},
	{"pnpm i ", "pnpm", "npm", "i", false},
	{"pnpm dlx ", "pnpm", "npm", "dlx", true},

	// bun (npm registry → ecosystem "npm")
	{"bun add ", "bun", "npm", "add", false},
	{"bun install", "bun", "npm", "install", false},
	{"bun i ", "bun", "npm", "i", false},
	{"bun x ", "bun", "npm", "x", true},

	// yarn (npm registry → ecosystem "npm")
	{"yarn add ", "yarn", "npm", "add", false},
	{"yarn install", "yarn", "npm", "install", false},

	// Python
	{"pip install", "pip", "pypi", "install", false},
	{"pip3 install", "pip3", "pypi", "install", false},

	// Go
	{"go get", "go", "go", "get", false},

	// Ruby
	{"gem install", "gem", "rubygems", "install", false},

	// Rust
	{"cargo add", "cargo", "cargo", "add", false},
	{"cargo install", "cargo", "cargo", "install", false},

	// PHP
	{"composer require", "composer", "packagist", "require", false},
}

// Parse parses an agent-issued install command and returns a ParsedCommand.
// ok is false when the command is not a recognised install/exec verb (e.g.
// "npm ls", "npm run start", "npm publish" — §10-7 non-install).
//
// Compound-command coverage (SECURITY): an install verb is detected even when it
// is NOT the first token of the command. The command is split on shell separators
// (`&&`, `||`, `;`, `|`, `&`, newlines) and each segment is examined, with leading
// environment-variable assignments (`VAR=val cmd`) and a `sudo` prefix stripped.
// This closes a bypass where `cd /project && npm install evil-pkg` or
// `NODE_ENV=prod npm install evil-pkg` would otherwise escape both the nudge and
// the catalog block (engine.go routes Bash-command package extraction through this
// function). The FIRST segment that resolves to an install verb wins.
//
// Parse is PURE: it does no I/O, no exec, no globals mutation. It may safely
// be called from any goroutine without synchronisation.
func Parse(cmd string) (ParsedCommand, bool) {
	for _, seg := range splitSegments(cmd) {
		if pc, ok := parseSegment(cmd, seg); ok {
			return pc, true
		}
	}
	// No segment resolved to an install verb → non-install (§10-7).
	return ParsedCommand{}, false
}

// parseSegment attempts to parse a single shell segment as an install command.
// raw is the full original command (preserved verbatim in ParsedCommand.Raw);
// seg is the individual segment. Leading env-var assignments and a sudo prefix
// are stripped before the install-prefix table is consulted.
func parseSegment(raw, seg string) (ParsedCommand, bool) {
	trimmed := stripLeadingEnvAssignments(strings.TrimSpace(seg))

	// Strip a leading "sudo " and set Sudo=true (§6.4 criterion 10). Handle both
	// "sudo VAR=val cmd" and "VAR=val sudo cmd" by re-stripping env after sudo.
	sudo := false
	if strings.HasPrefix(strings.ToLower(trimmed), "sudo ") {
		sudo = true
		trimmed = stripLeadingEnvAssignments(strings.TrimSpace(trimmed[len("sudo "):]))
	}

	lower := strings.ToLower(trimmed)

	for _, entry := range installTable {
		if !strings.HasPrefix(lower, entry.prefix) {
			continue
		}

		// Rest of command after the matched prefix.
		rest := strings.TrimSpace(trimmed[len(entry.prefix):])

		// For exec-only verbs (npx, pnpm dlx, bun x) the "package" may be a
		// script or binary name; treat it the same as an install token.
		token := firstPackageToken(rest)
		pkg := ""
		ver := ""
		unpinned := false

		if token != "" {
			name, v := splitVersion(token)
			if name != "" {
				pkg = normalize(name)
				ver = strings.TrimSpace(v)
				unpinned = computeUnpinned(pkg, ver)
			}
		} else {
			// No-arg install (§10-8): IsInstall=true, Package="" — Unpinned is not
			// meaningful for a no-arg install (no package to pin), leave false.
		}

		return ParsedCommand{
			Raw:       raw,
			Manager:   entry.manager,
			Ecosystem: entry.ecosystem,
			Verb:      entry.verb,
			Package:   pkg,
			Version:   ver,
			IsInstall: true,
			IsExec:    entry.isExec,
			Sudo:      sudo,
			Unpinned:  unpinned,
		}, true
	}

	return ParsedCommand{}, false
}

// splitSegments splits a shell command on command separators (`&&`, `||`, `;`,
// `|`, `&`, newline, carriage-return) into individual command segments. It scans
// byte-by-byte (no sentinel substitution) so it is safe on inputs containing NUL
// or arbitrary control bytes, and never panics. Quoting is intentionally NOT
// honoured: for a security detector, over-splitting a quoted separator only risks
// examining an extra (harmless) segment, whereas UNDER-splitting would miss an
// install verb — so we err toward more segments.
func splitSegments(cmd string) []string {
	var segs []string
	start := 0
	for i := 0; i < len(cmd); {
		if i+1 < len(cmd) {
			if two := cmd[i : i+2]; two == "&&" || two == "||" {
				segs = append(segs, cmd[start:i])
				i += 2
				start = i
				continue
			}
		}
		switch cmd[i] {
		case ';', '|', '&', '\n', '\r':
			segs = append(segs, cmd[start:i])
			i++
			start = i
		default:
			i++
		}
	}
	return append(segs, cmd[start:])
}

// stripLeadingEnvAssignments removes any leading `VAR=value` environment-variable
// assignments from a command segment (e.g. "NODE_ENV=prod FORCE=1 npm install"
// → "npm install"). It stops at the first token that is not a valid assignment.
func stripLeadingEnvAssignments(s string) string {
	for {
		s = strings.TrimLeft(s, " \t")
		sp := strings.IndexAny(s, " \t")
		if sp <= 0 {
			return s
		}
		if !isEnvAssignment(s[:sp]) {
			return s
		}
		s = s[sp+1:]
	}
}

// isEnvAssignment reports whether tok has the shape NAME=... where NAME is a
// valid shell identifier ([A-Za-z_][A-Za-z0-9_]*). The value part is not
// inspected. Used to skip leading env-var assignments in stripLeadingEnvAssignments.
func isEnvAssignment(tok string) bool {
	eq := strings.IndexByte(tok, '=')
	if eq <= 0 {
		return false
	}
	for i := 0; i < eq; i++ {
		c := tok[i]
		switch {
		case c == '_', c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z':
			// valid identifier char
		case c >= '0' && c <= '9' && i > 0:
			// digit allowed after the first character
		default:
			return false
		}
	}
	return true
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

// normalize lowercases and trims a package name to match catalog index key
// normalization (the catalog index lowercases the package on build).
func normalize(pkg string) string {
	return strings.ToLower(strings.TrimSpace(pkg))
}

// computeUnpinned returns true (NUDGE-05) when the install is not exactly
// pinned:
//   - version is "" (bare name, no "@version" at all)
//   - version ends with "latest" (e.g. "@latest")
//   - version starts with "^" or "~" (semver range)
//
// An exact version string like "5.4.0" returns false.
func computeUnpinned(pkg, version string) bool {
	if pkg == "" {
		return false // no package at all; no meaningful pin state
	}
	if version == "" {
		return true // bare name, no version specified
	}
	if strings.HasSuffix(version, "latest") {
		return true // @latest or @next/latest
	}
	if strings.HasPrefix(version, "^") || strings.HasPrefix(version, "~") {
		return true // semver range
	}
	return false
}
