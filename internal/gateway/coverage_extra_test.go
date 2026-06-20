package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/home-beekeeper/beekeeper/internal/nudge"
	"github.com/home-beekeeper/beekeeper/internal/policy"
)

// --- parser internals -------------------------------------------------------

// TestRejectDuplicateKeysEmptyInput verifies scanDuplicateKeys returns nil on an
// empty token stream (the io.EOF branch).
func TestRejectDuplicateKeysEmptyInput(t *testing.T) {
	if err := rejectDuplicateKeys([]byte("")); err != nil {
		t.Errorf("rejectDuplicateKeys(empty) = %v, want nil (EOF → nil)", err)
	}
}

// TestRejectDuplicateKeysTruncatedObject verifies scanDuplicateKeys surfaces the
// decoder error when an object is truncated mid-stream (dec.Token error branch).
func TestRejectDuplicateKeysTruncatedObject(t *testing.T) {
	// "{" opens an object; the next Token() inside the for-More loop errors.
	if err := rejectDuplicateKeys([]byte(`{"a":`)); err == nil {
		t.Error("rejectDuplicateKeys(truncated object) = nil, want decoder error")
	}
}

// TestRejectDuplicateKeysTruncatedArray verifies the array recurse path errors on
// a truncated array element.
func TestRejectDuplicateKeysTruncatedArray(t *testing.T) {
	if err := rejectDuplicateKeys([]byte(`[{"a":1`)); err == nil {
		t.Error("rejectDuplicateKeys(truncated array) = nil, want decoder error")
	}
}

// TestRejectDuplicateKeysNestedArrayOfScalars verifies the array branch walks an
// array of scalars cleanly (no false positive).
func TestRejectDuplicateKeysNestedArrayOfScalars(t *testing.T) {
	if err := rejectDuplicateKeys([]byte(`{"items":[1,2,3],"ok":true}`)); err != nil {
		t.Errorf("rejectDuplicateKeys(valid array) = %v, want nil", err)
	}
}

// TestRejectDuplicateKeysDeepDuplicate verifies a duplicate key buried in a
// nested object is caught.
func TestRejectDuplicateKeysDeepDuplicate(t *testing.T) {
	if err := rejectDuplicateKeys([]byte(`{"a":{"b":1,"b":2}}`)); err == nil {
		t.Error("rejectDuplicateKeys(nested duplicate) = nil, want duplicate-key error")
	}
}

// TestCheckDepthEmptyAndNull verifies checkDepth no-ops on empty and null params.
func TestCheckDepthEmptyAndNull(t *testing.T) {
	if err := checkDepth(nil, 0); err != nil {
		t.Errorf("checkDepth(nil) = %v, want nil", err)
	}
	if err := checkDepth(json.RawMessage(`null`), 0); err != nil {
		t.Errorf("checkDepth(null) = %v, want nil", err)
	}
}

// TestCheckDepthDepthExceededAtEntry verifies the early depth>max guard in
// checkDepth (depth parameter already over the limit).
func TestCheckDepthDepthExceededAtEntry(t *testing.T) {
	if err := checkDepth(json.RawMessage(`{}`), maxRecursionDepth+1); err == nil {
		t.Error("checkDepth(depth over max) = nil, want depth-exceeded error")
	}
}

// TestCheckDepthUnmarshalErrorIgnored verifies checkDepth returns nil on
// unparseable params (parseSingle reports the real error; checkDepth ignores it).
func TestCheckDepthUnmarshalErrorIgnored(t *testing.T) {
	if err := checkDepth(json.RawMessage(`{not json`), 0); err != nil {
		t.Errorf("checkDepth(invalid json) = %v, want nil (ignored here)", err)
	}
}

// TestCheckValueDepthArrayNesting verifies checkValueDepth descends into arrays
// and flags an array nested past the limit.
func TestCheckValueDepthArrayNesting(t *testing.T) {
	// Shallow array of objects — must pass.
	if err := checkValueDepth([]any{map[string]any{"k": 1}}, 0); err != nil {
		t.Errorf("checkValueDepth(shallow array) = %v, want nil", err)
	}
	// An array nested deeper than maxRecursionDepth must error.
	var deep any = "leaf"
	for i := 0; i < maxRecursionDepth+2; i++ {
		deep = []any{deep}
	}
	if err := checkValueDepth(deep, 0); err == nil {
		t.Error("checkValueDepth(deep array) = nil, want depth-exceeded error")
	}
}

