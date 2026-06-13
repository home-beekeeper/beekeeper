# Phase 6: Corroboration Severity Hardening — Pattern Map

**Mapped:** 2026-06-03
**Files analyzed:** 10 (4 modified source, 4 call-site wiring, 3 test extensions)
**Analogs found:** 10 / 10

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/policy/types.go` | model | transform | existing `CorroborationThresholds` struct in same file | exact — extend in-place |
| `internal/policy/corroboration.go` | utility (pure) | transform | existing `validateCorroborationThresholds` + `corroborate()` in same file | exact — extend in-place |
| `internal/policy/engine.go` | utility (pure) | transform | `Evaluate()` in same file — `CatalogHealthy` thread-through only | exact |
| `internal/policyloader/loader.go` | model | transform | existing `PolicyRule` struct + `WarnAt`/`BlockAt`/`QuarantineAt` fields in same file | exact — add one field |
| `internal/policyloader/validate.go` | utility | transform | existing `ValidateSchema` rule loop in same file | exact — extend the loop |
| `internal/policyloader/test.go` | utility (bridge) | transform | existing `ThresholdsFromPolicyFiles` loop in same file | exact — extend the loop |
| `internal/check/handler.go` | controller | request-response | existing `thresholds := policyloader.ThresholdsFromPolicyFiles(policyFiles)` block at line 246 | exact — one line added after |
| `internal/gateway/policy.go` | middleware | request-response | existing `thresholds` + `policy.Evaluate` block at line 119–121 | exact |
| `internal/watch/handler.go` | controller | event-driven | existing `thresholds` + `policy.Evaluate` block at line 132–143 | exact |
| `internal/scan/scanner.go` | service | batch | existing `thresholds` + `policy.Evaluate` block at line 280–290 | exact |
| `internal/policy/corroboration_test.go` | test | — | existing table-driven corroboration tests (lines 11–218) | exact — add new `Test*` functions |
| `internal/policyloader/test_test.go` | test | — | existing `TestThresholdsFromPolicyFile` + `TestPolicyTest_BlockRule` | exact — add one new function |
| `internal/check/handler_test.go` | test | — | existing `TestCatalogMatchWarns` + `buildTestIndex` helper | exact — add new `Test*` functions |

---

## Pattern Assignments

### `internal/policy/types.go` (model, transform)

**Analog:** same file — existing `CorroborationThresholds` struct (lines 90–104)

**Existing struct to extend** (lines 90–104):
```go
type CorroborationThresholds struct {
    WarnAt      int // minimum signed-source count for warn level (default 1)
    BlockAt     int // minimum signed-source count for block level (default 2)
    QuarantineAt int // minimum signed-source count for block+quarantine (default 3)
}

func DefaultCorroborationThresholds() CorroborationThresholds {
    return CorroborationThresholds{
        WarnAt:       1,
        BlockAt:      2,
        QuarantineAt: 3,
    }
}
```

**New type to add** (insert before `CorroborationThresholds`):
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

**Fields to add to `CorroborationThresholds`** (after `QuarantineAt`):
```go
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
// Default: true (caller sets false when SourceState.Degraded is true).
CatalogHealthy bool
```

**Updated `DefaultCorroborationThresholds()`** (replace in-place):
```go
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

**Pattern notes:**
- Place `SeverityThreshold` type immediately before `CorroborationThresholds` — same proximity pattern as `CatalogMatch` → `Decision` ordering in the file.
- `CatalogHealthy bool` default `true`: mirrors the "healthy unless proven degraded" principle used by `SourceState.Degraded` (state.go:29 — `Degraded` starts false).

---

### `internal/policy/corroboration.go` (utility/pure, transform)

**Analog:** same file — `validateCorroborationThresholds` (lines 32–40) and `corroborate()` escalation table (lines 97–107)

**Existing `validateCorroborationThresholds` to extend** (lines 32–40):
```go
func validateCorroborationThresholds(t CorroborationThresholds) error {
    if t.WarnAt > t.BlockAt {
        return fmt.Errorf("corroboration: WarnAt (%d) must be <= BlockAt (%d)", t.WarnAt, t.BlockAt)
    }
    if t.BlockAt > t.QuarantineAt {
        return fmt.Errorf("corroboration: BlockAt (%d) must be <= QuarantineAt (%d)", t.BlockAt, t.QuarantineAt)
    }
    return nil
}
```

