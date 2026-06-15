package policy

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// fakeMultiCatalog implements MultiCatalogLookup with canned matches keyed by
// "ecosystem::package". It performs no I/O and is safe for pure unit tests of
// Evaluate. Replaces the Phase 1 fakeCatalog for Phase 2 tests.
type fakeMultiCatalog struct {
	matchesByKey map[string][]CatalogMatch
}

func (f fakeMultiCatalog) LookupAll(ecosystem, pkg string) []CatalogMatch {
	return f.matchesByKey[ecosystem+"::"+pkg]
}

// newFakeMultiWithMatches builds a fakeMultiCatalog from a variadic list of
// (key, match) pairs where key is "ecosystem::package".
func newFakeMulti(matchesByKey map[string][]CatalogMatch) fakeMultiCatalog {
	return fakeMultiCatalog{matchesByKey: matchesByKey}
}

// nxConsoleMatch returns a single CatalogMatch for the Nx Console advisory.
func nxConsoleMatch(source string, signed bool) CatalogMatch {
	return CatalogMatch{
		CatalogSource: source,
		EntryID:       "advisory-2026-nx-console",
		Ecosystem:     "editor-extension",
		Package:       "nrwl.angular-console",
		Version:       "18.95.0",
		Severity:      "critical",
		Signed:        signed,
	}
}

// ─── Phase 2 multi-source corroboration tests ───────────────────────────────

// TestEvaluateSingleSignedSourceWarns: one signed critical match via fakeMultiCatalog →
// Phase-6: Level "block", Allow false, CorroborationCount 1.
// (nxConsoleMatch has Severity:"critical"; DefaultCorroborationThresholds() sets
// SeverityOverrides["critical"]={BlockAt:1}, so 1 signed critical source → block.)
func TestEvaluateSingleSignedSourceWarns(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{
		"editor-extension::nrwl.angular-console": {
			nxConsoleMatch("bumblebee", true),
		},
	})
	tc := ToolCall{
		AgentName: "test-agent",
		ToolName:  "Install",
		ToolInput: map[string]any{
			"ecosystem": "editor-extension",
			"package":   "nrwl.angular-console",
			"version":   "18.95.0",
		},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds(), AgentContext{})

	// Phase-6 (CORR-01): critical severity + 1 signed source → block (effectiveBlockAt=1).
	if d.Level != "block" {
		t.Errorf("Level = %q, want %q (critical single signed source must block)", d.Level, "block")
	}
	if d.Allow {
		t.Errorf("Allow = true, want false (critical single signed source → block)")
	}
	if d.CorroborationCount != 1 {
		t.Errorf("CorroborationCount = %d, want 1", d.CorroborationCount)
	}
	if len(d.SourcesAgreed) != 1 || d.SourcesAgreed[0] != "bumblebee" {
		t.Errorf("SourcesAgreed = %v, want [bumblebee]", d.SourcesAgreed)
	}
	if d.Quarantine {
		t.Error("Quarantine = true, want false (signedCount=1 < QuarantineAt=2)")
	}
}

// TestEvaluateTwoSignedSourcesBlock: two signed sources for same pkg →
// Level "block", Allow false, CorroborationCount 2, SourcesAgreed length 2.
func TestEvaluateTwoSignedSourcesBlock(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{
		"npm::lodash": {
			{CatalogSource: "bumblebee", EntryID: "bumblebee-lodash", Ecosystem: "npm", Package: "lodash", Signed: true, Severity: "high"},
			{CatalogSource: "osv", EntryID: "osv-lodash", Ecosystem: "npm", Package: "lodash", Signed: true, Severity: "high"},
		},
	})
	tc := ToolCall{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "npm install lodash@4.17.20"},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds(), AgentContext{})

	if d.Level != "block" {
		t.Errorf("Level = %q, want %q", d.Level, "block")
	}
	if d.Allow {
		t.Errorf("Allow = true, want false (block decision)")
	}
	if d.CorroborationCount != 2 {
		t.Errorf("CorroborationCount = %d, want 2", d.CorroborationCount)
	}
	if len(d.SourcesAgreed) != 2 {
		t.Errorf("len(SourcesAgreed) = %d, want 2; got %v", len(d.SourcesAgreed), d.SourcesAgreed)
	}
	if d.Quarantine {
		t.Error("Quarantine = true, want false (only 2 sources)")
	}
}

