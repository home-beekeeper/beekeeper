# Beekeeper

**A safety harness for autonomous coding agents.**

Version 0.1 PRD. Mfanafuthi Mhlanga / Mzansi Agentive Pty Ltd. May 2026.

---

## 1. Problem

Autonomous coding agents like Claude Code, Cursor, OpenCode, Codex CLI, Continue, and OpenClaw run with the full privileges of the developer who launched them. They execute shell commands, install packages, modify files, fetch URLs, and call MCP tools as fluently as a human at the terminal. The blast radius of a compromised agent run is the blast radius of the developer's machine.

The threats are no longer hypothetical. In May 2026 alone:

- Nx Console VS Code extension was trojanized for an 18-minute window on the Marketplace. A GitHub employee installed it. The attacker exfiltrated ~3,800 GitHub-internal repositories. The payload stole credentials from 1Password vaults, Claude Code configurations, npm tokens, GitHub tokens, and AWS credentials.
- TeamPCP, the threat actor behind the Nx attack, also compromised Checkmarx, Trivy, SAP, TanStack, and Bitwarden via the same supply chain pattern.
- The Mini Shai-Hulud worm infected 324 packages across npm and PyPI with 643 malicious versions.
- The AntV worm wave compromised additional ecosystems.
- Laravel Lang's Composer/Packagist packages were poisoned across `lang`, `http-statuses`, `attributes`, and `actions`.
- A `node-ipc` credential stealer hit 7 malicious versions.
- A typosquat of `github.com/shopsprint/decimal` shipped a DNS TXT backdoor in Go modules.
- GemStuffer exfiltrated 155 versions across 123 RubyGems targeting UK local government.
- TrapDoor hit npm, PyPI, and Cargo simultaneously with 378 malicious versions.

Existing defenses do not cover the agent runtime case:

- **Bumblebee** detects on-disk exposure after the fact. Read-only. No runtime enforcement. macOS/Linux only as of v0.1.1.
- **Socket Firewall Free** wraps human-invoked package manager commands. Not designed for agent-initiated tool calls.
- **pnpm v11** ships `minimumReleaseAge: 1440` and `blockExoticSubdeps: true` on by default, but only applies when the agent uses pnpm specifically.
- **LlamaFirewall** detects prompt injection and agent misalignment. Does not address supply chain.
- **ContextForge, MCPGuard, MCPX** centralize MCP traffic for governance. Do not include threat intelligence matching.
- **OSV-Scanner** scans lockfiles against the OSV database. Inventory, not runtime enforcement.
- **Snyk, Dependabot** are CVE-based. Lag behind supply chain compromises by hours to days.

No tool unifies threat intelligence matching, runtime enforcement at the tool-call layer, prompt injection defense, and inventory orchestration for the autonomous agent use case. That is the gap Beekeeper fills.

## 2. Positioning

Beekeeper is the safety harness that turns existing autonomous coding agents into evaluator-generator pairs. It mediates tool calls, package installs, file access, and network egress against threat intelligence from Bumblebee, OSV, Socket's public catalog, and LlamaFirewall. A hijacked or off-task agent cannot successfully act on the user's machine without Beekeeper deciding to permit it.

The framing is deliberately the GSD pattern. The agent is the generator. Beekeeper is the evaluator. Policies are inert specifications. Catalogs are evidence. The harness has no opinions about what the agent should do, only opinions about what it must not.

## 3. Scope and non-goals

### 3.1 In scope for v1

- Real-time interception of tool calls from Claude Code, Cursor, Codex CLI, Continue, OpenCode, OpenClaw, and any MCP-speaking agent.
- Package-install policy matching against multiple catalog sources (Bumblebee threat_intel, OSV, Socket public API).
- Release-age policy applied to npm, PyPI, Cargo, RubyGems, Composer, Go modules.
- Lifecycle script policy (allowlist-only execution).
- Sensitive path and file access rules.
- Network egress rules per tool call.
- Prompt injection detection via LlamaFirewall sidecar.
- Output filtering for credentials and sensitive data.
- Behavioral baseline per project.
- Multi-agent observability with parent-child policy inheritance.
- Sentry process correlation engine: agent-independent detection of credential exfiltration patterns via fanotify on Linux, eslogger on macOS, ETW on Windows.
- Sensitive path access detection from any process (not just agents), with cross-correlation against Bumblebee inventory.
- Bumblebee orchestration: scheduled scans, auto-trigger on new catalog drops.
- Catalog auto-sync from upstream sources.
- Distributed audit logging via syslog, OpenTelemetry, file sinks.
- Policy as code (declarative JSON, version-controlled, testable).
- TUI dashboard for live observability and audit review.
- Protected-mode elevation install (`beekeeper protect install`) for kernel-touching features.
- Native cross-platform support: macOS, Linux, Windows from v1.0.

### 3.2 Out of scope for v1

- True kernel-mode syscall blocking. v1 Sentry detects and alerts; blocking syscalls before they complete requires deeper kernel work. v3.
- Sandbox / microVM orchestration. v3.
- Local LLM-based tool-call anomaly classifier. v3.
- General-purpose Falco-equivalent rule engine. v1 ships specific, narrow rules targeted at the threat classes Beekeeper exists for. Users can add their own JSON rules; Beekeeper is not trying to be a generic rule engine.
- Desktop GUI or web UI. TUI only.
- Replacement for EDR, antivirus, or network firewalls. Beekeeper is a complement, not a substitute.
- Custom threat intelligence research. Beekeeper consumes upstream catalogs and ships a small default ruleset only.

## 4. Architecture

Single Go binary, multiple operational modes selected by subcommand, with optional Python and TypeScript sidecars for specialized work.

### 4.1 Components

**Beekeeper CLI** (Go, primary surface). One static binary. Manages configuration, runs scans, queries the audit log, tests policies, installs hooks. The control plane.

**Hook handler mode** (Go, same binary). When invoked as `beekeeper check`, reads tool call JSON from stdin, runs the policy engine, exits with allow or block. Sub-100ms target latency for the critical path of every agent tool call.

**Gateway daemon** (Go, same binary). When invoked as `beekeeper gateway`, runs as a long-lived MCP proxy. Accepts connections from agents, forwards tool calls to upstream MCP servers, applies policy in flight. Managed as a user-level service via launchd, systemd, or Windows Service Manager.

**Sentry daemon** (Go, same binary, requires elevation). When invoked as `beekeeper sentry` after `beekeeper protect install`, ingests three OS-native event streams: process events (creation, exec, parent PID, descendant tree), file access events on the configured sensitive-path watchlist, and outbound network connection initiations with process attribution. Runs the process correlation engine: a small set of narrow rules tuned for the Bumblebee-correlated case (recent-extension behavior fusion). Implementation: fanotify + eBPF on Linux, eslogger on macOS, ETW on Windows. Detects credential exfiltration patterns regardless of whether any agent is in the loop, closing the gap where the threat originates from a malicious extension, package postinstall, or compromised dependency acting on its own.

**Scan orchestrator** (Go, same binary). When invoked as `beekeeper scan`, invokes Bumblebee, applies Beekeeper-specific rules on top of Bumblebee's findings, merges output into a unified NDJSON stream.

**Catalog sync daemon** (Go, same binary, optional). When invoked as `beekeeper catalogs watch`, polls upstream threat intel repositories on a configurable interval, detects new catalogs, triggers a deep scan when newly-cataloged packages are present on the machine.

**LlamaFirewall sidecar** (Python, optional). Manages PromptGuard 2 and CodeShield. Communicates with Beekeeper over a local Unix domain socket or Windows named pipe using length-prefixed JSON. Beekeeper supervises the process lifecycle.

**TypeScript hook scaffolds** (v1.5 deliverable). Reference implementations of Claude Code, Cursor, and Codex hook handlers that defer to `beekeeper check`. Distributed via npm for users who prefer the TypeScript surface.

