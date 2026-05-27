package sentry

import (
	"net"
	"testing"
	"time"
)

// buildTree converts a slice of ProcessNode values into the map[uint32]ProcessNode
// format consumed by EvaluateEvent and the helper functions.
func buildTree(nodes []ProcessNode) map[uint32]ProcessNode {
	tree := make(map[uint32]ProcessNode, len(nodes))
	for _, n := range nodes {
		tree[n.PID] = n
	}
	return tree
}

// editorTree returns a process tree with cursor(pid=1) → child(pid=100).
func editorTree() map[uint32]ProcessNode {
	return buildTree([]ProcessNode{
		{PID: 1, PPID: 0, Exe: "/usr/bin/cursor"},
		{PID: 100, PPID: 1, Exe: "/usr/bin/some-tool"},
	})
}

// emptyInventory returns an InventorySnapshot with no extensions.
func emptyInventory() InventorySnapshot {
	return InventorySnapshot{RecentExtensions: map[string]time.Time{}}
}

// freshInventory returns an InventorySnapshot with one extension installed N
// minutes ago relative to now.
func freshInventory(now time.Time, minutesAgo float64) InventorySnapshot {
	return InventorySnapshot{
		RecentExtensions: map[string]time.Time{
			"test-ext-1": now.Add(-time.Duration(minutesAgo * float64(time.Minute))),
		},
	}
}

// defaultCfg returns a zero RuleConfig (triggers applyDefaults inside EvaluateEvent).
func defaultCfg() RuleConfig { return RuleConfig{} }

// noBaseline returns a BaselineState with immediate enforcement.
func noBaseline() BaselineState { return BaselineState{DurationDays: 0} }

// hasAlert reports whether alerts contains an alert with the given ruleID.
func hasAlert(alerts []SentryAlert, ruleID string) bool {
	for _, a := range alerts {
		if a.RuleID == ruleID {
			return true
		}
	}
	return false
}

// ---- SENTRY-001 tests -------------------------------------------------------

func TestSENTRY001Fires(t *testing.T) {
	tree := editorTree()
	state := NewRuleState()
	now := time.Now().UTC()

	var allAlerts []SentryAlert

	// First sensitive-file access — should not fire yet.
	e1 := SentryEvent{
		Kind:     EventFileAccess,
		PID:      100,
		PPID:     1,
		Exe:      "/usr/bin/some-tool",
		FilePath: "/home/user/.ssh/id_rsa",
		WallTime: now,
	}
	alerts := EvaluateEvent(e1, state, tree, emptyInventory(), defaultCfg(), noBaseline(), now)
	allAlerts = append(allAlerts, alerts...)

	if hasAlert(allAlerts, "SENTRY-001") {
		t.Error("SENTRY-001 fired after only 1 sensitive path access (threshold=2)")
	}

	// Second sensitive-file access — should fire.
	e2 := SentryEvent{
		Kind:     EventFileAccess,
		PID:      100,
		PPID:     1,
		Exe:      "/usr/bin/some-tool",
		FilePath: "/home/user/.aws/credentials",
		WallTime: now.Add(5 * time.Second),
	}
	alerts = EvaluateEvent(e2, state, tree, emptyInventory(), defaultCfg(), noBaseline(), now.Add(5*time.Second))
	allAlerts = append(allAlerts, alerts...)

	if !hasAlert(allAlerts, "SENTRY-001") {
		t.Error("expected SENTRY-001 to fire after 2 sensitive path accesses")
	}
}

func TestSENTRY001NoFireNonEditor(t *testing.T) {
	// bash(pid=200, ppid=0) — not editor-descended.
	tree := buildTree([]ProcessNode{
		{PID: 200, PPID: 0, Exe: "/bin/bash"},
	})
	state := NewRuleState()
	now := time.Now().UTC()

	for _, path := range []string{"/home/user/.ssh/id_rsa", "/home/user/.aws/credentials"} {
		ev := SentryEvent{
			Kind:     EventFileAccess,
			PID:      200,
			PPID:     0,
			Exe:      "/bin/bash",
			FilePath: path,
			WallTime: now,
		}
		alerts := EvaluateEvent(ev, state, tree, emptyInventory(), defaultCfg(), noBaseline(), now)
		if len(alerts) != 0 {
			t.Errorf("expected no alerts for non-editor process, got %d", len(alerts))
		}
	}
}

func TestSENTRY001NoFireSinglePath(t *testing.T) {
	tree := editorTree()
	state := NewRuleState()
	now := time.Now().UTC()

	ev := SentryEvent{
		Kind:     EventFileAccess,
		PID:      100,
		PPID:     1,
		Exe:      "/usr/bin/some-tool",
		FilePath: "/home/user/.ssh/id_rsa",
		WallTime: now,
	}
	alerts := EvaluateEvent(ev, state, tree, emptyInventory(), defaultCfg(), noBaseline(), now)
	if hasAlert(alerts, "SENTRY-001") {
		t.Error("SENTRY-001 should not fire after only 1 sensitive path")
	}
}

