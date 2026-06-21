# Roadmap: Beekeeper

> Multi-milestone master. Shipped milestones are collapsed with links to their full archives under `milestones/`. The parked v1.1.0 "Pollen" section is preserved in place (resume artifacts live in `docs/release-runbook.md` + `phases/05-*/.continue-here.md`).

## Milestones

- ✅ **v1.0.0 — Comprehensive Standalone Release** — Phases 1–11 (shipped 2026-06-01)
  Full detail: [`milestones/v1.0.0-ROADMAP.md`](milestones/v1.0.0-ROADMAP.md). Audit: PASSED — [`milestones/v1.0.0-MILESTONE-AUDIT.md`](milestones/v1.0.0-MILESTONE-AUDIT.md).

- 🔄 **v1.1.0 — "Pollen"** — Phases 1–5 (PARKED at maintainer release checkpoint)
  Goal: Own Windows inventory compatibility via a bounded Apache-2.0 Bumblebee derivative so the Windows CI matrix goes fully green. *(The standalone Pollen tool shipped publicly as v0.2.0 in the v1.3.0 cycle; the GSD milestone remains parked, not closed.)*

- ✅ **v1.2.0 — "Runtime Behavioral Hardening"** — Phases 6–9 (shipped 2026-06-04)
  Full detail: [`milestones/v1.2.0-ROADMAP.md`](milestones/v1.2.0-ROADMAP.md).

- ✅ **v1.3.0 — "Web Presence & Documentation"** — Phases 10–21 (shipped 2026-06-11)
  Full detail: [`milestones/v1.3.0-ROADMAP.md`](milestones/v1.3.0-ROADMAP.md). Summary: [`MILESTONES.md`](MILESTONES.md).

- ✅ **v1.4.0 — "Adjudicated Corpus (Local Loop)"** — Phases 22–25 (shipped 2026-06-15)
  Full detail: [`milestones/v1.4.0-ROADMAP.md`](milestones/v1.4.0-ROADMAP.md). Requirements: [`milestones/v1.4.0-REQUIREMENTS.md`](milestones/v1.4.0-REQUIREMENTS.md). Audit: RESOLVED — found the FRB-05 enforcement BLOCKER, fixed same-day at close ([`milestones/v1.4.0-MILESTONE-AUDIT.md`](milestones/v1.4.0-MILESTONE-AUDIT.md)).

- 🔨 **v1.5.0 — "Install Posture"** — Phases 26–31 (ACTIVE; ships publicly as release `v1.1.0`)
  Goal: retire the package-manager nudge and ship tool-agnostic install posture (default posture enforced at the hook, a read-only machine-wide posture view, scoped audited overrides), honest about its enforcement boundaries. Scope source: [`beekeeper-install-posture-prd.md`](../beekeeper-install-posture-prd.md). Requirements: [`REQUIREMENTS.md`](REQUIREMENTS.md). *Internal milestone number is v1.5.0 to avoid colliding with the parked v1.1.0 "Pollen" GSD milestone; the release tag is v1.1.0.*

## Phases

<details>
<summary>✅ v1.0.0 Comprehensive Standalone Release (Phases 1–11) — SHIPPED 2026-06-01</summary>

- [x] Phase 1: Foundation + Hook Handler (6/6 plans)
- [x] Phase 2: Policy Engine + Multi-Source Catalogs (9/9)
- [x] Phase 3: Editor Extension Defense (5/5)
- [x] Phase 4: Integration Surfaces (5/5)
- [x] Phase 5: Linux Sentry (5/5)
- [x] Phase 6: LlamaFirewall + Audit Sinks (5/5)
- [x] Phase 7: Cross-Platform Sentry (5/5)
- [x] Phase 8: TUI Dashboard (9/9)
- [x] Phase 9: Policy as Code + Self-Defense Capstone (5/5)
- [x] Phase 10: Cross-Phase Integration Closure (1/1)
- [x] Phase 11: v1.0.0 PRD-Gap Closure (pre-push) (1/1)

</details>

### v1.1.0 "Pollen" — Windows Inventory Compatibility (PARKED)

- [x] **Phase 1: Fork Setup & Discipline** — tag `v0.1.1-pollen.1` shipped
- [~] **Phase 2: Windows Root Resolver** — code complete; tag deferred to M2 close
- [~] **Phase 3: Windows Path Representation** — code complete & verified 2026-06-02; tag deferred
- [~] **Phase 4: Windows Extension & MCP Coverage** — code complete & verified 2026-06-02; tag deferred
- [ ] **Phase 5: Contribution-Back & Milestone Close** — not started (parked)

<details>
<summary>✅ v1.2.0 Runtime Behavioral Hardening (Phases 6–9) — SHIPPED 2026-06-04</summary>