**New bounds-check block to append** (after the existing two checks, before `return nil`):
```go
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
```

**New `findSeverityOverride` helper** (insert after `validateCorroborationThresholds`, before `corroborate`):
```go
// findSeverityOverride returns the most-restrictive SeverityThreshold override
// from t.SeverityOverrides that applies to any match, or nil when:
//   - CatalogHealthy is false (sanity gate: degraded catalog suppresses escalation)
//   - all non-dissented matches have Version == "*" (all-versions guard)
//   - no match severity is in SeverityOverrides
//
// "Most restrictive" means the override with the lowest BlockAt.
// Pure: reads only matches, overrides map, and the healthy flag — no I/O.
// Imports: only "fmt" and "sort" (existing) — never add "os", "net", etc.
func findSeverityOverride(
    matches []CatalogMatch,
    overrides map[string]SeverityThreshold,
    catalogHealthy bool,
) *SeverityThreshold {
    if !catalogHealthy {
        return nil  // CORR-02: sanity gate
    }
    if len(overrides) == 0 {
        return nil
    }

    // CORR-02 all-versions guard: if ANY non-dissented match has Version == "*",
    // do not escalate. A mis-tagged wildcard entry must never block at single-source.
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

**Existing escalation table to replace** (lines 97–107):
```go
// Escalation decision table (PLCY-01).
switch {
case signedCount >= t.QuarantineAt && hasSignedSource:
    return "block", true, signedCount, agreedList, dissentList
case signedCount >= t.BlockAt && hasSignedSource:
    return "block", false, signedCount, agreedList, dissentList
case signedCount >= t.WarnAt || hasUnsigned:
    return "warn", false, signedCount, agreedList, dissentList
default:
    return "allow", false, signedCount, agreedList, dissentList
}
```

**New escalation table** (replace lines 97–107 with):
```go
// CORR-01/02: check for per-severity threshold override.
// findSeverityOverride returns nil when: catalog is degraded (CatalogHealthy=false),
// any non-dissented match is an all-versions wildcard (Version=="*"), or no severity
// matches SeverityOverrides keys.
effectiveBlockAt := t.BlockAt
effectiveQuarantineAt := t.QuarantineAt
if ov := findSeverityOverride(matches, t.SeverityOverrides, t.CatalogHealthy); ov != nil {
    effectiveBlockAt = ov.BlockAt
    effectiveQuarantineAt = ov.QuarantineAt
}

// Escalation decision table (PLCY-01 + CORR-01 severity override).
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

**Import constraint:** `corroboration.go` currently imports only `"fmt"` and `"sort"` (line 3–6). After Phase 6 it must still import only `"fmt"` and `"sort"`. `TestCorroborationImportsArePure` (lines 220–256) enforces this — do NOT add `"internal/catalog"` or any I/O package.

---

### `internal/policy/engine.go` (utility/pure, transform)

**Analog:** same file — `Evaluate()` signature at line 64

**Existing signature** (line 64):
```go
func Evaluate(tc ToolCall, idx MultiCatalogLookup, t CorroborationThresholds, ac AgentContext) Decision {
```

**No signature change required.** `CatalogHealthy` is a field on `CorroborationThresholds` — callers set it before passing the struct. `Evaluate` passes `t` to `corroborate(matches, t)` at line 138. No change to `engine.go` body is needed; the threading is done at call sites.

**Cross-check:** `selftest.go:121` calls `policy.DefaultCorroborationThresholds()` directly. After Phase 6, defaults include `CatalogHealthy: true` and `SeverityOverrides["critical"]`. Executor must audit selftest fixtures for `Severity: "critical"` entries with `ExpectLevel: "warn"` — those expectations may need updating to `"block"` (see Research open question A2).

---

### `internal/policyloader/loader.go` (model, transform)

**Analog:** same file — existing `PolicyRule` struct fields `WarnAt`, `BlockAt`, `QuarantineAt` (lines 46–48)

**Existing fields pattern** (lines 46–48):
```go
WarnAt       int `json:"warn_at,omitempty"`       // for corroboration_threshold rules
BlockAt      int `json:"block_at,omitempty"`      // for corroboration_threshold rules
QuarantineAt int `json:"quarantine_at,omitempty"` // for corroboration_threshold rules
```

