---
phase: 24-first-responder-corpus-binding
audited: 2026-06-14
auditor: gsd-security-auditor (claude-sonnet-4-6)
asvs_level: 1
status: verified
threats_open: 0
threats_closed: 13
threats_total: 13
block_on: high
---

# Phase 24 — First Responder Corpus Binding: Security Audit

**Phase:** 24 — First Responder Corpus Binding (v1.4.0)
**Threats Closed:** 13/13
**ASVS Level:** 1
**Audited:** 2026-06-14

---

## Threat Verification

| Threat ID | Category | Disposition | Evidence |
|-----------|----------|-------------|----------|
| T-24-OVR-TAMPER | Tampering | mitigate | `internal/catalog/local_overlay.go:99,103,110,114` — `writeFileAtomic` on JSON, `platform.SetOwnerOnly` on both `.json` and `.idx`; `TestLocalOverlayFilePermissions` PASS (SKIP on Windows, expected — SetOwnerOnly applies DACL) |
| T-24-OVR-POISON | Tampering (poisoning) | mitigate | `internal/catalog/local_overlay.go:68` doc asserts `CatalogSignature` MUST be `""`; `TestLocalOverlayUnsignedIsWarnTier` PASS; `TestLocalOverlayPlusBumblebeeIsEnforce` PASS — two distinct CatalogSource values returned, corroboration engine counts 2 sources for enforce |
| T-24-OVR-DOS | DoS | mitigate | `internal/corpus/reader.go:38-41,57-60,49` — `os.IsNotExist` → `nil,nil`; malformed line `json.Unmarshal` error → `continue`; `maxRecordsToScan` cap + 1MB buffer; `TestReadMaliciousRecords/missing_file_returns_nil_nil` PASS; `TestReadMaliciousRecords/malformed_line_skipped_valid_returned` PASS |
| T-24-OVR-RACE | Tampering (integrity) | accept | `writeFileAtomic` + `BuildIndex` internal atomic-rename bound corruption to last-writer-wins; accepted same-class as `state.json` race; local-only, no fleet push — rationale documented in 24-01-PLAN.md threat register and in `local_overlay.go` file header |
| T-24-NOPURGE | Tampering / blast radius | mitigate | `grep "Purge(" internal/watch/firstresponder.go` → 0 matches; `TestFirstResponderCorpusNoPurge` PASS (quarantine entry survives, reversible); `TestCorpusPathHasNoPurgeCall` PASS (static gate — both sub-tests: `watch/firstresponder.go` and `corpus/reader.go`) |
| T-24-SENTRY-SINGLESRC | Spoofing / Elevation | mitigate | `internal/watch/firstresponder.go:237` — `rec.PushEnvelope.SourceCount >= corpusThreshold` guards `targets.AddTarget`; `TestFirstResponderCorpusSentryGate` PASS (SourceCount=2 → target added); `TestFirstResponderCorpusSingleSourceNoSentry` PASS (SourceCount=1 → no target); watch is DETECTION-ONLY |
| T-24-AUDIT-REDACT | Info disclosure | mitigate | `internal/watch/firstresponder.go:313,375` — `audit.RedactRecord(rec, audit.DefaultRedactPatterns())` called in both `writeFirstResponderAudit` and `writeCorpusFirstResponderAudit` (the corpus-path audit helper) before every write |
| T-24-CORPUS-FAILOPEN | DoS | accept | `internal/watch/firstresponder.go:199-201` — corpus read error is logged (`log.Printf`) and the `else` block is skipped; scan-hit quarantine results already computed and persist; corpus is additive, not the primary gate; rationale: primary scan-hit fail-closed path is unchanged |
| T-24-SYNC-FAILOPEN | DoS | mitigate | `cmd/beekeeper/catalogs_daemon.go:136-175` — all three FRB calls (`firstResponderFn` L140, `corpus.ReadMaliciousRecords` L160, `catalog.AddLocalOverlayEntry` L169) use non-fatal `fmt.Fprintf(os.Stderr, ...)` error branches; none returns from `runCatalogsSync`; FRB block runs before HTTP fetch (L131 comment + ordering confirmed in code) |
| T-24-NOPURGE-CMD | Tampering / blast radius | mitigate | `grep "Purge(" cmd/beekeeper/catalogs_daemon.go` → 0 matches; confirmed by Grep tool search returning no results |
| T-24-OVERLAY-UNSIGNED | Tampering (poisoning) | mitigate | `cmd/beekeeper/catalogs_daemon.go:272` — `buildOverlayEntry` sets `CatalogSignature: ""` with inline comment `MUST be empty — unsigned → warn-only per Pitfall 3`; `TestLocalOverlayUnsignedIsWarnTier` PASS |
| T-24-CARD-COLOR | Spoofing (UI) | mitigate | Human-gate satisfied: corpus path reuses existing `catalog_quarantine` / `pending-quarantine` audit record types (no new type introduced); TUI renders these with locked coral `#f0883e` / red `#f85149` palette; maintainer APPROVED FRB-03 visual checkpoint during execution on 2026-06-14 (recorded in `24-03-SUMMARY.md` Task 4 + `24-VERIFICATION.md` human_verification section) |
| T-24-SC | Tampering (supply chain) | mitigate | `go mod tidy && git diff --exit-code go.mod` → exit 0, no change; `internal/corpus/reader.go` imports stdlib only (`bufio`, `encoding/json`, `fmt`, `os`); `internal/catalog/local_overlay.go` imports stdlib + existing `internal/platform` only; zero new external dependencies |

---