// TestEvaluateThreeSignedSourcesQuarantine: three signed sources →
// Level "block", Allow false, Quarantine true.
func TestEvaluateThreeSignedSourcesQuarantine(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{
		"npm::malicious-pkg": {
			{CatalogSource: "bumblebee", Ecosystem: "npm", Package: "malicious-pkg", Signed: true},
			{CatalogSource: "osv", Ecosystem: "npm", Package: "malicious-pkg", Signed: true},
			{CatalogSource: "socket", Ecosystem: "npm", Package: "malicious-pkg", Signed: true},
		},
	})
	tc := ToolCall{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "npm install malicious-pkg@1.0.0"},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds(), AgentContext{})

	if d.Level != "block" {
		t.Errorf("Level = %q, want %q", d.Level, "block")
	}
	if d.Allow {
		t.Errorf("Allow = true, want false")
	}
	if !d.Quarantine {
		t.Error("Quarantine = false, want true (three signed sources → quarantine)")
	}
	if d.CorroborationCount != 3 {
		t.Errorf("CorroborationCount = %d, want 3", d.CorroborationCount)
	}
}

// TestEvaluateNoMatchAllows: empty LookupAll result → Level "allow", Allow true.
func TestEvaluateNoMatchAllows(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{}) // empty catalog
	tc := ToolCall{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "npm install express@4.18.2"},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds(), AgentContext{})

	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q", d.Level, "allow")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true")
	}
	if len(d.CatalogMatches) != 0 {
		t.Errorf("len(CatalogMatches) = %d, want 0", len(d.CatalogMatches))
	}
}

// TestEvaluateUnsignedNeverBlocks: two unsigned sources → Level "warn", Allow true.
func TestEvaluateUnsignedNeverBlocks(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{
		"npm::suspect-pkg": {
			{CatalogSource: "sourceA", Ecosystem: "npm", Package: "suspect-pkg", Signed: false},
			{CatalogSource: "sourceB", Ecosystem: "npm", Package: "suspect-pkg", Signed: false},
		},
	})
	tc := ToolCall{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "npm install suspect-pkg@1.0.0"},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds(), AgentContext{})

	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q (unsigned sources must never block)", d.Level, "warn")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true (unsigned sources → warn, not block)")
	}
}

// ─── Migrated Phase 1 tests (using fakeMultiCatalog) ────────────────────────

// TestCatalogMatchProducesWarn: single signed source → warn, Allow true.
func TestCatalogMatchProducesWarn(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{
		"editor-extension::nrwl.angular-console": {
			nxConsoleMatch("bumblebee", false), // unsigned
		},
	})
	tc := ToolCall{
		AgentName: "test-agent",
		ToolName:  "Install",
		ToolInput: map[string]any{
			"ecosystem": "editor-extension",
			"package":   "nrwl.angular-console",
			"version":   "18.95.0",
		},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds(), AgentContext{})

	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q", d.Level, "warn")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true (warn does not block)")
	}
	if len(d.CatalogMatches) != 1 {
		t.Fatalf("len(CatalogMatches) = %d, want 1", len(d.CatalogMatches))
	}
	m := d.CatalogMatches[0]
	if m.EntryID != "advisory-2026-nx-console" {
		t.Errorf("EntryID = %q, want %q", m.EntryID, "advisory-2026-nx-console")
	}
	if m.Ecosystem != "editor-extension" || m.Package != "nrwl.angular-console" || m.Version != "18.95.0" {
		t.Errorf("match identity = %+v, want editor-extension/nrwl.angular-console/18.95.0", m)
	}
	if len(d.RuleIDs) == 0 {
		t.Errorf("RuleIDs = %v, want at least one rule ID", d.RuleIDs)
	}
}

