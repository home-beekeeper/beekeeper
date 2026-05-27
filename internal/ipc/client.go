//go:build linux || darwin

package ipc

import (
	"fmt"
	"net"
	"time"
)

// Connect dials a Unix socket at sockPath with the given timeout.
func Connect(sockPath string, timeout time.Duration) (net.Conn, error) {
	conn, err := net.DialTimeout("unix", sockPath, timeout)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", sockPath, err)
	}
	return conn, nil
}

// SendCommand encodes cmd and writes it to conn. A write deadline equal to
// timeout from now is applied before the write.
func SendCommand(conn net.Conn, cmd IPCCommand, timeout time.Duration) error {
	if err := conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return fmt.Errorf("setting write deadline: %w", err)
	}
	return Encode(conn, cmd)
}

// ReadResponse reads and decodes a single IPCResponse from conn. A read
// deadline equal to timeout from now is applied before the read.
func ReadResponse(conn net.Conn, timeout time.Duration) (IPCResponse, error) {
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return IPCResponse{}, fmt.Errorf("setting read deadline: %w", err)
	}
	var resp IPCResponse
	if err := Decode(conn, &resp); err != nil {
		return IPCResponse{}, err
	}
	return resp, nil
}
