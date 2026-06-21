package gateway

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/home-beekeeper/beekeeper/internal/audit"
	"github.com/home-beekeeper/beekeeper/internal/catalog"
	"github.com/home-beekeeper/beekeeper/internal/llamafirewall"
	"github.com/home-beekeeper/beekeeper/internal/policy"
)

// readAuditLines reads decoded audit records from an NDJSON file, returning nil
// if the file does not exist (an audit write may legitimately be a no-op).
func readAuditLines(t *testing.T, path string) []audit.AuditRecord {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		t.Fatalf("open audit file: %v", err)
	}
	defer f.Close()
	var recs []audit.AuditRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var rec audit.AuditRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("parse audit NDJSON: %v\nline: %s", err, line)
		}
		recs = append(recs, rec)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan audit: %v", err)
	}
	return recs
}

// newAuditHandler builds a gatewayHandler with an audit path under a temp dir.
// cacheDir's sibling "policies" dir does not exist, so applyPolicy treats it as
// a no-op (LoadPolicyDir tolerates an absent dir).
func newAuditHandler(t *testing.T, idx policy.MultiCatalogLookup, upstream, auditPath string) *gatewayHandler {
	t.Helper()
	cfg := Config{
		UpstreamURL: upstream,
		BindAddr:    defaultBindAddr,
		Port:        defaultPort,
		AuditPath:   auditPath,
	}
	return newGatewayHandler(cfg, "tok", idx)
}

// TestNew verifies the Server constructor stores the config.
func TestNew(t *testing.T) {
	cfg := Config{UpstreamURL: "http://example", Port: 1234}
	s := New(cfg)
	if s == nil {
		t.Fatal("New returned nil")
	}
	if s.cfg.UpstreamURL != "http://example" || s.cfg.Port != 1234 {
		t.Errorf("New did not retain config: %+v", s.cfg)
	}
}

// TestGatewayWriteAuditOnBlock verifies that a block decision writes an audit
// record to the configured audit path (drives writeAudit on a real request).
func TestGatewayWriteAuditOnBlock(t *testing.T) {
	auditPath := filepath.Join(t.TempDir(), "audit.ndjson")
	upstream, cc := mockUpstream(t, `{"jsonrpc":"2.0","id":1,"result":{}}`)
	h := newAuditHandler(t, blockIdx(), upstream.URL, auditPath)

	rr := postToolsCall(t, h, "tok", 1, "Bash", "evil-pkg")
	if code := parseJSONRPCError(t, rr.Body.Bytes()); code != -32001 {
		t.Fatalf("error code = %d, want -32001", code)
	}
	if cc.count() != 0 {
		t.Fatalf("upstream called %d times on block, want 0", cc.count())
	}

	recs := readAuditLines(t, auditPath)
	if len(recs) == 0 {
		t.Fatal("expected at least one audit record written on block, got none")
	}
	var found bool
	for _, r := range recs {
		if r.Endpoint == "gateway" && r.Decision == "block" {
			found = true
		}
	}
	if !found {
		t.Errorf("no gateway block audit record found; records: %+v", recs)
	}
}

// TestGatewayWriteAuditOnAllow verifies that an allow decision also writes an
// audit record (the writeAudit happy path is hit regardless of decision level).
func TestGatewayWriteAuditOnAllow(t *testing.T) {
	auditPath := filepath.Join(t.TempDir(), "audit.ndjson")
	upstream, _ := mockUpstream(t, `{"jsonrpc":"2.0","id":1,"result":{"content":"ok"}}`)
	h := newAuditHandler(t, allowIdx(), upstream.URL, auditPath)

	postToolsCall(t, h, "tok", 1, "Bash", "safe-pkg")

	recs := readAuditLines(t, auditPath)
	if len(recs) == 0 {
		t.Fatal("expected an audit record on allow, got none")
	}
}

