// Package check implements the `beekeeper check` hook handler: the fail-closed
// entry point that reads an untrusted tool call from stdin, evaluates it against
// the mmap catalog index via the pure policy engine, writes an NDJSON audit
// record, and returns an allow (exit 0) or block (non-zero) decision.
//
// The cardinal rule of this package is FAIL CLOSED: a panic, timeout, oversized
// stdin, malformed JSON, or missing/corrupt catalog index must all result in a
// BLOCK, never a silent allow. fail_open/fail_warn is an explicit, documented
// opt-out (see internal/config) that reduces security.
package check

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/mzansi-agentive/beekeeper/internal/audit"
	"github.com/mzansi-agentive/beekeeper/internal/catalog"
	"github.com/mzansi-agentive/beekeeper/internal/config"
	"github.com/mzansi-agentive/beekeeper/internal/llamafirewall"
	"github.com/mzansi-agentive/beekeeper/internal/policy"
)

const (
	// maxStdin is the 1MB hard cap on tool-call JSON read from stdin (HOOK-04).
	maxStdin = 1 << 20
	// execTimeout is the 5s execution deadline for a single check (HOOK-04).
	execTimeout = 5 * time.Second
	// memLimit is the 256MB soft memory cap for the process (HOOK-04).
	memLimit = 256 * 1024 * 1024

	exitAllow = 0
	exitBlock = 1
)

// GlobalHookTracker accumulates per-invocation hook handler latency samples for
// beekeeper diag. Initialized once at package level; Record() is called at the
// end of runCheck. Because beekeeper check is a one-shot process (one invocation
// per tool call), samples are also persisted to a ring file via appendHookLatency
// so diag can compute p95/p99 across process restarts.
var GlobalHookTracker = &llamafirewall.LatencyTracker{}

// catalogIndex combines io.Closer with policy.MultiCatalogLookup so that the
// hook handler can both evaluate policy decisions and release the mmap resource.
// Plan 08 wires the full multi-source MultiIndex here.
type catalogIndex interface {
	io.Closer
	policy.MultiCatalogLookup
}

// catalogOpener opens a Bumblebee mmap index by path and returns the raw
// *catalog.Index. Tests substitute a function that returns an error or a fake to
// exercise fail-closed paths without a real index file.
type catalogOpener func(path string) (*catalog.Index, error)

func defaultOpener(path string) (*catalog.Index, error) {
	return catalog.OpenIndex(path)
}

// Result is the outcome of a single check: the policy Decision and the process
// exit code the caller (cmd/beekeeper) must use. ExitCode is 0 only when the
// tool call is permitted.
type Result struct {
	Decision policy.Decision
	ExitCode int
}

// RunCheck reads a tool call from stdin, evaluates it under hard caps, writes
// an audit record, prints the Decision as JSON to stdout, and returns a Result.
// It NEVER returns a silent allow on failure: every failure path produces a
// block unless cfg explicitly opts into fail-open/fail-warn.
//
// cacheDir is the Beekeeper catalogs directory (e.g. ~/.beekeeper/catalogs).
// It is used to locate OSV and Socket caches for the multi-source aggregator.
func RunCheck(ctx context.Context, stdin io.Reader, cfg config.Config, indexPath, auditPath, cacheDir string) Result {
	return runCheck(ctx, stdin, cfg, indexPath, auditPath, cacheDir, defaultOpener)
}

// hookInput extends ToolCall with the Claude Code hook stdin fields that are
// not part of the pure policy.ToolCall struct. The agent_id field is present
// in Claude Code PreToolUse stdin and used to populate AgentContext.AgentID
// when the BEEKEEPER_AGENT_ID env var is absent (INTG-07).
type hookInput struct {
	policy.ToolCall
	AgentID string `json:"agent_id"` // Claude Code hook stdin only; absent in Cursor/Codex
}

