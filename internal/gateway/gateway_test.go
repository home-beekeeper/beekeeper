package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bantuson/beekeeper/internal/llamafirewall"
	"github.com/bantuson/beekeeper/internal/nudge"
	"github.com/bantuson/beekeeper/internal/policy"
)

// mockGatewayScanner implements GatewayScanner for testing.
type mockGatewayScanner struct {
	resp     llamafirewall.ScanResponse
	err      error
	degraded bool
	calls    int
}

func (m *mockGatewayScanner) Scan(_ context.Context, _ llamafirewall.ScanRequest) (llamafirewall.ScanResponse, error) {
	m.calls++
	return m.resp, m.err
}

func (m *mockGatewayScanner) IsDegraded() bool { return m.degraded }

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

// TestIsLoopbackAddr verifies the address classification helper (TM-A-01).
func TestIsLoopbackAddr(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		// Loopback addresses — should return true.
		{"", true},           // empty → default 127.0.0.1
		{"127.0.0.1", true},  // IPv4 loopback
		{"127.0.0.2", true},  // 127.0.0.0/8 range
		{"127.1.2.3", true},  // 127.0.0.0/8 range
		{"::1", true},        // IPv6 loopback
		{"localhost", true},  // hostname alias
		{"LOCALHOST", true},  // case-insensitive

		// Non-loopback — should return false.
		{"0.0.0.0", false},        // all-interfaces IPv4
		{"::", false},             // all-interfaces IPv6
		{"192.168.1.1", false},    // LAN IP
		{"10.0.0.1", false},       // private range
		{"203.0.113.1", false},    // external IP
		{"example.com", false},    // external hostname
		{"lan-host", false},       // non-localhost hostname
	}
	for _, tc := range tests {
		t.Run(tc.addr, func(t *testing.T) {
			got := IsLoopbackAddr(tc.addr)
			if got != tc.want {
				t.Errorf("IsLoopbackAddr(%q) = %v, want %v", tc.addr, got, tc.want)
			}
		})
	}
}