- [x] Phase 6: Corroboration Severity Hardening (3/3 plans)
- [x] Phase 7: Sensitive-Path Runtime Enforcement (3/3)
- [x] Phase 8: Package-Manager Nudge + Behavioral Test Suite (8/8)
- [x] Phase 9: v1.2.0 Tech-Debt Cleanup (5/5 +1 fix)

Full detail: [`milestones/v1.2.0-ROADMAP.md`](milestones/v1.2.0-ROADMAP.md).

</details>

<details>
<summary>✅ v1.3.0 Web Presence & Documentation (Phases 10–21) — SHIPPED 2026-06-11</summary>

- [x] Phase 10: Hook-Block Protocol Compliance & Multi-Harness Enforcement (6/6) — seed, shipped 2026-06-05
- [x] Phase 11: Scaffold & Toolchain Isolation (1/1) — SITE-01, SITE-02
- [x] Phase 12: Design System (3/3) — DSYS-01–04
- [x] Phase 13: Docs Content Pipeline (3/3) — DOCS-01
- [x] Phase 14: Changelog Pipeline (2/2) — CHG-01–03
- [x] Phase 15: Marketing Home (3/3) — HOME-01–05 (SITE-03 deferred→Vercel)
- [x] Phase 16: 3D Layer (3/3) — GFX-01–04
- [x] Phase 17: SEO & Static Assets (3/3) — SEO-01
- [x] Phase 18: Full Content Authoring (6/6) — DOCS-02–09
- [x] Phase 18.1: Docs Theme Restyle (quick task, INSERTED) — DSYS-05
- [x] Phase 19: Test Suite & CI (3/3) — QA-01, QA-02
- [x] Phase 20: Runtime Hardening II (Tiers 1–3) (6/6) — CSYNC-01..06, LLMF-01..06, SENT-01..11
- [x] Phase 21: Full-System Validation & CI Calibration (4/4) — VAL-01..08

Full detail: [`milestones/v1.3.0-ROADMAP.md`](milestones/v1.3.0-ROADMAP.md). One intentional deferral: SITE-03 (live Vercel deploy).

</details>

<details>
<summary>✅ v1.4.0 Adjudicated Corpus (Local Loop) (Phases 22–25) — SHIPPED 2026-06-15</summary>

- [x] Phase 22: Schema & Envelope Lock (3/3) — SCHEMA-01..06, SCOPE-01..02 (schema FROZEN at CorpusSchemaVersion 1.0)
- [x] Phase 23: Corpus Store & Adjudication Engine (3/3) — ADJ-01..07, STORE-01..05, ENV-01..03
- [x] Phase 24: First Responder Corpus Binding (3/3) — FRB-01..05
- [x] Phase 25: Launch Readiness (3/3) — LAUNCH-01..04

Full detail: [`milestones/v1.4.0-ROADMAP.md`](milestones/v1.4.0-ROADMAP.md). The milestone audit found the FRB-05 enforcement BLOCKER (the local overlay was written but never read by the live `beekeeper check` path); fixed same-day via quick task `260615-ky4` (overlay now consulted; allow→warn escalation, unsigned per CTLG-07). Carried forward: overlay wiring for gateway/scan/watch; the warn-vs-block design question; STORE-02 fan-out seam unused. See [`milestones/v1.4.0-MILESTONE-AUDIT.md`](milestones/v1.4.0-MILESTONE-AUDIT.md).

</details>

### 🔨 v1.5.0 Install Posture (Phases 26–31) — ACTIVE — ships as release v1.1.0

Retire the nudge; ship tool-agnostic install posture with honest enforcement boundaries. PRD: `beekeeper-install-posture-prd.md`. Two human gates: **Gate 1** (enforcement-boundary review) after Phase 27; **Gate 2** (release signing) after Phase 31.

- [x] **Phase 26: Nudge Removal & Posture Rule Foundation** — NMIG-01, NMIG-02, NMIG-04, IPST-04, IPST-05 ✅ 2026-06-21
  Goal: remove the nudge steering (preserving release-age + the pm-config readers), add the new git/remote-URL pure detection, repoint the shim. Pure-library layer only; no behavior wired to the hook yet.
  Success: (1) the `beekeeper nudge` CLI, `config set nudge.*`, the steer-to-pnpm/Bun copy, and `ensureNudgeBlockDefault` are gone; (2) `pkgparse` + a pure policy evaluator detect git/remote/URL/file install specs with unit tests; (3) `internal/nudge/detect.go` + `scanners.go` are relocated (not deleted) under the posture package; (4) build + vet green, pure-import purity tests still pass; (5) the shim builds and routes through `beekeeper check`.