// runCheck is the testable core; opener is injected so tests can force a panic
// or error from index opening.
func runCheck(ctx context.Context, stdin io.Reader, cfg config.Config, indexPath, auditPath, cacheDir string, open catalogOpener) (result Result) {
	// Record the start time for hook latency tracking (CODE-06). The elapsed
	// duration is appended to the persisted ring file at the end of runCheck
	// so beekeeper diag can report accumulated p95/p99 across one-shot invocations.
	start := time.Now()

	// HOOK-04: 256MB soft memory cap; combined with 1MB stdin LimitReader this
	// bounds tool-call evaluation memory.
	debug.SetMemoryLimit(memLimit)

	// toolCall is captured so the top-level recover can still write a
	// best-effort audit record on panic. It is the zero value until decode
	// succeeds, which finalize records as an empty agent/tool.
	var toolCall policy.ToolCall

	// Top-level fail-closed guard: any panic becomes a block decision, honoring
	// the configured fail mode (default closed). This is the last line of
	// defense against a silent allow.
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "beekeeper check: recovered panic: %v\n", r)
			d := failDecision(cfg, "internal error (fail-closed)")
			result = finalizeWithAC(d, cfg, toolCall, auditPath, policy.AgentContext{})
		}
	}()

	// Best-effort hook latency recording (CODE-06, T-09-14): record elapsed time
	// in the in-memory tracker and append to the persisted ring file. Errors are
	// silently ignored — a ring-write failure MUST NOT alter result or fail-closed
	// behavior. The defer runs after the recover guard so panics are captured first.
	defer func() {
		elapsedMS := time.Since(start).Milliseconds()
		GlobalHookTracker.Record(elapsedMS)
		if cacheDir != "" {
			ringPath := filepath.Join(filepath.Dir(cacheDir), hookLatencyFile)
			appendHookLatency(ringPath, elapsedMS)
		}
	}()

	// HOOK-04: enforce a 5s execution deadline.
	ctx, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()

	// HOOK-04: cap stdin at 1MB. We read one byte past the cap to detect
	// truncation: if the decoder consumed up to the limit AND another byte is
	// available, the input was oversized and we fail closed rather than
	// evaluate a truncated tool call.
	limited := &io.LimitedReader{R: stdin, N: maxStdin + 1}

	// Decode into hookInput to also capture the Claude Code stdin agent_id field.
	var hi hookInput
	dec := json.NewDecoder(limited)
	if err := dec.Decode(&hi); err != nil {
		// Distinguish oversized input from genuinely malformed JSON: if the
		// limited reader is exhausted we very likely truncated a large payload.
		if limited.N <= 0 {
			return finalizeWithAC(failDecision(cfg, "stdin exceeds 1MB cap (fail-closed)"), cfg, toolCall, auditPath, policy.AgentContext{})
		}
		return finalizeWithAC(failDecision(cfg, "invalid tool call JSON (fail-closed)"), cfg, toolCall, auditPath, policy.AgentContext{})
	}
	toolCall = hi.ToolCall
	stdinAgentID := hi.AgentID

	// Oversized detection for valid-but-too-large input: a successful decode
	// that consumed everything up to (and including) the extra cap byte means
	// the payload was at least maxStdin+1 bytes.
	if limited.N <= 0 {
		return finalizeWithAC(failDecision(cfg, "stdin exceeds 1MB cap (fail-closed)"), cfg, toolCall, auditPath, policy.AgentContext{})
	}

	// Early timeout check after the (potentially slow) stdin read.
	if ctx.Err() != nil {
		return finalizeWithAC(failDecision(cfg, "execution timeout (fail-closed)"), cfg, toolCall, auditPath, policy.AgentContext{})
	}

	// HOOK-02: load the catalog via the mmap index, never a cold JSON parse.
	bbIdx, err := open(indexPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper check: catalog index unavailable: %v\n", err)
		return finalizeWithAC(failDecision(cfg, "catalog index unavailable (fail-closed)"), cfg, toolCall, auditPath, policy.AgentContext{})
	}
	defer bbIdx.Close()

	// Give network adapters a dedicated sub-context capped at 3s. This prevents
	// a slow stdin decode (up to ~4.9s in the worst case) from leaving the OSV
	// and Socket HTTP calls with only milliseconds before the outer 5s deadline
	// expires, which would force both sources to degrade (WR-05).
	netCtx, netCancel := context.WithTimeout(ctx, 3*time.Second)
	defer netCancel()

	// Build the OSV adapter. On OSV error, LookupAll returns nil — the source
	// degrades to no-match (T-02-08-01).
	httpClient := &http.Client{Timeout: 4 * time.Second}
	var osvAdapter policy.MultiCatalogLookup = &catalog.OSVAdapter{
		Client:   httpClient,
		CacheDir: cacheDir,
		Ctx:      netCtx,
	}

	// Build the Socket adapter. Empty token → Socket disabled (not an error).
	// Degraded Socket degrades to warn-only, never blocks (T-02-08-01).
	var socketAdapter policy.MultiCatalogLookup
	if token := cfg.SocketAPIToken(); token != "" {
		socketAdapter = catalog.SocketAdapter{
			Client:   httpClient,
			CacheDir: cacheDir,
			Token:    token,
			Ctx:      netCtx,
		}
	}

	// Aggregate all three sources into a MultiIndex. Nil adapters are skipped.
	multiIdx := catalog.NewMultiIndex(bbIdx, osvAdapter, socketAdapter)

	// Re-check the deadline before the pure evaluation.
	if ctx.Err() != nil {
		return finalizeWithAC(failDecision(cfg, "execution timeout (fail-closed)"), cfg, toolCall, auditPath, policy.AgentContext{})
	}

	// Build agent context from env vars + stdin agent_id (INTG-07).
	ac := readAgentContext(stdinAgentID)

	// Pure, synchronous policy evaluation (no I/O, no goroutines).
	// multiIdx implements policy.MultiCatalogLookup aggregating Bumblebee+OSV+Socket.
	decision := policy.Evaluate(toolCall, multiIdx, policy.DefaultCorroborationThresholds(), ac)

	// Final deadline check: if we blew the budget during evaluation, fail closed
	// rather than emit a possibly-stale allow.
	if ctx.Err() != nil {
		return finalizeWithAC(failDecision(cfg, "execution timeout (fail-closed)"), cfg, toolCall, auditPath, policy.AgentContext{})
	}

	// Successful evaluation results are NOT subject to fail-mode overrides —
	// fail modes only govern the failure paths above.
	return finalizeWithAC(decision, cfg, toolCall, auditPath, ac)
}