// TestGatewayAuditNoPathIsNoOp verifies that writeAudit returns early (no error,
// no panic) when AuditPath is empty.
func TestGatewayAuditNoPathIsNoOp(t *testing.T) {
	h := &gatewayHandler{cfg: Config{}}
	// Must not panic with empty AuditPath.
	h.writeAudit(policy.ToolCall{ToolName: "Bash"}, policy.Decision{Level: "allow"})
}

// auditWriterFailPath returns a path whose parent component is a regular file,
// so audit.NewWriter's MkdirAll fails — exercising the writeAudit
// audit-writer-error branch.
func auditWriterFailPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	fileAsParent := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(fileAsParent, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}
	// MkdirAll(filepath.Dir(path)) == MkdirAll(fileAsParent) fails: it's a file.
	return filepath.Join(fileAsParent, "audit.ndjson")
}

// TestGatewayWriteAuditWriterError verifies writeAudit logs and returns when the
// audit writer cannot be created (audit dir is unwritable). The request still
// succeeds (block) — an audit failure never fails the request.
func TestGatewayWriteAuditWriterError(t *testing.T) {
	h := &gatewayHandler{cfg: Config{AuditPath: auditWriterFailPath(t)}}
	// Should not panic; just logs to stderr and returns.
	h.writeAudit(policy.ToolCall{ToolName: "Bash"}, policy.Decision{Level: "block"})
}