// TestGatewayStartRefusesNonLoopbackWithoutOptIn verifies that Start returns an
// error when BindAddr is non-loopback and AllowRemote is false (TM-A-01).
func TestGatewayStartRefusesNonLoopbackWithoutOptIn(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		UpstreamURL: "http://localhost:9999",
		BindAddr:    "0.0.0.0",
		AllowRemote: false, // no opt-in → must be refused
	}
	err := Start(ctx, cfg)
	if err == nil {
		t.Fatal("Start returned nil; expected error for non-loopback bind without --allow-remote")
	}
	if !strings.Contains(err.Error(), "refusing to bind non-loopback address") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestGatewayStartNonLoopbackWithOptInWarns verifies that Start proceeds (does not
// return the gate error) when AllowRemote is true, even for a non-loopback address.
// Because we can't actually bind 0.0.0.0 reliably in unit tests (port conflicts, CI
// restrictions), we verify absence of the gate error and capture the stderr warning.
func TestGatewayStartNonLoopbackWithOptInWarns(t *testing.T) {
	// We test only the gate check, not the full daemon lifecycle. Redirect stderr to
	// capture the warning. The test expects the error to NOT be the gate refusal —
	// it may be a downstream error (catalog not found, etc.) which is acceptable here.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so Start exits after gate check + token gen + catalog open

	cfg := Config{
		UpstreamURL: "http://localhost:9999",
		BindAddr:    "0.0.0.0",
		AllowRemote: true, // opt-in → gate must pass
		StateFile:   os.DevNull,
		IndexPath:   os.DevNull, // will fail to open — that is expected
	}

	// Redirect os.Stderr to capture the plaintext-HTTP warning.
	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := Start(ctx, cfg)

	w.Close()
	os.Stderr = origStderr
	buf := new(strings.Builder)
	io.Copy(buf, r) //nolint:errcheck

	// The gate error must NOT appear — AllowRemote was set.
	if err != nil && strings.Contains(err.Error(), "refusing to bind non-loopback address") {
		t.Errorf("gate should pass when AllowRemote=true, got gate refusal: %v", err)
	}

	// The plaintext-HTTP warning must appear on stderr.
	if !strings.Contains(buf.String(), "plain HTTP") {
		t.Errorf("expected plaintext-HTTP warning on stderr; got: %q", buf.String())
	}
}

// TestGatewayStartLoopbackNeedsNoOptIn verifies that a loopback bind (127.0.0.1)
// succeeds without AllowRemote and emits no warning.
func TestGatewayStartLoopbackNeedsNoOptIn(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := Config{
		UpstreamURL: "http://localhost:9999",
		BindAddr:    "127.0.0.1",
		AllowRemote: false, // loopback — no opt-in required
		StateFile:   os.DevNull,
		IndexPath:   os.DevNull,
	}

	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := Start(ctx, cfg)

	w.Close()
	os.Stderr = origStderr
	buf := new(strings.Builder)
	io.Copy(buf, r) //nolint:errcheck

	// Must not be the gate refusal error.
	if err != nil && strings.Contains(err.Error(), "refusing to bind non-loopback address") {
		t.Errorf("loopback bind should not trigger gate refusal: %v", err)
	}

	// No plaintext-HTTP warning for loopback bind.
	if strings.Contains(buf.String(), "plain HTTP") {
		t.Errorf("loopback bind should not emit plaintext-HTTP warning; got: %q", buf.String())
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

	d := applyPolicy(msg, allowIdx(), Config{}, policy.AgentContext{})
	if d.Level != "block" {
		t.Errorf("decision.Level = %q, want block (malformed params → fail-closed)", d.Level)
	}
}

// TestGatewayTwoSourceCorroborationProducesBlock verifies INT-BLOCK-3 closure:
// a gateway tools/call for a package matched by two signed sources (bumblebee +
// osv via blockIdx) must produce a block decision (-32001 JSON-RPC error).
func TestGatewayTwoSourceCorroborationProducesBlock(t *testing.T) {
	// blockIdx returns two signed matches (bumblebee + osv) → corroboration block.
	h := newTestHandler("test-token", blockIdx(), "http://upstream-unused")

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"Bash","arguments":{"ecosystem":"npm","package":"evil-pkg"}}}`)
	rr := postJSON(t, h, "test-token", body)

	// Expect -32001 JSON-RPC error (blocked by Beekeeper).
	code := parseJSONRPCError(t, rr.Body.Bytes())
	if code != -32001 {
		t.Errorf("error code = %d, want -32001 (block) for 2-source corroboration", code)
	}
}

// TestGatewayPolicyOverlayAffectsDecision verifies INT-WARN-1 closure for gateway:
// a package_allowlist block rule in a policy file causes an engine-allow decision
// to become a block at the gateway path.
func TestGatewayPolicyOverlayAffectsDecision(t *testing.T) {
	dir := t.TempDir()

	// cacheDir → sibling policies/ with a block rule for "overlay-test-evil-pkg".
	cacheDir := filepath.Join(dir, "catalogs")
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	policiesDir := filepath.Join(dir, "policies")
	if err := os.MkdirAll(policiesDir, 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	policyJSON := `{
		"schema_version": "1",
		"name": "gw-test-block",
		"rules": [
			{
				"id": "gw-block-overlay-test",
				"rule_type": "package_allowlist",
				"ecosystem": "npm",
				"packages": ["overlay-test-evil-pkg"],
				"action": "block"
			}
		]
	}`
	if err := os.WriteFile(filepath.Join(policiesDir, "test-block.json"), []byte(policyJSON), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// allowIdx() returns no catalog matches → engine decision is allow.
	// The overlay block rule must override to block.
	cfg := Config{
		UpstreamURL: "http://upstream-unused",
		BindAddr:    defaultBindAddr,
		Port:        defaultPort,
		CacheDir:    cacheDir,
	}
	h := newGatewayHandler(cfg, "test-token", allowIdx())

	body := []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"Bash","arguments":{"ecosystem":"npm","package":"overlay-test-evil-pkg"}}}`)
	rr := postJSON(t, h, "test-token", body)

	code := parseJSONRPCError(t, rr.Body.Bytes())
	if code != -32001 {
		t.Errorf("error code = %d, want -32001 (block) for policy overlay block rule", code)
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

// TestScanProxiedResponseInjectionDetected verifies INT-BLOCK-1 + INT-WARN-2 closure:
// ScanProxiedResponse invokes the scanner for a prompt-eligible tool and returns a
// replaced (warning) payload when injection is detected.
func TestScanProxiedResponseInjectionDetected(t *testing.T) {
	scanner := &mockGatewayScanner{
		resp: llamafirewall.ScanResponse{
			Result:     llamafirewall.ResultInjection,
			Confidence: 0.99,
			Reason:     "injection detected",
			LatencyMS:  5,
		},
	}

	body := []byte(`{"content":"malicious content with injection"}`)
	// "read_file" is a prompt-eligible tool (returns file content — injection surface).
	out, injected := ScanProxiedResponse(context.Background(), "read_file", body, scanner)

	if !injected {
		t.Error("injected = false, want true (injection detected)")
	}
	if scanner.calls == 0 {
		t.Error("scanner.calls = 0, want >= 1 (scanner must be invoked)")
	}
	// Output should be the warning payload (different from original body).
	if bytes.Equal(out, body) {
		t.Error("output body unchanged despite injection, want replaced warning payload")
	}
}

// TestScanProxiedResponseNilScannerIsNoOp verifies that ScanProxiedResponse
// with a nil scanner returns the original body unchanged (LLMF disabled = no-op).
func TestScanProxiedResponseNilScannerIsNoOp(t *testing.T) {
	body := []byte(`{"content":"safe content"}`)
	out, injected := ScanProxiedResponse(context.Background(), "read_file", body, nil)

	if injected {
		t.Error("injected = true with nil scanner, want false")
	}
	if !bytes.Equal(out, body) {
		t.Error("output body changed with nil scanner, want original unchanged")
	}
}

// TestScanProxiedResponseDegradedScannerIsNoOp verifies that a degraded scanner
// (sidecar exhausted retries) returns the original body unchanged — fail-open
// behavior for the scan path (degraded != unreachable; supervisor already decided).
func TestScanProxiedResponseDegradedScannerIsNoOp(t *testing.T) {
	scanner := &mockGatewayScanner{degraded: true}
	body := []byte(`{"content":"some content"}`)
	out, injected := ScanProxiedResponse(context.Background(), "read_file", body, scanner)

	if injected {
		t.Error("injected = true with degraded scanner, want false")
	}
	if !bytes.Equal(out, body) {
		t.Error("output body changed with degraded scanner, want original unchanged")
	}
	if scanner.calls != 0 {
		t.Errorf("scanner called %d times, want 0 for degraded scanner", scanner.calls)
	}
}

// TestGatewayNudgeMerge verifies the cache-backed nudge merge at the proxy.go
// applyPolicy call site (WARNING 2 closed). When the nudge.DetectStateFn seam
// is injected to return a hardened pnpm state, a Bash "npm install" command
// should produce an advisory (warn) decision — the nudge merge must run after
// applyPolicy returns its overlay-applied result (CR-02, T-08-17).
//
// Plan structure: TestGatewayNudgeMerge proves the proxy call-site merge produces
// a cache-backed nudge decision using h.cfg.Nudge (WARNING 2 + WARNING 3 closed).
func TestGatewayNudgeMerge(t *testing.T) {
	// Inject a synthetic PMState: pnpm hardened so nudge.Evaluate → Advise.
	orig := nudge.DetectStateFn
	nudge.DetectStateFn = func(_ context.Context, _ nudge.Config) nudge.PMState {
		return nudge.PMState{
			PnpmInstalled: true,
			PnpmVersion:   "11.5.0",
			PnpmHardened:  true,
			NodeVersion:   "22.5.0",
		}
	}
	defer func() { nudge.DetectStateFn = orig }()

	// Use a real upstream to avoid connection errors — tools/call on allowIdx
	// will produce allow, but nudge merge should elevate to warn.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":"ok"}}`))
	}))
	defer upstream.Close()

	// Construct handler with nudge enabled (DefaultConfig).
	cfg := Config{
		UpstreamURL: upstream.URL,
		BindAddr:    defaultBindAddr,
		Port:        defaultPort,
		Nudge:       nudge.DefaultConfig(),
	}
	h := newGatewayHandler(cfg, "test-token", allowIdx())

	// Bash npm install → allowIdx returns allow; nudge merge should elevate to warn.
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"Bash","arguments":{"command":"npm install lodash"}}}`)
	rr := postJSON(t, h, "test-token", body)

	// A warn decision is forwarded to upstream AND has _beekeeper_warning injected.
	// The HTTP status is 200 (JSON-RPC spec), but no -32001 error (not blocked).
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not valid JSON: %v\nbody: %s", err, rr.Body.String())
	}
	// Must NOT be a -32001 error (nudge Advise is warn not block in soft mode).
	if errField, ok := resp["error"]; ok {
		errMap, _ := errField.(map[string]any)
		if errMap != nil {
			code, _ := errMap["code"].(float64)
			if code == -32001 {
				t.Errorf("nudge Advise should NOT block (soft mode); got -32001 error: %v", errField)
			}
		}
	}
	// Must have _beekeeper_warning injected into result (warn path).
	result, _ := resp["result"].(map[string]any)
	if result == nil {
		t.Fatalf("expected result object, got: %v", resp)
	}
	if _, hasWarning := result["_beekeeper_warning"]; !hasWarning {
		t.Errorf("expected _beekeeper_warning in result for nudge Advise (warn); result: %v", result)
	}
}

// TestGatewayAdvisoryCapPerSession verifies the at-most-one-advisory-per-session
// cap (NUDGE-03): the second Advise for the same agent-id is suppressed as an
// advisory message (no _beekeeper_warning on the second response) while the
// audit record is still written. The block decision is never capped.
func TestGatewayAdvisoryCapPerSession(t *testing.T) {
	// Inject pnpm-hardened state → Advise on every npm install.
	orig := nudge.DetectStateFn
	nudge.DetectStateFn = func(_ context.Context, _ nudge.Config) nudge.PMState {
		return nudge.PMState{
			PnpmInstalled: true,
			PnpmVersion:   "11.5.0",
			PnpmHardened:  true,
			NodeVersion:   "22.5.0",
		}
	}
	defer func() { nudge.DetectStateFn = orig }()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":"ok"}}`))
	}))
	defer upstream.Close()

	cfg := Config{
		UpstreamURL: upstream.URL,
		BindAddr:    defaultBindAddr,
		Port:        defaultPort,
		Nudge:       nudge.DefaultConfig(),
	}
	h := newGatewayHandler(cfg, "test-token", allowIdx())

	installBody := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"Bash","arguments":{"command":"npm install lodash"}}}`)

	// sendWithAgent sends a tools/call with the given agent-id header.
	sendWithAgent := func(agentID string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(installBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-token")
		if agentID != "" {
			req.Header.Set("X-Beekeeper-Agent-Id", agentID)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}

	// First request for agent-A: expect advisory (_beekeeper_warning) injected.
	rr1 := sendWithAgent("agent-A")
	var resp1 map[string]any
	if err := json.Unmarshal(rr1.Body.Bytes(), &resp1); err != nil {
		t.Fatalf("first response not valid JSON: %v", err)
	}
	result1, _ := resp1["result"].(map[string]any)
	if result1 == nil {
		t.Fatalf("first request: expected result object, got: %v", resp1)
	}
	if _, hasWarn := result1["_beekeeper_warning"]; !hasWarn {
		t.Errorf("first request for agent-A: expected _beekeeper_warning (first advisory), got none; result: %v", result1)
	}

	// Second request for agent-A: advisory cap fires — no _beekeeper_warning.
	rr2 := sendWithAgent("agent-A")
	var resp2 map[string]any
	if err := json.Unmarshal(rr2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("second response not valid JSON: %v", err)
	}
	result2, _ := resp2["result"].(map[string]any)
	if result2 == nil {
		t.Fatalf("second request: expected result object, got: %v", resp2)
	}
	if _, hasWarn := result2["_beekeeper_warning"]; hasWarn {
		t.Errorf("second request for agent-A: advisory cap should suppress _beekeeper_warning; got warning in result: %v", result2)
	}

	// Third request for agent-B (different session): first advisory for B — must be surfaced.
	rr3 := sendWithAgent("agent-B")
	var resp3 map[string]any
	if err := json.Unmarshal(rr3.Body.Bytes(), &resp3); err != nil {
		t.Fatalf("third response not valid JSON: %v", err)
	}
	result3, _ := resp3["result"].(map[string]any)
	if result3 == nil {
		t.Fatalf("third request (agent-B): expected result object, got: %v", resp3)
	}
	if _, hasWarn := result3["_beekeeper_warning"]; !hasWarn {
		t.Errorf("first advisory for agent-B should NOT be suppressed; result: %v", result3)
	}
}
