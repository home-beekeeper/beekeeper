package ipc

import (
	"bytes"
	"encoding/binary"
	"io"
	"strings"
	"testing"
)

// TestEncodeDecodeCmdRoundTrip verifies that every CommandKind survives a
// full Encode → Decode round-trip with its Kind field preserved.
func TestEncodeDecodeCmdRoundTrip(t *testing.T) {
	kinds := []CommandKind{
		CmdStatusRequest,
		CmdRulesListRequest,
		CmdRulesEnableRequest,
		CmdRulesDisableRequest,
	}

	for _, k := range kinds {
		k := k
		t.Run(string(k), func(t *testing.T) {
			orig := IPCCommand{Kind: k, RuleID: "test-rule"}

			var buf bytes.Buffer
			if err := Encode(&buf, orig); err != nil {
				t.Fatalf("Encode: %v", err)
			}

			var got IPCCommand
			if err := Decode(&buf, &got); err != nil {
				t.Fatalf("Decode: %v", err)
			}

			if got.Kind != orig.Kind {
				t.Errorf("Kind: got %q, want %q", got.Kind, orig.Kind)
			}
			if got.RuleID != orig.RuleID {
				t.Errorf("RuleID: got %q, want %q", got.RuleID, orig.RuleID)
			}
		})
	}
}

// TestEncodeDecodeStatusResponse verifies that a StatusResponse with all
// non-zero fields round-trips without data loss.
func TestEncodeDecodeStatusResponse(t *testing.T) {
	orig := StatusResponse{
		DaemonPID:        12345,
		Uptime:           "1h30m",
		Tier:             2,
		TierReason:       "fanotify available",
		RulesActive:      42,
		EventsProcessed:  999999,
		EventsDropped:    7,
		BaselineActive:   true,
		BaselineDaysLeft: 28,
		SockPath:         "/run/beekeeper/sentry.sock",
	}

	var buf bytes.Buffer
	if err := Encode(&buf, orig); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	var got StatusResponse
	if err := Decode(&buf, &got); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if got.DaemonPID != orig.DaemonPID {
		t.Errorf("DaemonPID: got %d, want %d", got.DaemonPID, orig.DaemonPID)
	}
	if got.Uptime != orig.Uptime {
		t.Errorf("Uptime: got %q, want %q", got.Uptime, orig.Uptime)
	}
	if got.Tier != orig.Tier {
		t.Errorf("Tier: got %d, want %d", got.Tier, orig.Tier)
	}
	if got.TierReason != orig.TierReason {
		t.Errorf("TierReason: got %q, want %q", got.TierReason, orig.TierReason)
	}
	if got.RulesActive != orig.RulesActive {
		t.Errorf("RulesActive: got %d, want %d", got.RulesActive, orig.RulesActive)
	}
	if got.EventsProcessed != orig.EventsProcessed {
		t.Errorf("EventsProcessed: got %d, want %d", got.EventsProcessed, orig.EventsProcessed)
	}
	if got.EventsDropped != orig.EventsDropped {
		t.Errorf("EventsDropped: got %d, want %d", got.EventsDropped, orig.EventsDropped)
	}
	if got.BaselineActive != orig.BaselineActive {
		t.Errorf("BaselineActive: got %v, want %v", got.BaselineActive, orig.BaselineActive)
	}
	if got.BaselineDaysLeft != orig.BaselineDaysLeft {
		t.Errorf("BaselineDaysLeft: got %d, want %d", got.BaselineDaysLeft, orig.BaselineDaysLeft)
	}
	if got.SockPath != orig.SockPath {
		t.Errorf("SockPath: got %q, want %q", got.SockPath, orig.SockPath)
	}
}

// TestDecodeTooLarge verifies that Decode returns ErrMessageTooLarge when the
// declared payload length exceeds 64 KB.
func TestDecodeTooLarge(t *testing.T) {
	var buf bytes.Buffer
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(maxMessageSize+1))
	buf.Write(hdr[:])

	var v IPCCommand
	err := Decode(&buf, &v)
	if err != ErrMessageTooLarge {
		t.Fatalf("expected ErrMessageTooLarge, got %v", err)
	}
}

// TestDecodeTruncated verifies that Decode returns an error when the reader
// ends before the declared number of payload bytes are available.
func TestDecodeTruncated(t *testing.T) {
	var buf bytes.Buffer
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], 100) // claim 100 bytes
	buf.Write(hdr[:])
	buf.Write([]byte("only10byt")) // only 9 bytes (< 100)

	var v IPCCommand
	err := Decode(&buf, &v)
	if err == nil {
		t.Fatal("expected error for truncated payload, got nil")
	}
	if err == ErrMessageTooLarge {
		t.Fatalf("got ErrMessageTooLarge, expected io error (e.g. io.ErrUnexpectedEOF)")
	}
}

// TestDecodeInvalidJSON verifies that Decode returns a json-related error when
// the payload is not valid JSON.
func TestDecodeInvalidJSON(t *testing.T) {
	payload := []byte("{invalid}")
	var buf bytes.Buffer
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(payload)))
	buf.Write(hdr[:])
	buf.Write(payload)

	var v IPCCommand
	err := Decode(&buf, &v)
	if err == nil {
		t.Fatal("expected json error, got nil")
	}
	// Must be a json-related error, not ErrMessageTooLarge or EOF.
	if err == ErrMessageTooLarge || err == io.EOF || err == io.ErrUnexpectedEOF {
		t.Fatalf("expected json parse error, got %v", err)
	}
	if !strings.Contains(err.Error(), "invalid") && !strings.Contains(err.Error(), "json") {
		// Accept any non-nil json error (the exact message varies).
		// Just ensure it is not one of the known framing errors.
		t.Logf("json error (acceptable): %v", err)
	}
}

// TestEncodeNearLimit verifies that Encode accepts a payload of exactly
// maxMessageSize-1 bytes without returning ErrMessageTooLarge.
func TestEncodeNearLimit(t *testing.T) {
	// Build a JSON string whose marshalled representation is maxMessageSize-1
	// bytes. A JSON string with N chars marshals to N+2 bytes (the quotes).
	// We want the whole JSON value (a struct with one string field) to be
	// maxMessageSize-1, but for simplicity just encode a raw string directly
	// as a bytes.Buffer writer target via a map.
	//
	// Easier: construct a []byte payload whose json.Marshal is exactly
	// maxMessageSize-1 bytes. json.Marshal([]byte) produces base64, so
	// instead use a map[string]string.
	//
	// Simplest approach: encode a struct whose JSON is small, then verify that
	// a value whose JSON is exactly maxMessageSize-1 bytes succeeds.

	// A string of length N encodes to `"<N chars>"` = N+2 bytes in JSON.
	// We want the total JSON to be maxMessageSize-1.
	// Use a plain string value: json.Marshal(s) = `"..."` = len(s)+2.
	// So len(s) = maxMessageSize - 1 - 2 = maxMessageSize - 3.
	s := strings.Repeat("a", maxMessageSize-3)

	var buf bytes.Buffer
	if err := Encode(&buf, s); err != nil {
		t.Fatalf("Encode near limit: %v (buf len %d)", err, buf.Len())
	}
}
