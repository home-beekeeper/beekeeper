# Roadmap: Beekeeper

## Overview

Beekeeper is built in nine phases, each delivering a coherent, independently verifiable capability. Phase 1 lays the foundation: a working hook handler with Bumblebee catalog matching that protects the developer's own machine from day one. Phases 2-4 expand the policy engine, multi-source corroboration, editor extension defense, and integration surfaces. Phases 5-7 add OS-level Sentry event streams (Linux eBPF, then macOS eslogger and Windows ETW). Phase 6 brings the LlamaFirewall sidecar and full audit sinks. Phase 8 delivers the TUI dashboard. Phase 9 closes the loop with policy as code, the beekeeper-self catalog, and SLSA Level 3 provenance — the point at which the developer trusts Beekeeper on real work.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3...): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Foundation + Hook Handler** - Working `beekeeper check` with Bumblebee catalog, sub-100ms target, reproducible Sigstore releases
- [ ] **Phase 2: Policy Engine + Multi-Source Catalogs** - Full corroboration semantics (OSV + Socket), lifecycle/path/egress/baseline policies, catalog watch daemon
- [ ] **Phase 3: Editor Extension Defense** - Agent CLI intercept, fsnotify extension watcher, quarantine workflow, `beekeeper scan`
- [ ] **Phase 4: Integration Surfaces** - Hook installs for Claude Code/Cursor/Codex, MCP gateway daemon, shim layer, multi-agent observability
- [ ] **Phase 5: Linux Sentry** - Privileged systemd daemon, fanotify + cilium/ebpf event ingestion, 5-rule correlation engine, 7-day baseline
- [ ] **Phase 6: LlamaFirewall + Audit Sinks** - Optional Python sidecar with PromptGuard 2 / CodeShield / AlignmentCheck; full audit sinks (syslog, OTLP, HTTPS); audit query/export commands
- [ ] **Phase 7: Cross-Platform Sentry** - macOS eslogger Sentry, Windows ETW Sentry, SLSA Level 3 provenance + SBOM
- [ ] **Phase 8: TUI Dashboard** - Bubble Tea v2 dashboard, all panels (activity, alerts, catalog, scan, policies, quarantine, health), admin mode
- [ ] **Phase 9: Policy as Code + Self-Defense Capstone** - Declarative JSON policies, `policy test/validate/list`, layered config, `beekeeper-self` catalog live, v1.0.0

## Phase Details

### Phase 1: Foundation + Hook Handler
**Goal**: Developer can protect their own machine from day one — `beekeeper check` evaluates real tool calls against the Bumblebee catalog and blocks or warns before any agent action takes effect
**Depends on**: Nothing (first phase)
**Requirements**: HOOK-01, HOOK-02, HOOK-03, HOOK-04, CTLG-01, CTLG-05, CTLG-07, SFDF-01, SFDF-02, SFDF-03, SFDF-04
**Success Criteria** (what must be TRUE):
  1. Developer can run `beekeeper check` on a tool call JSON from stdin and receive allow (exit 0) or block (exit non-zero) with a structured reason in under 100ms p95 on a realistic Bumblebee catalog
  2. A crash or timeout in `beekeeper check` results in a block decision, never a silent allow
  3. Developer can run `beekeeper catalogs sync` and the Bumblebee `threat_intel/` catalog is fetched, signature-verified, and cached with a mmap-loadable binary index
  4. Every release binary is reproducibly buildable via `make verify-release VERSION=X.Y.Z`, Sigstore-signed, and accompanied by `SECURITY.md` with a responsible disclosure process
  5. `go mod verify` passes in CI confirming all dependencies are pinned in `go.sum`
**Plans**: 6 plans
- [x] 01-PLAN-project-scaffold.md — Go module, Cobra CLI skeleton, cross-platform state dir + permissions, CI matrix (wave 1)
- [x] 01-PLAN-catalog-sync.md — Bumblebee fetch, schema parse, mmap binary index, signature presence check (wave 2)
- [x] 01-PLAN-self-defense.md — Reproducible builds, cosign v3 signing, Renovate pinning, SECURITY.md (wave 2)
- [x] 01-PLAN-policy-engine.md — Pure internal/policy Evaluate, Bumblebee single-source warn semantics (wave 3)
- [x] 01-PLAN-audit-logging.md — Owner-only NDJSON audit writer, audit tail (wave 4)
- [x] 01-PLAN-hook-handler.md — Fail-closed beekeeper check, config fail mode, selftest, latency benchmark (wave 5)

