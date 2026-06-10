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

// ---- isSensitivePath Windows backslash regression tests --------------------

// TestIsSensitivePathWindows verifies that isSensitivePath recognises native
// Windows backslash paths produced by the ETW kernel logger, as well as
// confirming that Unix forward-slash paths still match and that non-sensitive
// Windows paths do not.
//
// This is an all-OS regression test — it is NOT build-tagged because the fix
// (filepath.ToSlash normalisation) must be correct on every platform.
func TestIsSensitivePathWindows(t *testing.T) {
	cases := []struct {
		name     string
		path     string
		wantTrue bool
	}{
		{
			name:     "Windows backslash AWS credentials path",
			path:     `C:\Users\runner\.aws\credentials`,
			wantTrue: true,
		},
		{
			name:     "Windows backslash SSH private key path",
			path:     `C:\Users\runner\.ssh\id_rsa`,
			wantTrue: true,
		},
		{
			name:     "Unix forward-slash AWS credentials path (no regression)",
			path:     "/home/user/.aws/credentials",
			wantTrue: true,
		},
		{
			name:     "Windows backslash non-sensitive path",
			path:     `C:\Users\runner\Documents\notes.txt`,
			wantTrue: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isSensitivePath(tc.path)
			if got != tc.wantTrue {
				t.Errorf("isSensitivePath(%q) = %v; want %v", tc.path, got, tc.wantTrue)
			}
		})
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

// ---- Phase 20 (SENT) — watchlist / SENTRY-006 / SENTRY-007 / external dest ----

// agentTree returns a bare-terminal agent process tree: shell(pid=1, no editor)
// → claude(pid=50) → child(pid=200). The leaf is agent-descended but NOT
// editor-descended.
func agentTree() map[uint32]ProcessNode {
	return buildTree([]ProcessNode{
		{PID: 1, PPID: 0, Exe: "/bin/bash"},
		{PID: 50, PPID: 1, Exe: "/usr/local/bin/claude"},
		{PID: 200, PPID: 50, Exe: "/usr/bin/some-tool"},
	})
}

// TestSENTRY001FiresCloudWatchlist proves the Phase-20 watchlist expansion: a
// 2-cloud-cred read (.aws + .config/gcloud) by a monitored descendant fires
// SENTRY-001.
func TestSENTRY001FiresCloudWatchlist(t *testing.T) {
	now := time.Now()
	state := NewRuleState()
	tree := editorTree()

	mk := func(path string) SentryEvent {
		return SentryEvent{Kind: EventFileAccess, PID: 100, PPID: 1, Exe: "/usr/bin/some-tool", FilePath: path}
	}
	EvaluateEvent(mk("/home/u/.aws/credentials"), state, tree, emptyInventory(), defaultCfg(), noBaseline(), now)
	alerts := EvaluateEvent(mk("/home/u/.config/gcloud/credentials.db"), state, tree, emptyInventory(), defaultCfg(), noBaseline(), now.Add(3*time.Second))
	if !hasAlert(alerts, "SENTRY-001") {
		t.Error("expected SENTRY-001 on .aws + .config/gcloud (watchlist expansion)")
	}
}

// TestSENTRY006AgentBareTerminal proves a bare-terminal agent reading 2
// sensitive paths in the window fires SENTRY-006.
func TestSENTRY006AgentBareTerminal(t *testing.T) {
	now := time.Now()
	state := NewRuleState()
	tree := agentTree()

	mk := func(path string) SentryEvent {
		return SentryEvent{Kind: EventFileAccess, PID: 200, PPID: 50, Exe: "/usr/bin/some-tool", FilePath: path}
	}
	EvaluateEvent(mk("/home/u/.aws/credentials"), state, tree, emptyInventory(), defaultCfg(), noBaseline(), now)
	alerts := EvaluateEvent(mk("/home/u/.ssh/id_rsa"), state, tree, emptyInventory(), defaultCfg(), noBaseline(), now.Add(2*time.Second))
	if !hasAlert(alerts, "SENTRY-006") {
		t.Error("expected SENTRY-006 for a bare-terminal agent reading 2 sensitive paths")
	}
}

// TestSENTRY006NoDoubleFireIntegratedTerminal proves an editor integrated
// terminal produces exactly one alert (SENTRY-001) and NOT SENTRY-006.
func TestSENTRY006NoDoubleFireIntegratedTerminal(t *testing.T) {
	now := time.Now()
	state := NewRuleState()
	// cursor(editor) → cursor-agent(agent) → leaf: BOTH editor- and agent-descended.
	tree := buildTree([]ProcessNode{
		{PID: 1, PPID: 0, Exe: "/usr/bin/cursor"},
		{PID: 50, PPID: 1, Exe: "/usr/bin/cursor-agent"},
		{PID: 200, PPID: 50, Exe: "/usr/bin/some-tool"},
	})
	mk := func(path string) SentryEvent {
		return SentryEvent{Kind: EventFileAccess, PID: 200, PPID: 50, Exe: "/usr/bin/some-tool", FilePath: path}
	}
	EvaluateEvent(mk("/home/u/.aws/credentials"), state, tree, emptyInventory(), defaultCfg(), noBaseline(), now)
	alerts := EvaluateEvent(mk("/home/u/.ssh/id_rsa"), state, tree, emptyInventory(), defaultCfg(), noBaseline(), now.Add(2*time.Second))
	if !hasAlert(alerts, "SENTRY-001") {
		t.Error("expected SENTRY-001 on the integrated terminal")
	}
	if hasAlert(alerts, "SENTRY-006") {
		t.Error("SENTRY-006 must NOT fire on an editor integrated terminal (no double-fire, D-T3-gate)")
	}
}

// TestIsExternalDest covers the private/external classification incl. the
// IPv4-mapped IPv6 case.
func TestIsExternalDest(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"8.8.8.8", true},
		{"1.1.1.1", true},
		{"127.0.0.1", false},
		{"::1", false},
		{"10.0.0.1", false},
		{"172.16.0.1", false},
		{"192.168.1.1", false},
		{"169.254.1.1", false},
		{"100.64.0.1", false}, // CGNAT
		{"::ffff:10.0.0.1", false}, // IPv4-mapped private
		{"fc00::1", false},         // ULA
	}
	for _, c := range cases {
		got := isExternalDest(net.ParseIP(c.ip))
		if got != c.want {
			t.Errorf("isExternalDest(%s) = %v, want %v", c.ip, got, c.want)
		}
	}
}

