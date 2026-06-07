package check

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bantuson/beekeeper/internal/audit"
	"github.com/bantuson/beekeeper/internal/config"
	"github.com/bantuson/beekeeper/internal/nudge"
	"github.com/bantuson/beekeeper/internal/pkgparse"
	"github.com/bantuson/beekeeper/internal/policy"
)

// TestNudgeLevelToDecisionBlockParseable covers the block branch of
// nudgeLevelToDecision when d.Original is a parseable npm install command. The
// deny message must (1) deny, (2) be level "block", (3) carry the NUDGE-03 rule
// id, and (4) offer BOTH a concrete pnpm AND a concrete bun command so the agent
// has an actionable hardened path. This is the user-facing payload of block mode.
func TestNudgeLevelToDecisionBlockParseable(t *testing.T) {
	d := nudge.Decision{
		Action:   nudge.Block,
		Level:    "block",
		Reason:   nudge.ReasonPnpmEnforceBlock,
		Original: "npm install lodash",
	}
	got := nudgeLevelToDecision(d)

	if got.Allow {
		t.Errorf("Allow = true, want false for a Block decision")
	}
	if got.Level != "block" {
		t.Errorf("Level = %q, want %q", got.Level, "block")
	}
	if !containsRule(got.RuleIDs, "NUDGE-03") {
		t.Errorf("RuleIDs = %v, want to contain NUDGE-03", got.RuleIDs)
	}
	// The deny message must mention both hardened managers by name.
	if !strings.Contains(got.Reason, "pnpm") {
		t.Errorf("Reason missing pnpm guidance:\n%s", got.Reason)
	}
	if !strings.Contains(got.Reason, "bun") {
		t.Errorf("Reason missing bun guidance:\n%s", got.Reason)
	}
	// Parseable original → the message should carry the concrete equivalents
	// ("pnpm add lodash" / "bun add lodash"), not the generic fallbacks.
	if !strings.Contains(got.Reason, "pnpm add lodash") {
		t.Errorf("Reason missing concrete pnpm equivalent (pnpm add lodash):\n%s", got.Reason)
	}
	if !strings.Contains(got.Reason, "bun add lodash") {
		t.Errorf("Reason missing concrete bun equivalent (bun add lodash):\n%s", got.Reason)
	}
}

// TestNudgeLevelToDecisionBlockUnparseable covers the fallback path: when
// pkgparse.Parse(d.Original) returns ok=false, the deny message must still offer
// the generic "pnpm install" / "bun install" defaults rather than panicking or
// emitting an empty suggestion. An empty Original is a non-install string for
// pkgparse (ok=false).
func TestNudgeLevelToDecisionBlockUnparseable(t *testing.T) {
	// Sanity-check the precondition: the chosen Original is genuinely unparseable.
	for _, orig := range []string{"", "echo hello", "npm ls"} {
		if _, ok := pkgparse.Parse(orig); ok {
			t.Fatalf("precondition failed: pkgparse.Parse(%q) returned ok=true; pick a non-install string", orig)
		}
	}

	d := nudge.Decision{
		Action:   nudge.Block,
		Level:    "block",
		Reason:   nudge.ReasonPnpmEnforceBlock,
		Original: "", // unparseable → fallback defaults
	}
	got := nudgeLevelToDecision(d)

	if got.Allow {
		t.Errorf("Allow = true, want false")
	}
	if got.Level != "block" {
		t.Errorf("Level = %q, want block", got.Level)
	}
	if !strings.Contains(got.Reason, "pnpm install") {
		t.Errorf("Reason missing fallback 'pnpm install':\n%s", got.Reason)
	}
	if !strings.Contains(got.Reason, "bun install") {
		t.Errorf("Reason missing fallback 'bun install':\n%s", got.Reason)
	}
}

