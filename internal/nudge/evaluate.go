package nudge

// PURE — imports only "fmt" and the pure packages (pkgparse imports only "strings").
// No os/exec/time/sync/context/io/net. Enforced by TestNudgeEvaluateImportsArePure.
//
// Evaluate is the heart of the nudge feature: a pure decision over a
// caller-resolved PMState, mirroring policy.EvaluateReleaseAge exactly.
// Detection I/O lives in detect.go/an adapter — it resolves PMState and passes
// it here. This function never exec's a subprocess, reads a file, or accesses
// the clock (T-08-06).

import (
	"fmt"

	"github.com/home-beekeeper/beekeeper/internal/pkgparse"
)

// Action is the nudge decision: what Beekeeper recommends doing with the command.
type Action int

const (
	// Proceed means the command is not nudged (feature disabled, non-install, or
	// no hardened PM with requireHardened=false). The agent's command runs as-is.
	Proceed Action = iota
	// Advise means Beekeeper recommends a better PM but lets the command proceed.
	// Level "warn" (exit 0 — soft advisory, not a block). PRD §3.2.
	Advise
	// Rewrite means Beekeeper rewrites the command to its pnpm/bun equivalent.
	// Only in hard mode. Level "warn" (exit 0 — the rewrite is advisory/audit
	// only; Beekeeper does not execute the rewritten command). PRD §3.2.
	Rewrite
	// Block means Beekeeper blocks the command entirely. Only when
	// cfg.RequireHardened=true and no hardened PM is installed. Level "block".
	Block
)

// ActionString maps an Action to the closed §9 audit enum string
// ("proceed" | "advise" | "rewrite" | "block"). This is the nudge_action audit
// field — deliberately separate from the repo's existing "allow|warn|block"
// Decision/Level vocabulary so §9 semantics are preserved through the forensic
// record (BLOCKER 2 / T-08-09b resolution).
//
// Out-of-range values return "proceed" (safe default) — never panics.
func ActionString(a Action) string {
	switch a {
	case Proceed:
		return "proceed"
	case Advise:
		return "advise"
	case Rewrite:
		return "rewrite"
	case Block:
		return "block"
	default:
		return "proceed" // safe default for any out-of-range int
	}
}

// PMState is the caller-resolved local package-manager state.
// All detection I/O happens before Evaluate is called (in detect.go/adapter).
type PMState struct {
	NpmInstalled bool
	NpmVersion   string

	PnpmInstalled bool
	PnpmVersion   string
	// PnpmHardened is true when pnpm version meets the floor (>= 11.0.0).
	// Set by the detection adapter.
	PnpmHardened bool

	BunInstalled bool
	BunVersion   string
	// BunScannerOK is true when @socketsecurity/bun-security-scanner is present
	// in bunfig.toml (either project root or ~/.bunfig.toml).
	BunScannerOK bool

	// NodeVersion is the active Node.js version string, e.g. "22.5.0".
	// Required for the pnpm 11 Node >= 22 compatibility check (§10-6).
	NodeVersion string
}

// Decision is what Evaluate returns for a parsed install command.
// It carries the Action, structured reason code, original and rewritten command
// strings, the detected PMState for the audit record, and AuditFields for the
// §9 schema.
type Decision struct {
	// Action is the nudge recommendation.
	Action Action
	// Reason is the structured reason code from the closed enum in reasons.go.
	Reason string
	// Original is the original command as invoked (cmd.Raw).
	Original string
	// Rewritten is the pnpm/bun equivalent command string. Only populated when
	// Action == Rewrite. Advisory only — Beekeeper does not execute it.
	Rewritten string
	// Detected is the PMState that was passed to Evaluate (for audit provenance).
	Detected PMState
	// Level maps to the repo's existing "allow|warn|block" vocabulary for
	// mergeDecisions / exitCodeFor compatibility (research Pattern 2, A1):
	//   Advise  → "warn"  (exit 0)
	//   Rewrite → "warn"  (exit 0)
	//   Block   → "block" (exit 1)
	//   Proceed → "allow" (exit 0)
	Level string
	// AuditFields carries the §9 audit schema fields:
	//   nudge_action (closed §9 enum via ActionString — NOT "allow|warn|block")
	//   reason_code
	//   original_command
	//   rewritten_command (when Action == Rewrite)
	//   pm_state (flattened)
	// AuditFields does NOT set "decision" — the allow|warn|block mapping is done
	// by the Plan 06 adapter from Level, so the two vocabularies stay separate.
	AuditFields map[string]any
}

