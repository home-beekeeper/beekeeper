---
phase: 08-package-manager-nudge-behavioral-test-suite
reviewed: 2026-06-04T00:00:00Z
depth: standard
files_reviewed: 47
files_reviewed_list:
  - cmd/beekeeper/config.go
  - cmd/beekeeper/config_test.go
  - cmd/beekeeper/main.go
  - cmd/beekeeper/nudge.go
  - cmd/beekeeper/nudge_test.go
  - internal/audit/types.go
  - internal/audit/types_test.go
  - internal/check/e2e_test.go
  - internal/check/handler.go
  - internal/check/handler_test.go
  - internal/check/integration_test.go
  - internal/check/nudge_adapter.go
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/gateway/drift.go
  - internal/gateway/drift_test.go
  - internal/gateway/gateway.go
  - internal/gateway/gateway_test.go
  - internal/gateway/policy.go
  - internal/gateway/proxy.go
  - internal/nudge/config.go
  - internal/nudge/config_test.go
  - internal/nudge/detect.go
  - internal/nudge/detect_test.go
  - internal/nudge/evaluate.go
  - internal/nudge/evaluate_test.go
  - internal/nudge/reasons.go
  - internal/nudge/reasons_test.go
  - internal/nudge/rewrite.go
  - internal/nudge/rewrite_test.go
  - internal/nudge/scanners.go
  - internal/nudge/scanners_fuzz_test.go
  - internal/nudge/scanners_test.go
  - internal/nudge/version.go
  - internal/nudge/version_test.go
  - internal/pkgparse/fuzz_test.go
  - internal/pkgparse/pkgparse.go
  - internal/pkgparse/pkgparse_test.go
  - internal/platform/dirs.go
  - internal/platform/dirs_test.go
  - internal/policy/engine.go
  - internal/policy/engine_test.go
  - internal/policyloader/enforce.go
  - internal/shim/shim.go
  - internal/shim/shim_test.go
findings:
  critical: 0
  warning: 4
  info: 5
  total: 9
status: issues_found
---

# Phase 8: Code Review Report

**Reviewed:** 2026-06-04T00:00:00Z
**Depth:** standard
**Files Reviewed:** 47
**Status:** issues_found

## Summary

Phase 8 (Package-Manager Nudge + Behavioral Test Suite) was reviewed against
the locked CLAUDE.md / project-context invariants and for correctness/security
defects. The architecture invariants are largely upheld and well-defended:

- **Purity holds.** `internal/policy`, `internal/pkgparse`, and `nudge.Evaluate`
  (evaluate.go / version.go / rewrite.go / reasons.go / config.go) import only
  pure packages; all detection I/O is confined to `detect.go` + `scanners.go`.
- **Fail-open nudge exception is correct.** `DetectState` treats every
  exec/timeout/read error as "PM not installed" and proceeds. The 2s
  `exec.CommandContext` timeout is real (per-call `context.WithTimeout`), argv is
  fixed (`"pnpm"/"bun"/"node", "--version"`) with no attacker-controlled path or
  shell, and a detection failure never blocks or panics.
- **Merge ordering is correct.** In both `check/handler.go` and `gateway/proxy.go`
  the nudge merge runs AFTER `ApplyPolicyOverlay` and after the SPATH block via
  most-restrictive-wins (`mergeDecisions`/`mergeGatewayDecisions`), so a nudge
  advisory can never downgrade a catalog/path/overlay block.
- **Cache placement is correct.** The 60s `nudge.Cache` is constructed only in
  `newGatewayHandler`; the one-shot `check` hook and the `shim` call
  `nudge.DetectStateFn` fresh (Flag 2).
- **sudo is parsed but never rewritten** (`Evaluate` returns `Advise/sudo-passthrough`
  before any rewrite branch). **`ValidateNudgeConfig` is fail-closed** and the
  `config set` path validates the candidate before any write.
- **Parsers do not panic** on malformed/adversarial input; the fuzz targets exist
  and the parsers themselves are bounded (no unbounded loops, no ReDoS — they use
  `strings` primitives, not regex).

The findings below are quality/robustness issues. No BLOCKER-class correctness or
security defect was proven. The most material item is WR-01 (sensitive data in
nudge audit records bypasses redaction).

## Warnings

### WR-01: Nudge audit records persist the raw command unredacted (forensic-log credential leak)

