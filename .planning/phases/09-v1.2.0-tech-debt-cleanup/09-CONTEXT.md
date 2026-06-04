# Phase 9 — v1.2.0 Tech-Debt Cleanup — Context

**Inserted:** 2026-06-04 (post-milestone-audit)
**Milestone:** v1.2.0 "Runtime Behavioral Hardening"
**Source:** `.planning/v1.2.0-MILESTONE-AUDIT.md` (status `tech_debt`) + `gsd-integration-checker` cross-phase report
**Scope decision:** "Everything" — Core 4 + security hardening (IN-01/02/03) + drift registry query (maintainer choice, 2026-06-04)

> ⚠ **Tooling note (carry into plan/execute):** `gsd-sdk` phase-number resolvers map bare `9` to the **archived v1.0.0** phase 9 dir (`.planning/milestones/v1.0.0-phases/09-policy-as-code-self-defense-capstone/`). The live phase dir is `.planning/phases/09-v1.2.0-tech-debt-cleanup/`. Pass explicit paths to all agents; update ROADMAP/REQUIREMENTS/STATE **manually**. Do not trust `init.plan-phase 9` / `state.begin-phase 9` / `phase.complete 9`.

## Why this phase exists

The v1.2.0 milestone audit found **no blockers** — all 17 requirements satisfied, all 3 phases verified passed, cross-phase integration fully WIRED end-to-end — but surfaced one **new release-gate robustness concern** plus a set of documented/deferred items. The maintainer chose to clear the debt before closing v1.2.0 rather than carry it to v1.3.0. This phase reopens v1.2.0 at Phase 9; v1.2.0 is **not** ready for close until Phase 9 completes & verifies.

## Definition of done

Every item below is fixed in code, covered by a test that would have caught the original gap, and the existing v1.2.0 test suite + release gates (live-binary E2E `-tags e2e`, fuzz `-tags fuzz`) stay green. No regression to the F1/F2/F3 enforcement behavior proven in Phases 6–8.

## Work items

### Core remediation

**CLEAN-01 — Hermetic CORR E2E release gate** *(highest value)*
- **Problem:** `internal/check/e2e_test.go` `TestE2ELiveBinary/CORR_aifigure_critical_block` only blocks because the compiled binary reaches **live OSV**. The seeded local catalog entry is unsigned + `Versions:["*"]` → corroboration yields `warn` alone (wildcard guard + unsigned source suppress escalation). Offline / rate-limited / egress-blocked CI flakes the release gate (exit 0 vs 1).
- **Fix:** Seed a **signed** local OSV-equivalent `ai-figure` entry (non-wildcard version, `CatalogSignature` set → `Signed:true`) in the E2E fixture so the CORR case blocks **offline**, mirroring the hermetic unit tests (`TestRunCheckAiFigureBlocks` uses `sha256:corr02-test-sig`). Alternatively gate/skip the case when OSV is unreachable so a network outage never red-lights a release — but prefer the hermetic-seed approach.
- **Proof:** E2E CORR case passes with network disabled (or with the OSV adapter stubbed/blocked); release gate is network-independent.

**CLEAN-02 — `config.LoadLayered` Nudge pointer merge (root cause)**
- **Problem:** `config.LoadLayered`'s `merge()` does not merge the `Config.Nudge *NudgeConfig` pointer field. Currently mitigated at the consumer layer (`defaultNudgeConfigHelper()` in `cmd/beekeeper/nudge.go`; `cfg.Nudge != nil` guard in `internal/check/handler.go:302-304`). Any future `LoadLayered` consumer reading `cfg.Nudge` without a nil-check silently gets `nil` instead of defaults.
- **Fix:** Merge the `Nudge` pointer in `LoadLayered`'s `merge()` (deep-merge or last-non-nil-wins consistent with other blocks); then remove the now-redundant consumer workarounds (or leave guards as defense-in-depth but document them as such).
- **Proof:** A layered-config test asserts `cfg.Nudge` is populated with defaults when no layer sets it, and overridden when a project layer sets `nudge.*`, without any consumer-side helper.

**CLEAN-03 — Phase 6 Nyquist reconcile**
- **Problem:** `.planning/phases/06-corroboration-severity-hardening/06-VALIDATION.md` frontmatter is stale (`status: draft`, `nyquist_compliant: false`, `wave_0_complete: false`) despite `06-VERIFICATION.md` being `passed` 5/5.
- **Fix:** Run `/gsd-validate-phase 6` (explicit live path) to reconcile, or update the VALIDATION frontmatter to reflect the verified-passed reality with evidence. Bring Phase 6 to COMPLIANT.
- **Proof:** `06-VALIDATION.md` reflects compliant status consistent with its passed verification.

