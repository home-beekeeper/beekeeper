package nudge

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/bantuson/beekeeper/internal/pkgparse"
)

// pmStateHardenedPnpm returns a PMState with pnpm >= 11 installed + hardened.
func pmStateHardenedPnpm(nodeVersion string) PMState {
	return PMState{
		NpmInstalled:  true,
		NpmVersion:    "10.9.0",
		PnpmInstalled: true,
		PnpmVersion:   "11.5.1",
		PnpmHardened:  true,
		NodeVersion:   nodeVersion,
	}
}

// pmStateHardenedBun returns a PMState with bun >= 1.3 installed.
func pmStateHardenedBun(scannerOK bool) PMState {
	return PMState{
		NpmInstalled: true,
		NpmVersion:   "10.9.0",
		BunInstalled: true,
		BunVersion:   "1.3.14",
		BunScannerOK: scannerOK,
	}
}

// pmStateNone returns a PMState with no hardened PM.
func pmStateNone() PMState {
	return PMState{
		NpmInstalled: true,
		NpmVersion:   "10.9.0",
	}
}

// parseOrFatal parses a command and fails the test if not ok.
func parseOrFatal(t *testing.T, cmd string) pkgparse.ParsedCommand {
	t.Helper()
	parsed, ok := pkgparse.Parse(cmd)
	if !ok {
		t.Fatalf("pkgparse.Parse(%q) = ok=false; expected an install command", cmd)
	}
	return parsed
}