// Evaluate is a pure function: given a caller-resolved ParsedCommand and PMState
// plus a Config, it returns a Decision without any I/O, goroutines, globals
// mutation, or wall-clock access (T-08-06). Mirrors EvaluateReleaseAge exactly.
//
// Decision flow (PRD §4):
//  1. If cfg.Enabled is false OR cmd.IsInstall is false → Proceed/not-applicable.
//     1b. If cmd.Ecosystem != "npm" (pip/go/gem/cargo/composer) → Proceed/not-applicable.
//     1c. If cmd.Manager is already pnpm/bun → Proceed/already-hardened-pm.
//  2. If cmd.Sudo → Advise/sudo-passthrough, NEVER Rewrite/Block (§10-10, T-08-07).
//     2b. BLOCK MODE: if mode == "block" and not exec and Node not known-too-old →
//     Block/pnpm-enforce-block (or bun-enforce-block when Preferred==bun).
//     Detection-INDEPENDENT enforcement — does NOT fail open on a flaky pnpm probe.
//  3. If pnpm installed and hardened (soft/hard advisory path):
//     a. If Node < floor → Advise/node-incompatible-with-pnpm-11 (§10-6).
//     b. If mode == "hard" → Rewrite to pnpm (§10-2).
//     c. Else → Advise/pnpm-available-soft (§10-1); no-arg gets softer reason (§10-8).
//  4. Else if bun installed and meets floor:
//     a. If scanner absent → Advise/bun-available-no-scanner (§10-5).
//     b. If mode == "hard" → Rewrite to bun.
//     c. Else → Advise/bun-available-soft.
//  5. If cfg.RequireHardened → Block/no-hardened-pm (§10-4).
//     Else → Proceed/no-hardened-pm (§10-3).
//
// When BOTH pnpm and bun are hardened, cfg.Preferred selects which branch to
// enter first (default "pnpm").
func Evaluate(cmd pkgparse.ParsedCommand, state PMState, cfg Config) Decision {
	// 1. Short-circuit: disabled or non-install.
	if !cfg.Enabled || !cmd.IsInstall {
		return makeDecision(Proceed, ReasonNotApplicable, cmd.Raw, "", state)
	}

	// 1b. Scope: the nudge targets the npm/JS ecosystem only. pip/go/gem/cargo/
	// composer installs proceed unchanged — a JS package manager cannot replace
	// them, so nudging (and especially BLOCKING) them would be wrong.
	if cmd.Ecosystem != "npm" {
		return makeDecision(Proceed, ReasonNotApplicable, cmd.Raw, "", state)
	}

	// 1c. If the command ALREADY uses a hardened PM (pnpm/bun), proceed — there is
	// nothing to steer toward. This is what prevents block mode from blocking
	// `pnpm install` itself, and removes the redundant pnpm→pnpm advisory that
	// soft/hard mode would otherwise emit for an already-hardened command.
	if cmd.Manager == "pnpm" || cmd.Manager == "bun" {
		return makeDecision(Proceed, ReasonAlreadyHardened, cmd.Raw, "", state)
	}

	// 2. Sudo passthrough — parse + log, NEVER rewrite/block (§10-10, T-08-07).
	// A privileged command is never blocked even in block mode (blocking a
	// `sudo npm install` is more disruptive than the advisory it replaces).
	if cmd.Sudo {
		return makeDecision(Advise, ReasonSudoPassthrough, cmd.Raw, "", state)
	}

	// 2b. BLOCK MODE (supply-chain enforcement). Deny unhardened npm/yarn installs
	// outright, INDEPENDENT of PM detection. This is deliberately NOT gated on the
	// pnpm/bun version probe: that probe is flaky on Windows corepack `.cmd` shims
	// (it intermittently exceeds its subprocess deadline), and a security
	// ENFORCEMENT control must never fail OPEN just because a slow subprocess timed
	// out — that would let a compromised `npm install` through exactly when the
	// agent is under attack. We have already excluded the cases where blocking
	// would be wrong: non-npm ecosystems (1b), commands already using a hardened PM
	// (1c), and sudo (2). Exec verbs (npx/dlx/x) are also left to the advisory path
	// — blocking an exec is too disruptive and not always rewritable.
	//
	// The single carve-out: an explicitly-detected too-old Node (< pnpm 11 floor).
	// Node detection is reliable (a real `node.exe`, ~50ms), so when we positively
	// know Node is too old, blocking would dead-end the agent (no pnpm 11 path); we
	// fall through to the detection-based advisory instead. An UNKNOWN Node version
	// still blocks (fail-closed for an enforcement control).
	if cfg.Mode == "block" && !cmd.IsExec {
		nodeKnownTooOld := state.NodeVersion != "" &&
			!meetsFloor(state.NodeVersion, cfg.VersionFloors.Node)
		if !nodeKnownTooOld {
			if cfg.Preferred == "bun" {
				return makeDecisionBlock(ReasonBunEnforceBlock, cmd.Raw, rewriteToBun(cmd), state)
			}
			return makeDecisionBlock(ReasonPnpmEnforceBlock, cmd.Raw, rewriteToPnpm(cmd), state)
		}
		// Node too old → fall through to the advisory path below (never block into
		// a dead end).
	}

	// Determine whether pnpm and bun meet their respective floors.
	pnpmReady := state.PnpmInstalled && state.PnpmHardened &&
		meetsFloor(state.PnpmVersion, cfg.VersionFloors.Pnpm)
	bunReady := state.BunInstalled &&
		meetsFloor(state.BunVersion, cfg.VersionFloors.Bun)

	// Preferred-PM ordering: when both are ready, enter the preferred branch first.
	preferBun := cfg.Preferred == "bun"

	if pnpmReady && !preferBun || (pnpmReady && preferBun && !bunReady) {
		return evaluatePnpm(cmd, state, cfg)
	}
	if bunReady {
		return evaluateBun(cmd, state, cfg)
	}
	if pnpmReady {
		// pnpmReady but not selected first (preferBun=true and bunReady was false path led here)
		return evaluatePnpm(cmd, state, cfg)
	}

	// 5. No hardened PM.
	if cfg.RequireHardened {
		// §10-4: block
		return makeDecision(Block, ReasonNoHardenedPM, cmd.Raw, "", state)
	}
	// §10-3: proceed
	return makeDecision(Proceed, ReasonNoHardenedPM, cmd.Raw, "", state)
}

