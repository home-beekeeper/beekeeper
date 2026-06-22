# HANDOFF — milestone v1.5.0 "Install Posture" (ships as release v1.1.0)

> Resume doc for the nudge-removal + install-posture milestone. Read this, then
> `.planning/STATE.md` (Current Position) and memory `install-posture-arch.md`.
> Branch: **`feat/install-posture-foundation`** (all commits SSH-signed, NOT pushed).
> Last updated: 2026-06-22.

## Where we are

**Phases 26-29 COMPLETE + verified. GATE 1 PASSED. 5 of 6 phases done. ~34 signed commits.**

- **26** nudge removed; `internal/posture` (relocated pm-config readers); new git/remote-URL pure rule (`pkgparse.RemoteSource` + `policy.EvaluateRemoteSource`); shim repointed.
- **27** posture wired into the pre-exec hook (`internal/check/posture_adapter.go`) at WARN + FAIL-SOFT (unknown/timeout/missing-ts stays warn, never blocks); `SENTRY-009` detection-only install observation (cross-platform); canonical `posture.BoundaryStatement`/`BoundaryShort`; **shim made real** (`buildShimToolCall` reconstructs the install so catalog+posture actually fire — was a latent no-op). **GATE 1 PASSED** (maintainer ratified the map + boundary copy + warn/fail-soft default; resolved the shim; added IPOVR-03).
- **28** read-only `beekeeper posture` view (CLI `cmd/beekeeper/posture.go` + TUI panel + pure `internal/posture/view.go`); byte-for-byte read-only guarantee test; release-age figure derived from the policy default.
- **29** per-rule opt-up-to-block (`config.PostureConfig`, tighten-only untrusted merge; `posturizeWithAction` blocks a DEFINITE violation under block mode, unknown stays fail-soft) [IPOVR-03]; scoped override allow-once / allow-always / block via `beekeeper posture allow|enforce` [IPOVR-01/02]; **posture-scoped allowlist that can NOT bypass a catalog malware block** (proven by `TestRunCheckPostureAllowAlwaysDoesNotBypassCatalogBlock`); owner-only atomic allow-once store; distinct `posture_override` audit records; display-only TUI incident card.

All verified independently each phase: `go build/vet/test ./...` green; the load-bearing security properties proven on the live `RunCheck` path; new code em-dash-free.

## Decisions locked (do not relitigate)

- Public release **v1.1.0**; GSD milestone **v1.5.0** (avoids the parked-Pollen v1.1.0 collision).
- Default posture = **warn + fail-soft** at the hook (ratified at Gate 1). The unknown path (missing timestamp / registry error / timeout) stays warn even under block mode.
- **Shim kept + made real + documented experimental** (maintainer decision; bypassable by absolute path, requires PATH prepend).
- **IPOVR-03** (opt a rule up to block) is in v1.0 (maintainer-added at Gate 1); the finer per-ecosystem/per-project matrix stays roadmap.
- **Schema-freeze item: DECISION = KEEP the typed `posture_override_*` fields** (maintainer-confirmed 2026-06-22). Rationale: they are audit-only (written via plain `audit.NewWriter`, never routed to the corpus `StoreSink`), so no `CorpusRecord` content changes and the frozen moat schema is intact today. They are forward-useful for the future corpus rollout (PRD §6/§7): operator overrides are human ground-truth adjudications, a natural `adjudication_source: operator_override`; typed fields map cleanly into a future corpus adjudication layer where re-encoding would be throwaway. The proper `CorpusSchemaVersion` 1.0→1.1 bump + migration happens in THAT future milestone when overrides are deliberately ingested into the corpus (with `org_only`/`community_shareable` scope gating for privacy). Do NOT re-encode.

## NEXT — Phase 30 (Docs, Home Page & Boundary Statements) — IPBND-02, NMIG-03, REL-01 (prep)

1. **Verify the web/docs repo location FIRST.** Per memory [[beekeeper-web-split-repo]] the marketing/docs site is a SEPARATE repo at `C:/Users/Bantu/mzansi-agentive/beekeeper-web` (Next.js `web/`), with an accuracy gate that resolves `source_doc` frontmatter against the sibling `../beekeeper`. The home-page bullet + docs edits likely live THERE, not in this repo. Confirm before editing.
2. Replace the home page "Agent safety" **nudge** bullet with the install-posture framing + the nudge-obsolescence note (npm v12 blocks install scripts by default, so the nudge premise is gone; install posture is the tool-agnostic successor). No em dashes, sentence case.
3. Bring the install-posture docs current: the three rules + warn/fail-soft default + the `beekeeper posture` view + the scoped override (`posture allow|enforce`) + the per-rule opt-up-to-block. Propagate the boundary statement (`posture.BoundaryStatement`) everywhere it is described, to the harness-coverage honesty standard. Do NOT document roadmap items as shipped.
4. Document **SENTRY-009** (THREAT-MODEL / docs list SENTRY-001..008 today).
5. Remove all steer-to-pnpm/Bun copy from product + docs (the Go-side `docs/nudge.md` is stale per memory).
6. Bump version to **v1.1.0** + changelog entry (shipped install-posture feature + a roadmap note for the deferred layers: config mutation, per-ecosystem matrix, shim as a first-class surface). Signed commits, PR to main per the branch ruleset.

## THEN — Phase 31 (Test, Coverage & E2E + CI Matrix), THEN Gate 2

- Phase 31: complete the coverage/E2E bar; coverage report with honest gaps; local (Win + WSL) + CI matrix green.
- **Gate 2**: I prepare the v1.1.0 release fully; the **maintainer cuts and signs the tag** (do not self-sign).

## How to resume

1. `git checkout feat/install-posture-foundation` (already there); `git status` clean.
2. Read `.planning/STATE.md` Current Position + this file + memory `install-posture-arch.md`.
3. GSD phase resolver is BROKEN for this milestone (summary-style ROADMAP) — hand-manage phases; phase dirs live under `.planning/phases/NN-*/`; pass explicit plan paths to any gsd-executor; Go subagents have the toolchain here.
4. Start Phase 30 by confirming the web-repo location, then plan + execute (delegate implementation to a gsd-executor, verify + review yourself).

## No blockers.