**New field to add** (after `QuarantineAt`, same style):
```go
// CriticalBlockAt sets the minimum signed-source count to block at
// "critical" severity. Extends the corroboration_threshold rule type (CORR-01).
// Valid range: 1 <= CriticalBlockAt <= block_at.
// Zero means "use default" (not "block unconditionally").
CriticalBlockAt int `json:"critical_block_at,omitempty"`
```

**JSON compatibility:** `LoadPolicyFile` uses `json.Decoder` with `DisallowUnknownFields` (line 81). Adding `CriticalBlockAt` to `PolicyRule` makes `critical_block_at` a known field — existing policy files without it remain valid (field zero-values). No `schema_version` bump required.

---

### `internal/policyloader/validate.go` (utility, transform)

**Analog:** same file — `ValidateSchema` rule loop (lines 47–56)

**Existing rule loop** (lines 47–56):
```go
for i, r := range pf.Rules {
    if !legalRuleTypes[r.RuleType] {
        errs = append(errs, fmt.Errorf("rule[%d] %q: unknown rule_type %q",
            i, r.ID, r.RuleType))
    }
    if r.Action != "" && !legalActions[r.Action] {
        errs = append(errs, fmt.Errorf("rule[%d] %q: invalid action %q (want \"block\", \"warn\", or \"allow\")",
            i, r.ID, r.Action))
    }
}
```

**Additional check to insert** (inside the same `for i, r := range pf.Rules` loop, after the existing `Action` check — same error-accumulation pattern):
```go
if r.RuleType == "corroboration_threshold" && r.CriticalBlockAt != 0 {
    // CriticalBlockAt zero means "use default" — only validate non-zero values.
    if r.CriticalBlockAt < 1 {
        errs = append(errs, fmt.Errorf("rule[%d] %q: critical_block_at (%d) must be >= 1",
            i, r.ID, r.CriticalBlockAt))
    }
    // Upper bound check (CriticalBlockAt <= global BlockAt) is done at eval time by
    // validateCorroborationThresholds, which has the fully-resolved global BlockAt.
}
```

**Pattern note:** `legalRuleTypes["corroboration_threshold"]` already exists (line 19) — no new enum entry needed.

---

### `internal/policyloader/test.go` (`ThresholdsFromPolicyFiles`, bridge utility)

**Analog:** same file — `ThresholdsFromPolicyFiles` inner loop (lines 64–79)

**Existing inner loop** (lines 64–79):
```go
func ThresholdsFromPolicyFiles(files []PolicyFile) policy.CorroborationThresholds {
    t := policy.DefaultCorroborationThresholds()
    for _, pf := range files {
        for _, r := range pf.Rules {
            if r.RuleType != "corroboration_threshold" {
                continue
            }
            if r.WarnAt > 0 {
                t.WarnAt = r.WarnAt
            }
            if r.BlockAt > 0 {
                t.BlockAt = r.BlockAt
            }
            if r.QuarantineAt > 0 {
                t.QuarantineAt = r.QuarantineAt
            }
        }
    }
    return t
}
```

**Additional block to add** (inside the inner loop, after the `QuarantineAt` check — mirrors the `if r.WarnAt > 0` / `if r.BlockAt > 0` pattern exactly):
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

**Also extend `thresholdsFromPolicyFile`** (unexported, lines 28–49): add the identical `CriticalBlockAt` block to the inner loop. This ensures `RunPolicyTest` (which uses `thresholdsFromPolicyFile`) is also Phase-6-aware.

---

### Call-Site Wiring: All Four `ThresholdsFromPolicyFiles` / `policy.Evaluate` Sites

Each call site follows the same pattern: read the pre-existing `thresholds` line, add one line after it that calls the `resolveCatalogHealthy` helper and sets `thresholds.CatalogHealthy`.

#### `resolveCatalogHealthy` helper (new, shared pattern)

Place one copy in each consumer package's existing utility/sanity file (or a new `sanity.go` with just this function). Pattern: read `catalog.LoadState`, check `SourceState.Degraded`, default to `true` on any error.

**Imports required** (same packages the caller already uses — `catalog` is already imported at all four sites):
```go
import (
    "path/filepath"
    "github.com/bantuson/beekeeper/internal/catalog"
)
```