// readAgentContext builds a policy.AgentContext from environment variables
// and the agent_id field from Claude Code hook stdin. The env vars are the
// authoritative source (BEEKEEPER_AGENT_ID overrides the stdin agent_id);
// stdin agent_id is the fallback for Claude Code hooks where the env var may
// not be set by the orchestration layer.
//
// Env vars (INTG-07):
//   - BEEKEEPER_AGENT_ID: current agent session ID
//   - BEEKEEPER_PARENT_AGENT_ID: parent agent session ID
//   - BEEKEEPER_AGENT_DEPTH: nesting depth integer (negative → normalized to 0)
//   - BEEKEEPER_AGENT_LINEAGE: comma-separated parent IDs from root to parent
func readAgentContext(stdinAgentID string) policy.AgentContext {
	depth := 0
	if d := os.Getenv("BEEKEEPER_AGENT_DEPTH"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
			depth = parsed
		}
		// Negative or invalid values remain 0 (normalize to root depth).
	}

	var lineage []string
	if l := os.Getenv("BEEKEEPER_AGENT_LINEAGE"); l != "" {
		// WR-08: trim whitespace from each element after split so that
		// "root, mid, child" (with spaces) produces ["root","mid","child"]
		// rather than ["root"," mid"," child"] which would not match agent IDs.
		parts := strings.Split(l, ",")
		for _, entry := range parts {
			if trimmed := strings.TrimSpace(entry); trimmed != "" {
				lineage = append(lineage, trimmed)
			}
		}
	}

	// Env var takes precedence; fall back to stdin agent_id from Claude Code hook.
	agentID := os.Getenv("BEEKEEPER_AGENT_ID")
	if agentID == "" {
		agentID = stdinAgentID
	}

	return policy.AgentContext{
		AgentID:       agentID,
		ParentAgentID: os.Getenv("BEEKEEPER_PARENT_AGENT_ID"),
		Depth:         depth,
		Lineage:       lineage,
	}
}

