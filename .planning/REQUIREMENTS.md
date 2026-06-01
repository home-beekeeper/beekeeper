# Requirements: Beekeeper

**Defined:** 2026-05-26
**Core Value:** A hijacked or off-task agent cannot successfully act on the developer's machine without Beekeeper deciding to permit it.

## v1 Requirements

Requirements for v1.0.0 release. Each maps to roadmap phases.

### Hook Handler (HOOK)

- [ ] **HOOK-01**: `beekeeper check` reads tool call JSON from stdin, evaluates policy engine, exits 0 (allow) or non-zero (block) with structured reason — sub-100ms p95 target
- [ ] **HOOK-02**: Hook handler loads catalog index via mmap (pre-built by `catalogs sync`), not cold-loading from JSON per invocation
- [ ] **HOOK-03**: Hook handler fails closed by default (crash or timeout → block); `fail_open` and `fail_warn` modes configurable
- [ ] **HOOK-04**: Hard caps on stdin size (1MB), execution time (5s), memory enforced

### Policy Engine (PLCY)

- [x] **PLCY-01**: Catalog matching with corroboration semantics — 1 independent source → warn; 2 sources → enforce (block); 3 sources → enforce + quarantine recommendation
- [ ] **PLCY-02**: Release-age policy for npm, PyPI, Cargo, RubyGems, Composer, Go modules — default 24h minimum age, configurable per-ecosystem and per-package via allowlist
- [ ] **PLCY-03**: Lifecycle script policy — allowlist-only deny for `preinstall`, `postinstall`, `install` scripts across all ecosystems; structured block with reason returned to agent
- [ ] **PLCY-04**: Sensitive path policy — default blocklist: `~/.ssh/`, `~/.aws/`, `~/.gnupg/`, `~/.config/Claude/`, all MCP config files, `~/.config/op/`, `~/.config/gh/`, `~/.netrc`, `~/.npmrc`, `~/.pypirc`, `~/.cargo/credentials.toml`, `.env`/`.env.local`/`.env.*` files
- [ ] **PLCY-05**: Network egress policy — per-tool egress allowlists, outbound size limits per tool call
- [ ] **PLCY-06**: Multi-turn exfiltration detection — rolling entropy of recent outputs, base64 detection across turns
- [x] **PLCY-07**: Behavioral baseline engine — per-project frequency counters keyed by (tool, target_pattern), deviation triggers warn-level decisions; counter-based, no ML
- [ ] **PLCY-08**: Output credential filtering — redact API key prefixes, JWT tokens, `Bearer` headers from tool outputs before returning to agent

### Catalog Management (CTLG)

- [ ] **CTLG-01**: Bumblebee `threat_intel/` catalog sync — primary source, extended with `source_url`, `catalog_signature`, `catalog_source` fields; Beekeeper records set `scanner_name: "beekeeper"`
- [ ] **CTLG-02**: OSV catalog source — queried via public REST API (`POST https://api.osv.dev/v1/query`, no auth required); results cached by package+version with 24h TTL in `~/.beekeeper/catalogs/osv-cache/`; the `osv-scanner/v2` library's `localmatcher` is internal and not usable for single-package queries
- [ ] **CTLG-03**: Socket public API — PURL endpoint (`POST /v0/purl`), results cached by package+version with 24h TTL; exponential backoff on 429
- [x] **CTLG-04**: `beekeeper-self` catalog — self-quarantine feed listing known-compromised Beekeeper releases by version + signature hash; checked on every startup and catalog sync
- [ ] **CTLG-05**: `beekeeper catalogs sync` — fetch and cache all enabled catalog sources
- [ ] **CTLG-06**: `beekeeper catalogs watch` daemon — configurable poll interval (default 1h), triggers immediate Bumblebee scan on any new `threat_intel/` catalog entries
- [ ] **CTLG-07**: Catalog signature verification — unsigned sources treated as warning-only regardless of corroboration count
- [ ] **CTLG-08**: Sanity bounds on catalog deltas — per-sync delta size limits, per-catalog rule count limits; exceeding limits triggers degraded-mode (new entries warning-only until user review)
- [x] **CTLG-09**: Catalog provenance in every NDJSON audit record — which catalog, which catalog version, which corroborated, which dissented

