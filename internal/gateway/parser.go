// Package gateway implements the MCP gateway daemon — a stateless per-request
// HTTP proxy that applies the Beekeeper policy engine inline to every
// tools/call JSON-RPC request (INTG-03, INTG-04).
//
// This file contains the bounded JSON-RPC 2.0 parser that is the sole entry
// point for untrusted bytes. It is isolated here so that FuzzParseMessage in
// parser_fuzz_test.go can target it precisely without needing a running server.
package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// Parser bounds (INTG-03 spec).
const (
	maxRequestBody    = 1 << 20 // 1MB body cap (same as check.RunCheck stdin cap)
	maxMethodLen      = 256     // reject method names longer than 256 bytes
	maxBatchItems     = 50      // hard cap on JSON-RPC batch size
	maxRecursionDepth = 10      // max nesting depth for params validation
)

// JSONRPCMessage is a decoded JSON-RPC 2.0 request or response.
// The ID field uses `any` to correctly handle string, number, and null IDs
// (JSON-RPC 2.0 spec §5: id may be string, number, or null). Using `any`
// allows crypto/subtle comparison and echo-back without type-asserting the id
// (Pitfall 4: never use int or string for this field).
type JSONRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`          // always "2.0"
	ID      any             `json:"id"`               // string | number | null (any)
	Method  string          `json:"method,omitempty"` // request method
	Params  json.RawMessage `json:"params,omitempty"` // request params (deferred decode)
	Result  json.RawMessage `json:"result,omitempty"` // response result
	Error   *JSONRPCError   `json:"error,omitempty"`  // response error
}

// JSONRPCError is the standard JSON-RPC 2.0 error object.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// ParseError is returned by ParseMessage for all invalid inputs.
// Code is a standard JSON-RPC 2.0 error code:
//   - -32700: parse error (malformed JSON, empty body)
//   - -32600: invalid request (wrong version, oversized method, oversized batch, depth exceeded)
//
// ParseError.Code is always non-zero — the fuzz invariant requires this.
type ParseError struct {
	Code int
	Msg  string
}

func (e *ParseError) Error() string { return e.Msg }

// ParseMessage decodes a single JSON-RPC 2.0 request or the first item of a
// batch from b. All bounds are enforced before any caller-visible state is
// returned.
//
// Bounds enforced:
//   - empty body           → ParseError{-32700, "empty body"}
//   - invalid JSON         → ParseError{-32700, …}
//   - jsonrpc != "2.0"     → ParseError{-32600, …}
//   - method > 256 bytes   → ParseError{-32600, …}
//   - batch > 50 items     → ParseError{-32600, …}
//   - params depth > 10    → ParseError{-32600, …}
//
// ParseMessage never panics. Any return where err == nil guarantees
// msg.JSONRPC == "2.0". Any return where err != nil is a *ParseError with
// Code != 0 (the FuzzParseMessage invariant).
func ParseMessage(b []byte) (JSONRPCMessage, error) {
	if len(b) == 0 {
		return JSONRPCMessage{}, &ParseError{Code: -32700, Msg: "empty body"}
	}

	trimmed := bytes.TrimSpace(b)
	if len(trimmed) == 0 {
		return JSONRPCMessage{}, &ParseError{Code: -32700, Msg: "empty body after trim"}
	}

	if trimmed[0] == '[' {
		return parseAsBatch(trimmed)
	}
	return parseSingle(trimmed)
}

// parseSingle decodes a single JSON-RPC object and validates all bounds.
func parseSingle(b []byte) (JSONRPCMessage, error) {
	var msg JSONRPCMessage
	if err := json.Unmarshal(b, &msg); err != nil {
		return JSONRPCMessage{}, &ParseError{Code: -32700, Msg: "invalid JSON: " + err.Error()}
	}

	// Request-smuggling defense (T-04-03-11): reject any object in the request
	// tree that contains duplicate keys. Go's encoding/json silently takes the
	// LAST value for a duplicate key, but the gateway forwards the RAW bodyBytes
	// upstream — an upstream parser that chooses a different duplicate (e.g.
	// first-wins) would execute a DIFFERENT tool than the one Beekeeper evaluated
	// (parser/forwarder differential). A well-formed MCP tools/call never contains
	// duplicate keys, so this is a hard fail-closed reject, not a best-effort
	// heuristic. This runs AFTER json.Unmarshal so genuinely malformed JSON is
	// still reported as -32700 (parse error), while valid-but-duplicate JSON is
	// reported as -32600 (invalid request).
	if err := rejectDuplicateKeys(b); err != nil {
		return JSONRPCMessage{}, &ParseError{Code: -32600, Msg: "duplicate JSON key (request-smuggling guard): " + err.Error()}
	}

	if msg.JSONRPC != "2.0" {
		return JSONRPCMessage{}, &ParseError{Code: -32600, Msg: `jsonrpc must be "2.0"`}
	}
	if len(msg.Method) > maxMethodLen {
		return JSONRPCMessage{}, &ParseError{Code: -32600, Msg: fmt.Sprintf("method name exceeds %d bytes", maxMethodLen)}
	}
	if err := checkDepth(msg.Params, 0); err != nil {
		return JSONRPCMessage{}, &ParseError{Code: -32600, Msg: "params depth exceeds limit: " + err.Error()}
	}
	return msg, nil
}

// rejectDuplicateKeys walks the JSON value encoded in b using a streaming
// json.Decoder token scanner and returns an error if any JSON object anywhere
// in the value tree declares the same key twice. This closes the parser /
// forwarder differential that lets a duplicate-key payload smuggle a different
// tool name past policy evaluation (e.g. {"name":"safe","name":"shell_exec"}):
// Go's json.Unmarshal is last-wins, but the gateway forwards the raw bytes, so
// an upstream that disagrees on which duplicate wins would diverge.
//
// The scan covers the top-level request object AND the nested params object
// (and every other nested object/array) in a single pass. It never panics:
// any malformed JSON is reported as an error here and would be rejected by
// json.Unmarshal in parseSingle regardless.
func rejectDuplicateKeys(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	return scanDuplicateKeys(dec)
}

// scanDuplicateKeys consumes exactly one JSON value from dec. For objects it
// tracks the set of keys seen at that object's level and returns an error on
// the first repeat; for arrays it recurses into each element; scalars are
// consumed as-is. Nested objects/arrays are validated recursively so a
// duplicate key buried inside params (or any deeper object) is caught.
func scanDuplicateKeys(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}

	delim, ok := tok.(json.Delim)
	if !ok {
		// Scalar (string, number, bool, null) — nothing to recurse into.
		return nil
	}

	switch delim {
	case '{':
		seen := make(map[string]struct{})
		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				return err
			}
			key, ok := keyTok.(string)
			if !ok {
				// JSON object keys are always strings; anything else is malformed.
				return fmt.Errorf("non-string object key")
			}
			if _, dup := seen[key]; dup {
				return fmt.Errorf("key %q appears more than once in the same object", key)
			}
			seen[key] = struct{}{}
			// Recurse into the value for this key.
			if err := scanDuplicateKeys(dec); err != nil {
				return err
			}
		}
		// Consume the closing '}'.
		if _, err := dec.Token(); err != nil {
			return err
		}
	case '[':
		for dec.More() {
			if err := scanDuplicateKeys(dec); err != nil {
				return err
			}
		}
		// Consume the closing ']'.
		if _, err := dec.Token(); err != nil {
			return err
		}
	}
	return nil
}

// parseAsBatch decodes a JSON-RPC batch array and validates the item count.
//
// WR-07: batch requests with more than one item are rejected with a -32600
// error rather than silently processing only item[0] and dropping the rest.
// A client that sends [req1, req2, req3] would otherwise receive only a
// response for req1 with no indication that req2 and req3 were dropped,
// potentially causing the client to block waiting for responses that never
// arrive. Returning an explicit error is the correct behaviour when batch
// fan-out is not implemented.
//
// Single-item batches ([req1]) are accepted and processed normally since
// they have no ambiguity about dropped requests.
func parseAsBatch(b []byte) (JSONRPCMessage, error) {
	var batch []json.RawMessage
	if err := json.Unmarshal(b, &batch); err != nil {
		return JSONRPCMessage{}, &ParseError{Code: -32700, Msg: "invalid batch JSON: " + err.Error()}
	}
	if len(batch) == 0 {
		return JSONRPCMessage{}, &ParseError{Code: -32600, Msg: "empty batch"}
	}
	if len(batch) > maxBatchItems {
		return JSONRPCMessage{}, &ParseError{Code: -32600, Msg: fmt.Sprintf("batch exceeds %d items", maxBatchItems)}
	}
	// WR-07: reject multi-item batches explicitly rather than silently dropping
	// items 1..N-1. Clients sending a batch must be informed that batch fan-out
	// is not supported so they can switch to individual requests.
	if len(batch) > 1 {
		return JSONRPCMessage{}, &ParseError{Code: -32600, Msg: "batch requests are not supported by this gateway"}
	}
	return parseSingle(batch[0])
}

// checkDepth verifies that a JSON value encoded in raw does not exceed
// maxRecursionDepth levels of object/array nesting. depth is the current
// nesting level of the caller.
//
// If raw is empty or null, no check is needed. Unmarshal errors here are
// ignored — parseSingle catches them first.
func checkDepth(raw json.RawMessage, depth int) error {
	if depth > maxRecursionDepth {
		return fmt.Errorf("depth %d exceeds maximum %d", depth, maxRecursionDepth)
	}
	if len(raw) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil // unmarshal errors are reported by parseSingle
	}
	return checkValueDepth(v, depth)
}

// checkValueDepth recursively walks the decoded JSON value tree, counting
// nesting levels. It returns an error if any path exceeds maxRecursionDepth.
func checkValueDepth(v any, depth int) error {
	if depth > maxRecursionDepth {
		return fmt.Errorf("depth %d exceeds maximum %d", depth, maxRecursionDepth)
	}
	switch val := v.(type) {
	case map[string]any:
		for _, child := range val {
			if err := checkValueDepth(child, depth+1); err != nil {
				return err
			}
		}
	case []any:
		for _, item := range val {
			if err := checkValueDepth(item, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}
