package gateway

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mzansi-agentive/beekeeper/internal/audit"
	"github.com/mzansi-agentive/beekeeper/internal/policy"
)

const bearerPrefix = "Bearer "

// gatewayHandler is the per-request HTTP handler for the MCP gateway. It
// enforces token authentication on every request and routes tools/call methods
// through the policy engine before forwarding.
//
// Tool-call methods are handled manually (handleToolCall) to allow writing
// JSON-RPC error responses to the client on block decisions. Non-tool-call
// methods are forwarded transparently via httputil.ReverseProxy (Pitfall 3:
// never use ReverseProxy as sole handler for tools/call).
type gatewayHandler struct {
	token        string
	reverseProxy *httputil.ReverseProxy
	cfg          Config
	idx          policy.MultiCatalogLookup
}

// newGatewayHandler constructs a gatewayHandler for the given upstream URL.
// Uses Rewrite (deprecated httputil proxy API not used; T-04-03-12 mitigation)
// to set the upstream target.
func newGatewayHandler(cfg Config, token string, idx policy.MultiCatalogLookup) *gatewayHandler {
	upstream, _ := url.Parse(cfg.UpstreamURL)

	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(upstream)
			pr.SetXForwarded()
		},
	}

	return &gatewayHandler{
		token:        token,
		reverseProxy: rp,
		cfg:          cfg,
		idx:          idx,
	}
}

// ServeHTTP implements http.Handler for the MCP gateway. It applies the
// following pipeline on every request:
//
//  1. Token auth check: Authorization: Bearer <token> via constant-time compare
//     (T-04-03-02). Missing or wrong token → JSON-RPC -32600 error.
//  2. Bounded body read: io.LimitReader capped at maxRequestBody+1
//     (T-04-03-03). Oversized body → JSON-RPC -32700 error.
//  3. JSON-RPC parsing: ParseMessage enforces all bounds
//     (T-04-03-04, T-04-03-05, T-04-03-09).
//  4. Method routing:
//     - "tools/call" → handleToolCall (policy gate + fail-closed recover)
//     - everything else → transparent ReverseProxy forward
func (h *gatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Step 1: token auth (T-04-03-02: constant-time comparison).
	if !h.verifyToken(r) {
		writeJSONRPCError(w, nil, -32600, "unauthorized", nil)
		return
	}

	// Step 2: bounded body read (T-04-03-03).
	limited := io.LimitReader(r.Body, maxRequestBody+1)
	bodyBytes, err := io.ReadAll(limited)
	if err != nil {
		writeJSONRPCError(w, nil, -32700, "body read error", nil)
		return
	}
	if int64(len(bodyBytes)) > maxRequestBody {
		writeJSONRPCError(w, nil, -32700, "request body exceeds 1MB cap", nil)
		return
	}

	// Step 3: JSON-RPC parsing with all bounds enforced.
	msg, parseErr := ParseMessage(bodyBytes)
	if parseErr != nil {
		pe, _ := parseErr.(*ParseError)
		code := -32700
		if pe != nil {
			code = pe.Code
		}
		writeJSONRPCError(w, nil, code, "parse error: "+parseErr.Error(), nil)
		return
	}

	// Step 4: method routing.
	if msg.Method == "tools/call" {
		h.handleToolCall(w, r, msg, bodyBytes)
		return
	}

	// Transparent forward for all other methods (e.g. tools/list, resources/list,
	// initialize). Pitfall 7: pass initialize through — MCP July 2026 removed
	// the handshake but older clients may still send it.
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	h.reverseProxy.ServeHTTP(w, r)
}

// policyResult carries the outcome of an applyPolicy call run in a goroutine.
type policyResult struct {
	d policy.Decision
}