**State directory.** `~/.beekeeper/` on Unix, `%APPDATA%\beekeeper\` on Windows.

```
.beekeeper/
├── config.json           # User-level config
├── catalogs/             # Cached threat intel JSONs
├── policies/             # Active policy files
├── audit/                # NDJSON audit log, rotated
├── baselines/            # Per-project behavioral counters
├── llamafirewall/        # Sidecar models and cache (if enabled)
└── state.json            # Runtime state for daemons
```

### 4.2 Data flow

```
Agent (Claude Code, Cursor, ...)
    │
    │ tool call (PreToolUse hook OR MCP gateway)
    ▼
Beekeeper policy engine
    │
    ├──► Catalog matcher (Bumblebee threat_intel + OSV + Socket)
    ├──► Release-age policy
    ├──► Lifecycle script policy
    ├──► Sensitive path / egress rules
    ├──► Behavioral baseline check
    ├──► LlamaFirewall sidecar (if enabled, for relevant inputs)
    │
    ▼
Decision: allow | warn | block
    │
    ├──► NDJSON audit record (always)
    ├──► Structured reason returned to agent (block / warn)
    │
    ▼
Real MCP server / shell / tool (if allowed)
```

## 5. Policy engine

### 5.1 Catalog matching

Inputs:

- **Bumblebee `threat_intel/`**: primary, auto-synced from upstream.
- **OSV database**: synced via OSV-Scanner's offline DB mechanism, refreshed daily.
- **Socket public API**: queried for any package install not matched by the local cache.
- **User-provided catalogs**: JSON files in the same schema, dropped in `policies/`.
- **`beekeeper-self` catalog**: a special source listing known-compromised Beekeeper releases. See Section 12.

**Match semantics: corroboration-based, not union-of-bad.** Catalog source diversity is the 2FA principle applied to threat intelligence. Any single catalog source is a single factor; a compromised source can push bad entries on its own. Multiple independent sources agreeing on a match is a much stronger signal that can't be forged without compromising several vendors at once.

Default thresholds (configurable per ecosystem and per severity):

- **Single-source match → notify only.** Surface in the TUI, write to audit log, return a warning to the agent that includes the source. Do not enforce.
- **Two-source agreement → enforce.** Block the install, return a structured block to the agent, audit-log the decision with all corroborating sources.
- **Three-source agreement → enforce + quarantine recommendation.** If the package is already installed, surface a high-confidence quarantine action in the TUI.

Users can lower the threshold (more aggressive enforcement) for trusted environments or raise it (more conservative) when they want to avoid disruption from false positives. The threshold is per-ecosystem; users can enforce more aggressively on editor extensions where the blast radius is high and more conservatively on Go modules where false positives have higher cost.

**Catalog records include severity, source attribution, and evidence. All matches are NDJSON-logged with full provenance**, including which catalogs corroborated and which dissented. Forensics can always trace which source caused which decision.

Schema is Bumblebee's exposure catalog format extended with optional fields:

```json
{
  "id": "advisory-2026-XXXX",
  "name": "...",
  "ecosystem": "npm | pypi | go | rubygems | packagist | cargo | editor-extension | browser-extension | mcp",
  "package": "...",
  "versions": ["..."],
  "severity": "critical | high | medium | low",
  "source_url": "https://...",
  "catalog_signature": "...",
  "catalog_source": "bumblebee | osv | socket | user | beekeeper-self"
}
```

### 5.2 Release-age policy

For any package install detected on a covered ecosystem, Beekeeper queries the registry for publish timestamp and compares against the configured minimum age. Default: 24 hours, matching pnpm v11. Configurable per-ecosystem and per-package via allowlist (e.g., `minimumReleaseAgeExclude` style).

This rule applies regardless of which package manager the agent invokes. The agent calling `npm install foo` gets the same release-age policy that pnpm v11 enforces natively for `pnpm install foo`. Closes the npm gap.

### 5.3 Lifecycle script policy

Allowlist-only. Maintained in `policies/lifecycle.json`. Default deny for `preinstall`, `postinstall`, `install` scripts. Mirrors pnpm v11's `allowBuilds` model and applies it across npm, pip, Cargo, RubyGems, Composer.

For agents that legitimately need a build step (native modules, etc.), the agent receives a structured warning explaining the block and a recommendation to add the package to the allowlist explicitly.

### 5.4 Editor extension policy

Editor extensions (VS Code, Cursor, Windsurf, OpenVSX) are a first-class threat surface. The Nx Console compromise of May 2026 is the canonical example: trojanized extension live on the Marketplace for 18 minutes, single GitHub employee infected, ~3,800 GitHub-internal repositories exfiltrated. Beekeeper covers this surface across three enforcement layers.

**Layer 1: Agent-initiated CLI installs.** The hook handler recognizes editor extension install commands and routes them through the catalog matcher and release-age policy:

- `code --install-extension <id>[@<version>]`
- `code-insiders --install-extension ...`
- `cursor --install-extension ...`
- `windsurf --install-extension ...`
- Bulk forms with multiple `--install-extension` flags

Matched against editor-extension catalogs (`nx-console-vscode-2026-05-18.json` and equivalents). Release-age policy applies: by default, extensions less than 24 hours old at publish time are blocked unless the publisher is allowlisted.

**Layer 2: File-watcher daemon.** GUI installs and auto-updates never invoke a shell command, so hook-based defense cannot see them. Beekeeper runs a file-watcher daemon (`beekeeper watch`, or a feature of the catalog sync daemon) using OS-native filesystem notifications (inotify on Linux, FSEvents on macOS, ReadDirectoryChangesW on Windows, via the `fsnotify` Go library) over the extension directories:

- VS Code: `~/.vscode/extensions/` and platform equivalents
- Cursor: `~/.cursor/extensions/`
- Windsurf: `~/.windsurf/extensions/`
- OpenVSX: applicable per-editor paths

On any new directory appearance or `package.json` version change:

- Parse the extension manifest to extract `publisher.name` and `version`
- Match against the editor-extension catalog
- On hit: emit a critical audit record, surface a desktop notification (configurable), optionally quarantine the directory by moving it to `~/.beekeeper/quarantine/extensions/` and invoke the editor's removal CLI
- On miss: query the Marketplace publish timestamp; if the version is younger than the release-age threshold, quarantine and notify (this implements `minimumReleaseAge` semantics for extensions, which neither VS Code nor Cursor support natively as of this writing)
- All decisions contribute to the per-project behavioral baseline

**Layer 3: Inventory scans.** Already-installed compromised extensions are surfaced by scheduled Bumblebee scans, which already enumerate extension directories and match against the same catalogs. The scan orchestrator (`beekeeper scan`) makes this routine; the catalog sync daemon triggers an immediate scan when new editor-extension catalogs land upstream, closing the loop on Marketplace-pulled extensions that remain on disk.

**Editor-side configuration.** On first run, `beekeeper init` detects installed editors and offers to:

- Disable auto-update of extensions in the editor's settings (Beekeeper writes the setting on consent)
- Enable the file-watcher for detected extension directories
- Set a release-age threshold for new extensions

These are recommendations the user accepts or declines; Beekeeper does not modify editor settings without explicit consent.

### 5.5 Sentry process correlation engine

Hook-based and gateway-based defense only sees what passes through the agent loop. The Nx Console class of attack happens entirely outside that loop: a malicious extension activates on editor startup, reads credentials, and exfiltrates over HTTPS without any agent being involved. Sentry is the layer that addresses this gap.

Sentry runs as a privileged daemon (installed via `beekeeper protect install`, see Section 8.4) and ingests three OS-native event streams:

1. **Process events.** Process creation with full executable path, PID, parent PID, command line. Lets Sentry reconstruct the process descendant tree from any editor or shell.
2. **File access events.** Reads of paths in the configured sensitive-path watchlist (default: credential paths, MCP configs, browser cookie databases, SSH keys). Carries the accessing process identity.
3. **Outbound network connection events.** Connection initiation with 5-tuple and process attribution where the OS exposes it.

Implementation per platform: fanotify (file events) + eBPF (process + network) on Linux; eslogger on macOS (which streams EndpointSecurity events to processes running as root without requiring the entitlement); ETW with the relevant security providers on Windows.

Sentry ships with a small set of narrow correlation rules. Not a general-purpose rule engine. The discipline is: each rule targets a specific known attack pattern with low false-positive surface, made possible by fusing Sentry's stream with Bumblebee's inventory.

**Default rules in v1:**

- *Extension-host credential cluster.* A process descended from a Code, Cursor, Windsurf, or Codium process reads two or more files matching the sensitive-path catalog within a 60-second window. Severity: critical.
- *Extension-host credential CLI burst.* A process descended from the editor spawns two or more known credential CLIs (`gh auth`, `aws configure`, `op signin`, `vault token`, `npm whoami`, `gcloud auth`) within a 60-second window. Severity: critical.
- *Extension-host phone-home.* A process descended from the editor initiates an outbound connection to a domain not in the configured allowlist within 10 minutes of extension activation. Severity: high.
- *Fresh-extension behavior correlation.* Any of the above fires AND a Bumblebee-tracked extension was installed or auto-updated within the last 30 minutes. Severity: critical, with cross-referenced extension identity and Marketplace publisher in the audit record.
- *Exfil signature fusion.* Sensitive path read + outbound connection initiation + same process descendant of a recently-installed editor extension, within a 5-minute window. This is the Nx Console signature exactly. Severity: critical with recommended quarantine action.

Each rule emits a structured NDJSON audit record with full provenance: which process, which parent chain, which files were read, which network destinations were contacted, which Bumblebee inventory entries are correlated.

**What Sentry does, what it does not.** Sentry detects and alerts. It does not block syscalls in flight. True prevention via kernel-mode mediation is v3 work. The honest framing: Sentry shortens the time from compromise to detection from "hours to days" (waiting for a Bumblebee catalog entry to land) to "seconds to minutes" (the rules fire as the exfiltration pattern executes). That window matters because credentials become invalid as they get rotated; reducing detection time directly reduces blast radius.

**Tuning and false positives.** Conservative defaults. Each rule has a configurable severity threshold for whether it triggers a notification, only an audit record, or active quarantine. Beekeeper ships a "baseline mode" for the first 7 days after install that downgrades all Sentry rules to audit-only while it learns the user's baseline. After baseline, rules promote to their configured severities. Users can extend the baseline period or stay in audit-only mode indefinitely.

### 5.6 Sensitive path policy

Default blocklist:

- `~/.ssh/`
- `~/.aws/`
- `~/.gnupg/`
- `~/.config/Claude/` and all MCP config files Bumblebee enumerates
- `~/.config/op/` (1Password CLI)
- `~/.config/gh/` (GitHub CLI)
- `~/.netrc`, `~/.npmrc`, `~/.pypirc`, `~/.cargo/credentials.toml`
- `.env`, `.env.local`, `.env.*` files anywhere
- User-supplied additional paths

Agents attempting reads of blocked paths receive a structured deny with a reason. The agent can be configured to surface the block in its response so the user knows it was attempted.

### 5.7 Network egress policy

Per-tool egress allowlists. Default allow for common documentation domains (docs.anthropic.com, official package registries). Default deny for paste sites, generic webhooks, telemetry endpoints with no provenance.

Outbound size limits per tool call. Multi-turn exfiltration detection via rolling entropy of recent outputs to user and base64 detection across turns.

### 5.8 Behavioral baseline

Per-project frequency counters keyed by (tool, target_pattern). Tracked over a rolling window. Sudden deviations from baseline trigger warn-level decisions even if no other rule fires. Threshold configurable.

Designed as rule augmentation, not ML. No model training. Counter-based with explicit thresholds documented in the policy file.

### 5.9 Prompt injection layer (LlamaFirewall integration)

Optional sidecar. When enabled:

- **PromptGuard 2** (86M BERT, real-time-capable) scans tool outputs flowing back into agent context (WebFetch results, file reads, MCP tool responses). Detected injection attempts return a redacted payload to the agent with a structured warning.
- **CodeShield** runs on agent-generated file writes containing code. Insecure patterns get flagged or blocked per policy.
- **AlignmentCheck** (experimental, optional) inspects chain-of-thought for goal hijacking signals.

Latency budget: PromptGuard 2 at sub-100ms p95 on the 86M variant per Meta's published benchmarks. Beekeeper surfaces actual measured latency in `beekeeper diag`.

## 6. Integration surfaces

### 6.1 Claude Code hooks

`beekeeper hooks install --target claude-code` writes the appropriate entries to `~/.claude/settings.json`:

```json
{
  "hooks": {
    "PreToolUse": [
      {"matcher": ".*", "hooks": [{"type": "command", "command": "beekeeper check"}]}
    ],
    "PostToolUse": [
      {"matcher": ".*", "hooks": [{"type": "command", "command": "beekeeper audit-record"}]}
    ],
    "UserPromptSubmit": [],
    "SessionStart": []
  }
}
```

### 6.2 MCP gateway

For Cursor, Codex CLI, Continue, OpenCode, and any MCP-speaking client, the user configures their MCP client to point at `beekeeper gateway` instead of directly at their MCP servers. Beekeeper proxies, applies policy, audits.

When ContextForge or MCPGuard is already deployed, Beekeeper runs in policy-plugin mode (v1.5 deliverable), exposing its policy engine through the host gateway's plugin interface rather than running a parallel proxy.

### 6.3 Shim layer (opt-in)

`beekeeper shim install` symlinks wrapper binaries earlier in PATH for npm, pnpm, pip, cargo, go, gem, composer, npx, pipx. Each shim invokes `beekeeper check` with the parsed command, then proxies to the real binary if allowed.

Covers the case where an agent shells out to a package manager via Bash without going through a tool call Beekeeper can intercept at the MCP layer.

## 7. Output and observability

### 7.1 NDJSON audit log

Bumblebee-schema-compatible. Every policy decision emits one record. Fields include `record_type`, `record_id`, `scanner_name: "beekeeper"`, `scanner_version`, `agent_name`, `tool_name`, `decision`, `reason`, `rule_ids`, `catalog_matches`, `endpoint` block matching Bumblebee's.

Records flow to:

- Local file (`audit/beekeeper.ndjson`, rotated)
- Optional syslog (RFC 5424)
- Optional OpenTelemetry exporter (OTLP)
- Optional HTTPS POST sink (matches Bumblebee's transport options)

### 7.2 TUI dashboard

`beekeeper dashboard` opens a terminal interface for live observability and audit review. Single screen, sshable, no browser required. Built with Bubble Tea (Go TUI framework). Read-only by default; with `--admin` flag, supports policy toggling and quarantine actions without leaving the TUI.

**Panels:**

- **Live activity feed.** Tool calls flowing through hooks and the gateway, scrolling in real-time with decision indicator (allow / warn / block), agent identity, tool name, target. Filterable.
- **Sentry alerts.** Recent process correlation events from the Sentry daemon, severity-color-coded, with expandable detail showing the full process tree, file accesses, and network destinations.
- **Catalog freshness.** Per-source: last successful sync, delta count since last sync, next scheduled sync. Red indicator if any source is stale beyond the configured threshold.
- **Scan status.** Last Bumblebee scan timestamp, findings count, next scheduled scan, and a one-key trigger for an immediate scan.
- **Active policies.** List of loaded policy files with rule counts. Drill-down shows individual rules and their enabled/disabled state.
- **Quarantine.** Items currently in `~/.beekeeper/quarantine/`, with restore and purge actions.
- **System health.** Sentry daemon status and CPU/memory usage, Gateway daemon status, LlamaFirewall sidecar status and inference latency.

**Why TUI ships in v1.** The TUI is what makes the system legible. A defender running Beekeeper needs to be able to see what's happening, what was blocked, what was missed, what's stale. NDJSON in a file works for machines; a TUI works for humans. Shipping v1 without it leaves the project hard to evaluate at a glance, which hurts both adoption and the portfolio dimension.

**Implementation notes.** Bubble Tea is mature, single-binary-friendly (no CGO requirement for the basics), and produces accessible output. Refresh cadence is event-driven (file watcher on the audit log) with a 1-second polling fallback. The TUI reads from the same audit log NDJSON that other consumers read; it has no privileged channel into the daemons.

## 8. Runtime model

Beekeeper is a multi-process system with six independent cadences. Understanding the runtime shape is essential because the system runs persistently in the background, and resource cost matters for solo developers on battery-powered machines.

### 8.1 Cadences

| Cadence | Component | Trigger | Cost per event |
|---|---|---|---|
| ~100ms per call | Hook handler (`beekeeper check`) | Agent tool call | Fresh Go process: ~10-30ms cold start + ~microseconds policy eval |
| Continuous | Sentry daemon | Kernel event stream | Per-event filter check ~microseconds; rule eval on watchlist hits |
| Continuous | Gateway daemon | MCP traffic from agents | Per-call proxy overhead ~milliseconds |
| Continuous | Extension watcher | Filesystem events on editor dirs | Per-event ~microseconds, manifest parse on hit ~milliseconds |
| Hourly | Catalog sync daemon | Configurable timer | HTTP fetch + JSON parse, ~1 second per source |
| Daily | Bumblebee scan | Cron + new-catalog trigger | Full deep scan ~seconds to tens of seconds |
| On demand | CLI and TUI | User invocation | Varies per command |

The catalog sync daemon has a special property: when it detects new entries in upstream `threat_intel/`, it triggers an immediate Bumblebee scan against the newly-cataloged ecosystems. This is the killer feature of the continuous architecture. A new attack campaign published upstream at 3am results in the user's machine being scanned by 4am without manual action. Detection latency for novel threats collapses from "hours after the user notices" to "minutes after the catalog publishes."

### 8.2 Resource envelopes

Honest estimates based on comparable tools (Falco, Tetragon for Sentry; Bumblebee selftest for scan cost; published LlamaFirewall benchmarks for the sidecar). Real numbers require prototyping; these are the targets.

**Hook handler:** Each invocation is a fresh process, ~10-30ms cold start dominant. Memory: ~10-20MB resident, exits immediately. Cumulative cost over a heavy session (50-200 calls/hour): negligible. Comparable to running `git status` repeatedly.

**Sentry daemon:** The expensive component. Target steady-state: 0.5-3% of one core, 200MB resident. Transient spikes to 5-10% of a core during file-event-heavy operations (`npm install` generates thousands of events). For comparison, Falco at default config uses 1-5% of a core on a dev laptop; Sentry should be cheaper because its event filtering is narrower (specific watchlists, not system-wide).

**Catalog sync:** CPU spike of <1 second hourly. Negligible. Network: ~5-20MB/day across all enabled catalog sources.

**Bumblebee scan:** Spikes one core to high utilization for seconds to tens of seconds, then exits. Memory ~50-100MB peak. Negligible cumulative daily cost.

**Gateway daemon:** Mostly idle. Memory ~50MB resident plus per-connection overhead. CPU when idle: rounds to zero. Active: low single-digit percent during tool-call bursts.

**LlamaFirewall sidecar (optional):** Memory ~400-800MB to hold PromptGuard 2 (86M parameters) in RAM. CPU per inference ~20-50ms on CPU, faster on GPU if available. Fires only on relevant inputs (tool outputs containing potentially-injectable text). Cumulative cost bounded by tool call rate.

**24-hour totals for a fully-armed Beekeeper:**

- Average CPU: 1-3% of one core
- Peak CPU during active dev work: 5-10% of one core, transient
- Steady-state memory: 300-500MB total
- Daily disk I/O: ~10-50MB (audit log writes, catalog refreshes)

For context, Slack desktop uses 500MB-1GB of memory and 2-5% CPU continuously while idle. Beekeeper fully deployed is meaningfully lighter than Slack.

### 8.3 Cadence configuration

Every cadence is user-tunable. Realistic config knobs:

- **Bumblebee scan schedule.** Cron expression in config. Default `0 3 * * *` (3am daily). Configurable from `@hourly` to disabled. Auto-trigger on new catalog drops is a separate toggle (default on).
- **Catalog sync interval.** Minutes. Default 60. Range 5-1440.
- **Sentry rules.** Each rule enableable/disableable individually with per-rule severity threshold for notifications.
- **Sentry baseline period.** Days during which all Sentry rules are audit-only while the system learns. Default 7. Settable to 0 (immediate enforcement) or "indefinite" (always audit-only).
- **LlamaFirewall enablement and sampling.** Enable/disable, sample rate (1.0 = scan every tool output, lower = sample). For resource-constrained machines.
- **Resource limits.** Soft CPU and memory caps on Sentry. On cap exceedance, Sentry logs a warning and sheds load by reducing rule complexity or sampling events. Configurable thresholds for what "exceedance" means.
- **Battery awareness.** On laptops, optionally suspend Sentry's heaviest rules when on battery and the user is idle. Off by default; opt-in for users who want maximum battery life.
- **Quiet hours.** Time windows where desktop notifications are suppressed. Audit log still records; only UI notifications are muted.
- **Failure modes per component.** `fail_closed` (default for hook in enforce mode), `fail_open`, `fail_warn`. Configurable separately for hook handler, gateway, Sentry, LlamaFirewall.

### 8.4 Elevation strategy

Beekeeper splits cleanly into unprivileged and privileged components. The unprivileged tier covers most of the value; the privileged tier adds Sentry's process correlation and reverse-direction credential watching.

**Unprivileged install** (default). User-level binary installed via `go install` or `curl | sh`. Provides hook-based protection, gateway, extension directory watcher, catalog sync, Bumblebee orchestration, TUI, all CLI commands. No admin or root required. This is what every user gets out of the box.

**Protected-mode install** (`beekeeper protect install`, opt-in). Installs the Sentry daemon as a privileged service (systemd unit on Linux, launchd plist on macOS, Windows Service on Windows). Requires admin/root at install time. Once installed, the daemon runs in the background and the unprivileged CLI communicates with it via a Unix domain socket or Windows named pipe.

**Why opt-in.** Beekeeper is a new open-source security tool with no commercial backer. Requesting admin elevation as part of the default install is a trust ask before the user knows the project. Making protected mode explicit and opt-in respects user agency and lets people evaluate the unprivileged tier before granting deeper access.

**Platform-specific elevation reality:**

- *Linux.* fanotify needs `CAP_SYS_ADMIN`; eBPF programs need `CAP_BPF` (kernel 5.8+) or `CAP_SYS_ADMIN` (older). Reading other users' `/proc/<pid>/` needs root. The privileged daemon runs as root via systemd; the unprivileged CLI talks to it via a Unix socket with permission gating.
- *macOS.* EndpointSecurity framework requires Apple's `com.apple.developer.endpoint-security.client` entitlement, which is non-trivial for indie developers to obtain. Beekeeper v1 ships with `eslogger`-based monitoring on macOS: works, requires `sudo`, no entitlement needed, less complete than full EndpointSecurity but covers the core attack patterns. Applying for the entitlement is a v1.x improvement path.
- *Windows.* ETW providers vary by privilege requirement. The Sentry daemon installs as a Windows Service running with SYSTEM or LocalService privileges. ETW also rate-limits under load; Beekeeper surfaces this in `beekeeper diag` so users know when detection coverage is degraded.

**Beekeeper without elevation.** Honest about what users get without `beekeeper protect install`:

- Full hook interception of agent tool calls
- Full MCP gateway with policy enforcement
- Extension directory monitoring (Beekeeper can see new extensions appear)
- Daily Bumblebee scans
- Catalog sync and threat-intel-driven re-scans
- TUI and audit log

What users do *not* get without elevation: real-time detection of attacks that originate outside the agent loop (Nx Console-class compromises that activate inside the editor extension host and never touch an agent). This is the genuine value of opting in.

### 8.5 Audit log as integration point

The NDJSON audit log is the contract between Beekeeper components and external consumers. Everything writes to it; the TUI, exports, alerts, and any future tooling read from it. Schema-compatible with Bumblebee's record format.

This matters architecturally because Beekeeper is not trying to be a dashboard provider or a SIEM. It produces a stream of structured events. Other tools can be built against that stream. The TUI is one consumer; users can write their own consumers in any language. The same NDJSON file flows to optional remote sinks (syslog, OpenTelemetry OTLP, HTTPS POST endpoint) without component coupling.

The GSD pattern again: events are inert structured records that any evaluator can consume. No tight coupling between producer and consumer. Future Beekeeper versions can add new event types without breaking existing consumers, and existing consumers continue to work without modification.

## 9. Configuration

Layered JSON config, merged in order:

1. `/etc/beekeeper/config.json` (system, optional)
2. `~/.beekeeper/config.json` (user)
3. `<project>/.beekeeper/config.json` (project, when present)
4. Environment variables (`BEEKEEPER_*` overrides)
5. CLI flags

Policy files are separate from config. Policy files live in `policies/` and are loaded by the engine; config governs how Beekeeper itself runs.

Example minimal user config:

```json
{
  "catalogs": {
    "bumblebee_threat_intel": {"enabled": true, "sync_interval": "1h"},
    "osv": {"enabled": true, "offline_db": true},
    "socket_public": {"enabled": true}
  },
  "release_age": {"default_minutes": 1440},
  "lifecycle_scripts": {"policy": "deny_by_default"},
  "llamafirewall": {"enabled": false},
  "audit": {"sinks": ["file", "syslog"]}
}
```

## 10. CLI surface

```
beekeeper init
beekeeper hooks install --target {claude-code|cursor|codex|continue|opencode|openclaw}
beekeeper gateway [--port N]
beekeeper shim install
beekeeper shim uninstall

