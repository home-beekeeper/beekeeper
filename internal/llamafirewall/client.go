//go:build linux || darwin

package llamafirewall

import (
	"net"
	"time"
)

// Client is an IPC client that communicates with the LlamaFirewall Python sidecar
// over a Unix domain socket using the length-prefixed JSON framing defined in
// proto.go.
type Client struct {
	conn    net.Conn
	timeout time.Duration
}

// Dial connects to the LlamaFirewall sidecar at sockPath with the given timeout.
// Returns an initialised *Client ready for Scan calls.
func Dial(sockPath string, timeout time.Duration) (*Client, error) {
	conn, err := net.DialTimeout("unix", sockPath, timeout)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, timeout: timeout}, nil
}

// Scan sends req to the sidecar and returns the ScanResponse. Deadlines are set
// for both the write and read phases using the Client's timeout.
func (c *Client) Scan(req ScanRequest) (ScanResponse, error) {
	_ = c.conn.SetWriteDeadline(time.Now().Add(c.timeout))
	if err := Encode(c.conn, req); err != nil {
		return ScanResponse{}, err
	}
	_ = c.conn.SetReadDeadline(time.Now().Add(c.timeout))
	var resp ScanResponse
	if err := Decode(c.conn, &resp); err != nil {
		return ScanResponse{}, err
	}
	return resp, nil
}

// Close closes the underlying Unix socket connection.
func (c *Client) Close() error { return c.conn.Close() }
