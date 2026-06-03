# Architecture Research: v1.2.0 Runtime Behavioral Hardening

**Domain:** Subsequent milestone integration — three features wired into an existing shipped Go safety harness
**Researched:** 2026-06-03
**Confidence:** HIGH (all findings derived from direct inspection of real source files)

---

## Existing Architecture Constraints (Fixed — Do Not Re-Research)

All findings below are derived from reading the actual source. These are not assumptions.

| Constraint | Source | Status |
|------------|--------|--------|
| `internal/policy` is a pure function library — no I/O, no goroutines, no globals | `internal/policy/types.go:1-14` | LOCKED |
| `EvaluatePath` exists, pure, tested in isolation | `internal/policy/path.go:76-107` | EXISTS, UNWIRED |
| `policyloader.ApplyPolicyOverlay` already processes `sensitive_path` rules from JSON policy files | `internal/policyloader/enforce.go:101-128` | EXISTS |
| `extractTargetPath` in enforce.go reads only `tc.ToolInput["path"]` — misses `file_path` and Bash targets | `internal/policyloader/enforce.go:279-287` | GAP (PLCY-05) |
| `corroborate()` counts only SIGNED sources toward block threshold; bumblebee entries have `Signed:false` | `internal/policy/corroboration.go:67-68,98-107` | GAP (PLCY-07) |
| One-shot process per `beekeeper check` invocation | `internal/check/handler.go:86-88` | FIXED |
| `runCheck` pipeline: decode stdin → open mmap → build MultiIndex → load policy files → `policy.Evaluate` → `ApplyPolicyOverlay` → `finalizeWithAC` | `internal/check/handler.go:101-272` | EXISTING PIPELINE |
| Existing audit `record_type` values: `policy_decision`, `tool_result`, `llmf_alert`, `sentry_alert` | `internal/audit/types.go:22-62` | SCHEMA ANCHOR |
| `FromDecision` maps a `policy.Decision` to `AuditRecord` with `RecordType:"policy_decision"` hardcoded | `internal/audit/types.go:98-150` | DOES NOT COVER nudge/version_drift |

---

## System Overview: v1.2.0 Integration Points

```
beekeeper check (one-shot process)
---------------------------------------------------------------------
stdin JSON
  |
  v
[decode hookInput]                       handler.go:153
  |
  v
[open mmap bbIdx]                        handler.go:177
  |
  v
[build MultiIndex]                       handler.go:194-211
  |
  v
[load policyFiles + derive thresholds]   handler.go:233-246
  |   thresholds now carry SeverityOverrides (PLCY-07)
  v
[policy.Evaluate]                        handler.go:251
  |   corroborate() applies severity escalation (PLCY-07)
  v
[extractPathTargets + EvaluatePath]      NEW -- PLCY-05
  |   extract file_path / path / Bash targets; tilde-expand in caller
  |   call EvaluatePath per path; merge most-restrictive-wins
  v
[nudge.ParseCommand + nudge.Evaluate]    NEW -- NUDGE (if Bash tool and PM install)
  |   PMState resolved by caller (no cache needed; one-shot process)
  |   write separate "nudge" audit record
  |   only Block action escalates main decision
  v
[ApplyPolicyOverlay]                     handler.go:267
  |   declarative sensitive_path overlay rules on top
  v
[finalizeWithAC -> writeAuditWithAC -> w.Write]
  |   record_type:"policy_decision" unchanged
  v
exit 0 (allow) / exit 1 (block)

beekeeper gateway (long-lived MCP proxy)
---------------------------------------------------------------------
  Same policy.Evaluate + EvaluatePath + nudge.Evaluate calls
  PM detection cache (60s TTL) belongs here -- process lives across requests
  nudgeCacheAdapter on gateway handler struct (not a package-level global)

beekeeper shim (npm shim --> beekeeper check as subprocess)
---------------------------------------------------------------------
  nudge.Evaluate called before proxying; one-shot like check; no cache needed
```

---

## PLCY-05: Wiring EvaluatePath into runCheck

### Insertion Point

Insert path evaluation **immediately after** `policy.Evaluate` returns (handler.go line ~251) and **before** `ApplyPolicyOverlay` (line 267). Rationale: `ApplyPolicyOverlay` already processes `sensitive_path` overlay rules from JSON policy files (enforce.go:101-128). The merged decision from `EvaluatePath` feeds into `ApplyPolicyOverlay`'s most-restrictive-wins logic correctly. Running path evaluation before the overlay means the overlay can still escalate but cannot silently downgrade a path-block without an explicit `allow` rule.

