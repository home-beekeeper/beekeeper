// Package llamafirewall implements the IPC protocol used between the Go
// supervisor and the LlamaFirewall Python sidecar. Communication uses a simple
// length-prefixed JSON framing: a 4-byte big-endian uint32 length followed by
// that many bytes of JSON.
//
// The protocol is identical to internal/ipc framing except the message size
// cap is 1 MB (vs 64 KB for IPC), accommodating larger prompt/code payloads
// sent to LlamaFirewall for alignment and injection scanning.
package llamafirewall

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
)

// ScanKind identifies the type of scan requested from LlamaFirewall.
type ScanKind string

const (
	// ScanPrompt requests a prompt-injection scan.
	ScanPrompt ScanKind = "scan_prompt"
	// ScanCode requests a code-safety scan.
	ScanCode ScanKind = "scan_code"
)

// ScanRequest is sent from the Go supervisor to the LlamaFirewall sidecar.
type ScanRequest struct {
	Kind      ScanKind `json:"kind"`
	Content   string   `json:"content"`
	Context   string   `json:"context,omitempty"`
	RequestID string   `json:"request_id"`
	// Token is the per-launch bearer token (Phase 20, LLMF). The supervisor sets
	// it on every request; the sidecar rejects a mismatch. It restores the
	// access control the old 0600 unix socket gave now that IPC is loopback TCP.
	Token string `json:"token,omitempty"`
}

// ScanResult is the top-level verdict returned by LlamaFirewall.
type ScanResult string

const (
	// ResultClean indicates no threat detected.
	ResultClean ScanResult = "clean"
	// ResultInjection indicates a prompt-injection attack detected.
	ResultInjection ScanResult = "injection"
	// ResultUnsafe indicates unsafe code or tool use detected.
	ResultUnsafe ScanResult = "unsafe"
	// ResultError indicates the sidecar could not complete the scan (model
	// missing, import error, crash). The Go layer treats it fail-closed (block),
	// never as clean (Phase 20, LLMF — replaces the old swallow-into-clean).
	ResultError ScanResult = "error"
)

// ScanResponse is returned from the LlamaFirewall sidecar to the Go supervisor.
type ScanResponse struct {
	RequestID  string     `json:"request_id"`
	Result     ScanResult `json:"result"`
	Confidence float64    `json:"confidence"`
	Reason     string     `json:"reason,omitempty"`
	LatencyMS  int64      `json:"latency_ms"`
	Error      string     `json:"error,omitempty"`
}

// ErrMessageTooLarge is returned when a message exceeds the 1 MB limit.
var ErrMessageTooLarge = errors.New("llamafirewall: message exceeds 1MB limit")

// ErrNotSupported is returned on platforms where LlamaFirewall is not available.
var ErrNotSupported = errors.New("llamafirewall: not supported on this platform")

// maxMessageSize is the hard cap on any single LlamaFirewall IPC message (1 MiB).
// LlamaFirewall payloads can be significantly larger than CLI IPC messages
// (which are capped at 64 KB) due to full prompt/code content being transmitted.
const maxMessageSize = 1024 * 1024 // 1MB

// Encode marshals v to JSON and writes it to w with a 4-byte big-endian
// uint32 length prefix. Returns ErrMessageTooLarge if the JSON representation
// exceeds maxMessageSize.
func Encode(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(data) > maxMessageSize {
		return ErrMessageTooLarge
	}

	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(data)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// Decode reads a length-prefixed JSON message from r and unmarshals it into v.
// Returns ErrMessageTooLarge if the declared length exceeds maxMessageSize.
// Uses io.ReadFull for both the header and payload reads.
func Decode(r io.Reader, v any) error {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return err
	}

	size := binary.BigEndian.Uint32(hdr[:])
	if size > maxMessageSize {
		return ErrMessageTooLarge
	}

	payload := make([]byte, size)
	if _, err := io.ReadFull(r, payload); err != nil {
		return err
	}

	return json.Unmarshal(payload, v)
}
