// Package check — impure nudge adapter for NUDGE-03/04/06/08.
//
// This file contains the I/O-bearing nudge functions that extract the bash
// command, call the EXPORTED nudge.DetectStateFn seam (fresh, no cache —
// Flag 2 Position B: the one-shot hook gets no cache hits), evaluate the pure
// nudge decision, and build the §9 audit record. All detection I/O lives HERE,
// mirroring paths.go's role for the SPATH feature.
//
// The exported nudge.DetectStateFn seam (not the private nudge.DetectState) is
// called so that a behavioral test in package check (Plan 07) can inject a
// synthetic PMState without a real pnpm/bun on PATH (T-08-10b: cross-package
// swap via defer-restore).
//
// nudge.ConfigFrom is the SINGLE config→nudge.Config mapper; there is NO local
// toNudgeConfig copy here (BLOCKER 1 closed, single-mapper rule).
package check

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/bantuson/beekeeper/internal/audit"
	"github.com/bantuson/beekeeper/internal/config"
	"github.com/bantuson/beekeeper/internal/nudge"
	"github.com/bantuson/beekeeper/internal/pkgparse"
	"github.com/bantuson/beekeeper/internal/policy"
)

// evaluateNudge resolves the PM state fresh via the EXPORTED nudge.DetectStateFn
// seam and returns a (policy.Decision, audit.AuditRecord, bool) triple.
//
// Returns ok=false (skip) when:
//   - tc.ToolName is not "Bash" (nudge only applies to shell commands)
//   - the Bash command string is absent or empty
//   - pkgparse.Parse reports !IsInstall (non-install commands like npm ls/run
//     never trigger detection — Pitfall 2)
//
// When ok=true:
//   - decision carries the merged Level (allow|warn|block) for mergeDecisions
//   - rec carries the §9 audit fields including NudgeAction (advise|proceed|rewrite|block)
//     — the closed §9 enum, distinct from the repo allow|warn|block Level vocabulary
//
// Detection runs through nudge.DetectStateFn (the EXPORTED seam). In production
// this calls nudge.DetectState with the 2s per-exec timeout. In tests, a Plan 07
// behavioral test can inject a synthetic PMState via:
//
//	orig := nudge.DetectStateFn
//	nudge.DetectStateFn = func(ctx context.Context, cfg nudge.Config) nudge.PMState {
//	    return syntheticState
//	}
//	defer func() { nudge.DetectStateFn = orig }()
//
// T-08-18: detection runs ONLY when parsed.IsInstall; non-install commands never exec.
// T-08-19: every nudge decision emits a record_type "nudge" record (§9 provenance).
func evaluateNudge(ctx context.Context, tc policy.ToolCall, nc config.NudgeConfig) (policy.Decision, audit.AuditRecord, bool) {
	// Only Bash tool calls carry a shell command string.
	if tc.ToolName != "Bash" {
		return policy.Decision{}, audit.AuditRecord{}, false
	}
	cmd, ok := tc.ToolInput["command"].(string)
	if !ok || cmd == "" {
		return policy.Decision{}, audit.AuditRecord{}, false
	}

	// Parse the install command. Non-install commands return ok=false immediately.
	parsed, parsedOK := pkgparse.Parse(cmd)
	if !parsedOK || !parsed.IsInstall {
		return policy.Decision{}, audit.AuditRecord{}, false
	}

	// Build nudge.Config via the LOCKED single mapper (BLOCKER 1 closed).
	// Do NOT define a local toNudgeConfig — call nudge.ConfigFrom.
	nudgeCfg := nudge.ConfigFrom(
		nc.Enabled,
		nc.Mode,
		nc.Preferred,
		nc.CheckSocketScanner,
		nc.VersionFloors.Pnpm,
		nc.VersionFloors.Bun,
		nc.VersionFloors.Node,
		nc.MajorDriftCheck.Enabled,
		nc.MajorDriftCheck.Interval,
	)

	// Resolve PMState FRESH via the EXPORTED seam (NOT nudge.DetectState directly).
	// The check hook is a one-shot process; no cache is effective here (Flag 2 Position B).
	// A behavioral test (Plan 07) in package check can inject detection by reassigning
	// nudge.DetectStateFn and defer-restoring it.
	state := nudge.DetectStateFn(ctx, nudgeCfg)

	// Pure decision — no I/O, no side effects.
	d := nudge.Evaluate(parsed, state, nudgeCfg)

	// Map nudge.Decision → policy.Decision using d.Level.
	// Advise/Rewrite → Level "warn" (exit 0); Block → "block"; Proceed → "allow".
	policyDec := nudgeLevelToDecision(d)

	// Build the §9 audit record (record_type "nudge").
	// NudgeAction is the closed §9 enum (ActionString), distinct from Level.
	rec := buildNudgeAuditRecord(tc, d)

	return policyDec, rec, true
}

