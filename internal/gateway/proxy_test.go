package gateway

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mzansi-agentive/beekeeper/internal/policy"
)

// upstreamCallCount is an atomic counter used by tests that need to verify
// whether the upstream was called.
type callCounter struct {
	n atomic.Int64
}

func (c *callCounter) count() int {
	return int(c.n.Load())
}

// mockUpstream creates an httptest.Server that records calls and returns the
// given response body. It returns the server and a *callCounter.
func mockUpstream(t *testing.T, respBody string) (*httptest.Server, *callCounter) {
	t.Helper()
	cc := &callCounter{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cc.n.Add(1)
		// Read and discard request body.
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(respBody))
	}))
	t.Cleanup(srv.Close)
	return srv, cc
}

// toolsCallBody builds a tools/call JSON-RPC body for the given tool and package.
func toolsCallBody(id any, toolName, pkg string) []byte {
	b, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "tools/call",
		"params": map[string]any{
			"name": toolName,
			"arguments": map[string]any{
				"command": "npm install " + pkg,
			},
		},
	})
	return b
}

// postToolsCall sends a tools/call request to h for the given tool/package.
func postToolsCall(t *testing.T, h http.Handler, token string, id any, toolName, pkg string) *httptest.ResponseRecorder {
	t.Helper()
	body := toolsCallBody(id, toolName, pkg)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// TestGatewayBlocksToolCall verifies that a block decision returns -32001 and
// the upstream is NOT called (T-04-03-06: fail-closed invariant).
func TestGatewayBlocksToolCall(t *testing.T) {
	upstream, cc := mockUpstream(t, `{"jsonrpc":"2.0","id":1,"result":{}}`)
	h := newTestHandler("tok", blockIdx(), upstream.URL)

	rr := postToolsCall(t, h, "tok", 1, "Bash", "malicious-pkg")

	code := parseJSONRPCError(t, rr.Body.Bytes())
	if code != -32001 {
		t.Errorf("error code = %d, want -32001", code)
	}
	if cc.count() != 0 {
		t.Errorf("upstream was called %d times, want 0 (upstream must not be called on block)", cc.count())
	}
}

// TestGatewayAllowsToolCall verifies that an allow decision forwards to upstream
// and returns the upstream response.
func TestGatewayAllowsToolCall(t *testing.T) {
	upstreamResp := `{"jsonrpc":"2.0","id":1,"result":{"content":"ok"}}`
	upstream, cc := mockUpstream(t, upstreamResp)
	h := newTestHandler("tok", allowIdx(), upstream.URL)

	rr := postToolsCall(t, h, "tok", 1, "Bash", "safe-pkg")

	if cc.count() != 1 {
		t.Errorf("upstream call count = %d, want 1", cc.count())
	}

	// Response should come from upstream.
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not valid JSON: %v\nbody: %s", err, rr.Body.String())
	}
	if _, ok := resp["error"]; ok {
		t.Errorf("expected upstream response, got error: %s", rr.Body.String())
	}
}

// TestGatewayPassthrough verifies that non-tools/call methods are proxied
// transparently without policy evaluation.
func TestGatewayPassthrough(t *testing.T) {
	upstreamResp := `{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`
	upstream, cc := mockUpstream(t, upstreamResp)
	// Use blockIdx to confirm that policy is NOT evaluated for non-tool-call methods.
	// If policy were evaluated for tools/list, we'd expect -32001; we should see upstream.
	h := newTestHandler("tok", blockIdx(), upstream.URL)

	// Send tools/list — not a tools/call.
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if cc.count() != 1 {
		t.Errorf("upstream call count = %d, want 1 (tools/list must be proxied)", cc.count())
	}

	// Should not be an error response.
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not valid JSON: %v\nbody: %s", err, rr.Body.String())
	}
	if errField, ok := resp["error"]; ok {
		t.Errorf("tools/list got error response (policy must not evaluate): %v", errField)
	}
}

// TestGatewayFailClosed verifies that a policy engine panic results in -32002
// and the upstream is NEVER called (T-04-03-06).
func TestGatewayFailClosed(t *testing.T) {
	upstream, cc := mockUpstream(t, `{"jsonrpc":"2.0","id":1,"result":{}}`)

	// panicking idx causes a panic during LookupAll.
	panicIdx := &fakeIdx{
		fn: func(ecosystem, pkg string) []policy.CatalogMatch {
			panic("simulated policy engine panic")
		},
	}

	h := newTestHandler("tok", panicIdx, upstream.URL)

	rr := postToolsCall(t, h, "tok", 1, "Bash", "anything")

	// The panic recover must write -32002 to the client.
	code := parseJSONRPCError(t, rr.Body.Bytes())
	if code != -32002 {
		t.Errorf("error code = %d, want -32002 (fail-closed on panic)", code)
	}

	// Upstream must NOT have been called.
	if cc.count() != 0 {
		t.Errorf("upstream was called %d times, want 0 (fail-closed: upstream never called on panic)", cc.count())
	}
}