// TestParseMessageDepthExceededViaArrays verifies that ParseMessage rejects a
// params value with deeply nested arrays (-32600), exercising the array branch
// end-to-end.
func TestParseMessageDepthExceededViaArrays(t *testing.T) {
	deep := strings.Repeat("[", maxRecursionDepth+3) + "1" + strings.Repeat("]", maxRecursionDepth+3)
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":` + deep + `}`)
	_, err := ParseMessage(body)
	if err == nil {
		t.Fatal("ParseMessage(deeply nested array params) = nil error, want -32600")
	}
	pe, ok := err.(*ParseError)
	if !ok || pe.Code != -32600 {
		t.Errorf("error = %v, want *ParseError code -32600", err)
	}
}

// --- drift realMetadataFetch parse branches ---------------------------------

// driftServer returns an httptest server emitting raw bodies per PM path.
// A nil/empty body for a PM means that path is omitted (404).
func driftServer(t *testing.T, pnpmBody, bunBody string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/pnpm/"):
			if pnpmBody == "" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(pnpmBody))
		case strings.Contains(r.URL.Path, "/bun/"):
			if bunBody == "" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(bunBody))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestRealMetadataFetchParseError verifies per-PM fail-open when pnpm returns
// invalid JSON: pnpm is omitted, bun is present, nil error.
func TestRealMetadataFetchParseError(t *testing.T) {
	srv := driftServer(t, `{"latest": not-json}`, `{"latest":"1.4.0"}`)
	orig := npmDriftRegistryBase
	npmDriftRegistryBase = srv.URL
	defer func() { npmDriftRegistryBase = orig }()

	versions, err := realMetadataFetch(context.Background())
	if err != nil {
		t.Fatalf("realMetadataFetch error = %v, want nil (per-PM fail-open)", err)
	}
	if _, ok := versions["pnpm"]; ok {
		t.Errorf("pnpm present despite parse error: %v", versions["pnpm"])
	}
	if versions["bun"] != "1.4.0" {
		t.Errorf("bun = %q, want 1.4.0", versions["bun"])
	}
}

// TestRealMetadataFetchEmptyLatest verifies that an empty "latest" field omits
// the PM (the empty-latest fail-open branch).
func TestRealMetadataFetchEmptyLatest(t *testing.T) {
	srv := driftServer(t, `{"latest":""}`, `{"latest":"1.4.0"}`)
	orig := npmDriftRegistryBase
	npmDriftRegistryBase = srv.URL
	defer func() { npmDriftRegistryBase = orig }()

	versions, err := realMetadataFetch(context.Background())
	if err != nil {
		t.Fatalf("realMetadataFetch error = %v, want nil", err)
	}
	if _, ok := versions["pnpm"]; ok {
		t.Errorf("pnpm present despite empty latest: %v", versions["pnpm"])
	}
	if versions["bun"] != "1.4.0" {
		t.Errorf("bun = %q, want 1.4.0", versions["bun"])
	}
}

// TestRealMetadataFetchNetworkError verifies a dial error omits both PMs and
// returns nil error (both fail-open). Points the base at an unreachable port.
func TestRealMetadataFetchNetworkError(t *testing.T) {
	orig := npmDriftRegistryBase
	npmDriftRegistryBase = "http://127.0.0.1:1"
	defer func() { npmDriftRegistryBase = orig }()

	versions, err := realMetadataFetch(context.Background())
	if err != nil {
		t.Fatalf("realMetadataFetch error = %v, want nil (network fail-open)", err)
	}
	if len(versions) != 0 {
		t.Errorf("versions = %v, want empty map on total network failure", versions)
	}
}

// --- proxy: advisory cap with a block decision ------------------------------

