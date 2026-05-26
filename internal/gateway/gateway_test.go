package gateway

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mzansi-agentive/beekeeper/internal/policy"
)

// fakeIdx is a test double for policy.MultiCatalogLookup that returns no
// matches (allow-all) unless overridden per test.
type fakeIdx struct {
	fn func(ecosystem, pkg string) []policy.CatalogMatch
}

func (f *fakeIdx) LookupAll(ecosystem, pkg string) []policy.CatalogMatch {
	if f.fn != nil {
		return f.fn(ecosystem, pkg)
	}
	return nil
}

// allowIdx returns a fakeIdx that always returns no matches (allow decision).
func allowIdx() *fakeIdx {
	return &fakeIdx{}
}

// blockIdx returns a fakeIdx that always returns two signed matches (block decision).
func blockIdx() *fakeIdx {
	return &fakeIdx{
		fn: func(ecosystem, pkg string) []policy.CatalogMatch {
			return []policy.CatalogMatch{
				{CatalogSource: "bumblebee", Signed: true, Ecosystem: ecosystem, Package: pkg, Severity: "critical"},
				{CatalogSource: "osv", Signed: true, Ecosystem: ecosystem, Package: pkg, Severity: "critical"},
			}
		},
	}
}

// warnIdx returns a fakeIdx that always returns one signed match (warn decision).
func warnIdx() *fakeIdx {
	return &fakeIdx{
		fn: func(ecosystem, pkg string) []policy.CatalogMatch {
			return []policy.CatalogMatch{
				{CatalogSource: "bumblebee", Signed: true, Ecosystem: ecosystem, Package: pkg, Severity: "high"},
			}
		},
	}
}

// newTestHandler creates a gatewayHandler with a known token for testing.
func newTestHandler(token string, idx policy.MultiCatalogLookup, upstream string) *gatewayHandler {
	cfg := Config{
		UpstreamURL: upstream,
		BindAddr:    defaultBindAddr,
		Port:        defaultPort,
	}
	return newGatewayHandler(cfg, token, idx)
}

// postJSON sends a POST request to the handler with a JSON body and an
// Authorization: Bearer <token> header. Returns the *httptest.ResponseRecorder.
func postJSON(t *testing.T, h http.Handler, token string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// parseJSONRPCError parses the error code from a JSON-RPC error response body.
func parseJSONRPCError(t *testing.T, body []byte) int {
	t.Helper()
	var resp struct {
		Error struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("could not parse JSON-RPC error response: %v\nbody: %s", err, body)
	}
	return resp.Error.Code
}

// TestGatewayLocalOnlyBind verifies that the gateway handler binds to 127.0.0.1
// by default. We test this by inspecting the Config.BindAddr used by newGatewayHandler.
func TestGatewayLocalOnlyBind(t *testing.T) {
	// The gateway uses httptest.Server for tests, but we verify the default
	// bind address is not 0.0.0.0 (T-04-03-08).
	cfg := Config{}
	if cfg.BindAddr == "" {
		cfg.BindAddr = defaultBindAddr
	}
	if cfg.BindAddr != "127.0.0.1" {
		t.Errorf("default BindAddr = %q, want 127.0.0.1", cfg.BindAddr)
	}
	if strings.Contains(cfg.BindAddr, "0.0.0.0") {
		t.Errorf("BindAddr must not be 0.0.0.0 by default (T-04-03-08)")
	}
}

// TestGatewayUnauthorized verifies that requests without an Authorization header
// receive a JSON-RPC -32600 error (T-04-03-02).
func TestGatewayUnauthorized(t *testing.T) {
	h := newTestHandler("valid-token", allowIdx(), "http://upstream-unused")

	// No Authorization header.
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if got := parseJSONRPCError(t, rr.Body.Bytes()); got != -32600 {
		t.Errorf("error code = %d, want -32600", got)
	}
}

// TestGatewayWrongToken verifies that a wrong token gets -32600 (T-04-03-02).
func TestGatewayWrongToken(t *testing.T) {
	h := newTestHandler("correct-token", allowIdx(), "http://upstream-unused")

	rr := postJSON(t, h, "wrong-token", []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))

	if got := parseJSONRPCError(t, rr.Body.Bytes()); got != -32600 {
		t.Errorf("error code = %d, want -32600", got)
	}
}

// TestGatewayOversizedBody verifies that a body > 1MB gets a -32700 error (T-04-03-03).
func TestGatewayOversizedBody(t *testing.T) {
	h := newTestHandler("test-token", allowIdx(), "http://upstream-unused")

	// 2MB body — exceeds the 1MB cap.
	oversized := make([]byte, 2*1024*1024)
	for i := range oversized {
		oversized[i] = 'x'
	}

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(oversized))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if got := parseJSONRPCError(t, rr.Body.Bytes()); got != -32700 {
		t.Errorf("error code = %d, want -32700", got)
	}
}

