package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bantuson/beekeeper/internal/audit"
	"github.com/bantuson/beekeeper/internal/llamafirewall"
	"github.com/bantuson/beekeeper/internal/nudge"
	"github.com/bantuson/beekeeper/internal/pkgparse"
	"github.com/bantuson/beekeeper/internal/policy"
	"github.com/bantuson/beekeeper/internal/policyloader"
)

// Config holds the runtime configuration for the MCP gateway daemon.
// It is constructed by cmd/beekeeper/main.go from CLI flags and the layered
// config and passed to Start(). No token is stored here — the token is runtime
// state written to state.json (T-04-03-10: never in CLI flags or config.json).
type Config struct {
	// UpstreamURL is the MCP server URL to proxy to (required).
	// e.g. "http://localhost:3000"
	UpstreamURL string

	// BindAddr is the address to bind the gateway HTTP server to.
	// Default: "127.0.0.1" (loopback only). Binding a non-loopback address
	// (e.g. "0.0.0.0") requires AllowRemote: true (TM-A-01 gate). The gateway
	// is plain HTTP — a non-loopback bind exposes the bearer token in cleartext.
	BindAddr string

	// AllowRemote, when true, permits binding to a non-loopback address.
	// The operator must set this explicitly (--allow-remote CLI flag). When false
	// (the default), Start refuses to bind anything other than loopback (TM-A-01).
	// A prominent plaintext-HTTP warning is printed to stderr when this is set.
	AllowRemote bool

	// Port is the TCP port to bind on. Default: 7837.
	Port int

	// StateFile is the path to ~/.beekeeper/state.json.
	StateFile string

	// IndexPath is the path to the Bumblebee mmap catalog index.
	IndexPath string

	// CacheDir is the directory for OSV and Socket response caches.
	CacheDir string

	// AuditPath is the path to the NDJSON audit log file.
	AuditPath string

	// SocketToken is the Socket PURL API authentication token.
	// Empty string disables Socket (not an error).
	SocketToken string

	// FailOpen, when true, allows tool calls on policy engine failure.
	// Default (false) is fail-closed — the secure setting.
	FailOpen bool

	// Scanner is an optional LlamaFirewall scanner for post-response scanning
	// (INT-BLOCK-1 / LLMF-02). Nil when LlamaFirewall is disabled (default).
	// When non-nil, the gateway proxy calls ScanProxiedResponse on each upstream
	// response for warn and allow decisions.
	Scanner GatewayScanner

	// Nudge holds the resolved nudge.Config for the gateway (WARNING 3 fix).
	// newGatewayHandler defaults a zero-value Nudge to nudge.DefaultConfig() so
	// the gateway always evaluates against secure version floors, even before the
	// daemon literal sets this field (T-08-25b: fail toward secure defaults).
	// The daemon population (gatewayCfg.Nudge = nudge.ConfigFrom(...)) is performed
	// in cmd/beekeeper/main.go newGatewayCmd — Plan 08 owns that file (zero Wave-4
	// overlap between plans 06 and 08).
	Nudge nudge.Config
}

// toolCallParams is the JSON-RPC 2.0 params shape for a tools/call request.
// The MCP July 2026 spec defines params as {"name":"<tool>","arguments":{...}}.
type toolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// applyPolicy extracts the ToolCall from a tools/call JSONRPCMessage, evaluates
// it against the policy engine with policy-file-derived thresholds, applies the
// policy overlay, and returns the Decision. It is the single bridge between the
// gateway's JSON-RPC layer and the pure policy engine.
//
// The AgentContext passed to Evaluate comes from extractAgentContext(r) — the
// optional X-Beekeeper-* headers injected by MCP clients that support them.
//
// If the params JSON cannot be decoded (malformed tools/call), applyPolicy
// returns a block decision so the gateway fails closed (T-04-03-06).
//
// Policy overlay (INT-WARN-1 + INT-BLOCK-3): policy files are loaded from the
// standard ~/.beekeeper/policies dir (derived from cfg.CacheDir) and applied in
// the same way as handler.go. Missing policies dir = no-op; malformed file = skip.
func applyPolicy(msg JSONRPCMessage, idx policy.MultiCatalogLookup, cfg Config, ac policy.AgentContext) policy.Decision {
	var params toolCallParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		// Malformed tools/call params → fail closed (block).
		return policy.Decision{
			Allow:  false,
			Level:  "block",
			Reason: fmt.Sprintf("malformed tools/call params (fail-closed): %v", err),
			RuleIDs: []string{"INTG-04"},
		}
	}

	tc := policy.ToolCall{
		ToolName:  params.Name,
		ToolInput: params.Arguments,
	}

	// Load policy files to derive corroboration thresholds (INT-BLOCK-3 / PLCY-07):
	// mirrors handler.go runCheck so gateway enforcement matches hook enforcement.
	var policyFiles []policyloader.PolicyFile
	if cfg.CacheDir != "" {
		policiesDir := filepath.Join(filepath.Dir(cfg.CacheDir), "policies")
		var loadErr error
		policyFiles, loadErr = policyloader.LoadPolicyDir(policiesDir)
		if loadErr != nil {
			// Directory read error: fail closed per gateway FailOpen flag.
			if !cfg.FailOpen {
				return policy.Decision{
					Allow:  false,
					Level:  "block",
					Reason: fmt.Sprintf("policies directory unreadable (fail-closed): %v", loadErr),
				}
			}
			// fail-open: log and continue with defaults.
			policyFiles = nil
		}
	}
	thresholds := policyloader.ThresholdsFromPolicyFiles(policyFiles)
	// CORR-02: thread catalog sanity state into thresholds.
	thresholds.CatalogHealthy = resolveCatalogHealthy(cfg.CacheDir)

	decision := policy.Evaluate(tc, idx, thresholds, ac)

	// Apply package_allowlist / sensitive_path overlay (INT-WARN-1).
	if len(policyFiles) > 0 {
		decision = policyloader.ApplyPolicyOverlay(policyFiles, tc, decision)
	}

	return decision
}