- [~] **Phase 27: Layer 1 Hook Enforcement + Sentry Observation** — IPST-01, IPST-02, IPST-03, IPST-06, IPBND-01. **← Gate 1 (implemented + verified 2026-06-21; awaiting maintainer boundary review)**
  Goal: wire the posture rules (release-age warn, lifecycle warn, git/remote warn) into the pre-exec hook via the existing engine, replacing the nudge block; have Sentry observe + audit installs as detection-not-prevention; write the canonical boundary statement in code.
  Success: (1) an agent npm install of a <24h version warns at the hook with the reason in the audit record; (2) a lifecycle-script install and a git/remote-URL install each warn with their reason; (3) tier caveats are inherited and documented in code; (4) a Sentry-observed install (incl. human-run) produces an audit record labeled detection; (5) the canonical boundary statement exists in code, ready for Gate 1 review.
- [ ] **Phase 28: Layer 2 `beekeeper posture` View (read-only)** — IPVIEW-01, IPVIEW-02, IPBND-01
  Goal: a machine-wide read-only `beekeeper posture` view (CLI + TUI) that reads each detected pm's config and shows it against Beekeeper's enforced posture, naming gaps.
  Success: (1) `beekeeper posture` shows npm + at least one other ecosystem (pnpm) config vs enforced posture and names the covered gaps; (2) a test asserts the view writes no pm config file; (3) the boundary statement appears in the view output.
- [ ] **Phase 29: Layer 3 Scoped Override** — IPOVR-01, IPOVR-02
  Goal: on a posture decision, offer allow-once / allow-always-with-recorded-reason / block; each writes a scoped audit entry; allow-always persists via the existing policy overlay.
  Success: (1) the three graduated responses are offered (CLI + TUI incident card); (2) each produces the correct distinct audit-log entry; (3) allow-always persists as a scoped overlay entry and is honored on the next matching install.
- [ ] **Phase 30: Docs, Home Page & Boundary Statements** — IPBND-02, NMIG-03, REL-01 (prep)
  Goal: bring the docs current to the shipped feature, propagate the boundary statement everywhere, replace the home "Agent safety" nudge bullet, remove nudge copy, and prepare the release (version → v1.1.0, changelog).
  Success: (1) install-posture docs + posture-view usage + boundary statement land, no roadmap item documented as shipped; (2) the home bullet is replaced with install-posture framing + the npm v12 obsolescence note; (3) all steer-to-pnpm/Bun copy is gone; (4) version bumped to v1.1.0 and changelog updated with a roadmap note for the deferred layers.
- [ ] **Phase 31: Test, Coverage & E2E + CI Matrix** — IPST/IPVIEW/IPOVR test coverage, REL-01 (finalize). **← Gate 2 after**
  Goal: complete the test/coverage/E2E bar and get local (Win+WSL) + CI matrix green; produce a coverage report with honest gaps named.
  Success: (1) every posture rule + decision branch + tier-caveat path has a behavior-asserting test; release-age/lifecycle/git boundary conditions covered; (2) each scoped-override path asserts its audit entry; the view read-only and observed-install audit are tested; (3) E2E proves agent-install-blocked-at-hook-with-reason+audit, human-install-observed-not-blocked, and the view across npm + one other; (4) coverage report with deliberate gaps documented; (5) local + CI matrix green.

## Carried Candidates for the Next Milestone

- Live external `beekeeper-self` hosting (separate host + signing key) + end-to-end refuse-to-run validation; independent external security review + VDP publication (from v1.0.0).
- Deferred nudge/corroboration follow-ups: NUDGE-F1 (hard-rewrite on-by-default), NUDGE-F2 (Yarn Berry + pip/cargo/gem/composer), NUDGE-F3 (`GHSA-*` vs `MAL-*`), CORR-F1 (OSV/Socket as automatic hot-path second source).
- **SITE-03** — live Vercel deploy of the v1.3.0 site (page already build-verified; pending repo push / Vercel setup).
- Docs: command-card-per-copy split + a full Fumadocs-theme redesign (own UI-SPEC).
- **Independent of any milestone:** v1.1.0 "Pollen" resumes via `docs/release-runbook.md` when the maintainer chooses.

## Progress