// evaluatePnpm handles the pnpm branch of the decision tree.
// (Block mode is handled earlier in Evaluate, detection-independent, so it is
// not repeated here — this branch only runs when pnpm was actually detected.)
func evaluatePnpm(cmd pkgparse.ParsedCommand, state PMState, cfg Config) Decision {
	// §10-6: Node.js version must meet the floor for pnpm 11.
	if !meetsFloor(state.NodeVersion, cfg.VersionFloors.Node) {
		return makeDecision(Advise, ReasonNodeIncompatiblePnpm11, cmd.Raw, "", state)
	}
	// §10-2: hard mode → rewrite.
	if cfg.Mode == "hard" {
		rewritten := rewriteToPnpm(cmd)
		return makeDecisionRewrite(ReasonPnpmHardRewrite, cmd.Raw, rewritten, state)
	}
	// §10-1 / §10-8: soft mode — no-arg install gets a softer reason code.
	reason := ReasonPnpmAvailableSoft
	if cmd.Package == "" {
		reason = ReasonNoArgInstallSoft // §10-8
	}
	return makeDecision(Advise, reason, cmd.Raw, "", state)
}

// evaluateBun handles the bun branch of the decision tree.
func evaluateBun(cmd pkgparse.ParsedCommand, state PMState, cfg Config) Decision {
	// §10-5: scanner absent → advisory to install it (bun is not fully hardened
	// without the Socket scanner, so we advise rather than block toward it).
	if cfg.CheckSocketScanner && !state.BunScannerOK {
		return makeDecision(Advise, ReasonBunAvailableNoScanner, cmd.Raw, "", state)
	}
	// Hard mode → rewrite.
	if cfg.Mode == "hard" {
		rewritten := rewriteToBun(cmd)
		return makeDecisionRewrite(ReasonPnpmHardRewrite, cmd.Raw, rewritten, state)
		// Note: we reuse ReasonPnpmHardRewrite here — the plan's reason enum does
		// not define a separate "bun-hard-rewrite". If the PRD later adds one this
		// is where it would go.
	}
	// Soft → advise bun.
	reason := ReasonBunAvailableSoft
	if cmd.Package == "" {
		reason = ReasonNoArgInstallSoft
	}
	return makeDecision(Advise, reason, cmd.Raw, "", state)
}