// TestNudgeLevelToDecisionNonBlock covers the non-block (Advise) path: the
// decision must ALLOW and carry the soft reason verbatim (no deny message).
func TestNudgeLevelToDecisionNonBlock(t *testing.T) {
	d := nudge.Decision{
		Action:   nudge.Advise,
		Level:    "warn",
		Reason:   nudge.ReasonPnpmAvailableSoft,
		Original: "npm install foo",
	}
	got := nudgeLevelToDecision(d)

	if !got.Allow {
		t.Errorf("Allow = false, want true for an Advise decision")
	}
	if got.Level != "warn" {
		t.Errorf("Level = %q, want warn", got.Level)
	}
	// The reason is the soft advisory, prefixed with the nudge action — it must NOT
	// be the block deny message.
	if strings.Contains(got.Reason, "Beekeeper blocked") {
		t.Errorf("Advise decision unexpectedly carries the block deny message:\n%s", got.Reason)
	}
	if !strings.Contains(got.Reason, nudge.ReasonPnpmAvailableSoft) {
		t.Errorf("Reason = %q, want to contain %q", got.Reason, nudge.ReasonPnpmAvailableSoft)
	}
}

// containsRule reports whether ids contains want.
func containsRule(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

// TestBuildNudgeAuditRecordRewrittenAndPMState covers the two previously-uncovered
// branches of buildNudgeAuditRecord: the d.Rewritten != "" branch (RewrittenCommand
// set) and the pm_state AuditFields branch (PMState set).
func TestBuildNudgeAuditRecordRewrittenAndPMState(t *testing.T) {
	tc := policy.ToolCall{
		ToolName: "Bash",
		ToolInput: map[string]any{
			"command": "npm install foo",
		},
	}
	d := nudge.Decision{
		Action:    nudge.Rewrite,
		Level:     "warn",
		Reason:    nudge.ReasonPnpmHardRewrite,
		Original:  "npm install foo",
		Rewritten: "pnpm add foo",
		AuditFields: map[string]any{
			"pm_state": "pnpm=11.5.1(hardened=true) bun=(scanner=false) node=22.5.0",
		},
	}

	rec := buildNudgeAuditRecord(tc, d)

	if rec.RecordType != "nudge" {
		t.Errorf("RecordType = %q, want %q", rec.RecordType, "nudge")
	}
	if rec.RewrittenCommand != "pnpm add foo" {
		t.Errorf("RewrittenCommand = %q, want %q (d.Rewritten != \"\" branch)", rec.RewrittenCommand, "pnpm add foo")
	}
	if rec.PMState != "pnpm=11.5.1(hardened=true) bun=(scanner=false) node=22.5.0" {
		t.Errorf("PMState = %q, want the AuditFields[pm_state] value", rec.PMState)
	}
	if rec.NudgeAction != "rewrite" {
		t.Errorf("NudgeAction = %q, want %q", rec.NudgeAction, "rewrite")
	}
	if rec.OriginalCommand != "npm install foo" {
		t.Errorf("OriginalCommand = %q, want %q", rec.OriginalCommand, "npm install foo")
	}
}

// TestBuildNudgeAuditRecordNoRewriteNoPMState covers the complementary branches:
// when d.Rewritten == "" RewrittenCommand stays empty, and when AuditFields lacks
// a pm_state key (or is nil) PMState stays empty — no panic on the type assertion.
func TestBuildNudgeAuditRecordNoRewriteNoPMState(t *testing.T) {
	tc := policy.ToolCall{ToolName: "Bash"}
	d := nudge.Decision{
		Action:      nudge.Proceed,
		Level:       "allow",
		Reason:      nudge.ReasonNoHardenedPM,
		Original:    "npm install foo",
		Rewritten:   "",  // → RewrittenCommand must stay empty
		AuditFields: nil, // → PMState must stay empty (no panic)
	}

	rec := buildNudgeAuditRecord(tc, d)

	if rec.RewrittenCommand != "" {
		t.Errorf("RewrittenCommand = %q, want empty (no rewrite)", rec.RewrittenCommand)
	}
	if rec.PMState != "" {
		t.Errorf("PMState = %q, want empty (no pm_state field)", rec.PMState)
	}
	if rec.NudgeAction != "proceed" {
		t.Errorf("NudgeAction = %q, want %q", rec.NudgeAction, "proceed")
	}
}

// TestWriteNudgeAuditRecordEmptyPath covers the early-return guard: an empty
// auditPath must be a no-op (no panic, no file).
func TestWriteNudgeAuditRecordEmptyPath(t *testing.T) {
	// Must not panic; nothing to assert on disk.
	writeNudgeAuditRecord("", audit.AuditRecord{RecordType: "nudge"})
}

// TestWriteNudgeAuditRecordHappyPath covers the success path: a valid temp path
// receives a single redacted NDJSON record. We assert the record round-trips and
// that a secret embedded in the command is REDACTED (WR-01) — proving the redact
// step in writeNudgeAuditRecord is actually applied, not just that a write happened.
func TestWriteNudgeAuditRecordHappyPath(t *testing.T) {
	auditPath := filepath.Join(t.TempDir(), "audit", "beekeeper.ndjson")

	const secret = "ghp_DEADBEEFdeadbeef0123456789abcdefABCD"
	rec := audit.AuditRecord{
		RecordType:      "nudge",
		ToolName:        "Bash",
		Decision:        "block",
		NudgeAction:     "block",
		OriginalCommand: "npm install foo --token=" + secret,
	}

	writeNudgeAuditRecord(auditPath, rec)

	got := readLastAuditRecord(t, auditPath)
	if got.RecordType != "nudge" {
		t.Fatalf("RecordType = %q, want %q (record not written)", got.RecordType, "nudge")
	}
	if got.NudgeAction != "block" {
		t.Errorf("NudgeAction = %q, want %q", got.NudgeAction, "block")
	}
	// The secret must NOT survive into the audit log verbatim (WR-01 redaction).
	if strings.Contains(got.OriginalCommand, secret) {
		t.Errorf("audit OriginalCommand leaked the secret verbatim: %q", got.OriginalCommand)
	}
}

// TestWriteNudgeAuditRecordWriterError covers the audit.NewWriter error branch:
// when the audit path's parent cannot be created (a regular file sits where a
// directory must be), NewWriter fails and writeNudgeAuditRecord returns without
// panicking and without changing any decision (audit failures are non-fatal).
func TestWriteNudgeAuditRecordWriterError(t *testing.T) {
	dir := t.TempDir()
	// Create a regular FILE named "blocker"; then ask to write the audit log at
	// blocker/audit/beekeeper.ndjson — os.MkdirAll("blocker/audit") must fail
	// because "blocker" is not a directory.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("seed blocker file: %v", err)
	}
	badPath := filepath.Join(blocker, "audit", "beekeeper.ndjson")

	// Must not panic; the error path just logs to stderr and returns.
	writeNudgeAuditRecord(badPath, audit.AuditRecord{RecordType: "nudge"})

	// And nothing should have been created at the bad path.
	if _, err := os.Stat(badPath); err == nil {
		t.Errorf("audit record unexpectedly written to %q despite NewWriter error", badPath)
	}
}

