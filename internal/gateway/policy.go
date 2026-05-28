package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mzansi-agentive/beekeeper/internal/llamafirewall"
	"github.com/mzansi-agentive/beekeeper/internal/policy"
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
	// Default: "127.0.0.1" (never "0.0.0.0" without allow_remote_gateway).
	BindAddr string

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
}

// toolCallParams is the JSON-RPC 2.0 params shape for a tools/call request.
// The MCP July 2026 spec defines params as {"name":"<tool>","arguments":{...}}.
type toolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// applyPolicy extracts the ToolCall from a tools/call JSONRPCMessage, evaluates
// it against the policy engine, and returns the Decision. It is the single
// bridge between the gateway's JSON-RPC layer and the pure policy engine.
//
// The AgentContext passed to Evaluate comes from extractAgentContext(r) — the
// optional X-Beekeeper-* headers injected by MCP clients that support them.
//
// If the params JSON cannot be decoded (malformed tools/call), applyPolicy
// returns a block decision so the gateway fails closed (T-04-03-06).
func applyPolicy(msg JSONRPCMessage, idx policy.MultiCatalogLookup, ac policy.AgentContext) policy.Decision {
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

	return policy.Evaluate(tc, idx, policy.DefaultCorroborationThresholds(), ac)
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
