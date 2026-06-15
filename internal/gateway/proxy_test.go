package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/bantuson/beekeeper/internal/llamafirewall"
	"github.com/bantuson/beekeeper/internal/policy"
	"github.com/bantuson/beekeeper/internal/policyloader"
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

// TestGatewayBlocksDuplicateKeyBody verifies the request-smuggling guard
// (T-04-03-11): a tools/call whose params contain a duplicate key
// ({"name":"safe","name":"shell_exec"}) is rejected at parse time with a
// JSON-RPC -32600 error and the upstream is NEVER called. Without the guard,
// policy would evaluate the Go last-wins value while the raw bytes forwarded
// upstream could be parsed first-wins → policy/exec divergence.
func TestGatewayBlocksDuplicateKeyBody(t *testing.T) {
	// allowIdx would otherwise let any tool through — we must see the parse-time
	// block, not an allow.
	upstream, cc := mockUpstream(t, `{"jsonrpc":"2.0","id":1,"result":{}}`)
	h := newTestHandler("tok", allowIdx(), upstream.URL)

	// Duplicate "name" key in the params object: "safe" vs "shell_exec".
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"safe","name":"shell_exec","arguments":{}}}`)
	rr := postJSON(t, h, "tok", body)

	code := parseJSONRPCError(t, rr.Body.Bytes())
	if code != -32600 {
		t.Errorf("error code = %d, want -32600 (duplicate-key request rejected at parse)", code)
	}
	if cc.count() != 0 {
		t.Errorf("upstream was called %d times, want 0 (duplicate-key body must not be forwarded)", cc.count())
	}
}

// TestGatewaySanitizesUpstreamHeaders verifies fix #2: on the manual-forward
// (warn) path, internal X-Beekeeper-* headers and hop-by-hop headers from the
// client are NOT propagated to the upstream MCP server, while ordinary headers
// (e.g. Content-Type) are. The gateway Authorization token is also stripped.
func TestGatewaySanitizesUpstreamHeaders(t *testing.T) {
	var gotHeaders http.Header
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":"ok"}}`))
	}))
	defer upstream.Close()

	// warnIdx → warn decision → forwardWithWarningInjection (the manual path).
	h := newTestHandler("tok", warnIdx(), upstream.URL)

	body := toolsCallBody(1, "Bash", "warn-pkg")
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer tok")
	// Internal agent-context headers — must NOT reach upstream.
	req.Header.Set("X-Beekeeper-Agent-Id", "agent-123")
	req.Header.Set("X-Beekeeper-Parent-Agent-Id", "parent-456")
	req.Header.Set("X-Beekeeper-Agent-Depth", "2")
	// Hop-by-hop headers — must NOT reach upstream.
	req.Header.Set("Connection", "X-Custom-Smuggled")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Transfer-Encoding", "chunked")
	// A Connection-listed token must also be dropped.
	req.Header.Set("X-Custom-Smuggled", "should-be-dropped")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if gotHeaders == nil {
		t.Fatal("upstream was not called; cannot inspect forwarded headers")
	}

	// X-Beekeeper-* must be absent.
	for _, k := range []string{"X-Beekeeper-Agent-Id", "X-Beekeeper-Parent-Agent-Id", "X-Beekeeper-Agent-Depth"} {
		if v := gotHeaders.Get(k); v != "" {
			t.Errorf("internal header %s leaked to upstream: %q", k, v)
		}
	}
	// Hop-by-hop headers must be absent.
	for _, k := range []string{"Connection", "Upgrade", "Transfer-Encoding"} {
		if v := gotHeaders.Get(k); v != "" {
			t.Errorf("hop-by-hop header %s forwarded to upstream: %q", k, v)
		}
	}
	// The Connection-listed token must be dropped.
	if v := gotHeaders.Get("X-Custom-Smuggled"); v != "" {
		t.Errorf("Connection-listed header X-Custom-Smuggled forwarded to upstream: %q", v)
	}
	// The gateway Authorization token must be stripped.
	if v := gotHeaders.Get("Authorization"); v != "" {
		t.Errorf("gateway Authorization token leaked to upstream: %q", v)
	}
	// An ordinary header must still be forwarded.
	if v := gotHeaders.Get("Content-Type"); v == "" {
		t.Error("ordinary Content-Type header was not forwarded to upstream")
	}
}

