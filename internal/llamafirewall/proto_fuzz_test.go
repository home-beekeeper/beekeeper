//go:build linux

// RELEASE GATE: This file is required to exist for Beekeeper v0.6.0 release.
// FuzzLlamaFirewallProto must pass (seed corpus run) in CI before any release tag.
// Run: go test -tags linux -run=FuzzLlamaFirewallProto ./internal/llamafirewall/...
// Fuzz: go test -tags linux -fuzz=FuzzLlamaFirewallProto -fuzztime=60s ./internal/llamafirewall/...

package llamafirewall

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// FuzzLlamaFirewallProto is the RELEASE GATE fuzz test for the LlamaFirewall
// IPC framing layer.
//
// Contract: Decode must NEVER panic on any input. Any error is acceptable
// (ErrMessageTooLarge, io.ErrUnexpectedEOF, json.SyntaxError).
// Encode on a constructed ScanRequest must NEVER panic regardless of content.
func FuzzLlamaFirewallProto(f *testing.F) {
	// Seed 1: valid encoded ScanRequest.
	var seed1 bytes.Buffer
	_ = Encode(&seed1, ScanRequest{Kind: ScanPrompt, Content: "hello", RequestID: "x"})
	f.Add(seed1.Bytes())

	// Seed 2: zero-length buffer.
	f.Add([]byte{})

	// Seed 3: oversized prefix (0xFFFFFFFF) + 4 data bytes — triggers ErrMessageTooLarge.
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x01, 0x02, 0x03, 0x04})

	// Seed 4: length prefix = 100, only 1 trailing byte — triggers io.ErrUnexpectedEOF.
	var hdr4 [4]byte
	binary.BigEndian.PutUint32(hdr4[:], 100)
	f.Add(append(hdr4[:], byte(0x00)))

	// Seed 5: valid ScanRequest JSON encoded with Encode framing.
	var seed5 bytes.Buffer
	_ = Encode(&seed5, ScanRequest{
		Kind:      ScanPrompt,
		Content:   "hello",
		RequestID: "x",
	})
	f.Add(seed5.Bytes())

	f.Fuzz(func(t *testing.T, data []byte) {
		// Decode must never panic; any error is acceptable.
		var req ScanRequest
		_ = Decode(bytes.NewReader(data), &req)

		// Encode on a constructed ScanRequest derived from fuzz fields must not panic.
		testReq := ScanRequest{
			Kind:      ScanKind(req.Kind),
			Content:   req.Content,
			RequestID: req.RequestID,
		}
		var buf bytes.Buffer
		_ = Encode(&buf, testReq)
	})
}
