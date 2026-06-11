//go:build linux || darwin

package ipc

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// tempSock returns a short Unix-socket path under /tmp. /tmp keeps the path well
// under the ~104-byte sun_path limit (t.TempDir on macOS lives under a long
// /var/folders path that can overflow it). Shared by the ipc Tier-B tests.
func tempSock(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "bkipc")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "s.sock")
}

// TestNewServerSocketPermsAndOwner asserts NewServer creates the socket at 0600.
func TestNewServerSocketPermsAndOwner(t *testing.T) {
	sock := tempSock(t)
	s, err := NewServer(sock, uint32(os.Getuid()))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer s.Close()

	info, err := os.Stat(sock)
	if err != nil {
		t.Fatalf("stat socket: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("socket perm = %o, want 0600 (owner-only)", perm)
	}
}

// TestNewServerRemovesPreExisting asserts a stale socket file at the path is
// removed so NewServer can bind.
func TestNewServerRemovesPreExisting(t *testing.T) {
	sock := tempSock(t)
	if err := os.WriteFile(sock, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := NewServer(sock, uint32(os.Getuid()))
	if err != nil {
		t.Fatalf("NewServer should remove a pre-existing socket and bind: %v", err)
	}
	s.Close()
}

// TestServeReturnsOnContextCancel asserts Serve returns ctx.Err() when the
// context is cancelled (no hang, clean shutdown).
func TestServeReturnsOnContextCancel(t *testing.T) {
	sock := tempSock(t)
	s, err := NewServer(sock, uint32(os.Getuid()))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- s.Serve(ctx, func(c net.Conn) { c.Close() }) }()

	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-errCh:
		if err == nil {
			t.Error("Serve should return ctx.Err() on cancel, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Error("Serve did not return within 2s of context cancel")
	}
}

// TestServeRejectsUnauthenticatedPeer asserts the fail-closed peer-auth path: a
// connection whose peer UID does not match the server's ownerUID is closed
// BEFORE the handler runs.
func TestServeRejectsUnauthenticatedPeer(t *testing.T) {
	sock := tempSock(t)
	// ownerUID deliberately != the current process UID → verifyPeerUID rejects
	// the same-process connection.
	s, err := NewServer(sock, uint32(os.Getuid())+1)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	handlerCalled := make(chan struct{}, 1)
	go s.Serve(ctx, func(c net.Conn) { handlerCalled <- struct{}{}; c.Close() })

	conn, err := Connect(sock, time.Second)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer conn.Close()
	select {
	case <-handlerCalled:
		t.Error("handler ran for an unauthenticated (wrong-UID) peer — fail-closed peer-auth not enforced")
	case <-time.After(300 * time.Millisecond):
		// expected: connection closed before the handler is dispatched
	}
}

// TestServeAcceptsAuthenticatedPeer asserts a same-UID peer is authenticated and
// dispatched to the handler.
func TestServeAcceptsAuthenticatedPeer(t *testing.T) {
	sock := tempSock(t)
	s, err := NewServer(sock, uint32(os.Getuid()))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	handlerCalled := make(chan struct{}, 1)
	go s.Serve(ctx, func(c net.Conn) { handlerCalled <- struct{}{}; c.Close() })

	conn, err := Connect(sock, time.Second)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer conn.Close()
	select {
	case <-handlerCalled:
		// expected: authenticated peer reaches the handler
	case <-time.After(2 * time.Second):
		t.Error("handler not called for an authenticated same-UID peer")
	}
}