```go
// handler.go -- proposed insertion after line 251
decision := policy.Evaluate(toolCall, multiIdx, thresholds, ac)

// PLCY-05: sensitive-path evaluation
paths := extractPathTargets(toolCall)  // new helper -- see paths.go
for _, p := range paths {
    pathDecision := policy.EvaluatePath(p, sensitivePathConfig(cfg))
    decision = mergeDecisions(decision, pathDecision)
}

// NUDGE: package-manager nudge (see next section)
// ...

if len(policyFiles) > 0 {
    decision = policyloader.ApplyPolicyOverlay(policyFiles, toolCall, decision)
}
```

### Path Target Extraction: The Gap

`extractTargetPath` in `policyloader/enforce.go:279-287` reads only `tc.ToolInput["path"]`. Claude Code tools use different key names:

| Tool | Key in ToolInput | Shape |
|------|-----------------|-------|
| Read | `file_path` | string |
| Write | `file_path` | string |
| Edit | `file_path` | string |
| MultiEdit | `file_path` | string |
| Bash | `command` | string containing `cat ~/.aws/credentials` etc. |
| Legacy overlay path | `path` | string (kept for compat) |

The new `extractPathTargets(tc policy.ToolCall) []string` lives in `internal/check/paths.go` (separate file for clarity). It handles:

1. `tc.ToolInput["file_path"].(string)` — Read/Write/Edit/MultiEdit
2. `tc.ToolInput["path"].(string)` — legacy key used by policyloader overlay today; keep for compatibility
3. Bash command parsing: a conservative allowlist of read-command prefixes (`cat `, `type `, `Get-Content `, `head `, `tail `, `less `, `more `) scanning for recognizable credential-path tokens. Do NOT use regex over the full command string; use the same prefix-matching approach as `extractFromCommand` in engine.go.

**Purity constraint preserved:** `extractPathTargets` is I/O-free (reads from already-decoded `map[string]any`). `EvaluatePath` receives the resolved string. Tilde expansion (`~` → home directory) and OS separator normalization happen in the caller before `EvaluatePath` is called, per the existing `path.go:19-21` docs. This mirrors the established caller-resolves-I/O pattern.

**Multiple paths from one tool call:** `extractPathTargets` returns `[]string`. Call `EvaluatePath` once per path, then take the most-restrictive decision across all results.

### Merge Semantics

```go
// internal/check/handler.go (or paths.go)
func mergeDecisions(base, overlay policy.Decision) policy.Decision {
    levelRank := map[string]int{"allow": 0, "warn": 1, "block": 2}
    if levelRank[overlay.Level] > levelRank[base.Level] {
        return overlay
    }
    return base
}
```

This is identical to the existing policyloader merge logic (enforce.go:139-165). Do not invent a new merge strategy; make it consistent.

### Interaction with policyloader sensitive_path Overlay

`ApplyPolicyOverlay` already checks `sensitive_path` rules from JSON files against `extractTargetPath(tc)` (enforce.go:101-128). That function only reads `tc.ToolInput["path"]`, missing `file_path` and Bash targets. PLCY-05 also fixes `extractTargetPath` in `policyloader/enforce.go` to read `file_path`. After PLCY-05, the order of evaluation is:

1. `policy.EvaluatePath` with `DefaultSensitivePaths()` + config-loaded patterns → hardcoded engine block
2. `ApplyPolicyOverlay sensitive_path` rules from JSON files → declarative operator overlay

A JSON policy file `allow` rule for a sensitive path remains the only legitimate escape hatch. This is intentional and already documented in `docs/THREAT-MODEL.md §1`.

**Files modified for PLCY-05:**
- `internal/check/handler.go` — insert evaluation block
- `internal/check/paths.go` — NEW: `extractPathTargets`, tilde expansion, Bash target scanning
- `internal/policyloader/enforce.go` — fix `extractTargetPath` to also read `file_path`

---

## NUDGE: Package Architecture and the Caching Problem

### Package Boundary

```
internal/nudge/
├── nudge.go       # Evaluate(ParsedCommand, PMState, Config) Decision  PURE
├── detect.go      # DetectPMState() (PMState, error)                   IMPURE
├── parse.go       # ParseCommand(string) (ParsedCommand, bool)         PURE
├── rewrite.go     # Rewrite(ParsedCommand, PMState, Config) string     PURE
├── version.go     # semver comparison helpers                          PURE
├── reasons.go     # closed reason-code enum (constants)                PURE
└── *_test.go
```

`nudge.Evaluate` is pure: it takes an already-resolved `PMState` and returns a `Decision`. Detection I/O (`detect.go`) runs in the calling adapter in `internal/check/`, `internal/gateway/`, or `internal/shim/` — never inside `nudge.Evaluate`. This mirrors `policy.EvaluateReleaseAge(ReleaseAgeInput, cfg)` exactly: the caller resolves I/O, passes a pure input struct, gets a pure Decision.