// handleToolCall evaluates the tool call against the policy engine and either:
//   - block   → writes JSON-RPC -32001 error (upstream NEVER called)
//   - warn    → forwards to upstream AND injects _beekeeper_warning into response
//   - allow   → forwards to upstream transparently
//
// Fail-closed guard: a deferred recover() catches any policy engine panic and
// writes JSON-RPC -32002 to the client. The upstream is NEVER called on panic
// (T-04-03-06: TestGatewayFailClosed verifies this invariant).
func (h *gatewayHandler) handleToolCall(w http.ResponseWriter, r *http.Request, msg JSONRPCMessage, bodyBytes []byte) {
	// Fail-closed guard: any policy engine panic → JSON-RPC -32002 error.
	// The recover must be the first defer so it catches panics from anything below.
	defer func() {
		if rec := recover(); rec != nil {
			fmt.Fprintf(os.Stderr, "beekeeper gateway: recovered panic in handleToolCall: %v\n", rec)
			if h.cfg.FailOpen {
				writeJSONRPCError(w, msg.ID, -32002, "internal error (fail-open: reduced security)", nil)
			} else {
				writeJSONRPCError(w, msg.ID, -32002, "internal error (fail-closed)", nil)
			}
		}
	}()

	// 500ms hard deadline for policy evaluation (CONTEXT.md: <100ms p95 target).
	// CR-01: run applyPolicy in a goroutine so the deadline actually interrupts it
	// if a catalog adapter blocks past the timeout.
	evalCtx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()

	ac := extractAgentContext(r)

	ch := make(chan policyResult, 1)
	go func() {
		ch <- policyResult{d: applyPolicy(msg, h.idx, ac)}
	}()

	var decision policy.Decision
	select {
	case res := <-ch:
		decision = res.d
	case <-evalCtx.Done():
		// Distinguish client disconnect from genuine policy timeout for logging clarity.
		if evalCtx.Err() == context.DeadlineExceeded {
			fmt.Fprintf(os.Stderr, "beekeeper gateway: policy evaluation timeout (500ms)\n")
			if h.cfg.FailOpen {
				writeJSONRPCError(w, msg.ID, -32002, "policy timeout (fail-open: reduced security)", nil)
			} else {
				writeJSONRPCError(w, msg.ID, -32002, "policy timeout (fail-closed)", nil)
			}
		} else {
			fmt.Fprintf(os.Stderr, "beekeeper gateway: client disconnected during policy evaluation\n")
			writeJSONRPCError(w, msg.ID, -32002, "request cancelled (fail-closed)", nil)
		}
		return
	}

	// Write audit record regardless of decision level (never errors the request).
	tc := policy.ToolCall{}
	if err := extractToolCallFromMsg(msg, &tc); err == nil {
		h.writeAudit(tc, decision)
	}

	switch decision.Level {
	case "block":
		writeJSONRPCError(w, msg.ID, -32001, "blocked by Beekeeper: "+decision.Reason, map[string]any{
			"decision": decision.Level,
			"reason":   decision.Reason,
			"rule_ids": decision.RuleIDs,
		})
		return

	case "warn":
		// Forward to upstream and inject _beekeeper_warning into the response.
		h.forwardWithWarningInjection(w, r, msg, bodyBytes, decision)
		return

	default: // "allow"
		// Restore body for transparent forward.
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		h.reverseProxy.ServeHTTP(w, r)
	}
}

// forwardWithWarningInjection forwards a warn-level tool call to the upstream
// MCP server and injects a _beekeeper_warning field into the JSON-RPC result.
// The warning is a vendor extension field (underscore prefix per MCP convention;
// Assumption A3 in RESEARCH.md: MCP clients ignore unknown fields).
func (h *gatewayHandler) forwardWithWarningInjection(w http.ResponseWriter, r *http.Request, msg JSONRPCMessage, bodyBytes []byte, decision policy.Decision) {
	// Make a direct HTTP request to the upstream rather than using ReverseProxy,
	// so we can capture and modify the response before writing it to the client.
	//
	// WR-02: use url.JoinPath to avoid double-slash when UpstreamURL has a
	// trailing slash (e.g. "http://localhost:3000/" + "/mcp" → double slash).
	base, err := url.Parse(strings.TrimRight(h.cfg.UpstreamURL, "/"))
	if err != nil {
		writeJSONRPCError(w, msg.ID, -32002, "upstream URL invalid (fail-closed)", nil)
		return
	}
	upstreamURL := base.JoinPath(r.URL.Path).String()
	req, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, bytes.NewReader(bodyBytes))
	if err != nil {
		writeJSONRPCError(w, msg.ID, -32002, "upstream request creation failed (fail-closed)", nil)
		return
	}

	// Copy original request headers (excluding hop-by-hop headers).
	// CR-02: never forward the Beekeeper gateway Authorization header to the
	// upstream MCP server — the gateway token is an internal secret and must
	// not appear in upstream logs or be accepted by the upstream server.
	for k, vv := range r.Header {
		if strings.EqualFold(k, "Authorization") {
			continue // strip Beekeeper's own gateway token from upstream requests
		}
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		writeJSONRPCError(w, msg.ID, -32002, "upstream request failed (fail-closed)", nil)
		return
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxRequestBody))
	if err != nil {
		writeJSONRPCError(w, msg.ID, -32002, "upstream response read failed (fail-closed)", nil)
		return
	}

	// Inject _beekeeper_warning into the result field of the JSON-RPC response.
	injected := injectWarning(respBytes, decision.Reason)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(injected)
}