### Phase 2: Policy Engine + Multi-Source Catalogs
**Goal**: The policy engine enforces corroboration-based threat intelligence — two independent catalog sources (OSV offline DB and Socket PURL API) are required to block, closing the false-positive gap from single-source enforcement
**Depends on**: Phase 1
**Requirements**: PLCY-01, PLCY-02, PLCY-03, PLCY-04, PLCY-05, PLCY-06, PLCY-07, PLCY-08, CTLG-02, CTLG-03, CTLG-06, CTLG-08, CTLG-09
**Success Criteria** (what must be TRUE):
  1. A package flagged by one catalog source alone triggers a warn-level decision; flagged by two independent sources triggers a block; flagged by three triggers block + quarantine recommendation
  2. An `npm install` of a package younger than 24 hours is blocked with a structured reason citing the release-age policy; the threshold is configurable per-ecosystem
  3. A tool call attempting to read `~/.ssh/` or `~/.aws/` is blocked by the sensitive path policy; lifecycle scripts (`preinstall`, `postinstall`) are blocked unless the package is on the allowlist
  4. `beekeeper catalogs watch` daemon detects a new Bumblebee `threat_intel/` entry within its poll interval and triggers an immediate scan without developer intervention
  5. Any catalog delta exceeding sanity bounds puts the affected source into degraded mode (warning-only) and the degradation is recorded in the audit log with full catalog provenance
**Plans**: TBD
**Research note**: Socket PURL API free-tier rate limits are not fully documented — implement 24h TTL cache per package+version aggressively and validate empirically; fsnotify Windows behavior with VS Code extension junction points needs live testing

### Phase 3: Editor Extension Defense
**Goal**: Agent-initiated extension installs and silently dropped extension directories are intercepted and evaluated before they can execute, closing the Nx Console-class attack surface
**Depends on**: Phase 2
**Requirements**: EDXT-01, EDXT-02, EDXT-03, EDXT-04, EDXT-05, EDXT-06
**Success Criteria** (what must be TRUE):
  1. When an agent runs `code --install-extension <id>` (or cursor/windsurf variants, including bulk multi-flag forms), Beekeeper intercepts the call, evaluates it against the catalog and release-age policy, and blocks or warns before the extension is installed
  2. `beekeeper watch` detects a new directory in `~/.vscode/extensions/` via OS-native filesystem notifications and triggers catalog match and release-age check within seconds, without polling
  3. On a catalog hit for a new extension, Beekeeper emits a critical audit record, optionally sends a desktop notification, and moves the extension to `~/.beekeeper/quarantine/extensions/`
  4. Developer can run `beekeeper quarantine list`, `quarantine restore <id>`, and `quarantine purge` to manage quarantined items
  5. `beekeeper init` detects installed editors and offers (with consent) to disable extension auto-update and enable the file-watcher for detected extension directories
**Plans**: TBD

### Phase 4: Integration Surfaces
**Goal**: Every major agent surface (Claude Code hooks, Cursor, Codex CLI, MCP gateway, PATH shims) is wired to the Beekeeper policy engine with a single install command
**Depends on**: Phase 2
**Requirements**: INTG-01, INTG-02, INTG-03, INTG-04, INTG-05, INTG-06, INTG-07
**Success Criteria** (what must be TRUE):
  1. `beekeeper hooks install --target claude-code` writes valid PreToolUse and PostToolUse hooks to `~/.claude/settings.json` and Claude Code begins routing tool calls through `beekeeper check` without any manual configuration
  2. `beekeeper gateway` starts a stateless MCP proxy on `127.0.0.1` that applies the policy engine inline to every proxied tool call and requires per-request token auth, with JSON-RPC responses correlated by `id` field (not position)
  3. Continue, OpenCode, and OpenClaw can be pointed at `beekeeper gateway` and their tool calls are evaluated through the same policy engine as native hook integrations
  4. `beekeeper shim install` places wrapper binaries for npm, pnpm, pip, cargo, go, gem, composer, npx, and pipx earlier in PATH; each shim invokes `beekeeper check` and proxies to the real binary only if allowed
  5. Subagent tool calls carry parent-context propagation, and policy decisions for child agents are recorded with parent-child lineage in the audit log