// TestGatewayIDCorrelation verifies that error responses echo the exact id field
// from the request — string, integer, and null (T-04-03-07).
func TestGatewayIDCorrelation(t *testing.T) {
	h := newTestHandler("tok", blockIdx(), "http://upstream-unused")

	tests := []struct {
		name    string
		id      any
		wantID  any // what we expect echoed back in the error response
		idBytes string
	}{
		{"string id", "req-abc", "req-abc", `"req-abc"`},
		{"integer id", float64(42), float64(42), `42`},
		{"null id", nil, nil, `null`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(`{"jsonrpc":"2.0","id":` + tc.idBytes + `,"method":"tools/call","params":{"name":"Bash","arguments":{"command":"npm install evil"}}}`)
			req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer tok")
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			var resp map[string]any
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("response not valid JSON: %v\nbody: %s", err, rr.Body.String())
			}

			gotID := resp["id"]
			switch tc.idBytes {
			case "null":
				if gotID != nil {
					t.Errorf("id = %v (%T), want nil", gotID, gotID)
				}
			case `"req-abc"`:
				if gotID != tc.wantID {
					t.Errorf("id = %v (%T), want %v (%T)", gotID, gotID, tc.wantID, tc.wantID)
				}
			default:
				// Integer echoed as float64 from JSON.
				if gotID != tc.wantID {
					t.Errorf("id = %v (%T), want %v (%T)", gotID, gotID, tc.wantID, tc.wantID)
				}
			}
		})
	}
}

// TestGatewayWarnInjectsField verifies that a warn decision forwards to upstream
// AND injects _beekeeper_warning into the response result.
func TestGatewayWarnInjectsField(t *testing.T) {
	upstreamResp := `{"jsonrpc":"2.0","id":1,"result":{"content":"ok"}}`
	upstream, cc := mockUpstream(t, upstreamResp)
	h := newTestHandler("tok", warnIdx(), upstream.URL)

	rr := postToolsCall(t, h, "tok", 1, "Bash", "warn-pkg")

	// Upstream MUST be called (warn forwards to upstream).
	if cc.count() != 1 {
		t.Errorf("upstream call count = %d, want 1 (warn must forward to upstream)", cc.count())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not valid JSON: %v\nbody: %s", err, rr.Body.String())
	}

	// _beekeeper_warning must be present in result.
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result field missing or not an object: %v", resp["result"])
	}
	warning, ok := result["_beekeeper_warning"].(string)
	if !ok || warning == "" {
		t.Errorf("_beekeeper_warning missing or empty in warn response: %v", result)
	}
}

// TestGatewayPassthroughInitialize verifies that initialize (removed in MCP
// July 2026 but may appear from older clients) is proxied without policy eval.
// This is Pitfall 7: never intercept initialize/initialized.
func TestGatewayPassthroughInitialize(t *testing.T) {
	upstreamResp := `{"jsonrpc":"2.0","id":1,"result":{"serverInfo":{}}}`
	upstream, cc := mockUpstream(t, upstreamResp)
	h := newTestHandler("tok", blockIdx(), upstream.URL) // blockIdx would block if evaluated

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if cc.count() != 1 {
		t.Errorf("upstream call count = %d, want 1 (initialize must be passed through)", cc.count())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if errField, ok := resp["error"]; ok {
		t.Errorf("initialize got error response (must be proxied): %v", errField)
	}
}

// TestGatewayNoDirector verifies that gateway.go does not use
// httputil.ReverseProxy.Director (deprecated in Go 1.25+).
// This test does a simple functional check: if Director were used incorrectly
// instead of Rewrite, the upstream URL would not be set. We verify the proxy
// forwards correctly.
func TestGatewayProxyForwards(t *testing.T) {
	upstreamResp := `{"jsonrpc":"2.0","id":1,"result":{"proxied":true}}`
	upstream, cc := mockUpstream(t, upstreamResp)
	h := newTestHandler("tok", allowIdx(), upstream.URL)

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	rr := postJSON(t, h, "tok", body)

	if cc.count() != 1 {
		t.Errorf("upstream call count = %d, want 1", cc.count())
	}
	_ = rr
}
