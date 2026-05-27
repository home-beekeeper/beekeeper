// Package ipc implements the Unix socket IPC protocol used between the
// Beekeeper CLI and the Sentry daemon. Communication uses a simple
// length-prefixed JSON framing: a 4-byte big-endian uint32 length followed
// by that many bytes of JSON.
package ipc

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
)

// CommandKind identifies the type of IPC command.
type CommandKind string

const (
	CmdStatusRequest       CommandKind = "status_request"
	CmdRulesListRequest    CommandKind = "rules_list_request"
	CmdRulesEnableRequest  CommandKind = "rules_enable_request"
	CmdRulesDisableRequest CommandKind = "rules_disable_request"
)

// IPCCommand is sent from the CLI to the Sentry daemon.
type IPCCommand struct {
	Kind   CommandKind `json:"kind"`
	RuleID string      `json:"rule_id,omitempty"`
}

// IPCResponse is sent from the Sentry daemon back to the CLI.
type IPCResponse struct {
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// StatusResponse is the payload for a CmdStatusRequest response.
type StatusResponse struct {
	DaemonPID        int    `json:"daemon_pid"`
	Uptime           string `json:"uptime"`
	Tier             int    `json:"tier"`
	TierReason       string `json:"tier_reason"`
	RulesActive      int    `json:"rules_active"`
	EventsProcessed  uint64 `json:"events_processed"`
	EventsDropped    uint64 `json:"events_dropped"`
	BaselineActive   bool   `json:"baseline_active"`
	BaselineDaysLeft int    `json:"baseline_days_left"`
	SockPath         string `json:"sock_path"`
}

// RuleInfo describes a single Sentry rule.
type RuleInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Enabled  bool   `json:"enabled"`
	Severity string `json:"severity"`
}

// RulesListResponse is the payload for a CmdRulesListRequest response.
type RulesListResponse struct {
	Rules []RuleInfo `json:"rules"`
}

// ErrMessageTooLarge is returned when a message exceeds the 64 KB limit.
var ErrMessageTooLarge = errors.New("ipc: message exceeds 64KB limit")

// ErrNotSupported is returned on platforms where the IPC socket is not
// available (Windows).
var ErrNotSupported = errors.New("ipc: not supported on this platform")

// maxMessageSize is the hard cap on any single IPC message (64 KiB).
const maxMessageSize = 64 * 1024

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
