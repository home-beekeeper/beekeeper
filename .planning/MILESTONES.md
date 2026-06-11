# Milestones

## v1.3.0 — Web Presence & Documentation (Shipped: 2026-06-11)

**Phases:** 13 (10–21, numbering continues from v1.2.0; Phase 18.1 inserted) · **Plans:** 43 (+1 quick task) · **Timeline:** 2026-06-05 → 2026-06-11
**Audit:** none — every phase was individually verified (per-phase verifier / UAT / code review). Archive: [`milestones/v1.3.0-ROADMAP.md`](milestones/v1.3.0-ROADMAP.md) / [`milestones/v1.3.0-REQUIREMENTS.md`](milestones/v1.3.0-REQUIREMENTS.md).

**Delivered:** Beekeeper's public-facing presence and pre-ship validation gate. A greenfield Next.js 16 static-export site under `web/` — a distinctive marketing home, a complete Fumadocs documentation set accurate to the shipped binary, a versioned changelog, an R3F hive hero, and a path-filtered web CI — plus the Go-core hardening + release gate: Runtime Hardening II (catalog sync daemon, a now-functional LlamaFirewall opt-in, expanded Sentry coverage + honesty edits) and Full-System Validation (coverage gate, 17-harness conformance, cross-platform CI matrix, fuzz gate, live e2e, signed-off validation register). The Phase 10 seed (multi-harness hook-block protocol fix, config-as-secret self-protection, `$VAR` hardening, TUI real-data) shipped 2026-06-05 and is folded in.

**Key accomplishments:**

1. **Marketing home + accurate docs** — hero with dual CTA + go-install chip, Nx Console origin story, how-it-works, six shipped-capability cards, 15-harness matrix, and a known-gaps honesty callout; a full Fumadocs docs set (getting-started → installation → configuration → CLI reference → security posture + co-located gaps → integration → troubleshooting → audit-log) authored research-first and gated accurate-to-binary (`source_doc:` provenance + accuracy spec + human THREAT-MODEL.md review).
2. **Static-export pipeline + 3D + SEO** — Next.js 16 / Tailwind v4 / shadcn/ui / Fumadocs / React-Three-Fiber, fully Go-isolated under `web/`; an R3F hive hero behind `dynamic(ssr:false)` with a static-SVG fallback (LCP < 2.5s, no WebGL leaks); sitemap/robots/canonical metadata + a static OG card; a versioned changelog with cosign/SLSA verify commands and the red exit-1→exit-2 breaking-change callout.
3. **Runtime Hardening II (Phase 20)** — an unprivileged background `catalogs daemon` (interval-gated, ETag-conditional); the LlamaFirewall opt-in made actually-functional (silent fail-open fixed, AlignmentCheck/cloud path removed, loopback-TCP+token IPC, venv + gated-22M-model bootstrap, Windows StateDir fix); Sentry coverage expanded (SENTRY-006/007/008 + `EventFileWrite` per-OS ingestion + cloud-cred watchlist) with honesty edits to PROJECT/THREAT-MODEL/home.
4. **Full-System Validation (Phase 21)** — `internal/coveragegate` (package-level linkage + a fail-closed reason-coded allowlist, self-defense VAL-08) closing the surfaced Tier-A gaps; a 17-harness golden-file deny + installer conformance suite (incl. Hermes fail-open + Kilo/Trae UNGUARDED); a cross-platform CI matrix (2 Linux kernels + macOS + Windows, 3 GOOS, eBPF/eslogger/ETW/peer-cred); a blocking Sentry fuzz gate; a Claude Code live exit-2 canary e2e; and a signed-off `docs/validation-register.md`.
5. **Test suite + CI isolation (Phase 19)** — Vitest unit (33) + Playwright e2e (12, all 4 critical paths) + postbuild SEO (29), a 6-gate path-filtered `web.yml`, and bidirectional `paths-ignore` so web and Go CI never cross-trigger; the 4 legacy Python specs retired after byte-for-byte JS parity.
6. **Pollen v0.2.0 released in-cycle** — the parked v1.1.0 fork was license-cleared (Apache-2.0 derivative of `perplexityai/bumblebee`), coverage-hardened, and shipped as a public, signed (cosign + SLSA + CycloneDX SBOM) standalone Windows tool; beekeeper's bumblebee→pollen scanner seam now resolves on all OSes and beekeeper gained its own Apache-2.0 LICENSE.

