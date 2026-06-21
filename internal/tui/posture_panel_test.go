package tui

import (
	"context"
	"strings"
	"testing"

	posture "github.com/home-beekeeper/beekeeper/internal/posture"
)

// newPosturePanelForTest builds a PosturePanel directly from a comparison model,
// bypassing live detection so the test never depends on a real npm/pnpm/bun.
func newPosturePanelForTest(state posture.PMState, weakness string) *PosturePanel {
	p := &PosturePanel{
		comparison: posture.BuildComparison(state, posture.DefaultEnforced(), weakness),
	}
	p.bodyCache = p.buildBody()
	return p
}

func TestPosturePanel_NpmAndPnpm(t *testing.T) {
	p := newPosturePanelForTest(posture.PMState{
		NpmInstalled:  true,
		NpmVersion:    "11.0.0",
		PnpmInstalled: true,
		PnpmVersion:   "11.1.0",
		PnpmHardened:  true,
	}, "")

	body := p.Body(100, 30)
	if !strings.Contains(body, "npm") {
		t.Error("body must mention npm")
	}
	if !strings.Contains(body, "pnpm") {
		t.Error("body must mention pnpm")
	}
	if !strings.Contains(body, "aligned, no gap") {
		t.Error("hardened pnpm must show aligned in the panel")
	}
	// The canonical boundary statement must be visible (IPBND-01).
	if !strings.Contains(body, posture.BoundaryShort) {
		t.Error("panel body must contain BoundaryShort")
	}
	// No em dash anywhere.
	if strings.ContainsRune(body, '—') {
		t.Error("panel body must not contain an em dash")
	}

	if !strings.Contains(p.Count(), "managers") {
		t.Errorf("count should mention managers: %q", p.Count())
	}
	if p.Title() != "Install posture" {
		t.Errorf("unexpected title %q", p.Title())
	}
	if p.Critical() {
		t.Error("posture panel is never critical")
	}
	if !strings.Contains(p.Footer(), "read-only") {
		t.Errorf("footer must state read-only: %q", p.Footer())
	}
}

func TestPosturePanel_NothingDetected(t *testing.T) {
	p := newPosturePanelForTest(posture.PMState{}, "")
	body := p.Body(100, 30)
	if !strings.Contains(body, "No package managers detected") {
		t.Errorf("empty panel should say nothing detected:\n%s", body)
	}
	// Boundary still present even with no managers.
	if !strings.Contains(body, posture.BoundaryShort) {
		t.Error("empty panel must still show the boundary statement")
	}
}

// TestPosturePanel_UpdateIsReadOnly verifies Update never mutates and returns the
// same panel (the panel handles no state-changing keys).
func TestPosturePanel_UpdateIsReadOnly(t *testing.T) {
	p := newPosturePanelForTest(posture.PMState{NpmInstalled: true, NpmVersion: "11.0.0"}, "")
	before := p.Body(100, 30)
	got, cmd := p.Update(nil)
	if cmd != nil {
		t.Error("read-only panel Update must return a nil command")
	}
	if got.Body(100, 30) != before {
		t.Error("read-only panel Update must not change the body")
	}
}

// TestNewPosturePanel_Seam exercises the construction seam path to keep the
// detection injectable for the dashboard wiring.
func TestNewPosturePanel_Seam(t *testing.T) {
	restoreDetect := postureDetectFn
	restoreWeak := postureWeaknessFn
	t.Cleanup(func() {
		postureDetectFn = restoreDetect
		postureWeaknessFn = restoreWeak
	})
	postureDetectFn = func(_ context.Context, _ posture.Config) posture.PMState {
		return posture.PMState{BunInstalled: true, BunVersion: "1.3.0", BunScannerOK: false}
	}
	postureWeaknessFn = func() string { return "" }

	p := NewPosturePanel()
	body := p.Body(100, 30)
	if !strings.Contains(body, "bun") {
		t.Errorf("seam-injected bun should appear in panel:\n%s", body)
	}
}
