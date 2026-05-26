package policy

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestReleaseAgeYoungPackageBlocked: AgeMinutes < default threshold → block.
func TestReleaseAgeYoungPackageBlocked(t *testing.T) {
	input := ReleaseAgeInput{
		Ecosystem:  "npm",
		Package:    "fresh-pkg",
		AgeMinutes: 30, // 30 minutes old — below 1440 min default
	}
	cfg := DefaultReleaseAgeConfig()
	d := EvaluateReleaseAge(input, cfg)

	if d.Level != "block" {
		t.Errorf("Level = %q, want %q (young package must be blocked)", d.Level, "block")
	}
	if d.Allow {
		t.Errorf("Allow = true, want false (young package must be blocked)")
	}
	if !strings.Contains(d.Reason, "30") {
		t.Errorf("Reason %q should contain the age minutes (30)", d.Reason)
	}
	if len(d.RuleIDs) == 0 {
		t.Errorf("RuleIDs is empty, want at least one rule ID")
	}
}

// TestReleaseAgeOldPackageAllowed: AgeMinutes > default threshold → allow.
func TestReleaseAgeOldPackageAllowed(t *testing.T) {
	input := ReleaseAgeInput{
		Ecosystem:  "npm",
		Package:    "stable-pkg",
		AgeMinutes: 2000, // well over 24h
	}
	cfg := DefaultReleaseAgeConfig()
	d := EvaluateReleaseAge(input, cfg)

	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q (old package must be allowed)", d.Level, "allow")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true (old package must be allowed)")
	}
}

// TestReleaseAgeTimestampMissingBlocks: TimestampMissing true → fail-closed block.
func TestReleaseAgeTimestampMissingBlocks(t *testing.T) {
	input := ReleaseAgeInput{
		Ecosystem:        "npm",
		Package:          "unknown-pkg",
		AgeMinutes:       0,
		TimestampMissing: true,
	}
	cfg := DefaultReleaseAgeConfig()
	d := EvaluateReleaseAge(input, cfg)

	if d.Level != "block" {
		t.Errorf("Level = %q, want %q (missing timestamp must fail closed)", d.Level, "block")
	}
	if d.Allow {
		t.Errorf("Allow = true, want false (missing timestamp must fail closed)")
	}
	if !strings.Contains(d.Reason, "unavailable") {
		t.Errorf("Reason %q should contain 'unavailable'", d.Reason)
	}
}

// TestReleaseAgeAllowlistExempt: package on allowlist → allow regardless of age.
func TestReleaseAgeAllowlistExempt(t *testing.T) {
	input := ReleaseAgeInput{
		Ecosystem:  "npm",
		Package:    "my-trusted-pkg",
		AgeMinutes: 5, // brand new
	}
	cfg := DefaultReleaseAgeConfig()
	cfg.Exclude = []string{"my-trusted-pkg"}
	d := EvaluateReleaseAge(input, cfg)

	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q (allowlisted package must be exempt)", d.Level, "allow")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true (allowlisted package must be exempt)")
	}
	if !strings.Contains(d.Reason, "allowlist") {
		t.Errorf("Reason %q should mention 'allowlist'", d.Reason)
	}
}

// TestReleaseAgePerEcosystemOverride: npm with 60-min threshold, 90-min-old pkg → allow.
func TestReleaseAgePerEcosystemOverride(t *testing.T) {
	input := ReleaseAgeInput{
		Ecosystem:  "npm",
		Package:    "semi-fresh-pkg",
		AgeMinutes: 90, // 90 min old
	}
	cfg := DefaultReleaseAgeConfig()
	cfg.PerEcosystemMinutes = map[string]int64{
		"npm": 60, // custom: only 60 min required for npm
	}
	d := EvaluateReleaseAge(input, cfg)

	// 90 minutes > 60 minute threshold → should allow
	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q (90-min-old npm pkg with 60-min override must allow)", d.Level, "allow")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true (per-ecosystem override honored)")
	}
}

// TestReleaseAgeTimestampMissingOverridesAllowlist: even allowlisted packages fail
// closed on missing timestamp (missing = unknown age; can't exempt from unknown).
// Actually per spec: allowlist check happens BEFORE missing check.
// Let's verify allowlist takes precedence over missing timestamp per the spec:
// "if input.Package is in cfg.Exclude → allow" is the FIRST check.
func TestReleaseAgeAllowlistBeforeMissing(t *testing.T) {
	input := ReleaseAgeInput{
		Ecosystem:        "npm",
		Package:          "trusted-pkg",
		AgeMinutes:       0,
		TimestampMissing: true,
	}
	cfg := DefaultReleaseAgeConfig()
	cfg.Exclude = []string{"trusted-pkg"}
	d := EvaluateReleaseAge(input, cfg)

	// Per the plan action: "if input.Package is in cfg.Exclude → allow" is FIRST
	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q (allowlist takes precedence over missing timestamp)", d.Level, "allow")
	}
}

// TestReleaseAgeImportsArePure enforces the pure-library contract on release_age.go.
func TestReleaseAgeImportsArePure(t *testing.T) {
	const srcPath = "release_age.go"
	src, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("reading %s: %v", srcPath, err)
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, srcPath, src, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parsing %s: %v", srcPath, err)
	}

	forbidden := map[string]bool{
		"os":       true,
		"net":      true,
		"net/http": true,
		"io":       true,
		"sync":     true,
		"time":     true,
		"context":  true,
	}

	for _, imp := range f.Imports {
		path := imp.Path.Value
		if len(path) >= 2 {
			path = path[1 : len(path)-1]
		}
		if forbidden[path] {
			t.Errorf("release_age.go imports forbidden package %q — violates pure-library contract", path)
		}
	}
}
