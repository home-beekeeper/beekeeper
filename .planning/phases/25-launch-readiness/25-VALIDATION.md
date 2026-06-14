---
phase: 25
slug: 25-launch-readiness
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-14
---

# Phase 25 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Phase 25 (Launch Readiness) is a validation + benchmark + docs phase over code already shipped in Phases 22–24. Derived from `25-RESEARCH.md` § Validation Architecture.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing package (stdlib) |
| **Config file** | none (tests use `t.TempDir()`) |
| **Quick run command** | `go test ./internal/corpus/... ./internal/check/... ./cmd/beekeeper/... -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~60–90 seconds (full suite, 27 pkgs) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/corpus/... ./internal/check/... ./cmd/beekeeper/... -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd-verify-work`:** Full suite green + `go vet ./...` + `go build ./...` + `go mod tidy && git diff --exit-code go.mod` (zero-new-deps gate)
- **Max feedback latency:** ~90 seconds

---

## Per-Task Verification Map

> Task IDs (`25-NN-NN`) are assigned by the planner; rows are keyed by requirement so the planner can attach each to a concrete task. "File Exists" reflects state at research time.

| Requirement | Behavior | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|-------------|----------|------------|-----------------|-----------|-------------------|-------------|--------|
| LAUNCH-01 | Nx Console trace runs trace→record→adjudication→signature→local feedback with all 4 layers populated | T-25-PURGE (no auto-purge in feedback) | Confirmed-malicious arms card + overlay, never auto-purges | integration | `go test ./cmd/beekeeper/... -run TestRunCatalogsSyncFirstResponder -count=1` | ✅ exists — extend 7-point gate to 11-point | ⬜ pending |
| LAUNCH-02 | Each of 8 Sentry patterns (SENTRY-001..008) produces a moat-grade record (all 4 layers present) | T-25-INJECT (no `auto_purge` via test inputs) | Outcome layer present as `unresolved` placeholder (non-retrofittable) | unit/integration | `go test ./internal/corpus/... -run TestAllSentryPatternsProduceMoatRecord -count=1` | ❌ Wave 0 gap | ⬜ pending |
| LAUNCH-03 (perf) | `beekeeper check` p99 < 100ms with corpus enabled (corpus loop off hot path) | T-25-PERF (baseline inflation) | Deterministic gate, 100ms (4x headroom over ~25ms) | benchmark/gate | `go test ./internal/check/... -run TestBenchmarkRunCheckGate -count=1` | ❌ Wave 0 gap (existing BenchmarkRunCheck eyeball-only) | ⬜ pending |
| LAUNCH-03 (offline) | Disconnected machine blocks on last-synced mmap catalog | — | Fail-closed: block (not allow) with no live sources | unit | `go test ./internal/check/... -run TestOfflineProtective -count=1` | ❌ Wave 0 gap (pattern exists; test does not) | ⬜ pending |
| LAUNCH-04 (no-exfil) | No corpus data leaves the machine (static import gate) | T-25-EXFIL | Corpus store has no `net`/`net/http` import path | static/unit | `go test ./internal/corpus/... -run TestCorpusStoreHasNoNetworkImports -count=1` | ❌ Wave 0 gap | ⬜ pending |
| LAUNCH-04 (docs) | THREAT-MODEL.md §13 names 3 residual gaps verbatim (SENTRY-008 OIDC theft, GitHub API dead-drop, DNS-tunnel ingested-but-undetected) | T-25-DOCS (understatement = false confidence) | All three gaps named; treat as correctness req | grep gate + human-review | grep gate over `docs/THREAT-MODEL.md` (test or manual) | ❌ Wave 0 gap | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/corpus/launch_e2e_test.go` — LAUNCH-02 8-pattern moat-record table (and, per planner choice, LAUNCH-01 assertions #8–11 here or extended in `cmd/beekeeper/catalogs_daemon_test.go`)
- [ ] `internal/check/handler_test.go` — `TestBenchmarkRunCheckGate` (latency-ring p99 assertion via existing `LTRing.P99()`) + `TestOfflineProtective`
- [ ] `internal/corpus/store_test.go` — `TestCorpusStoreHasNoNetworkImports` (import assertion, `go list`-style)
- [ ] `docs/THREAT-MODEL.md` — new `## 13. Adjudicated Corpus (Local Loop) — v1.4.0` section naming all 3 residual gaps + a grep tripwire test

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| THREAT-MODEL.md residual-gap framing reads honestly (not just present, but correctly scoped) | LAUNCH-04 | Editorial honesty cannot be fully automated; grep proves presence, not framing | Maintainer reads §13; confirms each gap is named AND scoped as architectural-mitigation-only, consistent with the v1.0.0–v1.3.0 honesty precedent |
| BenchmarkRunCheck p99 on production hardware (the carried-over Phase-23 eyeball item) | LAUNCH-03 | Dev/CI hardware differs; p99 is a development-machine gate | Run `go test -bench=BenchmarkRunCheck ./internal/check/...` on the maintainer's machine; confirm p99 well under 100ms |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