### The 60-Second Detection Cache: Where It Belongs

The PRD's 60-second cache is meaningful ONLY in long-lived processes. `beekeeper check` is a **one-shot process** — it exits after one tool call evaluation. A 60-second in-memory cache is **dead code** in `beekeeper check` and would leak state between test cases if placed in the package.

| Consumer | Process lifetime | Cache needed? | Cache location |
|----------|-----------------|---------------|----------------|
| `beekeeper check` | one-shot, ~3ms | NO | N/A — detect once per invocation (~5ms for subprocess exec, acceptable) |
| `beekeeper gateway` | long-lived daemon | YES | `internal/gateway/nudge_cache.go` — `nudgeCacheAdapter` struct with `sync.Mutex` + `detectedAt time.Time` TTL |
| `beekeeper shim` (npm shim → beekeeper check subprocess) | one-shot | NO | N/A |

The `nudge` package's `detect.go` exposes `DetectPMState() (PMState, error)` without any cache or package-level state. Gateway wraps this in a `nudgeCacheAdapter`. One-shot callers call `detect.go` directly.

### nudgeCacheAdapter (gateway only)

```go
// internal/gateway/nudge_cache.go
type nudgeCacheAdapter struct {
    mu         sync.Mutex
    state      nudge.PMState
    detectedAt time.Time
    ttl        time.Duration // 60s default
}

func (a *nudgeCacheAdapter) State() nudge.PMState {
    a.mu.Lock()
    defer a.mu.Unlock()
    if time.Since(a.detectedAt) > a.ttl || a.detectedAt.IsZero() {
        a.state, _ = nudge.DetectPMState()
        a.detectedAt = time.Now()
    }
    return a.state
}
```

`nudgeCacheAdapter` is a field on the gateway handler struct, not a package-level global. This makes it injectable in tests (pass a mock implementing a `PMStateProvider` interface).

### Integration in runCheck (handler.go)

```go
// After EvaluatePath merge, before ApplyPolicyOverlay
if cfg.Nudge.Enabled {
    if cmd, ok := toolCall.ToolInput["command"].(string); ok {
        if parsedCmd, isInstall := nudge.ParseCommand(cmd); isInstall {
            pmState, _ := nudge.DetectPMState() // one-shot: no cache in check
            nudgeDecision := nudge.Evaluate(parsedCmd, pmState, cfg.NudgeConfig())
            writeNudgeAuditRecord(nudgeDecision, toolCall, auditPath, ac) // separate record
            if nudgeDecision.Action == nudge.Block {
                decision = policy.Decision{
                    Allow:   false,
                    Level:   "block",
                    Reason:  "nudge: " + nudgeDecision.Reason,
                    RuleIDs: []string{"nudge-block"},
                }
            }
            // Advise / Rewrite: informational only; main decision unchanged
        }
    }
}
```

The nudge decision does NOT merge into the main `policy.Decision` flow for Advise/Rewrite actions — those are written to a separate audit record. Only `Block` action escalates the main decision. This preserves the audit trail shape: one `policy_decision` record per tool call, plus zero or one `nudge` record if nudge evaluated.

---

## NUDGE: Audit Record Integration

### New Record Types in internal/audit/

Add two new struct types in a new file `internal/audit/nudge_types.go`. Do NOT modify the existing `AuditRecord` / `FromDecision` pair.

```go
// internal/audit/nudge_types.go

// NudgeRecord is emitted for every nudge evaluation (record_type:"nudge").
// It is a separate NDJSON record, not a replacement for the policy_decision record.
type NudgeRecord struct {
    RecordType      string       `json:"record_type"` // always "nudge"
    RecordID        string       `json:"record_id"`
    Timestamp       string       `json:"timestamp"`
    ScannerName     string       `json:"scanner_name"` // "beekeeper"
    AgentName       string       `json:"agent_name"`
    ToolName        string       `json:"tool_name"`
    OriginalCommand string       `json:"original_command"`
    Decision        string       `json:"decision"` // proceed|advise|rewrite|block
    ReasonCode      string       `json:"reason_code"`
    RewrittenCmd    string       `json:"rewritten_command,omitempty"`
    PMState         NudgePMState `json:"pm_state"`
}

// VersionDriftRecord is emitted by the weekly drift check (record_type:"version_drift").
type VersionDriftRecord struct {
    RecordType    string `json:"record_type"` // always "version_drift"
    RecordID      string `json:"record_id"`
    Timestamp     string `json:"timestamp"`
    ScannerName   string `json:"scanner_name"`
    PMName        string `json:"pm_name"`
    CurrentMajor  string `json:"current_major"`
    DetectedMajor string `json:"detected_major"`
    Severity      string `json:"severity"` // always "info"
}

// NudgePMState mirrors nudge.PMState for the audit schema.
type NudgePMState struct {
    NpmVersion   string `json:"npm_version,omitempty"`
    PnpmVersion  string `json:"pnpm_version,omitempty"`
    PnpmHardened bool   `json:"pnpm_hardened,omitempty"`
    BunVersion   string `json:"bun_version,omitempty"`
    BunScannerOK bool   `json:"bun_scanner_ok,omitempty"`
    NodeVersion  string `json:"node_version,omitempty"`
}
```

