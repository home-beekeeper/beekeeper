---
phase: 24
slug: first-responder-corpus-binding
status: approved
nyquist_compliant: true
wave_0_complete: true
created: 2026-06-14
---

# Phase 24 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Seeded from `24-RESEARCH.md` → Validation Architecture.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (stdlib) |
| **Config file** | none — `go test ./...` |
| **Quick run command** | `go test ./internal/corpus/... ./internal/catalog/... ./internal/watch/... -short` |
| **Full suite command** | `go test ./... -count=1` |
| **Build verification** | `go build ./...` |
| **Estimated runtime** | ~60–120 seconds (full suite) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/corpus/... ./internal/catalog/... ./internal/watch/... -short`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd-verify-work`:** Full suite green + the synthetic Nx Console FRB evaluator gate (see below) passes
- **Max feedback latency:** ~120 seconds

---

## Per-Task Verification Map

> Task IDs are finalized by the planner; this map is requirement-level until then. Each row is an automated FRB check from the research test map.

| Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | Status |
|------|------|-------------|------------|-----------------|-----------|-------------------|--------|
| 24-01 | 1 | FRB-01 | — | `ReadMaliciousRecords` returns only `TrueLabel=="malicious"`, latest-per-cluster wins | unit | `go test ./internal/corpus/... -run TestReadMaliciousRecords` | ⬜ pending |
| 24-01 | 1 | FRB-05 | T-24-OVR-DOS | Malformed/missing corpus NDJSON → skipped/nil, never panics | unit | `go test ./internal/corpus/... -run TestReadMaliciousRecords` | ⬜ pending |
| 24-01 | 1 | FRB-05 | T-24-OVR-TAMPER | Overlay file is 0600 / owner-only | unit | `go test ./internal/catalog/... -run TestLocalOverlayFilePermissions` | ⬜ pending |
| 24-01 | 1 | FRB-05 | — | Local overlay survives mock `SyncConditional` (bumblebee.* write untouched) | unit | `go test ./internal/catalog/... -run TestLocalOverlaySurvivesSync` | ⬜ pending |
| 24-01 | 1 | FRB-05 | T-24-OVR-POISON | Unsigned overlay entry → source_count:1 (warn, not enforce alone) | unit | `go test ./internal/catalog/... -run TestLocalOverlayUnsignedIsWarnTier` | ⬜ pending |
| 24-01 | 1 | FRB-05 | — | Overlay + bumblebee match → source_count:2, confidence_tier:"enforce" | unit | `go test ./internal/catalog/... -run TestLocalOverlayPlusBumblebeeIsEnforce` | ⬜ pending |
| 24-01 | 1 | FRB-05 | — | Overlay entry appears in `MultiIndex.LookupAll` | unit | `go test ./internal/catalog/... -run TestMultiIndexQueriesOverlay` | ⬜ pending |
| 24-02 | 2 | FRB-01 | — | Confirmed-malicious adjudication arms TUI quarantine card (writes `catalog_quarantine` audit record) | integration | `go test ./internal/watch/... -run TestFirstResponderCorpusMaliciousArmsCard` | ⬜ pending |
| 24-02 | 2 | FRB-04 | — | Sentry watch added only when source_count >= 2 (enforce tier) | unit | `go test ./internal/watch/... -run TestFirstResponderCorpusSentryGate` | ⬜ pending |
| 24-02 | 2 | FRB-04 | — | Single-source (watch tier) does NOT elevate Sentry watch | unit | `go test ./internal/watch/... -run TestFirstResponderCorpusSingleSourceNoSentry` | ⬜ pending |
| 24-02 | 2 | FRB-02 | T-24-NOPURGE | No `quarantine.Purge` call from corpus path (static + behavioral) | negative | `grep -rn 'Purge(' internal/corpus/ internal/watch/firstresponder.go` returns 0 + `go test ./internal/watch/... -run TestFirstResponderCorpusNoPurge` | ⬜ pending |
| 24-03 | 3 | FRB-03 | — | Restore reverses a corpus-adjudication quarantine cleanly | unit | `go test ./internal/quarantine/... -run TestRestoreCorpusQuarantineEntry` | ⬜ pending |
| 24-03 | 3 | FRB-01..05 | — | Synthetic Nx Console round-trip evaluator gate (see below) | integration | `go test ./cmd/beekeeper/... -run TestRunCatalogsSyncFirstResponder` | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Evaluator Gate — Synthetic Nx Console Round-Trip

The PRD §4 Phase-2 gate: a confirmed local Nx-Console-style match arms the card and does NOT auto-purge; restore reverses a purge cleanly. Fixture = a confirmed-malicious, enforce-tier `corpus.CorpusRecord` for `npm:@nrwl/nx-console`. The gate asserts:

1. `ReadMaliciousRecords` returns the record.
2. `RunFirstResponder` (fake CrossReference returning a matching ScanHit) writes a `catalog_quarantine` audit record (FRB-01).
3. `sentry-targets.json` contains the package (source_count=2 ≥ threshold=2) (FRB-04).
4. `quarantine.List(...)` contains one armed entry (FRB-01).
5. NO `quarantine.Purge` was called (FRB-02).
6. `quarantine.Restore(...)` succeeds (FRB-03).
7. `MultiIndex.LookupAll("npm", "@nrwl/nx-console")` returns ≥ 1 match with `CatalogSource=="local-overlay"` (FRB-05).

---

## Wave 0 Requirements

- [ ] `internal/corpus/reader_test.go` — new file (RED skeletons for `ReadMaliciousRecords`)
- [ ] `internal/catalog/local_overlay_test.go` — new file (RED skeletons for overlay add/load/survival/permissions/tier)
- [ ] `internal/watch/firstresponder_test.go` — EXTEND (corpus-adjudication test cases)

*Existing Go test infrastructure covers everything else; no framework install needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Red=attacker / coral=Beekeeper TUI color semantic preserved on the corpus-armed card | FRB-03 | Bubble Tea terminal rendering; color semantic is visual | Run the TUI with a seeded corpus-malicious record; confirm the armed quarantine card uses the existing red/coral palette (no new colors introduced) |

*All other phase behaviors have automated verification.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies (Wave 0 RED skeletons in 24-01/24-02; every implementation task carries an `<automated>` `go test -run` command; the only exception is the 24-03 Task 4 human-verify checkpoint for the FRB-03 red/coral semantic, which is exempt by type)
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (3 test files: `corpus/reader_test.go`, `catalog/local_overlay_test.go`, `watch/firstresponder_test.go`)
- [x] No watch-mode flags
- [x] Feedback latency < 120s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-06-14