**Helper body** (identical across all four packages):
```go
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

**Key `catalog.LoadState` facts** (verified from `internal/catalog/state.go`):
- Signature: `LoadState(path string) (WatchState, error)` — takes a **full path**, not a directory.
- Missing file returns `WatchState{Sources: make(map[string]SourceState)}, nil` (not an error).
- `WatchState.Sources["bumblebee"].Degraded bool` — the pre-computed flag written by the watch daemon at `catalog/watch.go:177–186`.
- State path derivation: `filepath.Join(filepath.Dir(cacheDir), "state.json")` — `cacheDir` is `~/.beekeeper/catalogs`; state.json lives at `~/.beekeeper/state.json`.

#### `internal/check/handler.go` (line 246 context)

**Existing call site** (lines 246–251):
```go
// Derive corroboration thresholds from loaded policy files. Falls back to
// PLCY-01 defaults when no policy file sets a threshold field.
thresholds := policyloader.ThresholdsFromPolicyFiles(policyFiles)

// Pure, synchronous policy evaluation (no I/O, no goroutines).
decision := policy.Evaluate(toolCall, multiIdx, thresholds, ac)
```

**New line to insert** (between `thresholds :=` and `decision :=`):
```go
// CORR-02: thread catalog sanity state into thresholds.
// Reads SourceState.Degraded from state.json — written by the watch daemon.
thresholds.CatalogHealthy = resolveCatalogHealthy(cacheDir)
```

The variable `cacheDir` is already in scope at this point (it is the `cacheDir` parameter to `runCheck`).

#### `internal/gateway/policy.go` (line 119 context)

**Existing call site** (lines 119–121):
```go
thresholds := policyloader.ThresholdsFromPolicyFiles(policyFiles)

decision := policy.Evaluate(tc, idx, thresholds, ac)
```

**New line to insert**:
```go
thresholds.CatalogHealthy = resolveCatalogHealthy(cfg.CacheDir)
```

`cfg.CacheDir` is accessible in the `applyPolicy` function (confirmed: `cfg.CacheDir` used on line 103).

#### `internal/watch/handler.go` (line 132 context)

**Existing call site** (lines 132–143):
```go
thresholds := policyloader.ThresholdsFromPolicyFiles(policyFiles)

// 7. Catalog evaluation using policy-file-derived thresholds.
tc := policy.ToolCall{ ... }
catalogDecision := policy.Evaluate(tc, multiIdx, thresholds, policy.AgentContext{})
```

**New line to insert** (after `thresholds :=`):
```go
thresholds.CatalogHealthy = resolveCatalogHealthy(h.CacheDir)
```

`h.CacheDir` is the handler struct field used at line 128 (`policiesDir := filepath.Join(filepath.Dir(h.CacheDir), "policies")`).

#### `internal/scan/scanner.go` (line 280 context)

**Existing call site** (lines 280–290):
```go
thresholds := policyloader.ThresholdsFromPolicyFiles(policyFiles)

tc := policy.ToolCall{ ... }
catalogDecision := policy.Evaluate(tc, multiIdx, thresholds, policy.AgentContext{})
```

**New line to insert**:
```go
thresholds.CatalogHealthy = resolveCatalogHealthy(cfg.CacheDir)
```

`cfg.CacheDir` is in scope (used at line 276 for `policiesDir`).

---

## Test File Patterns

### `internal/policy/corroboration_test.go` (extend existing)

**Analog:** existing tests in same file (lines 11–218)

**Test function signature pattern** (copy from `TestCorroborationOneSignedSource`, lines 32–52):
```go
func TestCorroborationXxx(t *testing.T) {
    matches := []CatalogMatch{
        {CatalogSource: "bumblebee", Severity: "critical", Signed: false},
        {CatalogSource: "osv", Severity: "unknown", Signed: true},
    }
    level, quarantine, count, agreed, _ := corroborate(matches, DefaultCorroborationThresholds())
    if level != "block" {
        t.Errorf("level = %q, want %q ...", level, "block")
    }
    ...
}
```

**Import pattern** (lines 1–8 — no new imports required for new corroboration tests):
```go
package policy