// TestEvaluateNudgeBlockModeLiveBehavior is the REGRESSION TEST for the live
// production behavior on this machine: nudge.mode=block + an "npm install <pkg>"
// Bash tool call must produce a DENY (Allow=false, Level=block) whose reason
// offers BOTH pnpm and bun hardened equivalents.
//
// It drives the real evaluateNudge entry point (config → pkgparse → nudge.Evaluate
// → nudgeLevelToDecision) with a mode=block config, and injects a synthetic
// hardened-pnpm PMState via the EXPORTED nudge.DetectStateFn seam so the test does
// not depend on a real pnpm/bun being on PATH. Block mode is detection-INDEPENDENT,
// so the block would fire regardless — but injecting Node>=22 also exercises the
// non-dead-end path explicitly.
func TestEvaluateNudgeBlockModeLiveBehavior(t *testing.T) {
	orig := nudge.DetectStateFn
	nudge.DetectStateFn = func(_ context.Context, _ nudge.Config) nudge.PMState {
		return nudge.PMState{
			PnpmInstalled: true,
			PnpmVersion:   "11.5.1",
			PnpmHardened:  true,
			NodeVersion:   "22.5.0",
		}
	}
	defer func() { nudge.DetectStateFn = orig }()

	nc := config.DefaultNudgeConfig()
	nc.Mode = "block"

	tc := policy.ToolCall{
		ToolName: "Bash",
		ToolInput: map[string]any{
			"command": "npm install left-pad",
		},
	}

	dec, rec, ok := evaluateNudge(context.Background(), tc, nc)
	if !ok {
		t.Fatalf("evaluateNudge ok = false, want true for an npm install command")
	}

	// 1. The decision must DENY.
	if dec.Allow {
		t.Errorf("Allow = true, want false — mode=block must DENY npm install (live regression)")
	}
	if dec.Level != "block" {
		t.Errorf("Level = %q, want block", dec.Level)
	}
	if !containsRule(dec.RuleIDs, "NUDGE-03") {
		t.Errorf("RuleIDs = %v, want to contain NUDGE-03", dec.RuleIDs)
	}

	// 2. The deny reason must offer BOTH hardened managers, with the concrete
	//    package the agent asked for.
	if !strings.Contains(dec.Reason, "pnpm add left-pad") {
		t.Errorf("Reason missing concrete pnpm equivalent:\n%s", dec.Reason)
	}
	if !strings.Contains(dec.Reason, "bun add left-pad") {
		t.Errorf("Reason missing concrete bun equivalent:\n%s", dec.Reason)
	}

	// 3. The §9 audit record must reflect the block.
	if rec.RecordType != "nudge" {
		t.Errorf("audit RecordType = %q, want nudge", rec.RecordType)
	}
	if rec.NudgeAction != "block" {
		t.Errorf("audit NudgeAction = %q, want block", rec.NudgeAction)
	}
	if rec.Decision != "block" {
		t.Errorf("audit Decision = %q, want block", rec.Decision)
	}
}

