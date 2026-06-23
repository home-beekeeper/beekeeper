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

- ✅ **v1.5.0 — "Install Posture"** — Phases 26–31 (shipped 2026-06-22 as release `v1.1.0`)
  Full detail: [`milestones/v1.5.0-ROADMAP.md`](milestones/v1.5.0-ROADMAP.md). Requirements: [`milestones/v1.5.0-REQUIREMENTS.md`](milestones/v1.5.0-REQUIREMENTS.md). Audit: `tech_debt` (zero blockers) — [`milestones/v1.5.0-MILESTONE-AUDIT.md`](milestones/v1.5.0-MILESTONE-AUDIT.md). Maintainer-signed `v1.1.0` tag on merged `main`. *Internal GSD number v1.5.0 (decoupled from the parked v1.1.0 "Pollen" milestone); released publicly as v1.1.0.*

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

<details>
<summary>✅ v1.5.0 Install Posture (Phases 26–31) — SHIPPED 2026-06-22 (released as v1.1.0)</summary>

- [x] Phase 26: Nudge Removal & Posture Rule Foundation (2/2) — NMIG-01/02/04, IPST-04/05
- [x] Phase 27: Layer 1 Hook Enforcement + Sentry Observation (4/4) — IPST-01/02/03/06, IPBND-01, NMIG-04 (**Gate 1 PASSED**; IPOVR-03 added to v1.0; shim made real; SENTRY-009)
- [x] Phase 28: Layer 2 `beekeeper posture` View (1/1) — IPVIEW-01/02, IPBND-01
- [x] Phase 29: Layer 3 Scoped Override + Per-Rule Severity (2/2) — IPOVR-01/02/03
- [x] Phase 30: Docs, Home Page & Boundary Statements (1/1) — IPBND-02, NMIG-03, REL-01 (prep)
- [x] Phase 31: Test, Coverage & E2E + CI Matrix (1/1) — test coverage, REL-01 (finalize) (**Gate 2 after**)

Full detail: [`milestones/v1.5.0-ROADMAP.md`](milestones/v1.5.0-ROADMAP.md). Released as `v1.1.0` (maintainer-signed tag on merged `main`). Audit `tech_debt` (zero blockers). Deferred to roadmap: per-ecosystem policy matrix, the shim as a first-class enforcement surface, config mutation (PRD Layer 4).

</details>

## Carried Candidates for the Next Milestone

- Live external `beekeeper-self` hosting (separate host + signing key) + end-to-end refuse-to-run validation; independent external security review + VDP publication (from v1.0.0).
- Deferred nudge/corroboration follow-ups: NUDGE-F1 (hard-rewrite on-by-default), NUDGE-F2 (Yarn Berry + pip/cargo/gem/composer), NUDGE-F3 (`GHSA-*` vs `MAL-*`), CORR-F1 (OSV/Socket as automatic hot-path second source).
- **SITE-03** — live Vercel deploy of the v1.3.0 site (page already build-verified; pending repo push / Vercel setup).
- Docs: command-card-per-copy split + a full Fumadocs-theme redesign (own UI-SPEC).
- **From v1.5.0 (install posture):** deep per-rule per-ecosystem policy matrix + custom thresholds; the package-manager shim as a first-class machine-wide enforcement surface; config mutation (opt-in/reversible/audited recommended-config generation, PRD Layer 4); the deferred tech-debt items M-01/M-02 + lows (see `phases/31-test-coverage-e2e-ci/31-REVIEW-DECISION.md`).
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
| 27. Layer 1 Hook Enforcement + Sentry Observation (Gate 1) | v1.5.0 | 4/4 | Complete — **Gate 1 PASSED** (+ IPOVR-03 added to v1.0) | 2026-06-21 |
| 28. Layer 2 `beekeeper posture` View | v1.5.0 | 1/1 | Complete | 2026-06-21 |
| 29. Layer 3 Scoped Override + Per-Rule Severity | v1.5.0 | 2/2 | Complete | 2026-06-21 |
| 30. Docs, Home Page & Boundary Statements | v1.5.0 | 1/1 | Complete | 2026-06-22 |
| 31. Test, Coverage & E2E + CI Matrix (Gate 2 after) | v1.5.0 | 1/1 | Complete — **Gate 2 PASSED** (maintainer-signed v1.1.0 tag) | 2026-06-22 |
