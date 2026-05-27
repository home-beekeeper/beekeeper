//go:build windows

package ipc

import (
	"net"
	"time"
)

// Connect is not supported on Windows; always returns ErrNotSupported.
func Connect(sockPath string, timeout time.Duration) (net.Conn, error) {
	return nil, ErrNotSupported
}

// NewServer is not supported on Windows; always returns ErrNotSupported.
func NewServer(sockPath string, ownerUID uint32) (*Server, error) {
	return nil, ErrNotSupported
}

// Server is a placeholder type on Windows. All methods return ErrNotSupported
// or nil without doing any work.
type Server struct{}

// Close is a no-op on Windows.
func (s *Server) Close() error { return nil }