// TestEvaluatePRDSection10 covers all 10 acceptance criteria from PRD §10.
func TestEvaluatePRDSection10(t *testing.T) {
	softCfg := DefaultConfig() // mode=soft, requireHardened=false
	hardCfg := DefaultConfig()
	hardCfg.Mode = "hard"

	tests := []struct {
		name      string
		cmd       pkgparse.ParsedCommand
		state     PMState
		cfg       Config
		wantAct   Action
		wantReason string
		wantLevel string
		wantAllow bool   // Level "allow"/"warn" → Allow=true; "block" → Allow=false
		wantRewritten string // non-empty only for Rewrite
		wantNudgeAction string
	}{
		// §10-1: pnpm >= 11 installed, mode soft, "npm install foo" → Advise
		{
			name:        "§10-1 pnpm soft advise",
			cmd:         parseOrFatal(t, "npm install foo"),
			state:       pmStateHardenedPnpm("22.5.0"),
			cfg:         softCfg,
			wantAct:     Advise,
			wantReason:  ReasonPnpmAvailableSoft,
			wantLevel:   "warn",
			wantAllow:   true,
			wantNudgeAction: "advise",
		},
		// §10-2: pnpm >= 11 installed, mode hard → Rewrite with "pnpm add foo"
		{
			name:        "§10-2 pnpm hard rewrite",
			cmd:         parseOrFatal(t, "npm install foo"),
			state:       pmStateHardenedPnpm("22.5.0"),
			cfg:         hardCfg,
			wantAct:     Rewrite,
			wantReason:  ReasonPnpmHardRewrite,
			wantLevel:   "warn",
			wantAllow:   true,
			wantRewritten: "pnpm add foo",
			wantNudgeAction: "rewrite",
		},
		// §10-3: no hardened PM, requireHardened false → Proceed
		{
			name:        "§10-3 no hardened PM proceed",
			cmd:         parseOrFatal(t, "npm install foo"),
			state:       pmStateNone(),
			cfg:         softCfg,
			wantAct:     Proceed,
			wantReason:  ReasonNoHardenedPM,
			wantLevel:   "allow",
			wantAllow:   true,
			wantNudgeAction: "proceed",
		},
		// §10-4: no hardened PM, requireHardened true → Block
		{
			name: "§10-4 requireHardened block",
			cmd:  parseOrFatal(t, "npm install foo"),
			state: pmStateNone(),
			cfg: func() Config {
				c := DefaultConfig()
				c.RequireHardened = true
				return c
			}(),
			wantAct:     Block,
			wantReason:  ReasonNoHardenedPM,
			wantLevel:   "block",
			wantAllow:   false,
			wantNudgeAction: "block",
		},
		// §10-5: bun >= 1.3 installed, scanner absent → Advise bun-available-no-scanner
		{
			name:        "§10-5 bun no scanner",
			cmd:         parseOrFatal(t, "npm install foo"),
			state:       pmStateHardenedBun(false),
			cfg:         softCfg,
			wantAct:     Advise,
			wantReason:  ReasonBunAvailableNoScanner,
			wantLevel:   "warn",
			wantAllow:   true,
			wantNudgeAction: "advise",
		},
		// §10-6: pnpm 11 installed, NodeVersion < 22 → Advise node-incompatible-with-pnpm-11
		{
			name:        "§10-6 node incompatible",
			cmd:         parseOrFatal(t, "npm install foo"),
			state:       pmStateHardenedPnpm("20.0.0"),
			cfg:         softCfg,
			wantAct:     Advise,
			wantReason:  ReasonNodeIncompatiblePnpm11,
			wantLevel:   "warn",
			wantAllow:   true,
			wantNudgeAction: "advise",
		},
		// §10-7: non-install → Proceed not-applicable (npm ls returns ok=false from pkgparse)
		// We simulate this by creating a command with IsInstall=false explicitly.
		// (pkgparse returns ok=false for "npm ls" so Evaluate is never called;
		//  we test the cfg.Enabled=false path which is equivalent to IsInstall=false)
		{
			name: "§10-7 non-install IsInstall=false",
			cmd: pkgparse.ParsedCommand{
				Raw:       "npm ls",
				Manager:   "npm",
				IsInstall: false,
			},
			state:       pmStateHardenedPnpm("22.5.0"),
			cfg:         softCfg,
			wantAct:     Proceed,
			wantReason:  ReasonNotApplicable,
			wantLevel:   "allow",
			wantAllow:   true,
			wantNudgeAction: "proceed",
		},
		// §10-8: no-arg "npm install" → Advise with softer reason no-arg-install-soft
		{
			name:        "§10-8 no-arg soft reason",
			cmd:         parseOrFatal(t, "npm install"),
			state:       pmStateHardenedPnpm("22.5.0"),
			cfg:         softCfg,
			wantAct:     Advise,
			wantReason:  ReasonNoArgInstallSoft,
			wantLevel:   "warn",
			wantAllow:   true,
			wantNudgeAction: "advise",
		},
		// §10-9: npx → still nudged (Advise/Rewrite per mode)
		{
			name:        "§10-9 npx nudged soft",
			cmd:         parseOrFatal(t, "npx create-app"),
			state:       pmStateHardenedPnpm("22.5.0"),
			cfg:         softCfg,
			wantAct:     Advise,
			wantReason:  ReasonPnpmAvailableSoft,
			wantLevel:   "warn",
			wantAllow:   true,
			wantNudgeAction: "advise",
		},
		// §10-10: sudo → Advise sudo-passthrough, NEVER Rewrite (even in hard mode)
		{
			name: "§10-10 sudo passthrough never rewrite",
			cmd: func() pkgparse.ParsedCommand {
				p, _ := pkgparse.Parse("sudo npm install foo")
				return p
			}(),
			state:       pmStateHardenedPnpm("22.5.0"),
			cfg:         hardCfg, // hard mode — but sudo must still not rewrite
			wantAct:     Advise,
			wantReason:  ReasonSudoPassthrough,
			wantLevel:   "warn",
			wantAllow:   true,
			wantNudgeAction: "advise",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := Evaluate(tc.cmd, tc.state, tc.cfg)

			if d.Action != tc.wantAct {
				t.Errorf("Action = %v (%s), want %v (%s)", d.Action, ActionString(d.Action), tc.wantAct, ActionString(tc.wantAct))
			}
			if d.Reason != tc.wantReason {
				t.Errorf("Reason = %q, want %q", d.Reason, tc.wantReason)
			}
			if d.Level != tc.wantLevel {
				t.Errorf("Level = %q, want %q", d.Level, tc.wantLevel)
			}
			// Allow is derived from Level: "allow"/"warn" → true, "block" → false
			wantAllow := d.Level != "block"
			if wantAllow != tc.wantAllow {
				t.Errorf("(Level=%q → Allow=%v), want Allow=%v", d.Level, wantAllow, tc.wantAllow)
			}
			if tc.wantRewritten != "" && d.Rewritten != tc.wantRewritten {
				t.Errorf("Rewritten = %q, want %q", d.Rewritten, tc.wantRewritten)
			}
			if tc.wantRewritten == "" && d.Action != Rewrite && d.Rewritten != "" {
				t.Errorf("Rewritten = %q, expected empty for non-Rewrite decision", d.Rewritten)
			}
			// ActionString must match the closed §9 enum.
			gotNudgeAction := ActionString(d.Action)
			if gotNudgeAction != tc.wantNudgeAction {
				t.Errorf("ActionString(%v) = %q, want %q", d.Action, gotNudgeAction, tc.wantNudgeAction)
			}
			// AuditFields["nudge_action"] must carry the closed §9 enum string.
			if d.AuditFields == nil {
				t.Errorf("AuditFields is nil")
			} else {
				if na, ok := d.AuditFields["nudge_action"]; !ok {
					t.Errorf("AuditFields missing nudge_action key")
				} else if na != tc.wantNudgeAction {
					t.Errorf("AuditFields[nudge_action] = %q, want %q", na, tc.wantNudgeAction)
				}
				// AuditFields must NOT set "decision" (that's the allow|warn|block Level field,
				// mapped by the Plan 06 adapter — not by Evaluate).
				if _, ok := d.AuditFields["decision"]; ok {
					t.Errorf("AuditFields[decision] is set by Evaluate — it must NOT be (§9 vocabulary separation)")
				}
			}
			// Reason code must be valid.
			if !IsValidReason(d.Reason) {
				t.Errorf("Reason %q is not a valid reason code", d.Reason)
			}
			// Original must equal cmd.Raw.
			if d.Original != tc.cmd.Raw {
				t.Errorf("Original = %q, want cmd.Raw %q", d.Original, tc.cmd.Raw)
			}
		})
	}
}

