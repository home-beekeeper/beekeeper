//go:build fuzz

// RELEASE GATE: FuzzBunfig and FuzzPnpmWorkspace must pass (seed corpus run) in
// CI before any release tag.
// Run: go test -tags fuzz -run "FuzzBunfig|FuzzPnpmWorkspace" ./internal/posture/...
// Fuzz: go test -tags fuzz -fuzz=FuzzBunfig -fuzztime=60s ./internal/posture/...
//
// These scanners must never panic on any input (the pm-config-reader never-panic
// contract, relocated with the readers from the former nudge package).

package posture

import (
	"testing"
)

// FuzzBunfig is the RELEASE GATE fuzz test for the bunfig.toml scanner.
//
// Contract: scanBunfig must NEVER panic on any input. It must always return
// (bool, bool) without any untyped panic, out-of-bounds access, or infinite
// loop, regardless of the input bytes.
func FuzzBunfig(f *testing.F) {
	// Valid: scanner present.
	f.Add("[install.security]\nscanner = \"@socketsecurity/bun-security-scanner\"\n")

	// Valid: scanner absent.
	f.Add("[install]\nregistry = \"https://registry.npmjs.org\"\n")

	// Empty.
	f.Add("")

	// Blank lines only.
	f.Add("\n\n\n")

	// Comment only.
	f.Add("# just a comment\n")

	// Truncated section header cut off.
	f.Add("[install.security")

	// Malformed bracket.
	f.Add("[missing-close\nscanner = \"@socketsecurity/bun-security-scanner\"\n")

	// Huge repetitive content (~10 KB).
	f.Add(string(makeLargeContent(10_000)))

	// Non-UTF8 bytes (hex escapes — no literal NUL in source).
	f.Add("[install.security]\nscanner = \"\xff\xfe\"\n")

	// Control byte via escape (not literal NUL in source).
	f.Add("[install.security]\nscanner" + string([]byte{0x00}) + "= \"value\"\n")

	// Shell metacharacters.
	f.Add("[install.security]\nscanner = \"$(rm -rf /)\"\n")
	f.Add("[install.security]\nscanner = \"value; evil\"\n")
	f.Add("[install.security]\nscanner = \"value && evil\"\n")

	// Key without value.
	f.Add("[install.security]\nscanner\n")

	// Value without key.
	f.Add("[install.security]\n= \"@socketsecurity/bun-security-scanner\"\n")

	// Multiple sections.
	f.Add("[a]\n[b]\n[install.security]\nscanner = \"@socketsecurity/bun-security-scanner\"\n")

	f.Fuzz(func(t *testing.T, data string) {
		// scanBunfig must NEVER panic on any input.
		_, _ = scanBunfig(data)
	})
}

// FuzzPnpmWorkspace is the RELEASE GATE fuzz test for the pnpm-workspace.yaml scanner.
//
// Contract: scanPnpmWorkspace must NEVER panic on any input. It must always
// return the 5-tuple without any untyped panic, out-of-bounds access, or
// infinite loop, regardless of the input bytes.
func FuzzPnpmWorkspace(f *testing.F) {
	// Valid: minimumReleaseAge and blockExoticSubdeps present.
	f.Add("minimumReleaseAge: 1440\nblockExoticSubdeps: true\n")

	// Valid: weakness values.
	f.Add("minimumReleaseAge: 0\nblockExoticSubdeps: false\n")

	// Empty.
	f.Add("")

	// Blank lines only.
	f.Add("\n\n\n")

	// Comment only.
	f.Add("# pnpm workspace\n")

	// Truncated.
	f.Add("minimumReleaseAge:")

	// Non-integer value.
	f.Add("minimumReleaseAge: not-a-number\n")

	// Large integer overflow attempt.
	f.Add("minimumReleaseAge: 99999999999999999999999999999999999\n")

	// Negative integer.
	f.Add("minimumReleaseAge: -1\n")

	// Huge repetitive content (~10 KB).
	f.Add(string(makeLargeContent(10_000)))

	// Non-UTF8 bytes via escape.
	f.Add("minimumReleaseAge: \xff\n")

	// Control byte via escape.
	f.Add("minimumReleaseAge" + string([]byte{0x00}) + ": 1440\n")

	// Shell metacharacters.
	f.Add("minimumReleaseAge: $(rm -rf /)\n")

	// Extra colons.
	f.Add("minimumReleaseAge: 1440: extra\n")

	// Unknown boolean values.
	f.Add("blockExoticSubdeps: yes\n")
	f.Add("blockExoticSubdeps: 1\n")
	f.Add("blockExoticSubdeps: TRUE\n")

	// YAML list item (should not crash).
	f.Add("packages:\n  - \"packages/*\"\nminimumReleaseAge: 1440\n")

	f.Fuzz(func(t *testing.T, data string) {
		// scanPnpmWorkspace must NEVER panic on any input.
		_, _, _, _, _ = scanPnpmWorkspace(data)
	})
}
