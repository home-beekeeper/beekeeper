---
phase: 25
slug: 25-launch-readiness
status: verified
threats_open: 0
asvs_level: 1
created: 2026-06-14
---

# Phase 25 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail. Phase 25 (Launch Readiness) ships **tests + docs only** over code built in Phases 22-24 — no new product code, no new attack surface, no new auth/crypto/IPC/network path. Register authored at plan time (all 3 plans carried `<threat_model>` blocks); verified by gsd-security-auditor (State B). **8/8 threats closed, 0 open.**

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| test-input → corpus mapper | Synthetic AuditRecords / seed records cross into `MapToCorpusRecord` / `BuildPushEnvelope`; a malicious test could try to seed a purge-class intent | synthetic test fixtures (no real data) |
| disk catalog index → runCheck | The mmap catalog is the last-synced threat intel; offline, it is the sole defense boundary | threat-intel lookups |
| corpus store → (no network) | LAUNCH-04 asserts this boundary is sealed: `store.go` has no network import | corpus records (must stay local) |
| documentation → operator trust | THREAT-MODEL.md sets operator expectations; understated gaps create false confidence (a real, if non-code, attack on the trust boundary) | residual-gap disclosures |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-25-INJECT | Tampering | LAUNCH-02 synthetic AuditRecords → `MapToCorpusRecord`/`BuildPushEnvelope` | mitigate | `internal/corpus/launch_e2e_test.go:218` asserts `ActionHint == ActionHintWatchAndBlock`; `ActionHint` typed const with only `ActionHintWatchAndBlock` pushable (SCHEMA-04, `internal/corpus/action_hint.go:36`); ENV-02 purge rejection `internal/corpus/emitter.go:207`; `FuzzBuildPushEnvelope` standing guard intact `internal/corpus/fuzz_test.go:107` | closed |
| T-25-PURGE | Tampering | LAUNCH-01 FRB feedback path | accept | Gate assertions #4/#5 `cmd/beekeeper/catalogs_daemon_test.go:274-279` prove quarantine entry survives (no auto-purge); `TestCorpusPathHasNoPurgeCall` static guard `internal/watch/nopurge_test.go:33`; no new purge surface | closed |
| T-25-SEED | Information Disclosure | test seed records in test files | accept | Seeds use `@nrwl/nx-console` public fixture + `launch02-repo-fp`/`launch02-node` sentinels; no real secrets/paths/PII; HMAC placeholder salts; test-only scope | closed |
| T-25-PERF | Repudiation | `TestBenchmarkRunCheckGate` p99 baseline | mitigate | `internal/check/handler_test.go:1554`: N=100, fixed budget 100ms (200ms Windows), p99 via sorted slice; absolute budget — a slow CI runner cannot silently inflate a passing baseline | closed |
| T-25-EXFIL | Information Disclosure | corpus store network egress | mitigate | `TestCorpusStoreHasNoNetworkImports` `internal/corpus/store_test.go:220`: `go/parser` ImportsOnly AST scan forbids `net`/`net/http`/`os/exec` (forward-slash keys, verified); `store.go` imports clean; combined with STORE-03 (owner-only, no remote sink) | closed |
| T-25-OFFLINE-FAILOPEN | Denial of Service / Tampering | offline runCheck decision | mitigate | `TestOfflineProtective` `internal/check/handler_test.go:1631`: asserts `res.Decision.Allow == false` (line 1661, fail-closed BLOCK) with no live network sources | closed |
| T-25-DOCS | Information Disclosure (false confidence) | THREAT-MODEL.md §13 residual-gap framing | mitigate | `TestThreatModelNamesResidualGaps` `cmd/beekeeper/threatmodel_names_test.go:16` asserts 4 verbatim strings present in `docs/THREAT-MODEL.md:1225`; blocking maintainer honesty checkpoint APPROVED live this session | closed |
| T-25-DOCS-OVERCLAIM | Repudiation | §13 local-first / no-exfil statement | mitigate | `docs/THREAT-MODEL.md:1229-1231` cites STORE-03 + `TestCorpusStoreHasNoNetworkImports` inline; the no-exfil claim is machine-verifiable, not unbacked prose | closed |

*Status: open · closed*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-25-01 | T-25-PURGE | The extended evaluator gate (assertions #4/#5) already proves no auto-purge occurs; `TestCorpusPathHasNoPurgeCall` (Phase 24 static gate) is the standing guard. Phase 25 adds only read-only assertions — no new purge surface. The existing two-layer guard (behavioral + static) is sufficient. | maintainer (via /gsd-secure-phase) | 2026-06-14 |
| AR-25-02 | T-25-SEED | Test seeds use publicly-known fixture identifiers (`@nrwl/nx-console`) and synthetic sentinels; no real credentials, repo paths, user identifiers, or PII; HMAC placeholder salts carry no secret material. Test-only scope. | maintainer (via /gsd-secure-phase) | 2026-06-14 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-06-14 | 8 | 8 | 0 | gsd-security-auditor (sonnet), State B, verified by orchestrator |

---

## Auditor Finding — Investigated, False Positive

The auditor flagged that the `TestCorpusStoreHasNoNetworkImports` forbidden-map keys appeared to use backslash separators (`"net\http"`, `"os\exec"`), which would silently no-op on Linux/macOS. **Investigated directly: this is a misread (rendering artifact).** `internal/corpus/store_test.go:221-225` uses correct forward-slash keys (`"net/http"`, `"os/exec"`); `\h`/`\e` would be invalid Go escape sequences and would not compile, and the full suite is green. The AST gate strips quotes from `imp.Path.Value` and matches forward-slash import paths correctly on all platforms. No fix required; T-25-EXFIL mitigation is sound.

---

## Notes

- Phase 25 ships tests + documentation only over code built in Phases 22-24 — no new network paths, auth/crypto, or IPC. Both 25-01/25-02 SUMMARY Threat Surface Scans report no new flags; 25-03 closes T-25-DOCS / T-25-DOCS-OVERCLAIM.
- `TestOfflineProtective` uses the malformed-JSON fail-closed path (the strongest offline proof: no catalog, no network, no policy evaluation needed) rather than a catalog-backed corroboration block, because the test catalog's single-source entry is warn-not-block (PLCY-01). Documented in 25-02-SUMMARY.md; does not weaken the LAUNCH-03 offline guarantee.
- LAUNCH-02 `TrueLabel="unresolved"` acceptance is RESEARCH Assumption A2 (the non-retrofittable outcome layer is moat-grade by being PRESENT, not resolved), documented verbatim in `internal/corpus/launch_e2e_test.go`. Maintainer-overridable; not overridden.
- The maintainer honesty checkpoint (25-03 Task 2, `gate="blocking"`) was APPROVED live this session.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-06-14