// makeDecision builds a Decision value for Advise/Proceed/Block cases.
func makeDecision(act Action, reason, original, _ string, state PMState) Decision {
	level := levelFor(act)
	fields := buildAuditFields(act, reason, original, "", state)
	return Decision{
		Action:      act,
		Reason:      reason,
		Original:    original,
		Detected:    state,
		Level:       level,
		AuditFields: fields,
	}
}

// makeDecisionRewrite builds a Decision value for Rewrite cases.
func makeDecisionRewrite(reason, original, rewritten string, state PMState) Decision {
	fields := buildAuditFields(Rewrite, reason, original, rewritten, state)
	return Decision{
		Action:      Rewrite,
		Reason:      reason,
		Original:    original,
		Rewritten:   rewritten,
		Detected:    state,
		Level:       levelFor(Rewrite),
		AuditFields: fields,
	}
}

// makeDecisionBlock builds a Decision value for the block-mode enforcement path.
// suggested is the hardened-PM equivalent command (e.g. "pnpm install") carried
// in Rewritten so the deny message can tell the agent exactly what to run instead.
// Level is "block" (Allow=false downstream) — this is the only nudge path that
// actually denies the tool call.
func makeDecisionBlock(reason, original, suggested string, state PMState) Decision {
	fields := buildAuditFields(Block, reason, original, suggested, state)
	return Decision{
		Action:      Block,
		Reason:      reason,
		Original:    original,
		Rewritten:   suggested,
		Detected:    state,
		Level:       levelFor(Block), // "block"
		AuditFields: fields,
	}
}

// levelFor maps a nudge Action to the repo's "allow|warn|block" Level string
// used by mergeDecisions / exitCodeFor (research Pattern 2, A1).
func levelFor(a Action) string {
	switch a {
	case Block:
		return "block"
	case Advise, Rewrite:
		return "warn"
	default:
		return "allow"
	}
}

// buildAuditFields populates the §9 AuditFields map.
// nudge_action uses the closed §9 enum (ActionString) — NOT "allow|warn|block".
// "decision" is NOT set here (mapped by Plan 06 adapter from Level).
func buildAuditFields(act Action, reason, original, rewritten string, state PMState) map[string]any {
	fields := map[string]any{
		"nudge_action":     ActionString(act),
		"reason_code":      reason,
		"original_command": original,
		"pm_state":         fmtPMState(state),
	}
	if rewritten != "" {
		fields["rewritten_command"] = rewritten
	}
	return fields
}

// fmtPMState produces a concise string representation of the PM state for the
// audit record's pm_state field (matches the §9 schema shape).
func fmtPMState(s PMState) string {
	return fmt.Sprintf(
		"pnpm=%s(hardened=%v) bun=%s(scanner=%v) node=%s",
		s.PnpmVersion, s.PnpmHardened,
		s.BunVersion, s.BunScannerOK,
		s.NodeVersion,
	)
}