beekeeper protect install        # privileged sentry daemon install
beekeeper protect uninstall
beekeeper protect status
beekeeper sentry rules list
beekeeper sentry rules enable <id>
beekeeper sentry rules disable <id>

beekeeper check                  # one-shot policy eval, reads tool call from stdin
beekeeper scan [--deep]          # invoke bumblebee + apply beekeeper rules
beekeeper catalogs sync
beekeeper catalogs diff
beekeeper catalogs watch         # background sync daemon
beekeeper watch                  # filesystem watcher for editor extensions

beekeeper quarantine list
beekeeper quarantine restore <id>
beekeeper quarantine purge

beekeeper policy test <file>     # dry-run policy against sample tool call
beekeeper policy validate <file>
beekeeper policy list

beekeeper audit query [--since|--agent|--tool|--decision]
beekeeper audit tail
beekeeper audit export --format {ndjson|csv|otlp}

beekeeper dashboard              # TUI

beekeeper diag                   # latency, sidecar status, catalog freshness
beekeeper version
beekeeper selftest               # embedded fixture test
```

## 11. Threat model

### 11.1 What Beekeeper defends against

- Compromised package installs (typosquats, hijacked legitimate packages) via npm, PyPI, Cargo, RubyGems, Composer, Go modules, matched against unified catalogs.
- Compromised editor extensions (VS Code, Cursor, Windsurf, OpenVSX) detected via three layers: agent-initiated CLI installs blocked by the policy engine; GUI and auto-update installs detected by the file-watcher daemon with catalog matching at install time; already-installed compromised extensions surfaced via scheduled Bumblebee scans.
- Hijacked agents acting on prompt injection from tool outputs.
- Off-task agents reading sensitive files outside the task scope.
- Credential exfiltration via tool calls (file read, network egress, output stream).
- Lifecycle script execution from untrusted packages.
- Subagent escalation (child agent attempting operations the parent could not).
- Credential exfiltration originating outside any agent loop (malicious extension activating in editor host, compromised package postinstall, hijacked dependency acting independently). Detected via Sentry process correlation when protected mode is installed.
- Reduced time-to-detection for novel supply chain compromises through catalog-delta-triggered scans: when an upstream `threat_intel/` catalog publishes a new campaign, Beekeeper sweeps the local machine within the sync interval (default 1 hour) without manual action.

### 11.2 What Beekeeper does not defend against

- Pre-existing malware on the machine before Beekeeper was installed. Bumblebee scan catches known-bad packages; Beekeeper does not do general malware detection.
- Kernel-level rootkits. Out of scope for user-space mediation. v3 territory.
- Direct human malice. If the developer types `rm -rf ~/` themselves, Beekeeper is not involved. Beekeeper protects against compromised agents, not deliberate user action.
- Adversarial Beekeeper bypass via shim PATH manipulation. Shims are opt-in defense in depth, not the primary surface. Hook and gateway layers are the real enforcement points.
- Sophisticated prompt injections that route around PromptGuard 2. LlamaFirewall reduces attack success rate by ~90%, not 100%. Defense in depth still required.
- Extension code execution between install and detection. The file watcher fires when the extension directory appears, but the editor may load and activate the extension before Beekeeper can quarantine it. The race window is small but real. True prevention requires hooking the editor's extension loader, which is out of scope and would require editor-vendor cooperation. For high-security environments, disable editor auto-update and rely on agent-mediated CLI installs only.
- Agent-independent attacks when running unprivileged. Without `beekeeper protect install`, Sentry is not available and attacks that originate outside the agent loop are only detected by the daily Bumblebee scan (i.e. after the catalog entry lands upstream). Users on machines where elevation is not acceptable should understand this gap.

### 11.3 Beekeeper compromise scenarios

If Beekeeper itself is compromised, the agent has the developer's privileges directly. Mitigations:

- Single static binary, distributable via `go install` with module checksum verification.
- Policy files are version-controlled JSON, not code. Adversary cannot smuggle execution via a policy.
- Catalog files are JSON. Same protection.
- LlamaFirewall sidecar is supervised; if it dies or hangs, Beekeeper fails closed on prompt injection checks (configurable to fail open with a warning).
- Audit log is append-only NDJSON with optional remote sink. Local tampering is detectable; remote-sunk records survive local tampering.

## 12. Self-defense and supply chain integrity

The previous section addressed the threats Beekeeper defends *the user* against. This section addresses the threats Beekeeper itself poses to the user and how the project mitigates them.

This is the question most security tools fail at. The supply chain compromises Beekeeper exists to detect happened because security vendors became attack vectors themselves: Checkmarx, Trivy, SAP, TanStack, and Bitwarden were all compromised by TeamPCP through the same patterns Beekeeper is built to catch. A security tool that gets compromised is worse than no security tool, because it carries elevated trust into every environment that runs it. Beekeeper plans for its own compromise from day one.

### 12.1 Threat model: how Beekeeper could become the attack

Six attack surfaces, ordered by realistic risk:

1. **Release pipeline compromise.** Highest-risk surface. Every supply chain compromise of 2025-2026 worth naming hit here. Specific failure modes: stolen GitHub Actions OIDC token used to push a malicious release; compromised maintainer credentials (the single-developer problem); compromised CI runner injecting code at build time; hijacked release artifact replacement on GitHub Releases or the `go install` proxy; backdoored third-party dependency (`fsnotify`, `bubbletea`, `cilium/ebpf`) injecting code into the build; compromised signing key allowing attackers to sign malicious binaries that pass verification.

2. **Catalog feed compromise.** Beekeeper consumes threat intel from upstream sources. Two flavors of attack: *catalog DoS* (attacker adds many false positives, generating noise that gets users to disable enforcement entirely, leaving them undefended); *catalog injection* (attacker adds entries with fields that cause Beekeeper to fetch additional rules from attacker-controlled URLs, or otherwise escalate beyond static JSON matching).

3. **Sentry daemon exploitation.** A privileged daemon with kernel access running on thousands of developer machines is an attractive target. Memory corruption bugs exploitable from event data, IPC parser bugs reachable from any local process, eBPF program bugs that the verifier doesn't catch: each is a privilege escalation primitive sitting on every protected-mode install.

4. **MCP gateway exploitation.** Long-running network-facing process. Hostile clients are explicitly in scope (by Beekeeper's own threat model, the agent talking to the gateway might be compromised). Protocol parser bugs become remote code execution primitives accessible from any local process.

5. **Hook handler exploitation.** Lower-privilege than Sentry but runs constantly. JSON parser bugs and policy engine logic bugs are exploitable from any tool call.

6. **Audit log leakage.** Captures sensitive metadata: which files agents accessed, which tools they called, command lines, fragments of tool outputs. If audit logs leak through insecure file permissions, accidental remote sink misconfiguration, or malicious read access from another local process, they're a reconnaissance goldmine for attackers planning the next stage of compromise on that machine.

### 12.2 Build and release pipeline hardening

- **Reproducible builds from day one.** Every Beekeeper binary must be reproducible from source. Same git commit produces identical binary bytes. This is the single most important mitigation: it lets anyone verify a release wasn't tampered with by building independently and comparing hashes. Go's `-trimpath -buildvcs=false` flags and `-mod=readonly` get most of the way; the rest is build environment determinism. v0.1.0 requirement.
- **Sigstore signing via GitHub Actions OIDC.** No long-lived signing keys to steal. Every release artifact signed with transparency log entries. Anyone can verify the signature came from the official workflow on the official repo. v0.1.0 requirement.
- **SLSA Level 3 provenance** as the v0.9.0 target. Every binary ships with attestation describing which workflow built it, which commit, which dependencies. Users (and Beekeeper itself in catalog matching) can verify provenance before trusting.
- **Pinned dependencies with hash verification.** `go.mod` and `go.sum` lock dependencies to specific hashes. Renovate-bot or similar handles updates with explicit human review. Third-party Go dependencies are kept to the absolute minimum justified set; each addition requires explicit documentation of why stdlib isn't sufficient.
- **Two-account release approval.** Even as a solo project initially, the GitHub repo requires approval on release PRs from a second account with separate credentials and 2FA. Until the project has co-maintainers, this is the maintainer's own second account; it's not real two-person review, but it forces the second-credential check that defeats single-credential-theft attacks. When real co-maintainers exist, the rule becomes genuine two-person review.
- **No `curl | sh` install path as the recommended option.** That pattern is convenient but exactly how supply chain attacks propagate. Recommended install paths are `go install` with proxy verification or downloading a release artifact and verifying its Sigstore signature before running. We document this prominently and accept the friction cost. A `curl | sh` script may exist for ergonomics but is not the recommended path and the docs are explicit about the trade-off.

### 12.3 Catalog feed integrity: the 2FA principle

Catalog source diversity is treated as 2FA for threat intelligence. The corroboration-based match semantics from Section 5.1 are the architectural expression of this: a single source can warn but cannot enforce. An attacker needs to compromise multiple independent vendors to push a bad enforcement rule through Beekeeper, not just one.

Reinforcing measures:

- **Catalog signatures required.** Each catalog source signs its catalogs. Beekeeper verifies signatures before applying. If an upstream source doesn't sign (Bumblebee's current state), Beekeeper either mirrors and signs locally or treats unsigned sources as warning-only regardless of corroboration. We accept the friction of "you can't add a catalog source without a signature path."
- **Catalog provenance in every audit record.** Every catalog match records which catalog, which catalog version, which signature, and which other sources corroborated or dissented. Forensics traces every decision to its evidence.
- **No remote URL fetching from catalog data.** Catalog schema is strictly static JSON. No fields that resolve to URLs Beekeeper fetches at evaluation time. If future schema needs external references, they're added explicitly with hardcoded allowlists, not derived from catalog content.
- **Sanity bounds on catalog deltas.** A catalog sync that suddenly adds an order-of-magnitude more entries than usual is suspicious. Hard limits on per-sync delta size, per-catalog rule counts, and per-package version-list lengths. Exceeding limits triggers degraded mode where new entries are warning-only until the user reviews. Defeats catalog-DoS attacks.
- **Degraded mode under uncertainty: read-and-notify, same as Bumblebee.** When catalog source integrity is uncertain (signature verification failure, source unreachable for an extended period, sanity bounds exceeded), Beekeeper degrades to read-only matching: it still detects and surfaces findings in the TUI and audit log, but does not enforce. This matches Bumblebee's posture: surface what we see, do not take destructive action under uncertainty. The user is notified prominently in the TUI that the system is degraded.

### 12.4 Daemon and IPC security

- **Memory safety wherever possible.** Go for the core eliminates the C-style memory corruption class for the main binary. The eBPF programs on Linux are the exception; they need careful review and we use the vetted `cilium/ebpf` library rather than hand-rolled BPF.
- **IPC authorization.** The Unix domain socket / Windows named pipe between the unprivileged CLI and the privileged Sentry daemon is permission-gated. Only the user who installed Beekeeper can talk to the daemon. On Linux, socket peer credential verification via `SO_PEERCRED`; on macOS, Unix permissions plus XPC entitlements where applicable; on Windows, named pipe ACLs.
- **No code execution from IPC.** The IPC protocol is strictly structured commands: list rules, dump audit, quarantine an item, reload config. No "execute this string," no "load this module," no "fetch this URL." The protocol surface is small and stays small. Any future expansion of the IPC surface requires explicit security review documented in this section.
- **Privilege separation within the Sentry daemon.** The daemon does the minimum privileged work and drops capabilities for anything that doesn't need them. eBPF loading needs root; rule evaluation does not; event filtering does not. Privilege separation by component, with `seccomp-bpf` filters on Linux limiting which syscalls the unprivileged components can make.
- **MCP gateway localhost-only by default.** Binds to `127.0.0.1`, not `0.0.0.0`. No remote access unless explicitly configured. The configuration to expose remotely requires both a flag and an explicit acknowledgment in the config file.
- **MCP gateway client authentication.** Even on localhost, the gateway requires a per-session token issued at agent setup time. Other local processes can't talk to the gateway without the token. Deviation from typical MCP server practice but appropriate for a security-sensitive proxy.
- **Strict protocol parsing.** Bounded message sizes, bounded recursion in tool definitions, bounded handshake states. Reject malformed messages aggressively rather than handling them gracefully.
- **Fuzz testing in CI.** Policy engine, IPC protocol parser, catalog parser, MCP message parser, and Sentry rule evaluator all fuzz-tested. Failures block release.
- **Hook handler fail-closed by default.** Crash or timeout → block tool call. Configurable `fail_open` mode documented as reducing security.
- **Hook handler resource limits.** Hard caps on stdin size (default 1MB), execution time (default 5s), memory. Beyond limits, fail-closed.

### 12.5 Audit log confidentiality

- **Strict file permissions.** `0600` on Unix, equivalent ACLs on Windows. Owner-only read access enforced at write time, with periodic re-verification.
- **No remote sink without explicit configuration.** Syslog, OTLP, and HTTPS sinks are off by default. Users opt in with clear awareness in the configuration UI that audit data leaves the machine.
- **Sensitive field redaction.** Audit records capture event metadata, not full payloads. Tool call arguments are recorded but with configurable redaction patterns for credential shapes (API key prefixes, JWT tokens, `Bearer ` headers). The log records "agent attempted file read at `/path`" not the contents of that file.
- **Log rotation and retention defaults.** 30-day default with rotation. Older logs auto-purged or archived to encrypted local storage. Configurable for compliance environments that require longer retention.

### 12.6 The recursive principle: Beekeeper detects Beekeeper compromise

The `beekeeper-self` catalog source is a dedicated upstream feed listing known-compromised Beekeeper releases by version and signature hash. Beekeeper consults this catalog on every startup and during every catalog sync.

If a malicious Beekeeper release ever gets signed and shipped, whether through pipeline compromise, signing key theft, or malicious maintainer action, the next `beekeeper-self` catalog sync delivers a self-quarantine signal. Beekeeper refuses to run, surfaces the warning prominently, and points the user to the verification path for downloading and verifying a known-good version.

This is the same principle as Bumblebee detecting Bumblebee-shaped problems on disk, applied recursively. It's not bulletproof: an attacker who compromises both Beekeeper *and* the `beekeeper-self` catalog source defeats it. But it raises the bar significantly. v1.0.0 requirement.

The `beekeeper-self` catalog source is hosted separately from the main Beekeeper repository and has its own signing key, its own access control, and (eventually) its own maintainer set. The intent is that compromising both surfaces simultaneously requires defeating two independent security postures.

### 12.7 Verification path for users

Users (and future maintainers, and security researchers) can audit and verify a Beekeeper release end-to-end:

1. Clone the repo at the release tag.
2. Run `make verify-release VERSION=X.Y.Z`. The Makefile target reproduces the build deterministically and compares hashes against the published release artifacts.
3. Verify the Sigstore signature against the published transparency log entry.
4. Verify the SLSA provenance attestation matches the expected GitHub Actions workflow.
5. Inspect the SBOM (CycloneDX format) published with each release for any unexpected dependencies.

This verification path is documented in `SECURITY.md` and `BUILDING.md`. Every release announcement links to it. The project maintains the position that distrust is the appropriate posture: users should not have to trust the maintainer; they should be able to verify.

### 12.8 Security disclosure process

A `SECURITY.md` file in the repo documents the responsible disclosure process: how to report vulnerabilities (private security advisory on GitHub, with a backup email contact), expected response time (initial acknowledgment within 48 hours), the project's stance on coordinated disclosure (90 days standard, negotiable for complex issues), and the public security advisory format used for confirmed issues.

The disclosure process is not optional infrastructure for a security tool. It exists before v0.1.0 and is in place for every release.

## 13. Failure modes

### 13.1 Beekeeper unavailable

If the `beekeeper check` binary fails or times out during a hook, behavior is configurable:

- `fail_closed` (default for `enforce` mode): block the tool call, agent receives a structured error.
- `fail_open`: allow the tool call, log a critical failure to the audit log.
- `fail_warn`: allow but inject a warning into the agent's response.

### 13.2 Hook latency exceeds budget

If `beekeeper check` exceeds the configured latency budget (default 500ms hard cap, 100ms p95 target), the decision is logged as a latency violation. Sustained latency violations trigger a diagnostic notification.

### 13.3 Catalog sync failure

Beekeeper continues with cached catalogs. Sync failures logged. After 7 days without successful sync, all decisions get a `stale_catalog` annotation in the audit log.

### 13.4 LlamaFirewall sidecar crash

Beekeeper supervisor restarts the sidecar up to 3 times with exponential backoff. Persistent failure: prompt injection checks fall back to configured failure mode. Agent operation continues; prompt injection layer is degraded, not blocking.

### 13.5 Sentry daemon crash or rule overload

If the Sentry daemon crashes, the supervisor restarts it with exponential backoff. During the gap, agent-independent detection is unavailable; hook and gateway protection continue uninterrupted because they live in separate processes. The TUI surfaces Sentry status prominently so the user knows when detection coverage is degraded.

If Sentry's event ingestion exceeds the configured resource cap (CPU or memory), the daemon enters degraded mode: it sheds load by sampling events rather than processing every one, logs a warning, and notifies the user via desktop notification and TUI badge. Rule false-negative rate goes up during degraded mode but the system stays responsive. Users can configure the daemon to fail-closed (suspend the agent loop on Sentry unavailability) or fail-open (continue with hook/gateway protection only) depending on their threat model.

## 14. Stack

### 14.1 Core

- Go 1.25+ for CLI, hook handler, gateway daemon, scan orchestrator, catalog sync, file watcher, audit logging, policy engine.
- Minimal non-stdlib dependencies in the core, matching Bumblebee's discipline as closely as possible. Custom parsers for lockfile formats, custom JSON handling tuned for the policy engine hot path. Justified exception: `fsnotify` for cross-platform filesystem notifications (inotify/FSEvents/ReadDirectoryChangesW), since hand-rolling three platform-specific watcher implementations adds more risk than the dependency.
- Single static binary per platform. Distributed via `go install` and pre-built releases via GitHub Releases.

### 14.2 Sidecars

- Python 3.11+ for LlamaFirewall sidecar. Managed by Beekeeper supervisor. Communicates over Unix socket / Windows named pipe using length-prefixed JSON. Python is the right tool here because LlamaFirewall, PyTorch, and the BERT model ecosystem live in Python.
- TypeScript / Node.js (Bun runtime) for v1.5 hook scaffold libraries distributed via npm. Reference implementations of Claude Code, Cursor, and Codex hooks for users who prefer the JS surface. The core binary remains Go; these are thin wrappers.

### 14.3 Development tooling

- GSD as primary execution framework. Beekeeper development follows GSD phase gates: planning, architecture review, milestone-based build, evaluation harness via Playwright (for TUI), Go test suite, NDJSON schema validation.
- Claude Code on Max plan as the primary builder.
- WSL Ubuntu for Linux build/test on a Windows host. Cross-compilation to all three OS targets in CI.

### 14.4 Testing and cross-platform CI

- Go test suite for unit + integration. Embedded fixtures matching Bumblebee's selftest pattern.
- Bubble Tea snapshot tests for the TUI (v1).
- Adversarial corpus for the policy engine: a curated set of real malicious tool call patterns from the May 2026 incidents, used as regression tests.
- LlamaFirewall sidecar integration tests via Python pytest, invoked from the Go test harness.
- Sentry rule fixtures: synthetic process trees, file access sequences, and network connection events asserted against expected rule fires. Honeypot end-to-end test plants a fake credentials file and a planted process; assertions check that Beekeeper Sentry flags the extension-host descendant case and ignores the bash case.
- OS-specific integration tests behind `//go:build linux`, `//go:build darwin`, `//go:build windows` tags. Only run on the matching platform.

