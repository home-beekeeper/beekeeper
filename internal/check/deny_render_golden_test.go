package check

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bantuson/beekeeper/internal/policy"
)

// updateGolden regenerates the committed deny-contract golden files:
//
//	go test -run TestRenderDenyGolden -update ./internal/check/
var updateGolden = flag.Bool("update", false, "regenerate golden deny-contract files")

// canonicalDeny renders a DenyOutput to a stable, byte-comparable layout. A
// fixed reason string keeps the goldens deterministic.
func canonicalDeny(out DenyOutput) []byte {
	return []byte(fmt.Sprintf("exit=%d\nstdout=%s\nstderr=%s\n", out.ExitCode, out.Stdout, out.Stderr))
}

// TestRenderDenyGolden is the byte-exact VAL-02 deny-contract gate: for every
// HarnessID const (15) plus the unknown/empty fail-closed default (16 cases),
// RenderDeny's canonical output must match the committed
// testdata/deny/<harness>.golden file. A drift in any harness's exit code or JSON
// shape fails the build. Regenerate with -update.
func TestRenderDenyGolden(t *testing.T) {
	const reason = "credential read blocked by beekeeper policy" // FIXED canary → deterministic goldens

	harnesses := []HarnessID{
		HarnessClaudeCode, HarnessCursor, HarnessCodex, HarnessAugment, HarnessCodeBuddy,
		HarnessQwen, HarnessCopilot, HarnessGemini, HarnessAntigravity, HarnessWindsurf,
		HarnessCline, HarnessHermes, HarnessOpenCode, HarnessKilo, HarnessTrae,
	}
	type row struct {
		name string
		h    HarnessID
	}
	rows := make([]row, 0, len(harnesses)+1)
	for _, h := range harnesses {
		rows = append(rows, row{name: string(h), h: h})
	}
	rows = append(rows, row{name: "unknown", h: HarnessID("")}) // fail-closed default

	dir := filepath.Join("testdata", "deny")
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			out := RenderDeny(r.h, policy.Decision{Allow: false, Level: "block", Reason: reason})
			got := canonicalDeny(out)
			golden := filepath.Join(dir, r.name+".golden")

			if *updateGolden {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}

			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("reading golden %s (run `go test -run TestRenderDenyGolden -update ./internal/check/` to generate): %v", golden, err)
			}
			if string(got) != string(want) {
				t.Errorf("deny-contract drift for %q:\n got: %q\nwant: %q", r.name, got, want)
			}
		})
	}
}

// TestRenderDenyHonestySeams asserts the two regression-critical honesty seams
// as explicit, named cases so a fail-open/UNGUARDED regression fails loudly even
// if a golden were carelessly regenerated.
func TestRenderDenyHonestySeams(t *testing.T) {
	const reason = "x"

	t.Run("hermes is the exit-0 fail-open seam", func(t *testing.T) {
		out := RenderDeny(HarnessHermes, policy.Decision{Allow: false, Level: "block", Reason: reason})
		if out.ExitCode != 0 {
			t.Errorf("hermes ExitCode = %d, want 0 (Hermes ignores exit codes; the block is JSON-only)", out.ExitCode)
		}
		if !strings.Contains(string(out.Stdout), `"action":"block"`) {
			t.Errorf("hermes stdout = %q, want it to contain \"action\":\"block\"", out.Stdout)
		}
	})

	for _, h := range []HarnessID{HarnessKilo, HarnessTrae} {
		t.Run(string(h)+" is UNGUARDED (exit 2, no stdout JSON)", func(t *testing.T) {
			out := RenderDeny(h, policy.Decision{Allow: false, Level: "block", Reason: reason})
			if out.ExitCode != 2 {
				t.Errorf("%s ExitCode = %d, want 2", h, out.ExitCode)
			}
			if len(out.Stdout) != 0 {
				t.Errorf("%s Stdout = %q, want empty (native tools UNGUARDED; no stdout deny form)", h, out.Stdout)
			}
		})
	}

	t.Run("unknown harness fails closed (exit 2, no stdout)", func(t *testing.T) {
		out := RenderDeny(HarnessID("bogus"), policy.Decision{Allow: false, Level: "block", Reason: reason})
		if out.ExitCode != 2 {
			t.Errorf("unknown harness ExitCode = %d, want 2 (fail closed, never silently allow)", out.ExitCode)
		}
		if len(out.Stdout) != 0 {
			t.Errorf("unknown harness Stdout = %q, want empty", out.Stdout)
		}
	})
}