// TestUnsignedCatalogIsWarnOnly: unsigned match → warn, Allow true, Signed false.
func TestUnsignedCatalogIsWarnOnly(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{
		"editor-extension::nrwl.angular-console": {
			nxConsoleMatch("bumblebee", false), // unsigned
		},
	})
	tc := ToolCall{
		ToolInput: map[string]any{
			"ecosystem": "editor-extension",
			"package":   "nrwl.angular-console",
			"version":   "18.95.0",
		},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds(), AgentContext{})

	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q", d.Level, "warn")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true — unsigned match must never block (CTLG-07)")
	}
	if len(d.CatalogMatches) != 1 {
		t.Fatalf("len(CatalogMatches) = %d, want 1", len(d.CatalogMatches))
	}
	if d.CatalogMatches[0].Signed {
		t.Errorf("Signed = true, want false for unsigned match")
	}
}

// TestSignedCatalogStillWarnWithSingleSource: one signed critical source → block (Phase-6 CORR-01).
// (nxConsoleMatch has Severity:"critical"; DefaultCorroborationThresholds() sets
// SeverityOverrides["critical"]={BlockAt:1}, so 1 signed critical source → block.)
func TestSignedCatalogStillWarnWithSingleSource(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{
		"editor-extension::nrwl.angular-console": {
			nxConsoleMatch("bumblebee", true), // signed, critical severity
		},
	})
	tc := ToolCall{
		ToolInput: map[string]any{
			"ecosystem": "editor-extension",
			"package":   "nrwl.angular-console",
			"version":   "18.95.0",
		},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds(), AgentContext{})

	// Phase-6 (CORR-01): critical + 1 signed source → block.
	if d.Level != "block" {
		t.Errorf("Level = %q, want %q (critical single signed source → block)", d.Level, "block")
	}
	if d.Allow {
		t.Errorf("Allow = true, want false (critical single signed source → block)")
	}
	if len(d.CatalogMatches) != 1 {
		t.Fatalf("len(CatalogMatches) = %d, want 1", len(d.CatalogMatches))
	}
	if !d.CatalogMatches[0].Signed {
		t.Errorf("Signed = false, want true for signed match")
	}
}

// TestNoMatchAllows: no entry in catalog → allow.
func TestNoMatchAllows(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{}) // empty
	tc := ToolCall{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "npm install express@4.18.2"},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds(), AgentContext{})

	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q", d.Level, "allow")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true")
	}
	if len(d.CatalogMatches) != 0 {
		t.Errorf("len(CatalogMatches) = %d, want 0", len(d.CatalogMatches))
	}
	if d.Reason != "no catalog match" {
		t.Errorf("Reason = %q, want %q", d.Reason, "no catalog match")
	}
}

// TestRemediatedVersionAllows: lookup returns nothing (version not matched by key).
func TestRemediatedVersionAllows(t *testing.T) {
	// Only the affected version key is in the catalog.
	idx := newFakeMulti(map[string][]CatalogMatch{
		"editor-extension::nrwl.angular-console": {
			// Adversarial: version 18.100.0 is safe (entry only covers 18.95.0 semantics
			// but for fakeMultiCatalog, the key already resolved — return empty to simulate no match)
		},
	})
	tc := ToolCall{
		ToolInput: map[string]any{
			"ecosystem": "editor-extension",
			"package":   "nrwl.angular-console",
			"version":   "18.100.0",
		},
	}
	// The fakeMultiCatalog key is "editor-extension::nrwl.angular-console" but matches are empty.
	d := Evaluate(tc, idx, DefaultCorroborationThresholds(), AgentContext{})

	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q", d.Level, "allow")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true")
	}
}

// TestNpmInstallCommandExtraction: npm install command extracts ecosystem+pkg+version.
// In Phase 2, the version on the CatalogMatch is pre-set by the adapter; the fake
// sets it to match what the command extracts.
func TestNpmInstallCommandExtraction(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{
		"npm::some-pkg": {
			{CatalogSource: "bumblebee", EntryID: "advisory-2026-npm", Ecosystem: "npm", Package: "some-pkg", Version: "1.0.0", Severity: "high", Signed: false},
		},
	})
	tc := ToolCall{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "npm install some-pkg@1.0.0"},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds(), AgentContext{})

	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q", d.Level, "warn")
	}
	if len(d.CatalogMatches) != 1 {
		t.Fatalf("len(CatalogMatches) = %d, want 1", len(d.CatalogMatches))
	}
	m := d.CatalogMatches[0]
	if m.Ecosystem != "npm" || m.Package != "some-pkg" || m.Version != "1.0.0" {
		t.Errorf("extracted match = %+v, want npm/some-pkg/1.0.0", m)
	}
}