**Cross-platform CI matrix.** GitHub Actions on `ubuntu-latest`, `macos-latest` (Apple Silicon and Intel), and `windows-latest`. Every PR runs the full test suite on all three. Free for public repos, no hardware cost. The honest constraint: the maintainer (initially solo Windows-native) cannot debug macOS failures locally; CI-driven iteration on macOS is the only path. For macOS-specific bugs that resist CI iteration, the project budget allows for ad-hoc EC2 mac instance time when needed (approximately $26/day with 24h minimum dedication). Not a continuous cost. Community testers are recruited for pre-release macOS acceptance testing where possible.

**Bumblebee compatibility test.** Beekeeper runs alongside Bumblebee on the same machine in CI. NDJSON output is asserted to be schema-consistent, no double-counting of findings, audit log records correctly attribute scanner_name.

## 15. Phasing

The AI-native development context (Claude Code on Max plan + GSD harness) collapses traditional implementation timelines for well-specified work. v1.0.0 absorbs scope that would historically be split across v1 and v2. The phasing below reflects this: v1.0.0 is a comprehensive release positioned as a standalone open-source project that could stand on its own merits even without further development.

**Every milestone includes explicit self-defense deliverables.** Beekeeper's internal threat model (Section 12) is not a v1.0.0 final-polish concern; it is built into every phase from v0.1.0 onward. Shipping a security tool without taking its own integrity seriously would be both ironic and irresponsible.