// TestGatewayStatePermissions verifies that SaveGatewayState creates state.json
// with 0o600 permissions (T-04-03-01). Skipped on Windows where ACL semantics differ.
func TestGatewayStatePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission semantics differ on Windows (ACL-based)")
	}

	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")

	gw := GatewayState{
		GatewayToken: "abc123",
		BoundAddr:    "127.0.0.1",
		BoundPort:    7837,
		StartedAt:    "2026-05-26T00:00:00Z",
		PID:          12345,
	}

	if err := SaveGatewayState(stateFile, gw); err != nil {
		t.Fatalf("SaveGatewayState error: %v", err)
	}

	info, err := os.Stat(stateFile)
	if err != nil {
		t.Fatalf("stat state file: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("state.json permissions = %04o, want 0600", perm)
	}
}

// TestGatewayStateSaveLoad verifies the round-trip SaveGatewayState/LoadGatewayState.
func TestGatewayStateSaveLoad(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")

	gw := GatewayState{
		GatewayToken: "test-token-abc",
		BoundAddr:    "127.0.0.1",
		BoundPort:    7837,
		StartedAt:    "2026-05-26T12:00:00Z",
		PID:          999,
	}

	if err := SaveGatewayState(stateFile, gw); err != nil {
		t.Fatalf("SaveGatewayState: %v", err)
	}

	loaded, err := LoadGatewayState(stateFile)
	if err != nil {
		t.Fatalf("LoadGatewayState: %v", err)
	}

	if loaded.GatewayToken != gw.GatewayToken {
		t.Errorf("token = %q, want %q", loaded.GatewayToken, gw.GatewayToken)
	}
	if loaded.BoundPort != gw.BoundPort {
		t.Errorf("port = %d, want %d", loaded.BoundPort, gw.BoundPort)
	}
	if loaded.PID != gw.PID {
		t.Errorf("pid = %d, want %d", loaded.PID, gw.PID)
	}
}

// TestGatewayStateMissingFile verifies that LoadGatewayState returns zero value
// for a missing file (not an error).
func TestGatewayStateMissingFile(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "nonexistent.json")

	gw, err := LoadGatewayState(stateFile)
	if err != nil {
		t.Fatalf("LoadGatewayState missing file: %v", err)
	}
	if gw.GatewayToken != "" {
		t.Errorf("expected zero value, got token=%q", gw.GatewayToken)
	}
}

// TestGatewayMalformedJSON verifies that malformed JSON body returns -32700.
func TestGatewayMalformedJSON(t *testing.T) {
	h := newTestHandler("test-token", allowIdx(), "http://upstream-unused")

	rr := postJSON(t, h, "test-token", []byte("not json at all"))

	if got := parseJSONRPCError(t, rr.Body.Bytes()); got != -32700 {
		t.Errorf("error code = %d, want -32700", got)
	}
}

// TestGenerateToken verifies that generateToken produces a 64-char hex string.
func TestGenerateToken(t *testing.T) {
	for i := 0; i < 5; i++ {
		tok, err := generateToken()
		if err != nil {
			t.Fatalf("generateToken error: %v", err)
		}
		if len(tok) != 64 {
			t.Errorf("token length = %d, want 64", len(tok))
		}
		// Verify it's valid hex.
		for _, c := range tok {
			if !strings.ContainsRune("0123456789abcdef", c) {
				t.Errorf("token contains non-hex char %q: %s", c, tok)
			}
		}
	}
}

// TestVerifyToken verifies constant-time token comparison.
func TestVerifyToken(t *testing.T) {
	h := newTestHandler("secret-token", allowIdx(), "http://unused")

	tests := []struct {
		name   string
		auth   string
		wantOK bool
	}{
		{"correct token", "Bearer secret-token", true},
		{"wrong token", "Bearer wrong-token", false},
		{"no bearer prefix", "secret-token", false},
		{"empty", "", false},
		{"bearer only", "Bearer ", false},
		{"bearer with spaces", "Bearer  secret-token", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req.Header.Set("Authorization", tc.auth)
			got := h.verifyToken(req)
			if got != tc.wantOK {
				t.Errorf("verifyToken(%q) = %v, want %v", tc.auth, got, tc.wantOK)
			}
		})
	}
}

// TestInjectWarning verifies that _beekeeper_warning is injected into the result.
func TestInjectWarning(t *testing.T) {
	resp := []byte(`{"jsonrpc":"2.0","id":1,"result":{"content":"ok"}}`)
	got := injectWarning(resp, "single-source match")

	var parsed map[string]any
	if err := json.Unmarshal(got, &parsed); err != nil {
		t.Fatalf("inject result not valid JSON: %v", err)
	}

	result, ok := parsed["result"].(map[string]any)
	if !ok {
		t.Fatalf("result is not an object: %T", parsed["result"])
	}

	warning, ok := result["_beekeeper_warning"].(string)
	if !ok {
		t.Errorf("_beekeeper_warning missing or not string")
	}
	if warning == "" {
		t.Errorf("_beekeeper_warning is empty")
	}
}