// TestVersionlessCommandStillWarns: no explicit version still warns (defense-favoring).
func TestVersionlessCommandStillWarns(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{
		"npm::bad-pkg": {
			{CatalogSource: "bumblebee", EntryID: "advisory-2026-npm-allver", Ecosystem: "npm", Package: "bad-pkg", Severity: "high", Signed: false},
		},
	})
	tc := ToolCall{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "npm install bad-pkg"},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds(), AgentContext{})

	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q", d.Level, "warn")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true")
	}
}

// TestNoPackageIdentifiedAllows: tool call with no extractable package → allow.
func TestNoPackageIdentifiedAllows(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{})
	tc := ToolCall{
		ToolName:  "Read",
		ToolInput: map[string]any{},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds(), AgentContext{})

	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q", d.Level, "allow")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true")
	}
	if d.Reason != "no package identified" {
		t.Errorf("Reason = %q, want %q", d.Reason, "no package identified")
	}
	if len(d.CatalogMatches) != 0 {
		t.Errorf("len(CatalogMatches) = %d, want 0", len(d.CatalogMatches))
	}
}

// TestPackageNameNormalizedBeforeLookup: mixed-case + padded input still hits.
func TestPackageNameNormalizedBeforeLookup(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{
		"npm::mixedcase": {
			{CatalogSource: "bumblebee", EntryID: "advisory-2026-case", Ecosystem: "npm", Package: "mixedcase", Severity: "medium", Signed: false},
		},
	})
	tc := ToolCall{
		ToolInput: map[string]any{
			"ecosystem": "npm",
			"package":   "  MixedCase  ",
			"version":   "2.0.0",
		},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds(), AgentContext{})

	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q (package name must be normalized before lookup)", d.Level, "warn")
	}
}

// ─── EDXT-01 tests ──────────────────────────────────────────────────────────

// TestExtensionInstallExtract verifies that extractExtensionInstall correctly
// identifies editor-extension install commands and extracts ecosystem, package,
// and version.
func TestExtensionInstallExtract(t *testing.T) {
	cases := []struct {
		name    string
		cmd     string
		wantEco string
		wantPkg string
		wantVer string
		wantOK  bool
	}{
		{
			name:    "code with version",
			cmd:     "code --install-extension ms-python.python@2026.4.0",
			wantEco: "editor-extension",
			wantPkg: "ms-python.python",
			wantVer: "2026.4.0",
			wantOK:  true,
		},
		{
			name:    "code without version",
			cmd:     "code --install-extension ms-python.python",
			wantEco: "editor-extension",
			wantPkg: "ms-python.python",
			wantVer: "",
			wantOK:  true,
		},
		{
			name:    "cursor with version",
			cmd:     "cursor --install-extension foo.bar@1.0.0",
			wantEco: "editor-extension",
			wantPkg: "foo.bar",
			wantVer: "1.0.0",
			wantOK:  true,
		},
		{
			name:    "windsurf recognized",
			cmd:     "windsurf --install-extension foo.bar",
			wantEco: "editor-extension",
			wantPkg: "foo.bar",
			wantVer: "",
			wantOK:  true,
		},
		{
			name:    "code-insiders recognized",
			cmd:     "code-insiders --install-extension foo.bar",
			wantEco: "editor-extension",
			wantPkg: "foo.bar",
			wantVer: "",
			wantOK:  true,
		},
		{
			name:   "npm install not recognized",
			cmd:    "npm install left-pad",
			wantOK: false,
		},
		{
			name:   "code list-extensions not recognized",
			cmd:    "code --list-extensions",
			wantOK: false,
		},
		{
			name:    "uppercase command case-insensitive",
			cmd:     "CODE --INSTALL-EXTENSION ms-python.python",
			wantEco: "editor-extension",
			wantPkg: "ms-python.python",
			wantVer: "",
			wantOK:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eco, pkg, ver, ok := extractExtensionInstall(tc.cmd)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (cmd: %q)", ok, tc.wantOK, tc.cmd)
			}
			if !tc.wantOK {
				return
			}
			if eco != tc.wantEco {
				t.Errorf("ecosystem = %q, want %q", eco, tc.wantEco)
			}
			if pkg != tc.wantPkg {
				t.Errorf("pkg = %q, want %q", pkg, tc.wantPkg)
			}
			if ver != tc.wantVer {
				t.Errorf("ver = %q, want %q", ver, tc.wantVer)
			}
		})
	}
}

