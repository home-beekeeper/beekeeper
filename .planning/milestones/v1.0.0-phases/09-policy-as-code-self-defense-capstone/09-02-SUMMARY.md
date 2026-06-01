---
phase: 09-policy-as-code-self-defense-capstone
plan: "02"
subsystem: config
tags: [go, config, layered-config, env-vars, five-layer-merge, self-catalog]

# Dependency graph
requires:
  - phase: 09-01
    provides: Phase 9 foundation; policyloader package skeleton
  - phase: 01-foundation
    provides: internal/config/config.go with Load, Config struct, FailMode constants
provides:
  - LoadLayered: five-layer config merge (system→user→project→env→flags)
  - LayerOpts: options struct for all five layers
  - merge: zero-value-safe Config merge function
  - applyEnvVars: BEEKEEPER_* env var mapping (five known vars only; no reflective apply)
  - applyFlagOverrides: CLI flag overlay (flags win over env)
  - SelfCatalogConfig: URL + PubKey fields on Config (consumed by Plans 03, 05)
affects: [09-03, 09-05, cmd-beekeeper-main, check-handler, gateway-policy]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "loadIfPresent: raw JSON parse without FailMode defaults for optional layers, enabling zero-value-safe merge"
    - "src-wins-if-non-zero merge: each field conditional; absent higher-layer field does not reset lower-layer value"
    - "Hardcoded BEEKEEPER_* env var mapping: only five known vars mapped; unknown vars ignored (T-09-05)"
    - "loadIfPresent vs Load separation: Load sets FailMode defaults (for single-file use); loadIfPresent does not (for layered use)"

key-files:
  created:
    - internal/config/layered.go
    - internal/config/layered_test.go
  modified:
    - internal/config/config.go

key-decisions:
  - "Optional layers (system, project) use loadIfPresent with raw JSON parse — Load's FailMode default fills absent FailMode='closed', which would incorrectly override lower-layer non-closed values; raw parse leaves FailMode='' so merge skips it"
  - "User layer uses Load (not loadIfPresent) because it is the authoritative baseline and its absent-file-as-defaults behaviour is correct for the user-config contract"
  - "LlamaFirewall.Enabled bool merge: src wins if src.Enabled=true OR if any other sidecar field is non-zero; this is the documented limitation for distinguishing explicit-false from absent for plain bool fields without pointer wrappers"
  - "Five BEEKEEPER_* vars only: BEEKEEPER_FAIL_MODE, BEEKEEPER_SOCKET_API_TOKEN, BEEKEEPER_LLAMAFIREWALL_ENABLED, BEEKEEPER_AUDIT_SINKS (comma-split), BEEKEEPER_SELF_CATALOG_URL — no reflective application (T-09-05 mitigated)"
  - "validate() reuses FailMode switch from Load; invalid merged fail_mode returns error not silent default (T-09-08 mitigated)"

patterns-established:
  - "loadIfPresent: use for optional layers where raw (non-defaulted) parse is needed"
  - "Five-layer merge order: Config{FailModeClosed} → system → user → project → env → flags → validate"
  - "FlagOverrides map[string]string: logical keys are BEEKEEPER_ suffix lower-cased"

requirements-completed: [CODE-05]

# Metrics
duration: 25min
completed: 2026-05-29
---

# Phase 9 Plan 02: Layered Config Merge Summary

**Five-layer config merge (CODE-05): LoadLayered with zero-value-safe merge, five BEEKEEPER_* env vars, and SelfCatalogConfig struct for beekeeper-self feed overrides**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-05-29T00:00:00Z
- **Completed:** 2026-05-29T00:25:00Z
- **Tasks:** 2 (Task 1: SelfCatalogConfig + LoadLayered skeleton + merge; Task 2: applyEnvVars + applyFlagOverrides)
- **Files modified:** 3

## Accomplishments

- `LoadLayered(opts LayerOpts) (Config, error)` implements full five-layer precedence: system → user → project → BEEKEEPER_* env → CLI flags
- Zero-value-safe `merge` function: a partial higher-layer config (e.g. project with only one field) cannot reset lower-layer non-zero values; specifically, an absent `fail_mode` in the project file leaves the user's `fail_mode:"open"` intact
- `SelfCatalogConfig{URL, PubKey}` added to `Config` with `SelfCatalog` field — Plans 03 and 05 consume these for beekeeper-self feed location and signature verification key override
- `applyEnvVars` maps exactly five BEEKEEPER_* vars; unknown env vars and non-BEEKEEPER_ vars are silently ignored (T-09-05 mitigated)
- `validate()` rejects invalid merged `fail_mode` rather than silently defaulting (T-09-08 mitigated)
- 12 passing tests covering all five matrix rows from 09-RESEARCH.md, all BEEKEEPER_* vars, flag-over-env precedence, and zero-value preservation

