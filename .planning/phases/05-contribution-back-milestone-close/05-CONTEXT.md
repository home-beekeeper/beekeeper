# Phase 5: Contribution-Back & Milestone Close - Context

**Gathered:** 2026-06-03
**Status:** Ready for planning
**Source:** PRD Express Path (beekeeper-m2-prd.md §6, §9.5/9.6, §10, §11 M2.5, §12) + maintainer decisions (this session)

<domain>
## Phase Boundary

The **final phase** of milestone v1.1.0 "Pollen". It closes the milestone by making the fork stand on its own: a documented sync workflow, beekeeper consuming a pinned published Pollen, the Windows honeypot E2E proving Sentry fires on Windows, `pollen-self` self-quarantine entries, and the deferred signed releases finally cut.

**Three repos in play:**
- **`home-beekeeper/pollen`** (sibling clone at `../pollen`) — UPSTREAM.md sync doc, VERSION/CHANGES for `pollen.5`, and the signed-release pipeline (`.goreleaser.yaml`, `.github/workflows/release.yml` already exist).
- **`beekeeper`** (this repo) — go.mod/CI Pollen pin (BKINT-02), Windows honeypot E2E (PTEST-05), `pollen-self` catalog entries (SDEF-01).
- **`perplexityai/bumblebee`** (third-party upstream) — **NO outward action this phase** (see locked decision D-2).

**Requirements:** SYNC-01, SYNC-02 (descoped — see D-2), BKINT-02, PTEST-05, SDEF-01.

**What "milestone complete" means after this phase** (roadmap SC1–SC6, as amended by maintainer decisions):
1. UPSTREAM.md documents a repeatable sync workflow a second maintainer could follow (SC1 / SYNC-01).
2. ~~An upstream PR is open against perplexityai/bumblebee~~ → **SC2 RELAXED/DEFERRED** (D-2): no contribution-back PRs this milestone.
3. Beekeeper go.mod pins Pollen at an explicit version; beekeeper CI installs Pollen and all inventory tests pass on ubuntu/macos/windows with **zero skips** (SC3 / BKINT-02).
4. Windows Sentry honeypot E2E fires the exfil-signature-fusion rule on the Windows CI runner (SC4 / PTEST-05).
5. `beekeeper-self` contains `pollen-self` entries; `beekeeper selftest` passes with the extended catalog (SC5 / SDEF-01).
6. `v0.1.1-pollen.5` tagged + signed — the milestone-complete tag, cut together with the deferred pollen.2/3/4 (SC6).

</domain>

<decisions>
## Implementation Decisions

### D-1 — Context source: PRD-derived
CONTEXT generated from the PRD (no discuss-phase), per maintainer choice this session.

