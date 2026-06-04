---
phase: 08-package-manager-nudge-behavioral-test-suite
plan: 04
subsystem: nudge
tags: [nudge, detection, impure-adapter, scanners, fuzz, cache, fail-open, BTEST-03, NUDGE-02]

# Dependency graph
requires:
  - phase: 08-03
    provides: "PMState, Config, DefaultConfig, meetsFloor, minimumReleaseAgeWeaknessBaseline — all consumed by detect.go and scanners.go"
provides:
  - "nudge.DetectState(ctx, cfg) PMState — 2s-timeout exec, fail-open on error/timeout, integrates scanner results"
  - "var nudge.DetectStateFn = DetectState — exported cross-package injection seam for check adapter (Plan 06) and Plan 07 behavioral tests"
  - "nudge.NewCache(detectFn, ttl) *Cache — gateway-only 60s TTL memoization with injectable clock"
  - "nudge.DetectBunScanner(paths) bool + nudge.DetectPnpmHardening(path) HardeningResult — injectable readFileFn scanners"
  - "nudge.scanBunfig(content) (bool, bool) + nudge.scanPnpmWorkspace(content) (int, bool, bool, bool, bool) — pure string scanners, never panic"
  - "FuzzBunfig + FuzzPnpmWorkspace — BTEST-03 release-gate fuzz targets, all seeds pass"
affects:
  - "08-06 (gateway/policy.go wraps DetectStateFn in Cache; drift.go calls IsMajorDrift)"
  - "08-07 (check/handler.go nudge wiring calls nudge.DetectStateFn fresh per invocation)"
  - "08-08 (integration_test.go + e2e_test.go call DetectStateFn via the check hook path)"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Injected-fn idiom for testable exec (mirrors shim.go osLookPath): pnpmVersionFn/bunVersionFn/nodeVersionFn are package-level vars; tests substitute slow/erroring fakes"
    - "Fail-open-by-design documented contract: detection timeout/error -> treat PM as not installed, never block agent (distinct from catalog/path fail-closed)"
    - "Exported var DetectStateFn = DetectState cross-package seam: check adapter calls this; Plan 07 tests swap it with defer-restore; unexported version-fns stay internal"
    - "Gateway-only Cache with injectable clock: newCacheWithClock() for TTL tests; NewCache() for production; NEVER in check hook (Flag 2 Position B)"
    - "Hand-written line scanners (no TOML/YAML dep): scanBunfig + scanPnpmWorkspace; return (value, ok) never panic on any input"
    - "1440-minute weakness baseline (Flag 5 correction, not 60); minimumReleaseAge=0 -> WeaknessLogged=true, Hardened stays true (§10-16)"
    - "Injectable readFileFn = os.ReadFile for DetectBunScanner/DetectPnpmHardening; tests supply fake content without disk I/O"

key-files:
  created:
    - internal/nudge/detect.go
    - internal/nudge/detect_test.go
    - internal/nudge/scanners.go
    - internal/nudge/scanners_test.go
    - internal/nudge/scanners_fuzz_test.go
  modified: []

key-decisions:
  - "DetectState integrates DetectPnpmHardening for the pnpm branch: when version meets floor, workspace hardening is checked; weakness is logged to HardeningResult.WeaknessLogged but Hardened stays true (§10-16 semantics applied consistently here and in scanners.go)"
  - "newCacheWithClock is unexported (test-only helper) alongside the exported NewCache; this avoids polluting the public API while keeping the TTL test injectable"
  - "bunfigPaths() + pnpmWorkspacePath() use os.Getwd() + os.UserHomeDir() which can fail gracefully — empty paths are safe-default-handled by DetectBunScanner/DetectPnpmHardening"
  - "scanBunfig returns ok=true when scanner entry found before a later malformed header (early return on success); test documented this behavior explicitly"
  - "parseInt is a private stdlib-free implementation (no strconv) to keep the file dependency surface minimal; handles negative numbers safely"

requirements-completed: [NUDGE-02, BTEST-03]

# Metrics
duration: 35min
completed: 2026-06-04
---

# Phase 8 Plan 04: Impure Detection Adapter + Scanners Summary

**DetectState (2s timeout, fail-open) + exported DetectStateFn seam + gateway-only Cache + hand-written bunfig.toml/pnpm-workspace.yaml scanners + FuzzBunfig/FuzzPnpmWorkspace release-gate fuzz targets**

## Performance

- **Duration:** ~35 min
- **Started:** 2026-06-04T00:00:00Z
- **Completed:** 2026-06-04T00:35:00Z
- **Tasks:** 2 (Task 1: scanners; Task 2: detect)
- **Files created:** 5

## Accomplishments

- `detect.go` is the ONLY nudge file that imports os/exec/context/time/sync — the pure/impure boundary is clean
- `DetectState` runs `pnpm/bun/node --version` each with a 2s hard timeout via `exec.CommandContext`; on timeout/error the PM is treated as "not installed" (fail-open by design, documented in the DetectState doc comment — §10-12, T-08-11)
- `var DetectStateFn = DetectState` is exported with a clear doc comment explaining the cross-package seam contract; unexported `pnpmVersionFn`/`bunVersionFn`/`nodeVersionFn` remain the internal default implementation unreachable from other packages (T-08-10b)
- Cache: injectable clock via `newCacheWithClock`; `NewCache` for production gateway use; `Cache.State` with mutex; TTL test proves §10-11 with an injected counting detect-fn + fake clock
- `scanBunfig`: hand-written line scanner detecting `[install.security]` + `scanner = "@socketsecurity/bun-security-scanner"`; tolerates quoting/whitespace variants; never panics; ok=false on malformed structure (§10-13)
- `scanPnpmWorkspace`: hand-written key scanner for `minimumReleaseAge` and `blockExoticSubdeps`; never panics; ok=false on non-integer or unknown boolean values (§10-13)
- `DetectPnpmHardening`: 1440-minute weakness baseline (Flag 5 correction); `minimumReleaseAge=0` → WeaknessLogged=true but Hardened stays true (§10-16); file-absent → Hardened=true (pnpm 11 defaults)
- `FuzzBunfig` + `FuzzPnpmWorkspace`: `//go:build fuzz` + RELEASE-GATE header; seed corpus covers empty/truncated/huge/non-UTF8/metacharacter/malformed inputs; all seeds pass with no panic (BTEST-03)
- No TOML/YAML library dependency added (REQUIREMENTS Out-of-Scope honored)

