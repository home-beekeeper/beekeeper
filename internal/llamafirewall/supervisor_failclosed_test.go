// Cross-platform (no build tag) fail-closed unit tests for the LlamaFirewall
// supervisor. The existing supervisor_test.go is gated //go:build linux because
// it dials a real Unix-domain socket / mock sidecar; on Windows and macOS those
// tests never run, leaving the fail-closed guarantee uncovered on the primary
// dev platform.
//
// These tests exercise only the platform-agnostic, hermetic paths of the
// supervisor — construction, the fail-closed/fail-open decision in Scan, the
// degraded/status reporting, Stop on a never-started supervisor, and the
// state.json round-trip via BEEKEEPER_HOME — WITHOUT spawning a real Python
// sidecar, opening any socket, or relying on Linux. They guard the
// CLAUDE.md non-negotiable: "LlamaFirewall: Python sidecar, supervised;
// fail-closed on crash."
package llamafirewall

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bantuson/beekeeper/internal/platform"
)

// newTestSupervisor builds a supervisor with the given FailMode and full
// sampling, using OS-neutral placeholder paths so the test never touches a real
// socket or sidecar script.
func newTestSupervisor(failMode string) *Supervisor {
	return NewSupervisor(LlamaFirewallConfig{
		Enabled:    true,
		FailMode:   failMode,
		SampleRate: 1.0,
	}, "test.sock", "fake_sidecar.py")
}

// TestNewSupervisorInitialState verifies NewSupervisor returns a sane,
// not-started supervisor: not degraded, no process, no client, MaxRetries set,
// and a platform-appropriate default Python interpreter.
func TestNewSupervisorInitialState(t *testing.T) {
	sup := newTestSupervisor("closed")

	if sup == nil {
		t.Fatal("NewSupervisor returned nil")
	}
	if sup.IsDegraded() {
		t.Error("freshly constructed supervisor must not be degraded")
	}
	if sup.proc != nil {
		t.Error("freshly constructed supervisor must have no process")
	}
	if sup.client != nil {
		t.Error("freshly constructed supervisor must have no client")
	}
	if sup.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want default 3", sup.MaxRetries)
	}
	if sup.SidecarPath != "fake_sidecar.py" {
		t.Errorf("SidecarPath = %q, want %q", sup.SidecarPath, "fake_sidecar.py")
	}
	if sup.SockPath != "test.sock" {
		t.Errorf("SockPath = %q, want %q", sup.SockPath, "test.sock")
	}

	wantPython := "python3"
	if runtime.GOOS == "windows" {
		wantPython = "python"
	}
	if sup.PythonPath != wantPython {
		t.Errorf("PythonPath = %q, want %q for GOOS=%s", sup.PythonPath, wantPython, runtime.GOOS)
	}
}

// TestNewSupervisorHonorsExplicitPython verifies an explicit PythonPath in the
// config is preserved verbatim (not overridden by the OS default).
func TestNewSupervisorHonorsExplicitPython(t *testing.T) {
	sup := NewSupervisor(LlamaFirewallConfig{
		Enabled:    true,
		FailMode:   "closed",
		SampleRate: 1.0,
		PythonPath: "/opt/custom/python",
	}, "test.sock", "fake_sidecar.py")

	if sup.PythonPath != "/opt/custom/python" {
		t.Errorf("PythonPath = %q, want explicit /opt/custom/python", sup.PythonPath)
	}
}

// TestScanFailsClosedWhenDegraded is the core fail-closed assertion: a degraded
// supervisor whose FailMode is "closed" MUST return ErrSidecarUnavailable and an
// EMPTY ScanResponse — it must never silently allow (ResultClean).
//
// Contract (supervisor.go Scan): when degraded && failMode is "closed"/"" it
// returns `ScanResponse{}, ErrSidecarUnavailable`.
func TestScanFailsClosedWhenDegraded(t *testing.T) {
	sup := newTestSupervisor("closed")
	sup.degraded = true
	sup.retries = sup.MaxRetries

	resp, err := sup.Scan(context.Background(), ScanRequest{
		Kind:      ScanPrompt,
		Content:   "test",
		RequestID: "req-degraded-closed",
	})

	if err != ErrSidecarUnavailable {
		t.Fatalf("degraded fail-closed: err = %v, want ErrSidecarUnavailable", err)
	}
	// The most important guarantee: an unavailable sidecar must NOT yield a
	// clean/allow verdict. The response must be the zero value.
	if resp.Result == ResultClean {
		t.Fatal("FAIL-OPEN LEAK: degraded fail-closed Scan returned ResultClean")
	}
	if resp.Result != "" {
		t.Errorf("degraded fail-closed: resp.Result = %q, want empty", resp.Result)
	}
}