**Known deferred / stale-at-close:**
- **SITE-03** (live Vercel deploy) — the one intentional requirement deferral; static export retained, page build-verified locally. Carried forward.
- 4 stale bookkeeping items acknowledged at close (NOT real gaps — each phase was individually verified): Phase 13 UAT `passed`/0-pending, Phase 11 VERIFICATION `human_needed` (SITE-01/02 complete), the `docs-styling-polish` todo (already promoted → Phase 18.1 + shipped), Phase 10 CONTEXT open questions (resolved at ship). See STATE.md "Deferred Items".
- Command-card-per-copy docs split + a future full Fumadocs-theme redesign → backlog.
- **v1.1.0 "Pollen" remains PARKED** at its maintainer release checkpoint — independent of this close; the standalone tool shipped as v0.2.0 but the GSD milestone is not archived (resume via `docs/release-runbook.md` + the 05-05 `.continue-here.md`).
- `docs/nudge.md` (Go-side) is stale relative to the accurate web docs (it understates `config set nudge.mode block`).

---

## v1.2.0 — Runtime Behavioral Hardening (Shipped: 2026-06-04)

**Phases:** 4 (6–9, numbering continues from v1.1.0 "Pollen") · **Plans:** 19 (+1 pre-existing policy fuzz-build fix) · **Timeline:** 2026-06-03 → 2026-06-04
**Audit:** `tech_debt` (no blockers) — all surfaced findings cleared by the inserted Phase 9 (verified passed 9/9). Archive: [`milestones/v1.2.0-ROADMAP.md`](milestones/v1.2.0-ROADMAP.md).

**Delivered:** Closed the three runtime-enforcement gaps that live `beekeeper check` validation exposed — with the agent itself as the test subject — each locked in by a behavioral test suite proving the wiring is live: a *critical*-severity malware package that warned instead of blocking (F1), credential files that returned ALLOW to agent reads (F2), and `pnpm`/`bun` installs that bypassed catalog matching entirely (F3).

**Key accomplishments:**

1. **Corroboration severity hardening (F1)** — per-severity `SeverityOverrides["critical"]={BlockAt:1}` so a known critical package (`ai-figure` / Shai-Hulud, OSV `MAL-2026-4126`) blocks at one trusted source, gated on `catalog/sanity.go` degraded state; a mis-tagged `versions:["*"]` critical entry still requires 2-source corroboration (anti-poisoning bound).
2. **Sensitive-path runtime enforcement (F2)** — `policy.EvaluatePath`/`DefaultSensitivePaths` wired live into `runCheck`: credential reads (`~/.aws/credentials`, `~/.ssh/id_rsa`, `.env`, MCP configs) and `cat`/`type`/`Get-Content` shell targets block fail-closed; traversal/tilde/Windows-env-var bypasses canonicalized away; `.env.example/.test/.schema` allowlisted; overlay can escalate but never downgrade a path block (CR-02).
3. **Package-manager nudge + F3 closure** — a single pure `internal/pkgparse` so pnpm/bun/yarn installs are catalog-matched; `internal/nudge` soft-advise-default / hard-rewrite + `requireHardened`-block, wired into check (fresh detect) + gateway (60s cache) + shim, with the `beekeeper nudge status|check|audit` CLI.
4. **Behavioral test suite + hermetic live-binary E2E gate** — table-driven §10 tests, RunCheck integration, fuzz targets (`bunfig.toml`/`pnpm-workspace.yaml` hand scanners), and a `-tags e2e` live-binary battery over SPATH+CORR+NUDGE as the release gate.
5. **Tech-debt cleanup (Phase 9, from the milestone audit)** — made the CORR E2E gate network-independent (signed non-wildcard fixture), fixed `config.LoadLayered`'s Nudge-pointer merge at its root, hardened SPATH against ancestor-symlink / Windows-ADS / verb-substring evasion, shipped a live `version_drift` npm registry query (fail-open, floors never bumped), reconciled Phase-6 Nyquist, and repaired a pre-existing `internal/policy` `-tags fuzz` build break (`ef4ea97`).
6. **Self-defense = the test suite** — every fix ships a regression test that fails on the pre-fix code; `internal/policy`/`internal/nudge`/`internal/pkgparse` stay pure (import-purity tests); `-tags fuzz` + `-tags e2e` release gates green.