## Evidence Summary by Plan

### 24-01: Corpus Reader + Local Overlay + MultiIndex Extension

| Test | Result | Covers |
|------|--------|--------|
| `TestReadMaliciousRecords` (5 sub-tests) | PASS | T-24-OVR-DOS, T-24-OVR-POISON (label filter) |
| `TestReadMaliciousRecordsLatestPerCluster` | PASS | T-24-OVR-DOS (scan collapse) |
| `TestLocalOverlaySurvivesSync` | PASS | T-24-OVR-TAMPER (sync immunity) |
| `TestLocalOverlayFilePermissions` | SKIP (Windows; expected) | T-24-OVR-TAMPER (DACL path) |
| `TestMultiIndexQueriesOverlay` | PASS | T-24-OVR-POISON (source attribution) |
| `TestLocalOverlayUnsignedIsWarnTier` | PASS | T-24-OVR-POISON, T-24-OVERLAY-UNSIGNED |
| `TestLocalOverlayPlusBumblebeeIsEnforce` | PASS | T-24-OVR-POISON (two-source enforcement shape) |
| `TestLocalOverlayIdempotentAdd` | PASS | T-24-OVR-DOS (cap + idempotency) |

### 24-02: RunFirstResponder Corpus Loop

| Test | Result | Covers |
|------|--------|--------|
| `TestFirstResponderCorpusMaliciousArmsCard` | PASS | T-24-AUDIT-REDACT, FRB-01 |
| `TestFirstResponderCorpusSentryGate` | PASS | T-24-SENTRY-SINGLESRC |
| `TestFirstResponderCorpusSingleSourceNoSentry` | PASS | T-24-SENTRY-SINGLESRC |
| `TestFirstResponderCorpusNoPurge` | PASS | T-24-NOPURGE (behavioral) |
| `TestFirstResponderCorpusPendingQuarantine` | PASS | T-24-CORPUS-FAILOPEN rationale |
| `TestCorpusPathHasNoPurgeCall` (2 sub-tests) | PASS | T-24-NOPURGE (static gate) |

### 24-03: runCatalogsSync Wiring + Evaluator Gate

| Test | Result | Covers |
|------|--------|--------|
| `TestRestoreCorpusQuarantineEntry` | PASS | T-24-CARD-COLOR (FRB-03 reversibility) |
| `TestRunCatalogsSyncFirstResponder` (7 assertions) | PASS | T-24-SYNC-FAILOPEN, T-24-NOPURGE-CMD, T-24-OVERLAY-UNSIGNED, T-24-SC, FRB-01..05 |

---

## Accepted Risk Log

### T-24-OVR-RACE — Concurrent overlay write race

**Disposition:** accept

**Rationale:** `writeFileAtomic` (atomic temp-file + rename) for the JSON and `BuildIndex`'s internal atomic rename bound the corruption window to last-writer-wins. This is the same race class accepted for `state.json` in prior phases. The overlay is local-only (no fleet push); concurrent writers are limited to the operator's own machine; the race window is rename-duration, not a multi-step tear. A file lock would require platform-specific logic that adds surface area exceeding the risk in v1. Documented in 24-01-PLAN.md threat register (T-24-OVR-RACE row).

### T-24-CORPUS-FAILOPEN — Corpus read error skips corpus block

**Disposition:** accept

**Rationale:** A `corpus.ReadMaliciousRecords` error inside `RunFirstResponder` is logged and the corpus block is skipped. The primary scan-hit quarantine path (already computed before the corpus block at `firstresponder.go:130-185`) is unaffected — scan-hit results persist regardless. The corpus signal is additive: it can arm additional quarantine entries, but its failure cannot remove an already-armed entry. The primary fail-closed guarantee (catalog-corroborated scan-hits are quarantined) is preserved. Documented in 24-02-PLAN.md threat register (T-24-CORPUS-FAILOPEN row).

---

## Unregistered Flags

None. No `## Threat Flags` section was present in any of the three SUMMARY files (24-01, 24-02, 24-03). The executor did not flag new attack surface beyond the pre-registered threat register.

---

## Human-Verified Gates

| Gate | Verified By | Date | Result |
|------|-------------|------|--------|
| T-24-CARD-COLOR — TUI red/coral palette on corpus-armed quarantine card (FRB-03) | Maintainer visual inspection during `/gsd-execute-phase 24` Task 4 | 2026-06-14 | APPROVED — corpus-armed card uses existing coral `#f0883e` / red `#f85149` palette; [r]estore and p→y-gated [p]urge actions render correctly; no new colors introduced |

---

## Notable Implementation Notes

**Rule-1 config-merge fix (layered.go):** A production defect found during 24-03 green iteration: `internal/config/layered.go` `merge()` and `mergeUntrusted()` were missing `Corpus` and `AutoQuarantine` struct handling, causing `cfg.Corpus.Enabled` to always be `false` (Go zero-value) after `resolveConfig()`. This would have permanently skipped the entire FRB adjudication pass in production. Fixed inline: `mergeCorpus` (L873) and `mergeAutoQuarantine` (L890) added and wired at L259 (`merge`) and L355 (`mergeUntrusted`). The evaluator gate (`TestRunCatalogsSyncFirstResponder`) would have failed without this fix — it is now the regression guard.

**Import boundary (ADJ-01 / Pitfall 5):** `ReadMaliciousRecords` is documented as off-hot-path only (must not be called from `internal/check/handler.go`). The restriction is enforced by code comment and by the fact that no `internal/corpus` import exists in `internal/check/` (not modified in this phase).
