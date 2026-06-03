# Phase 6: Corroboration Severity Hardening — Research

**Researched:** 2026-06-03
**Domain:** `internal/policy` pure function library — corroboration escalation + sanity gate
**Confidence:** HIGH (all findings from direct source inspection)

---

## Summary

Phase 6 is a focused, pure-logic change to `internal/policy`. No new packages, no new
dependencies, no wiring changes. The entirety of CORR-01 and CORR-02 lives in four files
(`internal/policy/types.go`, `internal/policy/corroboration.go`,
`internal/policyloader/loader.go`, `internal/policyloader/validate.go`) plus the
accompanying tests.

The gap (finding F1): `corroborate()` counts only **signed** sources toward the block
threshold. The bumblebee mmap adapter sets `Signed: e.CatalogSignature != ""`, and
bumblebee catalog entries carry `CatalogSignature: ""` (unsigned). OSV entries carry
`CatalogSignature: "osv-api"` (signed). For `ai-figure` (Shai-Hulud), the match is
bumblebee (unsigned) + OSV (signed) = `signedCount: 1` = warn. CORR-01 fixes this by
adding per-severity threshold overrides: `SeverityOverrides["critical"] = {BlockAt:1}`
lowers the block threshold to 1 signed source when ANY matched entry carries
`Severity: "critical"`. CORR-02 gates this escalation on catalog sanity state to prevent
a catalog-poisoning scenario where injecting 1001 critical entries triggers a block storm.

**The central design resolution** (the sanity-threading question): `internal/policy` is a
pure function library with no I/O. `catalog.CheckSanity()` is also pure (no I/O), but it
lives in `internal/catalog`. Calling it from `corroborate()` would create an import cycle
(`policy` → `catalog` → `policy`). The correct mechanism is the same caller-resolves-IO
pattern already used for `EvaluateReleaseAge`: the I/O caller (`handler.go`, `gateway`,
`watch`, `scan`) computes the sanity state **before** calling `policy.Evaluate`, and
threads it into the pure engine via a new `CatalogHealthy bool` field on
`CorroborationThresholds`. This is not I/O threading — `CheckSanity` is itself pure; it
only takes `(prevCount, newCount int, cfg SanityConfig)` and returns `SanityResult`. The
caller is responsible for supplying the counts (already tracked by the catalog sync state)
and passing the resulting `SanityResult.Alert || SanityResult.Block` boolean to the
engine.

**Primary recommendation:** Add `SeverityOverrides map[string]SeverityThreshold` and
`CatalogHealthy bool` to `CorroborationThresholds`; extend `corroborate()` and
`validateCorroborationThresholds`; add `CriticalBlockAt` field to `PolicyRule`; thread
`CatalogHealthy` from all four call sites of `ThresholdsFromPolicyFiles` +
`policy.Evaluate`. Default `SeverityOverrides["critical"] = {BlockAt:1, QuarantineAt:2}`.
Default `CatalogHealthy: true` (safe default; callers that cannot determine state pass
`true` and get the benefit of severity escalation; only confirmed-degraded state suppresses
it).

---

<user_constraints>
## User Constraints (from CONTEXT.md)

No CONTEXT.md exists for Phase 6. Constraints derive from CLAUDE.md and REQUIREMENTS.md.

### Locked Decisions (from CLAUDE.md + milestone research)
- `internal/policy` is a pure function library — no I/O, no goroutines, no side effects.
  Called synchronously from hook handler, gateway middleware, and Sentry correlation.
- Corroboration sanity bounds + catalog signature verification are self-defense
  non-negotiables (CLAUDE.md Phase 2 self-defense).
- Do NOT mark bundled bumblebee catalog `Signed:true` — inverts the trust model and
  creates a poisoning vector (REQUIREMENTS.md Out of Scope; SUMMARY.md Flag 3).
- `validateCorroborationThresholds` must reject `BlockAt < 1`.
- All-versions (`versions:["*"]`) critical entries still require 2-source corroboration.
- Escalation + sanity gate are one atomic deliverable (STATE.md Blockers/Concerns).
- No new module dependencies in Phase 6.

### Claude's Discretion
- Whether `CatalogHealthy` is a bool on `CorroborationThresholds` or a separate struct
  field (bool is sufficient; richer type is over-engineering for this phase).
- Whether `SeverityOverrides` is policy-file-configurable in Phase 6 or hardcoded
  defaults only (research recommends: make it policyloader-configurable, one new field
  `critical_block_at` in `corroboration_threshold` rules, mirrors existing `block_at`).
- Exact name of the boolean field (`CatalogHealthy`, `CatalogSane`, `SanityOK`) — any
  name that reads clearly in `CorroborationThresholds{CatalogHealthy: true}` is fine.

### Deferred Ideas (OUT OF SCOPE)
- OSV consulted as automatic second corroborating source on hot path (CORR-F1, v1.3.0+).
- Catalog signing infrastructure — bundled bumblebee with real Ed25519 keys (v1.3.0+).
- Distinguish GHSA-* (patched CVEs) from MAL-* (actively malicious) in critical path
  (NUDGE-F3, v1.3.0+).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CORR-01 | A critical-severity catalog match escalates to block at a single trusted source via `SeverityOverrides["critical"]={BlockAt:1}`, so `ai-figure` (Shai-Hulud) is blocked. | `SeverityThreshold` type + `SeverityOverrides` field on `CorroborationThresholds`; `findSeverityOverride` pure helper in `corroborate()`; default wired in `DefaultCorroborationThresholds()`. |
| CORR-02 | The escalation is gated on catalog sanity — does NOT apply when sanity reports degraded/alert; `validateCorroborationThresholds` rejects `BlockAt < 1`; all-versions (`versions:["*"]`) critical entry still requires 2-source corroboration. | `CatalogHealthy bool` on `CorroborationThresholds`; sanity state threaded from all four `policy.Evaluate` call sites; `allVersionsGuard` check in `corroborate()` before `findSeverityOverride` fires. |
</phase_requirements>

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Per-severity threshold logic | `internal/policy` (pure) | — | Decision is pure math over already-resolved `CatalogMatch.Severity` fields; no I/O needed |
| Sanity state computation | `internal/catalog` (pure `CheckSanity`) | Caller I/O tier | `CheckSanity(prevCount, newCount, cfg)` is pure; caller resolves counts from state |
| Sanity state threading | Caller I/O tier (`handler.go`, `gateway`, `watch`, `scan`) | `internal/policyloader` (via `CorroborationThresholds`) | Must not cross package boundary into `catalog`; passed as resolved bool |
| Policy-file-configurable overrides | `internal/policyloader` | — | Mirrors existing `corroboration_threshold` rule pattern; `internal/policy` stays config-free |
| Validation of threshold bounds | `internal/policy/corroboration.go` (`validateCorroborationThresholds`) | `internal/policyloader/validate.go` | Pure engine validates at eval time; policyloader validates at load time |