// TestGatewayAdvisoryCapDoesNotSuppressBlock verifies that when nudge config is
// in block mode, the per-session advisory cap never suppresses a block: a second
// install for the same agent still blocks (proxy.go line ~355 block branch).
func TestGatewayAdvisoryCapDoesNotSuppressBlock(t *testing.T) {
	orig := nudge.DetectStateFn
	nudge.DetectStateFn = func(_ context.Context, _ nudge.Config) nudge.PMState {
		// No pnpm installed → block mode offers pnpm and denies npm install.
		return nudge.PMState{NodeVersion: "22.5.0"}
	}
	defer func() { nudge.DetectStateFn = orig }()

	ncfg := nudge.DefaultConfig()
	ncfg.Mode = "block"

	cfg := Config{
		UpstreamURL: "http://upstream-unused",
		BindAddr:    defaultBindAddr,
		Port:        defaultPort,
		Nudge:       ncfg,
	}
	h := newGatewayHandler(cfg, "tok", allowIdx())

	installBody := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"Bash","arguments":{"command":"npm install lodash"}}}`)
	send := func() *int {
		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(installBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer tok")
		req.Header.Set("X-Beekeeper-Agent-Id", "agent-block")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		var resp struct {
			Error *struct {
				Code int `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("response not valid JSON: %v\nbody: %s", err, rr.Body.String())
		}
		if resp.Error == nil {
			return nil
		}
		return &resp.Error.Code
	}

	if c := send(); c == nil || *c != -32001 {
		t.Fatalf("first install: got code %v, want -32001 (block mode denies npm install)", c)
	}
	// Second install for the SAME agent must still block — the advisory cap only
	// suppresses warn-level advisories, never a block.
	if c := send(); c == nil || *c != -32001 {
		t.Errorf("second install: got code %v, want -32001 (block must not be capped)", c)
	}
}

// --- state: read error on a directory ---------------------------------------

// TestLoadGatewayStateReadError verifies LoadGatewayState surfaces a non-NotExist
// read error: pointing it at a directory yields a read error (not ErrNotExist).
func TestLoadGatewayStateReadError(t *testing.T) {
	dir := t.TempDir() // a directory, not a file
	_, err := LoadGatewayState(dir)
	if err == nil {
		t.Error("LoadGatewayState(directory) = nil error, want read error")
	}
}

// TestClearGatewayStateReadErrorOnDir verifies ClearGatewayState surfaces a
// non-NotExist read error when the path is a directory.
func TestClearGatewayStateReadErrorOnDir(t *testing.T) {
	dir := t.TempDir()
	if err := ClearGatewayState(dir); err == nil {
		t.Error("ClearGatewayState(directory) = nil error, want read error")
	}
}

// TestWriteStateFileAtomicCreateTempError verifies writeStateFileAtomic returns
// an error when the temp file cannot be created (parent dir does not exist).
func TestWriteStateFileAtomicCreateTempError(t *testing.T) {
	// Parent directory does not exist → os.CreateTemp fails.
	path := filepath.Join(t.TempDir(), "no-such-subdir", "state.json")
	if err := writeStateFileAtomic(path, []byte("{}")); err == nil {
		t.Error("writeStateFileAtomic with missing parent dir = nil, want create-temp error")
	}
}

// TestSaveGatewayStateMkdirError verifies SaveGatewayState fails when the parent
// path component is a regular file (MkdirAll cannot create the directory).
func TestSaveGatewayStateMkdirError(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	// blocker/state.json — blocker is a file, so MkdirAll(blocker) fails.
	path := filepath.Join(blocker, "state.json")
	if err := SaveGatewayState(path, GatewayState{GatewayToken: "tk"}); err == nil {
		t.Error("SaveGatewayState under a file-as-dir = nil, want mkdir error")
	}
}

// TestApplyPolicyAllowNoPolicyFiles is a small belt-and-suspenders check that the
// allow path returns allow when no catalog match and no policy files exist.
func TestApplyPolicyAllowNoPolicyFiles(t *testing.T) {
	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params:  []byte(`{"name":"Bash","arguments":{"command":"ls"}}`),
	}
	d := applyPolicy(msg, allowIdx(), Config{}, policy.AgentContext{})
	if d.Level != "allow" {
		t.Errorf("decision.Level = %q, want allow (no match, no policy)", d.Level)
	}
}
