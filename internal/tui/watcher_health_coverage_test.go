package tui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	audit "github.com/home-beekeeper/beekeeper/internal/audit"
	platform "github.com/home-beekeeper/beekeeper/internal/platform"
)

// --- tick command closures (watcher.go) ---

// TestTickCommandsProduceMessages proves each tick command, when its closure is
// invoked, yields a message of the expected type.
func TestTickCommandsProduceMessages(t *testing.T) {
	if _, ok := clockCmd()().(clockMsg); !ok {
		t.Error("clockCmd should produce a clockMsg")
	}
	if _, ok := stateTickCmd()().(stateTick); !ok {
		t.Error("stateTickCmd should produce a stateTick")
	}
	if _, ok := healthTickCmd()().(healthTick); !ok {
		t.Error("healthTickCmd should produce a healthTick")
	}
	if _, ok := stepTickCmd()().(stepTickMsg); !ok {
		t.Error("stepTickCmd should produce a stepTickMsg")
	}
}

// --- watchAuditLog (watcher.go) ---

// recvModel is a tiny Bubble Tea model that signals a channel the first time it
// receives a newRecordsMsg, then quits the program.
type recvModel struct {
	got chan []audit.AuditRecord
}

func (m recvModel) Init() tea.Cmd { return nil }
func (m recvModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if recs, ok := msg.(newRecordsMsg); ok {
		select {
		case m.got <- []audit.AuditRecord(recs):
		default:
		}
		return m, tea.Quit
	}
	return m, nil
}
func (m recvModel) View() tea.View { return tea.NewView("") }