---

## Standard Stack

### Core (no new dependencies)

Phase 6 adds zero new module dependencies. All work uses existing packages.

| Package | Role in Phase 6 |
|---------|----------------|
| `internal/policy` | Extend `types.go` and `corroboration.go` |
| `internal/policyloader` | Extend `loader.go` and `validate.go` for `critical_block_at` policy-file field |
| `internal/catalog` (read-only) | `catalog.CheckSanity` is the sanity source; callers read catalog state |
| `internal/check/handler.go` | Thread `CatalogHealthy` into `CorroborationThresholds` |
| `internal/gateway/policy.go` | Thread `CatalogHealthy` into `CorroborationThresholds` |
| `internal/watch/handler.go` | Thread `CatalogHealthy` into `CorroborationThresholds` |
| `internal/scan/scanner.go` | Thread `CatalogHealthy` into `CorroborationThresholds` |

---

## Architecture Patterns

### System Architecture Diagram

```
Catalog sanity state (prevCount, newCount from catalog state file)
  |
  v
catalog.CheckSanity(prevCount, newCount, DefaultSanityConfig())
  |  [pure function, internal/catalog — no import cycle]
  |  returns SanityResult{Alert: bool, Block: bool}
  |
  | I/O caller resolves: healthy = !(result.Alert || result.Block)
  v
CorroborationThresholds{
    WarnAt: 1, BlockAt: 2, QuarantineAt: 3,        ← unchanged defaults
    SeverityOverrides: {"critical": {BlockAt:1, QuarantineAt:2}},  ← CORR-01
    CatalogHealthy: healthy,                          ← CORR-02
}
  |
  v
policy.Evaluate(toolCall, multiIdx, thresholds, ac)
  |
  v
corroborate(matches, t)
  |
  ├── validateCorroborationThresholds(t)
  |     checks WarnAt <= BlockAt <= QuarantineAt
  |     checks SeverityOverrides[s].BlockAt >= 1 for all s      ← CORR-02
  |     checks SeverityOverrides[s].BlockAt <= t.BlockAt         ← sanity bound
  |     fail closed (return "block") on violation
  |
  ├── allVersionsGuard: does ANY match have Version == "*"?
  |     YES → skip severity override (still requires 2-source corroboration)
  |
  ├── findSeverityOverride(matches, t.SeverityOverrides, t.CatalogHealthy)
  |     scans matches for Severity in SeverityOverrides
  |     returns nil if CatalogHealthy == false (sanity gate)
  |     returns nil if allVersionsGuard applies
  |     returns most-restrictive override (lowest BlockAt) otherwise
  |
  └── escalation decision table (existing + severity override path)
        if override != nil:
          effectiveBlockAt = override.BlockAt
          effectiveQuarantineAt = override.QuarantineAt
        else:
          effectiveBlockAt = t.BlockAt      (2, global default)
          effectiveQuarantineAt = t.QuarantineAt (3, global default)
        → existing switch statement using effectiveBlockAt/QuarantineAt
```

### Recommended Project Structure

No new directories. All changes are in-place modifications to existing files:

```
internal/policy/
  types.go           # Add SeverityThreshold type; extend CorroborationThresholds
  corroboration.go   # Extend validateCorroborationThresholds; add findSeverityOverride
                     # + allVersionsGuard; extend corroborate() escalation table
  corroboration_test.go  # Add new table-driven tests (Shai-Hulud, degraded-catalog,
                          # all-versions guard, BlockAt<1 rejection)
internal/policyloader/
  loader.go          # Add CriticalBlockAt int to PolicyRule
  validate.go        # Add legalRuleTypes["corroboration_threshold"] already exists;
                     # add bounds check for CriticalBlockAt >= 1
  test.go            # Extend ThresholdsFromPolicyFiles to read CriticalBlockAt
  test_test.go       # Tests for ThresholdsFromPolicyFiles with CriticalBlockAt
internal/check/
  handler.go         # Thread CatalogHealthy into thresholds (after ThresholdsFromPolicyFiles call)
internal/gateway/
  policy.go          # Thread CatalogHealthy into thresholds
internal/watch/
  handler.go         # Thread CatalogHealthy into thresholds
internal/scan/
  scanner.go         # Thread CatalogHealthy into thresholds
```

### Pattern 1: Caller-Resolved Sanity State (mirrors EvaluateReleaseAge)

**What:** The I/O caller resolves external state (sanity result from `catalog.CheckSanity`)
before calling the pure engine. The pure engine receives a pre-resolved bool.

**When to use:** Any time the pure engine needs information from an external system.

**Example — how callers derive `CatalogHealthy`:**

```go
// Source: internal/check/handler.go (proposed addition after line 246)

// Derive corroboration thresholds from loaded policy files.
thresholds := policyloader.ThresholdsFromPolicyFiles(policyFiles)

// CORR-02: resolve catalog sanity state and thread into thresholds.
// Read SourceState.Degraded from state.json — the watch daemon already computes
// and persists the sanity result at sync time. No re-run of CheckSanity needed.
// Default to healthy=true when state is unavailable (fail-safe: escalation applies).
catalogHealthy := true
statePath := filepath.Join(filepath.Dir(cacheDir), "state.json")
if state, err := catalog.LoadState(statePath); err == nil {
    if src, ok := state.Sources["bumblebee"]; ok {
        catalogHealthy = !src.Degraded
    }
}
thresholds.CatalogHealthy = catalogHealthy

// Pure policy evaluation — no I/O after this point.
decision := policy.Evaluate(toolCall, multiIdx, thresholds, ac)
```

**Why this preserves purity:** `catalog.CheckSanity` is itself pure (`sanity.go:63` —
"pure function — no I/O"). The caller executes the I/O (`catalog.LoadState` reads state
file), then passes the pure computation result to the pure engine. `internal/policy` never
imports `internal/catalog`.

**Existing precedent:** `policy.EvaluateReleaseAge` receives `ReleaseAgeInput{AgeMinutes
int64}` — the caller resolves the HTTP call and hands a pre-computed integer to the pure
engine. Same pattern, different data.

### Pattern 2: findSeverityOverride (pure helper in corroborate.go)

**What:** Scan `[]CatalogMatch` for any entry whose `Severity` key exists in
`SeverityOverrides`, respecting the all-versions guard and the catalog-healthy gate.
Return the most-restrictive override found (lowest `BlockAt`), or nil.

**Example:**