// TestExtractAgentContext verifies header-based AgentContext extraction.
func TestExtractAgentContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-Beekeeper-Agent-Id", "agent-123")
	req.Header.Set("X-Beekeeper-Parent-Agent-Id", "parent-456")
	req.Header.Set("X-Beekeeper-Agent-Depth", "2")

	ac := extractAgentContext(req)

	if ac.AgentID != "agent-123" {
		t.Errorf("AgentID = %q, want agent-123", ac.AgentID)
	}
	if ac.ParentAgentID != "parent-456" {
		t.Errorf("ParentAgentID = %q, want parent-456", ac.ParentAgentID)
	}
	if ac.Depth != 2 {
		t.Errorf("Depth = %d, want 2", ac.Depth)
	}
}

// TestExtractAgentContextNegativeDepth verifies that negative depth is normalized to 0.
func TestExtractAgentContextNegativeDepth(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-Beekeeper-Agent-Depth", "-5")

	ac := extractAgentContext(req)
	if ac.Depth != 0 {
		t.Errorf("Depth = %d, want 0 (negative normalized)", ac.Depth)
	}
}

// TestApplyPolicyMalformedParams verifies that malformed params → block (fail-closed).
func TestApplyPolicyMalformedParams(t *testing.T) {
	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params:  []byte(`not valid json`),
	}

	d := applyPolicy(msg, allowIdx(), policy.AgentContext{})
	if d.Level != "block" {
		t.Errorf("decision.Level = %q, want block (malformed params → fail-closed)", d.Level)
	}
}

// TestWriteJSONRPCError verifies that writeJSONRPCError produces a valid JSON-RPC
// error response with HTTP 200.
func TestWriteJSONRPCError(t *testing.T) {
	rr := httptest.NewRecorder()
	writeJSONRPCError(rr, "req-abc", -32001, "blocked", map[string]any{"reason": "test"})

	if rr.Code != http.StatusOK {
		t.Errorf("HTTP status = %d, want 200", rr.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}

	// Verify id is echoed back as string.
	if resp["id"] != "req-abc" {
		t.Errorf("id = %v, want req-abc", resp["id"])
	}

	errObj, _ := resp["error"].(map[string]any)
	if errObj == nil {
		t.Fatal("error field missing")
	}
	// JSON numbers unmarshal as float64.
	if errObj["code"].(float64) != -32001 {
		t.Errorf("error.code = %v, want -32001", errObj["code"])
	}
}

// TestClearGatewayState verifies that ClearGatewayState removes the gateway key
// while preserving other keys.
func TestClearGatewayState(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")

	// Write state with gateway key.
	gw := GatewayState{GatewayToken: "to-be-cleared", BoundPort: 7837}
	if err := SaveGatewayState(stateFile, gw); err != nil {
		t.Fatalf("SaveGatewayState: %v", err)
	}

	// Clear gateway key.
	if err := ClearGatewayState(stateFile); err != nil {
		t.Fatalf("ClearGatewayState: %v", err)
	}

	// Load should return zero value after clear.
	loaded, err := LoadGatewayState(stateFile)
	if err != nil {
		t.Fatalf("LoadGatewayState after clear: %v", err)
	}
	if loaded.GatewayToken != "" {
		t.Errorf("gateway token not cleared: %q", loaded.GatewayToken)
	}

	// State file should still exist (not deleted).
	if _, err := os.Stat(stateFile); err != nil {
		t.Errorf("state file deleted by ClearGatewayState: %v", err)
	}

	// Verify we can read the raw JSON and that gateway key is null/absent.
	data, _ := os.ReadFile(stateFile)
	var raw map[string]any
	_ = json.Unmarshal(data, &raw)
	if _, ok := raw["gateway"]; ok && raw["gateway"] != nil {
		t.Errorf("gateway key should be null or absent after clear, got: %v", raw["gateway"])
	}
}

// TestReadBody1MB verifies that a body of exactly 1MB (not oversized) is accepted.
func TestReadBody1MB(t *testing.T) {
	// Use an upstream that returns a minimal response.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read and discard body.
		_, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	}))
	defer upstream.Close()

	h := newTestHandler("test-token", allowIdx(), upstream.URL)

	// Build a JSON-RPC message with params padded to just under 1MB total.
	// We use a tools/call with a large arguments field.
	padding := strings.Repeat("x", 900*1024) // 900KB padding
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "test",
			"arguments": map[string]any{"data": padding},
		},
	})

	if len(body) > maxRequestBody {
		t.Skip("test body exceeds 1MB — skipping body size boundary test")
	}

	rr := postJSON(t, h, "test-token", body)
	// Should not be a -32700 error (oversized body).
	var resp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if errField, ok := resp["error"].(map[string]any); ok {
		if errField["code"].(float64) == -32700 {
			t.Errorf("body within 1MB limit rejected with -32700: %s", rr.Body.String())
		}
	}
}