// ---- SENTRY-002 tests -------------------------------------------------------

func TestSENTRY002Fires(t *testing.T) {
	tree := editorTree()
	state := NewRuleState()
	now := time.Now().UTC()

	var allAlerts []SentryAlert

	for i, exe := range []string{"/usr/bin/gh", "/usr/bin/aws"} {
		ev := SentryEvent{
			Kind:    EventProcessCreate,
			PID:     100,
			PPID:    1,
			Exe:     exe,
			WallTime: now.Add(time.Duration(i) * time.Second),
		}
		alerts := EvaluateEvent(ev, state, tree, emptyInventory(), defaultCfg(), noBaseline(), now.Add(time.Duration(i)*time.Second))
		allAlerts = append(allAlerts, alerts...)
	}

	if !hasAlert(allAlerts, "SENTRY-002") {
		t.Error("expected SENTRY-002 to fire after spawning gh and aws")
	}
}

// ---- SENTRY-003 tests -------------------------------------------------------

func TestSENTRY003Fires(t *testing.T) {
	tree := editorTree()
	state := NewRuleState()
	now := time.Now().UTC()

	ev := SentryEvent{
		Kind:    EventNetworkConnect,
		PID:     100,
		PPID:    1,
		Exe:     "/usr/bin/some-tool",
		DstAddr: net.ParseIP("1.2.3.4"),
		DstPort: 443,
		WallTime: now,
	}
	alerts := EvaluateEvent(ev, state, tree, emptyInventory(), defaultCfg(), noBaseline(), now)
	if !hasAlert(alerts, "SENTRY-003") {
		t.Error("expected SENTRY-003 to fire on first outbound connection from editor-descended process")
	}
}

// ---- SENTRY-004 tests -------------------------------------------------------

func TestSENTRY004Fires(t *testing.T) {
	tree := editorTree()
	state := NewRuleState()
	now := time.Now().UTC()

	// Extension installed 5 minutes ago — within 30-minute FreshExtWindowMin.
	inv := freshInventory(now, 5)

	var allAlerts []SentryAlert

	// Trigger SENTRY-001 with two sensitive-file accesses.
	for i, path := range []string{"/home/user/.ssh/id_rsa", "/home/user/.aws/credentials"} {
		ev := SentryEvent{
			Kind:     EventFileAccess,
			PID:      100,
			PPID:     1,
			Exe:      "/usr/bin/some-tool",
			FilePath: path,
			WallTime: now.Add(time.Duration(i) * time.Second),
		}
		t2 := now.Add(time.Duration(i) * time.Second)
		alerts := EvaluateEvent(ev, state, tree, inv, defaultCfg(), noBaseline(), t2)
		allAlerts = append(allAlerts, alerts...)
	}

	if !hasAlert(allAlerts, "SENTRY-004") {
		t.Errorf("expected SENTRY-004 to fire; got alerts: %v", allAlerts)
	}
}

func TestSENTRY004NoFireOldExtension(t *testing.T) {
	tree := editorTree()
	state := NewRuleState()
	now := time.Now().UTC()

	// Extension installed 45 minutes ago — outside 30-minute FreshExtWindowMin.
	inv := freshInventory(now, 45)

	var allAlerts []SentryAlert

	for i, path := range []string{"/home/user/.ssh/id_rsa", "/home/user/.aws/credentials"} {
		ev := SentryEvent{
			Kind:     EventFileAccess,
			PID:      100,
			PPID:     1,
			Exe:      "/usr/bin/some-tool",
			FilePath: path,
			WallTime: now.Add(time.Duration(i) * time.Second),
		}
		t2 := now.Add(time.Duration(i) * time.Second)
		alerts := EvaluateEvent(ev, state, tree, inv, defaultCfg(), noBaseline(), t2)
		allAlerts = append(allAlerts, alerts...)
	}

	if hasAlert(allAlerts, "SENTRY-004") {
		t.Error("SENTRY-004 should not fire when extension is older than FreshExtWindowMin")
	}
}

// ---- SENTRY-005 tests -------------------------------------------------------