// TestSENTRY007Fires proves the generalized exfil fusion: cred read + external
// outbound fires SENTRY-007; loopback/RFC1918 outbound does not.
func TestSENTRY007Fires(t *testing.T) {
	now := time.Now()
	tree := editorTree()

	run := func(dst string) []SentryAlert {
		state := NewRuleState()
		// 1 recent cred read (no SENTRY-001 — threshold 2).
		EvaluateEvent(SentryEvent{Kind: EventFileAccess, PID: 100, PPID: 1, Exe: "/usr/bin/some-tool", FilePath: "/home/u/.aws/credentials"},
			state, tree, emptyInventory(), defaultCfg(), noBaseline(), now)
		// Outbound connection to dst.
		return EvaluateEvent(SentryEvent{Kind: EventNetworkConnect, PID: 100, PPID: 1, Exe: "/usr/bin/some-tool", DstAddr: net.ParseIP(dst)},
			state, tree, emptyInventory(), defaultCfg(), noBaseline(), now.Add(10*time.Second))
	}

	if !hasAlert(run("8.8.8.8"), "SENTRY-007") {
		t.Error("expected SENTRY-007 on cred-read + external outbound")
	}
	if hasAlert(run("127.0.0.1"), "SENTRY-007") {
		t.Error("SENTRY-007 must NOT fire on a loopback outbound")
	}
	if hasAlert(run("10.0.0.1"), "SENTRY-007") {
		t.Error("SENTRY-007 must NOT fire on an RFC1918 outbound")
	}
	if hasAlert(run("::ffff:10.0.0.1"), "SENTRY-007") {
		t.Error("SENTRY-007 must NOT fire on an IPv4-mapped private outbound")
	}
}

// TestSENTRY007WarnFirstInBaseline proves SENTRY-007 is warn-first (no
// quarantine recommendation, BaselineMode set) while a baseline window is active.
func TestSENTRY007WarnFirstInBaseline(t *testing.T) {
	now := time.Now()
	tree := editorTree()
	state := NewRuleState()
	baseline := BaselineState{DurationDays: 7, StartedAt: now.Add(-1 * 24 * time.Hour)}

	EvaluateEvent(SentryEvent{Kind: EventFileAccess, PID: 100, PPID: 1, Exe: "/usr/bin/some-tool", FilePath: "/home/u/.aws/credentials"},
		state, tree, emptyInventory(), defaultCfg(), baseline, now)
	alerts := EvaluateEvent(SentryEvent{Kind: EventNetworkConnect, PID: 100, PPID: 1, Exe: "/usr/bin/some-tool", DstAddr: net.ParseIP("8.8.8.8")},
		state, tree, emptyInventory(), defaultCfg(), baseline, now.Add(10*time.Second))

	for _, a := range alerts {
		if a.RuleID == "SENTRY-007" {
			if !a.BaselineMode {
				t.Error("SENTRY-007 BaselineMode = false during active baseline, want true (warn-first)")
			}
			if a.QuarantineRec {
				t.Error("SENTRY-007 QuarantineRec = true during baseline, want false (warn-first)")
			}
			return
		}
	}
	t.Error("expected a SENTRY-007 alert")
}

// ---- Phase 20 (SENT-05) — SENTRY-008 persistence-write ----

