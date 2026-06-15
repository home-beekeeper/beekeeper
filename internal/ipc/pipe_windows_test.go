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

// TestNewServerRejectsSquattedPipe is the pipe-squatting regression guard
// (T-IPC-02). winio.ListenPipe creates the FIRST pipe instance with FILE_CREATE
// semantics (the NT equivalent of FILE_FLAG_FIRST_PIPE_INSTANCE), so a second
// NewServer on the same pipe name — i.e. a name already "squatted" by the first
// server — must fail with a name-collision error rather than silently attaching
// to an existing pipe. This proves the fail-closed startup behavior the comment
// in pipe_windows.go documents, and catches any future winio change to
// FILE_OPEN_IF semantics that would reintroduce the squatting risk.
func TestNewServerRejectsSquattedPipe(t *testing.T) {
	withTestPipe(t, pipeNameForTest(t))

	// First server creates (owns) the pipe name.
	srv1, err := NewServer("", 0)
	if err != nil {
		t.Fatalf("first NewServer: %v", err)
	}
	defer srv1.Close()

	// Second server on the same (now-existing/squatted) name must fail closed.
	srv2, err := NewServer("", 0)
	if err == nil {
		srv2.Close()
		t.Fatal("second NewServer on an existing pipe name succeeded; want a name-collision failure (squatting must fail closed)")
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