```go
// Source: internal/policy/corroboration.go (proposed addition)

// findSeverityOverride returns the most-restrictive SeverityThreshold override
// from t.SeverityOverrides that applies to any match, or nil when:
//   - CatalogHealthy is false (sanity gate: degraded catalog suppresses escalation)
//   - all matches are all-version entries (Version == "*")
//   - no match severity is in SeverityOverrides
//
// "Most restrictive" means the override with the lowest BlockAt.
// Pure: reads only matches, overrides map, and the healthy flag — no I/O.
func findSeverityOverride(
    matches []CatalogMatch,
    overrides map[string]SeverityThreshold,
    catalogHealthy bool,
) *SeverityThreshold {
    if !catalogHealthy {
        return nil  // CORR-02: sanity gate — no escalation on degraded catalog
    }
    if len(overrides) == 0 {
        return nil
    }

    // CORR-02 all-versions guard: if ANY match has Version == "*", do not escalate.
    // A mis-tagged wildcard entry must never block at single-source.
    for _, m := range matches {
        if m.Dissented {
            continue
        }
        if m.Version == "*" {
            return nil
        }
    }

    var best *SeverityThreshold
    for _, m := range matches {
        if m.Dissented {
            continue
        }
        if ov, ok := overrides[m.Severity]; ok {
            if best == nil || ov.BlockAt < best.BlockAt {
                cp := ov  // copy to avoid aliasing map value
                best = &cp
            }
        }
    }
    return best
}
```

### Pattern 3: Default includes SeverityOverrides

```go
// Source: internal/policy/types.go (proposed extension of DefaultCorroborationThresholds)

func DefaultCorroborationThresholds() CorroborationThresholds {
    return CorroborationThresholds{
        WarnAt:       1,
        BlockAt:      2,
        QuarantineAt: 3,
        CatalogHealthy: true, // default healthy; callers override to false when degraded
        SeverityOverrides: map[string]SeverityThreshold{
            "critical": {BlockAt: 1, QuarantineAt: 2},
        },
    }
}
```

### Anti-Patterns to Avoid

- **Anti-Pattern A — `import "internal/catalog"` in `corroborate.go`:** Creates an import
  cycle. The import test `TestCorroborationImportsArePure` will catch this. The correct
  pattern is threading the pre-resolved `bool` from the caller.

- **Anti-Pattern B — `forceSigned = true` shortcut in `corroborate()`:** A flag that
  treats unsigned sources as signed without the sanity gate creates the poisoning vector
  (PITFALLS.md Pitfall 2). Never do this.

- **Anti-Pattern C — `SeverityOverrides["critical"].BlockAt = 0`:** Blocks every tool
  call regardless of catalog hits. `validateCorroborationThresholds` must reject `BlockAt
  < 1` with a descriptive error; the existing fail-closed path (`return "block"`) already
  covers the consequence.

- **Anti-Pattern D — all-versions guard skipped:** A catalog entry with `Versions: ["*"]`
  and `Severity: "critical"` must NOT trigger single-source block. The guard must run
  before `findSeverityOverride`. See REQUIREMENTS.md CORR-02 and PITFALLS.md Pitfall 8.

- **Anti-Pattern E — `CatalogHealthy` defaults to `false` (fail-closed default for the
  wrong thing):** If `CatalogHealthy` defaults `false`, the entire severity-escalation
  feature is dead code when callers forget to set it. Default `true` — escalation is
  active by default; callers explicitly suppress it on confirmed degradation. Failing open
  on the sanity check means "unable to determine degradation state → assume healthy →
  apply escalation." This is fail-safe for the feature, not a security fail-open.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead |
|---------|-------------|-------------|
| Sanity bound computation | Custom threshold math in `corroborate()` | Existing `catalog.CheckSanity()` called in the I/O caller |
| Severity enum validation | Separate package or regex | Simple `map[string]SeverityThreshold` lookup; unknown severities produce no override (no block) |
| Thread-safety for thresholds | `sync.RWMutex` on `CorroborationThresholds` | `CorroborationThresholds` is a value type passed by value to `corroborate()`; no shared state |

**Key insight:** The policy engine's purity guarantee means all thread-safety concerns
belong to the caller tier. `CorroborationThresholds` passed by value is intrinsically
safe.

---

## Critical Design Questions — Resolved

### Q1: How is sanity state threaded into the pure engine without breaking purity?

**Resolution:** Via `CatalogHealthy bool` on `CorroborationThresholds`. The I/O caller
calls `catalog.LoadState` + `catalog.CheckSanity` (both already exist), derives a bool,
and sets `thresholds.CatalogHealthy = !(sanity.Alert || sanity.Block)`. The pure engine
reads the pre-resolved bool. No import cycle. No I/O in `corroborate()`.

`catalog.CheckSanity` is itself pure (verified: `sanity.go:63` doc comment "pure function
— no I/O"). The caller-tier I/O is `catalog.LoadState` which reads
`~/.beekeeper/catalogs/state.json`.

**Fallback when state unavailable:** `catalogHealthy = true`. Rationale: inability to
read the state file is not evidence of catalog degradation; degradation evidence is a
large delta in entry count. Defaulting to `true` means escalation applies (the desired
production behavior); only confirmed degradation suppresses it.

### Q2: How does `CatalogMatch.Severity` get populated, and is "critical" reliable?

**Resolution:** Severity comes from two sources:

- **Bumblebee adapter** (`catalog/multi.go:138`): `Severity: e.Severity` — read directly
  from the catalog `Entry.Severity` field (schema: `critical|high|medium|low`). The
  live `ai-figure` bumblebee entry carries `severity: "critical"` [VERIFIED: SUMMARY.md
  Flag 3 live test outcome].

- **OSV adapter** (`catalog/osv.go:257`): `Severity: deriveSeverity(v)` — reads
  `DatabaseSpecific["severity"]` from the OSV JSON response. For `MAL-2026-4126`
  (ai-figure), OSV's `DatabaseSpecific.severity` is `"unknown"` [VERIFIED: SUMMARY.md
  Flag 3 live test outcome: "bumblebee severity 'critical', OSV severity 'unknown'"].

**Consequence for override keying:** `findSeverityOverride` scans ALL matches for any
whose `Severity` is in `SeverityOverrides`. In the ai-figure case:
- bumblebee match: `Severity: "critical"`, `Signed: false` — triggers override lookup
- OSV match: `Severity: "unknown"`, `Signed: true` — no override match

The override fires when the bumblebee match carries `Severity: "critical"`, even though
bumblebee is unsigned. The override then uses `effectiveBlockAt = 1`, and `signedCount =
1` (OSV is signed), so `signedCount >= effectiveBlockAt` → **block**. This is the correct
outcome: the OSV signed source provides the trusted confirmation; the bumblebee critical
tag provides the severity trigger.

