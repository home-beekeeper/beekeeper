//go:build linux

package llamafirewall

import (
	"context"
	"net"
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

// TestSampleRateGating verifies that SampleRate=0.0 results in no request being
// sent to the mock sidecar.
func TestSampleRateGating(t *testing.T) {
	// Track whether the mock sidecar ever accepted a connection.
	accepted := make(chan struct{}, 1)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		accepted <- struct{}{}
		conn.Close()
	}()

	cfg := LlamaFirewallConfig{
		Enabled:    true,
		FailMode:   "closed",
		SampleRate: 0.0, // never sample
	}
	sup := NewSupervisor(cfg, "/tmp/fake_sidecar.py")
	// Wire a real client — the 0.0 sample rate should prevent it from being used.
	c, err := Dial(ln.Addr().String(), testToken, time.Second)
	if err != nil {
		t.Fatalf("dial mock: %v", err)
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
	if resp.Result != ResultClean {
		t.Fatalf("expected ResultClean (not sampled), got %q", resp.Result)
	}
	if resp.Reason != "not sampled" {
		t.Fatalf("expected reason 'not sampled', got %q", resp.Reason)
	}

	// Verify the mock sidecar was never contacted.
	select {
	case <-accepted:
		t.Fatal("mock sidecar was contacted despite 0.0 sample rate")
	case <-time.After(100 * time.Millisecond):
		// Correct: no connection was made.
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
