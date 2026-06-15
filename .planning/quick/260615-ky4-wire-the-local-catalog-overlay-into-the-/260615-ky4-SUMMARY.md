---
phase: quick-260615-ky4
plan: 01
subsystem: catalog
tags: [catalog, overlay, corroboration, policy, frb-05, check, handler]

requires:
  - phase: v1.4.0-phase-24
    provides: "AddLocalOverlayEntry writes local-overlay.{json,idx} and NewMultiIndexWithOverlay query machinery"

provides:
  - "Live beekeeper check hot path queries local-overlay.idx via NewMultiIndexWithOverlay (FRB-05 enforcement)"
  - "RunCheck-level overlay escalation regression test (TestRunCheckLocalOverlayEscalates) drives real handler"
  - "Fail-closed no-overlay regression guard (TestRunCheckNoOverlayUnchanged)"

affects:
  - v1.4.0-milestone
  - gateway-overlay-followup

tech-stack:
  added: []
  patterns:
    - "Overlay-close isolation: defer closes only multiIdx.Overlay to avoid double-close of bbIdx (existing defer bbIdx.Close() still owns bumblebee)"
    - "overlayPath guard: cacheDir==\"\" passes \"\" to NewMultiIndexWithOverlay, which leaves Overlay=nil (fail-closed)"

key-files:
  created: []
  modified:
    - internal/check/handler.go
    - internal/check/handler_test.go

key-decisions:
  - "allow→warn (not →block) is the correct overlay escalation: overlay entries are unsigned (CatalogSignature==\"\"), so signedCount=0 and corroborate() returns warn-only per CTLG-07"
  - "Dedicated overlay defer (not multiIdx.Close()) prevents double-close of bbIdx which is already deferred at line 205"
  - "overlayPath=\"\" guard preserves behavior when cacheDir is empty (e.g. tests that pass empty cacheDir)"
  - "Gateway (internal/gateway/gateway.go:141), daemon adjudication re-query (catalogs_daemon.go:113), scan, and watch NewMultiIndex call sites deliberately left unchanged — each has its own rationale (see Deliberate Omissions)"

requirements-completed: [FRB-05]

duration: 25min
completed: 2026-06-15
---

# Quick Task 260615-ky4: Wire Local Catalog Overlay into Live Check Hot Path

**FRB-05 closed: confirmed-malicious local overlay entries now enforce in the live beekeeper check hot path via NewMultiIndexWithOverlay, proven by a RunCheck-level regression test that fails on revert**

## Performance

- **Duration:** ~25 min
- **Completed:** 2026-06-15
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Replaced `catalog.NewMultiIndex` with `catalog.NewMultiIndexWithOverlay` at handler.go line 236, making the live `beekeeper check` path query `local-overlay.idx` when present (FRB-05)
- Added `overlayPath` guard: when `cacheDir==""`, `overlayPath=""` is passed, which `NewMultiIndexWithOverlay` treats as "no overlay" (Overlay stays nil) — no behavior change for callers with empty cacheDir
- Added dedicated `defer` to close only `multiIdx.Overlay`, avoiding double-close of `bbIdx` (already deferred at line 205 via `defer bbIdx.Close()`)
- Added `TestRunCheckLocalOverlayEscalates`: seeds an unsigned `local-overlay.idx` in `cacheDir`, drives real `RunCheck`, asserts `Decision.Level=="warn"` — fails if `handler.go:236` is reverted to `NewMultiIndex` (revert-check confirmed)
- Added `TestRunCheckNoOverlayUnchanged`: same package, no overlay file present; asserts `Decision.Level=="allow"` (fail-closed guard: missing overlay causes no error, no fail-open, no panic)
- Added `buildOverlayTestIndex` helper: builds a `local-overlay.idx` with `CatalogSignature==""` per CTLG-07 (unsigned/warn-only invariant)

## Task Commits

1. **Task 1: Wire local catalog overlay into live check hot path** - `fc9fd66` (feat)
2. **Task 2: RunCheck-level overlay regression tests** - `4f1756f` (test)

## Files Created/Modified

