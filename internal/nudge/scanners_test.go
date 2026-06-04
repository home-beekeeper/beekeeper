package nudge

import (
	"os"
	"testing"
)

// TestScanBunfig covers scanBunfig with valid, absent, and malformed inputs.
func TestScanBunfig(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantOK      bool
		wantScanner bool
	}{
		{
			name: "scanner present double-quote",
			content: `[install.security]
scanner = "@socketsecurity/bun-security-scanner"
`,
			wantOK:      true,
			wantScanner: true,
		},
		{
			name: "scanner present single-quote",
			content: `[install.security]
scanner = '@socketsecurity/bun-security-scanner'
`,
			wantOK:      true,
			wantScanner: true,
		},
		{
			name: "scanner present no spaces around equals",
			content: `[install.security]
scanner="@socketsecurity/bun-security-scanner"
`,
			wantOK:      true,
			wantScanner: true,
		},
		{
			name: "scanner present with leading whitespace on key",
			content: `[install.security]
  scanner = "@socketsecurity/bun-security-scanner"
`,
			wantOK:      true,
			wantScanner: true,
		},
		{
			name:        "no install.security section",
			content:     `[install]\nregistry = "https://registry.npmjs.org"`,
			wantOK:      true,
			wantScanner: false,
		},
		{
			name:        "empty file",
			content:     "",
			wantOK:      true,
			wantScanner: false,
		},
		{
			name:        "install.security section without scanner key",
			content:     "[install.security]\n# no scanner key\n",
			wantOK:      true,
			wantScanner: false,
		},
		{
			name:        "scanner present wrong package name",
			content:     "[install.security]\nscanner = \"@other/scanner\"\n",
			wantOK:      true,
			wantScanner: false,
		},
		{
			name: "scanner present among other keys",
			content: `[install]
registry = "https://registry.npmjs.org"

[install.security]
depth = 5
scanner = "@socketsecurity/bun-security-scanner"
audit = true
`,
			wantOK:      true,
			wantScanner: true,
		},
		{
			name:        "comment lines only",
			content:     "# This is a comment\n# Another comment\n",
			wantOK:      true,
			wantScanner: false,
		},
		{
			name:        "malformed section header — unclosed bracket",
			content:     "[install.security\nscanner = \"@socketsecurity/bun-security-scanner\"\n",
			wantOK:      false,
			wantScanner: false,
		},
		{
			name: "scanner found before malformed section — returns true",
			// The scanner is found inside [install.security] before the malformed
			// header is encountered. Once found the loop returns immediately, so
			// ok=true scannerOK=true (malformed header after the match is unreachable).
			content:     "[install.security]\nscanner = \"@socketsecurity/bun-security-scanner\"\n[missing-close\n",
			wantOK:      true,
			wantScanner: true,
		},
		{
			name:        "malformed section header before any scanner entry",
			content:     "[missing-close\nscanner = \"@socketsecurity/bun-security-scanner\"\n",
			wantOK:      false,
			wantScanner: false,
		},
		{
			name:        "section not install.security — scanner found but wrong section",
			content:     "[dev.security]\nscanner = \"@socketsecurity/bun-security-scanner\"\n",
			wantOK:      true,
			wantScanner: false,
		},
		{
			name:        "non-UTF8 bytes — must not panic (§10-13)",
			content:     "[install.security]\nscanner = \"\xff\xfe\"\n",
			wantOK:      true,
			wantScanner: false,
		},
		{
			name:        "large repetitive content — must not panic",
			content:     string(makeLargeContent(100_000)),
			wantOK:      true,
			wantScanner: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotScanner, gotOK := scanBunfig(tc.content)
			if gotOK != tc.wantOK {
				t.Errorf("scanBunfig ok = %v, want %v", gotOK, tc.wantOK)
			}
			if gotScanner != tc.wantScanner {
				t.Errorf("scanBunfig scannerOK = %v, want %v", gotScanner, tc.wantScanner)
			}
		})
	}
}

// TestScanBunfigNeverPanics exercises scanBunfig with adversarial inputs to
// confirm it never panics (§10-13 contract).
func TestScanBunfigNeverPanics(t *testing.T) {
	adversarial := []string{
		"",
		"[",
		"]",
		"[[",
		"]]",
		"[" + string(make([]byte, 10_000)),
		"\x00\x01\x02\x03",
		"\xff\xfe\xfd",
		"scanner=",
		"scanner=\nscanner=\nscanner=",
		"[install.security]\nscanner",
		"[install.security]\n=value",
	}
	for i, s := range adversarial {
		// Call must not panic — if it does the test fails via the runtime.
		_, _ = scanBunfig(s)
		_ = i
	}
}

