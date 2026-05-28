//go:build linux

package llamafirewall

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// startMockSidecar creates a Unix listener at sockPath. It accepts exactly one
// connection, reads a ScanRequest, writes back resp, then closes the connection.
// Returns a cleanup function that stops the listener.
func startMockSidecar(t *testing.T, sockPath string, resp ScanResponse) func() {
	t.Helper()

	// Remove stale socket if present.
	_ = os.Remove(sockPath)

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("startMockSidecar: listen on %s: %v", sockPath, err)
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

	return func() {
		ln.Close()
		<-done
		_ = os.Remove(sockPath)
	}
}

// startCrashingSidecar accepts one connection then immediately closes it to
// simulate a sidecar crash during a read.
func startCrashingSidecar(t *testing.T, sockPath string) func() {
	t.Helper()
	_ = os.Remove(sockPath)
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("startCrashingSidecar: listen on %s: %v", sockPath, err)
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
	return func() {
		ln.Close()
		<-done
		_ = os.Remove(sockPath)
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
	sup := NewSupervisor(cfg, "/tmp/test.sock", "/tmp/fake_sidecar.py")
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
	sup := NewSupervisor(cfg, "/tmp/test_open.sock", "/tmp/fake_sidecar.py")
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
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "sidecar.sock")

	// Track whether the mock sidecar ever accepted a connection.
	accepted := make(chan struct{}, 1)
	_ = os.Remove(sockPath)
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	defer os.Remove(sockPath)

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
	sup := NewSupervisor(cfg, sockPath, "/tmp/fake_sidecar.py")
	// Wire a real client — the 0.0 sample rate should prevent it from being used.
	c, err := Dial(sockPath, time.Second)
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
// Unix sidecar.
func TestSupervisorScanSuccess(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "sidecar.sock")

	mockResp := ScanResponse{
		RequestID:  "req-4",
		Result:     ResultClean,
		Confidence: 0.99,
		LatencyMS:  10,
	}
	cleanup := startMockSidecar(t, sockPath, mockResp)
	defer cleanup()

	cfg := LlamaFirewallConfig{
		Enabled:    true,
		FailMode:   "closed",
		SampleRate: 1.0,
	}
	sup := NewSupervisor(cfg, sockPath, "/tmp/fake_sidecar.py")

	// Dial the mock sidecar directly and inject into supervisor.
	c, err := Dial(sockPath, time.Second)
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
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "sidecar_lat.sock")

	mockResp := ScanResponse{
		RequestID: "req-5",
		Result:    ResultClean,
		LatencyMS: 42,
	}
	cleanup := startMockSidecar(t, sockPath, mockResp)
	defer cleanup()

	cfg := LlamaFirewallConfig{
		Enabled:    true,
		FailMode:   "closed",
		SampleRate: 1.0,
	}
	sup := NewSupervisor(cfg, sockPath, "/tmp/fake_sidecar.py")

	c, err := Dial(sockPath, time.Second)
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
