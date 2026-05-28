//go:build linux || darwin

package ipc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
)

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

	l, err := net.Listen("unix", sockPath)
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
