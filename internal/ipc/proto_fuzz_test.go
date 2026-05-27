//go:build linux

// RELEASE GATE: This file is required to exist for Beekeeper v0.6.0 release.
// FuzzIPCMessage must pass (seed corpus run) in CI before any release tag.
// Run: go test -run=FuzzIPCMessage ./internal/ipc/...
// Fuzz: go test -fuzz=FuzzIPCMessage -fuzztime=60s ./internal/ipc/...

package ipc

import (
	"bytes"
	"testing"
)

// FuzzIPCMessage is the RELEASE GATE fuzz test for the IPC framing layer.
//
// Contract: Decode must NEVER panic on any input. Any error is acceptable.
// Encode on a constructed IPCCommand must NEVER panic regardless of RuleID.
func FuzzIPCMessage(f *testing.F) {
	// Seed corpus — valid encoded StatusRequest.
	var validBuf bytes.Buffer
	_ = Encode(&validBuf, IPCCommand{Kind: CmdStatusRequest})
	f.Add(validBuf.Bytes())

	// Zero-length buffer.
	f.Add([]byte{})

	// Length prefix 0xFFFFFFFF + 4 random bytes (triggers ErrMessageTooLarge).
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x01, 0x02, 0x03, 0x04})

	// Length prefix 100 + only 1 byte (triggers io.ErrUnexpectedEOF).
	f.Add([]byte{0x00, 0x00, 0x00, 0x64, 0x78})

	// Valid length prefix + valid JSON with embedded null byte.
	nullPayload := []byte("\"hello\x00world\"")
	var nullHdr [4]byte
	nullHdr[0] = 0
	nullHdr[1] = 0
	nullHdr[2] = byte(len(nullPayload) >> 8)
	nullHdr[3] = byte(len(nullPayload))
	f.Add(append(nullHdr[:], nullPayload...))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Decode must never panic; any error is acceptable.
		var cmd IPCCommand
		_ = Decode(bytes.NewReader(data), &cmd)

		// Encode on a constructed command with fuzz-derived RuleID must not panic.
		ruleID := string(data)
		constructed := IPCCommand{Kind: CmdRulesEnableRequest, RuleID: ruleID}
		var buf bytes.Buffer
		_ = Encode(&buf, constructed)
	})
}
