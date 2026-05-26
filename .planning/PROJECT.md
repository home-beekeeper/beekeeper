# Beekeeper

## What This Is

Beekeeper is a real-time safety harness for autonomous coding agents (Claude Code, Cursor, Codex CLI, Continue, OpenCode, OpenClaw). It mediates every tool call, package install, file access, and network egress against unified threat intelligence — catching compromised packages, hijacked agents, and malicious editor extensions before they act, not after. It wraps existing agents in an evaluator-generator pattern: the agent is the generator, Beekeeper is the evaluator.

Built by Mfanafuthi Mhlanga / Mzansi Agentive Pty Ltd. Solo project, public from day one, Apache 2.0.

## Core Value

A hijacked or off-task agent cannot successfully act on the developer's machine without Beekeeper deciding to permit it.

## Requirements

### Validated

- ✓ `beekeeper check` reads tool call JSON from stdin, evals policy, exits allow/block (single-source Bumblebee warn semantics; corroboration block deferred to Phase 2) — Phase 1
- ✓ Catalog matching against Bumblebee `threat_intel/` with mmap binary index (O(log n) lookup, zero JSON parse on hot path) — Phase 1
- ✓ NDJSON audit log with owner-only permissions (0600) and `beekeeper audit tail` — Phase 1
- ✓ `beekeeper catalogs sync` fetches and caches Bumblebee catalog (654 entries, mmap index) — Phase 1
- ✓ Reproducible builds (`-trimpath -buildvcs=false`), Sigstore/cosign v3 keyless signing, Renovate dependency pinning, `SECURITY.md` — Phase 1

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
- [ ] PromptGuard 2 (86M BERT) scanning tool outputs flowing into agent context for prompt injection
- [ ] CodeShield on agent-generated file writes containing code
- [ ] AlignmentCheck (experimental, optional) for goal hijacking signals
- [ ] Python sidecar supervised by Beekeeper; Unix socket / Windows named pipe IPC; fail-closed on sidecar crash

#### Audit Log & Observability
- [ ] NDJSON audit log — every policy decision, Bumblebee-schema-compatible, with catalog provenance
- [ ] Sinks: local file (default, `0600` permissions), optional syslog (RFC 5424), optional OpenTelemetry OTLP, optional HTTPS POST
- [ ] `beekeeper audit tail`, `beekeeper audit query`, `beekeeper audit export`

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
*Last updated: 2026-05-26 after Phase 1*