**Writer compatibility:** Check `internal/audit/writer.go`'s `Write` method signature. If it is typed to accept only `AuditRecord`, add a `WriteAny(w io.Writer, v any) error` helper or change `Write` to accept `any`. The NDJSON writer itself is `json.Marshal` + append — both `NudgeRecord` and `VersionDriftRecord` marshal cleanly.

**Schema compatibility:** `record_type` is already a string field in the NDJSON schema. The `beekeeper audit query` command (AUDT-02) already filters by `record_type`; new values are ignored by consumers reading for existing types. No migration needed. `beekeeper nudge audit` CLI (PRD §8) adds a dedicated query path for `record_type:"nudge"`.

---

## PLCY-07: Corroboration Hardening

### The Gap

`corroborate()` (corroboration.go:42-108) escalates based only on `signedCount` vs thresholds `{WarnAt:1, BlockAt:2, QuarantineAt:3}`. Bumblebee entries have `Signed:false` (line 67). Therefore bumblebee+OSV match where bumblebee is unsigned gives `signedCount:1` (only OSV is signed) → warn, not block, even for `severity:"critical"`.

Shai-Hulud worm case: bumblebee (`Signed:false`) + OSV (`Signed:true`) = signedCount 1 = warn. The gap is confirmed by reading the escalation table at corroboration.go:98-107.

### Minimal Safe Change

**Recommended: Per-severity escalation via `SeverityOverrides`.** Add to `CorroborationThresholds`:

```go
// internal/policy/types.go additions

type CorroborationThresholds struct {
    WarnAt      int
    BlockAt     int
    QuarantineAt int
    // PLCY-07: per-severity overrides. When ANY match has Severity in this map,
    // use the override's thresholds instead of the global WarnAt/BlockAt/QuarantineAt
    // for that evaluation.
    // Sanity bound: override BlockAt must be >= 1 (zero would block unconditionally).
    // Sanity bound: override BlockAt must be <= global BlockAt (cannot be looser).
    SeverityOverrides map[string]SeverityThreshold
}

type SeverityThreshold struct {
    BlockAt      int // minimum signed-source count for block at this severity
    QuarantineAt int // minimum signed-source count for quarantine at this severity
}
```

Default PLCY-07 config: `SeverityOverrides["critical"] = SeverityThreshold{BlockAt:1, QuarantineAt:2}`.

With this default, a single OSV signed match on a critical-severity package → block. Non-critical packages still require 2 signed sources. The configuration is auditable via the policy file system.

**Alternative considered (rejected): Treat bundled bumblebee as signed-equivalent** by setting `Signed:true` in the catalog adapter. Rejected because: (1) bumblebee catalog entries genuinely do not carry cryptographic signatures; changing the `Signed` field would mislead consumers and audit logs; (2) it removes the distinction between "catalog has a cryptographic signature" and "catalog is bundled with the beekeeper binary," which matters for the corroboration trust model.

### Sanity Bounds Locus

**The sanity-bounds locus is `internal/policy/corroboration.go:validateCorroborationThresholds`.** Extend it to:

1. Reject `SeverityOverrides[s].BlockAt < 1` — a zero threshold blocks every tool call for any package in the ecosystem (poisonable by a malicious catalog entry setting severity=critical universally).
2. Reject `SeverityOverrides[s].BlockAt > globalBlockAt` — an override looser than the global threshold weakens security for a more-dangerous severity class (misconfiguration).
3. Keep existing: `WarnAt <= BlockAt <= QuarantineAt`.
4. Keep existing: misconfigured thresholds → fail closed (return `"block"`) as today.

`validateCorroborationThresholds` is called at the START of every `corroborate()` call (line 43). The single locus prevents divergence across the three consumers (check, gateway, Sentry correlation). Do not add bounds checking in policyloader or elsewhere.

### corroborate() Change

