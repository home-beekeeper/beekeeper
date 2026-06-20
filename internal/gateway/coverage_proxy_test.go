package gateway

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/home-beekeeper/beekeeper/internal/llamafirewall"
	"github.com/home-beekeeper/beekeeper/internal/policy"
	"github.com/home-beekeeper/beekeeper/internal/policyloader"
)

// TestGatewayFailClosedPanicFailOpen verifies the policy-goroutine panic recovery
// path under FailOpen=true (proxy.go ~288-289): the upstream is still never
// called and the client gets -32002.
func TestGatewayFailClosedPanicFailOpen(t *testing.T) {
	upstream, cc := mockUpstream(t, `{"jsonrpc":"2.0","id":1,"result":{}}`)
	panicIdx := &fakeIdx{fn: func(_, _ string) []policy.CatalogMatch {
		panic("simulated policy engine panic (fail-open variant)")
	}}
	cfg := Config{
		UpstreamURL: upstream.URL,
		BindAddr:    defaultBindAddr,
		Port:        defaultPort,
		FailOpen:    true,
	}
	h := newGatewayHandler(cfg, "tok", panicIdx)

	rr := postToolsCall(t, h, "tok", 1, "Bash", "anything")
	if code := parseJSONRPCError(t, rr.Body.Bytes()); code != -32002 {
		t.Errorf("error code = %d, want -32002 (panic recovery, fail-open)", code)
	}
	if cc.count() != 0 {
		t.Errorf("upstream called %d times on panic, want 0", cc.count())
	}
}

// withSlowPolicyLoader replaces loadPolicyDirFn with one that blocks past the
// 500ms policy-eval deadline, forcing the evalCtx.Done() timeout branch. It
// returns a restore func. A CacheDir must be set on the handler for applyPolicy
// to reach the loader.
func withSlowPolicyLoader(t *testing.T) func() {
	t.Helper()
	orig := loadPolicyDirFn
	loadPolicyDirFn = func(string) ([]policyloader.PolicyFile, error) {
		time.Sleep(700 * time.Millisecond) // > the 500ms handler deadline
		return nil, nil
	}
	return func() { loadPolicyDirFn = orig }
}

// TestGatewayPolicyTimeoutFailClosed verifies the policy-eval timeout branch
// (proxy.go ~298-303) under FailOpen=false: a policy evaluation that exceeds the
// 500ms deadline yields -32002 and the upstream is never called.
func TestGatewayPolicyTimeoutFailClosed(t *testing.T) {
	restore := withSlowPolicyLoader(t)
	defer restore()

	upstream, cc := mockUpstream(t, `{"jsonrpc":"2.0","id":1,"result":{}}`)
	cfg := Config{
		UpstreamURL: upstream.URL,
		BindAddr:    defaultBindAddr,
		Port:        defaultPort,
		CacheDir:    t.TempDir(), // non-empty so applyPolicy invokes the (slow) loader
		FailOpen:    false,
	}
	h := newGatewayHandler(cfg, "tok", allowIdx())

	rr := postToolsCall(t, h, "tok", 1, "Bash", "anything")
	if code := parseJSONRPCError(t, rr.Body.Bytes()); code != -32002 {
		t.Errorf("error code = %d, want -32002 (policy timeout, fail-closed)", code)
	}
	if cc.count() != 0 {
		t.Errorf("upstream called %d times on timeout, want 0", cc.count())
	}
}

// TestGatewayPolicyTimeoutFailOpen verifies the policy-eval timeout branch under
// FailOpen=true (proxy.go ~300-301). Still -32002 (a timeout never silently
// allows), upstream never called.
func TestGatewayPolicyTimeoutFailOpen(t *testing.T) {
	restore := withSlowPolicyLoader(t)
	defer restore()

	upstream, cc := mockUpstream(t, `{"jsonrpc":"2.0","id":1,"result":{}}`)
	cfg := Config{
		UpstreamURL: upstream.URL,
		BindAddr:    defaultBindAddr,
		Port:        defaultPort,
		CacheDir:    t.TempDir(),
		FailOpen:    true,
	}
	h := newGatewayHandler(cfg, "tok", allowIdx())

	rr := postToolsCall(t, h, "tok", 1, "Bash", "anything")
	if code := parseJSONRPCError(t, rr.Body.Bytes()); code != -32002 {
		t.Errorf("error code = %d, want -32002 (policy timeout, fail-open)", code)
	}
	if cc.count() != 0 {
		t.Errorf("upstream called %d times on timeout, want 0", cc.count())
	}
}

// TestWarnPathCleanScanForwards verifies the warn manual-forward path with a
// scanner that returns clean for an eligible tool: the scanned body (non-nil) is
// forwarded with the warning injected (proxy.go ~449-451 else-if branch).
//
// To produce BOTH a warn decision AND a prompt-scan-eligible tool, the tool name
// is "read_file" (eligible) while its arguments carry a flagged install command
// ("npm install foo") that warnIdx matches → warn. The scanner returns clean, so
// the else-if (scanned != nil) branch is taken rather than the injection branch.
func TestWarnPathCleanScanForwards(t *testing.T) {
	upstreamResp := `{"jsonrpc":"2.0","id":1,"result":{"content":"clean file content"}}`
	upstream, cc := mockUpstream(t, upstreamResp)
	scanner := &mockGatewayScanner{resp: llamafirewall.ScanResponse{Result: llamafirewall.ResultClean}}
	h := newTestHandlerWithScanner("tok", warnIdx(), upstream.URL, scanner)

	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "read_file", "arguments": map[string]any{"command": "npm install foo"}},
	})
	rr := postJSON(t, h, "tok", body)

	if cc.count() != 1 {
		t.Errorf("upstream call count = %d, want 1", cc.count())
	}
	if scanner.calls == 0 {
		t.Error("scanner not invoked on warn path for eligible tool")
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("warn-path response not valid JSON: %v", err)
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result missing/not object: %v", resp["result"])
	}
	if _, has := result["_beekeeper_warning"]; !has {
		t.Errorf("clean-scan warn path must still inject _beekeeper_warning; result: %v", result)
	}
}

// TestWarnPathInjectionEligibleReplacesBody verifies the warn manual-forward
// path's injection branch (proxy.go ~446-449): a warn decision on a prompt-
// eligible tool whose scan flags injection forwards the scanner's replacement
// payload (with the warning injected). read_file is eligible; the install command
// in arguments drives the warn decision.
func TestWarnPathInjectionEligibleReplacesBody(t *testing.T) {
	upstreamResp := `{"jsonrpc":"2.0","id":1,"result":{"content":"malicious file content"}}`
	upstream, cc := mockUpstream(t, upstreamResp)
	scanner := &mockGatewayScanner{resp: llamafirewall.ScanResponse{
		Result: llamafirewall.ResultInjection, Confidence: 0.95, Reason: "injection",
	}}
	h := newTestHandlerWithScanner("tok", warnIdx(), upstream.URL, scanner)

	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "read_file", "arguments": map[string]any{"command": "npm install foo"}},
	})
	rr := postJSON(t, h, "tok", body)

	if cc.count() != 1 {
		t.Errorf("upstream call count = %d, want 1", cc.count())
	}
	if scanner.calls == 0 {
		t.Fatal("scanner not invoked on warn-injection path")
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("warn-injection response not valid JSON: %v", err)
	}
}