**Known deferred at close (carried to v1.3.0+):**
- NUDGE-F1 hard-rewrite on-by-default (gated on soft-advise production validation); NUDGE-F2 Yarn Berry + pip/cargo/gem/composer coverage; CORR-F1 OSV/Socket as an automatic hot-path second source; NUDGE-F3 `GHSA-*` vs `MAL-*` distinction in critical escalation.
- pnpm/bun/node floor **auto-update** on drift stays Out-of-Scope (drift is informational only).
- **v1.1.0 "Pollen" remains PARKED** at its maintainer release checkpoint (signed tags `pollen.2/.3/.4/.5`) — not part of this close; resume via `docs/release-runbook.md` + `HANDOFF.json`.

---

## v1.0.0 — Comprehensive Standalone Release (Shipped: 2026-06-01)

**Phases:** 11 (Phases 1–9 planned + Phase 10 integration closure + Phase 11 PRD-gap closure) · **Plans:** 51 · **Tasks:** 59
**Timeline:** 2026-05-26 → 2026-06-01 (7 days)
**Audit:** PASSED (re-verified after Phase 10 closed 4 cross-phase integration blockers)
**Pre-push PRD audit:** a direct `beekeeper-prd.md`-vs-code audit then found 6 more gaps the milestone audit missed (gateway PromptGuard scanned with an empty tool name → no-op; layered config not used by enforcement commands; Linux eBPF bytecode uncommitted/ungenerated; catalog-delta scan not triggered; `catalogs diff` missing; presence-only catalog signatures). **All 6 closed by Phase 11** (commits 3b79c90, 1f3682b, 86686d5, c42c681, 0b7f64f, deb8783) — re-verified green before the tag was moved to the fixed commit. Lesson: prior verification confirmed wiring *existed*, not that it was *effective* end-to-end (empty-arg no-op; helper present but never called by enforcement commands).

**Delivered:** A single static Go binary (`beekeeper`) that intercepts autonomous-agent tool calls before they execute and evaluates them against unified, corroboration-based threat intelligence — fail-closed by default, with a published self-defense threat model and a recursive self-quarantine feed.

**Key accomplishments:**

1. **Fail-closed hook handler + corroboration policy engine** — `beekeeper check` evaluates tool calls against an mmap catalog index under hard caps (1MB stdin / 5s / 256MB), with a pure `internal/policy` corroboration engine (1 source → warn, 2 → block, 3 → quarantine) across Bumblebee + OSV + Socket.
2. **Editor-extension defense** — agent `--install-extension` intercept, fsnotify watcher, and the watch → scan → quarantine workflow closing the Nx Console-class attack surface.
3. **Integration surfaces** — Claude Code / Cursor / Codex hook installers, a stateless fail-closed MCP gateway with per-session token auth and a fuzz-gated parser, and the PATH shim layer.
4. **Cross-platform Sentry** — Linux (eBPF + fanotify), macOS (eslogger), and Windows (ETW, no CGO) privileged daemons with a shared 5-rule correlation engine, talking to the unprivileged CLI over authenticated IPC.
5. **LlamaFirewall sidecar + full audit** — supervised Python sidecar (PromptGuard 2 / CodeShield), NDJSON audit log with syslog/OTLP/HTTPS sinks, and `audit query/tail/export`.
6. **Bubble Tea v2 TUI dashboard** — live activity, alerts, catalog, scan, policy, quarantine, and health panels, with admin mode and the Windows resize workaround.
7. **Policy as code + self-defense capstone** — declarative JSON policies (`policy validate/test/list`) enforced live across check/gateway/watch/scan, five-layer config merge, `beekeeper diag`, and the separately-signed `beekeeper-self` self-quarantine catalog.
8. **Self-defense from day one** — reproducible builds, Sigstore signing, SLSA Level 3 provenance + CycloneDX SBOM, and a public `docs/THREAT-MODEL.md` documenting the corroboration-poisoning surface and the fanotify mmap gap.

**Known deferred at close (carried to v1.x):**
- Live external `beekeeper-self` hosting (separate host + signing key) + end-to-end refuse-to-run validation — client side shipped; external ops gate.
- Independent external security review + VDP scope publication (PRD §15.5).
- Phases 02 and 05 verified via UAT (status approved/passed, 0 pending scenarios) rather than VERIFICATION.md — benign artifact-trail inconsistency.
- Distributed mode / team-shared catalogs; weighted corroboration (explicitly deferred per PRD §17).

---
