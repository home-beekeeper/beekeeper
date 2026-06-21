package sentry

import (
	"testing"
	"time"
)

// agentInstallTree returns a process tree with claude(pid=1) -> npm(pid=200),
// so the install spawn is agent-descended (monitored).
func agentInstallTree() map[uint32]ProcessNode {
	return buildTree([]ProcessNode{
		{PID: 1, PPID: 0, Exe: "/usr/local/bin/claude"},
		{PID: 200, PPID: 1, Exe: "/usr/bin/npm", Cmdline: "npm install left-pad"},
	})
}

// TestSENTRY009ObservesInstall: a monitored-descendant install spawn produces a
// detection-only observation alert with install attribution and NO
// block/kill/quarantine action.
func TestSENTRY009ObservesInstall(t *testing.T) {
	tree := agentInstallTree()
	state := NewRuleState()
	now := time.Now().UTC()

	event := SentryEvent{
		Kind:     EventProcessCreate,
		PID:      200,
		PPID:     1,
		Exe:      "/usr/bin/npm",
		Cmdline:  "npm install left-pad",
		WallTime: now,
	}

	alerts := EvaluateEvent(event, state, tree, emptyInventory(), defaultCfg(), noBaseline(), now)

	if !hasAlert(alerts, "SENTRY-009") {
		t.Fatalf("expected SENTRY-009 install observation; got %+v", alerts)
	}

	var obs SentryAlert
	for _, a := range alerts {
		if a.RuleID == "SENTRY-009" {
			obs = a
		}
	}

	// Detection-only invariant: info severity, NO quarantine recommendation.
	if obs.Severity != "info" {
		t.Errorf("Severity = %q, want info (detection-only)", obs.Severity)
	}
	if obs.QuarantineRec {
		t.Error("QuarantineRec = true, want false - Sentry install observation must never recommend quarantine")
	}
	// Install attribution is carried.
	if obs.ProcessPID != 200 {
		t.Errorf("ProcessPID = %d, want 200", obs.ProcessPID)
	}
	if len(obs.FilesAccessed) == 0 || obs.FilesAccessed[0] != "npm install left-pad" {
		t.Errorf("install attribution missing; FilesAccessed = %v, want the install cmdline", obs.FilesAccessed)
	}

	// Map to an audit record via the SAME mapping the daemons use (replicated
	// here as the cross-platform invariant) and assert detection-only labeling.
	rec := observeMappingForTest(obs)
	if rec.recordType != "sentry_install_observed" {
		t.Errorf("record_type = %q, want sentry_install_observed", rec.recordType)
	}
	if rec.decision != "observe" {
		t.Errorf("decision = %q, want observe (never block/warn/quarantine)", rec.decision)
	}
}

// TestSENTRY009IgnoresNonInstall: a manager process that is NOT an install
// (npm run build, go test, pip list) produces no observation.
func TestSENTRY009IgnoresNonInstall(t *testing.T) {
	tree := buildTree([]ProcessNode{
		{PID: 1, PPID: 0, Exe: "/usr/local/bin/claude"},
		{PID: 200, PPID: 1, Exe: "/usr/bin/npm", Cmdline: "npm run build"},
	})
	state := NewRuleState()
	now := time.Now().UTC()

	for _, cmd := range []string{"npm run build", "go test ./...", "pip list", "yarn"} {
		event := SentryEvent{Kind: EventProcessCreate, PID: 200, PPID: 1, Exe: "/usr/bin/npm", Cmdline: cmd, WallTime: now}
		alerts := EvaluateEvent(event, state, tree, emptyInventory(), defaultCfg(), noBaseline(), now)
		if hasAlert(alerts, "SENTRY-009") {
			t.Errorf("%q: unexpected SENTRY-009 observation for a non-install command", cmd)
		}
	}
}

// TestSENTRY009RequiresMonitoredAncestry: an install spawn that is NOT
// editor/agent-descended is not observed (ancestry gate, like the other rules).
func TestSENTRY009RequiresMonitoredAncestry(t *testing.T) {
	tree := buildTree([]ProcessNode{
		{PID: 1, PPID: 0, Exe: "/usr/bin/bash"}, // not an editor or agent
		{PID: 200, PPID: 1, Exe: "/usr/bin/npm", Cmdline: "npm install left-pad"},
	})
	state := NewRuleState()
	now := time.Now().UTC()

	event := SentryEvent{Kind: EventProcessCreate, PID: 200, PPID: 1, Exe: "/usr/bin/npm", Cmdline: "npm install left-pad", WallTime: now}
	alerts := EvaluateEvent(event, state, tree, emptyInventory(), defaultCfg(), noBaseline(), now)
	if hasAlert(alerts, "SENTRY-009") {
		t.Error("SENTRY-009 fired for a non-monitored-descendant install; want gated out")
	}
}

// TestIsInstallProcess unit-covers the pure recognizer.
func TestIsInstallProcess(t *testing.T) {
	cases := []struct {
		exe, cmdline string
		want         bool
	}{
		{"/usr/bin/npm", "npm install left-pad", true},
		{"npm", "npm i left-pad", true},
		{"/usr/bin/pnpm", "pnpm add react", true},
		{"/usr/bin/pip3", "pip3 install requests", true},
		{"/usr/bin/cargo", "cargo install ripgrep", true},
		{"/usr/bin/go", "go get example.com/x", true},
		{"/usr/bin/composer", "composer require monolog/monolog", true},
		{"/usr/bin/npm", "npm run build", false}, // not an install verb
		{"/usr/bin/go", "go test ./...", false},
		{"/usr/bin/bash", "bash install something", false}, // bash is not a manager
		{"/usr/bin/npm", "", false},
	}
	for _, c := range cases {
		if got := isInstallProcess(c.exe, c.cmdline); got != c.want {
			t.Errorf("isInstallProcess(%q, %q) = %v, want %v", c.exe, c.cmdline, got, c.want)
		}
	}
}

// observeRec mirrors the daemon decision-mapping outputs for the detection-only
// assertion in the rule test, without importing an OS-specific daemon package.
type observeRec struct {
	recordType string
	decision   string
}

// observeMappingForTest replicates the daemons' alertToAuditRecord decision
// mapping for the install-observation case. It is kept in the test so a drift
// between the rule's "info" severity contract and the daemons' mapping is caught
// here (the daemon files carry the production copy field-for-field).
func observeMappingForTest(alert SentryAlert) observeRec {
	recordType := "sentry_alert"
	decision := "block"
	switch {
	case alert.Severity == "info":
		recordType = "sentry_install_observed"
		decision = "observe"
	case alert.BaselineMode:
		recordType = "sentry_alert_baseline"
		decision = "warn"
	case !alert.QuarantineRec:
		decision = "warn"
	}
	return observeRec{recordType: recordType, decision: decision}
}