import (
    "go/parser"
    "go/token"
    "os"
    "testing"
)
```

**New tests to add** (Wave 0 gaps per 06-RESEARCH.md):
- `TestCorroborationShaiHuludCriticalBlock` — bumblebee(unsigned,critical)+OSV(signed,unknown) → block
- `TestCorroborationDegradedCatalogNoEscalation` — same matches, `CatalogHealthy=false` → warn
- `TestCorroborationAllVersionsCriticalWildcardStaysWarn` — Version="*" matches → warn
- `TestValidateCorroborationThresholdsRejectsBlockAtZero` — BlockAt=0 override → error + fail-closed block
- `TestValidateCorroborationThresholdsRejectsLooserOverride` — override BlockAt > global BlockAt → error
- `TestDefaultThresholdsIncludeSeverityOverrides` — assert defaults have `SeverityOverrides["critical"]` + `CatalogHealthy:true`

**Full code for the six new tests** is in 06-RESEARCH.md `## Code Examples` section — use those verbatim as the base.

**Table-driven variant** (for `TestCorroborationSeverityOverrideTable` — SC5 coverage): follow the `TestCorroborationXxx` flat-field approach rather than sub-tests; the existing test file uses flat functions, not `t.Run`. For SC5 a single table-driven test using `for _, tc := range []struct{...}{...}` with `t.Run(tc.name, ...)` is acceptable and preferred for the 6+ scenarios.

---

### `internal/policyloader/test_test.go` (extend existing)

**Analog:** existing `TestThresholdsFromPolicyFile` (lines 119–145) and `TestPolicyTest_BlockRule` (lines 42–83)

**New test to add** — `TestThresholdsFromPolicyFilesCriticalBlockAt`:
```go
func TestThresholdsFromPolicyFilesCriticalBlockAt(t *testing.T) {
    pf := PolicyFile{
        SchemaVersion: "1",
        Name:          "critical-block-policy",
        Rules: []PolicyRule{
            {
                ID:              "CORR-01",
                RuleType:        "corroboration_threshold",
                CriticalBlockAt: 1,
            },
        },
    }

    thresholds := ThresholdsFromPolicyFiles([]PolicyFile{pf})

    if thresholds.SeverityOverrides == nil {
        t.Fatal("SeverityOverrides is nil, want non-nil map")
    }
    ov, ok := thresholds.SeverityOverrides["critical"]
    if !ok {
        t.Fatal("SeverityOverrides[\"critical\"] not set")
    }
    if ov.BlockAt != 1 {
        t.Errorf("BlockAt = %d, want 1", ov.BlockAt)
    }
    if ov.QuarantineAt < 1 {
        t.Errorf("QuarantineAt = %d, want >= 1 (default: CriticalBlockAt+1)", ov.QuarantineAt)
    }
}
```

**Import pattern** (lines 1–8 — already correct, no new imports):
```go
package policyloader

import (
    "path/filepath"
    "testing"

    "github.com/bantuson/beekeeper/internal/policy"
)
```

---

### `internal/check/handler_test.go` (extend existing)

**Analog:** existing `buildTestIndex` helper (lines 22–39) and `TestCatalogMatchWarns` (lines 67–87)

**Index builder pattern for ai-figure fixture** (extend `buildTestIndex` or add a parallel helper):
```go
// buildCriticalTestIndex writes an index containing an unsigned critical-severity
// bumblebee entry (simulates the ai-figure/Shai-Hulud scenario for CORR-01 tests).
func buildCriticalTestIndex(t *testing.T, dir string) string {
    t.Helper()
    entries := []catalog.Entry{
        {
            ID:            "beekeeper-test-critical-unsigned",
            Name:          "test critical unsigned entry",
            Ecosystem:     "npm",
            Package:       "ai-figure-test",
            Versions:      []string{"1.0.0"},
            Severity:      "critical",
            CatalogSource: "bumblebee",
            // CatalogSignature deliberately empty → Signed:false in adapter
        },
    }
    idxPath := filepath.Join(dir, "bumblebee.idx")
    if err := catalog.BuildIndex(idxPath, entries); err != nil {
        t.Fatalf("BuildIndex: %v", err)
    }
    return idxPath
}
```

