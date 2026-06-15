//go:build linux || darwin

package ipc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"syscall"
)

// umaskMu serializes the umask save/restore window in NewServer. syscall.Umask
// is a process-global side effect; without this lock two concurrent NewServer
// calls could interleave and leave the process umask at the restrictive value
// (or, worse, race the socket creation of a caller that did not intend the
// tight umask). NewServer is not on a hot path, so the serialization cost is
// negligible.
var umaskMu sync.Mutex

// Server listens on a Unix domain socket and verifies peer credentials before
// dispatching to the provided Handler.
type Server struct {
	listener net.Listener
	sockPath string
	ownerUID uint32
}

// Handler is called for each authenticated connection.
type Handler func(conn net.Conn)

// NewServer creates a Unix socket at sockPath, restricts permissions to 0600,
// and sets socket ownership to ownerUID. Any pre-existing socket at that path
// is removed first.
func NewServer(sockPath string, ownerUID uint32) (*Server, error) {
	if err := os.Remove(sockPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("removing old socket: %w", err)
	}

	// TOCTOU close (T-IPC-01): net.Listen("unix") creates the socket inode under
	// the process umask, which on a default 0022 umask yields 0755 — world- and
	// group-readable/writable. The os.Chmod(0600) below narrows it, but between
	// the Listen and the Chmod there is a window where another local user could
	// connect() to the socket. Set a restrictive umask (0o177 → strips all but
	// owner rwx, so the socket is born at most 0600) around the Listen and
	// restore it immediately after. The Chmod and the SO_PEERCRED check in
	// Serve remain as defense-in-depth.
	//
	// syscall.Umask is process-global; umaskMu serializes the save/restore so a
	// concurrent NewServer cannot observe or clobber the temporary umask.
	l, err := listenUnixOwnerOnly(sockPath)
	if err != nil {
		return nil, fmt.Errorf("listening on %s: %w", sockPath, err)
	}

	if err := os.Chmod(sockPath, 0600); err != nil {
		l.Close()
		return nil, fmt.Errorf("chmod socket: %w", err)
	}

	if err := os.Lchown(sockPath, int(ownerUID), -1); err != nil {
		l.Close()
		return nil, fmt.Errorf("chown socket: %w", err)
	}

	return &Server{
		listener: l,
		sockPath: sockPath,
		ownerUID: ownerUID,
	}, nil
}

// Serve accepts connections in a loop. Each connection is authenticated via
// SO_PEERCRED; unauthenticated connections are closed immediately. Verified
// connections are handed off to handler in a new goroutine.
// Serve returns when ctx is cancelled.
func (s *Server) Serve(ctx context.Context, handler Handler) error {
	// Close the listener when the context is done.
	go func() {
		<-ctx.Done()
		s.listener.Close()
	}()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// If context was cancelled the error is expected.
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}

		go func(c net.Conn) {
			if err := verifyPeerUID(c, s.ownerUID); err != nil {
				c.Close()
				return
			}
			handler(c)
		}(conn)
	}
}

// Close shuts down the listener.
func (s *Server) Close() error {
	return s.listener.Close()
}

// verifyPeerUID is implemented per-platform in peer_linux.go and peer_darwin.go.

// listenUnixOwnerOnly creates a Unix-domain listener at sockPath with a
// restrictive umask in force so the socket inode is never momentarily exposed
// to other local users before the explicit Chmod(0600) narrows it. It restores
// the previous umask before returning regardless of success or failure.
//
// 0o177 clears group/other rwx and the owner's x bit, so a socket created while
// it is in force is born at 0600 (rw owner only). This closes the create→chmod
// TOCTOU window where a default 0022 umask would otherwise produce a 0755 socket
// reachable by any local user during the gap.
func listenUnixOwnerOnly(sockPath string) (net.Listener, error) {
	umaskMu.Lock()
	old := syscall.Umask(0o177)
	l, err := net.Listen("unix", sockPath)
	syscall.Umask(old)
	umaskMu.Unlock()
	return l, err
}
