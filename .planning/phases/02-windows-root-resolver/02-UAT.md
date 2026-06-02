---
status: complete
phase: 02-windows-root-resolver
source: [02-01-SUMMARY.md, 02-02-SUMMARY.md, 02-03-SUMMARY.md, 02-04-SUMMARY.md]
started: 2026-06-02T11:37:26Z
updated: 2026-06-02T11:37:26Z
mode: automated (run + auto-approve; code under test is the sibling repo ../pollen)
---

## Current Test

[testing complete]

## Tests

### 1. Cold-start tri-GOOS build (resolver wiring is core to roots.go)
expected: `cd ../pollen && GOOS=windows|linux|darwin go build ./...` all exit 0 — the Windows resolver compiles under the windows tag AND the `case "windows":` wiring + `roots_notwindows.go` stubs keep Linux/macOS building unchanged.
result: pass
evidence: GOOS=windows ✓, GOOS=linux ✓, GOOS=darwin ✓ (all build ./... exit 0)

### 2. Static analysis clean
expected: `cd ../pollen && go vet ./...` exits 0.
result: pass
evidence: go vet ./... ✓

### 3. Windows 8-ecosystem root discovery (WRES-01, WRES-02)
expected: `go test ./cmd/pollen/ -run '^TestWindowsBaselineRoots'` passes on the Windows host — npm/pnpm/Yarn/Bun/PyPI/Go/RubyGems/Composer roots resolve from `%APPDATA%`/`%LOCALAPPDATA%`/`%USERPROFILE%`/`%ProgramFiles%` (incl. glob-resolved Python*/Ruby*/.gem), empty-env-var guard emits no volume-less root.
result: pass
evidence: TestWindowsBaselineRoots ✓ (native Windows host)

### 4. Cross-platform parity test (PTEST-01)
expected: `go test ./cmd/pollen/ -run '^TestParityAllEcosystems$'` passes — same fixture → equivalent normalized records, `endpoint.os == runtime.GOOS`, all package ecosystems covered.
result: pass
evidence: `--- PASS: TestParityAllEcosystems (51.02s)` — "PTEST-01 PASSED on windows: endpoint.os correct, all 5 ecosystems covered (8 records after normalization)"

### 5. Full Pollen module test suite (native Windows)
expected: `cd ../pollen && go test ./...` green across all 19 packages (cmd/pollen + all internal/ecosystem/* + endpoint/exposure/model/normalize/output/scanner/walk).
result: pass
evidence: go test ./... ✓ (all 19 packages ok)

### 6. Windows CI skip discipline (Success Criterion 4 — skip half)
expected: all 6 Phase-2 `t.Skip` markers in `cmd/pollen/main_test.go` are flipped (0 "Phase 2 (v0.1.1-pollen.2)" strings remain); the differential test's `runtime.GOOS == "windows"` skip is preserved (differential is Linux+macOS-only by design).
result: pass
evidence: main_test.go Phase-2 skip strings = 0; differential_test.go windows skip present

### 7. PTEST-02 differential lock intact (Success Criterion 3)
expected: `cmd/pollen/normalize_diff.go` is byte-unchanged since the Phase-1 lock (`7ba6073`) — the Windows additions cannot drift the upstream-parity differential.
result: pass
evidence: `git diff --quiet 7ba6073 -- cmd/pollen/normalize_diff.go` exits 0 (UNCHANGED)

### 8. Release prep accuracy (02-04, local)
expected: `../pollen/VERSION` == `0.1.1-pollen.2`; `CHANGES.md` records the ACTUAL file `cmd/pollen/roots_windows.go` and does NOT use the non-existent PRD-draft `internal/resolver/resolver_windows.go` path.
result: pass
evidence: VERSION = 0.1.1-pollen.2; CHANGES roots_windows.go = 1, internal/resolver path = 0

## Summary

total: 8
passed: 8
issues: 0
pending: 0
skipped: 0

## Gaps

[none — all automated tests passed]

## Notes — CI-gated / deferred (not failures, tracked elsewhere)

These are NOT code issues; they are environment/release gates that cannot be exercised on the local Windows dev host and are tracked in STATE.md / ROADMAP / 02-04-SUMMARY:

- **Cross-OS parity legs (Linux + macOS):** `TestParityAllEcosystems` passed on Windows; the Linux/macOS runs of the same test execute when the branch is pushed and the 3-OS CI matrix fires. Local host can only exercise the Windows leg.
- **`-race` race detector:** the suite passes without `-race` locally; `-race` requires CGO + a C compiler (not installed on the Windows dev box — known constraint since Phase 1). The `go test -v -race ./...` (CGO_ENABLED=1) run executes on `windows-latest` CI.
- **Signed `v0.1.1-pollen.2` release (Success Criterion 4 — release half):** VERSION+CHANGES are committed locally (`../pollen` `c94b271`); push + tag + cosign signing + SBOM are **deferred to Milestone-2 close** by maintainer decision. Tracked in STATE.md Deferred Items and ROADMAP Phase 2.