// TestSanitizeUpstreamHeaders is a focused unit test of the shared sanitizer
// used by both manual-forward paths so they cannot drift.
func TestSanitizeUpstreamHeaders(t *testing.T) {
	in := http.Header{}
	in.Set("Authorization", "Bearer secret")
	in.Set("Content-Type", "application/json")
	in.Set("X-Beekeeper-Agent-Id", "a1")
	in.Set("Connection", "Keep-Alive, X-Listed")
	in.Set("Keep-Alive", "timeout=5")
	in.Set("Te", "trailers")
	in.Set("Trailer", "Expires")
	in.Set("Proxy-Authorization", "Basic abc")
	in.Set("Transfer-Encoding", "chunked")
	in.Set("Upgrade", "h2c")
	in.Set("X-Listed", "drop-me")
	in.Set("X-Safe", "keep-me")

	out := sanitizeUpstreamHeaders(in)

	mustAbsent := []string{
		"Authorization", "X-Beekeeper-Agent-Id", "Connection", "Keep-Alive",
		"Te", "Trailer", "Proxy-Authorization", "Transfer-Encoding", "Upgrade",
		"X-Listed",
	}
	for _, k := range mustAbsent {
		if v := out.Get(k); v != "" {
			t.Errorf("sanitizeUpstreamHeaders kept %s=%q, want it stripped", k, v)
		}
	}
	if out.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json (ordinary header dropped)", out.Get("Content-Type"))
	}
	if out.Get("X-Safe") != "keep-me" {
		t.Errorf("X-Safe = %q, want keep-me (non-internal custom header dropped)", out.Get("X-Safe"))
	}
}

// TestGatewayUnreadablePoliciesDirFailsClosed verifies fix #3: when the policies
// directory cannot be read (LoadPolicyDir returns an error), applyPolicy BLOCKS
// regardless of FailOpen. The unreadable-dir error is injected via the
// loadPolicyDirFn seam so the test is deterministic on every platform (a real
// unreadable directory is not portably fabricatable, especially on Windows where
// os.ReadDir on a non-dir maps to ErrNotExist → the missing-dir no-op path).
func TestGatewayUnreadablePoliciesDirFailsClosed(t *testing.T) {
	orig := loadPolicyDirFn
	loadPolicyDirFn = func(string) ([]policyloader.PolicyFile, error) {
		return nil, errors.New("permission denied reading policies dir")
	}
	defer func() { loadPolicyDirFn = orig }()

	for _, failOpen := range []bool{false, true} {
		name := "FailOpen=false"
		if failOpen {
			name = "FailOpen=true"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			cacheDir := filepath.Join(dir, "catalogs")
			if err := os.MkdirAll(cacheDir, 0o700); err != nil {
				t.Fatalf("MkdirAll cacheDir: %v", err)
			}

			msg := JSONRPCMessage{
				JSONRPC: "2.0",
				Method:  "tools/call",
				Params:  []byte(`{"name":"Bash","arguments":{"command":"ls"}}`),
			}
			cfg := Config{CacheDir: cacheDir, FailOpen: failOpen}

			d := applyPolicy(msg, allowIdx(), cfg, policy.AgentContext{})
			if d.Level != "block" {
				t.Errorf("decision.Level = %q, want block (unreadable policies dir must fail closed even with FailOpen=%v)", d.Level, failOpen)
			}
		})
	}
}

// newTestHandlerWithScanner creates a gatewayHandler with a scanner configured.
func newTestHandlerWithScanner(token string, idx policy.MultiCatalogLookup, upstream string, scanner GatewayScanner) *gatewayHandler {
	cfg := Config{
		UpstreamURL: upstream,
		BindAddr:    defaultBindAddr,
		Port:        defaultPort,
		Scanner:     scanner,
	}
	return newGatewayHandler(cfg, token, idx)
}