## Task Commits

1. **Task 1 + Task 2: SelfCatalogConfig + LoadLayered five-layer merge** - `b86914e` (feat)

## Files Created/Modified

- `internal/config/config.go` — Added `SelfCatalogConfig` struct and `SelfCatalog` field on `Config`
- `internal/config/layered.go` — New file: `LayerOpts`, `LoadLayered`, `merge`, `mergeAudit`, `mergeLlamaFirewall`, `applyEnvVars`, `applyFlagOverrides`, `validate`, `loadIfPresent`, `unmarshalConfig`, `parseEnvSlice`
- `internal/config/layered_test.go` — New file: 12 tests covering precedence matrix, missing optional layers, zero-value preservation, env var mapping (all five BEEKEEPER_* vars), flag-over-env, unknown env ignored

## Decisions Made

1. **loadIfPresent uses raw JSON parse** — The existing `Load` function fills absent `FailMode` with `"closed"`. If optional layers (system, project) were loaded via `Load`, a project file containing only `{"redact_patterns":["X"]}` would have `FailMode="closed"` applied by `Load`, overwriting the user's `FailMode="open"` in merge. Solution: `loadIfPresent` parses JSON without applying defaults, leaving `FailMode=""` (zero value) so `merge` skips it.

2. **User layer still uses `Load`** — The user layer is the authoritative baseline; Load's absent-file-as-defaults contract (`Config{FailMode:"closed"}, nil`) is the correct behaviour for the user layer.

3. **LlamaFirewall.Enabled merge heuristic** — `bool` fields cannot distinguish "absent" from "explicitly false" via JSON decode into a plain struct. The chosen approach: src's `Enabled` field is applied only when `src.Enabled == true` OR when another LlamaFirewall field is non-zero (indicating the user configured the sidecar block). This avoids a pointer wrapper in the Config struct while preventing an accidental disable. Documented limitation: a project that wants to disable LlamaFirewall without setting any other sidecar field cannot do so via the project config layer alone (must use env var or flag).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] loadIfPresent must not apply FailMode defaults**
- **Found during:** Task 1 (running TestLoadLayered_MissingOptionalLayers and TestMerge_ZeroValuePreservation)
- **Issue:** Initial implementation called `Load(opts.SystemPath/ProjectPath)` for optional layers. Since `Load` fills absent `FailMode` with `"closed"`, a project file with no `fail_mode` field got `FailMode="closed"` applied, then merge overwrote the user's `fail_mode:"open"` with `"closed"`.
- **Fix:** Replaced `Load` with `loadIfPresent` for optional layers. `loadIfPresent` reads and parses JSON directly without applying the `FailMode="closed"` default, leaving zero values as zero so merge can skip them.
- **Files modified:** `internal/config/layered.go`
- **Verification:** `TestMerge_ZeroValuePreservation` and `TestLoadLayered_MissingOptionalLayers` both pass.
- **Committed in:** `b86914e`

---

**Total deviations:** 1 auto-fixed (Rule 1 — bug in optional layer loading)
**Impact on plan:** Required fix; the entire zero-value-preservation guarantee depended on it. No scope creep.

## Issues Encountered

- None beyond the auto-fixed bug above.

## Threat Surface Scan

| Flag | File | Description |
|------|------|-------------|
| T-09-05 mitigated | `internal/config/layered.go` | `applyEnvVars` only maps hardcoded known BEEKEEPER_* vars; no reflective application; unknown vars silently ignored |
| T-09-07 mitigated | `internal/config/layered.go` | `merge` applies src only when non-zero; partial project layer cannot silently downgrade FailMode |
| T-09-08 mitigated | `internal/config/layered.go` | `validate()` reuses Load's FailMode switch; invalid merged fail_mode errors rather than defaulting |

No new unmitigated threat surfaces introduced.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `LoadLayered` and `SelfCatalogConfig` are ready for Plan 03 (beekeeper-self catalog client) and Plan 05 (self-quarantine logic)
- Plan 04 (`beekeeper diag`) can call `LoadLayered` with platform paths for full five-layer config resolution
- `cmd/beekeeper/main.go` can centralize config loading in `PersistentPreRunE` using `LoadLayered` in a future plan
- All 12 tests green; `go vet ./internal/config/...` clean

---
*Phase: 09-policy-as-code-self-defense-capstone*
*Completed: 2026-05-29*
