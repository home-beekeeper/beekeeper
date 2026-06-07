package nudge

import (
	"strings"
	"testing"

	"github.com/bantuson/beekeeper/internal/pkgparse"
)

// TestEvaluateBunBranches drives evaluateBun directly (white-box) to cover every
// branch of the bun decision path, mirroring the existing evaluatePnpm coverage.
// evaluateBun is only ever reached when bun was actually detected as ready
// (state.BunInstalled + meets floor), so each case constructs a hardened-bun
// PMState via pmStateHardenedBun and toggles the scanner / mode / package args.
func TestEvaluateBunBranches(t *testing.T) {
	// Base config: bun preferred so the bun branch is selected first, soft mode.
	// (evaluateBun is called directly here, so Preferred selection is moot for the
	// direct calls, but keeping it bun-preferred documents intent.)
	baseCfg := func() Config {
		c := DefaultConfig()
		c.Preferred = "bun"
		return c
	}

	tests := []struct {
		name       string
		cmd        pkgparse.ParsedCommand
		state      PMState
		cfg        Config
		wantAct    Action
		wantReason string
		wantLevel  string
		wantRewr   string // expected Rewritten (checked when non-empty)
	}{
		{
			// §10-5: CheckSocketScanner on AND scanner absent → Advise install-scanner.
			name:       "scanner-check on, scanner absent → advise no-scanner",
			cmd:        parseOrFatal(t, "npm install foo"),
			state:      pmStateHardenedBun(false), // BunScannerOK=false
			cfg:        baseCfg(),                 // CheckSocketScanner=true (default)
			wantAct:    Advise,
			wantReason: ReasonBunAvailableNoScanner,
			wantLevel:  "warn",
		},
		{
			// Hard mode (scanner present so the no-scanner branch is skipped) → Rewrite.
			name: "hard mode → rewrite to bun add",
			cmd:  parseOrFatal(t, "npm install foo"),
			state: pmStateHardenedBun(true), // scanner present so we fall past §10-5
			cfg: func() Config {
				c := baseCfg()
				c.Mode = "hard"
				return c
			}(),
			wantAct:    Rewrite,
			wantReason: ReasonPnpmHardRewrite, // bun hard reuses the pnpm-hard reason (see evaluateBun)
			wantLevel:  "warn",
			wantRewr:   "bun add foo",
		},
		{
			// Soft mode + a package present → Advise bun-available-soft.
			name:       "soft mode with package → advise bun-available-soft",
			cmd:        parseOrFatal(t, "npm install foo"),
			state:      pmStateHardenedBun(true),
			cfg:        baseCfg(), // soft (default)
			wantAct:    Advise,
			wantReason: ReasonBunAvailableSoft,
			wantLevel:  "warn",
		},
		{
			// Soft mode + no-arg install (cmd.Package=="") → softer no-arg reason.
			name:       "soft mode no-arg install → advise no-arg-install-soft",
			cmd:        parseOrFatal(t, "npm install"),
			state:      pmStateHardenedBun(true),
			cfg:        baseCfg(),
			wantAct:    Advise,
			wantReason: ReasonNoArgInstallSoft,
			wantLevel:  "warn",
		},
		{
			// CheckSocketScanner OFF + scanner absent must NOT hit the no-scanner
			// branch — it should fall through to the soft advisory. This proves the
			// branch guard is the conjunction (CheckSocketScanner && !BunScannerOK),
			// not either condition alone.
			name: "scanner-check OFF, scanner absent → soft advise (no-scanner branch skipped)",
			cmd:  parseOrFatal(t, "npm install foo"),
			state: pmStateHardenedBun(false), // scanner absent
			cfg: func() Config {
				c := baseCfg()
				c.CheckSocketScanner = false
				return c
			}(),
			wantAct:    Advise,
			wantReason: ReasonBunAvailableSoft,
			wantLevel:  "warn",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := evaluateBun(tc.cmd, tc.state, tc.cfg)

			if d.Action != tc.wantAct {
				t.Errorf("Action = %v (%s), want %v (%s)", d.Action, ActionString(d.Action), tc.wantAct, ActionString(tc.wantAct))
			}
			if d.Reason != tc.wantReason {
				t.Errorf("Reason = %q, want %q", d.Reason, tc.wantReason)
			}
			if d.Level != tc.wantLevel {
				t.Errorf("Level = %q, want %q", d.Level, tc.wantLevel)
			}
			if tc.wantRewr != "" && d.Rewritten != tc.wantRewr {
				t.Errorf("Rewritten = %q, want %q", d.Rewritten, tc.wantRewr)
			}
			// A non-Rewrite decision must carry no rewritten command.
			if tc.wantAct != Rewrite && d.Rewritten != "" {
				t.Errorf("Rewritten = %q, want empty for non-Rewrite decision", d.Rewritten)
			}
			if !IsValidReason(d.Reason) {
				t.Errorf("Reason %q is not a valid closed-enum reason code", d.Reason)
			}
			if d.Original != tc.cmd.Raw {
				t.Errorf("Original = %q, want cmd.Raw %q", d.Original, tc.cmd.Raw)
			}
		})
	}
}

