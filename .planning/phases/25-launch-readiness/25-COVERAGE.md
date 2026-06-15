---
artifact: coverage-hardening
milestone: v1.4.0
phase: 25-launch-readiness
date: 2026-06-15
bar: "100% logic + reason-coded allowlist for unreachable wiring/error branches"
status: complete
---

# v1.4.0 Coverage Hardening & System E2E — Report

Hand-managed coverage-hardening pass run before milestone closure (maintainer-requested: "100% unit and e2e tests, tested alone then as part of system-wide e2e validation"). Test-only — **zero product-code changes, zero new deps**. Bar agreed with maintainer: **100% on v1.4.0 logic + security functions; a reason-coded allowlist for genuinely unreachable wiring / fault-injection-only branches** (the project's established Phase-21 `coveragegate` discipline, not blanket 100%).

## Package coverage (before → after)

Package totals are diluted by large **pre-v1.4.0** code in catalog/watch/cmd that is out of this milestone's scope; the per-function table below is the real measure of the v1.4.0 surface.

| Package | Before | After |
|---------|--------|-------|
| internal/corpus | 75.1% | **91.7%** |
| internal/config | 77.3% | **81.7%** |
| internal/check | 87.2% | **88.1%** |
| internal/watch | 70.5% | **74.7%** |
| internal/catalog | 73.8% | **74.3%** |
| cmd/beekeeper | 24.0% | **25.5%** |

## v1.4.0 functions brought to 100% (logic)

- **config (Phase-24 prod-bug regression-lock):** `mergeCorpus` 57→**100%**, `mergeAutoQuarantine` 0→**100%** — the exact functions whose missing merge made `cfg.Corpus.Enabled` always false in prod; now unit-locked (previously proven only by one E2E gate).
- **corpus:** `OperatorAdjudication` 0→**100%** (all 4 operator sources + confidence mapping), `ResolveCorpusPath` 0→**100%** (boundary accept/reject), `readSaltFile` 78→**100%**, `deriveWasCorrect` 73→**100%**, `clusterKeyOf` 67→**100%**, `isPurgeClassIntent` 70→**100%**, `MapToCorpusRecord` 81→**100%**, `stripHomeSeg`/`allDigits`→**100%**.
- **watch:** `defaultFirstResponder` 0→**100%**, `parsePackageID` 75→**100%**, `ecosystemToProcess` 25→**100%**, `marshalTargetListJSON` 0→**100%**.
- **catalog:** `LoadLocalOverlay` 80→**100%**.
- **check:** `writeAudit` (deprecated wrapper) 0→**100%**.
- **cmd:** `offerCatalogSyncDaemon` 0→**100%**.

Partials raised (remainder = allowlisted error branches below): `RunAdjudicationBatch` 76→97.9%, `ReadMaliciousRecords` 83→94.3%, `store.Write` 78→88.9%, `AppendCorpusRecordLine` 71→85.7%, `RunFirstResponder` 84→88.9%, `writeFirstResponderAudit` 83→91.7%, `writeCorpusFirstResponderAudit` 85→92.3%, `writeAuditWithAC` 75→91.7%, `runCatalogsSync` 65→84.6%, `AddLocalOverlayEntry` 67→79.2%, `writeCorpusRecord` 56→71.9%, `LoadOrCreateSalt` 52→55.6%, `NewStoreSink` 56→66.7%.

## Reason-coded allowlist — intentionally uncovered branches

Every remaining uncovered v1.4.0 line falls in one of these categories. None is reachable without changing product code to add a fault-injection seam (which the chosen bar deliberately avoids).

| Reason code | What | Examples (file:line) |
|-------------|------|----------------------|
| `crypto-rand-fault` | `crypto/rand` cannot be faulted from a test | adjudicator.go:464 (record-id fallback); fingerprint.go:124 (salt gen); signer.go:140 (nonce) |
| `os-fault-injection` | `MkdirAll`/`OpenFile`/`Write`/`Close`/`SetOwnerOnly` errors need disk-full / kernel hooks; Windows DACL doesn't express the needed failure | store.go:52,64,120,189; fingerprint.go:118,144; signer.go:49-69; local_overlay.go:99-116; firstresponder.go:315-318,377-380; handler.go:535 |
| `unreachable-json-marshal` | `json.Marshal` of a fixed struct with only JSON-safe fields cannot fail | store.go:174 (AppendCorpusRecordLine); signer.go:131 (canonicalSigningInput); firstresponder.go writeFirstResponderAudit marshal |
| `platform-unavailable` | only fails when `%APPDATA%`/`UserConfigDir` is absent, or salt is non-hex (LoadOrCreateSalt guarantees hex) — unreachable on a functioning OS in CI | handler.go:574 (StateDir fallback), 603 (RepoFingerprint); catalogs_daemon.go:59-79,136 (CatalogDir/StateDir/AuditDir) |
| `runtime-cost` | the 50,000-record scan-cap break requires writing 50k+ NDJSON records | adjudicator.go:269; reader.go:49 |
| `thin-cobra-wiring` | pure Cobra command construction; sub-commands delegate to separately-tested funcs (per CLAUDE.md thin-cmd rule) | catalogs_daemon.go:283 (newCatalogsDaemonCmd); :29 (default firstResponderFn plumbing); :218 (304 NotModified — unexported URL, no injectable test server from cmd) |

To reach literal 100% these would require introducing fault-injection seams into product code (injectable `platform`/`crypto-rand`/`audit.NewWriter` vars). That was explicitly declined in favor of this documented allowlist.

## E2E coverage

**Per-component, "tested alone" (in-process integration — pre-existing, all green):** `TestRunCatalogsSyncFirstResponder` (Nx Console round trip), `TestAllSentryPatternsProduceMoatRecord` (8 patterns), `TestOfflineProtective` (catalog-backed offline block), `TestBenchmarkRunCheckGate` (p99), plus the new unit suites above.

**System-wide, real-binary (NEW this pass):** `TestCorpusMoatLoopE2E` (`//go:build e2e`, `cmd/beekeeper/corpus_moat_e2e_test.go`, commit `ae755fd`) — builds the real `beekeeper` binary and drives the full moat loop: live `beekeeper check` hot-path corpus write → live `beekeeper catalogs sync` (adjudication batch + first responder + overlay) → asserts four-layer `corpus.ndjson` record with 64-hex STORED signature, owner-only `local-overlay.json/.idx` written with the confirmed entry, `sentry-targets.json` armed (SourceCount≥2), NO auto-purge, and a SECOND `beekeeper check` of the package now BLOCKED by the overlay (loop closed). One pre-adjudicated moat-fixture record is seeded for deterministic four-layer assertion; every feedback stage (sync→adjudicate→first-responder→overlay→catch) runs through the live binary.

## System-wide validation result (2026-06-15)

- `go test ./...` — green, 27 packages (all unit + in-process integration together)
- `go vet ./...` — clean
- `go test -tags e2e ./cmd/beekeeper/... -run TestCorpusMoatLoopE2E` — **PASS** (74s)
- `go test -tags e2e ./internal/check/... -run TestE2ELiveBinary` — **PASS** (Phase-21 hook canary, no regression)
- `go mod tidy && git diff --exit-code go.mod go.sum` — zero dependency change
- (LlamaFirewall e2e remains CI/Linux-only by design — semgrep has no native Windows build.)

## Commits

`21e88c8` corpus unit · `a53beca` config/catalog/watch unit · `60f4610` check/cmd unit · `ae755fd` system moat-loop e2e.