### Integration Surfaces (INTG)

- [ ] **INTG-01**: `beekeeper hooks install --target claude-code` — writes PreToolUse and PostToolUse hooks to `~/.claude/settings.json`
- [ ] **INTG-02**: `beekeeper hooks install --target cursor` and `--target codex` — hook installation for Cursor and Codex CLI
- [x] **INTG-03**: MCP gateway daemon — `beekeeper gateway`, stateless per-request HTTP proxy (MCP July 2026 spec: no session state, no initialize/initialized handshake); binds `127.0.0.1` by default; per-session token auth required even on localhost
- [x] **INTG-04**: MCP gateway applies policy engine inline to every proxied tool call; JSON-RPC response correlation by `id` field (not position) for correct batch handling
- [ ] **INTG-05**: MCP gateway integrations for Continue, OpenCode, and OpenClaw via gateway mode
- [ ] **INTG-06**: Shim layer — `beekeeper shim install` symlinks wrapper binaries earlier in PATH for npm, pnpm, pip, cargo, go, gem, composer, npx, pipx; each shim invokes `beekeeper check` then proxies to real binary if allowed
- [ ] **INTG-07**: Multi-agent observability — parent-child policy inheritance; subagent context propagation in policy decisions

### Editor Extension Defense (EDXT)

- [ ] **EDXT-01**: Agent-initiated CLI intercept — recognize `code --install-extension`, `code-insiders --install-extension`, `cursor --install-extension`, `windsurf --install-extension` (including bulk multi-flag forms); route through catalog matcher and release-age policy
- [ ] **EDXT-02**: File-watcher daemon (`beekeeper watch`) — OS-native filesystem notifications via fsnotify v1.10.1 over `~/.vscode/extensions/`, `~/.cursor/extensions/`, `~/.windsurf/extensions/`; explicit per-directory `watcher.Add()` (no recursive API), filter `Create` events only on Windows NTFS
- [ ] **EDXT-03**: On new extension directory: parse manifest, catalog match, release-age check; on hit → emit critical audit record, desktop notification (configurable), quarantine (move to `~/.beekeeper/quarantine/extensions/`)
- [ ] **EDXT-04**: `beekeeper scan` — orchestrates Bumblebee scan + applies Beekeeper-specific rules on top; merges output to unified NDJSON stream; catalog-delta-triggered via `catalogs watch`
- [ ] **EDXT-05**: Quarantine workflow — `beekeeper quarantine list`, `quarantine restore <id>`, `quarantine purge`
- [ ] **EDXT-06**: `beekeeper init` detects installed editors and offers to disable extension auto-update (writes editor setting on consent) and enable file-watcher for detected extension directories

### Sentry Daemon — Linux (SLNX)