### v0.1.0: Minimum viable harness

Personal use deployable. Protects the builder's own machine first. Shipped, dogfooded, public.

Core deliverables:

- Go binary skeleton, project structure, CI matrix established on all three OSes.
- `beekeeper check` hook handler.
- `beekeeper hooks install --target claude-code`.
- Bumblebee `threat_intel/` catalog sync and matching.
- Release-age policy for npm and PyPI.
- Basic NDJSON audit log to local file.
- `beekeeper catalogs sync`, `beekeeper audit tail`.

Self-defense deliverables (Section 12):

- Reproducible builds: deterministic Go build flags, hashed against published releases.
- Sigstore signing via GitHub Actions OIDC for every release artifact.
- Pinned dependencies with `go.mod` and `go.sum` verification in CI.
- `0600` audit log permissions enforced on Unix from the first write.
- `SECURITY.md` published with disclosure process and contact.
- Hook handler fail-closed default with documented resource limits.

### v0.3.0: Full policy engine

Core deliverables:

- OSV and Socket public API catalog sources.
- Lifecycle script policy for npm, pip, pnpm, cargo, gem, composer.
- Editor extension CLI parsing for code, code-insiders, cursor, windsurf.
- File-watcher daemon (`beekeeper watch`) for VS Code, Cursor, Windsurf, OpenVSX extension directories.
- Quarantine workflow with restore and purge commands.
- Sensitive path policy.
- Network egress policy.
- Shim layer.
- `beekeeper scan` orchestrating Bumblebee.
- `beekeeper catalogs watch` daemon with catalog-delta-triggered scans.
- Cursor and Codex hook integration.

