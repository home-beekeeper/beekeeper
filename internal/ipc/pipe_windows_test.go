//go:build windows

package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

// pipeNameForTest returns a unique named pipe path for the given test to avoid
// collisions with a real Beekeeper installation or concurrent test runs.
func pipeNameForTest(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf(`\\.\pipe\beekeeper-test-%s`, t.Name())
}

// withTestPipe temporarily redirects PipePath to the given path for the
// duration of the test, restoring it in t.Cleanup.
func withTestPipe(t *testing.T, path string) {
	t.Helper()
	orig := PipePath
	PipePath = path
	t.Cleanup(func() { PipePath = orig })
}

func TestPipePathConstant(t *testing.T) {
	// The package-level var must default to the canonical production value.
	if PipePath != `\\.\pipe\beekeeper-sentry` {
		t.Errorf("PipePath = %q; want %q", PipePath, `\\.\pipe\beekeeper-sentry`)
	}
}

func TestGetCurrentUserSIDReturnsValid(t *testing.T) {
	sid, err := getCurrentUserSID()
	if err != nil {
		t.Fatalf("getCurrentUserSID() error: %v", err)
	}
	if len(sid) < 4 || sid[:4] != "S-1-" {
		t.Errorf("SID %q does not start with S-1-", sid)
	}
}

func TestNewServerCreatesPipe(t *testing.T) {
	withTestPipe(t, pipeNameForTest(t))

	srv, err := NewServer("ignored", 0)
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}
	defer srv.Close()

	if srv.listener == nil {
		t.Error("Server.listener is nil after NewServer")
	}
}

func TestPipeRoundTrip(t *testing.T) {
	withTestPipe(t, pipeNameForTest(t))

	srv, err := NewServer("", 0)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = srv.Serve(ctx, func(conn net.Conn) {
			defer conn.Close()
			var c IPCCommand
			if err := Decode(conn, &c); err != nil {
				return
			}
			_ = Encode(conn, IPCResponse{Kind: "ok", Payload: json.RawMessage(`{}`)})
		})
	}()

	// Give server a brief moment to enter the Accept loop.
	time.Sleep(50 * time.Millisecond)

	conn, err := Connect("", 2*time.Second)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer conn.Close()

	if err := SendCommand(conn, IPCCommand{Kind: CmdStatusRequest}, time.Second); err != nil {
		t.Fatalf("SendCommand: %v", err)
	}
	resp, err := ReadResponse(conn, time.Second)
	if err != nil {
		t.Fatalf("ReadResponse: %v", err)
	}
	if resp.Kind != "ok" {
		t.Errorf("got Kind=%q; want ok", resp.Kind)
	}

	cancel()
	wg.Wait()
}

func TestEncodeDecodeRoundTripsOnPipe(t *testing.T) {
	withTestPipe(t, pipeNameForTest(t))

	// Verify the 4-byte length-prefix framing is preserved through a real
	// named pipe (RESEARCH Pitfall 9: byte-mode pipe must not strip the header).
	srv, err := NewServer("", 0)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	type payload struct {
		Value string `json:"value"`
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = srv.Serve(ctx, func(conn net.Conn) {
			defer conn.Close()
			var p payload
			if err := Decode(conn, &p); err != nil {
				return
			}
			_ = Encode(conn, payload{Value: "echo:" + p.Value})
		})
	}()

	time.Sleep(50 * time.Millisecond)

	conn, err := Connect("", 2*time.Second)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer conn.Close()

	sent := payload{Value: "hello-pipe"}
	if err := Encode(conn, sent); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	var received payload
	if err := Decode(conn, &received); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	want := "echo:hello-pipe"
	if received.Value != want {
		t.Errorf("got Value=%q; want %q", received.Value, want)
	}

	cancel()
	wg.Wait()
}