// TestGatewayScannerInvokedForEligibleToolOnWarnPath verifies that when a scanner
// is present and the proxied tool is eligible (read_file), ScanProxiedResponse
// is actually invoked on the warn path (LLMF-02 gap closure).
// Previously, ScanProxiedResponse was called with "" so ShouldScanPrompt returned
// false and PromptGuard was a silent no-op.
func TestGatewayScannerInvokedForEligibleToolOnWarnPath(t *testing.T) {
	upstreamResp := `{"jsonrpc":"2.0","id":1,"result":{"content":"some file content"}}`
	upstream, _ := mockUpstream(t, upstreamResp)

	scanner := &mockGatewayScanner{
		resp: llamafirewall.ScanResponse{
			Result:     llamafirewall.ResultInjection,
			Confidence: 0.97,
			Reason:     "prompt injection marker detected",
		},
	}
	// warnIdx returns a single signed match → warn decision.
	h := newTestHandlerWithScanner("tok", warnIdx(), upstream.URL, scanner)

	// Send a tools/call for "read_file" (a ShouldScanPrompt-eligible tool).
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "read_file",
			"arguments": map[string]any{"path": "/etc/passwd"},
		},
	})
	req := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	// The scanner MUST have been invoked (real tool name was passed, not "").
	if scanner.calls == 0 {
		t.Error("scanner.calls = 0: scanner was not invoked; ScanProxiedResponse received empty toolName (LLMF-02 regression)")
	}
}

// TestGatewayScannerInvokedForEligibleToolOnAllowPath verifies that when a
// scanner is present and the tool is eligible (web_search), ScanProxiedResponse
// is invoked on the allow path (LLMF-02 gap closure).
func TestGatewayScannerInvokedForEligibleToolOnAllowPath(t *testing.T) {
	upstreamResp := `{"jsonrpc":"2.0","id":1,"result":{"content":"search results with injection"}}`
	upstream, _ := mockUpstream(t, upstreamResp)

	scanner := &mockGatewayScanner{
		resp: llamafirewall.ScanResponse{
			Result:     llamafirewall.ResultInjection,
			Confidence: 0.95,
			Reason:     "indirect prompt injection detected",
		},
	}
	// allowIdx returns no matches → allow decision.
	h := newTestHandlerWithScanner("tok", allowIdx(), upstream.URL, scanner)

	// Send a tools/call for "web_search" (ShouldScanPrompt-eligible).
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "web_search",
			"arguments": map[string]any{"query": "beekeeper docs"},
		},
	})
	req := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	// The scanner MUST have been invoked.
	if scanner.calls == 0 {
		t.Error("scanner.calls = 0: scanner was not invoked on allow path; ScanProxiedResponse received empty toolName (LLMF-02 regression)")
	}
}

// TestGatewayScannerSkippedForNonEligibleTool verifies that for a tool that is
// NOT in the ShouldScanPrompt list (e.g. "Bash"), the scanner is NOT invoked.
// This preserves the existing behavior for non-eligible tools.
func TestGatewayScannerSkippedForNonEligibleTool(t *testing.T) {
	upstreamResp := `{"jsonrpc":"2.0","id":1,"result":{"output":"bash output"}}`
	upstream, _ := mockUpstream(t, upstreamResp)

	scanner := &mockGatewayScanner{
		resp: llamafirewall.ScanResponse{Result: llamafirewall.ResultClean},
	}
	// allowIdx → allow decision.
	h := newTestHandlerWithScanner("tok", allowIdx(), upstream.URL, scanner)

	// "Bash" is NOT in ShouldScanPrompt → scanner must not be invoked.
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "Bash",
			"arguments": map[string]any{"command": "ls -la"},
		},
	})
	req := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	// Scanner must NOT be called for non-eligible tools.
	if scanner.calls != 0 {
		t.Errorf("scanner.calls = %d, want 0 (Bash is not prompt-scan-eligible)", scanner.calls)
	}
	_ = rr
}

// TestScanProxiedResponseRealToolNamePassedNotEmpty is a unit-level regression
// guard: ScanProxiedResponse("", ...) must be a no-op; passing the real tool
// name for an eligible tool must invoke the scanner.
func TestScanProxiedResponseRealToolNamePassedNotEmpty(t *testing.T) {
	scanner := &mockGatewayScanner{
		resp: llamafirewall.ScanResponse{Result: llamafirewall.ResultClean},
	}

	body := []byte(`{"content":"clean content"}`)

	// Empty tool name → no-op.
	_, _ = ScanProxiedResponse(context.Background(), "", body, scanner)
	if scanner.calls != 0 {
		t.Errorf("scanner invoked with empty toolName (%d calls), want 0", scanner.calls)
	}

	// Real eligible tool name → scanner must be invoked.
	scanner.calls = 0
	_, _ = ScanProxiedResponse(context.Background(), "read_file", body, scanner)
	if scanner.calls == 0 {
		t.Error("scanner not invoked with toolName=read_file, want >= 1")
	}
}