// failDecision builds the decision for a failure path, honoring the configured
// fail mode. The default (fail-closed) blocks; fail-open/fail-warn opt out of
// blocking and are documented as reducing security.
func failDecision(cfg config.Config, reason string) policy.Decision {
	if cfg.FailClosed() {
		return policy.Decision{
			Allow:  false,
			Level:  "block",
			Reason: reason,
		}
	}
	// Fail-open / fail-warn: allow on failure. Surface the original reason but
	// reflect the reduced-security disposition in the level.
	level := "allow"
	if cfg.FailMode == config.FailModeWarn {
		level = "warn"
	}
	return policy.Decision{
		Allow:  true,
		Level:  level,
		Reason: reason + " [fail_open: reduced security]",
	}
}

// finalize maps a decision to an exit code, writes the audit record, and prints
// the decision JSON to stdout. It is the single chokepoint every code path runs
// through so the audit-and-emit contract holds uniformly. Every path audits:
// for pre-decode failures tc is the zero ToolCall, yielding a best-effort record
// with empty agent/tool but a real decision and reason.
//
// Deprecated: prefer finalizeWithAC which carries AgentContext for lineage tracking.
func finalize(d policy.Decision, cfg config.Config, tc policy.ToolCall, auditPath string) Result {
	return finalizeWithAC(d, cfg, tc, auditPath, policy.AgentContext{})
}

// finalizeWithAC maps a decision to an exit code, writes the audit record with
// agent lineage fields, and prints the decision JSON to stdout. This is the
// single chokepoint that all code paths (including the panic recover) run through.
func finalizeWithAC(d policy.Decision, cfg config.Config, tc policy.ToolCall, auditPath string, ac policy.AgentContext) Result {
	writeAuditWithAC(tc, d, auditPath, ac)

	// Emit the structured decision to stdout.
	if data, err := json.Marshal(d); err == nil {
		fmt.Fprintln(os.Stdout, string(data))
	}

	return Result{Decision: d, ExitCode: exitCodeFor(d)}
}

// exitCodeFor maps a decision to a process exit code. In Phase 1 a "warn"
// decision keeps Allow=true and exits 0 (single-source warn does not block;
// corroboration-based blocking is Phase 2, PLCY-01). Only Allow=false blocks.
func exitCodeFor(d policy.Decision) int {
	if d.Allow {
		return exitAllow
	}
	return exitBlock
}

// writeAudit appends one NDJSON record for the decision with a zero AgentContext.
// Retained for compatibility; prefer writeAuditWithAC.
func writeAudit(tc policy.ToolCall, d policy.Decision, auditPath string) {
	writeAuditWithAC(tc, d, auditPath, policy.AgentContext{})
}

// writeAuditWithAC appends one NDJSON record for the decision with the given
// AgentContext lineage. An audit-write failure is logged to stderr but NEVER
// downgrades the decision — a block stays a block even if it could not be recorded.
//
// Redaction (T-04-05-02): default patterns (Bearer tokens, JWT tokens, common API
// key prefixes) are applied to every record before writing. This prevents sensitive
// credentials that appear in tool outputs from being persisted to the audit log.
func writeAuditWithAC(tc policy.ToolCall, d policy.Decision, auditPath string, ac policy.AgentContext) {
	w, err := audit.NewWriter(auditPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper check: audit writer unavailable: %v\n", err)
		return
	}
	defer w.Close()

	rec := audit.FromDecision(tc, d, newRecordID(), time.Now().UTC().Format(time.RFC3339), ac)
	// Apply sensitive field redaction before persisting (T-04-05-02).
	// DefaultRedactPatterns() compiles regexps once via sync.Once (WR-05).
	patterns := audit.DefaultRedactPatterns()
	rec = audit.RedactRecord(rec, patterns)
	if err := w.Write(rec); err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper check: audit write failed: %v\n", err)
	}
}

// Scannable is the injection interface for LLMF scanning in the hook handler.
// *llamafirewall.Supervisor satisfies this interface at runtime; tests inject mocks.
type Scannable interface {
	Scan(ctx context.Context, req llamafirewall.ScanRequest) (llamafirewall.ScanResponse, error)
	IsDegraded() bool
}

