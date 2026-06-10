package llamafirewall

import (
	"net"
	"time"
)

// Client is an IPC client that communicates with the LlamaFirewall Python
// sidecar over a loopback TCP connection using the length-prefixed JSON framing
// defined in proto.go.
//
// Every Scan request carries the per-launch bearer token; the sidecar rejects a
// mismatch. The token restores the access control the old 0600 unix socket gave
// now that the transport is loopback TCP, and the single TCP transport replaces
// the former unix-socket (Linux/macOS) + named-pipe (Windows) fork (Phase 20,
// LLMF).
type Client struct {
	conn    net.Conn
	token   string
	timeout time.Duration
}

// Dial connects to the LlamaFirewall sidecar at addr (a 127.0.0.1:<port>
// loopback address) with the given per-launch bearer token and timeout. Returns
// an initialised *Client ready for Scan calls.
func Dial(addr, token string, timeout time.Duration) (*Client, error) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, token: token, timeout: timeout}, nil
}

// Scan sends req to the sidecar and returns the ScanResponse. The per-launch
// bearer token is stamped onto every request before sending. Deadlines are set
// for both the write and read phases using the Client's timeout.
func (c *Client) Scan(req ScanRequest) (ScanResponse, error) {
	req.Token = c.token
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

// Close closes the underlying TCP connection.
func (c *Client) Close() error { return c.conn.Close() }
