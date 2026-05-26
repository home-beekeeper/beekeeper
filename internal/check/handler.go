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
	"os"
	"runtime/debug"
	"time"

	"github.com/mzansi-agentive/beekeeper/internal/audit"
	"github.com/mzansi-agentive/beekeeper/internal/catalog"
	"github.com/mzansi-agentive/beekeeper/internal/config"
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

// catalogOpener opens a catalog index by path. The default wraps
// catalog.OpenIndex; tests substitute a fake to exercise the fail-closed panic
// path without depending on a real index file. The returned value satisfies
// catalog.Indexer; if it also implements io.Closer the handler closes it.
//
// Note: this previously used policy.CatalogLookup, which was removed in Plan 05
// to break the import cycle between internal/catalog (adapter → policy.CatalogMatch)
// and internal/policy (CatalogLookup → catalog.Entry). catalog.Indexer is the
// equivalent interface defined in the catalog package. Plan 08 will update this
// to use policy.MultiCatalogLookup when the handler is rewired for corroboration.
type catalogOpener func(path string) (catalog.Indexer, error)

func defaultOpener(path string) (catalog.Indexer, error) {
	idx, err := catalog.OpenIndex(path)
	if err != nil {
		return nil, err
	}
	// *catalog.Index satisfies catalog.Indexer and io.Closer directly.
	return idx, nil
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
func RunCheck(ctx context.Context, stdin io.Reader, cfg config.Config, indexPath, auditPath string) Result {
	return runCheck(ctx, stdin, cfg, indexPath, auditPath, defaultOpener)
}

// runCheck is the testable core; opener is injected so tests can force a panic
// or error from index opening.
func runCheck(ctx context.Context, stdin io.Reader, cfg config.Config, indexPath, auditPath string, open catalogOpener) (result Result) {
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
			result = finalize(d, cfg, toolCall, auditPath)
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

	dec := json.NewDecoder(limited)
	if err := dec.Decode(&toolCall); err != nil {
		// Distinguish oversized input from genuinely malformed JSON: if the
		// limited reader is exhausted we very likely truncated a large payload.
		if limited.N <= 0 {
			return finalize(failDecision(cfg, "stdin exceeds 1MB cap (fail-closed)"), cfg, toolCall, auditPath)
		}
		return finalize(failDecision(cfg, "invalid tool call JSON (fail-closed)"), cfg, toolCall, auditPath)
	}

	// Oversized detection for valid-but-too-large input: a successful decode
	// that consumed everything up to (and including) the extra cap byte means
	// the payload was at least maxStdin+1 bytes.
	if limited.N <= 0 {
		return finalize(failDecision(cfg, "stdin exceeds 1MB cap (fail-closed)"), cfg, toolCall, auditPath)
	}

	// Early timeout check after the (potentially slow) stdin read.
	if ctx.Err() != nil {
		return finalize(failDecision(cfg, "execution timeout (fail-closed)"), cfg, toolCall, auditPath)
	}

	// HOOK-02: load the catalog via the mmap index, never a cold JSON parse.
	idx, err := open(indexPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper check: catalog index unavailable: %v\n", err)
		return finalize(failDecision(cfg, "catalog index unavailable (fail-closed)"), cfg, toolCall, auditPath)
	}
	if c, ok := idx.(io.Closer); ok {
		defer c.Close()
	}

	// Re-check the deadline before the pure evaluation.
	if ctx.Err() != nil {
		return finalize(failDecision(cfg, "execution timeout (fail-closed)"), cfg, toolCall, auditPath)
	}

	// Pure, synchronous policy evaluation (no I/O, no goroutines).
	decision := policy.Evaluate(toolCall, idx)

	// Final deadline check: if we blew the budget during evaluation, fail closed
	// rather than emit a possibly-stale allow.
	if ctx.Err() != nil {
		return finalize(failDecision(cfg, "execution timeout (fail-closed)"), cfg, toolCall, auditPath)
	}

	// Successful evaluation results are NOT subject to fail-mode overrides —
	// fail modes only govern the failure paths above.
	return finalize(decision, cfg, toolCall, auditPath)
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
func finalize(d policy.Decision, cfg config.Config, tc policy.ToolCall, auditPath string) Result {
	writeAudit(tc, d, auditPath)

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

// writeAudit appends one NDJSON record for the decision. An audit-write failure
// is logged to stderr but NEVER downgrades the decision — a block stays a block
// even if it could not be recorded.
func writeAudit(tc policy.ToolCall, d policy.Decision, auditPath string) {
	w, err := audit.NewWriter(auditPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper check: audit writer unavailable: %v\n", err)
		return
	}
	defer w.Close()

	rec := audit.FromDecision(tc, d, newRecordID(), time.Now().UTC().Format(time.RFC3339))
	if err := w.Write(rec); err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper check: audit write failed: %v\n", err)
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