**Plans**: TBD
**Research note**: MCP client implementation differences between Claude Code and Cursor expose different edge cases; July 2026 spec SDK lag may require working around SDK inconsistencies; fuzz the MCP message parser before v0.6.0 release as a release gate

### Phase 5: Linux Sentry
**Goal**: Developers who opt in on Linux gain OS-level process correlation — agent-linked credential access, phone-home attempts, and fresh-extension behavior clusters are detected and alerted in real time before reaching the audit log
**Depends on**: Phase 4
**Requirements**: SLNX-01, SLNX-02, SLNX-03, SLNX-04, SLNX-05, SLNX-06, SLNX-07, SLNX-08, SLNX-09, SLNX-10
**Success Criteria** (what must be TRUE):
  1. `beekeeper protect install` installs a systemd service that starts, persists across reboots, and can be uninstalled and queried via `beekeeper protect status`
  2. The Sentry daemon ingests process creation, file access on sensitive-path watchlist entries, and outbound network connections via eBPF and fanotify, with process attribution on each event
  3. On kernels below 5.15, `beekeeper protect status` explicitly reports the degradation tier and which event types are unavailable; the daemon does not silently run in a reduced state
  4. All five default rules (extension-host credential cluster, credential CLI burst, phone-home, fresh-extension correlation, exfil signature fusion) operate in audit-only mode for the first 7 days and emit Sentry alert records to the NDJSON audit log on trigger
  5. The unprivileged CLI communicates with the privileged Sentry daemon over a Unix socket authenticated by SO_PEERCRED peer credential verification
**Plans**: TBD
**Research note**: eBPF CO-RE multi-kernel CI matrix requires explicit Ubuntu 20.04 (kernel 5.4) and 22.04 (kernel 5.15) coverage — CI ubuntu-latest alone is insufficient; correlation rule thresholds (60-second windows, 2-occurrence triggers) derived from Nx Console postmortem timeline, not empirically validated; plan structured false positive measurement during this phase

### Phase 6: LlamaFirewall + Audit Sinks
**Goal**: Agent context is scanned for prompt injection and insecure code before it acts, and the full audit record is deliverable to syslog, OTLP endpoints, and HTTPS sinks — making Beekeeper observable to any monitoring stack
**Depends on**: Phase 5
**Requirements**: LLMF-01, LLMF-02, LLMF-03, LLMF-04, LLMF-05, LLMF-06, AUDT-01, AUDT-02, AUDT-03, AUDT-04, AUDT-05, AUDT-06, AUDT-07
**Success Criteria** (what must be TRUE):
  1. With `beekeeper llamafirewall enable`, the Python sidecar starts under Beekeeper supervision; a sidecar crash triggers up to 3 restart attempts with exponential backoff, and tool calls fail-closed during unavailability
  2. WebFetch results, file reads, and MCP tool responses flagged as prompt injection by PromptGuard 2 are redacted and replaced with a structured warning before reaching agent context
  3. Agent-generated file writes containing code are evaluated by CodeShield; insecure patterns are flagged or blocked per configured policy
  4. `beekeeper audit tail` streams live NDJSON records to the terminal; `beekeeper audit query --since|--agent|--tool|--decision` returns filtered results; `beekeeper audit export` produces ndjson, csv, or otlp output
  5. Syslog (RFC 5424), OTLP, and HTTPS POST sinks are off by default and each requires explicit opt-in config with a documented warning that audit data leaves the machine
**Plans**: TBD

### Phase 7: Cross-Platform Sentry
**Goal**: The full Sentry capability is available on macOS (via eslogger) and Windows (via ETW), and every Beekeeper release includes SLSA Level 3 provenance and a CycloneDX SBOM
**Depends on**: Phase 5
**Requirements**: SMAC-01, SMAC-02, SMAC-03, SMAC-04, SWIN-01, SWIN-02, SWIN-03, SWIN-04, SWIN-05, SWIN-06, SFDF-05
**Success Criteria** (what must be TRUE):
  1. `beekeeper protect install` on macOS installs a launchd plist; eslogger subprocess is supervised with a high-priority goroutine draining stdout; the same five default rules and 7-day baseline apply on normalized events
  2. `beekeeper protect status` on macOS explicitly documents Keychain and in-memory Cocoa API access as not observable by eslogger
  3. `beekeeper protect install` on Windows installs a Windows Service under LocalService privileges; ETW event ingestion uses `tekert/golang-etw` with no CGO; the `EventsLost` counter is surfaced in `beekeeper diag` and in the TUI
  4. At install time on Windows, Beekeeper probes for an existing NT Kernel Logger session conflict and surfaces it in `beekeeper protect status` if one is found
  5. Every release artifact is accompanied by a verifiable SLSA Level 3 provenance attestation (slsa-github-generator v2.1.0) and a CycloneDX SBOM generated by syft in the GoReleaser pipeline