- `internal/check/handler.go` - Replaced NewMultiIndex with NewMultiIndexWithOverlay at line 236; added overlayPath computation and dedicated overlay-only defer
- `internal/check/handler_test.go` - Added buildOverlayTestIndex helper, TestRunCheckLocalOverlayEscalates, TestRunCheckNoOverlayUnchanged

## CORRECTNESS NOTE: Why allow→warn (not →block)

Overlay entries are UNSIGNED (`CatalogSignature==""` — enforced by `AddLocalOverlayEntry` per CTLG-07). `internal/policy/corroboration.go` escalates to `block` only on `signedCount >= effectiveBlockAt && hasSignedSource`. With `signedCount=0` and `hasUnsigned=true`, `corroborate()` returns `warn`.

The honest, load-bearing escalation the overlay produces is **allow → warn**: a package with no catalog match (decision `allow`) becomes `warn` once the overlay contributes a `local-overlay` source. The test asserts exactly this flip — not a fabricated block via a signed overlay entry, which would contradict the CTLG-07 invariant.

## Deliberate Omissions (Surfaces NOT wired in this quick task)

These surfaces were left with plain `NewMultiIndex` intentionally. The rationale is recorded here so the milestone follow-up is traceable:

| Call Site | File | Reason Left Unchanged |
|-----------|------|-----------------------|
| daemon adjudication re-query | `cmd/beekeeper/catalogs_daemon.go:113` | MUST NOT read its own overlay output — feedback loop (adjudicator writes overlay, then immediately re-queries it → circular) |
| MCP gateway | `internal/gateway/gateway.go:141` | A real second interception surface for FRB-05, but out of scope for this tight quick task. **Clean follow-up: wire gateway the same way as handler.go** |
| scan | `internal/scan/scanner.go:368` | Separate flow (pollen-based package scan); overlay semantics differ |
| watch crossref | `internal/watch/crossref.go:190` | Separate flow; watch daemon uses its own catalog context |
| watch handler | `internal/watch/handler.go:122` | Same rationale as crossref |

The gateway is the only surface where overlay enforcement adds meaningful real-time value alongside the check path. It should be the next follow-up.

## Decisions Made

- Used `editor-extension` ecosystem for test target package: `osvEcosystem` returns `("", false)` → no HTTP call, no Socket token needed, deterministic and offline
- Used `evil.confirmed-malicious-overlay-pkg` as the fictional test package name (not in any real or test catalog) so the ONLY possible match comes from the overlay
- Dedicated overlay defer vs. `multiIdx.Close()`: multiIdx.Close() closes BOTH Overlay and Bumblebee; bbIdx already has its own defer → double-close. The dedicated defer is the correct pattern.

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

- Initial `go test ./...` appeared to show a `FAIL` for `internal/check` during the full-suite run; on re-run the suite was fully green (all 27 packages pass). The earlier output included a transient test failure unrelated to this change.

## Verification Results

- `go build ./...`: PASS
- `go vet ./internal/check/...`: PASS (clean)
- `go test ./internal/check/ -run "TestRunCheckLocalOverlayEscalates|TestRunCheckNoOverlayUnchanged" -count=1 -v`: PASS (both new tests)
- `go test ./internal/check/ -count=1`: PASS (full check package, no regressions to existing tests)
- `go test ./... -count=1`: PASS (all 27 packages green, zero new dependencies)
- Revert-check: reverting handler.go:236 to `catalog.NewMultiIndex` causes `TestRunCheckLocalOverlayEscalates` to fail with `Decision.Level="allow"` instead of `"warn"` — confirmed the test guards the live wiring path, not just the component

## Known Stubs

None.

## Threat Flags

None. This change is read-only at the overlay side (no new write paths). The overlay write path (`AddLocalOverlayEntry` + `platform.SetOwnerOnly`) is unchanged and already guarded by owner-only permissions per T-24-OVR-TAMPER.

## Self-Check: PASSED

- internal/check/handler.go: FOUND
- internal/check/handler_test.go: FOUND
- .planning/quick/260615-ky4-.../260615-ky4-SUMMARY.md: FOUND
- Commit fc9fd66 (feat): FOUND
- Commit 4f1756f (test): FOUND
- No tracked file deletions in commits

---
*Quick task: 260615-ky4*
*Completed: 2026-06-15*