Self-defense deliverables (Section 12):

- Corroboration-based catalog matching (Section 5.1): single-source warn, two-source enforce.
- Catalog signature verification for all sources that support signing.
- Catalog provenance fields in every NDJSON audit record.
- Sanity bounds on catalog deltas with degraded-mode trigger.
- Read-and-notify degraded mode when catalog source integrity is uncertain.
- No remote URL fetching from catalog data; schema strictly static JSON.
- Fuzz testing in CI for catalog parser and policy engine.

### v0.6.0: Gateway, observability, and Sentry foundation

Core deliverables:

- MCP gateway daemon (full implementation).
- Continue, OpenCode, OpenClaw integrations via MCP.
- Syslog and OpenTelemetry audit sinks.
- Behavioral baseline engine.
- Multi-agent observability (subagent context propagation).
- Sentry daemon Linux implementation (fanotify + eBPF).
- `beekeeper protect install` workflow.
- Default Sentry rule set with 7-day audit-only baseline period.

Self-defense deliverables (Section 12):

- MCP gateway localhost-only binding by default; explicit acknowledgment required to expose remotely.
- MCP gateway per-session token authentication for all clients.
- Strict MCP protocol parsing with bounded message sizes and recursion limits.
- Sentry daemon IPC authorization via `SO_PEERCRED` on Linux.
- Privilege separation within the Sentry daemon: eBPF loading is privileged; rule evaluation and event filtering drop capabilities.
- `seccomp-bpf` filters on the unprivileged Sentry components.
- Fuzz testing extended to MCP message parser and Sentry rule evaluator.
- Audit log sensitive field redaction patterns shipped with sensible defaults.