- [ ] **SLNX-01**: `beekeeper protect install` installs Sentry as privileged systemd unit; `beekeeper protect uninstall`, `beekeeper protect status`
- [ ] **SLNX-02**: Process event ingestion via eBPF tracepoints (`cilium/ebpf v0.21.0`, `bpf2go` generated, pre-compiled bytecode embedded at build time) — process creation, exec, parent PID, descendant tree reconstruction
- [ ] **SLNX-03**: File access event ingestion via fanotify (`CAP_SYS_ADMIN`) using `golang.org/x/sys/unix` — reads of paths in sensitive-path watchlist; carries accessing process identity
- [ ] **SLNX-04**: Outbound network connection events via eBPF — connection initiation with 5-tuple and process attribution
- [ ] **SLNX-05**: Capability probing at Sentry startup via `cilium/ebpf/features` — degrade gracefully on kernel < 5.15; minimum floor 5.4 (fanotify only, no ring buffer); surface degradation in `beekeeper protect status`
- [ ] **SLNX-06**: Ring buffer reader in dedicated goroutine feeding buffered channel (cap 10000) to correlation engine goroutine — decouples ingestion rate from evaluation latency
- [ ] **SLNX-07**: IPC authorization between unprivileged CLI and privileged Sentry daemon via Unix socket; peer credential verification via `unix.GetsockoptUcred` (SO_PEERCRED)
- [ ] **SLNX-08**: Default rule set: extension-host credential cluster, extension-host credential CLI burst, extension-host phone-home, fresh-extension behavior correlation, exfil signature fusion — each with severity and configurable notification threshold
- [ ] **SLNX-09**: 7-day audit-only baseline period before Sentry rules promote to enforcement; configurable (0 = immediate, "indefinite" = always audit-only)
- [ ] **SLNX-10**: Privilege separation — eBPF loading privileged; rule evaluation and event filtering drop capabilities; seccomp-bpf filters on unprivileged Sentry components

### Sentry Daemon — macOS (SMAC)

- [ ] **SMAC-01**: `beekeeper protect install` installs Sentry as privileged launchd plist
- [ ] **SMAC-02**: Process/file/network event ingestion via `eslogger` subprocess (`exec.CommandContext`); reads NDJSON from stdout pipe; normalizes to common `SentryEvent` struct; requires `sudo`
- [ ] **SMAC-03**: Applies same default rule set as Linux Sentry on normalized events; same 7-day baseline period
- [ ] **SMAC-04**: Documents eslogger coverage gaps in `beekeeper protect status` — Keychain/Security framework API access not observable; in-memory Cocoa API operations not observable

### Sentry Daemon — Windows (SWIN)

- [ ] **SWIN-01**: `beekeeper protect install` installs Sentry as Windows Service (LocalService privileges); `beekeeper protect uninstall`, `beekeeper protect status`
- [ ] **SWIN-02**: Event ingestion via ETW using `tekert/golang-etw` (no CGO); providers: `Microsoft-Windows-Kernel-Process` (process events), `Microsoft-Windows-Security-Auditing` (file/network events)
- [ ] **SWIN-03**: Probe for existing NT Kernel Logger session conflict at install time (only one session per machine); surface conflict in `beekeeper protect status` if EDR already owns the session
- [x] **SWIN-04**: Surface `EventsLost` count in `beekeeper diag` — ETW rate-limits under load; user knows when detection coverage is degraded
- [ ] **SWIN-05**: IPC between unprivileged CLI and Sentry service via Windows named pipe with DACL granting access only to installing user's SID
- [ ] **SWIN-06**: Applies same default rule set and 7-day baseline on normalized ETW events

### LlamaFirewall Sidecar (LLMF)

- [x] **LLMF-01**: Optional Python 3.11+ sidecar, disabled by default; Beekeeper supervises process lifecycle (restarts up to 3× with exponential backoff on crash)
- [x] **LLMF-02**: PromptGuard 2 (86M BERT) scans tool outputs flowing into agent context — WebFetch results, file reads, MCP tool responses; detected injection → redacted payload + structured warning returned to agent
- [x] **LLMF-03**: CodeShield runs on agent-generated file writes containing code — insecure patterns flagged or blocked per policy
- [x] **LLMF-04**: AlignmentCheck (experimental, opt-in separately) inspects chain-of-thought for goal hijacking signals
- [x] **LLMF-05**: IPC via Unix domain socket (Linux/macOS) or Windows named pipe using length-prefixed JSON; Beekeeper applies configurable fail mode on sidecar unavailability (fail_closed by default)
- [x] **LLMF-06**: Configurable sample rate (1.0 = scan every tool output); latency budget surfaced in `beekeeper diag` (target sub-100ms p95 per PromptGuard 2 benchmarks)

