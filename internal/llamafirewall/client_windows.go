//go:build windows

package llamafirewall

import (
	"fmt"
	"time"

	"golang.org/x/sys/windows"
)

// windowsPipePath is the fixed named-pipe path used on Windows.
// The sockPath argument to Dial is intentionally ignored on this platform.
const windowsPipePath = `\\.\pipe\beekeeper-llamafirewall`

// Client is an IPC client that communicates with the LlamaFirewall Python sidecar
// over a Windows named pipe using the length-prefixed JSON framing defined in
// proto.go.
type Client struct {
	handle  windows.Handle
	timeout time.Duration
}

// Dial opens the named pipe at windowsPipePath. sockPath is ignored on Windows.
// Returns an initialised *Client ready for Scan calls.
func Dial(sockPath string, timeout time.Duration) (*Client, error) {
	path, err := windows.UTF16PtrFromString(windowsPipePath)
	if err != nil {
		return nil, fmt.Errorf("encode pipe path: %w", err)
	}
	h, err := windows.CreateFile(
		path,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0, nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("open named pipe %s: %w", windowsPipePath, err)
	}
	return &Client{handle: h, timeout: timeout}, nil
}

// Scan sends req to the sidecar and returns the ScanResponse.
func (c *Client) Scan(req ScanRequest) (ScanResponse, error) {
	rw := &pipeReadWriter{handle: c.handle}
	if err := Encode(rw, req); err != nil {
		return ScanResponse{}, err
	}
	var resp ScanResponse
	if err := Decode(rw, &resp); err != nil {
		return ScanResponse{}, err
	}
	return resp, nil
}

// Close closes the named pipe handle.
func (c *Client) Close() error { return windows.CloseHandle(c.handle) }

// pipeReadWriter adapts a windows.Handle to the io.ReadWriter interface required
// by Encode/Decode.
type pipeReadWriter struct{ handle windows.Handle }

func (p *pipeReadWriter) Write(b []byte) (int, error) {
	var n uint32
	err := windows.WriteFile(p.handle, b, &n, nil)
	return int(n), err
}

func (p *pipeReadWriter) Read(b []byte) (int, error) {
	var n uint32
	err := windows.ReadFile(p.handle, b, &n, nil)
	return int(n), err
}