## Task Commits

1. **Task 1: scanners.go + scanners_test.go + scanners_fuzz_test.go** — `3a4b612` (feat)
2. **Task 2: detect.go + detect_test.go** — `ba87c6f` (feat)

## Files Created

- `internal/nudge/scanners.go` — `scanBunfig`, `scanPnpmWorkspace`, `DetectBunScanner`, `DetectPnpmHardening`, `HardeningResult`, injectable `readFileFn`
- `internal/nudge/scanners_test.go` — table-driven tests for all scanner behaviors, weakness baseline, file-read injection, §10-13 never-panic cases
- `internal/nudge/scanners_fuzz_test.go` — `FuzzBunfig` + `FuzzPnpmWorkspace` release-gate fuzz targets (`//go:build fuzz`)
- `internal/nudge/detect.go` — `DetectState`, `DetectStateFn` seam, `Cache` + `NewCache` + `newCacheWithClock`, injectable version-fns, `pnpmWorkspacePath` + `bunfigPaths`
- `internal/nudge/detect_test.go` — timeout fallback, error fallback, good versions, floor-not-met, scanner check, DetectStateFn swap + defer-restore, Cache TTL with injected clock

## Decisions Made

- `DetectState` integrates `DetectPnpmHardening` inline: when pnpm version meets the floor, the workspace file is checked; `HardeningResult.WeaknessLogged` is available for the audit layer but does not flip `PnpmHardened` (§10-16 applied consistently with scanners.go).
- `newCacheWithClock` is unexported (test-only) to keep the exported API surface minimal while enabling the §10-11 TTL test with an injected clock.
- `parseInt` is a private hand-rolled implementation (no `strconv`) so `scanners.go` imports only `os` and `strings` — consistent with the minimal-dep philosophy for a security tool.

## Deviations from Plan

### Auto-fixed Issues

**[Rule 1 - Bug] Test case for scanBunfig malformed header had wrong expectation**

- **Found during:** Task 1 test run
- **Issue:** Test case "section header then malformed one" expected `ok=false` for a file where the scanner entry is found in `[install.security]` BEFORE a later malformed `[missing-close` header. The scanner returns `true, true` immediately on finding the entry — the malformed header after the match is unreachable.
- **Fix:** Split into two cases: (1) "scanner found before malformed section" (wantOK=true, wantScanner=true — early return) and (2) "malformed header before any scanner entry" (wantOK=false — correct behavior). Documented both behaviors explicitly.
- **Files modified:** `internal/nudge/scanners_test.go`
- **Commit:** `3a4b612`

**[Rule 1 - Bug] NUL bytes in fuzz test string literals failed Go compilation**

- **Found during:** Task 1 fuzz test verification
- **Issue:** String literals with literal NUL bytes (`\x00`) are illegal in Go source files; `go test -tags fuzz` failed with "illegal character NUL".
- **Fix:** Replaced literal NUL bytes with `string([]byte{0x00})` concat form in seed corpus lines. Also removed problematic backtick-in-string construct.
- **Files modified:** `internal/nudge/scanners_fuzz_test.go`
- **Commit:** `3a4b612`

## Known Stubs

None — all code paths are fully implemented and test-covered. No placeholder values that flow to UI rendering.

## Threat Flags

All STRIDE threats from the plan's `<threat_model>` are mitigated:

| Threat | Mitigation | Status |
|--------|------------|--------|
| T-08-10 PATH hijack / arbitrary exec | Fixed argv `("pnpm"/"bun"/"node", "--version")` only; no user-controlled path in exec call | Applied |
| T-08-10b DetectStateFn seam abused at runtime | SeamDocumented: never reassigned in production code; test-only infrastructure | Applied |
| T-08-11 Slow PM blocks check budget | 2s hard timeout per exec; timeout -> PM not installed, proceed (fail-open) | Applied |
| T-08-12 Malformed config crashes security tool | scanBunfig/scanPnpmWorkspace return safe defaults, never panic; FuzzBunfig/FuzzPnpmWorkspace seeds pass | Applied |
| T-08-13 Reading arbitrary files | Only fixed paths: bunfig.toml (project root + ~/.bunfig.toml), pnpm-workspace.yaml (project root); read-only, never executed | Applied |

No new security surface beyond what the plan's threat model covers.

## Self-Check: PASSED

- `internal/nudge/detect.go` — FOUND
- `internal/nudge/detect_test.go` — FOUND
- `internal/nudge/scanners.go` — FOUND
- `internal/nudge/scanners_test.go` — FOUND
- `internal/nudge/scanners_fuzz_test.go` — FOUND
- Commit `3a4b612` — FOUND
- Commit `ba87c6f` — FOUND
- `go build ./...` — CLEAN
- `go test ./internal/nudge/...` — ALL PASS (42 tests, 0 failures)
- `go test -tags fuzz -run "FuzzBunfig|FuzzPnpmWorkspace" ./internal/nudge/...` — ALL SEEDS PASS (BTEST-03)

---
*Phase: 08-package-manager-nudge-behavioral-test-suite*
*Completed: 2026-06-04*
