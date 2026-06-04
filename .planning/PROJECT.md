# Beekeeper

## What This Is

Beekeeper is a real-time safety harness for autonomous coding agents (Claude Code, Cursor, Codex CLI, Continue, OpenCode, OpenClaw). It mediates every tool call, package install, file access, and network egress against unified threat intelligence — catching compromised packages, hijacked agents, and malicious editor extensions before they act, not after. It wraps existing agents in an evaluator-generator pattern: the agent is the generator, Beekeeper is the evaluator.

Built by Mfanafuthi Mhlanga / Mzansi Agentive Pty Ltd. Solo project, public from day one, Apache 2.0.

## Core Value

A hijacked or off-task agent cannot successfully act on the developer's machine without Beekeeper deciding to permit it.

## Current State

**v1.0.0 — Comprehensive Standalone Release — SHIPPED 2026-06-01.** All 10 phases complete (50 plans, 53 tasks, 270 commits over 7 days). The full v1.0.0 requirement set (HOOK/CTLG/SFDF/PLCY/EDXT/INTG/SLNX/LLMF/AUDT/SMAC/SWIN/TUI/CODE) is implemented, verified, and archived in [`milestones/v1.0.0-REQUIREMENTS.md`](milestones/v1.0.0-REQUIREMENTS.md); the milestone audit (PASSED) is in [`milestones/v1.0.0-MILESTONE-AUDIT.md`](milestones/v1.0.0-MILESTONE-AUDIT.md).

Shipped: single static Go binary; pure corroboration policy engine; editor-extension defense; Claude Code/Cursor/Codex hooks + MCP gateway + shims; cross-platform Sentry (Linux eBPF/fanotify, macOS eslogger, Windows ETW); supervised LlamaFirewall sidecar + full audit sinks; Bubble Tea v2 TUI; declarative policy-as-code enforced live across check/gateway/watch/scan; layered config; `beekeeper diag`; the `beekeeper-self` self-quarantine client; reproducible builds + Sigstore + SLSA L3 + SBOM; public `docs/THREAT-MODEL.md`.

**Carried to v1.x:** live external `beekeeper-self` hosting (separate host + signing key) + end-to-end refuse-to-run; independent external security review + VDP publication; distributed mode / team-shared catalogs; weighted corroboration. Tech stack: Go 1.25, no CGO in core; Python 3.11+ optional sidecar; Bubble Tea v2.

## Current Milestone: v1.2.0 Runtime Behavioral Hardening

**Goal:** Close the three runtime-enforcement gaps that live `beekeeper check` validation exposed (with the agent itself as the test subject), so a hijacked agent cannot read credential files, slip malware past via `pnpm`/`bun`, or install a *critical*-severity package on a warn-only pass — each locked in by a behavioral test suite.

**Progress (2026-06-04):** Phase 6 (corroboration severity hardening, CORR-01/02 — F1) ✅ complete & verified. Phase 7 (PLCY-05/SPATH-01..04 — F2) ✅ complete & verified — `EvaluatePath`/`DefaultSensitivePaths` is now wired live into `runCheck`: credential reads (`~/.aws/credentials`, `~/.ssh/id_rsa`, `.env`, MCP configs) and `cat`/`type %USERPROFILE%`/`Get-Content` shell targets block fail-closed, traversal/tilde/Windows-env-var bypasses canonicalized away, `.env.example/.test/.schema` allowlisted; overlay can escalate but never downgrade a path block. Next: **Phase 8** (NUDGE + BTEST — F3 + the behavioral test suite / live-binary E2E release gate).

**Target features:**
- **PLCY-05** (sensitive-path enforcement, F2) — wire the already-built `EvaluatePath`/`DefaultSensitivePaths` engine into the live `beekeeper check` path: evaluate `file_path` (Read/Write/Edit) **and** command targets (`cat`/`type`/`Get-Content` of credential files) against the sensitive-path policy; fail-closed; allowlist + policy-overlay merged. Today that engine is referenced only by its own test, so credential reads (`~/.aws/credentials`, `~/.ssh/id_rsa`, `~/.npmrc`, `.env`) return ALLOW.
- **NUDGE** package-manager nudge (F3) — full spec in [`.planning/specs/NUDGE-PRD.md`](specs/NUDGE-PRD.md). New `internal/nudge/` package: detect local pnpm(>=11)/bun(>=1.3)/node(>=22), recognize npm/pnpm/bun/yarn/npx install patterns, soft-advise by default / hard-rewrite on opt-in / block when `requireHardened` and no hardened PM is present; `record_type:"nudge"` audit; `beekeeper nudge status|check|audit` CLI; wired into check + gateway + shim. Also closes the F3 gap that `pnpm`/`bun` installs are currently unparsed and bypass catalog matching.
- **PLCY-07** corroboration hardening (F1) — a *critical*-severity catalog match must not pass warn-only. Today `npm install ai-figure` (Shai-Hulud worm, OSV `MAL-2026-4126`, matched bumblebee+OSV) only WARNED because bumblebee entries are unsigned (`Signed:false`) → `CorroborationCount:1 < BlockAt:2`. Add per-severity escalation (critical → block at one trusted source) or treat the bundled catalog as signed-equivalent, with sanity bounds + documented false-positive rigor.
- **BTEST** behavioral test suite (cross-cutting) — table-driven pure-policy tests + check-handler integration tests (stdin→decision) + a live-binary E2E battery (catalog-backed) mirroring the validation run that surfaced these gaps. Threaded through every phase per the project's testing requirements.

