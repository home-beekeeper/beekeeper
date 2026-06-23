package posture

import (
	"bytes"
	"strings"
	"testing"
)

// TestBuildComparison_NpmThreeGaps verifies that a detected npm (no install
// hardening of its own) produces a row covering all three Beekeeper gaps with the
// expected named gaps and the "covering 3 gaps" count.
func TestBuildComparison_NpmThreeGaps(t *testing.T) {
	state := PMState{NpmInstalled: true, NpmVersion: "11.0.0"}
	c := BuildComparison(state, DefaultEnforced(), "")

	if len(c.Managers) != 1 {
		t.Fatalf("want 1 manager, got %d", len(c.Managers))
	}
	row := c.Managers[0]
	if row.Manager != "npm" {
		t.Fatalf("want npm, got %q", row.Manager)
	}
	if row.GapCount() != 3 {
		t.Fatalf("want 3 gaps, got %d: %v", row.GapCount(), row.Gaps)
	}
	wantGaps := []string{"scripts warned", "release-age 24h", "git deps flagged"}
	for _, g := range wantGaps {
		if !containsStr(row.Gaps, g) {
			t.Errorf("npm gaps missing %q: %v", g, row.Gaps)
		}
	}
	if row.Aligned {
		t.Error("npm must not be aligned")
	}

	summary := row.Summary()
	if !strings.Contains(summary, "Covering 3 gaps") {
		t.Errorf("npm summary missing gap count: %q", summary)
	}
	if !strings.Contains(summary, "your npm version does not") {
		t.Errorf("npm summary missing trailing phrase: %q", summary)
	}
	if c.TotalGaps() != 3 {
		t.Errorf("want total 3 gaps, got %d", c.TotalGaps())
	}
}

// TestBuildComparison_PnpmAligned verifies a hardened pnpm is reported as aligned
// with no gap.
func TestBuildComparison_PnpmAligned(t *testing.T) {
	state := PMState{PnpmInstalled: true, PnpmVersion: "11.1.0", PnpmHardened: true}
	c := BuildComparison(state, DefaultEnforced(), "")

	if len(c.Managers) != 1 {
		t.Fatalf("want 1 manager, got %d", len(c.Managers))
	}
	row := c.Managers[0]
	if row.Manager != "pnpm" {
		t.Fatalf("want pnpm, got %q", row.Manager)
	}
	if !row.Aligned {
		t.Error("hardened pnpm must be aligned")
	}
	if row.GapCount() != 0 {
		t.Errorf("hardened pnpm must have 0 gaps, got %d: %v", row.GapCount(), row.Gaps)
	}
	summary := row.Summary()
	if !strings.Contains(summary, "aligned, no gap") {
		t.Errorf("pnpm summary should say aligned: %q", summary)
	}
	if !strings.Contains(summary, "minimumReleaseAge honored") {
		t.Errorf("pnpm summary should describe its hardening: %q", summary)
	}
}

// TestBuildComparison_PnpmWeakness verifies a pnpm with an explicit hardening
// downgrade is no longer aligned and surfaces the weakness note.
func TestBuildComparison_PnpmWeakness(t *testing.T) {
	state := PMState{PnpmInstalled: true, PnpmVersion: "11.1.0", PnpmHardened: true}
	note := "Hardening downgraded in pnpm-workspace.yaml."
	c := BuildComparison(state, DefaultEnforced(), note)

	row := c.Managers[0]
	if row.Aligned {
		t.Error("downgraded pnpm must not be aligned")
	}
	if row.GapCount() == 0 {
		t.Error("downgraded pnpm must report at least one gap")
	}
	if !containsLine(row.SelfPosture, note) {
		t.Errorf("downgraded pnpm should surface the weakness note: %v", row.SelfPosture)
	}
}

// TestBuildComparison_PnpmBelowFloor verifies a pnpm below the version floor
// (PnpmHardened=false) is treated like an unhardened manager covering all gaps.
func TestBuildComparison_PnpmBelowFloor(t *testing.T) {
	state := PMState{PnpmInstalled: true, PnpmVersion: "9.0.0", PnpmHardened: false}
	c := BuildComparison(state, DefaultEnforced(), "")

	row := c.Managers[0]
	if row.Aligned {
		t.Error("below-floor pnpm must not be aligned")
	}
	if row.GapCount() != 3 {
		t.Errorf("below-floor pnpm should cover 3 gaps, got %d", row.GapCount())
	}
}

