# Beekeeper

## What This Is

Beekeeper is a real-time safety harness for autonomous coding agents (Claude Code, Cursor, Codex CLI, Continue, OpenCode, OpenClaw). It mediates every tool call, package install, file access, and network egress against unified threat intelligence — catching compromised packages, hijacked agents, and malicious editor extensions before they act, not after. It wraps existing agents in an evaluator-generator pattern: the agent is the generator, Beekeeper is the evaluator.

Built by Mfanafuthi Mhlanga / Mzansi Agentive Pty Ltd. Solo project, public from day one, Apache 2.0.

## Core Value

A hijacked or off-task agent cannot successfully act on the developer's machine without Beekeeper deciding to permit it.

## Shipped: v1.5.0 — Install Posture (released as v1.1.0) — 2026-06-22

**Status:** ✅ SHIPPED 2026-06-22. Released publicly as `v1.1.0` (maintainer-signed SSH tag on merged `main` `4e92130`; GitHub PR #19 Go core + beekeeper-web #1 docs). Internal GSD milestone number is **v1.5.0** (continues the v1.4.0 line; deliberately not "v1.1.0", which is the parked Pollen GSD milestone). All 18 requirements satisfied; milestone audit `tech_debt` (zero blockers); both human gates cleared. Full detail: [`milestones/v1.5.0-ROADMAP.md`](milestones/v1.5.0-ROADMAP.md). **Next milestone:** TBD via `/gsd-new-milestone`.

**Goal:** Retire the package-manager nudge and ship tool-agnostic install posture: a shipped default posture enforced at the pre-exec hook, a read-only machine-wide posture view, and scoped audited overrides, honest about where enforcement actually reaches.

**Scope source:** `beekeeper-install-posture-prd.md` (repo root), v1.0 / first-release acceptance criteria 1-8 only. PRD Layer 4 (config mutation) and the deep per-rule editor are roadmap. Detailed REQ-IDs in `.planning/REQUIREMENTS.md` (phases 26-31).

**Target features:**
- Layer 1: default install posture (release-age <24h warn, lifecycle-script warn, git/remote-URL flag) enforced at the pre-exec hook via the existing warn/block/quarantine engine; observed + audited (detection, not prevention) at Sentry including human-run installs.
- Layer 2: read-only `beekeeper posture` view (CLI + TUI), machine-wide, reads each package manager's config and shows it against Beekeeper's enforced posture, naming gaps. Never writes config.
- Layer 3: scoped override (allow once / allow always with recorded reason / block), each written to the audit log; operates on a warn as well as a block. Plus per-rule severity opt-up (a user can raise an individual posture rule from warn to block via layered config, tighten-only from untrusted layers; the unknown path stays fail-soft warn) — added to v1.0 at Gate 1. The finer per-ecosystem/per-project policy matrix stays roadmap.
- Nudge removal + migration: remove the nudge and its steer-to-pnpm/Bun copy; preserve and reposition the release-age logic as the release-age posture rule; replace the home "Agent safety" nudge bullet with the install-posture framing + the npm v12 obsolescence note.

**Enforcement-boundary honesty (mandatory, the contract):** prevention happens at the hook for hooked Tier-1 harnesses only, inheriting every tier caveat; Sentry observes and audits (including human installs) but does not prevent; the MCP gateway is not a general install surface; the package-manager shim (which already exists but is repointed to posture and documented as roadmap/experimental, not deleted) is the labeled roadmap path to machine-wide human-install pre-exec enforcement. This statement appears wherever install posture is described, to the harness-coverage honesty standard.

**Two human gates:** Gate 1 (enforcement-boundary review) after Phase 27; Gate 2 (release signing) after Phase 31.

## Current State

**v1.4.0 — Adjudicated Corpus (Local Loop) — SHIPPED 2026-06-15.** Phases 22–25 (12 plans + 1 quick task) built the moat: incidents with a confirmed ground-truth outcome attached, captured from the first run on a single offline machine. A frozen four-layer event schema (behavior/decision/outcome/context) + push-envelope wire format with `auto_purge` made compile-time-unrepresentable (Phase 22); an append-only owner-only corpus store + off-hot-path adjudication engine with corroboration-gated confidence and HMAC fingerprints (Phase 23); First Responder corpus binding — confirmed-malicious arms the TUI quarantine card + a detection-only Sentry watch + a local catalog overlay, never auto-purging (Phase 24); and a launch-readiness gate proving the end-to-end moat loop + offline-protective + sub-100ms hot path + an honest threat model (Phase 25). All 32 requirements verified; **NO network transport stood up** (the wire format is emitted-in-shape so push is later wiring, not migration). The milestone audit found the FRB-05 enforcement gap (the local overlay was written but never read by the live `beekeeper check` path) and it was **fixed same-day** (quick task `260615-ky4`; overlay now consulted, allow→warn escalation since overlay entries are unsigned). Archived in [`milestones/v1.4.0-ROADMAP.md`](milestones/v1.4.0-ROADMAP.md) / [`milestones/v1.4.0-REQUIREMENTS.md`](milestones/v1.4.0-REQUIREMENTS.md) / [`milestones/v1.4.0-MILESTONE-AUDIT.md`](milestones/v1.4.0-MILESTONE-AUDIT.md). **Carried forward:** overlay wiring for the gateway/scan/watch surfaces; the warn-vs-block design question for confirmed-malicious overlay entries; PRD §6 (org push) + §7 (community feed) as future milestones.

**v1.3.0 — Web Presence & Documentation — SHIPPED 2026-06-11.** Phases 10–21 (43 plans + 1 quick task) delivered Beekeeper's public-facing presence and its pre-ship validation gate: a greenfield Next.js 16 static-export site under `web/` (distinctive marketing home + a complete Fumadocs docs set accurate to the shipped binary + versioned changelog + R3F hive hero + path-filtered web CI), plus Go-core hardening — Runtime Hardening II (catalog sync daemon, a now-functional LlamaFirewall opt-in, expanded Sentry coverage SENTRY-006/007/008 + file-write + honesty edits) and Full-System Validation (coverage gate + 17-harness conformance + cross-platform CI matrix + fuzz gate + live e2e + a signed-off validation register). 31/32 requirements validated; **SITE-03 (live Vercel deploy) is the one intentional deferral** (static export retained, page build-verified). Archived in [`milestones/v1.3.0-ROADMAP.md`](milestones/v1.3.0-ROADMAP.md) / [`milestones/v1.3.0-REQUIREMENTS.md`](milestones/v1.3.0-REQUIREMENTS.md). In-cycle, the parked v1.1.0 fork was license-cleared and shipped publicly as the standalone signed **Pollen v0.2.0** (the GSD v1.1.0 milestone itself remains PARKED).

**v1.2.0 — Runtime Behavioral Hardening — SHIPPED 2026-06-04.** Phases 6–9 (19 plans + 1 fix) closed the three runtime-enforcement gaps live `beekeeper check` validation exposed — critical-malware warn-only (F1), credential reads returning ALLOW (F2), and `pnpm`/`bun` catalog bypass (F3) — each locked by a behavioral test suite with a hermetic live-binary E2E + fuzz release gate. All 25 requirements (SPATH/NUDGE/CORR/BTEST + the audit-inserted CLEAN/HARDEN/DRIFT) complete & verified; archived in [`milestones/v1.2.0-ROADMAP.md`](milestones/v1.2.0-ROADMAP.md) / [`milestones/v1.2.0-REQUIREMENTS.md`](milestones/v1.2.0-REQUIREMENTS.md). Audit `tech_debt` (no blockers) → cleared by the inserted Phase 9.

**v1.0.0 — Comprehensive Standalone Release — SHIPPED 2026-06-01.** All 10 phases complete (50 plans, 53 tasks, 270 commits over 7 days). The full v1.0.0 requirement set (HOOK/CTLG/SFDF/PLCY/EDXT/INTG/SLNX/LLMF/AUDT/SMAC/SWIN/TUI/CODE) is implemented, verified, and archived in [`milestones/v1.0.0-REQUIREMENTS.md`](milestones/v1.0.0-REQUIREMENTS.md); the milestone audit (PASSED) is in [`milestones/v1.0.0-MILESTONE-AUDIT.md`](milestones/v1.0.0-MILESTONE-AUDIT.md).

Shipped: single static Go binary; pure corroboration policy engine; editor-extension defense; Claude Code/Cursor/Codex hooks + MCP gateway + shims; cross-platform Sentry (Linux eBPF/fanotify, macOS eslogger, Windows ETW); supervised LlamaFirewall sidecar + full audit sinks; Bubble Tea v2 TUI; declarative policy-as-code enforced live across check/gateway/watch/scan; layered config; `beekeeper diag`; the `beekeeper-self` self-quarantine client; reproducible builds + Sigstore + SLSA L3 + SBOM; public `docs/THREAT-MODEL.md`.

**Carried to v1.x:** live external `beekeeper-self` hosting (separate host + signing key) + end-to-end refuse-to-run; independent external security review + VDP publication; distributed mode / team-shared catalogs; weighted corroboration. Tech stack: Go 1.25, no CGO in core; Python 3.11+ optional sidecar; Bubble Tea v2.

## Shipped: v1.4.0 — Adjudicated Corpus (Local Loop) (2026-06-15)

**Status:** ✅ SHIPPED 2026-06-15 — all 32 requirements verified; milestone audit RESOLVED (the FRB-05 enforcement gap was found and fixed at close). Full detail: [`milestones/v1.4.0-ROADMAP.md`](milestones/v1.4.0-ROADMAP.md). The target features below were all delivered.

**Goal:** Capture the moat — incidents with confirmed ground-truth outcomes attached — from the first run on a single, offline machine; wire confirmed adjudications into First Responder; and freeze the push-envelope wire format without standing up any transport.

**Scope source:** `beekeeper-corpus-milestone-prd.md` (repo root), §3 "v1 scope (build now)" only. The PRD's own v1.1–1.9 (org self-host aggregation + push) and v2.0 (community shared corpus feed) are deferred to future milestones. Detailed REQ-IDs in `.planning/REQUIREMENTS.md`.

**Why it's the moat:** the behavior and decision layers of an event are cheap to reproduce and cheap for a competitor to copy. The outcome layer — the confirmed `true_label` and how it was established — cannot be retrofitted onto incidents already discarded, so the schema must capture it from the first run, on one machine, with zero network. Distribution (push) propagates the moat but is not the moat; it is built and hardened after launch.

**Target features:**
- Moat-grade four-layer event schema (behavior / decision / outcome / context), captured from the first offline run; conditional fields per `source_surface`; `cluster_id` binds correlated non-agent incidents (§3.1)
- Adjudication engine assigning `true_label` {malicious | benign | policy_correct | unresolved} + `adjudication_source` (6 values), corroboration-gated warn(1)/enforce(2+) with `source_count` recorded, async and off the hot path (§3.2)
- Append-only, owner-only local corpus store extending the NDJSON audit log (same sink model); never transmitted in v1; records emitted in push-envelope shape (§3.3)
- First Responder corpus binding — a confirmed-malicious adjudication arms a TUI quarantine card for any matching local install; purge stays human/org-policy gated (never automatic, never fleet-pushable); restore intact; red = attacker, coral = Beekeeper response (§3.4)
- Push-envelope wire format defined and emitted-in-shape, with NO transport stood up; `watch_and_block` the only pushable `action_hint`; `auto_purge` never present in a pushable envelope (§3.5)
- Scope tagging on every record from birth — `org_only` (default) vs `community_shareable`; promotion explicit + anonymization-gated (v2.0), never automatic (§3.6)

**Enforcement-stays-local invariant (held across all versions):** the policy engine, Sentry detection, First Responder, and the install-time catalog block all run locally, offline, fail-closed, against the last synced catalog. The corpus loop and (later) push are out of the hot path. The pure libraries (`internal/policy` / `nudge` / `pkgparse`) stay I/O-free.

**Self-defense (this milestone):** the corpus store is owner-only, append-only, and never transmitted in v1; the envelope's pushable-action allowlist (`watch_and_block` only, `auto_purge` never pushable) is the blast-radius guard baked in from the first record, so a future push channel cannot carry a destructive action.

## Shipped: v1.2.0 — Runtime Behavioral Hardening (2026-06-04)

Closed the three runtime-enforcement gaps live `beekeeper check` validation exposed (F1 critical-malware warn-only, F2 credential reads ALLOW, F3 pnpm/bun catalog bypass), each locked by a behavioral test suite. Delivered: per-severity sanity-gated corroboration escalation (CORR-01/02); `EvaluatePath` wired live into `check` with canonicalization + evasion hardening (SPATH-01..04 + HARDEN-01..03); a single pure `internal/pkgparse` + `internal/nudge` soft-advise/hard-rewrite nudge with the `beekeeper nudge` CLI (NUDGE-01..08); a hermetic live-binary E2E + fuzz release gate (BTEST-01..03). An audit-inserted Phase 9 then cleared the residual tech debt (hermetic CORR E2E gate, `LoadLayered` Nudge-merge root fix, SPATH ancestor-symlink/ADS/verb-boundary hardening, live `version_drift` registry query, Phase-6 Nyquist reconcile, +1 carried-over policy fuzz-build fix). Full detail: [`milestones/v1.2.0-ROADMAP.md`](milestones/v1.2.0-ROADMAP.md).

**Architecture constraint (held):** `internal/policy`, `internal/nudge`, and `internal/pkgparse` are pure function libraries — all detection/normalization I/O lives in adapters, mirroring `policy.EvaluateReleaseAge(ReleaseAgeInput, …)`.

## Shipped: v1.3.0 — Web Presence & Documentation (2026-06-11)

Shipped Beekeeper's public-facing presence and pre-ship validation gate across Phases 10–21. **Marketing home** — R3F hive hero + ambient accents, Nx Console origin story, 3-step how-it-works, six shipped-capability cards, the 15-harness matrix, and a known-gaps honesty callout. **Documentation (Fumadocs)** — getting started, installation (`go install` + cosign/SLSA verify), configuration, CLI reference, security posture + co-located known gaps, integration guides with point-of-use caveats, troubleshooting, and audit-log — authored research-first and gated accurate-to-binary (`source_doc:` provenance + accuracy spec + human THREAT-MODEL.md review). **Changelog** — versioned notes (v1.0.0/v1.2.0/v1.3.0) with verify commands + the red exit-1→exit-2 breaking-change callout. **Stack:** Next.js 16 App Router static export, Tailwind v4, shadcn/ui, Fumadocs, React-Three-Fiber, in-repo under `web/`, pnpm-workspace-isolated from the Go module.

Plus Go-core hardening: **Runtime Hardening II (Phase 20)** — an unprivileged `catalogs daemon`, the LlamaFirewall opt-in made actually-functional (silent fail-open fixed, cloud path removed, loopback-TCP+token IPC, gated-22M-model bootstrap, Windows StateDir fix), and expanded Sentry coverage (006/007/008 + `EventFileWrite` + cloud-cred watchlist) with honesty edits. **Full-System Validation (Phase 21)** — `internal/coveragegate` + fail-closed allowlist, 17-harness golden deny + installer conformance, a cross-platform CI matrix, a blocking Sentry fuzz gate, a Claude Code live exit-2 canary e2e, and a signed-off `docs/validation-register.md`. Full detail: [`milestones/v1.3.0-ROADMAP.md`](milestones/v1.3.0-ROADMAP.md).

**Architecture constraint (held):** web phases (11–19) did not modify Go behavior; Phase 21 explicitly relaxed that fence as the release gate (test coverage + CI matrix + coverage-surfaced fixes only — zero product behavior changed). The pure libraries (`policy`/`nudge`/`pkgparse`) stayed I/O-free.

**One intentional deferral:** SITE-03 (live Vercel deploy) — static export retained, page build-verified locally; carried forward.

> **Next milestone:** TBD via `/gsd-new-milestone`. v1.4.0 — Adjudicated Corpus (Local Loop) — SHIPPED 2026-06-15 (see "Shipped: v1.4.0" above; scope = `beekeeper-corpus-milestone-prd.md` §3). Carried-forward candidates (not yet scoped to a milestone): SITE-03 live Vercel deploy; live external `beekeeper-self` hosting + refuse-to-run E2E; independent external security review + VDP publication; deferred nudge/corroboration follow-ups (NUDGE-F1/F2/F3, CORR-F1); docs command-card-copy split + a full Fumadocs-theme redesign; the deferred first-responder follow-ons C2 (destructive package-manager uninstall) and C3 (browser-extension + mcp-config quarantine) from quick task 260612-f80. **Independent of any milestone:** v1.1.0 "Pollen" remains PARKED (see below) — its release train resumes via `docs/release-runbook.md` when the maintainer chooses (the standalone Pollen tool already shipped as v0.2.0 in this cycle).

## Parked Milestone: v1.1.0 Pollen (paused at maintainer release checkpoint — NOT closed)

> **Status: PARKED, not complete.** v1.1.0 is paused at its 05-05 maintainer release checkpoint (auth-gated GitHub push + signed tags `pollen.2/.3/.4/.5` + cosign verify). Resume artifacts are preserved: `HANDOFF.json`, `.planning/phases/05-contribution-back-milestone-close/.continue-here.md`, and `docs/release-runbook.md`. The four signed-tag releases remain in STATE.md "Deferred Items". Do **not** archive or close v1.1.0 until the maintainer runs the runbook and completes 05-05 Task 3. v1.2.0 phases continue the numbering at 06+ so the parked `05-*` phase dir is untouched.

**Goal:** Own Windows inventory compatibility by forking Bumblebee into a bounded Apache-2.0 derivative ("Pollen"), so the Windows CI matrix goes fully green and cross-platform test discipline holds instead of rotting behind `t.Skip`.

**Target features:**
- Pollen fork hygiene — separate module `github.com/home-beekeeper/pollen`, Apache-2.0 LICENSE/NOTICE/CHANGES/UPSTREAM, renamed `cmd/pollen`, reproducible builds + Sigstore
- Windows root resolver for 8 ecosystems (npm/pnpm/Yarn/Bun/PyPI/Go modules/RubyGems/Composer)
- Windows NDJSON path representation (native backslash paths, drive letters, `endpoint.os="windows"`, empty UID)
- Windows editor/browser/MCP config path coverage (VS Code family, Chromium + Firefox, Claude Desktop/Cursor/Windsurf/Cline/Gemini)
- Parity + differential tests; the Pollen compatibility test in beekeeper CI drives the Windows skip baseline to zero
- Upstream sync discipline + contribution-back PRs to `perplexityai/bumblebee`

**Scope source:** `beekeeper-m2-prd.md` (repo root). Detailed REQ-IDs in `.planning/REQUIREMENTS.md`. Spans two repos: the new `home-beekeeper/pollen` module **+** beekeeper `internal/inventory/` integration across a subprocess boundary (`internal/scan` swaps the `bumblebee` binary call for `pollen`). Pollen versions separately as `v0.1.1-pollen.1…5`, one per sub-phase. `threat_intel/` catalogs keep flowing through beekeeper's own `catalogs sync` (not duplicated in Pollen).

**Progress (4/5 phases):** Phase 1 Fork Setup & Discipline ✓ (tag `v0.1.1-pollen.1` shipped) · Phase 2 Windows Root Resolver ✓ (code complete & verified; signed tag deferred to M2 close) · Phase 3 Windows Path Representation ✓ (WPATH-01/02 — `filepath.FromSlash` path preservation + empty-Windows-uid guard; verified 2026-06-02; `v0.1.1-pollen.3` tag deferred) · **Phase 4 Windows Extension & MCP Coverage ✓** (WEXT-01/02/03 — Windows editor/browser/MCP root enumeration in Pollen `cmd/pollen/roots.go` + `internal/ecosystem/editorext`; BKINT-01 — beekeeper `internal/scan` subprocess seam renamed `bumblebee`→`pollen` **in place**; PTEST-04 — fixture-driven `TestPollenCompatibility` passes with **zero Windows `t.Skip`**; verified 2026-06-02; `v0.1.1-pollen.4` tag deferred to M2 close). Phase 5 (contribution-back & milestone close) not started. The PRD's idealized `internal/resolver/` / `internal/output/paths_windows.go` layout does **not** match the live fork — Windows fixes live in `cmd/pollen/roots.go` + `internal/ecosystem/` (+ `internal/endpoint` for Phase 3). **BKINT-01 decision (revised):** the beekeeper↔Pollen boundary is an in-place `internal/scan` seam rename (`runPollenFn`/`lookPollenFn`/`defaultRunPollen`), **not** a separate `internal/inventory/` package — research-confirmed as the smaller, mockable diff; the `internal/inventory/` reservation noted in earlier phases was dropped.

## Requirements

### Validated

#### v1.4.0 — Adjudicated Corpus (Local Loop) (2026-06-15)
- ✓ Four-layer event schema (behavior/decision/outcome/context) + frozen push-envelope wire format; `auto_purge` unrepresentable in a pushable envelope (compile-time guard); `scope` from birth (`org_only` default) — Phase 22 (SCHEMA-01..06, SCOPE-01..02)
- ✓ Append-only owner-only local corpus store (as an `audit.Sink`, redaction-first) + off-hot-path adjudication engine (corroboration-gated `true_label`/`confidence_tier`, HMAC fingerprints, per-install salt, ENV-03 fuzz gate) — Phase 23 (ADJ-01..07, STORE-01..05, ENV-01..03)
- ✓ First Responder corpus binding — confirmed-malicious arms the TUI quarantine card + a detection-only Sentry watch + a local catalog overlay (consulted by the live `beekeeper check` path; allow→warn since unsigned), never auto-purging — Phase 24 (FRB-01..05; check-path enforcement wired post-audit via quick task 260615-ky4)
- ✓ End-to-end moat loop proven (Nx Console + 8 Sentry patterns), offline-protective + sub-100ms hot path, no-network-egress AST gate, honest `docs/THREAT-MODEL.md` §13 — Phase 25 (LAUNCH-01..04)

- ✓ `beekeeper check` reads tool call JSON from stdin, evals policy, exits allow/block (single-source Bumblebee warn semantics; corroboration block deferred to Phase 2) — Phase 1
- ✓ Catalog matching against Bumblebee `threat_intel/` with mmap binary index (O(log n) lookup, zero JSON parse on hot path) — Phase 1
- ✓ NDJSON audit log with owner-only permissions (0600) and `beekeeper audit tail` — Phase 1
- ✓ `beekeeper catalogs sync` fetches and caches Bumblebee catalog (654 entries, mmap index) — Phase 1
- ✓ Reproducible builds (`-trimpath -buildvcs=false`), Sigstore/cosign v3 keyless signing, Renovate dependency pinning, `SECURITY.md` — Phase 1
- ✓ NDJSON audit log with rotation, query/export commands (`beekeeper audit query`, `beekeeper audit export --format ndjson/csv/otlp`), `--no-follow` tail — Phase 6 (AUDT-02, AUDT-05, AUDT-06, AUDT-07)
- ✓ Audit fan-out sinks: local file (default, 0600), syslog RFC 5424, OTLP, HTTPS POST — opt-in with data-egress warning — Phase 6 (AUDT-03, AUDT-04)
- ✓ LlamaFirewall IPC protocol: length-prefixed JSON, 1MB cap, 3 scan kinds (ScanPrompt/ScanCode/ScanAlignment), fuzz CI release gate — Phase 6 (LLMF-05)
- ✓ LlamaFirewall supervisor with exponential-backoff restart, Unix/named-pipe IPC client, ring-buffer P95 latency, Python sidecar — Phase 6 (LLMF-01, LLMF-06)
- ✓ PromptGuard 2 integration in hook handler (injection → llmf_alert; fail-closed on sidecar unavailable); CodeShield + AlignmentCheck wired in gateway — Phase 6 (LLMF-02, LLMF-03, LLMF-04)
- ✓ LLMF provenance fields in AuditRecord (LLMFScanned, LLMFScanKind, LLMFResult, LLMFLatencyMS, LLMFAlertType) — Phase 6 (AUDT-01)

### Active

#### Hook Handler & Policy Engine
- [ ] `beekeeper check` reads tool call JSON from stdin, evals policy, exits allow/block — sub-100ms target
- [ ] Catalog matching against Bumblebee `threat_intel/`, OSV, Socket public API with corroboration-based semantics (1 source → warn, 2 sources → enforce, 3 sources → enforce + quarantine recommendation)
- [ ] Release-age policy for npm, PyPI, Cargo, RubyGems, Composer, Go modules — default 24h, configurable per-ecosystem
- [ ] Lifecycle script policy (allowlist-only for pre/post/install scripts across all ecosystems)
- [ ] Sensitive path policy (default blocklist: `~/.ssh/`, `~/.aws/`, `~/.gnupg/`, MCP configs, credential CLIs, `.env` files)
- [ ] Network egress policy per tool call with outbound size limits
- [ ] Behavioral baseline per project (frequency counters, deviation detection)
- [ ] Output filtering for credentials and sensitive data
- [ ] Multi-turn exfiltration detection (rolling entropy + base64 detection across turns)

#### Editor Extension Defense
- [ ] Intercept agent-initiated `code --install-extension`, `cursor --install-extension`, `windsurf --install-extension` commands
- [ ] File-watcher daemon (`beekeeper watch`) over `~/.vscode/extensions/`, `~/.cursor/extensions/`, `~/.windsurf/extensions/` via OS-native fs notifications
- [ ] On new extension: catalog match + release-age check + quarantine workflow
- [ ] `beekeeper scan` orchestrating Bumblebee with Beekeeper-specific rules on top

#### Catalog Sync
- [ ] `beekeeper catalogs sync` — fetch and cache Bumblebee, OSV, Socket catalogs
- [ ] `beekeeper catalogs watch` daemon — configurable interval (default 1h), catalog-delta-triggered immediate scan on new threat_intel entries
- [ ] Catalog signature verification; unsigned sources treated as warning-only
- [ ] Sanity bounds on catalog deltas with degraded-mode trigger

#### Integration Surfaces
- [ ] `beekeeper hooks install --target claude-code` writes PreToolUse/PostToolUse hooks to `~/.claude/settings.json`
- [ ] `beekeeper hooks install` for Cursor, Codex CLI
- [ ] MCP gateway daemon (`beekeeper gateway`) — long-lived proxy, applies policy in flight, localhost-only by default, per-session token auth
- [ ] Shim layer (`beekeeper shim install`) — PATH symlinks for npm, pnpm, pip, cargo, go, gem, composer, npx, pipx
- [ ] Continue, OpenCode, OpenClaw integrations via MCP gateway
- [ ] Multi-agent observability with parent-child policy inheritance

#### Sentry Daemon (protected-mode, opt-in)
- [ ] Process event ingestion (creation, exec, parent PID, descendant tree)
- [ ] File access events on sensitive-path watchlist
- [ ] Outbound network connection events with process attribution
- [ ] Default rule set (SENTRY-001..005), each gated on editor-descendant ancestry (code/cursor/windsurf/codium): SENTRY-001 credential-file cluster, SENTRY-002 credential-CLI burst, SENTRY-003 first-outbound phone-home (no domain allowlist in v1), SENTRY-004 fresh-extension correlation, SENTRY-005 exfil-signature fusion. Sentry is DETECTION-ONLY (writes audit records; it does not quarantine or kill — extension quarantine lives in the unprivileged watch/scan layer). Scope is the editor-extension-trojan family.
- ✓ v1.3.0 (Phase 20, SENT): SENTRY-006 (agent-descendant credential cluster), SENTRY-007 (generalized exfil fusion, no fresh-extension precondition), SENTRY-008 (persistence-location write); agent-CLI ancestry + file-write ingestion + cloud-credential watchlist expansion. Standalone-terminal agents and persistence writes are now in scope; CI runners, DNS, and process-memory remain out of scope.
- [ ] 7-day audit-only baseline period before rules promote to enforcement
- [ ] `beekeeper protect install` — installs Sentry as privileged service (systemd/launchd/Windows Service)
- [ ] Linux: fanotify + eBPF (`cilium/ebpf`)
- [ ] macOS: eslogger-based (no entitlement required for v1)
- [ ] Windows: ETW with relevant security providers

#### LlamaFirewall Sidecar (optional)
- ✓ Python sidecar supervised by Beekeeper; loopback-TCP + per-launch bearer-token IPC (one transport on every OS); fail-closed on sidecar crash; exponential-backoff restart — Phase 6, IPC reworked Phase 20
- ✓ PromptGuard 2 injection scan on tool outputs; llmf_alert audit record; fail-closed on sidecar unavailable OR scan error — Phase 6 / Phase 20 (the silent fail-open no-op was fixed)
- ✓ Real CodeShield on agent-generated code (opt-in, experimental; gated 22M model bootstrapped via `beekeeper llamafirewall install`). The cloud AlignmentCheck (Together AI) path was REMOVED — Phase 20 (LLMF); no agent context leaves the host

#### Audit Log & Observability
- ✓ NDJSON audit log — every policy decision, Bumblebee-schema-compatible, with catalog provenance — Phase 1 / Phase 6
- ✓ Sinks: local file (default, `0600`), opt-in syslog RFC 5424, OTLP, HTTPS POST — Phase 6
- ✓ `beekeeper audit tail`, `beekeeper audit query`, `beekeeper audit export` — Phase 6

#### TUI Dashboard
- [ ] `beekeeper dashboard` — Bubble Tea TUI, single screen, event-driven refresh
- [ ] Panels: live activity feed, Sentry alerts, catalog freshness, scan status, active policies, quarantine, system health
- [ ] Read-only by default; `--admin` flag enables policy toggling and quarantine actions

#### Policy as Code
- [ ] Declarative JSON policy files, version-controlled, testable
- [ ] `beekeeper policy test <file>` — dry-run against sample tool call
- [ ] `beekeeper policy validate <file>`
- [ ] Layered config: system → user → project → env vars → CLI flags

#### Self-Defense (built into every phase)
- [ ] Reproducible builds: deterministic Go build flags, hash verification against published releases
- [ ] Sigstore signing via GitHub Actions OIDC from v0.1.0
- [ ] Pinned dependencies (`go.mod` / `go.sum`) with Renovate-bot for updates
- [ ] `beekeeper-self` catalog — self-quarantine on known-compromised Beekeeper releases
- [ ] SLSA Level 3 provenance by v0.9.0
- [ ] SBOM (CycloneDX) published with each release
- [ ] `SECURITY.md` with disclosure process from v0.1.0

### Out of Scope

- Kernel-mode syscall blocking (true prevention) — v3 work; v1 Sentry detects and alerts
- Sandbox / microVM orchestration — v3
- Local LLM-based tool-call anomaly classifier — v3
- General-purpose Falco-equivalent rule engine — Beekeeper ships narrow, targeted rules only
- Desktop GUI or web UI — TUI only for v1
- Replacement for EDR, antivirus, or network firewalls — complement, not substitute
- Custom threat intelligence research — consumes upstream catalogs + small default ruleset only
- macOS EndpointSecurity entitlement — v1 uses eslogger; entitlement application is v2
- Sentry coverage of CI/CD runners and system daemons — Sentry monitors editor- and agent-CLI descendants only (SENTRY-006 added agent ancestry in v1.3.0); broader execution contexts are future work
- DNS-tunneling and process-memory-scrape detection — need new event sources (Linux/Windows v1.x, macOS v2)
- Exfil over legitimate/allowlisted endpoints (GitHub API, AWS services, npm registry) — host-undetectable; architectural mitigation only, not a Sentry rule

## Context

- **Origin:** Triggered by the Nx Console VS Code extension compromise (May 2026, TeamPCP), which exfiltrated ~3,800 GitHub-internal repos via a trojanized extension. Perplexity open-sourced Bumblebee shortly after. Wanted to contribute Windows support to Bumblebee but a PR was already opened; saw the deeper gap — Bumblebee is read-only inventory/detection, no runtime enforcement for the agent tool-call layer.
- **Bumblebee relationship:** Beekeeper consumes Bumblebee as its primary scan orchestrator and `threat_intel/` catalog source. Schema-compatible NDJSON output. Not a fork — a harness on top.
- **Dev environment:** Windows-primary developer. No WSL heavy integration tests (RAM/disk constraints). Cross-platform validation (Linux, macOS) runs in CI only (GitHub Actions `ubuntu-latest`, `macos-latest` Intel + Apple Silicon). macOS-specific debugging via ad-hoc EC2 mac instance time when CI iteration is insufficient.
- **Build approach:** AI-native development — Claude Code on Max plan + GSD harness as the primary execution framework. Treats itself as the first dogfood target.
- **Trust threshold:** The full product (v1.0.0 with TUI + policy as code) is the milestone where the developer trusts it on real work. v0.1.0 is the first public ship; prototyping the full system to validate the whole architecture.

## Constraints

- **Tech:** Go 1.25+ single static binary, minimal non-stdlib dependencies. Python 3.11+ for LlamaFirewall sidecar only. TypeScript/Bun for v1.5 hook scaffolds (npm-distributed).
- **Platform:** Windows as primary dev/dogfood machine. WSL not viable for integration tests. GitHub Actions for Linux/macOS CI.
- **Foundation risk:** Windows-first CI matrix from commit 1 — if cross-platform CI isn't solid from the start, macOS/Linux bugs accumulate silently.
- **Solo:** Single developer. Two-account release approval enforced even solo (second-credential-theft mitigation).
- **Distribution:** `go install` and GitHub Releases with Sigstore. `curl | sh` not recommended; documented honestly with trade-off explanation.
- **License:** Apache 2.0 (locked, matching Bumblebee).

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go single static binary | Audit-friendly, easy distribution via `go install`, no CGO for core, eliminates C-class memory corruption | — Pending |
| Corroboration-based catalog matching (not union-of-bad) | Single compromised source can't trigger enforcement; requires 2+ independent sources to enforce — 2FA principle for threat intel | — Pending |
| Opt-in elevation for Sentry | New OSS tool; requiring admin at install is a trust ask before users know the project; let them evaluate unprivileged tier first | — Pending |
| LlamaFirewall via Python sidecar, not embedded | PromptGuard 2 + PyTorch live in Python; Go embedding would require CGO and obscure the boundary | — Pending |
| Bubble Tea for TUI | Mature, single-binary-friendly, no CGO requirement, accessible output, event-driven | — Pending |
| Fail-closed by default for hook handler | Crash or timeout → block; `fail_open` documented as reducing security | Shipped Phase 1 — top-level recover() → block; explicit fail_open is an opt-in, documented as reducing security; benchmarked at ~3.58ms/op on Celeron N4020 |
| MCP gateway localhost-only by default | Security-sensitive proxy; remote exposure requires explicit flag + config acknowledgment | — Pending |
| eslogger on macOS v1 (not EndpointSecurity entitlement) | Entitlement path is uncertain and slow for indie OSS; eslogger works with sudo, no entitlement | — Pending |
| Remote audit sink errors are fire-and-forget (nil returned) | A syslog/OTLP/HTTPS outage must never block or fail the local NDJSON write; fail-closed principle preserved for local audit trail | Shipped Phase 6 |
| AuditConfig imported by audit package from internal/config | Avoided struct duplication; no import cycle since config imports only stdlib | Shipped Phase 6 |
| LlamaFirewall injection detection exits 0 (not 1) in hook handler | PostToolUse hooks must not block agent flow; llmf_alert audit record is the forensic signal | Shipped Phase 6 |
| scan_code / scan_alignment are stubs in Python sidecar | CodeShield/AlignmentCheck require separate model integration; stubs unblock supervisor + IPC work | Shipped Phase 6 — follow-on item |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-06-22 — SHIPPED & ARCHIVED milestone v1.5.0 "Install Posture" (released publicly as v1.1.0, maintainer-signed tag on merged main): retired the package-manager nudge and shipped tool-agnostic install posture (three rules at the pre-exec hook warn + fail-soft; read-only `beekeeper posture` view; scoped audited overrides + per-rule warn→block opt-up; SENTRY-009 detection-only install observation; honest enforcement-boundary statement everywhere). All 18 reqs satisfied; audit `tech_debt` (zero blockers — see `milestones/v1.5.0-MILESTONE-AUDIT.md`); both human gates cleared (Gate 1 enforcement-boundary review, Gate 2 maintainer release signing). Posture is warn-only, most-restrictive-merged, structurally isolated (cannot lift a catalog/SPATH block, T-09-31). Deferred to roadmap: per-ecosystem policy matrix, shim as a first-class surface, config mutation (PRD Layer 4); tech debt M-01/M-02 + lows tracked in 31-REVIEW-DECISION.md. Prior: v1.4.0 milestone — "Adjudicated Corpus (Local Loop)" SHIPPED & ARCHIVED (`milestones/v1.4.0-*`): the four-layer moat schema + frozen push-envelope, the append-only corpus store + off-hot-path adjudication engine, First Responder corpus binding, and the launch-readiness gate — all 32 reqs verified, no transport stood up. The milestone audit caught the FRB-05 enforcement gap (local overlay never read by the check path) and it was fixed same-day (quick task 260615-ky4; allow→warn). Carried forward: overlay wiring for gateway/scan/watch + the warn-vs-block design question; PRD §6/§7 (org push / community feed) as future milestones. v1.0.0 + v1.2.0 + v1.3.0 SHIPPED & ARCHIVED. v1.1.0 "Pollen" remains PARKED at its maintainer release checkpoint (independent — not archived). Next milestone TBD via `/gsd-new-milestone`.*