// TestExtensionInstallBulk verifies that extractAllExtensionInstalls returns all
// extension IDs from a bulk multi-flag command, and that Evaluate returns the worst
// decision when any extension in a bulk command would block.
func TestExtensionInstallBulk(t *testing.T) {
	// Verify extractAllExtensionInstalls returns both IDs.
	cmd := "code --install-extension a.b@1 --install-extension c.d@2"
	ids := extractAllExtensionInstalls(cmd)
	if len(ids) != 2 {
		t.Fatalf("extractAllExtensionInstalls returned %d IDs, want 2: %v", len(ids), ids)
	}
	hasAB := false
	hasCD := false
	for _, id := range ids {
		if id == "a.b" {
			hasAB = true
		}
		if id == "c.d" {
			hasCD = true
		}
	}
	if !hasAB {
		t.Errorf("expected a.b in %v", ids)
	}
	if !hasCD {
		t.Errorf("expected c.d in %v", ids)
	}

	// Verify Evaluate returns the worst decision (block) when one extension is malicious.
	// c.d is listed as blocked by two signed sources; a.b is clean.
	idx := newFakeMulti(map[string][]CatalogMatch{
		"editor-extension::c.d": {
			{CatalogSource: "bumblebee", Ecosystem: "editor-extension", Package: "c.d", Signed: true, Severity: "critical"},
			{CatalogSource: "osv", Ecosystem: "editor-extension", Package: "c.d", Signed: true, Severity: "critical"},
		},
	})
	tc := ToolCall{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": cmd},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds(), AgentContext{})
	if d.Level != "block" {
		t.Errorf("Level = %q, want %q — bulk command with one blocked extension must block", d.Level, "block")
	}
	if d.Allow {
		t.Error("Allow = true, want false — bulk command containing blocked extension must not allow")
	}
}

// TestExtensionInstallVariants verifies that all four editor prefixes (code,
// code-insiders, cursor, windsurf) are recognized as editor-extension installs.
func TestExtensionInstallVariants(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
	}{
		{"code", "code --install-extension ms-python.python@2026.4.0"},
		{"code-insiders", "code-insiders --install-extension ms-python.python@2026.4.0"},
		{"cursor", "cursor --install-extension ms-python.python@2026.4.0"},
		{"windsurf", "windsurf --install-extension ms-python.python@2026.4.0"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eco, pkg, ver, ok := extractExtensionInstall(tc.cmd)
			if !ok {
				t.Fatalf("extractExtensionInstall returned ok=false for cmd %q", tc.cmd)
			}
			if eco != "editor-extension" {
				t.Errorf("ecosystem = %q, want %q", eco, "editor-extension")
			}
			if pkg != "ms-python.python" {
				t.Errorf("pkg = %q, want %q", pkg, "ms-python.python")
			}
			if ver != "2026.4.0" {
				t.Errorf("ver = %q, want %q", ver, "2026.4.0")
			}
		})
	}
}

// ─── Phase 4 AgentContext tests (INTG-07) ────────────────────────────────────

