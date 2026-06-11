//go:build linux || darwin

package ipc

import (
	"context"
	"net"
	"os"
	"testing"
	"time"
)

// TestClientServerRoundTrip exercises Connect -> SendCommand -> ReadResponse
// against a real NewServer with same-UID peer auth.
func TestClientServerRoundTrip(t *testing.T) {
	sock := tempSock(t)
	s, err := NewServer(sock, uint32(os.Getuid()))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Serve(ctx, func(c net.Conn) {
		defer c.Close()
		var cmd IPCCommand
		if derr := Decode(c, &cmd); derr != nil {
			return
		}
		_ = Encode(c, IPCResponse{Kind: string(cmd.Kind) + "_ok"})
	})

	conn, err := Connect(sock, time.Second)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer conn.Close()

	if err := SendCommand(conn, IPCCommand{Kind: CmdStatusRequest}, time.Second); err != nil {
		t.Fatalf("SendCommand: %v", err)
	}
	resp, err := ReadResponse(conn, 2*time.Second)
	if err != nil {
		t.Fatalf("ReadResponse: %v", err)
	}
	if resp.Kind != "status_request_ok" {
		t.Errorf("resp.Kind = %q, want status_request_ok", resp.Kind)
	}
}

// TestReadResponseDeadlineTimeout asserts a too-short read deadline returns an
// error rather than hanging when the server never writes a response.
func TestReadResponseDeadlineTimeout(t *testing.T) {
	sock := tempSock(t)
	s, err := NewServer(sock, uint32(os.Getuid()))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Handler authenticates but never writes a response — it holds the
	// connection open until the test's context is cancelled.
	go s.Serve(ctx, func(c net.Conn) {
		<-ctx.Done()
		c.Close()
	})

	conn, err := Connect(sock, time.Second)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer conn.Close()

	start := time.Now()
	if _, err := ReadResponse(conn, 100*time.Millisecond); err == nil {
		t.Fatal("ReadResponse should time out when no response is written, got nil error")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("ReadResponse hung %v instead of honoring the ~100ms read deadline", elapsed)
	}
}