**File:** `internal/check/nudge_adapter.go:150-163`, `internal/gateway/proxy.go:535-548`, `internal/gateway/policy.go:173-191`
**Issue:** The nudge audit record sets `OriginalCommand: d.Original` (= `cmd.Raw`,
the verbatim agent-supplied Bash command) and `RewrittenCommand`. Unlike the main
audit path — `check/handler.go:452-453` and `gateway/proxy.go:514-515` both call
`audit.RedactRecord(rec, patterns)` — `writeNudgeAuditRecord` and
`writeNudgeAudit` write the record with NO redaction pass. A command such as
`npm install --registry=https://x:Bearer abcdef...@host/` (or any install command
carrying a token/secret in an argument) is written verbatim to the NDJSON audit
log. Two compounding factors:
1. The nudge write paths skip `RedactRecord` entirely.
2. Even if they called it, `audit.RedactRecord` (redact.go:106-115) only redacts
   the `Reason` field — it never touches `OriginalCommand`, `RewrittenCommand`,
   `ReasonCode`, or `PMState`. So the new Phase-8 field carrying attacker-influenced
   raw input has no redaction coverage on any path.

Severity is WARNING rather than BLOCKER because the leaked data is confined to the
owner-only (0600) local audit log, but a forensic log is a known exfil target and
the project already treats credential redaction as a security control.
**Fix:** Run nudge records through redaction before writing, and extend
`RedactRecord` to cover the command fields:
```go
// in writeNudgeAuditRecord / writeNudgeAudit, before w.Write(rec):
rec = audit.RedactRecord(rec, audit.DefaultRedactPatterns())

// in audit/redact.go RedactRecord:
out.Reason = applyRedaction(rec.Reason, patterns)
out.OriginalCommand = applyRedaction(rec.OriginalCommand, patterns)
out.RewrittenCommand = applyRedaction(rec.RewrittenCommand, patterns)
out.PMState = applyRedaction(rec.PMState, patterns)
```

### WR-02: `parseInt` silently overflows on large `minimumReleaseAge` values

**File:** `internal/nudge/scanners.go:260-285`
**Issue:** `parseInt` accumulates with `n = n*10 + int(c-'0')` and has no overflow
guard. A `pnpm-workspace.yaml` containing
`minimumReleaseAge: 99999999999999999999999999999999999` (a seed already present in
`scanners_fuzz_test.go:101`) overflows a 64-bit `int` and wraps to an arbitrary —
possibly negative — value. The result is fed into the weakness comparison
`minAge < minimumReleaseAgeWeaknessBaseline` (scanners.go:233), so a wrapped value
can silently flip `WeaknessLogged`. The fuzz target only asserts no-panic, so this
wrong-result path is not caught. Impact is bounded (hardening stays true regardless,
and the field is advisory), but the parser produces a confidently-wrong integer on
attacker-controlled input.
**Fix:** Bound the accumulation and treat overflow as a parse error (which already
maps to `ok=false` → safe default):
```go
const maxAge = 1 << 31 // generous ceiling; minutes never legitimately exceed this
for i := start; i < len(s); i++ {
    c := s[i]
    if c < '0' || c > '9' {
        return 0, errNotInt
    }
    n = n*10 + int(c-'0')
    if n > maxAge {
        return 0, errNotInt // overflow / absurd value → parse trouble
    }
}
```

### WR-03: `detect.go` ignores the caller's outer deadline — each PM exec can run a full 2s

**File:** `internal/nudge/detect.go:56-84, 98-141`
**Issue:** Each version fn does `context.WithTimeout(ctx, detectionTimeout)` (2s),
and `DetectState` runs pnpm, then bun, then node sequentially. In the one-shot
`check` hook the detection is invoked with the request `ctx` that already carries
the 5s execTimeout (handler.go:141). If pnpm and bun both hang to their individual
2s deadlines, detection alone consumes ~4s of the 5s budget before node even starts,
and the post-evaluation `ctx.Err()` check (handler.go:313) then fails the whole
check closed — turning a slow/hung package manager into a hard block of an unrelated
tool call. Because each `WithTimeout(ctx, 2s)` derives from `ctx` it will be capped
by the outer deadline, but the sequential 2s-per-binary worst case is still large
relative to the 5s hook budget and the <100ms gateway target.
**Fix:** Run the three version detections concurrently (they are independent) and/or
budget detection against the remaining outer deadline rather than a fixed per-call
2s, so total detection latency is ~2s not ~6s and cannot consume the hook budget:
```go
// resolve pnpm/bun/node version fns in parallel under a single shared
// min(detectionTimeout, deadline(ctx)) budget
```

### WR-04: Drift scheduler spawns an unbounded goroutine per tick (slow-fetch pile-up)