// TestAgentContextDepthBlock: Evaluate with depth=11 (exceeds maxAgentDepth=10)
// must return a block decision with INTG-07 in rule_ids.
func TestAgentContextDepthBlock(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{}) // empty catalog — depth block fires first
	tc := ToolCall{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "npm install express"},
	}
	ac := AgentContext{Depth: 11}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds(), ac)

	if d.Allow {
		t.Error("Allow = true, want false — depth 11 exceeds maxAgentDepth (10)")
	}
	if d.Level != "block" {
		t.Errorf("Level = %q, want block", d.Level)
	}
	foundINTG07 := false
	for _, id := range d.RuleIDs {
		if id == "INTG-07" {
			foundINTG07 = true
		}
	}
	if !foundINTG07 {
		t.Errorf("RuleIDs = %v, want INTG-07 present", d.RuleIDs)
	}
	if !strings.Contains(d.Reason, "11") {
		t.Errorf("Reason = %q, want it to mention depth 11", d.Reason)
	}
}

// TestAgentContextDepthNormalize: Evaluate with depth=-1 normalizes to 0
// and must NOT produce a depth block (uses an empty catalog so result is allow).
func TestAgentContextDepthNormalize(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{}) // empty catalog
	tc := ToolCall{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "npm install safe-package@1.0.0"},
	}
	ac := AgentContext{Depth: -1}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds(), ac)

	// Negative depth normalized to 0 → no depth block; empty catalog → allow.
	if !d.Allow {
		t.Errorf("Allow = false, want true — depth=-1 normalized to 0 must not block")
	}
	if d.Level != "allow" {
		t.Errorf("Level = %q, want allow", d.Level)
	}
}

// TestPnpmAddCatalogMatch proves F3/SC1 closure: a "pnpm add <pkg>" command now
// resolves ecosystem "npm" via pkgparse and hits the catalog under key
// "npm::<pkg>". Before Plan 02, pnpm was absent from installPrefixes so pnpm
// installs silently bypassed catalog corroboration. This is the regression case
// that locks the SC1 fix end-to-end (NUDGE-01).
func TestPnpmAddCatalogMatch(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{
		// Key is "npm::evil-pkg" because pkgparse maps pnpm → Ecosystem "npm".
		"npm::evil-pkg": {
			{CatalogSource: "bumblebee", EntryID: "advisory-2026-pnpm-f3", Ecosystem: "npm", Package: "evil-pkg", Signed: true, Severity: "high"},
			{CatalogSource: "osv", EntryID: "osv-2026-pnpm-f3", Ecosystem: "npm", Package: "evil-pkg", Signed: true, Severity: "high"},
		},
	})
	tc := ToolCall{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "pnpm add evil-pkg"},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds(), AgentContext{})

	// Two signed sources → block (corroboration threshold met).
	if d.Level != "block" {
		t.Errorf("Level = %q, want %q — pnpm add must hit npm catalog (F3/SC1)", d.Level, "block")
	}
	if d.Allow {
		t.Errorf("Allow = true, want false — pnpm add of evil-pkg must be blocked")
	}
	if d.CorroborationCount != 2 {
		t.Errorf("CorroborationCount = %d, want 2", d.CorroborationCount)
	}
	// Reason must not mention "no package identified" or "no catalog match".
	if d.Reason == "no package identified" {
		t.Errorf("Reason = %q: pnpm was not recognised as an install verb (pkgparse routing broken)", d.Reason)
	}
	if d.Reason == "no catalog match" {
		t.Errorf("Reason = %q: pnpm add ecosystem was not mapped to \"npm\" (F3 bypass still present)", d.Reason)
	}
}

// TestBunAddCatalogMatch proves F3/SC1 closure for bun: same as pnpm case above.
func TestBunAddCatalogMatch(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{
		"npm::evil-pkg": {
			{CatalogSource: "bumblebee", EntryID: "advisory-2026-bun-f3", Ecosystem: "npm", Package: "evil-pkg", Signed: true, Severity: "high"},
			{CatalogSource: "osv", EntryID: "osv-2026-bun-f3", Ecosystem: "npm", Package: "evil-pkg", Signed: true, Severity: "high"},
		},
	})
	tc := ToolCall{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "bun add evil-pkg"},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds(), AgentContext{})

	if d.Level != "block" {
		t.Errorf("Level = %q, want %q — bun add must hit npm catalog (F3/SC1)", d.Level, "block")
	}
	if d.Allow {
		t.Errorf("Allow = true, want false — bun add of evil-pkg must be blocked")
	}
	if d.Reason == "no package identified" {
		t.Errorf("Reason = %q: bun was not recognised as an install verb (pkgparse routing broken)", d.Reason)
	}
	if d.Reason == "no catalog match" {
		t.Errorf("Reason = %q: bun add ecosystem was not mapped to \"npm\" (F3 bypass still present)", d.Reason)
	}
}