// TestScanFailsClosedWhenDegradedDefaultMode verifies the DEFAULT FailMode ("")
// also fails closed. This is critical: an operator who never set fail_mode must
// get fail-closed semantics, not accidental fail-open.
func TestScanFailsClosedWhenDegradedDefaultMode(t *testing.T) {
	sup := newTestSupervisor("") // default / unset fail mode
	sup.degraded = true

	resp, err := sup.Scan(context.Background(), ScanRequest{
		Kind:      ScanPrompt,
		Content:   "test",
		RequestID: "req-degraded-default",
	})

	if err != ErrSidecarUnavailable {
		t.Fatalf("default-mode degraded: err = %v, want ErrSidecarUnavailable", err)
	}
	if resp.Result == ResultClean {
		t.Fatal("FAIL-OPEN LEAK: default-mode degraded Scan returned ResultClean")
	}
}

// TestScanFailsClosedWhenClientNilNotStarted verifies that a supervisor that was
// NEVER started (no client, not degraded) with default/closed fail mode also
// fails closed. This is the realistic "Start() never called or socket never
// connected" path — it hits the `c == nil` branch rather than the degraded
// branch, and must likewise refuse to allow.
func TestScanFailsClosedWhenClientNilNotStarted(t *testing.T) {
	for _, failMode := range []string{"", "closed"} {
		sup := newTestSupervisor(failMode)
		// Not degraded, never started -> client is nil.
		if sup.degraded {
			t.Fatal("precondition: supervisor should not be degraded")
		}
		if sup.client != nil {
			t.Fatal("precondition: supervisor should have nil client")
		}

		resp, err := sup.Scan(context.Background(), ScanRequest{
			Kind:      ScanPrompt,
			Content:   "test",
			RequestID: "req-nilclient-" + failMode,
		})

		if err != ErrSidecarUnavailable {
			t.Fatalf("failMode=%q nil-client: err = %v, want ErrSidecarUnavailable", failMode, err)
		}
		if resp.Result == ResultClean {
			t.Fatalf("FAIL-OPEN LEAK: failMode=%q nil-client Scan returned ResultClean", failMode)
		}
	}
}

// TestScanFailsOpenWhenConfigured verifies the explicit opt-in fail-open / warn
// paths return ResultClean with no error and an annotated reason. This documents
// that fail-open is reachable ONLY when explicitly configured (the inverse of
// the fail-closed guarantee).
func TestScanFailsOpenWhenConfigured(t *testing.T) {
	for _, failMode := range []string{"open", "warn"} {
		// Degraded path.
		sup := newTestSupervisor(failMode)
		sup.degraded = true

		resp, err := sup.Scan(context.Background(), ScanRequest{
			Kind:      ScanPrompt,
			Content:   "test",
			RequestID: "req-open-degraded-" + failMode,
		})
		if err != nil {
			t.Fatalf("failMode=%q degraded: unexpected error %v", failMode, err)
		}
		if resp.Result != ResultClean {
			t.Fatalf("failMode=%q degraded: result = %q, want clean", failMode, resp.Result)
		}
		if resp.Reason != "sidecar unavailable (fail-open)" {
			t.Errorf("failMode=%q degraded: reason = %q, want fail-open annotation", failMode, resp.Reason)
		}
		if resp.RequestID != "req-open-degraded-"+failMode {
			t.Errorf("failMode=%q: RequestID not echoed, got %q", failMode, resp.RequestID)
		}

		// Nil-client (not-started) path also fails open when configured.
		sup2 := newTestSupervisor(failMode)
		resp2, err2 := sup2.Scan(context.Background(), ScanRequest{
			Kind:      ScanPrompt,
			Content:   "test",
			RequestID: "req-open-nilclient-" + failMode,
		})
		if err2 != nil {
			t.Fatalf("failMode=%q nil-client: unexpected error %v", failMode, err2)
		}
		if resp2.Result != ResultClean {
			t.Fatalf("failMode=%q nil-client: result = %q, want clean", failMode, resp2.Result)
		}
	}
}