### v0.9.0: Sentry cross-platform and defensive depth

Core deliverables:

- Sentry daemon macOS implementation (eslogger-based).
- Sentry daemon Windows implementation (ETW).
- LlamaFirewall sidecar integration (PromptGuard 2, CodeShield, AlignmentCheck).
- Output filtering for credentials and sensitive data.
- Multi-turn exfiltration detection.
- ContextForge / MCPGuard policy-plugin mode.

Self-defense deliverables (Section 12):

- SLSA Level 3 provenance attestation for every release.
- SBOM (CycloneDX format) published with each release.
- IPC authorization on macOS (Unix permissions + XPC entitlement) and Windows (named pipe ACLs).
- Two-account release approval rule active in GitHub repo settings.
- Verification path documented in `BUILDING.md` with `make verify-release` target.
- Cross-platform reproducible builds verified in CI matrix.

### v1.0.0: Comprehensive standalone release

Core deliverables:

- TUI dashboard (Bubble Tea), full functionality including admin mode.
- Policy as code (declarative JSON, version-controlled, testable, with `beekeeper policy test`).
- Distributed mode (team-shared catalogs, optional remote audit sinks).
- Cross-platform parity across macOS, Linux, Windows.
- Full documentation, threat model writeup, integration guides for all supported agents.
- Apache 2.0 license, code signing for distributed binaries where feasible.