// TestBulkExtensionInstallQuarantinePropagated verifies TM-B-04: a bulk
// multi-extension install command that contains a 3-source/critical item must
// produce a merged Decision with Quarantine=true. Previously the bulk path
// tracked levelRank only and silently dropped the Quarantine flag, so the
// merged Decision would block without quarantining the offending extension.
func TestBulkExtensionInstallQuarantinePropagated(t *testing.T) {
	// evil.ext is matched by three signed sources → should quarantine.
	// safe.ext has no catalog entry → allow.
	idx := newFakeMulti(map[string][]CatalogMatch{
		"editor-extension::evil.ext": {
			{CatalogSource: "bumblebee", Ecosystem: "editor-extension", Package: "evil.ext", Signed: true, Severity: "high"},
			{CatalogSource: "osv", Ecosystem: "editor-extension", Package: "evil.ext", Signed: true, Severity: "high"},
			{CatalogSource: "socket", Ecosystem: "editor-extension", Package: "evil.ext", Signed: true, Severity: "high"},
		},
	})
	tc := ToolCall{
		ToolName: "Bash",
		ToolInput: map[string]any{
			"command": "code --install-extension safe.ext --install-extension evil.ext",
		},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds(), AgentContext{})

	if d.Level != "block" {
		t.Errorf("Level = %q, want %q — bulk install with 3-source item must block", d.Level, "block")
	}
	if d.Allow {
		t.Error("Allow = true, want false — bulk install must be denied")
	}
	if !d.Quarantine {
		t.Error("Quarantine = false, want true — 3-source constituent must propagate Quarantine to bulk merged Decision (TM-B-04)")
	}
	if d.CorroborationCount != 3 {
		t.Errorf("CorroborationCount = %d, want 3 (from 3-source evil.ext match)", d.CorroborationCount)
	}
}

// TestEngineImportsArePure enforces the purity contract: engine.go must not
// import any package that performs I/O, concurrency, or wall-clock access.
func TestEngineImportsArePure(t *testing.T) {
	const enginePath = "engine.go"
	src, err := os.ReadFile(enginePath)
	if err != nil {
		t.Fatalf("reading %s: %v", enginePath, err)
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, enginePath, src, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parsing %s: %v", enginePath, err)
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
			t.Errorf("engine.go imports forbidden package %q — violates pure-library contract", path)
		}
	}
}

// TestEvaluateNpxPackageFlagExtracted is the engine-level proof for Fix 5: an
// exec invocation that carries the package on a `--package=` flag value
// ("npx --package=evil-pkg run-bin") must extract "evil-pkg" and hit the catalog
// match instead of dropping the package and silently allowing. Without the
// pkgparse flag-binding, the package would be "" and this would no-match → allow.
func TestEvaluateNpxPackageFlagExtracted(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{
		"npm::evil-pkg": {
			{CatalogSource: "bumblebee", Ecosystem: "npm", Package: "evil-pkg", Signed: true, Severity: "critical"},
		},
	})
	for _, cmd := range []string{
		"npx --package=evil-pkg run-bin",
		"npx -p evil-pkg run-bin",
		"npx --package evil-pkg run-bin",
	} {
		tc := ToolCall{
			ToolName:  "Bash",
			ToolInput: map[string]any{"command": cmd},
		}
		d := Evaluate(tc, idx, DefaultCorroborationThresholds(), AgentContext{})
		if d.Level != "block" {
			t.Errorf("cmd %q: Level = %q, want %q — flag-borne package must be extracted and matched", cmd, d.Level, "block")
		}
		if d.Allow {
			t.Errorf("cmd %q: Allow = true, want false (catalog-corroborated critical match)", cmd)
		}
	}
}
