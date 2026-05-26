package gateway

import (
	"strings"
	"testing"
)

func TestParseMessage(t *testing.T) {
	t.Run("valid cases", func(t *testing.T) {
		tests := []struct {
			name        string
			input       []byte
			wantMethod  string
			wantID      any
			wantIDType  string // "string", "float64", "nil"
		}{
			{
				name:       "valid tools/call",
				input:      []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"x","arguments":{}}}`),
				wantMethod: "tools/call",
				wantIDType: "float64",
			},
			{
				name:       "null id",
				input:      []byte(`{"jsonrpc":"2.0","id":null,"method":"tools/list","params":{}}`),
				wantMethod: "tools/list",
				wantIDType: "nil",
			},
			{
				name:       "string id",
				input:      []byte(`{"jsonrpc":"2.0","id":"req-abc","method":"tools/call","params":{}}`),
				wantMethod: "tools/call",
				wantIDType: "string",
			},
			{
				name:       "integer id (decoded as float64 by json.Unmarshal into any)",
				input:      []byte(`{"jsonrpc":"2.0","id":42,"method":"ping","params":{}}`),
				wantMethod: "ping",
				wantIDType: "float64",
			},
			{
				name:       "non-tools/call method",
				input:      []byte(`{"jsonrpc":"2.0","id":1,"method":"resources/list"}`),
				wantMethod: "resources/list",
				wantIDType: "float64",
			},
			{
				name:       "batch with single item",
				input:      []byte(`[{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{}}]`),
				wantMethod: "tools/call",
				wantIDType: "float64",
			},
			{
				name:       "method at exactly 256 bytes",
				input:      []byte(`{"jsonrpc":"2.0","id":1,"method":"` + strings.Repeat("a", 256) + `"}`),
				wantMethod: strings.Repeat("a", 256),
				wantIDType: "float64",
			},
			{
				name:       "batch with single item (allowed)",
				input:      []byte(`[{"jsonrpc":"2.0","id":2,"method":"ping","params":{}}]`),
				wantMethod: "ping",
				wantIDType: "float64",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				msg, err := ParseMessage(tc.input)
				if err != nil {
					t.Fatalf("ParseMessage returned unexpected error: %v", err)
				}
				if msg.JSONRPC != "2.0" {
					t.Errorf("JSONRPC = %q, want %q", msg.JSONRPC, "2.0")
				}
				if tc.wantMethod != "" && msg.Method != tc.wantMethod {
					t.Errorf("Method = %q, want %q", msg.Method, tc.wantMethod)
				}
				switch tc.wantIDType {
				case "nil":
					if msg.ID != nil {
						t.Errorf("ID = %v (%T), want nil", msg.ID, msg.ID)
					}
				case "string":
					if _, ok := msg.ID.(string); !ok {
						t.Errorf("ID type = %T, want string", msg.ID)
					}
				case "float64":
					if _, ok := msg.ID.(float64); !ok {
						t.Errorf("ID type = %T, want float64", msg.ID)
					}
				}
			})
		}
	})

	t.Run("error cases", func(t *testing.T) {
		tests := []struct {
			name      string
			input     []byte
			wantCode  int
		}{
			{
				name:     "empty input",
				input:    []byte{},
				wantCode: -32700,
			},
			{
				name:     "whitespace only",
				input:    []byte("   \t\n   "),
				wantCode: -32700,
			},
			{
				name:     "malformed JSON",
				input:    []byte("not json"),
				wantCode: -32700,
			},
			{
				name:     "wrong jsonrpc version 1.0",
				input:    []byte(`{"jsonrpc":"1.0","id":1,"method":"tools/call"}`),
				wantCode: -32600,
			},
			{
				name:     "missing jsonrpc field",
				input:    []byte(`{"id":1,"method":"tools/call"}`),
				wantCode: -32600,
			},
			{
				name:     "method name of 257 bytes (exceeds 256)",
				input:    []byte(`{"jsonrpc":"2.0","id":1,"method":"` + strings.Repeat("x", 257) + `"}`),
				wantCode: -32600,
			},
			{
				name:     "batch with 2 items (multi-item batch not supported)",
				input:    buildBatch(2),
				wantCode: -32600,
			},
			{
				name:     "batch with 50 items (at count limit, but multi-item not supported)",
				input:    buildBatch(50),
				wantCode: -32600,
			},
			{
				name:     "batch with 51 items (exceeds 50)",
				input:    buildBatch(51),
				wantCode: -32600,
			},
			{
				name:     "object nesting depth 11 (exceeds 10)",
				input:    buildDeepNested(11),
				wantCode: -32600,
			},
			{
				name:     "empty batch array",
				input:    []byte(`[]`),
				wantCode: -32600,
			},
			{
				name:     "invalid batch JSON",
				input:    []byte(`[not json]`),
				wantCode: -32700,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				msg, err := ParseMessage(tc.input)
				if err == nil {
					t.Fatalf("ParseMessage returned no error, got msg=%+v; want ParseError{%d}", msg, tc.wantCode)
				}
				pe, ok := err.(*ParseError)
				if !ok {
					t.Fatalf("error type = %T, want *ParseError", err)
				}
				if pe.Code != tc.wantCode {
					t.Errorf("ParseError.Code = %d, want %d (msg=%q)", pe.Code, tc.wantCode, pe.Msg)
				}
				if pe.Code == 0 {
					t.Errorf("ParseError.Code must never be 0 (fuzz invariant)")
				}
			})
		}
	})

	t.Run("ID type preservation", func(t *testing.T) {
		t.Run("integer id preserved as float64", func(t *testing.T) {
			msg, err := ParseMessage([]byte(`{"jsonrpc":"2.0","id":123,"method":"ping"}`))
			if err != nil {
				t.Fatal(err)
			}
			f, ok := msg.ID.(float64)
			if !ok {
				t.Fatalf("ID type = %T, want float64", msg.ID)
			}
			if f != 123 {
				t.Errorf("ID value = %v, want 123", f)
			}
		})

		t.Run("string id preserved as string", func(t *testing.T) {
			msg, err := ParseMessage([]byte(`{"jsonrpc":"2.0","id":"hello","method":"ping"}`))
			if err != nil {
				t.Fatal(err)
			}
			s, ok := msg.ID.(string)
			if !ok {
				t.Fatalf("ID type = %T, want string", msg.ID)
			}
			if s != "hello" {
				t.Errorf("ID value = %q, want %q", s, "hello")
			}
		})

		t.Run("null id preserved as nil", func(t *testing.T) {
			msg, err := ParseMessage([]byte(`{"jsonrpc":"2.0","id":null,"method":"ping"}`))
			if err != nil {
				t.Fatal(err)
			}
			if msg.ID != nil {
				t.Errorf("ID = %v (%T), want nil", msg.ID, msg.ID)
			}
		})
	})
}

// buildBatch creates a JSON-RPC batch array with n items.
func buildBatch(n int) []byte {
	var b []byte
	b = append(b, '[')
	for i := 0; i < n; i++ {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)...)
	}
	b = append(b, ']')
	return b
}

// buildDeepNested creates a JSON-RPC message whose params are nested at the
// given depth. depth=11 exceeds maxRecursionDepth=10.
func buildDeepNested(depth int) []byte {
	// Build nested JSON: {"a":{"a":{"a":...{}}...}}
	params := `{}`
	for i := 0; i < depth; i++ {
		params = `{"a":` + params + `}`
	}
	return []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":` + params + `}`)
}