// postToolUseInput is the shape of Claude Code PostToolUse hook stdin JSON.
// The hook writes tool result data that beekeeper records for observability.
type postToolUseInput struct {
	HookEventName string          `json:"hook_event_name"`
	ToolName      string          `json:"tool_name"`
	ToolUseID     string          `json:"tool_use_id"`
	ToolResult    json.RawMessage `json:"tool_result"` // Phase 6: LLMF scan target
}

// RunAuditRecord reads a PostToolUse JSON payload from stdin, writes a
// tool_result audit record, and returns 0 always. PostToolUse hooks must not
// disrupt the agent — any error (malformed JSON, audit write failure) is logged
// to stderr and the function still returns 0.
//
// This function is the handler for the `beekeeper audit-record` subcommand,
// registered in cmd/beekeeper/main.go (Plan 05).
func RunAuditRecord(stdin io.Reader, auditPath string) int {
	// Cap stdin at the same 1MB limit as RunCheck for consistency.
	limited := io.LimitReader(stdin, maxStdin)
	var input postToolUseInput
	if err := json.NewDecoder(limited).Decode(&input); err != nil {
		// Malformed JSON is tolerated — PostToolUse must not disrupt agent.
		fmt.Fprintf(os.Stderr, "beekeeper audit-record: malformed stdin: %v\n", err)
		return 0
	}

	// Write a tool_result audit record. RecordType is overridden from
	// policy_decision to tool_result to reflect the PostToolUse semantics.
	tc := policy.ToolCall{ToolName: input.ToolName}
	d := policy.Decision{Allow: true, Level: "allow", Reason: "tool_result"}
	w, err := audit.NewWriter(auditPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper audit-record: audit writer unavailable: %v\n", err)
		return 0
	}
	defer w.Close()

	rec := audit.FromDecision(tc, d, newRecordID(), time.Now().UTC().Format(time.RFC3339), policy.AgentContext{})
	rec.RecordType = "tool_result"
	if err := w.Write(rec); err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper audit-record: audit write failed: %v\n", err)
	}
	return 0
}

