---
phase: 4
slug: windows-extension-mcp-coverage-beekeeper-compat-test
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-02
---

# Phase 4 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from 04-RESEARCH.md "Validation Architecture". Two repos: Pollen (`../pollen`) and Beekeeper (this repo).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing stdlib (`go test`) — both repos |
| **Config file** | none (go.mod only) |
| **Quick run command (Pollen, Windows)** | `cd ../pollen && go test ./cmd/pollen/ -run TestWindowsExtensionMCPRoots` |
| **Quick run command (Beekeeper)** | `go test ./internal/scan/ -run TestPollenCompatibility` |
| **Full suite command (Pollen)** | `cd ../pollen && go test ./...` |
| **Full suite command (Beekeeper)** | `go test ./...` |
| **Estimated runtime** | ~30 seconds per repo (no `-race` locally; `-race` is CI-only per CLAUDE.md) |

---

## Sampling Rate

- **After every task commit (Pollen):** `cd ../pollen && go test ./cmd/pollen/ -run TestWindowsExtensionMCPRoots && go vet ./...`
- **After every task commit (Beekeeper):** `go test ./internal/scan/ && go vet ./internal/scan/`
- **After every plan wave:** full suite in the affected repo (`go test ./...`)
- **Before `/gsd-verify-work`:** Full suite green in BOTH repos
- **Max feedback latency:** ~30 seconds

---

## Per-Task Verification Map

> Task IDs (`4-PP-TT`) are assigned by the planner; rows below are the requirement→behavior→test contract the planner must map tasks onto. Threat refs carried from Phase 2 (no new attack surface — read-only path enumeration + subprocess rename).

| Behavior | Requirement | Threat Ref | Test Type | Automated Command | File Exists | Status |
|----------|-------------|------------|-----------|-------------------|-------------|--------|
| `resolveRoots(baseline)` returns VS Code / Insiders / Cursor / Windsurf / VSCodium editor-extension roots on Windows | WEXT-01 | — | unit (windows) | `cd ../pollen && go test ./cmd/pollen/ -run TestWindowsExtensionMCPRoots` | ❌ W0 | ⬜ pending |
| `resolveRoots(baseline)` returns Chrome / Chromium / Edge / Brave (per-profile) + Firefox roots on Windows | WEXT-02 | T-02-02 (junction, accepted) | unit (windows) | `cd ../pollen && go test ./cmd/pollen/ -run TestWindowsExtensionMCPRoots` | ❌ W0 | ⬜ pending |
| `resolveRoots(baseline)` returns Claude Desktop / Cursor / Windsurf / Cline / Gemini MCP config roots on Windows | WEXT-03 | — | unit (windows) | `cd ../pollen && go test ./cmd/pollen/ -run TestWindowsExtensionMCPRoots` | ❌ W0 | ⬜ pending |
| Pollen scanner emits editor-extension / browser-extension / mcp records for Windows fixtures (end-to-end) | WEXT-01/02/03 | — | integration (windows) | `cd ../pollen && go test ./cmd/pollen/ -run TestParityAllEcosystems` (or new fixture test) | ❌ W0 | ⬜ pending |
| Linux/macOS paths unaffected — byte-for-byte parity with upstream preserved | WEXT-01/02/03 | — | regression | `cd ../pollen && go test ./cmd/pollen/ -run 'TestDifferential\|TestParityAllEcosystems'` | ✅ existing | ⬜ pending |
| `bumblebee`→`pollen` rename compiles; existing scan tests pass | BKINT-01 | T-spoof (PATH, accepted) | unit | `go test ./internal/scan/` | ✅ update existing | ⬜ pending |
| `pollen_unavailable` status emitted when `pollen` not in PATH (fail-graceful) | BKINT-01 | — | unit | `go test ./internal/scan/ -run TestScanPollenUnavailable` | ❌ W0 | ⬜ pending |
| Pollen compat test: all 5 record types pass through, `scanner_name` asserted, no double-counting | PTEST-04 | — | integration | `go test ./internal/scan/ -run TestPollenCompatibility` | ❌ W0 | ⬜ pending |
| Zero `t.Skip` in beekeeper inventory tests on Windows (skip baseline = 0) | PTEST-04 | — | verification | `go test ./internal/scan/ -v 2>&1 \| Select-String SKIP` is empty | N/A (fixture-driven design) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `../pollen/cmd/pollen/roots_windows_test.go` — add `TestWindowsExtensionMCPRoots` covering WEXT-01/02/03 root discovery (editor + browser per-profile + MCP), using `t.Setenv(USERPROFILE/APPDATA/LOCALAPPDATA)` isolation (never `HOME` — Pitfall 5)
- [ ] `../pollen/cmd/pollen/testdata/` — fake extension/MCP fixture tree for the end-to-end Windows record test (extend `parity-fixture/` with Windows sub-trees OR a dedicated fixture; do NOT mutate the locked differential parity fixture)
- [ ] `beekeeper/internal/scan/scanner_test.go` — add `TestPollenCompatibility` (PTEST-04); add/rename `TestScanPollenUnavailable`; update `runBumblebeeFn` references to `runPollenFn` after the BKINT-01 rename

*Framework already present — both repos use stdlib `testing` only. No install needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real Windows CI run of `pollen scan` against planted fixtures across all five editors / browser families / MCP hosts | WEXT-01/02/03, PTEST-04 | Windows is the dev machine but the differential + full 3-OS matrix is CI-only; live junction-point behavior under `%LOCALAPPDATA%` is a carried Phase-2 open flag | Push to a branch; confirm the 3-OS GitHub Actions matrix is green with zero Windows `t.Skip` in inventory tests |

*All unit/integration behaviors above have automated verification; only the cross-OS CI matrix confirmation is environment-gated.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING (❌ W0) references above
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