```go
// internal/policy/corroboration.go -- inside corroborate(), after building signedCount

// PLCY-07: check if any matched entry has a severity that triggers an override.
maxSeverityOverride := findSeverityOverride(matches, t.SeverityOverrides)
effectiveBlockAt := t.BlockAt
effectiveQuarantineAt := t.QuarantineAt
if maxSeverityOverride != nil {
    effectiveBlockAt = maxSeverityOverride.BlockAt
    effectiveQuarantineAt = maxSeverityOverride.QuarantineAt
}

switch {
case signedCount >= effectiveQuarantineAt && hasSignedSource:
    return "block", true, signedCount, agreedList, dissentList
case signedCount >= effectiveBlockAt && hasSignedSource:
    return "block", false, signedCount, agreedList, dissentList
case signedCount >= t.WarnAt || hasUnsigned:
    return "warn", false, signedCount, agreedList, dissentList
default:
    return "allow", false, signedCount, agreedList, dissentList
}
```

`findSeverityOverride` is a pure helper that scans `matches` for severity values present in `t.SeverityOverrides` and returns the most-restrictive (lowest `BlockAt`) override found. Pure: only reads from the passed-in slices and map.

**Files modified for PLCY-07:**
- `internal/policy/types.go` — add `SeverityThreshold`, extend `CorroborationThresholds`
- `internal/policy/corroboration.go` — extend `validateCorroborationThresholds`, add `findSeverityOverride`, extend `corroborate()` escalation table
- `internal/policyloader/loader.go` — add `severity_overrides` field to `PolicyRule`
- `internal/policyloader/validate.go` — validate `severity_overrides` entries (BlockAt >= 1, <= globalBlockAt)

---

## Component Inventory: New vs Modified

### NEW Files

| File | Type | Responsibility |
|------|------|----------------|
| `internal/nudge/nudge.go` | NEW | Pure `Evaluate(ParsedCommand, PMState, Config) Decision` |
| `internal/nudge/detect.go` | NEW | Impure `DetectPMState() (PMState, error)` — exec subprocess with 2s timeout |
| `internal/nudge/parse.go` | NEW | Pure `ParseCommand(string) (ParsedCommand, bool)` — npm/npx/pnpm/bun/yarn install detection |
| `internal/nudge/rewrite.go` | NEW | Pure `Rewrite(ParsedCommand, PMState, Config) string` — hard-mode command rewriting |
| `internal/nudge/version.go` | NEW | Pure semver comparison helpers (pnpm >= 11.0, bun >= 1.3, node >= 22) |
| `internal/nudge/reasons.go` | NEW | Closed reason-code enum constants (PRD §6) |
| `internal/nudge/*_test.go` | NEW | Table-driven pure tests for all 17 PRD §10 acceptance criteria |
| `internal/check/paths.go` | NEW | `extractPathTargets(ToolCall) []string`, tilde expansion, Bash credential-read detection |
| `internal/audit/nudge_types.go` | NEW | `NudgeRecord`, `VersionDriftRecord`, `NudgePMState` structs |
| `internal/gateway/nudge_cache.go` | NEW | `nudgeCacheAdapter` with 60s TTL for gateway's long-lived use |
| `cmd/beekeeper/nudge.go` | NEW | `beekeeper nudge status|check|audit` CLI surface (PRD §8) |

### MODIFIED Files

| File | Change | Feature |
|------|--------|---------|
| `internal/policy/types.go` | Add `SeverityThreshold` type; add `SeverityOverrides map[string]SeverityThreshold` to `CorroborationThresholds` | PLCY-07 |
| `internal/policy/corroboration.go` | Extend `validateCorroborationThresholds` sanity bounds; add `findSeverityOverride`; extend `corroborate()` escalation table | PLCY-07 |
| `internal/check/handler.go` | Insert `extractPathTargets` + `EvaluatePath` block; insert `nudge.ParseCommand` + `nudge.Evaluate` + `writeNudgeAuditRecord` block | PLCY-05, NUDGE |
| `internal/policyloader/enforce.go` | Fix `extractTargetPath` to also read `file_path` key | PLCY-05 compat |
| `internal/policyloader/loader.go` | Add `severity_overrides` field to `PolicyRule`; extend `ThresholdsFromPolicyFiles` | PLCY-07 |
| `internal/policyloader/validate.go` | Validate `severity_overrides` entries (BlockAt bounds) | PLCY-07 |
| `internal/gateway/` (handler) | Wire same `EvaluatePath` + nudge evaluation; add `nudgeCacheAdapter` field; write nudge audit records | PLCY-05, NUDGE |
| `internal/shim/shim.go` (or npm shim template) | Wire nudge evaluation in npm shim path before proxy | NUDGE |
| `internal/config/` | Add `Nudge Config` block matching PRD §5 JSON shape | NUDGE |
| `internal/audit/writer.go` | Verify `Write` accepts `any`; add `WriteAny` helper if needed | NUDGE |

---

## Data Flow: runCheck Pipeline After v1.2.0