// RunAuditRecordWithLLMF extends RunAuditRecord with optional LlamaFirewall scanning.
// If scanner is nil, behavior is identical to RunAuditRecord.
// This function handles the `beekeeper audit-record` command when LLMF is enabled.
func RunAuditRecordWithLLMF(stdin io.Reader, auditPath string, cfg config.Config, scanner Scannable) int {
	limited := io.LimitReader(stdin, maxStdin)
	var input postToolUseInput
	if err := json.NewDecoder(limited).Decode(&input); err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper audit-record: malformed stdin: %v\n", err)
		return 0
	}

	// Extract tool result content for LLMF scanning.
	var toolResultContent string
	if len(input.ToolResult) > 0 {
		// Try to unmarshal as string first, then as object.
		_ = json.Unmarshal(input.ToolResult, &toolResultContent)
		if toolResultContent == "" {
			toolResultContent = string(input.ToolResult)
		}
	}

	ctx := context.Background()
	tc := policy.ToolCall{ToolName: input.ToolName}
	d := policy.Decision{Allow: true, Level: "allow", Reason: "tool_result"}

	// LLMF scanning fields for the audit record.
	var llmfScanned bool
	var llmfScanKind, llmfResult string
	var llmfConfidence float64
	var llmfLatencyMS int64

	if scanner != nil && !scanner.IsDegraded() && toolResultContent != "" {
		if ScanPromptEligible(input.ToolName) {
			resp, err := scanner.Scan(ctx, llamafirewall.ScanRequest{
				Kind:      llamafirewall.ScanPrompt,
				Content:   toolResultContent,
				Context:   input.ToolName,
				RequestID: newRecordID(),
			})
			if err != nil {
				// Sidecar unavailable + fail-closed = block the tool result.
				if cfg.FailClosed() {
					fmt.Fprintf(os.Stderr, "beekeeper audit-record: LLMF sidecar unavailable (fail-closed), blocking result\n")
					writeLLMFAlertRecord(auditPath, input.ToolName, "scan_failed", 0, 0)
					return 1 // block PostToolUse
				}
				// fail-open: log but continue
				fmt.Fprintf(os.Stderr, "beekeeper audit-record: LLMF scan error (fail-open): %v\n", err)
			} else {
				llmfScanned = true
				llmfScanKind = "prompt"
				llmfResult = string(resp.Result)
				llmfConfidence = resp.Confidence
				llmfLatencyMS = resp.LatencyMS
				if resp.Result == llamafirewall.ResultInjection {
					// Write llmf_alert record for injection detection.
					writeLLMFAlertRecord(auditPath, input.ToolName, string(resp.Result), resp.Confidence, resp.LatencyMS)
				}
			}
		} else if ScanCodeEligible(input.ToolName) {
			resp, err := scanner.Scan(ctx, llamafirewall.ScanRequest{
				Kind:      llamafirewall.ScanCode,
				Content:   toolResultContent,
				Context:   input.ToolName,
				RequestID: newRecordID(),
			})
			if err == nil {
				llmfScanned = true
				llmfScanKind = "code"
				llmfResult = string(resp.Result)
				llmfConfidence = resp.Confidence
				llmfLatencyMS = resp.LatencyMS
				if resp.Result == llamafirewall.ResultUnsafe {
					writeLLMFAlertRecord(auditPath, input.ToolName, string(resp.Result), resp.Confidence, resp.LatencyMS)
					action := cfg.LlamaFirewall.CodeShieldAction
					if action == "" {
						action = "warn"
					}
					if action == "block" {
						return 1 // block
					}
					// warn: write warn audit record but continue (return 0)
					d.Level = "warn"
				}
			}
		}

		if cfg.LlamaFirewall.AlignmentCheck {
			resp, err := scanner.Scan(ctx, llamafirewall.ScanRequest{
				Kind:      llamafirewall.ScanAlignment,
				Content:   input.ToolName,
				Context:   "alignment_check",
				RequestID: newRecordID(),
			})
			if err == nil && resp.Result == llamafirewall.ResultHijacked {
				writeLLMFAlertRecord(auditPath, input.ToolName, string(resp.Result), resp.Confidence, resp.LatencyMS)
			}
		}
	}

	w, err := audit.NewWriter(auditPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper audit-record: audit writer unavailable: %v\n", err)
		return 0
	}
	defer w.Close()

	rec := audit.FromDecision(tc, d, newRecordID(), time.Now().UTC().Format(time.RFC3339), policy.AgentContext{})
	rec.RecordType = "tool_result"
	// Populate LLMF fields.
	rec.LLMFScanned = llmfScanned
	rec.LLMFScanKind = llmfScanKind
	rec.LLMFResult = llmfResult
	rec.LLMFConfidence = llmfConfidence
	rec.LLMFLatencyMS = llmfLatencyMS

	patterns := audit.DefaultRedactPatterns()
	rec = audit.RedactRecord(rec, patterns)
	if err := w.Write(rec); err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper audit-record: audit write failed: %v\n", err)
	}
	return 0
}

// ScanPromptEligible returns true when tool output should be PromptGuard-scanned.
// Delegates to llamafirewall.ShouldScanPrompt for consistency.
func ScanPromptEligible(toolName string) bool {
	return llamafirewall.ShouldScanPrompt(toolName)
}

// ScanCodeEligible returns true when tool output should be CodeShield-scanned.
func ScanCodeEligible(toolName string) bool {
	return llamafirewall.ShouldScanCode(toolName)
}

// writeLLMFAlertRecord appends an llmf_alert audit record. Errors are logged to
// stderr but never propagate — alerts are best-effort and must not disrupt writes.
func writeLLMFAlertRecord(auditPath, toolName, scanResult string, confidence float64, latencyMS int64) {
	w, err := audit.NewWriter(auditPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper: llmf alert audit write failed: %v\n", err)
		return
	}
	defer w.Close()
	rec := audit.AuditRecord{
		RecordType:     "llmf_alert",
		RecordID:       newRecordID(),
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		ScannerName:    "beekeeper",
		ToolName:       toolName,
		LLMFScanned:    true,
		LLMFResult:     scanResult,
		LLMFConfidence: confidence,
		LLMFLatencyMS:  latencyMS,
	}
	if err := w.Write(rec); err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper: llmf alert audit write error: %v\n", err)
	}
}

// newRecordID returns a random 128-bit hex identifier for an audit record. On
// the astronomically unlikely event the RNG fails, it falls back to a
// timestamp-derived id so a record is still attributable.
func newRecordID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("ts-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