// TestScanPnpmWorkspace covers the pnpm-workspace.yaml scanner with valid,
// absent, and malformed inputs.
func TestScanPnpmWorkspace(t *testing.T) {
	tests := []struct {
		name             string
		content          string
		wantMinAge       int
		wantHasMinAge    bool
		wantBlockExotic  bool
		wantHasBlock     bool
		wantOK           bool
	}{
		{
			name:          "empty file",
			content:       "",
			wantOK:        true,
			wantHasMinAge: false,
			wantHasBlock:  false,
		},
		{
			name: "minimumReleaseAge present at 1440",
			content: `packages:
  - "packages/*"
minimumReleaseAge: 1440
`,
			wantOK:          true,
			wantMinAge:      1440,
			wantHasMinAge:   true,
			wantHasBlock:    false,
		},
		{
			name: "minimumReleaseAge set below baseline (weakness)",
			content: `minimumReleaseAge: 30
`,
			wantOK:          true,
			wantMinAge:      30,
			wantHasMinAge:   true,
			wantHasBlock:    false,
		},
		{
			name: "minimumReleaseAge=0 — §10-16 weakness but hardened stays true",
			content: `minimumReleaseAge: 0
`,
			wantOK:          true,
			wantMinAge:      0,
			wantHasMinAge:   true,
			wantHasBlock:    false,
		},
		{
			name: "blockExoticSubdeps=true",
			content: `blockExoticSubdeps: true
`,
			wantOK:           true,
			wantHasMinAge:    false,
			wantBlockExotic:  true,
			wantHasBlock:     true,
		},
		{
			name: "blockExoticSubdeps=false (weakness)",
			content: `blockExoticSubdeps: false
`,
			wantOK:           true,
			wantHasMinAge:    false,
			wantBlockExotic:  false,
			wantHasBlock:     true,
		},
		{
			name: "both keys present",
			content: `minimumReleaseAge: 1440
blockExoticSubdeps: true
`,
			wantOK:           true,
			wantMinAge:       1440,
			wantHasMinAge:    true,
			wantBlockExotic:  true,
			wantHasBlock:     true,
		},
		{
			name: "both keys with weakness",
			content: `minimumReleaseAge: 0
blockExoticSubdeps: false
`,
			wantOK:           true,
			wantMinAge:       0,
			wantHasMinAge:    true,
			wantBlockExotic:  false,
			wantHasBlock:     true,
		},
		{
			name: "minimumReleaseAge non-integer value — parse trouble",
			content: `minimumReleaseAge: lots
`,
			wantOK: false,
		},
		{
			name: "blockExoticSubdeps unknown value — parse trouble",
			content: `blockExoticSubdeps: maybe
`,
			wantOK: false,
		},
		{
			name:    "comment lines",
			content: "# pnpm workspace config\n# no keys\n",
			wantOK:  true,
		},
		{
			name: "non-UTF8 bytes — must not panic",
			content: "minimumReleaseAge: \xff\n",
			wantOK: false, // non-numeric → parse trouble
		},
		{
			name:    "large repetitive content",
			content: string(makeLargeContent(100_000)),
			wantOK:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			minAge, hasMinAge, blockExotic, hasBlock, ok := scanPnpmWorkspace(tc.content)
			if ok != tc.wantOK {
				t.Errorf("scanPnpmWorkspace ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return // parse trouble — remaining fields undefined
			}
			if hasMinAge != tc.wantHasMinAge {
				t.Errorf("hasMinReleaseAge = %v, want %v", hasMinAge, tc.wantHasMinAge)
			}
			if tc.wantHasMinAge && minAge != tc.wantMinAge {
				t.Errorf("minReleaseAge = %d, want %d", minAge, tc.wantMinAge)
			}
			if hasBlock != tc.wantHasBlock {
				t.Errorf("hasBlockExotic = %v, want %v", hasBlock, tc.wantHasBlock)
			}
			if tc.wantHasBlock && blockExotic != tc.wantBlockExotic {
				t.Errorf("blockExotic = %v, want %v", blockExotic, tc.wantBlockExotic)
			}
		})
	}
}