**File:** `internal/gateway/drift.go:162-178`
**Issue:** On every ticker tick the scheduler launches a *new* goroutine
(`go func() { ... h.checkDrift(driftCtx) }()`) "so a slow fetch doesn't block the
ticker." With the default 168h interval this is harmless, but the interval is
operator-configurable (`nudge.major_drift_check.interval`) down to any positive
duration. If `checkDrift` (which performs network/CLI metadata fetches) routinely
takes longer than the configured interval, goroutines accumulate without bound — a
self-inflicted resource leak driven by config. There is no in-flight guard.
**Fix:** Use a single worker with an in-flight flag (or a buffered channel of size 1)
so a tick is dropped while a previous check is still running:
```go
var running atomic.Bool
case <-ticker.C:
    if running.CompareAndSwap(false, true) {
        go func() {
            defer running.Store(false)
            driftCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
            defer cancel()
            h.checkDrift(driftCtx)
        }()
    }
```

## Info

### IN-01: Dead branch in `Evaluate` preferred-PM ordering

**File:** `internal/nudge/evaluate.go:156-165`
**Issue:** The trailing `if pnpmReady { return evaluatePnpm(...) }` at lines 162-164
is unreachable. The only state that could reach it is `pnpmReady && preferBun &&
!bunReady`, but that exact state is already matched by the second disjunct of the
guard at line 156 (`pnpmReady && preferBun && !bunReady`), which returns first.
The branch is dead code that obscures the (already convoluted) selection logic.
**Fix:** Remove lines 162-165, or rewrite the selection as an explicit
two-candidate ordered list (`preferred` first, then the other) for readability.

### IN-02: `meetsFloor` doc comment contradicts the implementation

**File:** `internal/nudge/version.go:13-20, 36-44`
**Issue:** The comment says "Patch versions are compared only as a tiebreaker when
major and minor are equal," implying patch is special-cased. The implementation
just compares all three components in a uniform loop — which is the correct and
simpler behavior, but the comment describes a different algorithm. Misleading docs
on a security-relevant comparison.
**Fix:** Reword to "Compares major, then minor, then patch components in order."

### IN-03: `evaluateBun` hard-mode rewrite reuses `ReasonPnpmHardRewrite`

**File:** `internal/nudge/evaluate.go:202-208`
**Issue:** A bun hard-mode rewrite emits `reason_code:"pnpm-hard-rewrite"` even
though the rewritten command is `bun add ...`. The forensic `reason_code` is
therefore inaccurate for the bun branch. The code comments acknowledge this as a
known gap (no `bun-hard-rewrite` enum value). Audit consumers filtering on
reason_code will mis-attribute bun rewrites to pnpm.
**Fix:** Add a `ReasonBunHardRewrite = "bun-hard-rewrite"` constant to reasons.go
(and validReasons) and use it in `evaluateBun`.

### IN-04: Per-invocation file scans on the check hot path (acceptable but worth documenting)

**File:** `internal/nudge/detect.go:102-131`
**Issue:** On every `beekeeper check` install-command invocation, `DetectState`
reads `pnpm-workspace.yaml` (when pnpm meets floor) and `bunfig.toml`
(when `CheckSocketScanner`) from disk. This is consistent with the documented
"fresh detection, no cache" contract for the one-shot hook, and the reads are
bounded to fixed paths, so it is not a violation. Flagged only so the
"no hot-path file I/O" wording in the Flag-2 contract is not misread as prohibiting
these detection-layer reads.
**Fix:** None required; consider a one-line clarification in the detect.go header
that the file scans are the intended detection I/O (distinct from catalog/cache I/O).

### IN-05: `config set` cannot reach `version_floors` / `drift` keys, so CLI-set configs are only partially validatable

**File:** `cmd/beekeeper/config.go:142-170`
**Issue:** `applyNudgeKey` supports only `enabled`, `mode`, `require_hardened`,
`preferred`, and `check_socket_scanner`. Version floors and the drift interval —
both validated by `ValidateNudgeConfig` and both with real fail-closed reject paths
— can only be changed by hand-editing `config.json`. This is a documented scope
choice (the help text lists exactly these five keys), not a bug, but it leaves the
floor/interval reject paths exercisable only via `Load`, never via the CLI surface
operators are pointed at.
**Fix:** Optional — extend `applyNudgeKey` with `nudge.version_floors.{pnpm,bun,node}`
and `nudge.major_drift_check.interval`, relying on the existing
`ValidateNudgeConfig` to reject malformed values fail-closed before write.

---

_Reviewed: 2026-06-04T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