// TestEvaluateNudgeBlockModeBypassResistance is a security regression: the
// compound-command and env-prefixed bypasses (`cd x && npm install evil`,
// `NODE_ENV=prod npm install evil`) must STILL be denied under mode=block. These
// were a real bypass that escaped the prefix parser; this locks in the fix at the
// check-adapter level (evaluateNudge → pkgparse.Parse compound coverage).
func TestEvaluateNudgeBlockModeBypassResistance(t *testing.T) {
	orig := nudge.DetectStateFn
	nudge.DetectStateFn = func(_ context.Context, _ nudge.Config) nudge.PMState {
		return nudge.PMState{
			PnpmInstalled: true,
			PnpmVersion:   "11.5.1",
			PnpmHardened:  true,
			NodeVersion:   "22.5.0",
		}
	}
	defer func() { nudge.DetectStateFn = orig }()

	nc := config.DefaultNudgeConfig()
	nc.Mode = "block"

	bypassCommands := []string{
		"cd /project && npm install evil-pkg",
		"NODE_ENV=production npm install evil-pkg",
	}
	for _, cmd := range bypassCommands {
		t.Run(cmd, func(t *testing.T) {
			tc := policy.ToolCall{
				ToolName:  "Bash",
				ToolInput: map[string]any{"command": cmd},
			}
			dec, _, ok := evaluateNudge(context.Background(), tc, nc)
			if !ok {
				t.Fatalf("evaluateNudge ok = false for %q; compound/env-prefixed install must be detected", cmd)
			}
			if dec.Allow {
				t.Errorf("Allow = true for %q, want false — bypass must still be blocked", cmd)
			}
			if dec.Level != "block" {
				t.Errorf("Level = %q for %q, want block", dec.Level, cmd)
			}
		})
	}
}