Self-defense deliverables (Section 12):

- `beekeeper-self` catalog source live, with separate hosting and signing infrastructure.
- Self-quarantine behavior on `beekeeper-self` match at startup and on every catalog sync.
- Full Section 12 self-defense documentation published as part of the user-facing docs.
- Independent security review by at least one external reviewer (community or paid) before tagging v1.0.0.
- Bug bounty or VDP scope defined and published.

This is the milestone at which Beekeeper is a comprehensive open-source security tool that stands on its own merits as a portfolio piece and a useful project for the broader community.

### v2.x: Hardening and ecosystem

- Cargo and additional ecosystem coverage as Bumblebee adds them.
- SARIF export for security team workflows.
- Webhook policy for tool call decisions (custom integrations).
- Reference integrations with major MCP gateways.
- Audit log analytics and trend detection.
- Cross-host correlation for team deployments.
- macOS EndpointSecurity entitlement application path, with full EndpointSecurity once obtained.
- Ongoing self-defense: independent annual security review cadence, expanded fuzz coverage, hardware-token signing for releases if maintainer set expands.

### v3.x: Kernel and runtime depth

- eBPF-based syscall mediation on Linux (true prevention, not just detection).
- EndpointSecurity full integration on macOS.
- Sandbox orchestration via Firecracker / gVisor.
- Local LLM-based tool-call anomaly classifier.
- Network egress mediation at the OS level (active blocking, not just detection).

## 16. Compatible layers

Explicit positioning vs. each adjacent tool:

| Tool | Relationship | Integration |
|---|---|---|
| Bumblebee | Consume | Embedded as scan orchestrator; `threat_intel` as primary catalog source |
| OSV-Scanner | Consume | OSV database via offline DB; Beekeeper invokes osv-scanner CLI |
| Socket Firewall Free | Coexist | Optional shim backend; users with Socket installed can route through it |
| Socket public API | Consume | Catalog source, free tier, rate-limited |
| pnpm v11 / Bun 1.3 | Steer toward | Beekeeper recommends pnpm/Bun with strict config when agent installs |
| LlamaFirewall | Consume | Sidecar process, managed by Beekeeper supervisor |
| ContextForge | Plugin into | v1.5 policy-plugin mode for users running ContextForge |
| MCPGuard / MCPX | Plugin into | Same pattern, ecosystem permitting |
| Claude Code Hooks | Use | Primary integration surface, not replaced |
| Falco / Tetragon | Foundation | v3 eBPF layer builds on Falco rules engine |
| NeMo Guardrails | Consume | Optional output-filter catalog source in v1.0 |

## 17. Open questions

Honest acknowledgment of what is not yet resolved:

- **Latency budget under load.** Sub-100ms p99 is the hook handler target. Actual measured performance with realistic catalog sizes and LlamaFirewall in-line is unknown until prototyped.
- **Sentry false positive rate.** The default rules are tuned conservatively but real-world false positive rate is unknown until prototyped against real developer workloads. The 7-day audit-only baseline period is the mitigation strategy; whether it's sufficient is an empirical question.
- **macOS EndpointSecurity entitlement.** v1 ships with eslogger-based monitoring on macOS as the realistic path for an indie OSS project. Applying for the entitlement is a v2 improvement path. Outcome of the application is uncertain.
- **Corroboration thresholds in practice.** The default thresholds (single-source warn, two-source enforce, three-source quarantine) are reasonable starting points but empirically untested. Some catalog sources may be more reliable than others; weighted corroboration (where Bumblebee's signal counts as 1.5 sources versus an unsigned user catalog counting as 0.5) is a potential extension if empirical data justifies it. For v1 we ship unweighted equal-vote corroboration and revisit based on operating experience.
- **`beekeeper-self` catalog governance.** Who maintains it? If the answer is "the same maintainer who runs the main repo," compromising both surfaces is one compromise, not two, and the recursive principle is weakened. Long-term answer is a separate maintainer set; short-term answer for v1.0.0 is documented honestly as "single maintainer, with the intent to separate as the project grows."
- **MCP gateway protocol versioning.** MCP roadmap includes gateway pattern changes in Q3 2026. Beekeeper's gateway mode may need adaptation.
- **Distribution and code signing.** Single Go binary is easy; macOS notarization and Windows code signing for a security tool with no commercial backer is a real friction point. Sigstore-based signing covers the OSS-verification path but doesn't satisfy OS-level "trusted publisher" requirements. We accept this friction for v1 and document the verification path users should follow.
- **Default elevation experience.** Should `beekeeper init` prompt the user to consider `beekeeper protect install` after initial setup, or wait for the user to discover it? Trade-off between discoverability of Sentry's value and not pressuring the trust ask.
- **External security review scope and source.** v1.0.0 requires independent review before tagging. Open: paid review (cost) vs. community review (timing uncertainty) vs. both. May depend on what's available in the OSS security community for a project of this profile when v1.0.0 approaches.
- **License.** Apache 2.0 to match Bumblebee. Locked.
- **Sub-component naming.** Sentry is the agent-independent detection layer. Open: do other components warrant distinct names (Hive for the policy + catalog store, Apiary for the audit log surface)? Or is "Beekeeper, the system" the only branding needed?
- **Funding model.** Solo OSS project. No commercial product roadmap committed at v1. Pre-1.0 work is solo, dogfooded, open from day one.

---

*End of v0.3 PRD.*