```
stdin JSON
  |
  v
decode hookInput (handler.go:153)
  |
  v
open mmap bbIdx (handler.go:177)
  |
  v
build MultiIndex (handler.go:194-211)
  |
  v
load policyFiles + derive thresholds (handler.go:233-246)
  |   thresholds carry SeverityOverrides[critical]={BlockAt:1,QuarantineAt:2} by default
  v
policy.Evaluate(toolCall, multiIdx, thresholds, ac) (handler.go:~251)
  |   corroborate() reads CatalogMatch.Severity, applies severity override (PLCY-07)
  |   critical + 1 signed source --> block (was: warn with bumblebee unsigned)
  v
extractPathTargets(toolCall) --> []resolvedPath         NEW -- PLCY-05
  |   reads file_path, path, parses Bash command
  |   tilde-expand + OS separator normalize in caller
  v
for each path: policy.EvaluatePath(path, DefaultSensitivePaths+cfg) --> pathDecision
  |
  v
decision = mergeDecisions(decision, pathDecision)
  |   block > warn > allow; path block beats catalog allow
  v
if nudge.Enabled AND ParseCommand(tc.ToolInput["command"]) matches:   NEW -- NUDGE
  pmState := nudge.DetectPMState()   one-shot; no cache (check is ephemeral)
  nudgeDecision := nudge.Evaluate(parsedCmd, pmState, cfg.NudgeConfig())
  writeNudgeAuditRecord(nudgeDecision, ...)   separate record_type:"nudge"
  if nudgeDecision.Action == Block:
    decision = blockDecision("nudge-block")
  [Advise/Rewrite: informational only -- do not merge into main decision]
  |
  v
ApplyPolicyOverlay(policyFiles, toolCall, decision) (handler.go:267)
  |   sensitive_path overlay rules from JSON files; most-restrictive-wins
  v
finalizeWithAC --> writeAuditWithAC --> w.Write(AuditRecord{RecordType:"policy_decision"})
  |   + separate w.Write(NudgeRecord{RecordType:"nudge"}) written earlier if applicable
  v
exit 0 / exit 1
```

---

## Architectural Patterns Applied

### Pattern: Caller-Resolved I/O, Pure Decision (existing — extended to nudge)

**What:** All I/O (network, subprocess exec, file reads) runs in the caller before the pure decision function is called. The decision function receives only plain value structs.

**Existing implementations:**
- `policy.EvaluateReleaseAge(ReleaseAgeInput, cfg)` — caller provides `AgeMinutes int64` (HTTP-resolved); `release_age.go:61-109`
- `policy.Evaluate(ToolCall, MultiCatalogLookup, ...)` — catalog lookup resolved by adapters before call; `engine.go:64`
- `policy.EvaluatePath(resolvedPath, cfg)` — caller normalizes tilde and OS separators; `path.go:76`

**Applied to nudge:** `nudge.Evaluate(ParsedCommand, PMState, Config)` — caller calls `nudge.DetectPMState()` first. The 60-second cache belongs in the caller adapter (gateway), not in `nudge.Evaluate` or `nudge.DetectPMState`.

### Pattern: Most-Restrictive-Wins Merge

**What:** When multiple evaluators produce decisions, the merge takes the decision with the highest level rank (block > warn > allow).

**Existing:** `ApplyPolicyOverlay` implements this internally (enforce.go:139-165).

**Extension:** A new `mergeDecisions` helper in `internal/check/paths.go` applies the same rule for the path-evaluation merge step before the overlay runs.

### Pattern: Separate Audit Record per Cross-Cutting Concern

**What:** Different concern types emit different `record_type` values. Consumers filter on `record_type`. Each concern owns its own struct, written by the wiring layer (not by the pure decision function).

**Existing:** `policy_decision`, `tool_result`, `llmf_alert`, `sentry_alert` are separate structs, separately written.

**Extension:** `nudge` and `version_drift` records follow the same pattern. `nudge.Evaluate` returns a `Decision` with `AuditFields map[string]any`; the handler maps this to `NudgeRecord` and calls `w.Write(nudgeRec)` separately from the main `policy_decision` write.

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: Cache in the Nudge Package

**What people do:** Put a `sync.Map` cache with a `time.Time` TTL inside `nudge.DetectPMState` or `nudge.Evaluate`.

**Why it's wrong:** Violates the purity constraint — package-level mutable state is a global side effect. In the one-shot `beekeeper check` process the cache is dead code (the process dies before it would ever be reused). In tests it leaks state between test cases.

**Do this instead:** Cache in the long-lived caller (gateway `nudgeCacheAdapter`). One-shot callers (check, shim) detect once per invocation — one subprocess exec taking ~5ms, acceptable at their call frequency.

### Anti-Pattern 2: Merging Nudge Decision into policy.Decision for Advise/Rewrite

**What people do:** Return a `policy.Decision` from nudge with `Level:"warn"` for Advise and merge it into the main decision, so the main audit record reflects "warn" for an npm install nudge.

