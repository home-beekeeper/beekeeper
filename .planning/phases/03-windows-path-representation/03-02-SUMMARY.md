---
phase: 03-windows-path-representation
plan: "02"
subsystem: endpoint
tags: [go, windows, uid, runtime.GOOS, pollen, endpoint, wpath-02]

# Dependency graph
requires:
  - phase: 03-windows-path-representation/03-01
    provides: context for Windows path representation phase (npm/pnpm path fixes)
provides:
  - "Windows-empty UID in Pollen host endpoint record: endpoint.Current() returns uid='' on Windows"
  - "Unix UID regression guard: TestCurrentWindowsUID asserts non-empty uid on Linux/macOS (D-04)"
affects: [03-03, 03-04]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Inline runtime.GOOS guard for OS-specific field suppression (inline over build-tag files for single-field override)"

key-files:
  created: []
  modified:
    - ../pollen/internal/endpoint/endpoint.go
    - ../pollen/internal/endpoint/endpoint_test.go

key-decisions:
  - "Inline runtime.GOOS != 'windows' guard on BOTH UID assignments (per RESEARCH Open Question 2 recommendation — fewer files than build-tag pair for single-field override)"
  - "Both the happy-path ep.UID = u.Uid and the error-fallback ep.UID = strconv.Itoa(os.Getuid()) guarded (Pitfall 5 — guarding only one leaks a non-empty uid on Windows when user.Current() fails)"
  - "No new imports, no schema change, no endpoint_windows.go / endpoint_notwindows.go build-tag files created"

patterns-established:
  - "WPATH-02 inline guard: if runtime.GOOS != 'windows' { ep.UID = u.Uid } paired with } else if runtime.GOOS != 'windows' { ep.UID = strconv.Itoa(os.Getuid()) }"

requirements-completed: [WPATH-02]

# Metrics
duration: 8min
completed: 2026-06-02
---

# Phase 03 Plan 02: Windows Endpoint UID (WPATH-02) Summary

**Inline runtime.GOOS guard on both UID assignments in endpoint.Current() eliminates SID leakage on Windows while preserving numeric uid on Unix**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-06-02T~15:30:00Z
- **Completed:** 2026-06-02T~15:38:00Z
- **Tasks:** 1
- **Files modified:** 2 (in pollen repo)

## Accomplishments

- Both defect sites in `endpoint.Current()` guarded: `ep.UID = u.Uid` (SID on Windows) and `ep.UID = strconv.Itoa(os.Getuid())` (returns -1 on Windows) now only execute when `runtime.GOOS != "windows"`
- On Windows `ep.UID` stays at its zero value `""` — no SID string, no "-1" artifact
- Unix behavior unchanged: numeric uid still populated on Linux/macOS (D-04 preserved)
- `TestCurrentWindowsUID` added to `endpoint_test.go`: asserts `ep.UID == ""` on Windows and `ep.UID != ""` on Unix (both branches active at test runtime)
- All three endpoint tests pass on Windows dev machine: `TestCurrentPopulatesDeviceID`, `TestCurrentEmptyDeviceID`, `TestCurrentWindowsUID`
- No new imports, no schema change (`uid` field present in NDJSON, just empty), no build-tag files created

## Task Commits

1. **Task 1: WPATH-02 guard both UID assignments + TestCurrentWindowsUID** - `92e9ac0` in pollen repo (fix)

## Files Created/Modified

- `../pollen/internal/endpoint/endpoint.go` - Both `ep.UID` assignments wrapped with `runtime.GOOS != "windows"` guard; comments explain SID/Getuid(-1) reasoning and WPATH-02 requirement
- `../pollen/internal/endpoint/endpoint_test.go` - Appended `TestCurrentWindowsUID` asserting empty uid on Windows and non-empty on Unix; existing tests untouched

## Decisions Made

- Used inline `runtime.GOOS != "windows"` guard (not build-tagged `endpoint_windows.go` / `endpoint_notwindows.go` pair): this is a single-field override where inline is the recommended minimal-diff approach per RESEARCH.md Open Question 2
- Guarded BOTH the happy path and the `else` error fallback (Pitfall 5): `else if runtime.GOOS != "windows"` shape is the cleanest form that correctly suppresses `-1` on Windows when `user.Current()` fails

## Deviations from Plan

None — plan executed exactly as written. Existing tests (`TestCurrentPopulatesDeviceID`, `TestCurrentEmptyDeviceID`) do not assert UID, so no Rule-1 skip-on-windows deviation was needed.

## Verification Results

```
go vet ./internal/endpoint/   — clean (no output)
go test ./internal/endpoint/ -v:
  === RUN   TestCurrentPopulatesDeviceID
  --- PASS: TestCurrentPopulatesDeviceID (0.02s)
  === RUN   TestCurrentEmptyDeviceID
  --- PASS: TestCurrentEmptyDeviceID (0.00s)
  === RUN   TestCurrentWindowsUID
  --- PASS: TestCurrentWindowsUID (0.00s)
  PASS
  ok  github.com/home-beekeeper/pollen/internal/endpoint  1.181s
```

Dev OS: Windows 11. `TestCurrentWindowsUID` exercised the `runtime.GOOS == "windows"` branch and confirmed `ep.UID == ""`.

## Issues Encountered

None.

## Threat Surface Scan

T-03-04 (Information Disclosure — SID leakage) is resolved: `endpoint.uid` is now `""` on Windows, eliminating the SID string from every emitted NDJSON record. No new attack surface introduced. T-03-05 and T-03-06 status unchanged (accepted).

## Known Stubs

None.

## Self-Check

- `../pollen/internal/endpoint/endpoint.go` — modified (confirmed via edit)
- `../pollen/internal/endpoint/endpoint_test.go` — modified (confirmed via edit)
- Pollen commit `92e9ac0` — confirmed (`git -C ../pollen log --oneline -1` → `92e9ac0`)
- `go test ./internal/endpoint/` exits 0 on Windows — confirmed above

## Self-Check: PASSED

## Next Phase Readiness

- WPATH-02 endpoint half complete
- Plan 03-03 (parity test extensions: `assertWindowsEndpointUID`) can proceed
- Plan 03-04 (beekeeper consumer round-trip test `TestScanWindowsShapedRecord`) can proceed
- Unix differential test (`TestDifferential` PTEST-02) unaffected — guard is always true on Unix

---
*Phase: 03-windows-path-representation*
*Completed: 2026-06-02*