// injectWarning adds a _beekeeper_warning field to a JSON-RPC response's result
// object. If the response is not a parseable JSON object with a "result" key,
// the original bytes are returned unchanged (defense-in-depth: never corrupt).
func injectWarning(respBytes []byte, reason string) []byte {
	var respMap map[string]any
	if err := json.Unmarshal(respBytes, &respMap); err != nil {
		return respBytes // unparseable — return original
	}

	result, hasResult := respMap["result"]
	if !hasResult {
		return respBytes // no result field — return original
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		// result is not an object (e.g. it's a string or array) — wrap it.
		resultMap = map[string]any{"_beekeeper_warning": reason, "result": result}
		respMap["result"] = resultMap
	} else {
		resultMap["_beekeeper_warning"] = reason
	}

	out, err := json.Marshal(respMap)
	if err != nil {
		return respBytes // marshal failed — return original
	}
	return out
}

// verifyToken validates the Authorization: Bearer <token> header using
// constant-time comparison to prevent timing attacks (T-04-03-02).
func (h *gatewayHandler) verifyToken(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, bearerPrefix) {
		return false
	}
	candidate := auth[len(bearerPrefix):]
	// Constant-time comparison prevents timing attacks on the token.
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(h.token)) == 1
}

// writeAudit appends one NDJSON audit record for the gateway decision.
// Audit write failures are logged to stderr but NEVER fail the request
// (an audit failure must not unblock a tool call or corrupt the response).
func (h *gatewayHandler) writeAudit(tc policy.ToolCall, d policy.Decision) {
	if h.cfg.AuditPath == "" {
		return
	}
	aw, err := audit.NewWriter(h.cfg.AuditPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper gateway: audit writer unavailable: %v\n", err)
		return
	}
	defer aw.Close()

	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return
	}
	recordID := hex.EncodeToString(raw[:])
	rec := audit.FromDecision(tc, d, recordID, time.Now().UTC().Format(time.RFC3339), policy.AgentContext{})
	rec.Endpoint = "gateway"
	// CR-06: apply sensitive-field redaction before writing to disk, consistent
	// with how check/handler.go (writeAuditWithAC) handles redaction (T-04-05-02).
	patterns := audit.DefaultRedactPatterns()
	rec = audit.RedactRecord(rec, patterns)
	if err := aw.Write(rec); err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper gateway: audit write failed: %v\n", err)
	}
}

// extractToolCallFromMsg extracts a policy.ToolCall from a tools/call
// JSONRPCMessage. It is used for audit record construction.
func extractToolCallFromMsg(msg JSONRPCMessage, tc *policy.ToolCall) error {
	var params toolCallParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return err
	}
	tc.ToolName = params.Name
	tc.ToolInput = params.Arguments
	return nil
}

// writeJSONRPCError writes a JSON-RPC 2.0 error response to w.
// The HTTP status is always 200 per JSON-RPC 2.0 spec (errors are application-
// level, not transport-level). The id field echoes the request ID exactly
// (string, int/float64, or null) — never correlated by position (T-04-03-07).
func writeJSONRPCError(w http.ResponseWriter, id any, code int, msg string, data any) {
	errObj := map[string]any{
		"code":    code,
		"message": msg,
	}
	if data != nil {
		errObj["data"] = data
	}

	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   errObj,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // JSON-RPC errors always use HTTP 200
	_ = json.NewEncoder(w).Encode(resp)
}