**Why it's wrong:** Pollutes the `policy_decision` audit record semantics. Consumers correlating `sources_agreed`, `corroboration_count`, and `catalog_matches` fields receive nonsensical values for a nudge-driven warn. The `decision` field becomes ambiguous between "threat-intel decision" and "package-manager preference."

**Do this instead:** Write a separate `record_type:"nudge"` record. Only a nudge `Block` action (no hardened PM + `requireHardened:true`) escalates the main `policy.Decision` to block.

### Anti-Pattern 3: I/O in corroborate()

**What people do:** To handle PLCY-07, fetch catalog metadata (severity from a registry API) inside `corroborate()`.

**Why it's wrong:** `corroborate()` is called from the pure `policy.Evaluate` path. Any I/O there breaks the purity constraint and makes the function untestable in isolation.

**Do this instead:** Severity is already present on each `CatalogMatch` struct (`Severity string` field, `types.go:43`). `findSeverityOverride` reads `m.Severity` from the already-resolved matches. No additional I/O needed.

### Anti-Pattern 4: SeverityOverride BlockAt = 0

**What people do:** Set `SeverityOverrides["critical"].BlockAt = 0` to "block everything critical regardless of catalog hits."

**Why it's wrong:** `BlockAt:0` means zero signed sources triggers block. Since zero signed sources is the state before any catalog lookup occurs, this blocks every tool call regardless of whether any catalog match occurred.

**Do this instead:** Minimum valid override is `BlockAt:1`. `validateCorroborationThresholds` enforces this bound and fails closed when violated.

### Anti-Pattern 5: Duplicating extractFromCommand / installPrefixes

**What people do:** Add a third copy of the `installPrefixes` table to `internal/nudge/parse.go` because it cannot import `internal/policy` (circular) and `policyloader` already has a duplicate (`installPrefixesOverlay` in enforce.go:240-253).

**Why it's wrong:** Three copies of the same table will diverge (they already diverge — policyloader's copy does not include pnpm/bun/yarn patterns needed by nudge).

**Do this instead:** Extract a shared `internal/pkgparse/` package containing the install-command prefix table and `ParseInstallCommand`. Both `internal/policy/engine.go` (via import), `internal/policyloader/enforce.go`, and `internal/nudge/parse.go` can import it. This is a small, pure package with no I/O. Alternatively, if the policy team decides the duplication is acceptable for scope reasons, at minimum add a comment to each copy pointing to the others.

---

## Build Order (Dependency-Ordered)

### Phase 1: Pure Policy Changes (no I/O, no wiring, testable in isolation)

1. **PLCY-07a:** `internal/policy/types.go` — add `SeverityThreshold`, extend `CorroborationThresholds`. No behavior change; existing tests still pass.
2. **PLCY-07b:** `internal/policy/corroboration.go` — extend `validateCorroborationThresholds`, add `findSeverityOverride`, extend `corroborate()` escalation table.
3. **PLCY-07c BTEST:** Pure-policy table tests for corroboration — cover: critical single-signed-source blocks; override looser-than-global rejected (fail-closed); override BlockAt=0 rejected; non-critical still requires 2 signed sources; Shai-Hulud worm fixture (bumblebee unsigned + OSV signed + severity=critical) → block.
4. **NUDGE-pure:** `internal/nudge/` pure files first (`nudge.go`, `parse.go`, `rewrite.go`, `version.go`, `reasons.go`). Table-driven tests for all PRD §10 criteria that do not require I/O (criteria 1-10, 14, 15, 16, 17).

### Phase 2: I/O Adapters

5. **NUDGE-detect:** `internal/nudge/detect.go` + `detect_test.go` — criteria 11 (cache absence in detect), 12 (2s timeout graceful), 13 (bunfig.toml parse failure safe fallback).
6. **PLCY-05-extract:** `internal/check/paths.go` — `extractPathTargets`, tilde expansion, Bash credential-read pattern matching. Unit tests with table of tool shapes (Read `file_path`, Write `file_path`, Bash `cat ~/.aws/credentials`).
7. **NUDGE-audit:** `internal/audit/nudge_types.go` — `NudgeRecord`, `VersionDriftRecord`, `NudgePMState`. Unit tests for JSON marshaling shape.

### Phase 3: Wiring (handler.go, gateway, shim, policyloader)