### Audit & Observability (AUDT)

- [ ] **AUDT-01**: NDJSON audit log — every policy decision emits one record with `record_type`, `record_id`, `scanner_name: "beekeeper"`, `agent_name`, `tool_name`, `decision`, `reason`, `rule_ids`, `catalog_matches` (with full provenance), `endpoint`; Bumblebee-schema-compatible
- [ ] **AUDT-02**: Local file sink — `audit/beekeeper.ndjson`, rotated, `0600` permissions enforced from first write on Unix; equivalent ACLs on Windows; 30-day default retention
- [ ] **AUDT-03**: Optional syslog sink (RFC 5424) — off by default; opt-in with clear awareness in config that audit data leaves machine
- [ ] **AUDT-04**: Optional OpenTelemetry OTLP sink — off by default; opt-in
- [ ] **AUDT-05**: `beekeeper audit tail` — stream live audit log to terminal
- [ ] **AUDT-06**: `beekeeper audit query --since|--agent|--tool|--decision` — filtered log queries
- [ ] **AUDT-07**: `beekeeper audit export --format {ndjson|csv|otlp}` — export audit data

### TUI Dashboard (TUI)

- [ ] **TUI-01**: `beekeeper dashboard` — Bubble Tea v2 (`charm.land/bubbletea/v2`), single screen, sshable; event-driven refresh via file watcher on audit log, 1-second polling fallback
- [ ] **TUI-02**: Live activity feed panel — tool calls in real time with decision indicator (allow/warn/block), agent identity, tool name, target; filterable
- [ ] **TUI-03**: Sentry alerts panel — process correlation events, severity-color-coded, expandable detail (process tree, file accesses, network destinations)
- [ ] **TUI-04**: Catalog freshness panel — per-source last sync, delta count, next sync, stale indicator
- [ ] **TUI-05**: Scan status panel — last Bumblebee scan timestamp, findings count, next scheduled scan, one-key immediate scan trigger
- [ ] **TUI-06**: Active policies panel — loaded policy files with rule counts; drill-down shows individual rules with enabled/disabled state
- [ ] **TUI-07**: Quarantine panel — items in `~/.beekeeper/quarantine/` with restore and purge actions
- [ ] **TUI-08**: System health panel — Sentry daemon status + CPU/memory, gateway daemon status, LlamaFirewall sidecar status + inference latency
- [ ] **TUI-09**: Admin mode (`--admin` flag) — enables policy toggling and quarantine actions without leaving TUI
- [ ] **TUI-10**: Windows resize workaround — polling goroutine via `golang.org/x/term` sends synthetic `WindowSizeMsg` every 500ms (Bubble Tea v2 Windows resize regression #1601)

### Policy as Code (CODE)

- [x] **CODE-01**: Declarative JSON policy files in `policies/` — loaded by policy engine; separate from config; version-controllable
- [x] **CODE-02**: `beekeeper policy test <file>` — dry-run policy against sample tool call JSON
- [x] **CODE-03**: `beekeeper policy validate <file>` — validate policy file schema
- [x] **CODE-04**: `beekeeper policy list` — list loaded policy files with rule counts
- [x] **CODE-05**: Layered config merge — `/etc/beekeeper/config.json` → `~/.beekeeper/config.json` → `<project>/.beekeeper/config.json` → `BEEKEEPER_*` env vars → CLI flags
- [x] **CODE-06**: `beekeeper diag` — display hook latency p95/p99, sidecar inference latency, catalog freshness per source, ETW `EventsLost` count

### Self-Defense (SFDF)

- [ ] **SFDF-01**: Reproducible builds from v0.1.0 — deterministic Go build flags (`-trimpath -buildvcs=false -mod=readonly`), `make verify-release VERSION=X.Y.Z` target that reproduces and compares hashes
- [ ] **SFDF-02**: Sigstore signing from v0.1.0 — GitHub Actions OIDC, cosign v3 (`--bundle artifact.sigstore.json`), GoReleaser v2.13.0+; no long-lived signing keys
- [ ] **SFDF-03**: Pinned dependencies from v0.1.0 — `go.mod` and `go.sum` with CI `go mod verify`; Renovate-bot with human review for updates
- [ ] **SFDF-04**: `SECURITY.md` published from v0.1.0 — responsible disclosure process, 48h acknowledgment SLA, 90-day coordinated disclosure default
- [ ] **SFDF-05**: SLSA Level 3 provenance by v0.9.0 — `slsa-github-generator@v2.1.0` (full semver pinned, NOT `@v2`); SBOM (CycloneDX) published with each release
- [x] **SFDF-06**: `beekeeper-self` catalog live at v1.0.0 — separate hosting from main repo, separate signing key, separate access control; self-quarantine fires on startup and every catalog sync

## v2 Requirements

Deferred to future releases.

### Distribution & Ecosystem
- **DIST-01**: macOS EndpointSecurity entitlement — full ES API replacing eslogger; requires Apple entitlement approval
- **DIST-02**: SARIF export for security team workflows
- **DIST-03**: TypeScript/Bun hook scaffolds distributed via npm (`beekeeper-hooks` package) — v1.5 deliverable
- **DIST-04**: Webhook policy for tool call decisions (custom integrations)
- **DIST-05**: Cross-host correlation for team deployments
- **DIST-06**: Audit log analytics and trend detection

### Hardening
- **HARD-01**: Source health velocity monitoring — entries-per-unit-time per source per ecosystem; anomalous velocity → degrade that source regardless of delta size bounds
- **HARD-02**: Weighted corroboration — Bumblebee counts as 1.5 vs. unsigned user catalog as 0.5 if empirical data justifies
- **HARD-03**: ContextForge / MCPGuard policy-plugin mode — expose policy engine through host gateway's plugin interface
- **HARD-04**: Independent annual security review cadence

## Out of Scope

| Feature | Reason |
|---------|--------|
| Kernel-mode syscall blocking (true prevention) | Requires deeper kernel work; v1 Sentry detects and alerts only; v3 work |
| Sandbox / microVM orchestration | v3 work |
| Local LLM-based tool-call anomaly classifier | v3 work |
| General-purpose Falco-equivalent rule engine | Beekeeper ships narrow, targeted rules only; not trying to be a generic rule engine |
| Desktop GUI or web UI | TUI only for v1 |
| EDR / antivirus / network firewall replacement | Complement, not substitute |
| Custom threat intelligence research | Consumes upstream catalogs + small default ruleset only |
| ContextForge / MCPGuard plugin mode | v1.5 deliverable, not v1.0 |
| Extension code blocking pre-activation | Requires hooking the editor's extension loader; out of scope, requires editor-vendor cooperation |

## Traceability

Populated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| HOOK-01 | Phase 1 | Pending |
| HOOK-02 | Phase 1 | Pending |
| HOOK-03 | Phase 1 | Pending |
| HOOK-04 | Phase 1 | Pending |
| CTLG-01 | Phase 1 | Pending |
| CTLG-05 | Phase 1 | Pending |
| CTLG-07 | Phase 1 | Pending |
| SFDF-01 | Phase 1 | Pending |
| SFDF-02 | Phase 1 | Pending |
| SFDF-03 | Phase 1 | Pending |
| SFDF-04 | Phase 1 | Pending |
| PLCY-01 | Phase 2 | Complete |
| PLCY-02 | Phase 2 | Pending |
| PLCY-03 | Phase 2 | Pending |
| PLCY-04 | Phase 2 | Pending |
| PLCY-05 | Phase 2 | Pending |
| PLCY-06 | Phase 2 | Pending |
| PLCY-07 | Phase 2 | Complete |
| PLCY-08 | Phase 2 | Pending |
| CTLG-02 | Phase 2 | Pending |
| CTLG-03 | Phase 2 | Pending |
| CTLG-06 | Phase 2 | Pending |
| CTLG-08 | Phase 2 | Pending |
| CTLG-09 | Phase 2 | Complete |
| EDXT-01 | Phase 3 | Pending |
| EDXT-02 | Phase 3 | Pending |
| EDXT-03 | Phase 3 | Pending |
| EDXT-04 | Phase 3 | Pending |
| EDXT-05 | Phase 3 | Pending |
| EDXT-06 | Phase 3 | Pending |
| INTG-01 | Phase 4 | Pending |
| INTG-02 | Phase 4 | Pending |
| INTG-03 | Phase 4 | Complete |
| INTG-04 | Phase 4 | Complete |
| INTG-05 | Phase 4 | Pending |
| INTG-06 | Phase 4 | Pending |
| INTG-07 | Phase 4 | Pending |
| SLNX-01 | Phase 5 | Pending |
| SLNX-02 | Phase 5 | Pending |
| SLNX-03 | Phase 5 | Pending |
| SLNX-04 | Phase 5 | Pending |
| SLNX-05 | Phase 5 | Pending |
| SLNX-06 | Phase 5 | Pending |
| SLNX-07 | Phase 5 | Pending |
| SLNX-08 | Phase 5 | Pending |
| SLNX-09 | Phase 5 | Pending |
| SLNX-10 | Phase 5 | Pending |
| LLMF-01 | Phase 6 | Complete |
| LLMF-02 | Phase 6 | Complete |
| LLMF-03 | Phase 6 | Complete |
| LLMF-04 | Phase 6 | Complete |
| LLMF-05 | Phase 6 | Complete |
| LLMF-06 | Phase 6 | Complete |
| AUDT-01 | Phase 6 | Pending |
| AUDT-02 | Phase 6 | Pending |
| AUDT-03 | Phase 6 | Pending |
| AUDT-04 | Phase 6 | Pending |
| AUDT-05 | Phase 6 | Pending |
| AUDT-06 | Phase 6 | Pending |
| AUDT-07 | Phase 6 | Pending |
| SMAC-01 | Phase 7 | Pending |
| SMAC-02 | Phase 7 | Pending |
| SMAC-03 | Phase 7 | Pending |
| SMAC-04 | Phase 7 | Pending |
| SWIN-01 | Phase 7 | Pending |
| SWIN-02 | Phase 7 | Pending |
| SWIN-03 | Phase 7 | Pending |
| SWIN-04 | Phase 7 | Complete |
| SWIN-05 | Phase 7 | Pending |
| SWIN-06 | Phase 7 | Pending |
| SFDF-05 | Phase 7 | Pending |
| TUI-01 | Phase 8 | Pending |
| TUI-02 | Phase 8 | Pending |
| TUI-03 | Phase 8 | Pending |
| TUI-04 | Phase 8 | Pending |
| TUI-05 | Phase 8 | Pending |
| TUI-06 | Phase 8 | Pending |
| TUI-07 | Phase 8 | Pending |
| TUI-08 | Phase 8 | Pending |
| TUI-09 | Phase 8 | Pending |
| TUI-10 | Phase 8 | Pending |
| CODE-01 | Phase 9 | Complete |
| CODE-02 | Phase 9 | Complete |
| CODE-03 | Phase 9 | Complete |
| CODE-04 | Phase 9 | Complete |
| CODE-05 | Phase 9 | Complete |
| CODE-06 | Phase 9 | Complete |
| SFDF-06 | Phase 9 | Complete |
| CTLG-04 | Phase 9 | Complete |

**Coverage:**
- v1 requirements: 89 total
- Mapped to phases: 89/89
- Unmapped: 0

---
*Requirements defined: 2026-05-26*
*Last updated: 2026-05-26 after roadmap creation (9 phases)*