// nudgeLevelToDecision maps a nudge Decision (with its Level) to a policy.Decision
// for mergeDecisions. The Level field on nudge.Decision already uses the
// "allow|warn|block" vocabulary (research A1 / levelFor), so we lift it directly.
func nudgeLevelToDecision(d nudge.Decision) policy.Decision {
	allow := d.Level != "block"
	reason := fmt.Sprintf("nudge(%s): %s", nudge.ActionString(d.Action), d.Reason)
	// On a block (mode=="block" enforcement), surface an actionable deny message:
	// Claude Code shows policy.Decision.Reason to the agent on a PreToolUse deny,
	// so tell it WHY and exactly what to run instead. A bare reason code would not
	// steer the agent (the whole point of block mode over a soft advisory).
	//
	// We offer BOTH hardened options — pnpm and bun — each with its concrete
	// equivalent command, computed by re-parsing the original command. Both have
	// strong supply-chain features (install cooldowns / minimumReleaseAge, strict
	// lockfile integrity, lifecycle-script controls) that plain npm/yarn lack.
	if d.Action == nudge.Block {
		pnpmCmd, bunCmd := "pnpm install", "bun install"
		if parsed, ok := pkgparse.Parse(d.Original); ok {
			pnpmCmd = nudge.RewriteToPnpm(parsed)
			bunCmd = nudge.RewriteToBun(parsed)
		}
		reason = fmt.Sprintf(
			"Beekeeper blocked this npm/yarn install to defend against supply-chain attacks. "+
				"Use a hardened package manager instead — either:\n"+
				"  • pnpm:  %s\n"+
				"  • bun:   %s\n"+
				"(Both add supply-chain protections plain npm lacks: install cooldowns, "+
				"strict lockfile integrity, and lifecycle-script controls.)",
			pnpmCmd, bunCmd,
		)
	}
	return policy.Decision{
		Allow:   allow,
		Level:   d.Level,
		Reason:  reason,
		RuleIDs: []string{"NUDGE-03"},
	}
}

// buildNudgeAuditRecord constructs the §9 audit record for a nudge decision.
// record_type is "nudge". NudgeAction is the closed §9 enum (advise|proceed|rewrite|block),
// distinct from the repo's allow|warn|block Decision field (T-08-09b).
func buildNudgeAuditRecord(tc policy.ToolCall, d nudge.Decision) audit.AuditRecord {
	rec := audit.AuditRecord{
		RecordType:      "nudge",
		RecordID:        newRecordID(),
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
		ScannerName:     "beekeeper",
		ToolName:        tc.ToolName,
		Decision:        d.Level,
		Reason:          d.Reason,
		Endpoint:        "check",
		OriginalCommand: d.Original,
		ReasonCode:      d.Reason,
		NudgeAction:     nudge.ActionString(d.Action),
	}
	if d.Rewritten != "" {
		rec.RewrittenCommand = d.Rewritten
	}
	// Use the pm_state string already prepared by nudge.buildAuditFields (via AuditFields).
	if ps, ok := d.AuditFields["pm_state"].(string); ok {
		rec.PMState = ps
	}
	return rec
}

// writeNudgeAuditRecord appends a nudge audit record best-effort.
// A write failure is logged to stderr but NEVER changes the decision
// (mirrors writeLLMFAlertRecord — audit failures are non-fatal).
func writeNudgeAuditRecord(auditPath string, rec audit.AuditRecord) {
	if auditPath == "" {
		return
	}
	w, err := audit.NewWriter(auditPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper: nudge audit write failed: %v\n", err)
		return
	}
	defer w.Close()
	// WR-01: redact sensitive command fields before writing, consistent with the
	// main audit path (writeAuditWithAC). OriginalCommand/RewrittenCommand carry
	// the verbatim agent-supplied Bash command, which may embed a token/secret.
	rec = audit.RedactRecord(rec, audit.DefaultRedactPatterns())
	if err := w.Write(rec); err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper: nudge audit write error: %v\n", err)
	}
}
