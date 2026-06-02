---
phase: 3
slug: windows-path-representation
status: approved
nyquist_compliant: true
wave_0_complete: true
created: 2026-06-02
approved: 2026-06-02
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Detailed observable behaviors + minimum test set: see `03-RESEARCH.md` § Validation Architecture.

---

## Test Infrastructure

Two-repo phase. Pollen changes (WPATH-01/02 emitter) live in `../pollen`; the beekeeper
round-trip test lives in this repo.

| Property | Value |
|----------|-------|
| **Framework** | `go test` (Go 1.25+) — both repos |
| **Config file** | none — `go.mod` per repo |
| **Quick run command (pollen)** | `cd ../pollen && go test ./internal/ecosystem/npm/ ./internal/ecosystem/pnpm/ ./internal/endpoint/` |
| **Quick run command (beekeeper)** | `go test ./internal/scan/` |
| **Full suite command (pollen)** | `cd ../pollen && go test ./...` (+ `go vet ./...`) |
| **Full suite command (beekeeper)** | `go test ./...` |
| **Differential safety (Unix-only)** | `cd ../pollen && go test ./cmd/pollen/ -run '^TestDifferential$'` (must stay byte-identical) |
| **Estimated runtime** | ~30–60 seconds per repo |

> **Windows-primary dev caveat (CLAUDE.md):** `go test -race` needs CGO/C compiler — race gate is
> CI-only. Windows-tagged tests (`//go:build windows`) run on the Windows dev box directly; any
> test that must stay Windows-skipped pending CI uses a structured `t.Skip` reason (Phase-2 pattern).

---

## Sampling Rate

- **After every task commit:** Run the relevant quick command for the repo touched.
- **After every plan wave:** Run the full suite for each repo touched.
- **Before `/gsd-verify-work`:** Full suite green in both repos; differential test green on the dev OS.
- **Max feedback latency:** ~60 seconds.

---

## Per-Task Verification Map

> Filled with concrete task IDs during execution. Anchor behaviors (from RESEARCH § Validation
> Architecture) the plans must cover:

| Behavior (anchor) | Requirement | Secure Behavior | Test Type | Automated Command | Status |
|-------------------|-------------|-----------------|-----------|-------------------|--------|
| npm/pnpm `projectPath` retains backslash+drive on Windows | WPATH-01 | no Unix→Win path artifacts in emitted records | unit (Windows) | `cd ../pollen && go test ./internal/ecosystem/npm/ ./internal/ecosystem/pnpm/` | ⬜ pending |
| Emitted `project_path`/`source_file` are native Windows paths | WPATH-01 | `C:\...` not `/c/...` | unit (Windows) | `cd ../pollen && go test ./internal/ecosystem/npm/ ./internal/ecosystem/pnpm/` | ⬜ pending |
| Unix NDJSON bytes unchanged (FromSlash no-op) | WPATH-01 | no upstream drift | differential | `cd ../pollen && go test ./cmd/pollen/ -run '^TestDifferential$'` | ⬜ pending |
| `endpoint.uid` empty on Windows; populated on Unix | WPATH-02 | no SID leakage as uid | unit | `cd ../pollen && go test ./internal/endpoint/` | ⬜ pending |
| `endpoint.os/arch/username` correct per platform | WPATH-02 | host identity accurate | unit | `cd ../pollen && go test ./internal/endpoint/` | ⬜ pending |
| Beekeeper parses Windows-shaped record without error (round-trip) | WPATH-02 | consumer tolerates empty uid + backslash paths | unit | `go test ./internal/scan/ -run Endpoint` | ⬜ pending |
| Parity asserts `endpoint.os` per platform + Win path shape | WPATH-01/02 | cross-platform equivalence | parity | `cd ../pollen && go test ./cmd/pollen/ -run Parity` | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- Existing infrastructure (`go test`) covers all phase requirements — no framework install needed.
- New test files extend existing analogs: `npm_test.go`/`pnpm_test.go`, `endpoint_test.go`,
  `output_test.go` (pollen); `scanner_test.go` (beekeeper). No new package (`internal/inventory/`
  deferred to Phase 4 per RESEARCH).

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Windows CI `pollen scan` emits backslash/drive paths end-to-end | WPATH-01 | full scan on a real Windows runner; SC1 is a CI assertion | Windows CI job runs `pollen scan` against the fixture tree; assert no `/c/` artifacts in NDJSON |

*Most behaviors have automated unit/differential/parity coverage; the end-to-end CI scan is the one manual/CI-runner gate.*

---

## Validation Sign-Off

- [x] All tasks have automated verify or Wave 0 dependencies — every plan task carries an `<automated>` `go test` verify (confirmed by plan-checker, all 3 plans `valid: true`)
- [x] Sampling continuity: no 3 consecutive tasks without automated verify — each plan has ≥1 automated verify per task
- [x] Wave 0 covers all MISSING references — no separate Wave 0 needed: `go test` infra already exists; the new Windows tests are appended to existing files (`npm_test.go`, `pnpm_test.go`, `endpoint_test.go`, `scanner_test.go`, `parity_test.go`) within their implementing tasks
- [x] No watch-mode flags — all commands are single-shot `go test`
- [x] Feedback latency < 60s — quick commands target single packages
- [x] `nyquist_compliant: true` set in frontmatter

**Note:** This phase has no traditional separate Wave-0 test-stub plan — the WPATH-01/02 Windows
tests are added in the same tasks that make the fix (the fix is a no-op on the dev OS until run on
Windows CI, so a fail-first stub adds no signal locally). Sampling continuity (Dimension 8c) is
satisfied because every task has an automated verify.

**Approval:** approved 2026-06-02 (orchestrator, post-plan-check remediation)