// TestEvaluateBunViaEvaluate is an end-to-end guard: Evaluate must route into the
// bun branch when only bun is hardened, so the direct-call coverage above maps to
// real reachable behavior (not a dead helper). pnpm absent + bun ready + soft →
// bun-available-soft.
func TestEvaluateBunViaEvaluate(t *testing.T) {
	cfg := DefaultConfig() // soft, scanner check on
	state := pmStateHardenedBun(true)
	d := Evaluate(parseOrFatal(t, "npm install foo"), state, cfg)
	if d.Action != Advise {
		t.Fatalf("Action = %v, want Advise (bun ready, soft)", d.Action)
	}
	if d.Reason != ReasonBunAvailableSoft {
		t.Errorf("Reason = %q, want %q (bun branch reached via Evaluate)", d.Reason, ReasonBunAvailableSoft)
	}
}

// TestRewriteToPnpmExported covers the EXPORTED RewriteToPnpm wrapper (called
// across the package boundary by the check deny-message builder). It must produce
// the same output as the unexported rewriteToPnpm and preserve the package token.
func TestRewriteToPnpmExported(t *testing.T) {
	tests := []struct {
		cmd  string
		want string
	}{
		{"npm install lodash", "pnpm add lodash"},
		{"npm install lodash@4.17.21", "pnpm add lodash@4.17.21"},
		{"npm install", "pnpm install"}, // no-arg form
		{"npx create-app", "pnpm dlx create-app"},
	}
	for _, tc := range tests {
		parsed := parseOrFatal(t, tc.cmd)
		got := RewriteToPnpm(parsed)
		if got != tc.want {
			t.Errorf("RewriteToPnpm(Parse(%q)) = %q, want %q", tc.cmd, got, tc.want)
		}
		// Sanity: exported wrapper must agree with the unexported implementation.
		if got != rewriteToPnpm(parsed) {
			t.Errorf("RewriteToPnpm != rewriteToPnpm for %q", tc.cmd)
		}
		if !strings.HasPrefix(got, "pnpm") {
			t.Errorf("RewriteToPnpm(%q) = %q, want a pnpm command", tc.cmd, got)
		}
	}
}

// TestRewriteToBunExported covers the EXPORTED RewriteToBun wrapper (see above).
func TestRewriteToBunExported(t *testing.T) {
	tests := []struct {
		cmd  string
		want string
	}{
		{"npm install lodash", "bun add lodash"},
		{"npm install lodash@4.17.21", "bun add lodash@4.17.21"},
		{"npm install", "bun install"}, // no-arg form
		{"npx create-app", "bun x create-app"},
	}
	for _, tc := range tests {
		parsed := parseOrFatal(t, tc.cmd)
		got := RewriteToBun(parsed)
		if got != tc.want {
			t.Errorf("RewriteToBun(Parse(%q)) = %q, want %q", tc.cmd, got, tc.want)
		}
		if got != rewriteToBun(parsed) {
			t.Errorf("RewriteToBun != rewriteToBun for %q", tc.cmd)
		}
		if !strings.HasPrefix(got, "bun") {
			t.Errorf("RewriteToBun(%q) = %q, want a bun command", tc.cmd, got)
		}
	}
}

// TestRewriteExportedPreservesPackage asserts the package name survives the
// exported rewrite path — the whole point of computing the equivalent command for
// the block deny message is that the agent sees the SAME package it asked for.
func TestRewriteExportedPreservesPackage(t *testing.T) {
	parsed := parseOrFatal(t, "npm install @scope/evil-pkg@1.2.3")
	pnpmCmd := RewriteToPnpm(parsed)
	bunCmd := RewriteToBun(parsed)
	if !strings.Contains(pnpmCmd, "@scope/evil-pkg@1.2.3") {
		t.Errorf("RewriteToPnpm dropped the package: %q", pnpmCmd)
	}
	if !strings.Contains(bunCmd, "@scope/evil-pkg@1.2.3") {
		t.Errorf("RewriteToBun dropped the package: %q", bunCmd)
	}
}