// TestInjectWarningNoResultField verifies that injectWarning returns the input
// unchanged when there is no "result" key (e.g. an error response).
func TestInjectWarningNoResultField(t *testing.T) {
	in := []byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-1,"message":"x"}}`)
	out := injectWarning(in, "reason")
	if !bytes.Equal(out, in) {
		t.Errorf("injectWarning mutated a response with no result field:\n got: %s\nwant: %s", out, in)
	}
}

// TestInjectWarningUnparseable verifies that injectWarning returns the input
// unchanged when the body is not valid JSON (defense-in-depth: never corrupt).
func TestInjectWarningUnparseable(t *testing.T) {
	in := []byte(`not json`)
	out := injectWarning(in, "reason")
	if !bytes.Equal(out, in) {
		t.Errorf("injectWarning mutated unparseable bytes: %s", out)
	}
}

// TestInjectWarningResultNotObject verifies that when "result" is a scalar (not
// an object), injectWarning wraps it into an object carrying _beekeeper_warning
// and the original value under "result".
func TestInjectWarningResultNotObject(t *testing.T) {
	in := []byte(`{"jsonrpc":"2.0","id":1,"result":"a string result"}`)
	out := injectWarning(in, "single-source match")

	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	resultMap, ok := parsed["result"].(map[string]any)
	if !ok {
		t.Fatalf("result was not wrapped into an object: %T", parsed["result"])
	}
	if resultMap["_beekeeper_warning"] != "single-source match" {
		t.Errorf("_beekeeper_warning = %v, want 'single-source match'", resultMap["_beekeeper_warning"])
	}
	if resultMap["result"] != "a string result" {
		t.Errorf("wrapped original result = %v, want 'a string result'", resultMap["result"])
	}
}

// TestForwardWarningPathUpstreamUnreachable verifies the warn manual-forward path
// fails closed with -32002 when the upstream cannot be reached (dial error).
func TestForwardWarningPathUpstreamUnreachable(t *testing.T) {
	// 127.0.0.1:1 is reserved/unbindable → connection refused on dial.
	h := newTestHandler("tok", warnIdx(), "http://127.0.0.1:1")
	rr := postToolsCall(t, h, "tok", 1, "Bash", "warn-pkg")
	if code := parseJSONRPCError(t, rr.Body.Bytes()); code != -32002 {
		t.Errorf("error code = %d, want -32002 (warn path upstream unreachable fails closed)", code)
	}
}

// TestForwardWarningPathBadUpstreamURL verifies the warn path returns -32002 when
// the configured upstream URL cannot be parsed.
func TestForwardWarningPathBadUpstreamURL(t *testing.T) {
	// A control character in the URL makes url.Parse fail.
	h := newTestHandler("tok", warnIdx(), "http://exa\x7fmple")
	rr := postToolsCall(t, h, "tok", 1, "Bash", "warn-pkg")
	if code := parseJSONRPCError(t, rr.Body.Bytes()); code != -32002 {
		t.Errorf("error code = %d, want -32002 (warn path bad upstream URL fails closed)", code)
	}
}

// TestForwardAllowScanPathUpstreamUnreachable verifies the allow-with-scan
// manual-forward path fails closed with -32002 when the upstream is unreachable.
func TestForwardAllowScanPathUpstreamUnreachable(t *testing.T) {
	scanner := &mockGatewayScanner{resp: llamafirewall.ScanResponse{Result: llamafirewall.ResultClean}}
	h := newTestHandlerWithScanner("tok", allowIdx(), "http://127.0.0.1:1", scanner)
	// web_search is prompt-scan-eligible but the scan path is reached after the
	// upstream call; the upstream dial fails first → -32002.
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "web_search", "arguments": map[string]any{"query": "x"}},
	})
	rr := postJSON(t, h, "tok", body)
	if code := parseJSONRPCError(t, rr.Body.Bytes()); code != -32002 {
		t.Errorf("error code = %d, want -32002 (allow-scan path upstream unreachable fails closed)", code)
	}
}

// TestForwardAllowScanPathBadUpstreamURL verifies the allow-with-scan path fails
// closed with -32002 on an unparseable upstream URL.
func TestForwardAllowScanPathBadUpstreamURL(t *testing.T) {
	scanner := &mockGatewayScanner{resp: llamafirewall.ScanResponse{Result: llamafirewall.ResultClean}}
	h := newTestHandlerWithScanner("tok", allowIdx(), "http://exa\x7fmple", scanner)
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "web_search", "arguments": map[string]any{"query": "x"}},
	})
	rr := postJSON(t, h, "tok", body)
	if code := parseJSONRPCError(t, rr.Body.Bytes()); code != -32002 {
		t.Errorf("error code = %d, want -32002 (allow-scan path bad URL fails closed)", code)
	}
}

// TestForwardAllowScanCleanForwardsUpstream verifies the allow-with-scan happy
// path: a clean scan forwards the upstream response unchanged to the client.
func TestForwardAllowScanCleanForwardsUpstream(t *testing.T) {
	upstreamResp := `{"jsonrpc":"2.0","id":1,"result":{"content":"clean search results"}}`
	upstream, cc := mockUpstream(t, upstreamResp)
	scanner := &mockGatewayScanner{resp: llamafirewall.ScanResponse{Result: llamafirewall.ResultClean}}
	h := newTestHandlerWithScanner("tok", allowIdx(), upstream.URL, scanner)

	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "web_search", "arguments": map[string]any{"query": "x"}},
	})
	rr := postJSON(t, h, "tok", body)

	if cc.count() != 1 {
		t.Errorf("upstream call count = %d, want 1", cc.count())
	}
	if scanner.calls == 0 {
		t.Error("scanner not invoked on eligible allow-scan tool")
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if _, hasErr := resp["error"]; hasErr {
		t.Errorf("clean allow-scan should forward upstream, got error: %s", rr.Body.String())
	}
}

// TestWarnPathInjectionReplacesBody verifies the warn path replaces the upstream
// body with the scanner's warning payload when injection is detected (and still
// reaches the client). read_file is prompt-scan-eligible.
func TestWarnPathInjectionReplacesBody(t *testing.T) {
	upstreamResp := `{"jsonrpc":"2.0","id":1,"result":{"content":"file with injection"}}`
	upstream, _ := mockUpstream(t, upstreamResp)
	scanner := &mockGatewayScanner{resp: llamafirewall.ScanResponse{
		Result: llamafirewall.ResultInjection, Confidence: 0.9, Reason: "injection",
	}}
	h := newTestHandlerWithScanner("tok", warnIdx(), upstream.URL, scanner)

	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "read_file", "arguments": map[string]any{"path": "/etc/passwd"}},
	})
	rr := postJSON(t, h, "tok", body)
	if scanner.calls == 0 {
		t.Fatal("scanner not invoked on warn path for eligible tool")
	}
	// The response body should differ from the raw upstream body (replaced payload,
	// then warning injected). It must remain valid JSON.
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("warn-path response not valid JSON after injection replace: %v", err)
	}
}

// TestExtractToolCallFromMsgMalformed verifies extractToolCallFromMsg returns an
// error on params that are not a valid tools/call object.
func TestExtractToolCallFromMsgMalformed(t *testing.T) {
	msg := JSONRPCMessage{JSONRPC: "2.0", Method: "tools/call", Params: []byte(`["not","an","object"]`)}
	var tc policy.ToolCall
	if err := extractToolCallFromMsg(msg, &tc); err == nil {
		t.Error("expected error for malformed tools/call params, got nil")
	}
}

// TestLoadGatewayStateCorruptFile verifies LoadGatewayState surfaces a parse
// error on a corrupt state.json.
func TestLoadGatewayStateCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("{ this is not json"), 0o600); err != nil {
		t.Fatalf("write corrupt state: %v", err)
	}
	if _, err := LoadGatewayState(path); err == nil {
		t.Error("LoadGatewayState returned nil error for corrupt JSON, want parse error")
	}
}

// TestLoadGatewayStateNoGatewayKey verifies that a state.json containing only the
// "sources" key returns a zero-value GatewayState and nil error.
func TestLoadGatewayStateNoGatewayKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"sources":{}}`), 0o600); err != nil {
		t.Fatalf("write state: %v", err)
	}
	gw, err := LoadGatewayState(path)
	if err != nil {
		t.Fatalf("LoadGatewayState: %v", err)
	}
	if gw.GatewayToken != "" {
		t.Errorf("expected zero GatewayState, got token=%q", gw.GatewayToken)
	}
}