**Scope source:** `.planning/specs/NUDGE-PRD.md` (nudge feature) + the runtime-validation findings F1/F2/F3 (captured in this milestone's discuss/plan artifacts). Detailed REQ-IDs in `.planning/REQUIREMENTS.md`.

**Architecture constraint reminder:** `internal/policy` stays a pure function library — nudge detection I/O lives in `internal/nudge/detect.go`, and `nudge.Evaluate` decides over a caller-resolved `PMState`, mirroring `policy.EvaluateReleaseAge(ReleaseAgeInput, …)`.

## Parked Milestone: v1.1.0 Pollen (paused at maintainer release checkpoint — NOT closed)

> **Status: PARKED, not complete.** v1.1.0 is paused at its 05-05 maintainer release checkpoint (auth-gated GitHub push + signed tags `pollen.2/.3/.4/.5` + cosign verify). Resume artifacts are preserved: `HANDOFF.json`, `.planning/phases/05-contribution-back-milestone-close/.continue-here.md`, and `docs/release-runbook.md`. The four signed-tag releases remain in STATE.md "Deferred Items". Do **not** archive or close v1.1.0 until the maintainer runs the runbook and completes 05-05 Task 3. v1.2.0 phases continue the numbering at 06+ so the parked `05-*` phase dir is untouched.

**Goal:** Own Windows inventory compatibility by forking Bumblebee into a bounded Apache-2.0 derivative ("Pollen"), so the Windows CI matrix goes fully green and cross-platform test discipline holds instead of rotting behind `t.Skip`.

**Target features:**
- Pollen fork hygiene — separate module `github.com/bantuson/pollen`, Apache-2.0 LICENSE/NOTICE/CHANGES/UPSTREAM, renamed `cmd/pollen`, reproducible builds + Sigstore
- Windows root resolver for 8 ecosystems (npm/pnpm/Yarn/Bun/PyPI/Go modules/RubyGems/Composer)
- Windows NDJSON path representation (native backslash paths, drive letters, `endpoint.os="windows"`, empty UID)
- Windows editor/browser/MCP config path coverage (VS Code family, Chromium + Firefox, Claude Desktop/Cursor/Windsurf/Cline/Gemini)
- Parity + differential tests; the Pollen compatibility test in beekeeper CI drives the Windows skip baseline to zero
- Upstream sync discipline + contribution-back PRs to `perplexityai/bumblebee`

**Scope source:** `beekeeper-m2-prd.md` (repo root). Detailed REQ-IDs in `.planning/REQUIREMENTS.md`. Spans two repos: the new `bantuson/pollen` module **+** beekeeper `internal/inventory/` integration across a subprocess boundary (`internal/scan` swaps the `bumblebee` binary call for `pollen`). Pollen versions separately as `v0.1.1-pollen.1…5`, one per sub-phase. `threat_intel/` catalogs keep flowing through beekeeper's own `catalogs sync` (not duplicated in Pollen).

**Progress (4/5 phases):** Phase 1 Fork Setup & Discipline ✓ (tag `v0.1.1-pollen.1` shipped) · Phase 2 Windows Root Resolver ✓ (code complete & verified; signed tag deferred to M2 close) · Phase 3 Windows Path Representation ✓ (WPATH-01/02 — `filepath.FromSlash` path preservation + empty-Windows-uid guard; verified 2026-06-02; `v0.1.1-pollen.3` tag deferred) · **Phase 4 Windows Extension & MCP Coverage ✓** (WEXT-01/02/03 — Windows editor/browser/MCP root enumeration in Pollen `cmd/pollen/roots.go` + `internal/ecosystem/editorext`; BKINT-01 — beekeeper `internal/scan` subprocess seam renamed `bumblebee`→`pollen` **in place**; PTEST-04 — fixture-driven `TestPollenCompatibility` passes with **zero Windows `t.Skip`**; verified 2026-06-02; `v0.1.1-pollen.4` tag deferred to M2 close). Phase 5 (contribution-back & milestone close) not started. The PRD's idealized `internal/resolver/` / `internal/output/paths_windows.go` layout does **not** match the live fork — Windows fixes live in `cmd/pollen/roots.go` + `internal/ecosystem/` (+ `internal/endpoint` for Phase 3). **BKINT-01 decision (revised):** the beekeeper↔Pollen boundary is an in-place `internal/scan` seam rename (`runPollenFn`/`lookPollenFn`/`defaultRunPollen`), **not** a separate `internal/inventory/` package — research-confirmed as the smaller, mockable diff; the `internal/inventory/` reservation noted in earlier phases was dropped.

## Requirements

### Validated

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
- [ ] Default rule set: extension-host credential cluster, credential CLI burst, phone-home, fresh-extension behavior correlation, exfil signature fusion
- [ ] 7-day audit-only baseline period before rules promote to enforcement
- [ ] `beekeeper protect install` — installs Sentry as privileged service (systemd/launchd/Windows Service)
- [ ] Linux: fanotify + eBPF (`cilium/ebpf`)
- [ ] macOS: eslogger-based (no entitlement required for v1)
- [ ] Windows: ETW with relevant security providers

#### LlamaFirewall Sidecar (optional)
- ✓ Python sidecar supervised by Beekeeper; Unix socket / Windows named pipe IPC; fail-closed on sidecar crash; exponential-backoff restart — Phase 6
- ✓ PromptGuard 2 injection scan on tool outputs; llmf_alert audit record; fail-closed on sidecar unavailable — Phase 6
- ✓ CodeShield on agent-generated file writes; AlignmentCheck wired in gateway (scan_code / scan_alignment stubs ready for model integration) — Phase 6

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
*Last updated: 2026-06-04 — v1.2.0 Phase 7 (PLCY-05/SPATH sensitive-path enforcement, F2) complete & verified; Phase 6 (CORR, F1) complete. Next: Phase 8 (NUDGE + BTEST, F3 + behavioral test suite). v1.1.0 Pollen parked at its maintainer release checkpoint (not closed).*