func TestSENTRY005Fires(t *testing.T) {
	tree := editorTree()
	state := NewRuleState()
	t0 := time.Now().UTC()

	// Step 1: sensitive file read at T=0 (adds to CredAccessByPID but doesn't trigger SENTRY-001 yet).
	ev1 := SentryEvent{
		Kind:     EventFileAccess,
		PID:      100,
		PPID:     1,
		Exe:      "/usr/bin/some-tool",
		FilePath: "/home/user/.ssh/id_rsa",
		WallTime: t0,
	}
	// Extension installed at T-1min (1 minute before T=0).
	inv := InventorySnapshot{
		RecentExtensions: map[string]time.Time{
			"evil-ext": t0.Add(-1 * time.Minute),
		},
	}
	EvaluateEvent(ev1, state, tree, inv, defaultCfg(), noBaseline(), t0)

	// Step 2: network connection at T+2min — within ExfilFusionWindowMin (5 min).
	t2 := t0.Add(2 * time.Minute)
	ev2 := SentryEvent{
		Kind:    EventNetworkConnect,
		PID:     100,
		PPID:    1,
		Exe:     "/usr/bin/some-tool",
		DstAddr: net.ParseIP("5.6.7.8"),
		DstPort: 80,
		WallTime: t2,
	}
	alerts := EvaluateEvent(ev2, state, tree, inv, defaultCfg(), noBaseline(), t2)

	if !hasAlert(alerts, "SENTRY-005") {
		t.Errorf("expected SENTRY-005 to fire; got: %v", alerts)
	}
}

// ---- Baseline mode tests ----------------------------------------------------

func TestBaselineModeNoQuarantine(t *testing.T) {
	tree := editorTree()
	state := NewRuleState()
	now := time.Now().UTC()

	// Baseline started now, duration 7 days — baseline is active.
	baseline := BaselineState{
		StartedAt:    now,
		DurationDays: 7,
	}

	var allAlerts []SentryAlert
	for i, path := range []string{"/home/user/.ssh/id_rsa", "/home/user/.aws/credentials"} {
		ev := SentryEvent{
			Kind:     EventFileAccess,
			PID:      100,
			PPID:     1,
			Exe:      "/usr/bin/some-tool",
			FilePath: path,
			WallTime: now.Add(time.Duration(i) * time.Second),
		}
		t2 := now.Add(time.Duration(i) * time.Second)
		alerts := EvaluateEvent(ev, state, tree, emptyInventory(), defaultCfg(), baseline, t2)
		allAlerts = append(allAlerts, alerts...)
	}

	found := false
	for _, a := range allAlerts {
		if a.RuleID == "SENTRY-001" {
			found = true
			if !a.BaselineMode {
				t.Error("expected BaselineMode=true during baseline window")
			}
			if a.QuarantineRec {
				t.Error("expected QuarantineRec=false during baseline window")
			}
		}
	}
	if !found {
		t.Error("expected SENTRY-001 to fire (alert not found)")
	}
}

// ---- Window expiry test -----------------------------------------------------

func TestWindowExpiry(t *testing.T) {
	tree := editorTree()
	state := NewRuleState()
	t0 := time.Now().UTC()

	// Add two sensitive-path reads at T=0 — SENTRY-001 fires.
	for _, path := range []string{"/home/user/.ssh/id_rsa", "/home/user/.aws/credentials"} {
		ev := SentryEvent{
			Kind:     EventFileAccess,
			PID:      100,
			PPID:     1,
			Exe:      "/usr/bin/some-tool",
			FilePath: path,
			WallTime: t0,
		}
		EvaluateEvent(ev, state, tree, emptyInventory(), defaultCfg(), noBaseline(), t0)
	}

	// Confirm we have 2 entries after the first two events.
	if len(state.CredAccessByPID[100]) < 2 {
		t.Fatalf("expected ≥2 entries in CredAccessByPID[100] after two reads, got %d", len(state.CredAccessByPID[100]))
	}

	// Add a third read at T=90s — this is within the 60s window relative to T=90s,
	// but the T=0 entries should be expired (cutoff = 90s - 60s = 30s > 0s).
	t90 := t0.Add(90 * time.Second)
	ev3 := SentryEvent{
		Kind:     EventFileAccess,
		PID:      100,
		PPID:     1,
		Exe:      "/usr/bin/some-tool",
		FilePath: "/home/user/.gnupg/secring.gpg",
		WallTime: t90,
	}
	EvaluateEvent(ev3, state, tree, emptyInventory(), defaultCfg(), noBaseline(), t90)

	// The T=0 entries (cutoff = t90 - 60s = t0+30s) should have been expired.
	// Only the T=90s entry should remain.
	entries := state.CredAccessByPID[100]
	for _, e := range entries {
		if e.SeenAt.Equal(t0) {
			t.Error("expected T=0 entries to be expired from CredAccessByPID[100]")
		}
	}

	// The T=90s entry should still be present.
	found90 := false
	for _, e := range entries {
		if e.SeenAt.Equal(t90) {
			found90 = true
			break
		}
	}
	if !found90 {
		t.Error("expected T=90s entry to remain in CredAccessByPID[100]")
	}
}
