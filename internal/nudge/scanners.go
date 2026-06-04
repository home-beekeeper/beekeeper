package nudge

// scanners.go — IMPURE (reads files via injectable readFileFn, but scan
// logic is pure string parsing). This is the file-scan half of the detection
// adapter: it reads bunfig.toml / pnpm-workspace.yaml from known fixed paths
// and delegates to the pure scanBunfig / scanPnpmWorkspace string parsers.
//
// Security contract (T-08-12, T-08-13):
//   - Both scanners return safe defaults on any parse error and NEVER panic.
//   - Read is limited to FIXED, bounded paths only (bunfig.toml project root +
//     ~/.bunfig.toml; pnpm-workspace.yaml project root). File contents are
//     parsed, never executed (V12).
//   - FuzzBunfig and FuzzPnpmWorkspace (scanners_fuzz_test.go) are release-gate
//     fuzz targets that enforce the never-panic contract (BTEST-03).

import (
	"os"
	"strings"
)

// readFileFn is the injectable file-read function used by DetectBunScanner and
// DetectPnpmHardening. Tests can substitute a fake to avoid real file I/O.
// The real impl is os.ReadFile.
var readFileFn = os.ReadFile

// scanBunfig scans the contents of a bunfig.toml file for the
// @socketsecurity/bun-security-scanner entry in the [install.security] section.
//
// Returns (scannerOK, ok):
//   - scannerOK true → scanner is configured in this file.
//   - ok false → parse trouble encountered; caller must treat BunScannerOK=false
//     (§10-13, §6.2). ok=false does NOT mean crash — it means "uncertain/malformed".
//
// This is a conservative line scanner, NOT a full TOML parser. Tolerates common
// quoting and whitespace variants. On any structural ambiguity returns ok=false
// (safe default). Never panics regardless of input content (T-08-12).
func scanBunfig(content string) (scannerOK bool, ok bool) {
	lines := splitLines(content)
	inInstallSecurity := false

	for _, raw := range lines {
		line := strings.TrimSpace(raw)

		// Skip blank lines and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Detect section headers like [install.security] or [install.security] ...
		if strings.HasPrefix(line, "[") {
			end := strings.Index(line, "]")
			if end == -1 {
				// Malformed section header — safe default, report parse trouble.
				return false, false
			}
			section := strings.TrimSpace(line[1:end])
			inInstallSecurity = (section == "install.security")
			continue
		}

		if !inInstallSecurity {
			continue
		}

		// We are inside [install.security]. Look for:
		//   scanner = "@socketsecurity/bun-security-scanner"
		//   scanner = '@socketsecurity/bun-security-scanner'
		//   scanner="@socketsecurity/bun-security-scanner"
		// etc.
		if !strings.Contains(line, "scanner") {
			continue
		}
		key, val, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		if key != "scanner" {
			continue
		}
		// Strip surrounding quotes (single or double) from the value.
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)
		if val == "@socketsecurity/bun-security-scanner" {
			return true, true
		}
	}
	// Reached EOF without finding the scanner entry — not present (ok=true, scannerOK=false).
	return false, true
}

// scanPnpmWorkspace scans the contents of a pnpm-workspace.yaml file for
// hardening-relevant keys: minimumReleaseAge and blockExoticSubdeps.
//
// Returns:
//   - minReleaseAge: the parsed value of minimumReleaseAge (if present).
//   - hasMinReleaseAge: true if the key was found and parsed.
//   - blockExotic: the parsed bool value of blockExoticSubdeps (if present).
//   - hasBlockExotic: true if the key was found and parsed.
//   - ok: false when a structural parse problem was encountered; caller must
//     treat hardening state as "unknown" (safe defaults). Never panics (T-08-12).
//
// Flag 5 correction: the weakness baseline is 1440, NOT 60 (per pnpm 11 default).
// The caller (DetectPnpmHardening) uses minimumReleaseAgeWeaknessBaseline from
// config.go for the comparison. §10-16: minimumReleaseAge=0 → weakness flagged
// but pnpm_hardened stays true.
func scanPnpmWorkspace(content string) (minReleaseAge int, hasMinReleaseAge bool, blockExotic bool, hasBlockExotic bool, ok bool) {
	ok = true // assume well-formed unless we hit trouble
	lines := splitLines(content)

	for _, raw := range lines {
		line := strings.TrimSpace(raw)

		// Skip blank lines and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Simple YAML key: value parsing (top-level keys only — these are
		// top-level pnpm workspace keys, not nested).
		key, val, found := strings.Cut(line, ":")
		if !found {
			// Not a key: value line — could be a list item or other YAML. Skip.
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)

		switch key {
		case "minimumReleaseAge":
			n, err := parseInt(val)
			if err != nil {
				// Non-integer value — parse trouble.
				ok = false
				return
			}
			minReleaseAge = n
			hasMinReleaseAge = true

		case "blockExoticSubdeps":
			switch val {
			case "true":
				blockExotic = true
				hasBlockExotic = true
			case "false":
				blockExotic = false
				hasBlockExotic = true
			default:
				// Unknown boolean value — parse trouble.
				ok = false
				return
			}
		}
	}
	return
}