8. **PLCY-05-wire:** Wire `extractPathTargets` + `policy.EvaluatePath` into `internal/check/handler.go`. Fix `extractTargetPath` in `policyloader/enforce.go` to read `file_path`. Integration test: check-handler stdin fixture `{"tool_name":"Read","tool_input":{"file_path":"~/.aws/credentials"}}` → block decision, `rule_ids:["sensitive-path-policy"]`.
9. **PLCY-07-policyloader:** Extend `policyloader/loader.go` + `validate.go` for `severity_overrides` in policy files. Tests for `ThresholdsFromPolicyFiles` with severity override entries.
10. **NUDGE-wire-check:** Wire `nudge.ParseCommand` + `nudge.Evaluate` + `writeNudgeAuditRecord` into `handler.go`. Check-handler integration test: Bash `npm install foo` → nudge `record_type:"nudge"` audit record written + `policy_decision` allow; when `requireHardened:true` + no PM installed → `policy_decision` block.
11. **NUDGE-wire-gateway:** Add `nudgeCacheAdapter` to gateway. Wire nudge evaluation in gateway handler. Integration test for 60s TTL cache (inject a clock interface or `time.Now` override).
12. **NUDGE-wire-shim:** Wire nudge evaluation in npm shim path before proxy.

### Phase 4: CLI and E2E

13. **NUDGE-cli:** `cmd/beekeeper/nudge.go` — `nudge status|check|audit` subcommands.
14. **BTEST-e2e:** Live-binary E2E battery:
    - `echo '{"tool_name":"Read","tool_input":{"file_path":"~/.aws/credentials"}}' | beekeeper check` → exit 1 (PLCY-05)
    - `echo '{"tool_name":"Bash","tool_input":{"command":"npm install ai-figure"}}' | beekeeper check` (OSV signed + bumblebee unsigned + severity=critical) → exit 1 (PLCY-07)
    - `echo '{"tool_name":"Bash","tool_input":{"command":"npm install chalk"}}' | beekeeper check` (no catalog match) → exit 0, nudge audit record written if pnpm/bun installed
    - `beekeeper nudge check "npm install chalk"` → human-readable dry-run output

---

## Integration Points Summary

| Boundary | Communication | Notes |
|----------|---------------|-------|
| `internal/check` -> `internal/nudge` | Direct Go call: `ParseCommand`, `DetectPMState`, `Evaluate` | No cache in check; one-shot |
| `internal/gateway` -> `internal/nudge` | Same, via `nudgeCacheAdapter.State()` wrapper | 60s TTL cache in gateway handler struct |
| `internal/shim` -> `internal/nudge` | Direct Go call | One-shot (shim invoked per command) |
| `internal/check` -> `internal/policy` (EvaluatePath) | Direct Go call with caller-resolved path | PLCY-05 |
| `corroborate()` -> severity data | Reads `CatalogMatch.Severity` already in memory | No new I/O (PLCY-07) |
| `internal/audit.Writer` -> `NudgeRecord` | `w.Write(nudgeRec)` | Verify writer accepts `any` before building |
| `policyloader.ThresholdsFromPolicyFiles` -> `CorroborationThresholds` | Returns extended struct with `SeverityOverrides` | PLCY-07 policyloader change |

---

## Sources

All findings derived from direct inspection of real source files at commit state 2026-06-03:

- `internal/policy/engine.go` — `extract()`, `installPrefixes`, `Evaluate()` (lines 64-176)
- `internal/policy/path.go` — `EvaluatePath()`, `DefaultSensitivePaths()`, caller-resolves-path docs (lines 29-168)
- `internal/policy/corroboration.go` — `corroborate()`, `validateCorroborationThresholds()`, escalation table (lines 32-108)
- `internal/policy/types.go` — `ToolCall`, `Decision`, `CorroborationThresholds`, `AgentContext` (lines 17-117)
- `internal/policy/release_age.go` — `ReleaseAgeInput`, `EvaluateReleaseAge()`, the pure-adapter pattern to mirror (lines 1-110)
- `internal/check/handler.go` — `runCheck()` pipeline with exact insertion-point line references (lines 101-272)
- `internal/policyloader/enforce.go` — `ApplyPolicyOverlay()`, `extractTargetPath()` gap, overlay merge logic (lines 39-394)
- `internal/policyloader/loader.go` — `PolicyFile`, `PolicyRule`, `LoadPolicyDir()` (lines 26-139)
- `internal/audit/types.go` — `AuditRecord`, `FromDecision()`, existing record types (lines 22-150)
- `internal/shim/shim.go` — `DefaultTools`, `Install()`, one-shot architecture (lines 1-80)
- `.planning/specs/NUDGE-PRD.md` — full nudge spec, §3 architecture, §3.3 integration points, §4 decision flow
- `.planning/PROJECT.md` — milestone context, architecture constraints
- `CLAUDE.md` — Architecture Constraints, Self-Defense Non-Negotiables, corroboration sanity bounds requirement

---

*Architecture research for: Beekeeper v1.2.0 Runtime Behavioral Hardening — PLCY-05 + NUDGE + PLCY-07*
*Researched: 2026-06-03*