// nudgeDecisionFor evaluates a nudge decision for a parsed install command given
// an already-resolved PMState and nudge Config. It returns (policy.Decision,
// audit.AuditRecord, true) on a nudge-applicable install command, or
// (_, _, false) when the command is not install-class.
//
// This helper is pure w.r.t. caching: it takes the already-resolved PMState
// (caller is responsible for cache lookup or fresh detection). It is separate
// from the cache-backed call site in proxy.go so applyPolicy stays a FREE
// function (WARNING 2 closed).
//
// Mirrors the evaluateNudge/nudgeLevelToDecision mapping in internal/check.
func nudgeDecisionFor(parsed pkgparse.ParsedCommand, state nudge.PMState, nc nudge.Config) (policy.Decision, audit.AuditRecord, bool) {
	if !parsed.IsInstall {
		return policy.Decision{}, audit.AuditRecord{}, false
	}
	d := nudge.Evaluate(parsed, state, nc)
	allow := d.Level != "block"
	policyDec := policy.Decision{
		Allow:  allow,
		Level:  d.Level,
		Reason: fmt.Sprintf("nudge(%s): %s", nudge.ActionString(d.Action), d.Reason),
		RuleIDs: []string{"NUDGE-03"},
	}
	var raw [16]byte
	_, _ = rand.Read(raw[:])
	recordID := hex.EncodeToString(raw[:])
	rec := audit.AuditRecord{
		RecordType:      "nudge",
		RecordID:        recordID,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
		ScannerName:     "beekeeper",
		Decision:        d.Level,
		Reason:          d.Reason,
		Endpoint:        "gateway",
		OriginalCommand: d.Original,
		ReasonCode:      d.Reason,
		NudgeAction:     nudge.ActionString(d.Action),
	}
	if d.Rewritten != "" {
		rec.RewrittenCommand = d.Rewritten
	}
	if ps, ok := d.AuditFields["pm_state"].(string); ok {
		rec.PMState = ps
	}
	return policyDec, rec, true
}

// GatewayScanner is satisfied by *llamafirewall.Supervisor.
type GatewayScanner interface {
	Scan(ctx context.Context, req llamafirewall.ScanRequest) (llamafirewall.ScanResponse, error)
	IsDegraded() bool
}

// ScanProxiedResponse runs PromptGuard 2 on a proxied MCP tool response body.
// Returns the (possibly replaced) body and whether injection was detected.
// If scanner is nil, the tool is not prompt-scan-eligible, or the scanner is
// degraded, returns body unchanged.
func ScanProxiedResponse(ctx context.Context, toolName string, body []byte, scanner GatewayScanner) ([]byte, bool) {
	if scanner == nil || scanner.IsDegraded() || !llamafirewall.ShouldScanPrompt(toolName) {
		return body, false
	}
	resp, err := scanner.Scan(ctx, llamafirewall.ScanRequest{
		Kind:      llamafirewall.ScanPrompt,
		Content:   string(body),
		Context:   toolName,
		RequestID: fmt.Sprintf("gw-%d", time.Now().UnixNano()),
	})
	if err != nil || resp.Result == llamafirewall.ResultClean {
		return body, false
	}
	// Replace body with structured warning payload.
	warning := llamafirewall.BuildWarningPayload(resp)
	return warning, true
}

// extractAgentContext reads optional multi-agent context from HTTP request
// headers. MCP clients that support INTG-07 inject these headers so the gateway
// can populate lineage for downstream policy decisions and audit records.
//
// Headers:
//   - X-Beekeeper-Agent-Id: current agent session ID
//   - X-Beekeeper-Parent-Agent-Id: parent agent session ID
//   - X-Beekeeper-Agent-Depth: nesting depth integer (negative → normalized to 0)
//
// All headers are optional. Missing or invalid values produce a zero-value
// AgentContext (root context with no lineage). Negative depth is normalized to 0
// (T-04-03: no elevation-of-privilege via BEEKEEPER_AGENT_DEPTH=-1).
func extractAgentContext(r *http.Request) policy.AgentContext {
	depth := 0
	if d := r.Header.Get("X-Beekeeper-Agent-Depth"); d != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(d)); err == nil && parsed > 0 {
			depth = parsed
		}
		// Negative or invalid values remain 0.
	}

	return policy.AgentContext{
		AgentID:       strings.TrimSpace(r.Header.Get("X-Beekeeper-Agent-Id")),
		ParentAgentID: strings.TrimSpace(r.Header.Get("X-Beekeeper-Parent-Agent-Id")),
		Depth:         depth,
	}
}