**CLEAN-04 — Stale comment `handler.go:261`**
- **Problem:** Comment says "The sensitive-path block is merged LAST" — accurate at Phase 7, now wrong: the NUDGE block (handler.go:290-309) is the actual last merge.
- **Fix:** Correct the comment to describe the real order: overlay → SPATH → NUDGE (most-restrictive-wins). Trivial.

### Security hardening (SPATH evasion edges — accepted-deferred in 07-REVIEW.md)

**HARDEN-01 — Symlink-ancestor bypass (IN-01)**
- **Problem:** `canonicalizePath`'s `EvalSymlinks` can resolve away a sensitive fragment when an **ancestor** directory is a symlink, letting a crafted path dodge the `DefaultSensitivePaths` match.
- **Fix:** Evaluate the sensitive-path match against **both** the pre- and post-`EvalSymlinks` forms (or match on the canonical AND the lexically-cleaned path), so an ancestor symlink cannot strip a `/.aws/` `/.ssh/` fragment. Keep the existing fail-closed-on-error behavior.
- **Proof:** Test plants an ancestor symlink and asserts the credential read still blocks.

**HARDEN-02 — Windows ADS / trailing-dot basename evasion (IN-02)**
- **Problem:** Windows Alternate Data Streams (`id_rsa:stream`) and trailing-dot/space basenames (`credentials. `) can evade basename/glob matching.
- **Fix:** Normalize Windows basenames in `canonicalizePath`/`isAllowedPath` — strip ADS suffix (`:streamname`) for match purposes and trim trailing dots/spaces — before evaluating against the blocklist. Fail-closed.
- **Proof:** Windows-gated tests assert `id_rsa:$DATA`, `credentials.`, `credentials ` all block.

**HARDEN-03 — Shell verb word-boundary (IN-03)**
- **Problem:** `extractBashCredentialPaths` verb matching can false-positive on tokens like `more`/`less` substrings without a word boundary.
- **Fix:** Apply word-boundary matching to the read-verb set (`cat`, `head`, `tail`, `less`, `more`, `type`, `Get-Content`, `gc`) so only standalone verbs trigger extraction.
- **Proof:** Test asserts a command containing a verb-substring (e.g. `formore.sh`) does not false-trigger, while real `more ~/.ssh/id_rsa` still flags.

### Feature completion

**DRIFT-01 — `realMetadataFetch` real registry query**
- **Problem:** `internal/gateway/drift.go:57-63` `realMetadataFetch` returns an empty map, so the production weekly `version_drift` check never emits a record (wiring/schema/`IsMajorDrift`/async behavior are all tested via injected `metadataFetchFn`, but production fetch is a stub).
- **Fix:** Implement a real registry metadata query (npm registry / equivalent) behind the existing `metadataFetchFn` seam so `checkDrift` emits live `version_drift` records. **Floor auto-update stays Out-of-Scope** — drift is informational only; never auto-bump pnpm/bun/node floors.
- **Proof:** With the real fetch, the gateway drift scheduler emits a well-formed `record_type:"version_drift"` audit record when the upstream major exceeds the configured floor; fail-open on fetch error preserved (never blocks).

## Constraints (unchanged from milestone)

- `internal/policy` and `internal/nudge`/`internal/pkgparse` stay **pure** (import-purity tests must remain green). All FS/env/network I/O in adapters.
- **Fail-closed** for catalog/path enforcement; nudge **detection** stays fail-open by design.
- No new heavy deps for HARDEN/DRIFT — prefer stdlib (`net/http` for the registry query is fine).
- `go test -race` is CGO-gated / CI-only on this Windows box; CI is authoritative for the race pass.

## Out of scope (stays deferred)

- pnpm/bun/node **floor auto-update** on drift (informational only — PRD §7.1).
- New TOML/YAML library dependency (hand scanners + fuzz remain).
- The `TestHookHandlerAllow` intermittent OSV-latency timeout (pre-existing, not v1.2.0; only touch if CLEAN-01's hermeticity work makes it free).

## Success criteria (phase-level)

1. CLEAN-01: CORR E2E release gate blocks offline (network-independent); `-tags e2e` green with OSV unreachable.
2. CLEAN-02: `LoadLayered` merges `Nudge`; consumer workarounds removed or documented as defense-in-depth; layered-config test proves defaulting + override.
3. CLEAN-03: Phase 6 VALIDATION.md COMPLIANT, consistent with its passed verification.
4. CLEAN-04: handler.go comment corrected.
5. HARDEN-01/02/03: each evasion edge blocked with a regression test that fails on the pre-fix code.
6. DRIFT-01: production drift emits live `version_drift` records via a real registry query; fail-open preserved; floors never auto-bumped.
7. Full v1.2.0 suite + fuzz + live-binary E2E remain green; no F1/F2/F3 regression.

---
_Seeds `/gsd-plan-phase` for Phase 9. Discuss-phase optional (gaps are concrete) — may plan directly as Phase 8 did._