**Alternative considered (rejected):** Key the override only off signed-source severity.
Rejected because: bumblebee may be the only source carrying the severity tag, and
requiring the severity tag to come from a signed source would make the override useless
for the exact failure mode we're fixing.

### Q3: Should SeverityOverrides be policy-file-configurable in Phase 6?

**Resolution:** YES — add `CriticalBlockAt int` to `PolicyRule` (policyloader) and
extend `ThresholdsFromPolicyFiles` to apply it. Rationale: making it configurable allows
operators to revert to 2-source behavior if false positives emerge (`critical_block_at:
2`), which is important for the PITFALLS.md Pitfall 8 (severity inflation) recovery path.
Hardcoding-only would require a Beekeeper release to tune.

Schema: add `critical_block_at` to the existing `corroboration_threshold` rule type
(no new rule_type, no schema_version bump):

```json
{
  "schema_version": "1",
  "name": "critical-block-policy",
  "rules": [{
    "id": "CORR-01",
    "rule_type": "corroboration_threshold",
    "critical_block_at": 1
  }]
}
```

**Validation bounds:** `critical_block_at` must satisfy `>= 1` AND `<= global block_at`.
Validated in both `policyloader/validate.go` (at load time) and
`validateCorroborationThresholds` (at eval time, fail-closed).

### Q4: Where exactly does the all-versions guard live in `corroborate()`?

**Resolution:** Inside `findSeverityOverride`, as a pre-check before iterating matches.
The guard returns `nil` (no override) immediately when ANY non-dissented match has
`Version == "*"`. This is simpler than a separate pre-call check in `corroborate()`, keeps
the guard collocated with the logic it protects, and is equally pure.

**The `"*"` value:** The bumblebee adapter (`catalog/multi.go:142-155`) emits one
`CatalogMatch` per version string in `Entry.Versions`. If `Entry.Versions = ["*"]`, the
match carries `Version: "*"`. The guard checks `m.Version == "*"` literally. This is
precise: it catches the explicit wildcard sentinel without affecting version-specific
entries.

Note: if `Entry.Versions = []` (empty — applies to all versions), the adapter emits a
single match with `Version: ""` (`multi.go:131-141`). The guard does NOT fire on `Version:
""`, because an empty version means "version unknown" (e.g., `npm install ai-figure`
without `@version`) rather than "all versions". This distinction is correct: an unknown
version should still receive critical escalation if severity is critical.

---

## Exact Signature Changes

### `internal/policy/types.go`

**Add:**

```go
// SeverityThreshold specifies escalation thresholds for a specific severity level.
// Used in CorroborationThresholds.SeverityOverrides for per-severity escalation (CORR-01).
//
// Sanity bounds enforced by validateCorroborationThresholds:
//   - BlockAt >= 1 (zero would block unconditionally on any catalog hit)
//   - BlockAt <= global CorroborationThresholds.BlockAt (cannot be looser than global)
//   - QuarantineAt >= BlockAt
type SeverityThreshold struct {
    BlockAt      int // minimum signed-source count to block at this severity (min 1)
    QuarantineAt int // minimum signed-source count to block+quarantine at this severity
}
```

**Modify `CorroborationThresholds`:**

```go
type CorroborationThresholds struct {
    WarnAt       int
    BlockAt      int
    QuarantineAt int
    // CORR-01: per-severity threshold overrides. When ANY non-dissented CatalogMatch
    // has Severity in this map AND CatalogHealthy is true AND no all-versions wildcard
    // applies, corroborate() uses the override's BlockAt/QuarantineAt instead of the
    // global values.
    //
    // Default: {"critical": {BlockAt:1, QuarantineAt:2}}
    // Nil map is valid (no overrides — standard global thresholds apply).
    SeverityOverrides map[string]SeverityThreshold
    // CORR-02: CatalogHealthy signals whether the catalog passed its sanity check.
    // When false, SeverityOverrides are suppressed (poisoning guard).
    // Default: true (caller sets false when catalog.CheckSanity reports Alert/Block).
    CatalogHealthy bool
}
```

**Modify `DefaultCorroborationThresholds()`:**

```go
func DefaultCorroborationThresholds() CorroborationThresholds {
    return CorroborationThresholds{
        WarnAt:       1,
        BlockAt:      2,
        QuarantineAt: 3,
        CatalogHealthy: true,
        SeverityOverrides: map[string]SeverityThreshold{
            "critical": {BlockAt: 1, QuarantineAt: 2},
        },
    }
}
```

### `internal/policy/corroboration.go`

**Modify `validateCorroborationThresholds`:**

```go
func validateCorroborationThresholds(t CorroborationThresholds) error {
    if t.WarnAt > t.BlockAt {
        return fmt.Errorf("corroboration: WarnAt (%d) must be <= BlockAt (%d)", t.WarnAt, t.BlockAt)
    }
    if t.BlockAt > t.QuarantineAt {
        return fmt.Errorf("corroboration: BlockAt (%d) must be <= QuarantineAt (%d)", t.BlockAt, t.QuarantineAt)
    }
    // CORR-02: validate per-severity overrides.
    for sev, ov := range t.SeverityOverrides {
        if ov.BlockAt < 1 {
            return fmt.Errorf("corroboration: SeverityOverrides[%q].BlockAt (%d) must be >= 1 (zero blocks unconditionally)", sev, ov.BlockAt)
        }
        if ov.BlockAt > t.BlockAt {
            return fmt.Errorf("corroboration: SeverityOverrides[%q].BlockAt (%d) must be <= global BlockAt (%d)", sev, ov.BlockAt, t.BlockAt)
        }
        if ov.QuarantineAt < ov.BlockAt {
            return fmt.Errorf("corroboration: SeverityOverrides[%q].QuarantineAt (%d) must be >= BlockAt (%d)", sev, ov.QuarantineAt, ov.BlockAt)
        }
    }
    return nil
}
```

**Add `findSeverityOverride` (pure helper):** See Architecture Patterns section above.

**Modify `corroborate()` escalation table** (after `signedCount` is computed):