// TestScanSampleRateZeroSkipsSidecar verifies that SampleRate <= 0 is treated as
// 1.0 (scan everything) per the documented contract, NOT as "skip everything".
// With rate clamped to 1.0 and a nil client + fail-closed mode, the request must
// still reach the fail-closed branch (proving the request was not silently
// dropped to a clean verdict by sampling).
func TestScanSampleRateZeroIsTreatedAsFull(t *testing.T) {
	sup := NewSupervisor(LlamaFirewallConfig{
		Enabled:    true,
		FailMode:   "closed",
		SampleRate: 0.0, // documented: zero/negative -> 1.0 (scan everything)
	}, "test.sock", "fake_sidecar.py")
	// Not degraded, nil client.

	resp, err := sup.Scan(context.Background(), ScanRequest{
		Kind:      ScanPrompt,
		Content:   "test",
		RequestID: "req-sample-zero",
	})

	// Because 0.0 is clamped to 1.0 (scan everything), the nil client forces
	// the fail-closed branch. If 0.0 were (wrongly) treated as "never sample"
	// we'd instead get a clean "not sampled" allow — which would be a fail-open
	// leak for a fail-closed config.
	if err != ErrSidecarUnavailable {
		t.Fatalf("sample-rate 0 with nil client: err = %v, want ErrSidecarUnavailable", err)
	}
	if resp.Result == ResultClean {
		t.Fatal("FAIL-OPEN LEAK: sample-rate 0 treated as 'never sample' -> clean verdict on fail-closed config")
	}
}

// TestIsDegradedReflectsState verifies IsDegraded tracks the internal flag.
func TestIsDegradedReflectsState(t *testing.T) {
	sup := newTestSupervisor("closed")
	if sup.IsDegraded() {
		t.Error("new supervisor: IsDegraded() = true, want false")
	}
	sup.mu.Lock()
	sup.degraded = true
	sup.mu.Unlock()
	if !sup.IsDegraded() {
		t.Error("after degraded=true: IsDegraded() = false, want true")
	}
}

// TestStatusInfoNotStarted verifies StatusInfo on a never-started supervisor
// reports the not-ready snapshot: PID 0, not degraded, zero-value StartedAt,
// and the configured SampleRate / FailMode echoed back.
func TestStatusInfoNotStarted(t *testing.T) {
	sup := NewSupervisor(LlamaFirewallConfig{
		Enabled:    true,
		FailMode:   "closed",
		SampleRate: 0.5,
	}, "test.sock", "fake_sidecar.py")

	st := sup.StatusInfo()
	if st.PID != 0 {
		t.Errorf("not-started StatusInfo: PID = %d, want 0", st.PID)
	}
	if st.Degraded {
		t.Error("not-started StatusInfo: Degraded = true, want false")
	}
	if !st.StartedAt.IsZero() {
		t.Errorf("not-started StatusInfo: StartedAt = %v, want zero", st.StartedAt)
	}
	if st.SampleRate != 0.5 {
		t.Errorf("StatusInfo: SampleRate = %v, want 0.5", st.SampleRate)
	}
	if st.FailMode != "closed" {
		t.Errorf("StatusInfo: FailMode = %q, want closed", st.FailMode)
	}
	if st.P95LatencyMS != 0 {
		t.Errorf("not-started StatusInfo: P95LatencyMS = %d, want 0", st.P95LatencyMS)
	}
}