// TestSENTRY008Fires proves a monitored-descendant write to a persistence path
// fires SENTRY-008.
func TestSENTRY008Fires(t *testing.T) {
	now := time.Now()
	state := NewRuleState()
	tree := editorTree()
	ev := SentryEvent{Kind: EventFileWrite, PID: 100, PPID: 1, Exe: "/usr/bin/some-tool", FilePath: "/home/u/.claude/settings.json"}
	alerts := EvaluateEvent(ev, state, tree, emptyInventory(), defaultCfg(), noBaseline(), now)
	if !hasAlert(alerts, "SENTRY-008") {
		t.Error("expected SENTRY-008 on a monitored write to ~/.claude/settings.json")
	}
}

// TestSENTRY008NoFireBenign proves a benign-path write does not fire.
func TestSENTRY008NoFireBenign(t *testing.T) {
	now := time.Now()
	state := NewRuleState()
	tree := editorTree()
	ev := SentryEvent{Kind: EventFileWrite, PID: 100, PPID: 1, Exe: "/usr/bin/some-tool", FilePath: "/home/u/project/README.md"}
	if alerts := EvaluateEvent(ev, state, tree, emptyInventory(), defaultCfg(), noBaseline(), now); hasAlert(alerts, "SENTRY-008") {
		t.Error("SENTRY-008 must NOT fire on a benign-path write")
	}
}

// TestSENTRY008NoFireNonMonitored proves a non-monitored writer does not fire.
func TestSENTRY008NoFireNonMonitored(t *testing.T) {
	now := time.Now()
	state := NewRuleState()
	tree := buildTree([]ProcessNode{{PID: 300, PPID: 0, Exe: "/bin/bash"}})
	ev := SentryEvent{Kind: EventFileWrite, PID: 300, PPID: 0, Exe: "/usr/bin/some-tool", FilePath: "/home/u/.claude/settings.json"}
	if alerts := EvaluateEvent(ev, state, tree, emptyInventory(), defaultCfg(), noBaseline(), now); hasAlert(alerts, "SENTRY-008") {
		t.Error("SENTRY-008 must NOT fire for a non-monitored (no editor/agent ancestor) writer")
	}
}

// TestSENTRY008DedupPerPath proves a repeated write to the same path does not
// re-alert (per-path-per-session dedup).
func TestSENTRY008DedupPerPath(t *testing.T) {
	now := time.Now()
	state := NewRuleState()
	tree := editorTree()
	ev := SentryEvent{Kind: EventFileWrite, PID: 100, PPID: 1, Exe: "/usr/bin/some-tool", FilePath: "/home/u/.claude/settings.json"}
	if a := EvaluateEvent(ev, state, tree, emptyInventory(), defaultCfg(), noBaseline(), now); !hasAlert(a, "SENTRY-008") {
		t.Fatal("expected first SENTRY-008")
	}
	if a := EvaluateEvent(ev, state, tree, emptyInventory(), defaultCfg(), noBaseline(), now.Add(time.Second)); hasAlert(a, "SENTRY-008") {
		t.Error("SENTRY-008 must NOT re-fire for the same path (per-path-per-session dedup)")
	}
}

// TestSENTRY007SeesPersistenceWrite proves the closed extension point: a
// persistence write (no cred read) followed by an external outbound fires
// SENTRY-007 via PersistWriteByPID.
func TestSENTRY007SeesPersistenceWrite(t *testing.T) {
	now := time.Now()
	state := NewRuleState()
	tree := editorTree()
	// A persistence write populates PersistWriteByPID (and fires SENTRY-008).
	EvaluateEvent(SentryEvent{Kind: EventFileWrite, PID: 100, PPID: 1, Exe: "/usr/bin/some-tool", FilePath: "/home/u/.config/systemd/user/evil.service"},
		state, tree, emptyInventory(), defaultCfg(), noBaseline(), now)
	// An external outbound now fuses on the recent persistence write (no cred read).
	alerts := EvaluateEvent(SentryEvent{Kind: EventNetworkConnect, PID: 100, PPID: 1, Exe: "/usr/bin/some-tool", DstAddr: net.ParseIP("8.8.8.8")},
		state, tree, emptyInventory(), defaultCfg(), noBaseline(), now.Add(10*time.Second))
	if !hasAlert(alerts, "SENTRY-007") {
		t.Error("expected SENTRY-007 to fuse on a recent persistence write + external outbound")
	}
}

// ---- Phase 20 (SENT-11, OPTIONAL) — EventDNSQuery passthrough ----

// TestDNSQueryPassThrough proves an EventDNSQuery flows through EvaluateEvent
// without panicking and produces no alert yet (the QNAME is ingested for the
// DNS-TXT tunnelling gap, but no DNS-exfil correlation rule consumes it).
func TestDNSQueryPassThrough(t *testing.T) {
	now := time.Now()
	state := NewRuleState()
	tree := editorTree()
	ev := SentryEvent{Kind: EventDNSQuery, PID: 100, PPID: 1, Exe: "/usr/bin/some-tool", FilePath: "exfil.attacker.example"}
	alerts := EvaluateEvent(ev, state, tree, emptyInventory(), defaultCfg(), noBaseline(), now)
	if len(alerts) != 0 {
		t.Errorf("EventDNSQuery should produce no alerts yet, got %d", len(alerts))
	}
}
