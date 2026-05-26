//go:build fuzz

// RELEASE GATE: This file is required to exist for Beekeeper v0.6.0 release.
// FuzzParseMessage must pass (seed corpus run) in CI before any release tag.
// Run: go test -tags fuzz -run=FuzzParseMessage ./internal/gateway/...
// Fuzz: go test -tags fuzz -fuzz=FuzzParseMessage -fuzztime=60s ./internal/gateway/...

package gateway

import (
	"testing"
)

// FuzzParseMessage is the RELEASE GATE fuzz test for the MCP message parser.
//
// Contract: ParseMessage must NEVER panic on any input and must ALWAYS return
// either a valid JSONRPCMessage (with JSONRPC=="2.0") or a typed *ParseError
// with Code != 0. No untyped panics, no out-of-bounds, no infinite loops.
//
// This fuzz target covers all bounds enforced by ParseMessage:
//   - Empty/whitespace-only input → ParseError{-32700}
//   - Invalid JSON → ParseError{-32700}
//   - Wrong JSONRPC version → ParseError{-32600}
//   - Method name > 256 bytes → ParseError{-32600}
//   - Batch > 50 items → ParseError{-32600}
//   - Params nesting depth > 10 → ParseError{-32600}
func FuzzParseMessage(f *testing.F) {
	// Seed corpus: representative and adversarial inputs covering all code paths.

	// Valid tools/call request
	f.Add([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"x","arguments":{}}}`))

	// Null id (notification-style)
	f.Add([]byte(`{"jsonrpc":"2.0","id":null,"method":"initialize","params":{}}`))

	// Batch with single item
	f.Add([]byte(`[{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{}}]`))

	// Empty input
	f.Add([]byte(``))

	// Empty JSON object (missing required fields)
	f.Add([]byte(`{}`))

	// Empty array
	f.Add([]byte(`[]`))

	// Null literal
	f.Add([]byte(`null`))

	// Oversized method name (257 bytes — exceeds maxMethodLen=256)
	oversizedMethod := make([]byte, 257)
	for i := range oversizedMethod {
		oversizedMethod[i] = 'a'
	}
	f.Add([]byte(`{"jsonrpc":"2.0","id":1,"method":"` + string(oversizedMethod) + `"}`))

	// Wrong jsonrpc version
	f.Add([]byte(`{"jsonrpc":"1.0","id":1,"method":"tools/call"}`))

	// Deep nesting: depth=11 (exceeds maxRecursionDepth=10)
	f.Add([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"a":{"b":{"c":{"d":{"e":{"f":{"g":{"h":{"i":{"j":{"k":"deep"}}}}}}}}}}}}`))

	// Large batch: 51 items (exceeds maxBatchItems=50)
	largeBatch := make([]byte, 0, 4096)
	largeBatch = append(largeBatch, '[')
	for i := 0; i < 51; i++ {
		if i > 0 {
			largeBatch = append(largeBatch, ',')
		}
		largeBatch = append(largeBatch, []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)...)
	}
	largeBatch = append(largeBatch, ']')
	f.Add(largeBatch)

	f.Fuzz(func(t *testing.T, data []byte) {
		// ParseMessage must never panic regardless of input.
		// Every return path must satisfy exactly one of:
		//   (a) err == nil AND msg.JSONRPC == "2.0"
		//   (b) err != nil AND err is *ParseError AND pe.Code != 0
		msg, err := ParseMessage(data)
		if err != nil {
			pe, ok := err.(*ParseError)
			if !ok {
				t.Errorf("ParseMessage returned non-ParseError: %T: %v", err, err)
				return
			}
			if pe.Code == 0 {
				t.Errorf("ParseError has zero Code for input %q", data)
			}
			return
		}
		// Success path: basic invariants
		if msg.JSONRPC != "2.0" {
			t.Errorf("ParseMessage returned non-2.0 jsonrpc: %q for input %q", msg.JSONRPC, data)
		}
	})
}