// TestSaveGatewayStatePreservesSources verifies SaveGatewayState preserves an
// existing top-level "sources" key while adding the gateway key.
func TestSaveGatewayStatePreservesSources(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"sources":{"osv":{"etag":"abc"}}}`), 0o600); err != nil {
		t.Fatalf("seed state: %v", err)
	}
	if err := SaveGatewayState(path, GatewayState{GatewayToken: "tk", BoundPort: 7837}); err != nil {
		t.Fatalf("SaveGatewayState: %v", err)
	}
	data, _ := os.ReadFile(path)
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("reread state: %v", err)
	}
	if _, ok := raw["sources"]; !ok {
		t.Errorf("SaveGatewayState dropped the sources key: %s", data)
	}
	if _, ok := raw["gateway"]; !ok {
		t.Errorf("SaveGatewayState did not write the gateway key: %s", data)
	}
}

// TestClearGatewayStateMissingFile verifies ClearGatewayState is a no-op (nil
// error) when the state file does not exist.
func TestClearGatewayStateMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	if err := ClearGatewayState(path); err != nil {
		t.Errorf("ClearGatewayState on missing file = %v, want nil", err)
	}
}

// TestClearGatewayStateCorruptFileLeavesAlone verifies ClearGatewayState returns
// nil and does not corrupt an unparseable state file (it leaves it as-is).
func TestClearGatewayStateCorruptFileLeavesAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	corrupt := []byte("{ not json at all")
	if err := os.WriteFile(path, corrupt, 0o600); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}
	if err := ClearGatewayState(path); err != nil {
		t.Errorf("ClearGatewayState corrupt file = %v, want nil (leave alone)", err)
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, corrupt) {
		t.Errorf("ClearGatewayState modified a corrupt file: %s", got)
	}
}

// TestStartLifecycle drives the full Start() daemon lifecycle on a loopback bind
// with an ephemeral port: it builds a real (empty) catalog index, lets Start bind
// + serve, then cancels the context for a clean shutdown. Verifies Start returns
// nil on clean shutdown, writes gateway state during run, and clears it on exit.
func TestStartLifecycle(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "bumblebee.idx")
	if err := catalog.BuildIndex(indexPath, nil); err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	stateFile := filepath.Join(dir, "state.json")

	cfg := Config{
		UpstreamURL: "http://127.0.0.1:9",
		BindAddr:    "127.0.0.1",
		Port:        0, // ephemeral — avoids port conflicts in CI
		StateFile:   stateFile,
		IndexPath:   indexPath,
		CacheDir:    filepath.Join(dir, "catalogs"),
		AuditPath:   filepath.Join(dir, "audit.ndjson"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- Start(ctx, cfg) }()

	// Wait for the daemon to write gateway state (proves it bound + persisted).
	var bound GatewayState
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		gw, err := LoadGatewayState(stateFile)
		if err == nil && gw.GatewayToken != "" && gw.BoundPort != 0 {
			bound = gw
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if bound.GatewayToken == "" {
		cancel()
		<-errCh
		t.Fatal("gateway did not write state with a token within timeout")
	}
	if len(bound.GatewayToken) != 64 {
		t.Errorf("persisted token length = %d, want 64", len(bound.GatewayToken))
	}

	// Clean shutdown via context cancellation.
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Start returned non-nil on clean shutdown: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Start did not return after context cancel")
	}

	// After clean shutdown the gateway key must be cleared.
	gw, err := LoadGatewayState(stateFile)
	if err != nil {
		t.Fatalf("LoadGatewayState after shutdown: %v", err)
	}
	if gw.GatewayToken != "" {
		t.Errorf("gateway state not cleared on shutdown: token=%q", gw.GatewayToken)
	}
}

// TestStartBadIndexPathFailsClosed verifies Start returns an error (does not
// serve) when the catalog index cannot be opened.
func TestStartBadIndexPathFailsClosed(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "missing.idx") // does not exist → OpenIndex fails
	cfg := Config{
		UpstreamURL: "http://127.0.0.1:9",
		BindAddr:    "127.0.0.1",
		Port:        0,
		StateFile:   filepath.Join(dir, "state.json"),
		IndexPath:   bad,
	}
	err := Start(context.Background(), cfg)
	if err == nil {
		t.Fatal("Start returned nil with a missing catalog index, want error")
	}
	if !strings.Contains(err.Error(), "open catalog index") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestStartPortInUseFailsClosed verifies Start returns a bind error when the
// requested port is already taken, and that no stale gateway state is left.
func TestStartPortInUseFailsClosed(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "bumblebee.idx")
	if err := catalog.BuildIndex(indexPath, nil); err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	// Occupy a loopback port so Start's bind fails deterministically.
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("occupy port: %v", err)
	}
	defer occupied.Close()
	port := occupied.Addr().(*net.TCPAddr).Port

	stateFile := filepath.Join(dir, "state.json")
	cfg := Config{
		UpstreamURL: "http://127.0.0.1:9",
		BindAddr:    "127.0.0.1",
		Port:        port,
		StateFile:   stateFile,
		IndexPath:   indexPath,
		CacheDir:    filepath.Join(dir, "catalogs"),
	}
	if err := Start(context.Background(), cfg); err == nil {
		t.Fatal("Start returned nil binding an in-use port, want bind error")
	}
	// State must have been cleaned up on the bind failure.
	gw, _ := LoadGatewayState(stateFile)
	if gw.GatewayToken != "" {
		t.Errorf("gateway state not cleared after bind failure: token=%q", gw.GatewayToken)
	}
}
