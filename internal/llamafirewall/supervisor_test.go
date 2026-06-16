//go:build linux

package llamafirewall

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"
)

// testToken is the bearer token used by the TCP mocks below. The mocks do not
// enforce it (token rejection is covered by client_token_test.go); they accept
// any request so the supervisor's Scan/latency paths can be exercised.
const testToken = "test-token"

// startMockSidecar creates a loopback TCP listener (one transport on every OS,
// matching the production sidecar — Phase 20, LLMF). It accepts exactly one
// connection, reads a ScanRequest, writes back resp, then closes the connection.
// Returns the dial address and a cleanup function that stops the listener.
func startMockSidecar(t *testing.T, resp ScanResponse) (string, func()) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("startMockSidecar: listen: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			// Listener closed — normal shutdown.
			return
		}
		defer conn.Close()

		var req ScanRequest
		if err := Decode(conn, &req); err != nil {
			return
		}
		_ = Encode(conn, resp)
	}()

	return ln.Addr().String(), func() {
		ln.Close()
		<-done
	}
}

// startCrashingSidecar accepts one connection then immediately closes it to
// simulate a sidecar crash during a read. Returns the dial address and cleanup.
func startCrashingSidecar(t *testing.T) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("startCrashingSidecar: listen: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		// Immediately close — simulates process crash.
		conn.Close()
	}()
	return ln.Addr().String(), func() {
		ln.Close()
		<-done
	}
}

// TestSupervisorFailsClosedAfterMaxRetries verifies that a degraded Supervisor
// with fail-mode "closed" returns ErrSidecarUnavailable.
func TestSupervisorFailsClosedAfterMaxRetries(t *testing.T) {
	cfg := LlamaFirewallConfig{
		Enabled:    true,
		FailMode:   "closed",
		SampleRate: 1.0,
	}
	sup := NewSupervisor(cfg, "/tmp/fake_sidecar.py")
	sup.degraded = true
	sup.retries = sup.MaxRetries

	_, err := sup.Scan(context.Background(), ScanRequest{
		Kind:      ScanPrompt,
		Content:   "test",
		RequestID: "req-1",
	})
	if err == nil {
		t.Fatal("expected ErrSidecarUnavailable, got nil")
	}
	if err != ErrSidecarUnavailable {
		t.Fatalf("expected ErrSidecarUnavailable, got %v", err)
	}
}

// TestSupervisorFailsOpenAfterMaxRetries verifies that a degraded Supervisor
// with fail-mode "open" returns ResultClean with no error.
func TestSupervisorFailsOpenAfterMaxRetries(t *testing.T) {
	cfg := LlamaFirewallConfig{
		Enabled:    true,
		FailMode:   "open",
		SampleRate: 1.0,
	}
	sup := NewSupervisor(cfg, "/tmp/fake_sidecar.py")
	sup.degraded = true
	sup.retries = sup.MaxRetries

	resp, err := sup.Scan(context.Background(), ScanRequest{
		Kind:      ScanPrompt,
		Content:   "test",
		RequestID: "req-2",
	})
	if err != nil {
		t.Fatalf("expected nil error in fail-open mode, got %v", err)
	}
	if resp.Result != ResultClean {
		t.Fatalf("expected ResultClean in fail-open, got %q", resp.Result)
	}
}

// TestSampleRateGating verifies the security-safe default: a SampleRate of 0
// (or unset) is treated as "use the default" and resolves to 1.0 (scan every
// request), NOT as "never scan". A 0 sample rate is treated as unset
// consistently across the config resolver (Config.LlamaFirewallSampleRate),
// the untrusted-layer merge (mergeLlamaFirewallUntrusted refuses a sample_rate
// reduction — THREAT-MODEL.md finding #4 depends on 0 meaning unset, not
// "off"), and the supervisor. Disabling LlamaFirewall is done via Enabled:false,
// never via a 0.0 sample rate, so a 0.0 rate must still forward the request to
// the sidecar rather than silently gate it out.
func TestSampleRateGating(t *testing.T) {
	mockResp := ScanResponse{
		RequestID:  "req-3",
		Result:     ResultClean,
		Confidence: 0.99,
		LatencyMS:  7,
	}
	addr, cleanup := startMockSidecar(t, mockResp)
	defer cleanup()

	cfg := LlamaFirewallConfig{
		Enabled:    true,
		FailMode:   "closed",
		SampleRate: 0.0, // treated as unset -> default 1.0 (scan everything)
	}
	sup := NewSupervisor(cfg, "/tmp/fake_sidecar.py")

	c, err := Dial(addr, testToken, time.Second)
	if err != nil {
		t.Fatalf("dial mock sidecar: %v", err)
	}
	defer c.Close()
	sup.client = c

	resp, err := sup.Scan(context.Background(), ScanRequest{
		Kind:      ScanPrompt,
		Content:   "test",
		RequestID: "req-3",
	})
	if err != nil {
		t.Fatalf("Scan with 0.0 sample rate returned error: %v", err)
	}
	// 0.0 must NOT be interpreted as "not sampled" — the safe default scans.
	if resp.Reason == "not sampled" {
		t.Fatal("SampleRate 0.0 must default to scanning, but the request was gated out as 'not sampled'")
	}
	// The request must have reached the sidecar (its response carries LatencyMS=7).
	if resp.Result != ResultClean {
		t.Fatalf("expected ResultClean from the sidecar, got %q", resp.Result)
	}
	if resp.LatencyMS != 7 {
		t.Fatalf("expected the sidecar response (LatencyMS=7); got %d — request was not forwarded", resp.LatencyMS)
	}
}