// TestStatusInfoDegraded verifies StatusInfo surfaces the degraded flag.
func TestStatusInfoDegraded(t *testing.T) {
	sup := newTestSupervisor("closed")
	sup.mu.Lock()
	sup.degraded = true
	sup.mu.Unlock()

	if !sup.StatusInfo().Degraded {
		t.Error("StatusInfo().Degraded = false after degraded=true, want true")
	}
}

// TestStopNeverStartedIsSafe verifies Stop on a supervisor that was never
// started does not panic and returns nil (nil proc and nil client are handled).
func TestStopNeverStartedIsSafe(t *testing.T) {
	sup := newTestSupervisor("closed")
	if err := sup.Stop(); err != nil {
		t.Fatalf("Stop on never-started supervisor: err = %v, want nil", err)
	}
	// Idempotent: a second Stop must also be safe.
	if err := sup.Stop(); err != nil {
		t.Fatalf("second Stop on never-started supervisor: err = %v, want nil", err)
	}
}

// TestPersistStateRoundTrip drives persistState hermetically by redirecting the
// state directory to a temp dir via BEEKEEPER_HOME (honored by platform.StateDir
// on all platforms), then reads state.json back and asserts the llamafirewall
// PID was recorded. No process is spawned.
func TestPersistStateRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("BEEKEEPER_HOME", tmp)

	// platform.StateDir() returns <BEEKEEPER_HOME>/beekeeper; persistState uses
	// os.WriteFile which does NOT create parent dirs, so create it first.
	stateDir, err := platform.StateDir()
	if err != nil {
		t.Fatalf("platform.StateDir: %v", err)
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}

	sup := newTestSupervisor("closed")
	const wantPID = 4242
	sup.persistState(wantPID)

	statePath := filepath.Join(stateDir, "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state.json: %v", err)
	}

	var state map[string]any
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("unmarshal state.json: %v", err)
	}
	lf, ok := state["llamafirewall"].(map[string]any)
	if !ok {
		t.Fatalf("state.json missing llamafirewall key; got %v", state)
	}
	// JSON numbers decode to float64.
	if pid, _ := lf["pid"].(float64); int(pid) != wantPID {
		t.Errorf("persisted pid = %v, want %d", lf["pid"], wantPID)
	}
	if ts, _ := lf["started_at"].(string); ts == "" {
		t.Error("persisted started_at is empty, want an RFC3339 timestamp")
	}
}

// TestPersistStatePreservesExistingKeys verifies persistState merges into an
// existing state.json rather than clobbering unrelated keys (it round-trips the
// existing map and only sets the "llamafirewall" entry).
func TestPersistStatePreservesExistingKeys(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("BEEKEEPER_HOME", tmp)

	stateDir, err := platform.StateDir()
	if err != nil {
		t.Fatalf("platform.StateDir: %v", err)
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	statePath := filepath.Join(stateDir, "state.json")

	// Seed an existing, unrelated key.
	seed := []byte(`{"sentry":{"running":true}}`)
	if err := os.WriteFile(statePath, seed, 0o600); err != nil {
		t.Fatalf("seed state.json: %v", err)
	}

	sup := newTestSupervisor("closed")
	sup.persistState(7)

	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state.json: %v", err)
	}
	var state map[string]any
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("unmarshal state.json: %v", err)
	}
	if _, ok := state["sentry"]; !ok {
		t.Error("persistState clobbered unrelated 'sentry' key")
	}
	if _, ok := state["llamafirewall"]; !ok {
		t.Error("persistState did not write 'llamafirewall' key")
	}
}

// TestPersistStateNoStateDirIsSafe verifies persistState does not panic when the
// state directory cannot be determined / written. It is best-effort
// (informational state only) and must swallow errors silently.
func TestPersistStateNoStateDirIsSafe(t *testing.T) {
	tmp := t.TempDir()
	// Point BEEKEEPER_HOME at a path whose <home>/beekeeper dir does NOT exist,
	// so os.WriteFile fails — persistState must not panic and must return cleanly.
	t.Setenv("BEEKEEPER_HOME", filepath.Join(tmp, "does", "not", "exist"))

	sup := newTestSupervisor("closed")
	// Should not panic even though the write will fail.
	sup.persistState(1)
}
