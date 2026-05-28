//go:build windows

package ipc

import (
	"context"
	"fmt"
	"net"
	"time"

	winio "github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

// PipePath is the canonical Beekeeper Sentry IPC named pipe path on Windows.
// The sockPath parameter to NewServer/Connect is ignored on Windows (Unix path
// semantics do not apply); this variable is used instead. This preserves the
// cross-platform API surface so callers can pass the same Unix-style path
// they pass on Linux/macOS.
//
// Using var rather than const allows tests to substitute a unique pipe path
// per test via pipeNameForTest to avoid collisions with real installations.
var PipePath = `\\.\pipe\beekeeper-sentry`

// Server is the Windows named-pipe equivalent of the Unix server in
// server.go (linux,darwin). It exposes the same Serve / Close API.
//
// Authentication is performed at the OS level: NewServer builds an SDDL DACL
// that grants access only to the installing user's SID. There is no
// SO_PEERCRED equivalent on Windows; the named-pipe DACL is the primary
// auth mechanism. Any principal not matching the SID will receive
// ERROR_ACCESS_DENIED when calling CreateFile on the pipe.
type Server struct {
	listener net.Listener
	sockPath string // kept for parity with the Unix Server fields; unused on Windows
}

// Handler is identical to the Unix Handler type (in server.go), redefined
// here because Go does not allow type declarations to span build tags.
type Handler func(conn net.Conn)

// getCurrentUserSID returns the SID of the current process's token user as a
// string in "S-1-5-..." form suitable for embedding in an SDDL descriptor.
func getCurrentUserSID() (string, error) {
	token := windows.GetCurrentProcessToken()
	user, err := token.GetTokenUser()
	if err != nil {
		return "", fmt.Errorf("GetTokenUser: %w", err)
	}
	return user.User.Sid.String(), nil
}

// NewServer creates a Windows named pipe at PipePath with a DACL restricting
// access to the current user's SID. The sockPath and ownerUID parameters are
// ignored on Windows: the pipe path is fixed to PipePath and the DACL is
// derived from the current process token.
//
// The SDDL format used is:
//
//	D:(A;;GRGW;;;<SID>)
//
// where:
//   - D: = Discretionary ACL (DACL)
//   - A  = Access Allowed ACE
//   - GRGW = GENERIC_READ | GENERIC_WRITE
//   - <SID> = the installing user's SID string
func NewServer(sockPath string, ownerUID uint32) (*Server, error) {
	_ = sockPath
	_ = ownerUID

	sid, err := getCurrentUserSID()
	if err != nil {
		return nil, fmt.Errorf("get current user SID: %w", err)
	}

	// Build the SDDL DACL: allow GENERIC_READ|GENERIC_WRITE for the current user only.
	sddl := fmt.Sprintf("D:(A;;GRGW;;;%s)", sid)

	l, err := winio.ListenPipe(PipePath, &winio.PipeConfig{
		SecurityDescriptor: sddl,
		MessageMode:        false, // byte mode — same as Unix sockets
		InputBufferSize:    65536,
		OutputBufferSize:   65536,
	})
	if err != nil {
		return nil, fmt.Errorf("listen pipe: %w", err)
	}
	return &Server{listener: l, sockPath: sockPath}, nil
}

// Serve accepts connections in a loop. Each connection is dispatched to
// handler in a new goroutine. Serve returns when ctx is cancelled.
//
// Authentication is enforced at the OS level by the DACL set on the pipe.
// There is no application-level peer credential check needed.
func (s *Server) Serve(ctx context.Context, handler Handler) error {
	go func() {
		<-ctx.Done()
		s.listener.Close()
	}()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		go handler(conn)
	}
}

// Close shuts down the listener.
func (s *Server) Close() error {
	return s.listener.Close()
}

// Connect dials the Beekeeper Sentry named pipe with the given timeout.
// The sockPath parameter is ignored on Windows; PipePath is always used.
func Connect(sockPath string, timeout time.Duration) (net.Conn, error) {
	_ = sockPath
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return winio.DialPipeContext(ctx, PipePath)
}

// SendCommand encodes cmd and writes it to conn. A write deadline equal to
// timeout from now is applied before the write.
func SendCommand(conn net.Conn, cmd IPCCommand, timeout time.Duration) error {
	_ = conn.SetWriteDeadline(time.Now().Add(timeout))
	return Encode(conn, cmd)
}

// ReadResponse reads and decodes a single IPCResponse from conn. A read
// deadline equal to timeout from now is applied before the read.
func ReadResponse(conn net.Conn, timeout time.Duration) (IPCResponse, error) {
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	var resp IPCResponse
	err := Decode(conn, &resp)
	return resp, err
}