// DetectBunScanner checks the known bunfig.toml locations for the Socket security
// scanner. Returns true when the scanner is configured in at least one file.
//
// Locations checked (§6.2):
//  1. <cwd>/bunfig.toml (project root — caller supplies cwd via cwd parameter)
//  2. os.UserHomeDir()/".bunfig.toml" (user home)
//
// On any read or parse error the result for that file is treated as absent
// (BunScannerOK=false from that file). Never panics (T-08-12).
//
// cfg.CheckSocketScanner=false short-circuits to false without any file I/O.
func DetectBunScanner(bunfigPaths []string) bool {
	for _, p := range bunfigPaths {
		if p == "" {
			continue
		}
		data, err := readFileFn(p)
		if err != nil {
			// File not found or unreadable — safe default: scanner absent.
			continue
		}
		ok, parseOK := scanBunfig(string(data))
		if !parseOK {
			// Parse trouble — safe default.
			continue
		}
		if ok {
			return true
		}
	}
	return false
}

// HardeningResult captures the pnpm hardening analysis result.
type HardeningResult struct {
	// Hardened is true when pnpm 11 hardening defaults appear to be in place
	// (or the workspace file is absent, meaning pnpm defaults apply).
	Hardened bool
	// WeaknessLogged is true when a configuration weakness was detected:
	//   - minimumReleaseAge explicitly set below the 1440-minute baseline
	//   - blockExoticSubdeps explicitly set to false
	// Note (§10-16): minimumReleaseAge=0 sets WeaknessLogged=true but Hardened
	// stays true (it's the user's explicit choice, not a removal of hardening).
	WeaknessLogged bool
}

// DetectPnpmHardening checks pnpm-workspace.yaml at the given path for
// hardening configuration.
//
// The file is optional — if it does not exist (err from readFileFn) the function
// returns Hardened=true (pnpm 11 defaults are on, no override to detect).
//
// Parse failures return Hardened=true, WeaknessLogged=false (safe: fail-open for
// detection, never block on parse trouble — §10-12/13).
func DetectPnpmHardening(workspacePath string) HardeningResult {
	if workspacePath == "" {
		return HardeningResult{Hardened: true}
	}

	data, err := readFileFn(workspacePath)
	if err != nil {
		// File absent — pnpm 11 defaults are on.
		return HardeningResult{Hardened: true}
	}

	minAge, hasMinAge, blockExotic, hasBlockExotic, ok := scanPnpmWorkspace(string(data))
	if !ok {
		// Parse trouble — safe default: report hardened (fail-open for detection).
		return HardeningResult{Hardened: true}
	}

	result := HardeningResult{Hardened: true}

	// Flag 5: weakness baseline is 1440 (NOT 60).
	// §10-16: minimumReleaseAge=0 → weakness flagged, but hardened stays true.
	if hasMinAge && minAge < minimumReleaseAgeWeaknessBaseline {
		result.WeaknessLogged = true
		// hardened stays true — §10-16: user's explicit choice
	}

	if hasBlockExotic && !blockExotic {
		result.WeaknessLogged = true
		// blockExoticSubdeps=false is a weakness but does not remove hardening.
	}

	_ = minAge // used in the comparison above; suppress vet warning if no weakness

	return result
}

// splitLines splits content on newlines, supporting both \n and \r\n.
// Returns an empty slice for empty content.
func splitLines(content string) []string {
	if content == "" {
		return nil
	}
	// Normalize Windows CRLF to LF.
	content = strings.ReplaceAll(content, "\r\n", "\n")
	return strings.Split(content, "\n")
}

// maxAge is a generous ceiling for parseInt accumulation (WR-02). It bounds the
// running total so a huge minimumReleaseAge (e.g. the 35-digit fuzz seed in
// scanners_fuzz_test.go) cannot silently overflow a 64-bit int and wrap to an
// arbitrary — possibly negative — value that flips WeaknessLogged. The field is
// minutes; 1<<31 (~4084 years) is far beyond any legitimate value, so anything
// past it is treated as parse trouble → ok=false → safe default.
const maxAge = 1 << 31

// parseInt parses a decimal integer string. Returns (0, err) on failure,
// including when the magnitude exceeds maxAge (overflow / absurd value). The
// overflow guard maps to errNotInt, which the caller already treats as a parse
// error → safe default (WR-02).
func parseInt(s string) (int, error) {
	if s == "" {
		return 0, errEmpty
	}
	n := 0
	neg := false
	start := 0
	if len(s) > 0 && s[0] == '-' {
		neg = true
		start = 1
	}
	if start >= len(s) {
		return 0, errEmpty
	}
	for i := start; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, errNotInt
		}
		n = n*10 + int(c-'0')
		if n > maxAge {
			// Overflow / absurd magnitude — treat as parse trouble. Bounding
			// here (before n can wrap) is what makes the guard sound: n never
			// grows past maxAge*10 + 9, well inside int range.
			return 0, errNotInt
		}
	}
	if neg {
		n = -n
	}
	return n, nil
}

// sentinel errors for parseInt — not exported; callers check (n, err) pattern.
var (
	errEmpty  = scannerError("empty integer string")
	errNotInt = scannerError("non-integer character in value")
)

type scannerError string

func (e scannerError) Error() string { return string(e) }