**New integration tests to add**:
- `TestRunCheckAiFigureBlocks` — critical unsigned bumblebee + OSV signed → exit 1, level "block"
- `TestRunCheckCriticalDegradedCatalogWarn` — same catalog, CatalogHealthy=false (degrade state) → warn
- `TestRunCheckCriticalBlockWithHealthyCatalog` — `CatalogHealthy` threaded at handler call site

**Note:** Integration tests in `handler_test.go` use `RunCheck(ctx, stdin, cfg, idxPath, auditPath, cacheDir)`. To simulate degraded state, write a `state.json` with `Sources["bumblebee"].Degraded=true` into the `cacheDir` parent before calling `RunCheck`. The `resolveCatalogHealthy` helper reads `filepath.Dir(cacheDir)/state.json`.

**Degraded state setup pattern** (based on `catalog.SaveState`):
```go
stateDir := filepath.Dir(cacheDir)  // e.g. t.TempDir()
statePath := filepath.Join(stateDir, "state.json")
err := catalog.SaveState(statePath, catalog.WatchState{
    Sources: map[string]catalog.SourceState{
        "bumblebee": {Degraded: true, DegradedReason: "test: injected degradation"},
    },
})
```

---

## Shared Patterns

### Caller-Resolved I/O Pattern (most important for Phase 6)

**Source:** `internal/check/handler.go` lines 246–251 + `internal/policy/release_age.go` (the `ReleaseAgeInput{AgeMinutes int64}` precedent)

**Apply to:** All four call sites (`handler.go`, `gateway/policy.go`, `watch/handler.go`, `scan/scanner.go`)

**Rule:** The pure engine receives a pre-resolved `bool` (`CatalogHealthy`). The I/O caller reads `catalog.LoadState` and sets `thresholds.CatalogHealthy = resolveCatalogHealthy(cacheDir)`. The pure engine never imports `internal/catalog`.

**Example from existing code** (lines 246–251 of `handler.go`):
```go
thresholds := policyloader.ThresholdsFromPolicyFiles(policyFiles)
// ← insert: thresholds.CatalogHealthy = resolveCatalogHealthy(cacheDir)
decision := policy.Evaluate(toolCall, multiIdx, thresholds, ac)
```

### Fail-Closed on Misconfiguration

**Source:** `internal/policy/corroboration.go` lines 42–46

**Apply to:** `validateCorroborationThresholds` extension + all new bounds checks

**Pattern:**
```go
if err := validateCorroborationThresholds(t); err != nil {
    // Misconfigured thresholds — fail closed to block.
    return "block", false, 0, nil, nil
}
```

All new override bounds violations must follow this same pattern: non-nil error from `validateCorroborationThresholds` → `corroborate` immediately returns `"block"`.

### Non-Zero Override Pattern

**Source:** `internal/policyloader/test.go` lines 36–48 — existing `if r.WarnAt > 0` / `if r.BlockAt > 0` / `if r.QuarantineAt > 0` checks

**Apply to:** `CriticalBlockAt` in `ThresholdsFromPolicyFiles` and `thresholdsFromPolicyFile`

**Rule:** Zero means "use default" for all threshold fields. Never write `if r.CriticalBlockAt != 0` with a zero-means-block semantic — the `>= 1` enforcement is in `validateCorroborationThresholds` and `ValidateSchema`, not in the threshold merge.

### Error Accumulation (ValidateSchema)

**Source:** `internal/policyloader/validate.go` lines 39–58 — `errs = append(errs, ...)` pattern

**Apply to:** New `CriticalBlockAt` validation in `ValidateSchema`

**Rule:** Always `append`, never early-return. Return all errors together so `policy validate` shows the complete picture.

### Purity Test (import guard)

**Source:** `internal/policy/corroboration_test.go` lines 220–256 — `TestCorroborationImportsArePure`

**Apply to:** No new imports may be added to `corroboration.go`. The existing test already enforces this. New `findSeverityOverride` uses only the types already in scope (`CatalogMatch`, `SeverityThreshold`) — no new imports needed.

---

## No Analog Found

All Phase 6 files have strong analogs. No files require falling back to research-only patterns.

---

## Metadata

**Analog search scope:** `internal/policy/`, `internal/policyloader/`, `internal/check/`, `internal/gateway/`, `internal/watch/`, `internal/scan/`, `internal/catalog/`
**Files read for pattern extraction:** 13 source files, 3 test files
**Pattern extraction date:** 2026-06-03
