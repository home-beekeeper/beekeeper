package llamafirewall

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

// TestDecodeRoundTripScanPrompt verifies Encode/Decode round-trip for a
// ScanRequest with Kind=ScanPrompt.
func TestDecodeRoundTripScanPrompt(t *testing.T) {
	req := ScanRequest{
		Kind:      ScanPrompt,
		Content:   "test",
		RequestID: "r1",
	}

	var buf bytes.Buffer
	if err := Encode(&buf, req); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	var got ScanRequest
	if err := Decode(&buf, &got); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if got.Kind != req.Kind {
		t.Errorf("Kind: got %q, want %q", got.Kind, req.Kind)
	}
	if got.Content != req.Content {
		t.Errorf("Content: got %q, want %q", got.Content, req.Content)
	}
	if got.RequestID != req.RequestID {
		t.Errorf("RequestID: got %q, want %q", got.RequestID, req.RequestID)
	}
}

// TestDecodeRoundTripScanCode verifies Encode/Decode round-trip for a
// ScanRequest with Kind=ScanCode.
func TestDecodeRoundTripScanCode(t *testing.T) {
	req := ScanRequest{
		Kind:      ScanCode,
		Content:   "func x(){}",
		RequestID: "r2",
	}

	var buf bytes.Buffer
	if err := Encode(&buf, req); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	var got ScanRequest
	if err := Decode(&buf, &got); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if got.Kind != req.Kind {
		t.Errorf("Kind: got %q, want %q", got.Kind, req.Kind)
	}
	if got.Content != req.Content {
		t.Errorf("Content: got %q, want %q", got.Content, req.Content)
	}
	if got.RequestID != req.RequestID {
		t.Errorf("RequestID: got %q, want %q", got.RequestID, req.RequestID)
	}
}

// TestDecodeRoundTripScanCodeWithToken verifies Encode/Decode round-trip for a
// ScanRequest with Kind=ScanCode and a bearer token (Phase 20, LLMF).
func TestDecodeRoundTripScanCodeWithToken(t *testing.T) {
	req := ScanRequest{
		Kind:      ScanCode,
		Content:   "thought",
		RequestID: "r3",
		Token:     "secret-token",
	}

	var buf bytes.Buffer
	if err := Encode(&buf, req); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	var got ScanRequest
	if err := Decode(&buf, &got); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if got.Kind != req.Kind {
		t.Errorf("Kind: got %q, want %q", got.Kind, req.Kind)
	}
	if got.Content != req.Content {
		t.Errorf("Content: got %q, want %q", got.Content, req.Content)
	}
	if got.RequestID != req.RequestID {
		t.Errorf("RequestID: got %q, want %q", got.RequestID, req.RequestID)
	}
	if got.Token != req.Token {
		t.Errorf("Token: got %q, want %q", got.Token, req.Token)
	}
}

// TestDecodeRoundTripScanResponse verifies Encode/Decode round-trip for a
// ScanResponse.
func TestDecodeRoundTripScanResponse(t *testing.T) {
	resp := ScanResponse{
		RequestID:  "r1",
		Result:     ResultInjection,
		Confidence: 0.97,
		LatencyMS:  42,
	}

	var buf bytes.Buffer
	if err := Encode(&buf, resp); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	var got ScanResponse
	if err := Decode(&buf, &got); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if got.RequestID != resp.RequestID {
		t.Errorf("RequestID: got %q, want %q", got.RequestID, resp.RequestID)
	}
	if got.Result != resp.Result {
		t.Errorf("Result: got %q, want %q", got.Result, resp.Result)
	}
	if got.Confidence != resp.Confidence {
		t.Errorf("Confidence: got %v, want %v", got.Confidence, resp.Confidence)
	}
	if got.LatencyMS != resp.LatencyMS {
		t.Errorf("LatencyMS: got %v, want %v", got.LatencyMS, resp.LatencyMS)
	}
}

// TestDecodeTooLarge verifies that Decode returns ErrMessageTooLarge when the
// 4-byte length prefix declares a payload larger than 1 MB.
func TestDecodeTooLarge(t *testing.T) {
	var buf bytes.Buffer
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(1024*1024+1))
	buf.Write(hdr[:])
	buf.Write([]byte{0x01, 0x02, 0x03, 0x04})

	var req ScanRequest
	err := Decode(&buf, &req)
	if err != ErrMessageTooLarge {
		t.Errorf("expected ErrMessageTooLarge, got %v", err)
	}
}

// TestDecodeTruncated verifies that Decode returns an error when the actual
// payload is shorter than the declared length.
func TestDecodeTruncated(t *testing.T) {
	var buf bytes.Buffer
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], 100)
	buf.Write(hdr[:])
	// Write only 10 bytes when 100 are expected.
	buf.Write(make([]byte, 10))

	var req ScanRequest
	err := Decode(&buf, &req)
	if err == nil {
		t.Error("expected error for truncated payload, got nil")
	}
}

// TestDecodeInvalidJSON verifies that Decode returns a JSON error when the
// payload is syntactically invalid JSON.
func TestDecodeInvalidJSON(t *testing.T) {
	payload := []byte("{invalid}")
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(payload)))

	var buf bytes.Buffer
	buf.Write(hdr[:])
	buf.Write(payload)

	var req ScanRequest
	err := Decode(&buf, &req)
	if err == nil {
		t.Error("expected JSON error for invalid payload, got nil")
	}
}

// TestEncodeNearLimit verifies that Encode succeeds for a payload just under
// the 1 MB limit.
func TestEncodeNearLimit(t *testing.T) {
	req := ScanRequest{
		Kind:      ScanPrompt,
		Content:   strings.Repeat("x", 1024*1024-200),
		RequestID: "near",
	}

	var buf bytes.Buffer
	err := Encode(&buf, req)
	if err != nil {
		t.Errorf("expected nil error for near-limit payload, got %v", err)
	}
}

// TestEncodeOverLimit verifies that Encode returns ErrMessageTooLarge for a
// payload that exceeds the 1 MB limit.
func TestEncodeOverLimit(t *testing.T) {
	req := ScanRequest{
		Kind:      ScanPrompt,
		Content:   strings.Repeat("x", 1024*1024+100),
		RequestID: "over",
	}

	var buf bytes.Buffer
	err := Encode(&buf, req)
	if err != ErrMessageTooLarge {
		t.Errorf("expected ErrMessageTooLarge, got %v", err)
	}
}