**Plans**: TBD
**Research note**: eslogger field names are partially undocumented — build the macOS Sentry parser against real eslogger output on macos-latest CI, not synthetic JSON; ETW MinimumBuffers and MaximumBuffers values need empirical measurement during this phase under worst-case npm install event rates on Windows

### Phase 8: TUI Dashboard
**Goal**: Developer can see everything Beekeeper knows — live tool call decisions, Sentry alerts, catalog freshness, scan status, active policies, quarantine, and system health — in a single terminal screen without leaving the keyboard
**Depends on**: Phase 7
**Requirements**: TUI-01, TUI-02, TUI-03, TUI-04, TUI-05, TUI-06, TUI-07, TUI-08, TUI-09, TUI-10
**Success Criteria** (what must be TRUE):
  1. `beekeeper dashboard` launches a Bubble Tea v2 TUI that updates in real time via file-watcher on the audit NDJSON log (1-second polling fallback) and works correctly on Windows Terminal, including after a window resize
  2. The live activity feed shows tool calls with allow/warn/block indicators, agent identity, tool name, and target; the feed is filterable without leaving the TUI
  3. The Sentry alerts panel shows process correlation events with severity color coding and expandable process tree, file access, and network destination detail
  4. Catalog freshness, scan status, active policy rules, quarantine items, and system daemon health (Sentry, gateway, LlamaFirewall) are each visible in dedicated panels without requiring a separate CLI command
  5. With `--admin` flag, developer can toggle individual policy rules and restore or purge quarantine items directly from the TUI without a separate terminal
**Plans**: TBD
**UI hint**: yes

### Phase 9: Policy as Code + Self-Defense Capstone
**Goal**: Policy is version-controllable, testable, and layered; Beekeeper monitors its own supply chain integrity via the separately hosted and signed `beekeeper-self` catalog — the system is ready to be trusted on real production work
**Depends on**: Phase 8
**Requirements**: CODE-01, CODE-02, CODE-03, CODE-04, CODE-05, CODE-06, SFDF-06, CTLG-04
**Success Criteria** (what must be TRUE):
  1. Developer can write a declarative JSON policy file, validate it with `beekeeper policy validate <file>`, dry-run it against a sample tool call with `beekeeper policy test <file>`, and list loaded policies with `beekeeper policy list`
  2. Config merges correctly across system → user → project → env var → CLI flag layers; a project-level `.beekeeper/config.json` overrides user-level config without requiring environment variables
  3. `beekeeper diag` displays hook latency p95/p99, sidecar inference latency, catalog freshness per source, and ETW `EventsLost` count in a single human-readable output
  4. The `beekeeper-self` catalog is live at a separate host with a separate signing key and separate access control; Beekeeper checks it on every startup and every catalog sync and self-quarantines if the running version appears as compromised
  5. The complete threat model is documented publicly, including the known coordinated false-positive poisoning attack surface for corroboration semantics and the fanotify mmap gap on Linux
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation + Hook Handler | 6/6 | Complete | 2026-05-26 |
| 2. Policy Engine + Multi-Source Catalogs | 0/TBD | Not started | - |
| 3. Editor Extension Defense | 0/TBD | Not started | - |
| 4. Integration Surfaces | 0/TBD | Not started | - |
| 5. Linux Sentry | 0/TBD | Not started | - |
| 6. LlamaFirewall + Audit Sinks | 0/TBD | Not started | - |
| 7. Cross-Platform Sentry | 0/TBD | Not started | - |
| 8. TUI Dashboard | 0/TBD | Not started | - |
| 9. Policy as Code + Self-Defense Capstone | 0/TBD | Not started | - |
