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

- 🚧 **v1.4.0 — "Adjudicated Corpus (Local Loop)"** — Phases 22–25 (IN PROGRESS, started 2026-06-13)
  Scope: `beekeeper-corpus-milestone-prd.md` §3 (local loop only); PRD §6 (v1.1–1.9 org self-host push) + §7 (v2.0 community feed) are future milestones. Requirements: [`REQUIREMENTS.md`](REQUIREMENTS.md). Research: [`research/SUMMARY.md`](research/SUMMARY.md).

- 📋 **Next milestone (after v1.4.0)** — TBD (carried candidates below; v1.1.0 Pollen resume is independent)

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

### 🚧 v1.4.0 "Adjudicated Corpus (Local Loop)" — Phases 22–25 (IN PROGRESS)

> Scope = `beekeeper-corpus-milestone-prd.md` §3 only. The moat is the OUTCOME layer (confirmed `true_label` + how it was established): it cannot be retrofitted onto incidents already discarded, so the schema captures it from the first offline run. Enforcement stays local/offline/fail-closed in every version; the corpus loop and (future) push are off the hot path. PRD §6 (org push) + §7 (community feed) are future milestones. Three open decisions (OQ-1 `downstream_clean` window / OQ-2 scan `cluster_id` unit / OQ-3 adjudicator lifecycle) carry proposed defaults in `REQUIREMENTS.md`, finalized at the owning phase's discuss step.

- [x] **Phase 22: Schema & Envelope Lock** (SCHEMA-01..06, SCOPE-01..02) — ✅ COMPLETE 2026-06-13 (3/3 plans; verifier 8/8; maintainer freeze sign-off; schema FROZEN at CorpusSchemaVersion 1.0; 1 deferred item tracked: IPv6 normalize quirk)
  Goal: Freeze the four-layer event schema (behavior/decision/outcome/context) and the push-envelope wire format — including `scope`, corroboration fields, and the `signing` stub — before any corpus record is written. Sign-off freezes the format.
  Success criteria:
  1. Every field in the Nx Console worked trace maps to a schema field with no gaps (SCHEMA-06).
  2. The envelope can represent a `watch_and_block` push carrying `confidence_tier` + `source_count`; `auto_purge` is unrepresentable in a pushable envelope (compile-time guard) (SCHEMA-03/04).
  3. The four-layer record's outcome fields serialize as `unresolved` placeholders from the first write; conditional fields validate for all six `source_surface` branches (SCHEMA-01).
  4. `scope` zero-value resolves to `org_only`; the only scope-changing path errors in v1 (SCOPE-01/02).
  5. `behavior_signature_hash` input + `ruleset_version` are frozen and documented (SCHEMA-05).

- [x] **Phase 23: Corpus Store & Adjudication Engine** (ADJ-01..07, STORE-01..05, ENV-01..03) — ✅ COMPLETE 2026-06-14 (3/3 plans; verifier 5/5; full suite green + ENV-03 fuzz 316k/0 + BenchmarkRunCheck ~25ms; orchestrator-caught ADJ-01 benchmark defect fixed before verify)
  Goal: Stand up the append-only local corpus store (as an `audit.Sink`) and the off-hot-path adjudication engine assigning the outcome layer with corroboration-gated confidence, emitting records in envelope shape.
  Plans:
  - [x] 23-01-PLAN.md — Corpus store (StoreSink as audit.Sink, redaction-first, append-only, 0600) + HMAC-SHA256 fingerprints + per-install salt (Wave 1) — STORE-01/02/03/04/05, ENV-01
  - [x] 23-02-PLAN.md — Emitter adapter (MapToCorpusRecord) + BuildPushEnvelope purge gate + Ed25519 signer stub + ENV-03 FuzzBuildPushEnvelope release gate (Wave 1) — ENV-01/02/03, ADJ-04/05
  - [x] 23-03-PLAN.md — Adjudicator (pure Adjudicate + bounded RunAdjudicationBatch) + runCatalogsSync batch-pass wiring + hot-path corpus write + MultiSink integration + BenchmarkRunCheck (Wave 2) — ADJ-01/02/03/04/05/06/07, ENV-01
  Success criteria:
  1. A synthetic incident records all four layers; a two-source adjudication is marked `enforce` weight, a one-source adjudication `watch` weight, with `source_count` from DISTINCT sources (ADJ-04/05).
  2. Records emit in push-envelope shape and persist append-only + owner-only; an injected secret-shaped field is redacted in the persisted file (STORE-01/02/04, ENV-01).
  3. Adjudication runs off the hot path — `beekeeper check` is never blocked and `internal/policy` stays I/O-free (ADJ-01).
  4. `repo_fingerprint`/`fleet_node_id` are non-reversible HMAC values; two installs of the same repo differ (STORE-05).
  5. A fuzz/property test confirms no envelope escapes with a non-allowlisted `action_hint` (ENV-02/03).

- [ ] **Phase 24: First Responder Corpus Binding** (FRB-01..05)
  Goal: A confirmed-malicious adjudication arms the TUI quarantine card and elevates detection (Sentry watch + local catalog overlay) without ever auto-purging.
  Success criteria:
  1. A confirmed local Nx-Console-style match arms the quarantine card and does NOT auto-purge (FRB-01/02).
  2. Restore reverses a purge cleanly; the red=attacker / coral=Beekeeper TUI semantic is preserved (FRB-03).
  3. The Sentry watch elevates only at corroboration ≥ threshold (a single source does not tighten), detection-only (FRB-04).
  4. A local-only catalog overlay entry is added (owner-only) and survives `catalogs sync` (FRB-05).

- [ ] **Phase 25: Launch Readiness** (LAUNCH-01..04)
  Goal: Prove the end-to-end moat loop on the Nx Console incident + all eight Sentry patterns, confirm offline-protective + sub-100ms hot path, and ship honest docs naming the residual gaps.
  Success criteria:
  1. The Nx Console incident runs trace → record → adjudication → signature → local feedback (catalog overlay + Sentry watch), all four layers populated (LAUNCH-01).
  2. Each of the eight Sentry patterns (SENTRY-001..008) produces a moat-grade record with all four layers (LAUNCH-02).
  3. A disconnected machine remains fully protective on the last synced catalog; `beekeeper check` p99 stays sub-100ms with the corpus enabled (benchmark gate) (LAUNCH-03).
  4. No corpus data leaves the machine (verified); `docs/THREAT-MODEL.md` states local-first and NAMES the residual gaps (SENTRY-008 CI-runner OIDC theft, GitHub API dead-drop, DNS-tunnel ingested-but-undetected) (LAUNCH-04).

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
| 24. First Responder Corpus Binding | v1.4.0 | 0/? | Not started | — |
| 25. Launch Readiness | v1.4.0 | 0/? | Not started | — |
