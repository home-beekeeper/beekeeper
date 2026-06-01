---
plan: 05-01
status: complete
wave: 1
---
# 05-01 Summary: IPC Infrastructure

## Artifacts
- internal/ipc/proto.go — IPCCommand/IPCResponse types; Encode/Decode with 4-byte length prefix + 64KB cap
- internal/ipc/server.go — Unix socket server with SO_PEERCRED UID verification (linux/darwin)
- internal/ipc/client.go — Connect/SendCommand/ReadResponse with deadlines (linux/darwin)
- internal/ipc/stub.go — ErrNotSupported stubs for Windows
- internal/ipc/proto_test.go — framing round-trip tests (passes on Windows)
- internal/ipc/proto_fuzz_test.go — FuzzIPCMessage release gate (linux CI)

## Verification
- go test ./internal/ipc/... passes (6/6 tests: TestEncodeDecodeCmdRoundTrip x4, TestEncodeDecodeStatusResponse, TestDecodeTooLarge, TestDecodeTruncated, TestDecodeInvalidJSON, TestEncodeNearLimit)
- go build ./... passes
- GetsockoptUcred pattern in server.go confirmed
- ErrNotSupported in stub.go confirmed
- binary.BigEndian in proto.go confirmed
- go vet ./internal/ipc/... passes with no output

## Notes
- golang.org/x/sys v0.30.0 was already an indirect dependency; no go.mod changes needed.
- The fuzz test uses `//go:build linux` (not `//go:build fuzz`) to match the specification's intent of linux-only CI gate; the seed corpus encodes deterministically via the Encode function itself.
- TestEncodeNearLimit uses a raw string value (json.Marshal of a string adds 2 bytes for quotes) to hit exactly maxMessageSize-1 bytes in the encoded payload.
