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
	"sync"
	"time"

	"github.com/home-beekeeper/beekeeper/internal/audit"
	"github.com/home-beekeeper/beekeeper/internal/nudge"
	"github.com/home-beekeeper/beekeeper/internal/pkgparse"
	"github.com/home-beekeeper/beekeeper/internal/policy"
)

const (
	bearerPrefix = "Bearer "
	// advisoryGlobalKey is the fallback key for the per-session advisory cap
	// when no agent-id header is present on the request (Open Q3 resolution).
	advisoryGlobalKey = "__global__"
)

// hopByHopHeaders are the HTTP/1.1 hop-by-hop headers (RFC 7230 §6.1) that are
// meaningful only for a single transport hop and must NOT be forwarded to the
// upstream by an intermediary. httputil.ReverseProxy strips these automatically;
// the manual-forward paths (forwardWithWarningInjection, forwardAllowWithScan)
// build their own *http.Request and so must strip them explicitly via
// sanitizeUpstreamHeaders. Keys are canonical (textproto.CanonicalMIMEHeaderKey)
// form so a case-insensitive map lookup matches r.Header's canonicalized keys.
var hopByHopHeaders = map[string]struct{}{
	"Connection":          {},
	"Proxy-Connection":    {}, // non-standard but seen in the wild
	"Keep-Alive":          {},
	"Proxy-Authenticate":  {},
	"Proxy-Authorization": {},
	"Te":                  {},
	"Trailer":             {},
	"Transfer-Encoding":   {},
	"Upgrade":             {},
}

// sanitizeUpstreamHeaders copies client request headers from src into a fresh
// http.Header suitable for the upstream MCP request on the manual-forward paths.
// It drops, in addition to the per-call Authorization strip the callers already
// perform:
//
//   - the Beekeeper gateway Authorization token (CR-02 — never leak it upstream);
//   - all hop-by-hop headers (RFC 7230 §6.1) so a client cannot smuggle a
//     Connection/Transfer-Encoding/Upgrade directive through the gateway;
//   - any header named in a Connection header's token list (RFC 7230 §6.1 —
//     these are connection-scoped and must not be forwarded);
//   - all internal X-Beekeeper-* headers — these are gateway-internal agent
//     context (X-Beekeeper-Agent-Id, -Parent-Agent-Id, -Agent-Depth) consumed by
//     extractAgentContext and must never reach the upstream MCP server, nor may a
//     client forge them onto the upstream.
//
// This is the single shared sanitizer so the warn-path and allow-path manual
// forwards cannot drift apart (only one place to audit). The transparent
// ReverseProxy path is unaffected — it strips hop-by-hop headers itself and
// strips Authorization in the Rewrite hook.
func sanitizeUpstreamHeaders(src http.Header) http.Header {
	// Collect Connection-listed tokens so we drop them too (RFC 7230 §6.1).
	connectionTokens := map[string]struct{}{}
	for _, val := range src.Values("Connection") {
		for _, tok := range strings.Split(val, ",") {
			tok = strings.TrimSpace(tok)
			if tok != "" {
				connectionTokens[http.CanonicalHeaderKey(tok)] = struct{}{}
			}
		}
	}

	out := make(http.Header, len(src))
	for k, vv := range src {
		canon := http.CanonicalHeaderKey(k)
		if strings.EqualFold(canon, "Authorization") {
			continue // strip Beekeeper's own gateway token from upstream requests
		}
		if _, hop := hopByHopHeaders[canon]; hop {
			continue
		}
		if _, listed := connectionTokens[canon]; listed {
			continue
		}
		if strings.HasPrefix(canon, "X-Beekeeper-") {
			continue // internal agent-context headers — never forward upstream
		}
		for _, v := range vv {
			out.Add(canon, v)
		}
	}
	return out
}

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
	// scanner is the optional LlamaFirewall scanner for post-response scanning.
	// Nil when LlamaFirewall is disabled (default). Populated from cfg.Scanner.
	scanner GatewayScanner

	// nudgeCache wraps nudge.DetectStateFn with a 60s TTL (Flag 2 Position B):
	// the ONLY place the cache is constructed — gateway-only, long-lived process.
	// Constructed once in newGatewayHandler; never nil after construction.
	// Comment: "Flag 2 Position B: the 60s cache lives ONLY in the long-lived
	// gateway and wraps nudge.DetectStateFn (the exported seam)."
	nudgeCache *nudge.Cache

	// advSeenMu guards advSeen for concurrent requests.
	advSeenMu sync.Mutex
	// advSeen is the per-session advisory-seen set for the at-most-one-advisory
	// cap (NUDGE-03). Keyed by agent-id when present on the request; else by the
	// process-global sentinel key advisoryGlobalKey. A session key present in
	// this map means an Advise has already been delivered; duplicate advisories
	// are suppressed (still audited) — resolves Open Q3.
	advSeen map[string]bool
}