| Phase | Milestone | Plans | Status | Completed |
|-------|-----------|-------|--------|-----------|
| 1. Foundation + Hook Handler | v1.0.0 | 5/5 | Complete | 2026-05-26 |
| 2. Policy Engine + Multi-Source Catalogs | v1.0.0 | 4/4 | Complete | 2026-05-27 |
| 3. Editor Extension Defense | v1.0.0 | 5/5 | Complete | 2026-05-26 |
| 4. Integration Surfaces | v1.0.0 | 3/3 | Complete | 2026-05-27 |
| 5. Linux Sentry | v1.0.0 | 5/5 | Complete | 2026-05-28 |
| 6. LlamaFirewall + Audit Sinks | v1.0.0 | 5/5 | Complete | 2026-06-03 |
| 7. Cross-Platform Sentry | v1.0.0 | 5/5 | Complete | 2026-05-28 |
| 8. TUI Dashboard | v1.0.0 | 9/9 | Complete | 2026-05-29 |
| 9. Policy as Code + Self-Defense Capstone | v1.0.0 | 5/5 | Complete | 2026-05-29 |
| 10. Cross-Phase Integration Closure | v1.0.0 | 6/6 | Complete | 2026-06-05 |
| 11. v1.0.0 PRD-Gap Closure (pre-push) | v1.0.0 | 1/1 | Complete | 2026-06-01 |
| **1. Fork Setup & Discipline** | **v1.1.0** | **5/5** | **Complete** | **2026-05-26** |
| **2. Windows Root Resolver** | **v1.1.0** | **3/4** | **Code complete — release deferred to M2 close** | **—** |
| **3. Windows Path Representation** | **v1.1.0** | **3/3** | **Code complete & verified — release deferred** | **2026-06-02** |
| **4. Windows Extension & MCP Coverage** | **v1.1.0** | **3/3** | **Code complete & verified — release deferred** | **2026-06-02** |
| **5. Contribution-Back & Milestone Close** | **v1.1.0** | **0/5** | **PARKED** | **—** |
| 6. Corroboration Severity Hardening | v1.2.0 | 3/3 | Complete | 2026-06-03 |
| 7. Sensitive-Path Runtime Enforcement | v1.2.0 | 3/3 | Complete | 2026-06-04 |
| 8. Package-Manager Nudge + Behavioral Test Suite | v1.2.0 | 8/8 | Complete | 2026-06-04 |
| 9. v1.2.0 Tech-Debt Cleanup | v1.2.0 | 5/5 | Complete | 2026-06-04 |
| 10. Hook-Block Protocol Compliance | v1.3.0 | 6/6 | Complete (seed) | 2026-06-05 |
| 11. Scaffold & Toolchain Isolation | v1.3.0 | 1/1 | Complete | 2026-06-07 |
| 12. Design System | v1.3.0 | 3/3 | Complete | 2026-06-08 |
| 13. Docs Content Pipeline | v1.3.0 | 3/3 | Complete | 2026-06-08 |
| 14. Changelog Pipeline | v1.3.0 | 2/2 | Complete | 2026-06-08 |
| 15. Marketing Home | v1.3.0 | 3/3 | Complete (SITE-03 deferred→Vercel) | 2026-06-08 |
| 16. 3D Layer | v1.3.0 | 3/3 | Complete | 2026-06-09 |
| 17. SEO & Static Assets | v1.3.0 | 3/3 | Complete | 2026-06-09 |
| 18. Full Content Authoring | v1.3.0 | 6/6 | Complete | 2026-06-09 |
| 18.1 Docs Theme Restyle | v1.3.0 | quick-task | Complete | 2026-06-09 |
| 19. Test Suite & CI | v1.3.0 | 3/3 | Complete | 2026-06-10 |
| 20. Runtime Hardening II (Tiers 1–3) | v1.3.0 | 6/6 | Complete | 2026-06-11 |
| 21. Full-System Validation & CI Calibration | v1.3.0 | 4/4 | Complete | 2026-06-11 |
| 22. Schema & Envelope Lock | v1.4.0 | 3/3 | Complete | 2026-06-13 |
| 23. Corpus Store & Adjudication Engine | v1.4.0 | 3/3 | Complete | 2026-06-14 |
| 24. First Responder Corpus Binding | v1.4.0 | 3/3 | Complete | 2026-06-14 |
| 25. Launch Readiness | v1.4.0 | 3/3 | Complete | 2026-06-14 |
| 26. Nudge Removal & Posture Rule Foundation | v1.5.0 | 2/2 | Complete | 2026-06-21 |
| 27. Layer 1 Hook Enforcement + Sentry Observation (Gate 1) | v1.5.0 | 2/2 | Implemented + verified; ⏸ awaiting Gate 1 | 2026-06-21 |
| 28. Layer 2 `beekeeper posture` View | v1.5.0 | 0/? | Not started | — |
| 29. Layer 3 Scoped Override | v1.5.0 | 0/? | Not started | — |
| 30. Docs, Home Page & Boundary Statements | v1.5.0 | 0/? | Not started | — |
| 31. Test, Coverage & E2E + CI Matrix (Gate 2 after) | v1.5.0 | 0/? | Not started | — |