// TestWatchAuditLogDeliversRecords runs watchAuditLog against a real program and
// proves a newly-appended record is delivered as a newRecordsMsg. The wait is
// bounded by a context deadline (a failsafe, not a timing-based assertion): the
// fallback ticker drives delivery within ~1s.
func TestWatchAuditLogDeliversRecords(t *testing.T) {
	dir := t.TempDir()
	auditPath := filepath.Join(dir, "beekeeper.ndjson")

	got := make(chan []audit.AuditRecord, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	p := tea.NewProgram(recvModel{got: got},
		tea.WithContext(ctx),
		tea.WithoutRenderer(),
		tea.WithoutSignalHandler(),
		tea.WithInput(nil),
	)

	go watchAuditLog(p, auditPath)

	// Write a complete record after the watcher is up.
	go func() {
		line, _ := json.Marshal(audit.AuditRecord{
			RecordType: "policy_decision", Decision: "block", AgentName: "claude",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
		// The fallback ticker re-reads every second regardless of fsnotify support.
		_ = os.WriteFile(auditPath, append(line, '\n'), 0o600)
	}()

	done := make(chan struct{})
	go func() { _, _ = p.Run(); close(done) }()

	select {
	case recs := <-got:
		if len(recs) == 0 || recs[0].Decision != "block" {
			t.Errorf("watchAuditLog delivered %+v, want a block record", recs)
		}
	case <-ctx.Done():
		p.Quit()
		t.Fatal("watchAuditLog did not deliver the appended record within the deadline")
	}
	p.Quit()
	<-done
}

// --- health probes (health.go) ---

// TestProbeCatalogsFreshStaleMissing covers the three probeCatalogs outcomes via
// a real bumblebee.idx in an isolated catalog dir.
func TestProbeCatalogsFreshStaleMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("BEEKEEPER_HOME", home)

	// Missing index → false.
	if probeCatalogs() {
		t.Error("probeCatalogs with no index should be false")
	}

	// Resolve the catalog dir the same way the probe does and create the index.
	// CatalogDir is BEEKEEPER_HOME-rooted, so this stays hermetic.
	idxDir, err := platform.CatalogDir()
	if err != nil {
		t.Fatal(err)
	}
	idxPath := filepath.Join(idxDir, "bumblebee.idx")
	if err := os.MkdirAll(idxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(idxPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !probeCatalogs() {
		t.Error("probeCatalogs with a fresh index should be true")
	}

	// Backdate the index past 25h → stale → false.
	old := time.Now().Add(-26 * time.Hour)
	if err := os.Chtimes(idxPath, old, old); err != nil {
		t.Fatal(err)
	}
	if probeCatalogs() {
		t.Error("probeCatalogs with a >25h index should be false")
	}
}

// TestProbeHooks covers both branches: a settings.json mentioning beekeeper, and
// one that does not.
func TestProbeHooks(t *testing.T) {
	home := t.TempDir()
	// Point the user home (probeHooks uses os.UserHomeDir, not BEEKEEPER_HOME).
	t.Setenv("HOME", home)              // unix
	t.Setenv("USERPROFILE", home)       // windows

	// No settings file → false.
	if probeHooks() {
		t.Error("probeHooks with no settings.json should be false")
	}

	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(claudeDir, "settings.json")

	// Settings without a beekeeper reference → false.
	if err := os.WriteFile(settings, []byte(`{"hooks":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if probeHooks() {
		t.Error("probeHooks without a beekeeper reference should be false")
	}

	// Settings with a beekeeper reference → true.
	if err := os.WriteFile(settings, []byte(`{"hooks":{"PreToolUse":[{"command":"beekeeper check"}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !probeHooks() {
		t.Error("probeHooks with a beekeeper reference should be true")
	}
}

// TestProbeLastBlock covers the no-blocks branch and a real recent block.
func TestProbeLastBlock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("BEEKEEPER_HOME", home)

	// No audit log yet → "no blocks yet".
	if got := probeLastBlock(); got != "no blocks yet" {
		t.Errorf("probeLastBlock with no log = %q, want 'no blocks yet'", got)
	}

	auditDir, err := platform.AuditDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(auditDir, 0o755); err != nil {
		t.Fatal(err)
	}
	line, _ := json.Marshal(audit.AuditRecord{
		RecordType: "policy_decision", Decision: "block",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
	if err := os.WriteFile(filepath.Join(auditDir, "beekeeper.ndjson"), append(line, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := probeLastBlock(); !strings.Contains(got, "last block just now") {
		t.Errorf("probeLastBlock with a recent block = %q, want 'last block just now'", got)
	}
}

// TestProbeLlamaFirewallMalformed covers the fail-safe branches: missing file,
// bad JSON, missing/typed-wrong fields — all must return false without panicking.
func TestProbeLlamaFirewallMalformed(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	// Missing file.
	if probeLlamaFirewall(dir) {
		t.Error("missing state.json should probe false")
	}

	cases := []string{
		`not json`,                              // unparseable
		`{}`,                                    // no llamafirewall key
		`{"llamafirewall": "wrong-type"}`,       // wrong sub-object type
		`{"llamafirewall": {}}`,                 // no pid
		`{"llamafirewall": {"pid": "x"}}`,       // pid wrong type
		`{"llamafirewall": {"pid": 0}}`,         // pid <= 0 → pidAlive false
	}
	for _, c := range cases {
		if err := os.WriteFile(statePath, []byte(c), 0o600); err != nil {
			t.Fatal(err)
		}
		if probeLlamaFirewall(dir) {
			t.Errorf("malformed state %q should probe false", c)
		}
	}
}

// TestRefreshHealthStateFailSafe proves refreshHealthState never panics and
// returns a degraded (all-false) state when nothing is reachable.
func TestRefreshHealthStateFailSafe(t *testing.T) {
	t.Setenv("BEEKEEPER_HOME", t.TempDir())
	hs := refreshHealthState(t.TempDir())
	if hs.GatewayOK || hs.SentryOK || hs.CatalogsOK || hs.LlamaFirewallOK {
		t.Errorf("an unreachable environment should report degraded health, got %+v", hs)
	}
	if hs.LastBlock == "" {
		t.Error("LastBlock should always carry a human-readable string")
	}
}

// TestPidAliveSelf proves the host pidAlive reports the current process alive and
// a non-positive pid dead.
func TestPidAliveSelf(t *testing.T) {
	if !pidAlive(os.Getpid()) {
		t.Error("pidAlive(self) should be true")
	}
	if pidAlive(0) || pidAlive(-1) {
		t.Error("pidAlive on a non-positive pid should be false")
	}
}