// newGatewayHandler constructs a gatewayHandler for the given upstream URL.
// Uses Rewrite (deprecated httputil proxy API not used; T-04-03-12 mitigation)
// to set the upstream target.
//
// WARNING 3 fix: when cfg.Nudge is a zero-value Config (empty Mode), default it
// to nudge.DefaultConfig() so the gateway always evaluates against secure version
// floors even before the daemon literal sets this field (T-08-25b). The daemon
// literal population is performed in cmd/beekeeper/main.go newGatewayCmd (Plan 08).
func newGatewayHandler(cfg Config, token string, idx policy.MultiCatalogLookup) *gatewayHandler {
	// Default a zero-value cfg.Nudge to the secure defaults (WARNING 3 fix).
	// An empty Mode is the reliable zero-value signal (DefaultConfig has Mode:"soft").
	if cfg.Nudge.Mode == "" {
		cfg.Nudge = nudge.DefaultConfig()
	}

	upstream, _ := url.Parse(cfg.UpstreamURL)

	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(upstream)
			pr.SetXForwarded()
			// Strip the Beekeeper gateway token — the upstream MCP server must
			// never receive it (CR-02: applies to all paths including allow/passthrough).
			pr.Out.Header.Del("Authorization")
		},
	}

	// Construct the 60s nudge.Cache ONCE, wrapping the EXPORTED nudge.DetectStateFn
	// seam (Flag 2 Position B — cache is gateway-only; check hook detects fresh).
	nc := nudge.NewCache(nudge.DetectStateFn, 60*time.Second)

	return &gatewayHandler{
		token:        token,
		reverseProxy: rp,
		cfg:          cfg,
		idx:          idx,
		scanner:      cfg.Scanner,
		nudgeCache:   nc,
		advSeen:      make(map[string]bool),
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

	// policyResult carries either a decision or a recovered panic value.
	type policyResultFull struct {
		d        policy.Decision
		panicked bool
		panicVal any
	}
	chFull := make(chan policyResultFull, 1)
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				chFull <- policyResultFull{panicked: true, panicVal: rec}
			}
		}()
		chFull <- policyResultFull{d: applyPolicy(msg, h.idx, h.cfg, ac)}
	}()

	var decision policy.Decision
	select {
	case res := <-chFull:
		if res.panicked {
			fmt.Fprintf(os.Stderr, "beekeeper gateway: recovered panic in policy goroutine: %v\n", res.panicVal)
			if h.cfg.FailOpen {
				writeJSONRPCError(w, msg.ID, -32002, "internal error (fail-open: reduced security)", nil)
			} else {
				writeJSONRPCError(w, msg.ID, -32002, "internal error (fail-closed)", nil)
			}
			return
		}
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
	// Also extract the tool call here for use in nudge merge and scanner call sites below.
	tc := policy.ToolCall{}
	var toolName string
	if err := extractToolCallFromMsg(msg, &tc); err == nil {
		toolName = tc.ToolName

		// NUDGE-03/04/08: cache-backed nudge merge at the applyPolicy CALL SITE
		// (proxy.go — WARNING 2 closed). applyPolicy is a FREE function with no
		// *gatewayHandler; the nudge.Cache + advisory-seen set on h are reachable
		// only here. CR-02 ordering: nudge merge runs AFTER applyPolicy returns its
		// overlay-applied decision — an allow overlay cannot downgrade a nudge Block
		// (T-08-17). mergeDecisions is most-restrictive-wins.
		//
		// PMState is resolved via h.nudgeCache.State (the 60s TTL cache wrapping
		// nudge.DetectStateFn — Flag 2 Position B: the cache is gateway-only and
		// produces hits for the long-lived process; the check hook detects fresh).
		if tc.ToolName == "Bash" {
			if bashCmd, ok := tc.ToolInput["command"].(string); ok && bashCmd != "" {
				if parsed, parseOK := pkgparse.Parse(bashCmd); parseOK && parsed.IsInstall {
					pmState := h.nudgeCache.State(r.Context(), h.cfg.Nudge)
					if nudgeDec, nudgeRec, nudgeOK := nudgeDecisionFor(parsed, pmState, h.cfg.Nudge); nudgeOK {
						// At-most-one-advisory-per-session cap (NUDGE-03).
						// Keyed by agent-id when present, else process-global sentinel.
						agentKey := ac.AgentID
						if agentKey == "" {
							agentKey = advisoryGlobalKey
						}
						suppressAdvisory := false
						if nudgeDec.Level == "warn" {
							h.advSeenMu.Lock()
							if h.advSeen[agentKey] {
								suppressAdvisory = true
							} else {
								h.advSeen[agentKey] = true
							}
							h.advSeenMu.Unlock()
						}
						// Merge the nudge decision into the policy decision regardless
						// of the cap — the cap only suppresses the advisory message, not
						// the audit record or the block decision.
						if !suppressAdvisory {
							decision = mergeGatewayDecisions(decision, nudgeDec)
						} else if nudgeDec.Level == "block" {
							// A Block is never suppressed — only advisory (warn) messages are capped.
							decision = mergeGatewayDecisions(decision, nudgeDec)
						}
						// Write nudge audit record best-effort (T-08-19).
						h.writeNudgeAudit(nudgeRec)
					}
				}
			}
		}

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
		// forwardWithWarningInjection also calls ScanProxiedResponse when scanner present.
		h.forwardWithWarningInjection(w, r, msg, bodyBytes, decision, toolName)
		return

	default: // "allow"
		// When a LLMF scanner is configured, use the direct forwarding path so we
		// can intercept the response and scan for prompt injection. Without a scanner
		// the transparent ReverseProxy path is used (no response capture overhead).
		if h.scanner != nil {
			h.forwardAllowWithScan(w, r, msg, bodyBytes, toolName)
			return
		}
		// Restore body for transparent forward.
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		h.reverseProxy.ServeHTTP(w, r)
	}
}