// TestScanPnpmWorkspaceNeverPanics exercises adversarial inputs (§10-13).
func TestScanPnpmWorkspaceNeverPanics(t *testing.T) {
	adversarial := []string{
		"",
		":",
		"::",
		"minimumReleaseAge:",
		"minimumReleaseAge: ",
		"\x00\x01\x02",
		"\xff\xfe",
		string(make([]byte, 10_000)),
		"minimumReleaseAge: -9999999999999999999",
		"blockExoticSubdeps: \x00",
	}
	for _, s := range adversarial {
		_, _, _, _, _ = scanPnpmWorkspace(s)
	}
}

// TestDetectPnpmHardeningWeaknessBaseline verifies the 1440-minute baseline
// (Flag 5 correction) and §10-16 semantics.
func TestDetectPnpmHardeningWeaknessBaseline(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		wantHardened   bool
		wantWeakness   bool
	}{
		{
			name:         "minimumReleaseAge=1440 — no weakness",
			content:      "minimumReleaseAge: 1440\n",
			wantHardened: true,
			wantWeakness: false,
		},
		{
			name:         "minimumReleaseAge=60 — below 1440 baseline, weakness",
			content:      "minimumReleaseAge: 60\n",
			wantHardened: true,
			wantWeakness: true,
		},
		{
			name:         "minimumReleaseAge=0 — §10-16: weakness flagged, hardened stays true",
			content:      "minimumReleaseAge: 0\n",
			wantHardened: true,
			wantWeakness: true,
		},
		{
			name:         "blockExoticSubdeps=false — weakness, hardened stays true",
			content:      "blockExoticSubdeps: false\n",
			wantHardened: true,
			wantWeakness: true,
		},
		{
			name:         "empty file — defaults apply, no weakness",
			content:      "",
			wantHardened: true,
			wantWeakness: false,
		},
		{
			name:         "parse failure — safe default: hardened true, no weakness",
			content:      "minimumReleaseAge: not-a-number\n",
			wantHardened: true,
			wantWeakness: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Override readFileFn to supply the test content.
			orig := readFileFn
			readFileFn = func(_ string) ([]byte, error) {
				return []byte(tc.content), nil
			}
			defer func() { readFileFn = orig }()

			result := DetectPnpmHardening("pnpm-workspace.yaml")
			if result.Hardened != tc.wantHardened {
				t.Errorf("Hardened = %v, want %v", result.Hardened, tc.wantHardened)
			}
			if result.WeaknessLogged != tc.wantWeakness {
				t.Errorf("WeaknessLogged = %v, want %v", result.WeaknessLogged, tc.wantWeakness)
			}
		})
	}
}

// TestDetectBunScannerFileRead verifies DetectBunScanner finds the scanner via
// injected readFileFn and handles file-not-found gracefully.
func TestDetectBunScannerFileRead(t *testing.T) {
	const goodContent = "[install.security]\nscanner = \"@socketsecurity/bun-security-scanner\"\n"
	const emptyContent = "[install]\n"

	t.Run("scanner present in first path", func(t *testing.T) {
		orig := readFileFn
		readFileFn = func(_ string) ([]byte, error) {
			return []byte(goodContent), nil
		}
		defer func() { readFileFn = orig }()

		if !DetectBunScanner([]string{"bunfig.toml"}) {
			t.Error("expected true — scanner present")
		}
	})

	t.Run("scanner absent in all paths", func(t *testing.T) {
		orig := readFileFn
		readFileFn = func(_ string) ([]byte, error) {
			return []byte(emptyContent), nil
		}
		defer func() { readFileFn = orig }()

		if DetectBunScanner([]string{"bunfig.toml", "~/.bunfig.toml"}) {
			t.Error("expected false — scanner absent")
		}
	})

	t.Run("file not found — safe default false", func(t *testing.T) {
		orig := readFileFn
		readFileFn = func(_ string) ([]byte, error) {
			return nil, os.ErrNotExist
		}
		defer func() { readFileFn = orig }()

		if DetectBunScanner([]string{"bunfig.toml"}) {
			t.Error("expected false — file not found")
		}
	})

	t.Run("empty paths slice — false", func(t *testing.T) {
		if DetectBunScanner(nil) {
			t.Error("expected false — no paths")
		}
	})

	t.Run("scanner present in second path", func(t *testing.T) {
		orig := readFileFn
		call := 0
		readFileFn = func(_ string) ([]byte, error) {
			call++
			if call == 1 {
				return []byte(emptyContent), nil
			}
			return []byte(goodContent), nil
		}
		defer func() { readFileFn = orig }()

		if !DetectBunScanner([]string{"bunfig.toml", "~/.bunfig.toml"}) {
			t.Error("expected true — scanner in second path")
		}
	})
}