```go
// CORR-01/02: check for per-severity threshold override.
// findSeverityOverride returns nil when: catalog is degraded (CatalogHealthy=false),
// any match is an all-versions wildcard (Version=="*"), or no severity matches
// SeverityOverrides keys.
effectiveBlockAt := t.BlockAt
effectiveQuarantineAt := t.QuarantineAt
if ov := findSeverityOverride(matches, t.SeverityOverrides, t.CatalogHealthy); ov != nil {
    effectiveBlockAt = ov.BlockAt
    effectiveQuarantineAt = ov.QuarantineAt
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

### `internal/policyloader/loader.go`

**Add to `PolicyRule`:**

```go
// CriticalBlockAt sets the minimum signed-source count to block at
// "critical" severity. Extends the corroboration_threshold rule type (CORR-01).
// Valid range: 1 <= CriticalBlockAt <= block_at.
// Zero means "use default" (not "block unconditionally").
CriticalBlockAt int `json:"critical_block_at,omitempty"`
```

### `internal/policyloader/validate.go`

**Add to `ValidateSchema` (inside the rule loop):**

```go
if r.RuleType == "corroboration_threshold" && r.CriticalBlockAt != 0 {
    // CriticalBlockAt zero means "use default" — only validate non-zero values.
    if r.CriticalBlockAt < 1 {
        errs = append(errs, fmt.Errorf("rule[%d] %q: critical_block_at (%d) must be >= 1",
            i, r.ID, r.CriticalBlockAt))
    }
    // Upper bound (>= global block_at) validated at eval time by
    // validateCorroborationThresholds (which has the resolved global BlockAt).
}
```

### `internal/policyloader/test.go` (`ThresholdsFromPolicyFiles`)

**Extend the inner loop:**

```go
if r.CriticalBlockAt > 0 {
    if t.SeverityOverrides == nil {
        t.SeverityOverrides = make(map[string]SeverityThreshold)
    }
    existing := t.SeverityOverrides["critical"]
    existing.BlockAt = r.CriticalBlockAt
    if existing.QuarantineAt == 0 {
        existing.QuarantineAt = r.CriticalBlockAt + 1 // default: quarantine one above block
    }
    t.SeverityOverrides["critical"] = existing
}
```

### Call Sites of `ThresholdsFromPolicyFiles` / `policy.Evaluate` — All Four

Every call site needs `CatalogHealthy` threaded in. The pattern is identical at all four:

**1. `internal/check/handler.go` (line 246):**

```go
thresholds := policyloader.ThresholdsFromPolicyFiles(policyFiles)
// CORR-02: thread catalog sanity state into thresholds.
thresholds.CatalogHealthy = resolveCatalogHealthy(cacheDir)
decision := policy.Evaluate(toolCall, multiIdx, thresholds, ac)
```

**2. `internal/gateway/policy.go` (line 119):**

```go
thresholds := policyloader.ThresholdsFromPolicyFiles(policyFiles)
thresholds.CatalogHealthy = resolveCatalogHealthy(cfg.CacheDir)
decision := policy.Evaluate(tc, idx, thresholds, ac)
```

**3. `internal/watch/handler.go` (line 132):**

```go
thresholds := policyloader.ThresholdsFromPolicyFiles(policyFiles)
thresholds.CatalogHealthy = resolveCatalogHealthy(cfg.CacheDir)
catalogDecision := policy.Evaluate(tc, multiIdx, thresholds, policy.AgentContext{})
```

**4. `internal/scan/scanner.go` (line 280):**

```go
thresholds := policyloader.ThresholdsFromPolicyFiles(policyFiles)
thresholds.CatalogHealthy = resolveCatalogHealthy(cfg.CacheDir)
catalogDecision := policy.Evaluate(tc, multiIdx, thresholds, policy.AgentContext{})
```

**One exception — `internal/check/selftest.go` (line 121):** Uses
`policy.DefaultCorroborationThresholds()` directly (not `ThresholdsFromPolicyFiles`).
`DefaultCorroborationThresholds()` already sets `CatalogHealthy: true`, so selftest
automatically gets severity escalation active. This is correct: selftest uses known-good
catalog fixtures, not a potentially-degraded real catalog.

**`resolveCatalogHealthy(cacheDir string) bool` — shared helper:**

Place in each package's existing utility file or in a new small `internal/check/sanity.go`
(check), `internal/watch/sanity.go` (watch), etc. Does NOT go in `internal/policy` or
`internal/catalog`.

```go
// resolveCatalogHealthy loads the catalog state file and runs CheckSanity.
// Returns true (healthy) when: state file is missing, state file is unreadable,
// or sanity thresholds are not exceeded. Returns false only when CheckSanity
// returns Alert=true or Block=true (confirmed degradation).
//
// Rationale for defaulting to true: inability to read state file is not evidence
// of catalog degradation; escalation applies when state is unknown.
// resolveCatalogHealthy reads the catalog watch state and returns false when
// the bumblebee source is marked Degraded by the watch daemon.
// catalog.LoadState takes a full path to state.json (not a directory).
// Returns true (healthy) when: state file is missing, state file is unreadable,
// or no source is marked Degraded. Returns false only on confirmed degradation.
func resolveCatalogHealthy(cacheDir string) bool {
    if cacheDir == "" {
        return true
    }
    statePath := filepath.Join(filepath.Dir(cacheDir), "state.json")
    state, err := catalog.LoadState(statePath)
    if err != nil {
        return true  // missing/unreadable state → assume healthy
    }
    if src, ok := state.Sources["bumblebee"]; ok {
        return !src.Degraded  // Degraded=true iff sanity check failed at last sync
    }
    return true  // bumblebee not in state yet (first run) → assume healthy
}
```

**Verify `catalog.LoadState` exists:** Search for state loading in `internal/catalog/state.go`.

---

## State File Investigation: VERIFIED

[VERIFIED: `internal/catalog/state.go` read directly during research.]

`catalog.LoadState(path string)` takes a **full path** (e.g. `~/.beekeeper/state.json`) and
returns `WatchState`. The `WatchState.Sources` map (keyed by source name, e.g. `"bumblebee"`)
carries `SourceState` with a `Degraded bool` field that the watch daemon already computes
and persists whenever `catalog.CheckSanity` reports `Alert=true` or `Block=true`.

**No need to re-run `CheckSanity` in `resolveCatalogHealthy`.** Read
`state.Sources["bumblebee"].Degraded` directly. The watch daemon is the authoritative
computation site; consumers in check/gateway/watch/scan are readers only.

**State path convention:** cacheDir is e.g. `~/.beekeeper/catalogs`; state.json lives at
`~/.beekeeper/state.json`. Callers must derive
`statePath = filepath.Join(filepath.Dir(cacheDir), "state.json")`.


---

## Common Pitfalls

### Pitfall 1: Import Cycle via `catalog.CheckSanity` in `corroborate()`

**What goes wrong:** Adding `import "github.com/bantuson/beekeeper/internal/catalog"` to
`corroboration.go` to call `CheckSanity` directly.

**Why it happens:** `internal/catalog` imports `internal/policy` (via
`policy.MultiCatalogLookup`, `policy.CatalogMatch`). Adding the reverse import creates a
cycle the Go compiler rejects.

**How to avoid:** `CatalogHealthy bool` on `CorroborationThresholds`. Caller resolves.
`TestCorroborationImportsArePure` (existing test in `corroboration_test.go`) will catch
any import cycle attempt.

**Warning signs:** `go build ./...` failing with "import cycle not allowed"; `corroboration.go`
gaining any import beyond `"fmt"` and `"sort"`.

### Pitfall 2: `CatalogHealthy` Defaults to `false` (Feature Dead Code)

**What goes wrong:** If `DefaultCorroborationThresholds()` sets `CatalogHealthy: false`,
all four call sites that currently call `ThresholdsFromPolicyFiles` (which calls
`DefaultCorroborationThresholds()`) will silently suppress severity escalation unless
they explicitly set `CatalogHealthy: true`.

**How to avoid:** `CatalogHealthy` defaults `true`. Callers explicitly set `false` on
confirmed degradation. The feature is "on" by default; callers suppress it.

### Pitfall 3: All-Versions Guard Missing or Misplaced

**What goes wrong:** A catalog entry with `Versions: ["*"]` still triggers single-source
block because the guard is not checked before `findSeverityOverride` fires.

**How to avoid:** Guard lives inside `findSeverityOverride` as the first check after the
`!catalogHealthy` gate. Test: `TestCorroborationAllVersionsCriticalOneSource` — single
source, severity "critical", Version "*" → must produce "warn" not "block".

### Pitfall 4: `SeverityOverrides["critical"].BlockAt > global BlockAt` Accepted

**What goes wrong:** A policy file sets `critical_block_at: 3` (looser than global
`block_at: 2`). `validateCorroborationThresholds` must reject this: a more-dangerous
severity class with a looser threshold is a security misconfiguration.

**How to avoid:** The bound check `ov.BlockAt > t.BlockAt` in
`validateCorroborationThresholds` catches this and fails closed (returns "block" — the
existing fail-closed path).

### Pitfall 5: `checkSelftest.go` Uses Default Thresholds — Verify Behavior

**What goes wrong:** `internal/check/selftest.go:121` calls
`policy.DefaultCorroborationThresholds()` directly. After Phase 6, these defaults include
`SeverityOverrides["critical"] = {BlockAt:1}`. If selftest fixtures include a critical
entry, the selftest may now block where it previously warned.

**How to avoid:** Audit selftest fixtures (`internal/check/selftest.go` and any fixture
files) for entries with `Severity: "critical"`. If any exist, verify the selftest still
passes the expected outcome (the correct outcome may BE "block" after Phase 6). The
selftest also uses `CatalogHealthy: true` (via default), so escalation is active.

---

## Code Examples

### Shai-Hulud Test Fixture (Roadmap SC1)

```go
// Source: internal/policy/corroboration_test.go (new test)
// TestCorroborationShaiHuludCriticalBlock: bumblebee (unsigned, critical) +
// OSV (signed, severity "unknown") → single signed source + critical severity override →
// effectiveBlockAt=1 → block.
func TestCorroborationShaiHuludCriticalBlock(t *testing.T) {
    matches := []CatalogMatch{
        {CatalogSource: "bumblebee", Severity: "critical", Signed: false},
        {CatalogSource: "osv", Severity: "unknown", Signed: true},
    }
    t := DefaultCorroborationThresholds() // includes CatalogHealthy:true + SeverityOverrides
    level, quarantine, count, agreed, _ := corroborate(matches, t)
    if level != "block" {
        t.Errorf("level = %q, want %q (critical single-signed-source must block)", level, "block")
    }
    if quarantine {
        t.Error("quarantine should be false (signedCount=1 < QuarantineAt=2)")
    }
    if count != 1 {
        t.Errorf("count = %d, want 1 (only OSV is signed)", count)
    }
    if len(agreed) != 2 {
        t.Errorf("agreed = %v, want [bumblebee osv] (both matched)", agreed)
    }
}
```

### Degraded Catalog Regression (Roadmap SC2)

```go
// TestCorroborationDegradedCatalogNoEscalation: CatalogHealthy=false suppresses
// critical-severity override → same fixture as Shai-Hulud → warn-only.
func TestCorroborationDegradedCatalogNoEscalation(t *testing.T) {
    matches := []CatalogMatch{
        {CatalogSource: "bumblebee", Severity: "critical", Signed: false},
        {CatalogSource: "osv", Severity: "unknown", Signed: true},
    }
    thresholds := DefaultCorroborationThresholds()
    thresholds.CatalogHealthy = false // simulate >1000 delta entries injected
    level, _, _, _, _ := corroborate(matches, thresholds)
    if level != "warn" {
        t.Errorf("level = %q, want warn (degraded catalog must not escalate)", level)
    }
}
```

### All-Versions Guard (Roadmap SC3)

```go
// TestCorroborationAllVersionsCriticalWildcardStaysWarn: versions:["*"] critical
// entry with one signed source must NOT block even with SeverityOverrides active.
func TestCorroborationAllVersionsCriticalWildcardStaysWarn(t *testing.T) {
    matches := []CatalogMatch{
        {CatalogSource: "bumblebee", Severity: "critical", Version: "*", Signed: false},
        {CatalogSource: "osv", Severity: "unknown", Version: "*", Signed: true},
    }
    thresholds := DefaultCorroborationThresholds()  // CatalogHealthy:true
    level, _, _, _, _ := corroborate(matches, thresholds)
    if level != "warn" {
        t.Errorf("level = %q, want warn (all-versions wildcard must require 2 sources)", level)
    }
}
```

### BlockAt < 1 Rejection (Roadmap SC4)

```go
// TestValidateCorroborationThresholdsRejectsBlockAtZero: BlockAt<1 override fails closed.
func TestValidateCorroborationThresholdsRejectsBlockAtZero(t *testing.T) {
    thresholds := DefaultCorroborationThresholds()
    thresholds.SeverityOverrides["critical"] = SeverityThreshold{BlockAt: 0, QuarantineAt: 1}
    // validateCorroborationThresholds should return non-nil error.
    if err := validateCorroborationThresholds(thresholds); err == nil {
        t.Error("want error for BlockAt=0, got nil")
    }
    // And corroborate should fail closed (block) not silently allow.
    matches := []CatalogMatch{{CatalogSource: "bumblebee", Severity: "critical", Signed: false}}
    level, _, _, _, _ := corroborate(matches, thresholds)
    if level != "block" {
        t.Errorf("level = %q, want block (misconfigured thresholds → fail closed)", level)
    }
}
```

---

## State of the Art

| Old Behavior | New Behavior (Phase 6) | When Changed |
|--------------|----------------------|--------------|
| `ai-figure` (bumblebee unsigned + OSV signed, severity critical) → exit 0 warn | → exit 1 block | Phase 6 |
| `validateCorroborationThresholds` checks WarnAt/BlockAt/QuarantineAt ordering only | Also checks `SeverityOverrides[s].BlockAt >= 1` and `<= globalBlockAt` | Phase 6 |
| `CorroborationThresholds` has no severity awareness | `SeverityOverrides map[string]SeverityThreshold` + `CatalogHealthy bool` | Phase 6 |
| Catalog sanity state is not threaded into the policy engine | `CatalogHealthy` bool resolves sanity at the I/O caller tier | Phase 6 |
| `corroboration_threshold` policy rule supports `warn_at`/`block_at`/`quarantine_at` | Also supports `critical_block_at` | Phase 6 |

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing (`testing` package) — existing, no config needed |
| Config file | None (Go test files co-located with source) |
| Quick run command | `go test ./internal/policy/... ./internal/policyloader/... -run TestCorroboration -v` |
| Full suite command | `go test ./internal/policy/... ./internal/policyloader/... ./internal/check/... ./internal/gateway/... ./internal/watch/... ./internal/scan/... -race` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CORR-01 | Critical single-signed-source → block (Shai-Hulud fixture) | unit | `go test ./internal/policy/... -run TestCorroborationShaiHuludCriticalBlock -v` | ❌ Wave 0 |
| CORR-01 | `DefaultCorroborationThresholds()` includes `SeverityOverrides["critical"]` | unit | `go test ./internal/policy/... -run TestDefaultThresholdsIncludeSeverityOverrides -v` | ❌ Wave 0 |
| CORR-01 | Non-critical single-source still warns (no regression) | unit | `go test ./internal/policy/... -run TestCorroborationOneSignedSource -v` | ✅ existing |
| CORR-01 | `critical_block_at` in policy file lowers effective threshold | unit | `go test ./internal/policyloader/... -run TestThresholdsFromPolicyFilesCriticalBlockAt -v` | ❌ Wave 0 |
| CORR-02 | Degraded catalog (CatalogHealthy=false) → escalation suppressed | unit | `go test ./internal/policy/... -run TestCorroborationDegradedCatalogNoEscalation -v` | ❌ Wave 0 |
| CORR-02 | `validateCorroborationThresholds` rejects `BlockAt < 1` | unit | `go test ./internal/policy/... -run TestValidateCorroborationThresholdsRejectsBlockAtZero -v` | ❌ Wave 0 |
| CORR-02 | `validateCorroborationThresholds` rejects override BlockAt > global BlockAt | unit | `go test ./internal/policy/... -run TestValidateCorroborationThresholdsRejectsLooserOverride -v` | ❌ Wave 0 |
| CORR-02 | All-versions wildcard critical entry → warn at single source | unit | `go test ./internal/policy/... -run TestCorroborationAllVersionsCriticalWildcardStaysWarn -v` | ❌ Wave 0 |
| CORR-02 | CatalogHealthy threaded at handler.go call site | integration | `go test ./internal/check/... -run TestRunCheckCriticalBlockWithHealthyCatalog -v` | ❌ Wave 0 |
| SC1 (roadmap) | `beekeeper check` with ai-figure → exit 1, `decision:"block"` | integration | `go test ./internal/check/... -run TestRunCheckAiFigureBlocks -v` | ❌ Wave 0 |
| SC2 (roadmap) | Degraded catalog (1001 entries injected) → ai-figure still warns | integration | `go test ./internal/check/... -run TestRunCheckCriticalDegradedCatalogWarn -v` | ❌ Wave 0 |
| SC3 (roadmap) | `versions:["*"]` critical entry → 2-source required, not 1 | unit | `go test ./internal/policy/... -run TestCorroborationAllVersionsCriticalWildcardStaysWarn -v` | ❌ Wave 0 |
| SC4 (roadmap) | `validateCorroborationThresholds` rejects `BlockAt < 1` | unit | `go test ./internal/policy/... -run TestValidateCorroborationThresholdsRejectsBlockAtZero -v` | ✅ (extend existing validate test) |
| SC5 (roadmap) | Table-driven tests in `internal/policy/` | unit | `go test ./internal/policy/... -race -v` | ❌ Wave 0 |
| Purity | `corroboration.go` imports no I/O packages | static | `go test ./internal/policy/... -run TestCorroborationImportsArePure -v` | ✅ existing |

### Sampling Rate

- **Per task commit:** `go test ./internal/policy/... -run TestCorroboration -v`
- **Per wave merge:** `go test ./internal/policy/... ./internal/policyloader/... -race`
- **Phase gate:** Full suite green before `/gsd-verify-work 6`:
  `go test ./internal/policy/... ./internal/policyloader/... ./internal/check/... ./internal/gateway/... ./internal/watch/... ./internal/scan/... -race`

### Wave 0 Gaps

- [ ] `internal/policy/corroboration_test.go` — add:
  `TestCorroborationShaiHuludCriticalBlock`,
  `TestCorroborationDegradedCatalogNoEscalation`,
  `TestCorroborationAllVersionsCriticalWildcardStaysWarn`,
  `TestValidateCorroborationThresholdsRejectsBlockAtZero`,
  `TestValidateCorroborationThresholdsRejectsLooserOverride`,
  `TestDefaultThresholdsIncludeSeverityOverrides`
- [ ] `internal/policyloader/test_test.go` — add:
  `TestThresholdsFromPolicyFilesCriticalBlockAt`
- [ ] `internal/check/handler_test.go` — add:
  `TestRunCheckAiFigureBlocks`, `TestRunCheckCriticalDegradedCatalogWarn`,
  `TestRunCheckCriticalBlockWithHealthyCatalog`
- [x] `catalog.LoadState` signature verified — `SourceState.Degraded bool` is the correct field; `resolveCatalogHealthy` uses `filepath.Dir(cacheDir) + "/state.json"` as path

*(No new framework install needed — `go test` already configured.)*

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | yes | Corroboration threshold enforcement; fail-closed on misconfiguration |
| V5 Input Validation | yes | `validateCorroborationThresholds` bounds-checks all override fields; `CriticalBlockAt >= 1` enforced in policyloader `ValidateSchema` |
| V6 Cryptography | no | Catalog signing is out of scope (v1.3.0+) |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Catalog poisoning — inject 1001 critical entries to trigger block storm | Tampering | `CatalogHealthy` gate suppresses override when `CheckSanity` reports Alert/Block; existing `AlertDeltaEntries: 1000` backstop |
| Policy file injection — set `critical_block_at: 0` | Tampering | `ValidateSchema` rejects `CriticalBlockAt < 1`; `validateCorroborationThresholds` fails closed |
| Single-source mis-tagging — bumblebee tags `react` as `critical` + `versions:["*"]` | Tampering | All-versions guard in `findSeverityOverride` requires 2-source for wildcard entries |
| Override misconfiguration — set `critical_block_at: 5` (looser than global `block_at: 2`) | Spoofing | `validateCorroborationThresholds` rejects `override.BlockAt > t.BlockAt`; fail closed |

---

## Environment Availability

Step 2.6: SKIPPED — Phase 6 is code/config-only changes to `internal/policy` and
`internal/policyloader`. No external tools, databases, or CLIs are required.

---

## Open Questions (RESOLVED)

1. **`catalog.LoadState` signature** — CLOSED (VERIFIED during research)
   - `catalog.LoadState(path string)` takes a full path. `WatchState.Sources["bumblebee"].Degraded bool`
     is the pre-computed sanity result written by the watch daemon. `resolveCatalogHealthy`
     reads this field directly. No additional state struct changes required for Phase 6.

2. **`check/selftest.go` — severity escalation in selftest fixtures**
   - What we know: `selftest.go:121` uses `DefaultCorroborationThresholds()` directly.
     After Phase 6, defaults include `SeverityOverrides["critical"]`.
   - What's unclear: Whether any selftest fixture carries `Severity: "critical"` and
     currently expects `warn`.
   - Recommendation: Planner must grep selftest fixtures for `severity.*critical` before
     writing the plan. If present, the expected decision in the selftest assertion changes
     from "warn" to "block" (which is the correct post-Phase-6 behavior).

3. **`policyloader/validate.go` — `DisallowUnknownFields` + `critical_block_at`**
   - What we know: `LoadPolicyFile` uses `json.Decoder` with `DisallowUnknownFields`. New
     JSON field `critical_block_at` in `PolicyRule` will be accepted since it maps to the
     new `CriticalBlockAt int` struct field.
   - What's unclear: Whether adding `CriticalBlockAt` to `PolicyRule` requires a
     `schema_version` bump in policy files.
   - Recommendation: No schema version bump — `critical_block_at` is an optional extension
     to the existing `corroboration_threshold` rule type. Files without it remain valid.
     Existing `DisallowUnknownFields` behavior is preserved because `CriticalBlockAt` is
     now a known field.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | ~~`catalog.LoadState` exposes `PrevEntryCount`/`EntryCount`~~ CLOSED: `SourceState.Degraded bool` is the correct field; `LoadState` takes a full path not cacheDir | Resolved during research | NONE |
| A2 | No selftest fixture carries `Severity: "critical"` with a "warn" expectation | Open Questions / Pitfall 5 | Selftest assertions need updating to "block" — this is the CORRECT behavior change |
| A3 | The live `ai-figure` bumblebee entry carries `severity: "critical"` (not `"high"` or `"unknown"`) | Code Examples | SeverityOverrides key must match; verify against live catalog before writing fixtures |

All other claims in this document are VERIFIED against live source files.

---

## Sources

### Primary (HIGH confidence)

- `internal/policy/corroboration.go` — `corroborate()`, `validateCorroborationThresholds()`,
  escalation table (lines 32–108); purity imports test (corroboration_test.go:220–256)
- `internal/policy/types.go` — `CorroborationThresholds`, `CatalogMatch.Severity`,
  `Decision`, `DefaultCorroborationThresholds()` (all lines)
- `internal/policy/engine.go` — `Evaluate()` signature, how `thresholds` + `matches` flow
  into `corroborate()` (lines 64–176)
- `internal/catalog/sanity.go` — `CheckSanity()` pure function, `SanityResult`,
  `DefaultSanityConfig()` (all lines)
- `internal/catalog/multi.go` — `bumblebeeMultiAdapter.LookupAll()`, `Signed` field
  derivation (`e.CatalogSignature != ""`), version emission for `versions:["*"]` entries
  (lines 117–157)
- `internal/catalog/osv.go` — `deriveSeverity()` returns "unknown" for MAL-2026-4126;
  OSV `Signed: true` (lines 105–116, 303–329)
- `internal/check/handler.go` — `ThresholdsFromPolicyFiles` + `policy.Evaluate` call site
  at line 246/251; `policy.Evaluate` consumers confirmed: check, gateway, watch, scan
- `internal/gateway/policy.go` — gateway `applyPolicy` call site (lines 119, 121)
- `internal/watch/handler.go:132`, `internal/scan/scanner.go:280–290` — remaining call sites
- `internal/policyloader/test.go` — `ThresholdsFromPolicyFiles` implementation (lines 51–80)
- `internal/policyloader/loader.go` — `PolicyRule` existing fields (lines 37–49)
- `internal/policyloader/validate.go` — `ValidateSchema`, legal rule types (lines 14–58)
- `internal/policyloader/enforce.go` — `extractTargetPath` (lines 279–287); overlay skip
  for `corroboration_threshold` (lines 124–127)
- `.planning/research/SUMMARY.md` — Flag 3 (PLCY-07 analysis); live `ai-figure` test
  result: bumblebee severity "critical", OSV severity "unknown"
- `.planning/research/ARCHITECTURE.md` — PLCY-07 section: types.go additions,
  `findSeverityOverride` design, `corroborate()` modification, policyloader extensions
- `.planning/research/PITFALLS.md` — Pitfall 2 (poisoning via signed-equivalent shortcut),
  Pitfall 8 (severity inflation / all-versions guard), Pitfall 9 (tests mocking too much)
- `.planning/REQUIREMENTS.md` — CORR-01, CORR-02 verbatim; Out of Scope table
- `.planning/ROADMAP.md` — Phase 6 five success criteria (SC1–SC5)
- `.planning/STATE.md` — Blockers/Concerns: "escalation + sanity gate are one atomic
  deliverable"; Phase 11 decisions: `VerifySignatureWithKey` + dissent sentinels

### Secondary (MEDIUM confidence)

- `internal/catalog/schema.go` — `Entry.Severity` field and legal values
  (`critical|high|medium|low`) [VERIFIED: lines 19–31]
- `internal/check/selftest.go:121` — uses `DefaultCorroborationThresholds()` directly
  [VERIFIED: Grep result; selftest fixtures not inspected for severity values — A2]

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new dependencies; all changes in existing files verified
- Architecture: HIGH — all four call sites of `policy.Evaluate` verified via grep;
  `CatalogMatch.Severity` population path verified in both adapters; purity constraint
  verified via import path analysis
- Pitfalls: HIGH — derived from live source inspection and prior milestone research
- Sanity threading mechanism: HIGH — `catalog.CheckSanity` is pure (confirmed);
  import cycle is real (confirmed); `CatalogHealthy bool` on thresholds is the
  exact mechanism that mirrors `EvaluateReleaseAge(ReleaseAgeInput{AgeMinutes int64})`

**Research date:** 2026-06-03
**Valid until:** 2026-07-03 (stable domain — pure Go library; no external API dependency)