// TestEvaluateDisabled verifies cfg.Enabled=false → Proceed/not-applicable.
func TestEvaluateDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = false
	cmd := parseOrFatal(t, "npm install foo")
	d := Evaluate(cmd, pmStateHardenedPnpm("22.5.0"), cfg)
	if d.Action != Proceed {
		t.Errorf("Action = %v, want Proceed when cfg.Enabled=false", d.Action)
	}
	if d.Reason != ReasonNotApplicable {
		t.Errorf("Reason = %q, want %q", d.Reason, ReasonNotApplicable)
	}
}

// TestActionString verifies the closed §9 enum.
func TestActionString(t *testing.T) {
	tests := []struct {
		a    Action
		want string
	}{
		{Proceed, "proceed"},
		{Advise, "advise"},
		{Rewrite, "rewrite"},
		{Block, "block"},
		{Action(99), "proceed"}, // out-of-range → safe default, never panic
		{Action(-1), "proceed"},
	}
	for _, tc := range tests {
		got := ActionString(tc.a)
		if got != tc.want {
			t.Errorf("ActionString(%d) = %q, want %q", tc.a, got, tc.want)
		}
	}
}

// TestEvaluateLevelMapping verifies the A1 Level mapping.
func TestEvaluateLevelMapping(t *testing.T) {
	// Advise → "warn" (exit 0)
	d := Evaluate(parseOrFatal(t, "npm install foo"), pmStateHardenedPnpm("22.5.0"), DefaultConfig())
	if d.Level != "warn" {
		t.Errorf("Advise Level = %q, want %q", d.Level, "warn")
	}
	// Rewrite → "warn" (exit 0)
	hardCfg := DefaultConfig()
	hardCfg.Mode = "hard"
	d2 := Evaluate(parseOrFatal(t, "npm install foo"), pmStateHardenedPnpm("22.5.0"), hardCfg)
	if d2.Level != "warn" {
		t.Errorf("Rewrite Level = %q, want %q", d2.Level, "warn")
	}
	// Proceed → "allow"
	d3 := Evaluate(parseOrFatal(t, "npm install foo"), pmStateNone(), DefaultConfig())
	if d3.Level != "allow" {
		t.Errorf("Proceed Level = %q, want %q", d3.Level, "allow")
	}
	// Block → "block"
	blockCfg := DefaultConfig()
	blockCfg.RequireHardened = true
	d4 := Evaluate(parseOrFatal(t, "npm install foo"), pmStateNone(), blockCfg)
	if d4.Level != "block" {
		t.Errorf("Block Level = %q, want %q", d4.Level, "block")
	}
}

// TestEvaluatePreferredPnpmWhenBothHardened tests cfg.Preferred=pnpm (default)
// when both pnpm and bun are hardened.
func TestEvaluatePreferredPnpmWhenBothHardened(t *testing.T) {
	both := PMState{
		NpmInstalled:  true,
		PnpmInstalled: true,
		PnpmVersion:   "11.5.1",
		PnpmHardened:  true,
		BunInstalled:  true,
		BunVersion:    "1.3.14",
		BunScannerOK:  true,
		NodeVersion:   "22.5.0",
	}
	cfg := DefaultConfig() // Preferred = "pnpm"
	d := Evaluate(parseOrFatal(t, "npm install foo"), both, cfg)
	// Should use pnpm path (pnpm-available-soft), not bun-available-soft
	if d.Reason != ReasonPnpmAvailableSoft {
		t.Errorf("Reason = %q, want %q (pnpm preferred when both hardened)", d.Reason, ReasonPnpmAvailableSoft)
	}
}

// TestNudgeEvaluateImportsArePure enforces the pure-library contract on evaluate.go.
// Copied verbatim from TestReleaseAgeImportsArePure in internal/policy/release_age_test.go,
// with srcPath and forbidden set adjusted for nudge.
func TestNudgeEvaluateImportsArePure(t *testing.T) {
	const srcPath = "evaluate.go"
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
		"os/exec":  true,
	}

	for _, imp := range f.Imports {
		path := imp.Path.Value
		if len(path) >= 2 {
			path = path[1 : len(path)-1]
		}
		if forbidden[path] {
			t.Errorf("evaluate.go imports forbidden package %q — violates pure-library contract (T-08-06)", path)
		}
	}

	// Also verify the forbidden packages don't appear anywhere in the file as
	// indirect references (belt-and-suspenders check for blank imports).
	for pkg := range forbidden {
		if strings.Contains(string(src), `"`+pkg+`"`) {
			t.Errorf("evaluate.go references forbidden package %q (possibly blank import)", pkg)
		}
	}
}