// TestDetectPnpmHardeningFileNotFound verifies that a missing workspace file
// returns Hardened=true (pnpm 11 defaults apply).
func TestDetectPnpmHardeningFileNotFound(t *testing.T) {
	orig := readFileFn
	readFileFn = func(_ string) ([]byte, error) {
		return nil, os.ErrNotExist
	}
	defer func() { readFileFn = orig }()

	result := DetectPnpmHardening("pnpm-workspace.yaml")
	if !result.Hardened {
		t.Error("expected Hardened=true when workspace file is absent")
	}
	if result.WeaknessLogged {
		t.Error("expected WeaknessLogged=false when workspace file is absent")
	}
}

// TestDetectPnpmHardeningEmptyPath verifies that an empty path returns hardened.
func TestDetectPnpmHardeningEmptyPath(t *testing.T) {
	result := DetectPnpmHardening("")
	if !result.Hardened {
		t.Error("expected Hardened=true for empty path")
	}
}

// TestParseIntOverflow verifies WR-02: an absurdly large minimumReleaseAge does
// not silently overflow int and wrap to a bogus (possibly negative) value.
// parseInt must return a parse error past the maxAge ceiling, which the scanner
// maps to ok=false → safe default. This guards the WeaknessLogged comparison
// (minAge < minimumReleaseAgeWeaknessBaseline) from a wrapped value.
func TestParseIntOverflow(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		wantN   int
	}{
		{
			// The exact 35-digit fuzz seed from scanners_fuzz_test.go:101.
			name:    "35-digit overflow seed",
			input:   "99999999999999999999999999999999999",
			wantErr: true,
		},
		{
			name:    "negative overflow seed",
			input:   "-9999999999999999999",
			wantErr: true,
		},
		{
			name:    "just above maxAge ceiling",
			input:   "2147483649", // maxAge = 1<<31 = 2147483648
			wantErr: true,
		},
		{
			name:    "at maxAge ceiling is accepted",
			input:   "2147483648",
			wantErr: false,
			wantN:   1 << 31,
		},
		{
			name:    "legitimate value 1440 accepted",
			input:   "1440",
			wantErr: false,
			wantN:   1440,
		},
		{
			name:    "zero accepted",
			input:   "0",
			wantErr: false,
			wantN:   0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n, err := parseInt(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseInt(%q) = (%d, nil), want error", tc.input, n)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseInt(%q) unexpected error: %v", tc.input, err)
			}
			if n != tc.wantN {
				t.Errorf("parseInt(%q) = %d, want %d", tc.input, n, tc.wantN)
			}
		})
	}
}

// TestScanPnpmWorkspaceOverflowIsParseTrouble verifies WR-02 end-to-end: a
// minimumReleaseAge that overflows is treated as parse trouble (ok=false), so
// DetectPnpmHardening returns the safe default rather than a wrapped value that
// could flip WeaknessLogged.
func TestScanPnpmWorkspaceOverflowIsParseTrouble(t *testing.T) {
	content := "minimumReleaseAge: 99999999999999999999999999999999999\n"
	_, _, _, _, ok := scanPnpmWorkspace(content)
	if ok {
		t.Fatalf("scanPnpmWorkspace ok = true for overflow value, want false (parse trouble)")
	}

	// End-to-end through DetectPnpmHardening: parse trouble → safe default.
	orig := readFileFn
	readFileFn = func(_ string) ([]byte, error) { return []byte(content), nil }
	defer func() { readFileFn = orig }()

	result := DetectPnpmHardening("pnpm-workspace.yaml")
	if !result.Hardened {
		t.Errorf("Hardened = false on overflow value, want true (safe default)")
	}
	if result.WeaknessLogged {
		t.Errorf("WeaknessLogged = true on overflow value, want false — a wrapped int must not flip the weakness flag")
	}
}

// makeLargeContent creates n bytes of benign repeating ASCII content for
// large-input tests.
func makeLargeContent(n int) []byte {
	b := make([]byte, n)
	const pat = "# comment line\n"
	for i := range b {
		b[i] = pat[i%len(pat)]
	}
	return b
}