// TestBuildComparison_BunWithScanner verifies a bun with the Socket scanner still
// leaves release-age and git-dep gaps to Beekeeper.
func TestBuildComparison_BunWithScanner(t *testing.T) {
	state := PMState{BunInstalled: true, BunVersion: "1.3.0", BunScannerOK: true}
	c := BuildComparison(state, DefaultEnforced(), "")

	row := c.Managers[0]
	if row.Manager != "bun" {
		t.Fatalf("want bun, got %q", row.Manager)
	}
	if row.GapCount() != 2 {
		t.Errorf("bun+scanner should cover 2 gaps, got %d: %v", row.GapCount(), row.Gaps)
	}
	if containsStr(row.Gaps, "scripts warned") {
		t.Error("bun+scanner should not list scripts warned as a gap")
	}
}

// TestBuildComparison_AbsentManagerOmitted verifies an undetected manager
// produces no row at all.
func TestBuildComparison_AbsentManagerOmitted(t *testing.T) {
	// Only npm installed; pnpm and bun absent.
	state := PMState{NpmInstalled: true, NpmVersion: "11.0.0"}
	c := BuildComparison(state, DefaultEnforced(), "")
	for _, m := range c.Managers {
		if m.Manager == "pnpm" || m.Manager == "bun" {
			t.Errorf("absent manager %q must be omitted", m.Manager)
		}
	}

	// Nothing installed at all → zero rows.
	empty := BuildComparison(PMState{}, DefaultEnforced(), "")
	if len(empty.Managers) != 0 {
		t.Errorf("no managers installed must yield 0 rows, got %d", len(empty.Managers))
	}
}

// TestBuildComparison_StableOrder verifies managers render in npm, pnpm, bun
// order regardless of which are present.
func TestBuildComparison_StableOrder(t *testing.T) {
	state := PMState{
		BunInstalled:  true,
		BunVersion:    "1.3.0",
		NpmInstalled:  true,
		NpmVersion:    "11.0.0",
		PnpmInstalled: true,
		PnpmVersion:   "11.1.0",
		PnpmHardened:  true,
	}
	c := BuildComparison(state, DefaultEnforced(), "")
	want := []string{"npm", "pnpm", "bun"}
	if len(c.Managers) != len(want) {
		t.Fatalf("want %d managers, got %d", len(want), len(c.Managers))
	}
	for i, name := range want {
		if c.Managers[i].Manager != name {
			t.Errorf("position %d: want %q, got %q", i, name, c.Managers[i].Manager)
		}
	}
}

// TestRender_BoundaryPresent verifies the rendered output contains the canonical
// boundary statement (IPBND-01) and the enforced posture.
func TestRender_BoundaryPresent(t *testing.T) {
	state := PMState{NpmInstalled: true, NpmVersion: "11.0.0"}
	c := BuildComparison(state, DefaultEnforced(), "")

	var short bytes.Buffer
	c.Render(&short, false)
	if !strings.Contains(short.String(), BoundaryShort) {
		t.Error("short render must contain BoundaryShort")
	}

	var full bytes.Buffer
	c.Render(&full, true)
	if !strings.Contains(full.String(), BoundaryStatement) {
		t.Error("full render must contain BoundaryStatement")
	}
	if !strings.Contains(full.String(), "release-age 24h") {
		t.Error("render must contain the enforced release-age line")
	}
}

// TestRender_NoEmDash guards the project style rule: the rendered view must
// contain no em dash characters.
func TestRender_NoEmDash(t *testing.T) {
	state := PMState{
		NpmInstalled:  true,
		NpmVersion:    "11.0.0",
		PnpmInstalled: true,
		PnpmVersion:   "11.1.0",
		PnpmHardened:  true,
		BunInstalled:  true,
		BunVersion:    "1.3.0",
	}
	c := BuildComparison(state, DefaultEnforced(), "Hardening downgraded in pnpm-workspace.yaml.")
	var buf bytes.Buffer
	c.Render(&buf, true)
	if strings.ContainsRune(buf.String(), '—') {
		t.Error("rendered posture view must not contain an em dash")
	}
}

func containsStr(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func containsLine(haystack []string, needle string) bool {
	return containsStr(haystack, needle)
}