### D-2 — SYNC-02 DESCOPED; SC2 relaxed/deferred (maintainer decision)
**No contribution-back PRs against `perplexityai/bumblebee` this milestone.**
- **Rationale (maintainer):** Upstream already has open Windows-support PRs (#3, #4) that are being ignored while other PRs get feedback/merged; a commenter was told the maintainers plan their own Windows update. Contribution-back has no viable path right now — we cannot rely on upstream, so the fork stands on its own.
- **What still happens:** SYNC-01 (UPSTREAM.md) records the prepared Windows patch set and this contribution-back rationale so the intent is documented and re-openable later. SELF-02 (separate catalog) stays v2.
- **SC2 disposition:** explicitly deferred to a future milestone; the verifier MUST NOT flag the absence of an upstream PR as a Phase-5 gap.

### D-3 — GitHub push is IN SCOPE this phase (reverses local-only posture)
Both `home-beekeeper/pollen` and `home-beekeeper/beekeeper` may be pushed to GitHub this phase. Neither is pushed yet (v1.0.0 stayed local). Rationale: with upstream unreliable, the fork needs its own published, signed release; BKINT-02's `go.mod` pin to a published Pollen version depends on Pollen being on GitHub.

### D-4 — Cut all four signed tags this phase
`v0.1.1-pollen.2`, `.3`, `.4` (the three deferred per D-06 precedent) **and** `.5` (the milestone-close tag) are all cut this phase via cosign keyless signing through GitHub Actions OIDC (the `.goreleaser.yaml` + `release.yml` pipeline already exists in `../pollen`). Exact deferred-release commands are recorded in `.planning/phases/02-windows-root-resolver/02-04-SUMMARY.md` and `.planning/phases/03-windows-path-representation/03-03-SUMMARY.md`.

### D-5 — Outward/auth-gated steps are CHECKPOINTED (Claude's synthesis — overridable at plan review)
GitHub-facing, irreversible, auth-gated actions — `gh repo create`, `git push`, pushing tags that trigger the signed-release workflow, and `cosign verify` — are planned as **`autonomous: false` checkpoint tasks**. The executor does ALL local preparation (VERSION/CHANGES bumps, UPSTREAM.md, go.mod pin edits, honeypot test, pollen-self entries, and an exact copy-paste **release runbook** + verification scripts) autonomously; the **maintainer performs/approves the GitHub-facing steps** with their own `gh`/cosign auth. This honors "we can push to GitHub" while keeping irreversible outward actions under maintainer control. If the maintainer prefers full autonomy or pure prepare-only, adjust at plan-review/execution time.

### D-6 — SYNC-01: UPSTREAM.md repeatable sync workflow ships
UPSTREAM.md documents the §6.2 8-step sync workflow: (1) `git remote update upstream`; (2) diff-review the pinned→target commit range (new files / NDJSON schema / root resolver / LICENSE+NOTICE); (3) run upstream tests on Linux+macOS; (4) cherry-pick/merge preserving Windows code paths; (5) run Pollen's full 3-OS CI matrix; (6) re-run the differential test (byte-for-byte vs upstream on Linux/macOS); (7) update the pinned commit + CHANGES.md; (8) tag a new `v0.1.1-pollen.N`. Must be followable by a second maintainer without prior context (SC1). UPSTREAM.md already exists in `../pollen` — extend/verify it, don't recreate.

### D-7 — BKINT-02: beekeeper consumes a pinned, published Pollen; Windows CI flips fully green
Beekeeper pins Pollen at an explicit version (no auto-update; bumps require explicit beekeeper PRs). Beekeeper CI installs Pollen and runs the compatibility test (PTEST-04, already landed) + the honeypot E2E (PTEST-05) so the Windows inventory-test skip baseline is **zero** — flipping Windows CI from "skipped Bumblebee tests" to fully green.
- **RESEARCH must resolve:** beekeeper currently consumes Pollen as a **subprocess binary** (`internal/scan` invokes the `pollen` binary — BKINT-01), NOT a Go-module import. The PRD §5.1 envisions a Go-module dependency. Determine whether BKINT-02's "go.mod pins Pollen" means (a) CI `go install github.com/home-beekeeper/pollen/cmd/pollen@v0.1.1-pollen.4` of the binary at a pinned version, or (b) an actual Go-module import added to beekeeper's go.mod, or (c) both. Pick the smallest faithful interpretation consistent with the live subprocess boundary and PRD intent; reconcile in the plan.

### D-8 — PTEST-05: Windows honeypot E2E on the existing ETW Sentry
A planted process tree on Windows that reads **synthetic** `%USERPROFILE%\.aws\credentials` (NOT real credentials) and makes an outbound connection must fire beekeeper's **exfil-signature-fusion** rule (in `internal/sentry/rules.go`) on the Windows runner. Builds on the v1.0.0 Windows ETW Sentry (`internal/sentry/windows/`). The test asserts the expected alert is emitted.

### D-9 — SDEF-01: `pollen-self` entries in the unified `beekeeper-self` catalog
Add `pollen-self` entries to the existing `beekeeper-self` catalog (`internal/catalog/selfcatalog.go`) so a known-bad Pollen release is detectable by beekeeper's self-quarantine. Unified catalog (NOT a separate `pollen-self` catalog — SELF-02 is explicitly v2). `beekeeper selftest` must pass with the extended catalog.

### Claude's Discretion
- Plan/wave structure across the three repos; sequencing of the signed-tag cuts (pollen.2→3→4→5 in order, since each builds on prior VERSION/CHANGES state).
- Exact `pollen-self` entry shape (version identifiers, hashes) consistent with the existing `beekeeper-self` schema in `selfcatalog.go`.
- Honeypot test harness layout (synthetic fixture tree, planted process simulation) consistent with the existing Sentry rule-fixture pattern.
- Whether BKINT-02 needs a beekeeper CI workflow edit (`.github/workflows/ci.yml`) to `go install` Pollen, and how the Windows job resolves the `pollen` binary on PATH.
- The precise content/structure of the release runbook produced for the checkpointed steps (D-5).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### PRD — authoritative spec for this phase
- `beekeeper-m2-prd.md` §6 (upstream sync discipline — pinning, the §6.2 8-step workflow, §6.3 threat-intel reference model) → SYNC-01
- `beekeeper-m2-prd.md` §9.5 (Windows honeypot variant) + §9.6 (CI matrix flips Windows green) → PTEST-05, BKINT-02
- `beekeeper-m2-prd.md` §10 (self-defense; §10.2 `pollen-self` entries, pinned Pollen version, SBOM) → SDEF-01, BKINT-02
- `beekeeper-m2-prd.md` §11 M2.5 + §12 (contribution-back) → context for the DESCOPED SYNC-02

### Requirements
- `.planning/REQUIREMENTS.md` — SYNC-01/02, BKINT-02, PTEST-05, SDEF-01 definitions; SELF-02/DIST-01 are v2 (out of scope)

### Deferred-release state (cut this phase)
- `.planning/STATE.md` "Deferred Items" table — the pollen.2/3/4 deferred signed tags + exact cut sequence
- `.planning/phases/02-windows-root-resolver/02-04-SUMMARY.md` — exact pollen.2 release commands
- `.planning/phases/03-windows-path-representation/03-03-SUMMARY.md` — pollen.3 release-prep + deferral precedent
- `.planning/phases/04-windows-extension-mcp-coverage-beekeeper-compat-test/04-03-SUMMARY.md` — pollen.4 release-prep

### Live code (read before editing)
- `../pollen/UPSTREAM.md` — existing sync doc to extend (SYNC-01)
- `../pollen/VERSION`, `../pollen/CHANGES.md` — bump to pollen.5
- `../pollen/.goreleaser.yaml`, `../pollen/.github/workflows/release.yml` — the existing cosign/SBOM signed-release pipeline (D-4)
- beekeeper `internal/sentry/rules.go` — the exfil-signature-fusion rule (PTEST-05)
- beekeeper `internal/sentry/windows/` — Windows ETW Sentry the honeypot exercises (PTEST-05)
- beekeeper `internal/catalog/selfcatalog.go` + `selfkey.go` — `beekeeper-self` catalog to extend with `pollen-self` (SDEF-01)
- beekeeper `internal/scan/scanner.go` — the BKINT-01 `pollen` subprocess seam (informs BKINT-02 pin shape)
- beekeeper `.github/workflows/ci.yml` — where Pollen install + Windows honeypot job land (BKINT-02)

</canonical_refs>

<specifics>
## Specific Ideas

- The honeypot credentials MUST be synthetic (`%USERPROFILE%\.aws\credentials` planted fixture), never real — this is a test artifact (PRD §9.5).
- Two-account release approval (PRD §10.2 / §12.2 two-person rule) is the discipline behind D-5's checkpointing — the signed release is a deliberate gated step, not a silent automated one.
- Cut the four tags in order (pollen.2 → .3 → .4 → .5); each release's VERSION/CHANGES state was prepared by its originating phase, so the runbook replays them in sequence.
- UPSTREAM.md sync workflow must be concrete enough that a second maintainer can run it cold (SC1) — real commands, not prose.
- The Windows-CI-skip baseline for beekeeper inventory tests must be **zero** after BKINT-02 (the milestone's whole point) — PTEST-04 already achieved zero skips for the compat test; BKINT-02 must not reintroduce any.
- `beekeeper selftest` must stay green with the extended `pollen-self` catalog (SDEF-01).

</specifics>

<deferred>
## Deferred Ideas

- **SYNC-02 / roadmap SC2** (upstream contribution-back PRs to perplexityai/bumblebee) — DEFERRED to a future milestone per maintainer decision D-2 (upstream Windows-support path is unviable now). Prepared patch set + rationale documented in UPSTREAM.md.
- **SELF-02** (separate `pollen-self` catalog distinct from `beekeeper-self`) — v2; this phase uses the unified catalog (D-9).
- **DIST-01** (public Pollen binary releases) — v2; source-only via `go install` stands.

</deferred>

---

*Phase: 05-contribution-back-milestone-close*
*Context gathered: 2026-06-03 via PRD Express Path + maintainer decisions*