// TestSupervisorScanSuccess verifies a successful round-trip scan using a mock
// loopback-TCP sidecar.
func TestSupervisorScanSuccess(t *testing.T) {
	mockResp := ScanResponse{
		RequestID:  "req-4",
		Result:     ResultClean,
		Confidence: 0.99,
		LatencyMS:  10,
	}
	addr, cleanup := startMockSidecar(t, mockResp)
	defer cleanup()

	cfg := LlamaFirewallConfig{
		Enabled:    true,
		FailMode:   "closed",
		SampleRate: 1.0,
	}
	sup := NewSupervisor(cfg, "/tmp/fake_sidecar.py")

	// Dial the mock sidecar directly and inject into supervisor.
	c, err := Dial(addr, testToken, time.Second)
	if err != nil {
		t.Fatalf("dial mock sidecar: %v", err)
	}
	defer c.Close()
	sup.client = c

	resp, err := sup.Scan(context.Background(), ScanRequest{
		Kind:      ScanPrompt,
		Content:   "hello",
		RequestID: "req-4",
	})
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if resp.Result != ResultClean {
		t.Fatalf("expected ResultClean, got %q", resp.Result)
	}
	if resp.LatencyMS != 10 {
		t.Fatalf("expected LatencyMS=10, got %d", resp.LatencyMS)
	}
}

// TestLatencyTrackerUpdatedOnScan verifies that after a successful Scan the
// supervisor's latency tracker is non-zero.
func TestLatencyTrackerUpdatedOnScan(t *testing.T) {
	mockResp := ScanResponse{
		RequestID: "req-5",
		Result:    ResultClean,
		LatencyMS: 42,
	}
	addr, cleanup := startMockSidecar(t, mockResp)
	defer cleanup()

	cfg := LlamaFirewallConfig{
		Enabled:    true,
		FailMode:   "closed",
		SampleRate: 1.0,
	}
	sup := NewSupervisor(cfg, "/tmp/fake_sidecar.py")

	c, err := Dial(addr, testToken, time.Second)
	if err != nil {
		t.Fatalf("dial mock sidecar: %v", err)
	}
	defer c.Close()
	sup.client = c

	_, err = sup.Scan(context.Background(), ScanRequest{
		Kind:      ScanPrompt,
		Content:   "test",
		RequestID: "req-5",
	})
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	if p95 := sup.latency.P95(); p95 == 0 {
		t.Fatal("expected non-zero P95 after scan, got 0")
	}
}

// TestSupervisorRestartOnCrash covers the two restart-on-crash guarantees with
// no real Python sidecar (it runs in the default suite, unlike the e2e job):
//
//  1. Crash detection: a sidecar that drops the connection mid-scan (modelled by
//     startCrashingSidecar) must surface an error to Scan — a "crashed" sidecar
//     never yields a silent clean verdict.
//  2. Restart budget: relaunch against an interpreter that cannot start models a
//     sidecar that keeps crashing; after MaxRetries the supervisor enters
//     degraded mode and then fails closed.
func TestSupervisorRestartOnCrash(t *testing.T) {
	cfg := LlamaFirewallConfig{Enabled: true, FailMode: "closed", SampleRate: 1.0}

	// (1) Crash during a scan -> Scan returns an error (fail-closed), not clean.
	addr, cleanup := startCrashingSidecar(t)
	defer cleanup()

	sup := NewSupervisor(cfg, "/tmp/fake_sidecar.py")
	c, err := Dial(addr, testToken, time.Second)
	if err != nil {
		t.Fatalf("dial crashing sidecar: %v", err)
	}
	defer c.Close()
	sup.client = c

	if _, err := sup.Scan(context.Background(), ScanRequest{
		Kind: ScanPrompt, Content: "x", RequestID: "crash",
	}); err == nil {
		t.Fatal("crash-during-scan: expected an error (fail-closed), got nil")
	}

	// (2) A sidecar that cannot relaunch exhausts the retry budget -> degraded.
	sup2 := NewSupervisor(cfg, "/nonexistent/sidecar.py")
	sup2.PythonPath = filepath.Join(t.TempDir(), "definitely-not-a-real-python")
	sup2.MaxRetries = 3
	for i := 0; i < sup2.MaxRetries; i++ {
		if sup2.IsDegraded() {
			t.Fatalf("entered degraded mode too early (attempt %d)", i)
		}
		sup2.relaunch(context.Background())
	}
	if !sup2.IsDegraded() {
		t.Fatal("supervisor did not enter degraded mode after MaxRetries failed relaunches")
	}
	if _, err := sup2.Scan(context.Background(), ScanRequest{
		Kind: ScanPrompt, Content: "x", RequestID: "degraded",
	}); err != ErrSidecarUnavailable {
		t.Fatalf("degraded Scan: err = %v, want ErrSidecarUnavailable", err)
	}
}