// forwardWithWarningInjection forwards a warn-level tool call to the upstream
// MCP server and injects a _beekeeper_warning field into the JSON-RPC result.
// The warning is a vendor extension field (underscore prefix per MCP convention;
// Assumption A3 in RESEARCH.md: MCP clients ignore unknown fields).
func (h *gatewayHandler) forwardWithWarningInjection(w http.ResponseWriter, r *http.Request, msg JSONRPCMessage, bodyBytes []byte, decision policy.Decision, toolName string) {
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

	// Copy original request headers through the shared sanitizer: strips the
	// Beekeeper gateway Authorization token (CR-02 — internal secret), all
	// hop-by-hop headers (RFC 7230 §6.1), and internal X-Beekeeper-* headers.
	// One shared helper so the warn/allow manual paths cannot drift.
	req.Header = sanitizeUpstreamHeaders(r.Header)

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

	// LlamaFirewall post-response scan (INT-BLOCK-1 / LLMF-02): scan the upstream
	// response body for prompt injection before forwarding. On injection detection
	// the body is replaced with a structured warning payload. Fail-closed on
	// sidecar unavailability when FailOpen is false.
	if h.scanner != nil {
		scanned, injectionDetected := ScanProxiedResponse(r.Context(), toolName, respBytes, h.scanner)
		if injectionDetected {
			// Injection detected: replace body with warning payload.
			respBytes = scanned
		} else if scanned != nil {
			respBytes = scanned
		}
	}

	// Inject _beekeeper_warning into the result field of the JSON-RPC response.
	injected := injectWarning(respBytes, decision.Reason)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(injected)
}

// forwardAllowWithScan is the allow-path equivalent of forwardWithWarningInjection
// used when a LLMF scanner is configured. It makes a direct HTTP request to the
// upstream, scans the response body for prompt injection, and writes the
// (possibly replaced) response to the client.
func (h *gatewayHandler) forwardAllowWithScan(w http.ResponseWriter, r *http.Request, msg JSONRPCMessage, bodyBytes []byte, toolName string) {
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
	// Shared sanitizer: strip gateway Authorization, hop-by-hop, and X-Beekeeper-*
	// headers (same path as the warn forward — single helper, no drift).
	req.Header = sanitizeUpstreamHeaders(r.Header)

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

	// LLMF scan for prompt injection on the allow path.
	if h.scanner != nil {
		scanned, _ := ScanProxiedResponse(r.Context(), toolName, respBytes, h.scanner)
		if scanned != nil {
			respBytes = scanned
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBytes)
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

// mergeGatewayDecisions returns the most restrictive of base and overlay.
// Rank: block(2) > warn(1) > allow(0). Same logic as mergeDecisions in
// internal/check/paths.go — duplicated here to keep internal/check untouched
// and avoid a cross-package import of the check helper.
func mergeGatewayDecisions(base, overlay policy.Decision) policy.Decision {
	rank := map[string]int{"allow": 0, "warn": 1, "block": 2}
	if rank[overlay.Level] > rank[base.Level] {
		return overlay
	}
	return base
}

// writeNudgeAudit appends a nudge audit record best-effort.
// A write failure is logged to stderr but NEVER fails the request.
func (h *gatewayHandler) writeNudgeAudit(rec audit.AuditRecord) {
	if h.cfg.AuditPath == "" {
		return
	}
	w, err := audit.NewWriter(h.cfg.AuditPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper gateway: nudge audit write failed: %v\n", err)
		return
	}
	defer w.Close()
	// WR-01: redact sensitive command fields before writing, consistent with the
	// main gateway audit path (writeAudit). OriginalCommand/RewrittenCommand carry
	// the verbatim agent-supplied Bash command, which may embed a token/secret.
	rec = audit.RedactRecord(rec, audit.DefaultRedactPatterns())
	if err := w.Write(rec); err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper gateway: nudge audit write error: %v\n", err)
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
