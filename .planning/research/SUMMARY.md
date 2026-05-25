# Project Research Summary

**Project:** Beekeeper
**Domain:** Agent runtime safety harness / developer workstation security daemon
**Researched:** 2026-05-26
**Confidence:** HIGH (core patterns), MEDIUM (platform-specific kernel interfaces)

## Executive Summary

Beekeeper is a Go-based, multi-process security daemon acting as the runtime enforcement layer for autonomous coding agents (Claude Code, Cursor, Codex) on developer workstations. The competitive landscape reveals a clear gap: every existing tool addresses only one slice of the attack surface. Bumblebee is inventory-only. LlamaFirewall is semantic-layer only. Socket Firewall Free is fetch-layer only. MCP gateways are protocol-layer only. Microsoft AGT is cloud and container focused. Beekeeper occupies the intersection of all these layers simultaneously: hook interception at the agent boundary, a stateless MCP proxy with inline threat intelligence, OS-level process correlation via eBPF/ETW/eslogger, and a continuous supply chain catalog with corroboration-based enforcement semantics. The Nx Console compromise (May 2026) provides direct empirical validation of the threat model: a malicious VS Code extension executed within 11-18 minutes of install, stole GitHub tokens and AWS credentials, and exfiltrated them, entirely undetected by any existing tool in the landscape.

The recommended approach is a pure-Go single binary (Go 1.25+, no CGO for core) with a strict two-tier privilege model: an unprivileged tier covering hooks, MCP gateway, file watcher, and catalog sync; and an optional privileged Sentry tier covering OS-level event streams. The stack is lean: fsnotify/v1.10.1 for file watching, cilium/ebpf v0.21.0 for Linux kernel event ingestion, charm.land/bubbletea/v2 for the TUI, with the Python LlamaFirewall sidecar kept architecturally isolated to preserve the Go binary boundary. The defining architectural characteristic is a pure-library policy engine (internal/policy) shared between the ephemeral hook handler and the long-lived gateway daemon, ensuring identical enforcement semantics across both surfaces with no policy drift.

The primary risk is adoption failure driven by false positives or perceived performance overhead. Research from real deployments of analogous tools (Falco, Claude Code hooks, security tool postmortems) is unambiguous: alert fatigue kills adoption faster than missing features do. The mitigation strategy is built into the architecture: corroboration-based catalog semantics (two independent sources required to enforce, one to warn), a mandatory 7-day audit-only baseline period before Sentry rules promote to enforcement, conservative defaults for Sentry process correlation rules, and a sub-100ms p99 latency target for the hook handler measured under cold-cache conditions with realistic catalog sizes. A secondary risk is kernel version fragmentation silently degrading Linux Sentry; this requires capability probing and explicit graceful degradation tiers from the first Sentry commit.

## Key Findings

### Recommended Stack

Beekeeper's stack is pure-Go from top to bottom except for the Python LlamaFirewall sidecar. Go 1.25 is the baseline, providing container-aware GOMAXPROCS, a 4x ECDSA/Ed25519 signing speedup, and the new net/http CrossOriginProtection middleware (enable on the gateway even for localhost binding). The stack is divided by component lifecycle: the ephemeral hook handler uses no Bubble Tea, no HTTP clients on the critical path, and no CGO; the long-lived gateway uses net/http with stateless per-request design (the July 2026 MCP spec eliminates sessions entirely); the Sentry daemon uses cilium/ebpf on Linux and golang.org/x/sys/windows for ETW on Windows; the TUI uses charm.land/bubbletea/v2 with a resize-poll goroutine workaround for a confirmed Windows Terminal bug (Issue #1601) where WindowSizeMsg never fires after initial startup.

**Core technologies:**
- Go 1.25: Primary language, single static binary, no CGO for core, go.sum integrity, container-aware GOMAXPROCS
- github.com/fsnotify/fsnotify v1.10.1: Cross-platform file watching. Watch parent directory and filter Create events by type. Recursive watching is not in the public API and must never be used.
- github.com/cilium/ebpf v0.21.0: Linux eBPF program loading, pure Go no CGO. Kernel 5.15+ for full feature set (CO-RE, ring buffers, bpf_link). Embed pre-compiled bytecode via bpf2go from day one.
- charm.land/bubbletea/v2 v2.0.6: TUI dashboard for long-lived processes only. Never use for beekeeper check due to escape sequence leak bug #1627. Must pair with charm.land/lipgloss/v2.
- github.com/google/osv-scanner/v2 v2.3.8: Import as Go library for the policy hot path. Download offline DB directly via GCS URLs, not subprocess.
- GoReleaser v2.13.0+ with cosign v3.x: Use the --bundle flag (v3 format). Earlier GoReleaser versions produce cosign v2 format incompatible with v3 verification.
- slsa-framework/slsa-github-generator v2.1.0: SLSA Level 3 provenance. Minimum v1.10.0 due to TUF mirror error in all earlier versions. Use actions/download-artifact@v3, not v4.
- Python 3.11+ (sidecar only): LlamaFirewall kept as supervised sidecar to preserve Go binary boundary. 400-800MB RAM cost means disabled by default.
- MCP protocol July 2026 spec: Stateless HTTP proxy design. Sessions eliminated. Implement Mcp-Method and Mcp-Name header forwarding from day one.

**What not to use:** encoding/json/v2 (experimental, not Go 1 compat guaranteed), Mcp-Session-Id tracking (removed from July 2026 spec), Bubble Tea for beekeeper check (escape sequence leak bug #1627), fsnotify recursive watching (not public API), cosign v2 output-signature/output-certificate pattern, slsa-github-generator before v1.10.0, CGO in core binary.

### Expected Features

Research confirms a clear MVP boundary. The Nx Console postmortem and supply chain incident data provide direct empirical grounding for which features have demonstrable ROI on day one versus which add complexity before the core threat class is addressed.

**Must have (table stakes), ship by v0.3.0:**
- beekeeper check hook handler: reads tool call from stdin, exits 0 (allow) or 2 (block), sub-100ms p99
- beekeeper hooks install --target claude-code: writes to ~/.claude/settings.json; zero-friction setup is mandatory for adoption
- Bumblebee threat_intel/ catalog sync and matching: schema requires top-level schema_version and entries object; bare arrays are rejected
- Release-age policy (24h default for npm/PyPI): pnpm v11 ships this by default; near-zero false positives on stable packages
- NDJSON audit log with 0600 permissions and auto-rotate
- Fail-closed default on crash or timeout
- Sigstore keyless signing and SECURITY.md: TeamPCP specifically targeted security tools; own integrity posture is evaluated by target audience
- Sensitive path policy covering ~/.ssh/, ~/.aws/, .env
- Lifecycle script enforcement (allowlist-only, cross-ecosystem): preinstall/postinstall is the primary supply chain malware delivery mechanism

**Should have (differentiators), v0.3.0 through v1.0.0:**
- OSV + Socket catalog sources with corroboration semantics: 2FA for threat intel; single source warns, two sources enforce; no competing tool provides this
- Editor extension enforcement across 3 layers (agent CLI + fsnotify file watcher + Bumblebee scan): the Nx Console attack surface is categorically unaddressed by every competing tool
- MCP gateway with inline threat intelligence: all existing MCP gateways provide access control and audit but zero supply chain matching
- Sentry daemon (Linux fanotify + eBPF): OS-level process correlation; no competing developer workstation tool does this
- TUI dashboard (beekeeper dashboard): legibility without a browser or cloud account
- Behavioral baseline (counter-based, not ML): transparent, auditable, zero training cost
- beekeeper-self catalog: separately hosted and signed; recursive compromise detection
- Windows first-class support: no existing agent safety tool ships Windows as a primary target; VS Code developer population is the primary Nx Console attack surface

**Defer to v2+:**
- macOS EndpointSecurity entitlement (Apple approval process uncertain for indie OSS; eslogger covers v1)
- Local LLM anomaly classifier (counter-based baseline is sufficient; ML adds false positive risk)
- Sandbox/microVM orchestration (hooks + Sentry cover practical threat classes without sandboxing overhead)
- eBPF-based syscall blocking (requires security review and co-maintainers; defer to v3)
- Cross-host team correlation (prove solo developer use case first)
- SARIF export (defer until enterprise use cases are confirmed)

**Anti-features to deliberately exclude:**
- Union-of-bad catalog semantics: one compromised source triggers false enforcement; alert fatigue kills adoption
- General-purpose rule engine: shifts rule quality burden to users; Falco high false positive reputation is the cautionary data point
- LlamaFirewall enabled by default: 400-800MB RAM cost produces negative first impression on resource-constrained laptops
- Kernel-mode syscall blocking in v1: privilege escalation attack surface from a solo-maintained OSS project is unjustifiable
- curl | sh as recommended install: self-defeating for a security tool; document as non-recommended with explicit risk explanation

### Architecture Approach

Beekeeper is a multi-process security daemon with a strict privilege boundary. The unprivileged tier (hook handler, MCP gateway, file watcher, catalog sync) delivers 80% of the value and runs without elevation. The privileged Sentry tier is opt-in via beekeeper protect install. The defining architectural principle is a pure-library policy engine (internal/policy) with no I/O, no goroutines, and no global state, shared by the ephemeral hook handler and the long-lived gateway daemon. The catalog is a two-level hash map (map[Ecosystem]map[PackageKey][]Entry) for O(1) exact-match lookups. Corroboration is a separate aggregation step over raw match results, keeping the two concerns independently testable.

**Major components:**
1. beekeeper check (ephemeral, unprivileged): stdin JSON in, exits 0/2, p99 < 100ms target under cold-cache realistic catalog
2. Policy Engine (internal/policy, pure library): shared by check and gateway; catalog match + corroboration + release-age + lifecycle + sensitive path + egress + baseline; no I/O
3. Catalog Store (internal/catalog): two-level hash index, mmap for fast cold load, atomic hot-reload via sync.RWMutex pointer swap
4. beekeeper gateway (long-lived daemon): stateless MCP proxy per July 2026 spec; per-request token validation; policy middleware on every tool call; Mcp-Method/Mcp-Name header forwarding
5. beekeeper watch (long-lived daemon): fsnotify loop over extension dirs; watch parent, filter Create events for new directories; trigger catalog match async
6. Sentry daemon (privileged, build-tag separated by platform): Linux fanotify + cilium/ebpf; macOS eslogger subprocess; Windows ETW via tekert/golang-etw (no CGO); shared correlation engine with 5 rules in v1 over 5-minute sliding window
7. IPC layer (internal/ipc): Unix domain socket on Linux/macOS, named pipe on Windows; SO_PEERCRED / pipe ACL authorization; length-prefixed JSON protocol
8. LlamaFirewall sidecar (optional, supervised): Python subprocess; fail-closed on crash; 200ms IPC timeout; max 3 restart attempts with exponential backoff
9. Audit Writer (internal/audit): append-only NDJSON, 0600 permissions; remote sinks (syslog, OTLP, HTTPS POST) off by default
10. TUI Dashboard (internal/tui): Bubble Tea v2; reads audit NDJSON (no privileged channel required); resize-poll goroutine for Windows Terminal bug workaround

### Critical Pitfalls

1. Hook handler cold-start latency blows out under realistic conditions: Benchmark p99 (not p50) from v0.1.0 under cold OS file cache with 5,000+ entry catalog. Implement pre-built binary catalog index written on catalogs sync and mmap-loaded on check. If p99 exceeds 80ms, implement persistent IPC daemon in v0.3.0. Never measure only warm-cache averages and consider it done.

2. eBPF kernel version fragmentation silently degrades Linux Sentry: Build capability probing and graceful degradation tiers (Tier 1: kernel 5.8+ full eBPF + ring buffers; Tier 2: 5.1-5.7 perf buffers + CAP_SYS_ADMIN; Tier 3: 4.20+ fanotify-only; Tier 4: inotify fallback) from the first Sentry commit. Document the fanotify mmap gap (credential reads via mmap() are invisible to fanotify) explicitly in the threat model and in beekeeper protect status output.

3. ETW event loss on Windows creates silent coverage blind spots: ETW uses a circular buffer that overflows under npm install event rates. Only one NT Kernel Logger session is permitted system-wide; existing EDRs may conflict. Surface the EventsLost counter in beekeeper diag and TUI as a release gate criterion. Implement ReadDirectoryChangesW polling for credential paths as a fallback when ETW is conflicted.

4. MCP gateway proxy correctness is harder than it appears: JSON-RPC 2.0 batched requests have non-deterministic response order; correlate by id field, never by position. Capability negotiation is a three-party problem (upstream capabilities, what the proxy can bridge, what the proxy advertises to clients). Per-request token validation is required; connection-level validation is a token reuse vulnerability. Fuzz the MCP message parser before v0.6.0 release; this is a release gate, not a backlog item.

5. Catalog corroboration 2-of-N is vulnerable to coordinated false-positive poisoning: An attacker controlling two catalog sources can trigger enforcement against legitimate packages as a DoS. Build source health monitoring (entry velocity spike detection) and delta sanity bounds from the first corroboration implementation in v0.3.0. Document the known attack surface in the public threat model at v1.0.0.

## Implications for Roadmap

Based on research, the architecture has clear dependency layers that dictate a natural build order. The policy engine is the central dependency; nothing else is meaningful without it. The hook handler is the fastest path to dogfooding on a real machine. Differentiating features build on the proven hook foundation. Supply chain integrity follows the release machinery.

### Phase 1: Foundation, Hook Handler and Bumblebee Catalog (v0.1.0)
**Rationale:** Minimum that protects the builder's own machine from day one. Validates hook integration with Claude Code. Proves catalog matching works. Generates real audit data against real tool calls. No elevation required. No daemons.
**Delivers:** Working beekeeper check with Bumblebee catalog matching, release-age policy for npm and PyPI, NDJSON audit log (0600), beekeeper hooks install for Claude Code, Sigstore keyless signing, SECURITY.md, reproducible builds.
**Addresses:** One-command install, real-time package install enforcement, audit log, Claude Code integration, fail-closed default, release-age policy, Sigstore releases, SECURITY.md.
**Avoids:** Hook latency pitfall: establish p50/p95/p99 latency benchmark with realistic catalog sizes; document in beekeeper diag output from v0.1.0.
**Architecture layers:** internal/config, internal/audit, internal/catalog (loader + index), internal/policy, internal/check, cmd/beekeeper.
**Research flag:** SKIP. All patterns well-documented. Go process startup, Cobra CLI, NDJSON, Sigstore, GoReleaser all have mature tooling.

### Phase 2: Extended Policy and Multi-Source Catalog (v0.3.0)
**Rationale:** Once the hook handler is proven, add the corroboration semantics that are the core differentiator. OSV offline DB and Socket PURL API are the two independent sources that make the 2FA property real. Editor extension enforcement addresses the Nx Console attack surface directly and is the second-highest-risk uncovered surface after phase 1.
**Delivers:** OSV and Socket catalog sources, corroboration semantics (1 source=warn, 2=enforce, 3=enforce+quarantine), sensitive path policy, lifecycle script policy (allowlist-only), editor extension CLI parsing, fsnotify extension directory watcher, shim layer (PATH wrappers), catalog delta-triggered automatic re-scan, catalog sync daemon with signature verification and sanity bounds, behavioral baseline (counter-based).
**Addresses:** Corroboration, sensitive paths, lifecycle scripts, editor extension enforcement, shim layer, catalog freshness visibility.
**Avoids:** Catalog corroboration poisoning pitfall: source health monitoring and delta sanity bounds built here, not retroactively. fsnotify watcher uses parent-dir-watch + filter-Create-events pattern from day one; recursive watching must never be attempted.
**Architecture layers:** internal/catalog/sync, internal/catalog/watcher, internal/watch, policy extensions (release-age, lifecycle, paths, egress, baseline).
**Research flag:** NEEDS RESEARCH. Socket PURL API rate limits partially undocumented. fsnotify Windows behavior with VS Code extension junction points needs live testing.

### Phase 3: MCP Gateway Daemon (v0.6.0, first half)
**Rationale:** Highest-complexity component after Sentry. Depends on proven policy engine from phases 1 and 2. The July 2026 MCP spec (stateless, sessions eliminated) simplifies implementation considerably vs. earlier designs. The IPC layer built here is reused by Sentry.
**Delivers:** Stateless MCP proxy with per-request policy enforcement, per-request token auth (not per-connection), Mcp-Method and Mcp-Name header forwarding, W3C Trace Context propagation, tools/list response caching per server-declared TTL, backpressure via context.WithTimeout, connection cap (default 10), fuzz testing corpus for MCP message parser.
**Addresses:** MCP gateway with inline threat intelligence, Cursor/Codex/Continue integrations.
**Avoids:** MCP proxy correctness pitfall: request ID correlation table from day one (never assume response order), three-party capability negotiation, per-request token validation (not per-connection), fuzz parser as release gate not backlog item.
**Architecture layers:** internal/ipc, internal/gateway.
**Research flag:** NEEDS RESEARCH. MCP client implementation differences between Claude Code and Cursor expose different edge cases. July 2026 spec SDK lag may require working around SDK inconsistencies rather than relying on spec-compliant SDK behavior.

### Phase 4: Linux Sentry and LlamaFirewall Sidecar (v0.6.0, second half)
**Rationale:** Sentry requires IPC from phase 3 and proven behavioral baseline from phase 2. Linux eBPF ecosystem is most mature. Ubuntu 22.04 CI gives kernel 5.15. LlamaFirewall ships here because it depends on the IPC pattern established for Sentry.
**Delivers:** Linux Sentry with fanotify file events and cilium/ebpf tracepoints via ring buffers, process correlation engine with 5 rules (2 enforcement defaults, 3 audit-only defaults), 7-day audit-only baseline mode, capability probing at beekeeper protect install, graceful degradation tiers (Tier 1 through Tier 4 based on kernel version), beekeeper-self catalog, LlamaFirewall sidecar supervisor with fail-closed wiring.
**Addresses:** Sentry daemon Linux, behavioral baseline, beekeeper-self catalog, LlamaFirewall sidecar, self-compromise detection.
**Avoids:** eBPF kernel version fragmentation pitfall: capability probing is a release gate; fanotify mmap gap documented explicitly in beekeeper protect status; extension-host phone-home and credential CLI burst rules ship as audit-only defaults.
**Architecture layers:** internal/sentry/linux, bpf/ (process_monitor.bpf.c, network_monitor.bpf.c via bpf2go), internal/llamafirewall.
**Research flag:** NEEDS RESEARCH. eBPF CO-RE multi-kernel CI matrix planning required. Correlation rule thresholds need empirical validation against real developer workflows. Explicit testing on Ubuntu 20.04 (kernel 5.4) and 22.04 (kernel 5.15) required; CI ubuntu-latest alone is insufficient.

### Phase 5: Cross-Platform Sentry and SLSA Provenance (v0.9.0)
**Rationale:** macOS and Windows Sentry build on the shared correlation engine from phase 4. SLSA Level 3 provenance ships as release infrastructure matures.
**Delivers:** macOS Sentry via eslogger with supervised subprocess and buffered stdout drain plus explicit coverage gap documentation (Keychain and in-memory access not covered), Windows Sentry via ETW using tekert/golang-etw (no CGO) with EventsLost surfacing and ReadDirectoryChangesW fallback for credential paths, SLSA Level 3 provenance via slsa-github-generator v2.1.0, CycloneDX SBOM via syft in GoReleaser pipeline.
**Addresses:** Windows first-class support, macOS Sentry, SLSA Level 3 provenance.
**Avoids:** ETW event loss pitfall: EventsLost surfacing is a release gate; single NT Kernel Logger session constraint probed at beekeeper protect install; eslogger pipe saturation handled by dedicated high-priority goroutine draining stdout tested during full npm install stress test.
**Architecture layers:** internal/sentry/darwin, internal/sentry/windows.
**Research flag:** NEEDS RESEARCH. eslogger field names are partially undocumented; test against actual eslogger output on macos-latest CI, not synthetic JSON. ETW MinimumBuffers and MaximumBuffers tuning values require empirical measurement during phase 5 development.

### Phase 6: TUI Dashboard and Policy as Code (v1.0.0)
**Rationale:** The TUI makes Beekeeper evaluable at a glance. Policy as code testing closes the loop on the understandable policy table-stakes requirement. Building TUI last means it ships with a full year of design learnings from preceding phases and real data to display.
**Delivers:** beekeeper dashboard (Bubble Tea v2, resize-poll goroutine for Windows Terminal, reads audit NDJSON without requiring a privileged channel), Sentry alerts panel, catalog freshness panel with red indicator on stale threshold, quarantine workflow (move to ~/.beekeeper/quarantine/ with TUI restore action), beekeeper policy test and validate commands, make verify-release with SLSA and SBOM and binary reproducibility check, complete threat model documentation.
**Addresses:** TUI dashboard, policy as code, quarantine workflow, multi-turn exfil detection groundwork.
**Avoids:** UX pitfall: stale catalog must be visible even when no active threats are detected; block messages must include which rule fired, which catalog sources corroborated, and what user action resolves it; teatest wrapped in internal beekeepertest package to isolate unstable API.
**Architecture layers:** internal/tui, internal/selfdefense, policy-as-code testing in internal/policy.
**Research flag:** SKIP. Bubble Tea v2 is well-documented. Audit log NDJSON streaming is standard. Wrap teatest in internal package regardless of namespace availability.

### Phase Ordering Rationale
- Policy engine first: internal/policy is the central dependency for the hook handler, gateway, Sentry baseline, and TUI; nothing else is meaningful without it
- Unprivileged tier before privileged tier: the Sentry 7-day baseline depends on real audit log data collected during hook handler operation; shipping Sentry first means audit-only mode regardless
- IPC before Sentry: the Unix socket and named pipe IPC layer built for the gateway in phase 3 is reused by Sentry in phase 4; build it once for two consumers
- Linux Sentry before cross-platform: eBPF ecosystem is most mature; validate the correlation engine design on the best-documented platform before adapting for eslogger and ETW
- TUI last: the TUI is a read consumer of the audit log and is more valuable when built with real data from preceding phases

### Research Flags

Phases needing deeper research during planning:
- Phase 3 (MCP Gateway): MCP client implementation differences between Claude Code and Cursor; July 2026 spec SDK lag handling
- Phase 4 (Linux Sentry): eBPF CO-RE multi-kernel CI matrix; correlation rule threshold empirical validation; Ubuntu 20.04 vs. 22.04 kernel testing
- Phase 5 (Cross-platform Sentry): eslogger field name completeness on live macOS; ETW buffer sizing empirical values on Windows

Phases with standard patterns (skip research during planning):
- Phase 1 (Hook Handler): Go process startup, Cobra CLI, NDJSON, Sigstore, GoReleaser all well-documented with mature tooling
- Phase 2 (Extended Policy): OSV offline DB, Socket PURL endpoint, fsnotify API, counter-based baseline all standard patterns with good documentation
- Phase 6 (TUI): Bubble Tea v2 well-documented; audit NDJSON streaming is standard; wrap teatest in internal package regardless

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Go 1.25, fsnotify v1.10.1, cilium/ebpf v0.21.0, bubbletea v2.0.6, osv-scanner v2.3.8 all version-verified against live sources. GoReleaser v2.13+ and cosign v3 bundle format verified. MCP July 2026 spec RC verified. Bubble Tea Windows bugs #1601 and #1627 verified against issue tracker. |
| Features | HIGH | Competitive landscape verified against live sources including Bumblebee GitHub, LlamaFirewall docs, Socket docs, MCP gateway comparisons, and Microsoft AGT and RAMPART announcements. Nx Console postmortem verified from multiple sources. pnpm v11 minimumReleaseAge verified. |
| Architecture | HIGH (core), MEDIUM (platform-specific) | Go multi-process patterns, policy engine design, catalog index, corroboration engine, and IPC authorization are HIGH confidence from established Go patterns. Linux fanotify and eBPF kernel version matrix are MEDIUM from community documentation. Windows ETW single-session constraint is HIGH from Microsoft docs. eslogger field coverage is MEDIUM due to partial documentation. |
| Pitfalls | HIGH | Hook latency real-world data from Claude Code hook deployment reports. eBPF kernel fragmentation from iovisor/bcc kernel version documentation. ETW event loss architecture from Microsoft docs. MCP proxy correctness failure modes from MCP transport architecture analysis. Falco false positive data from issue tracker. |

**Overall confidence:** HIGH

### Gaps to Address
- Socket API rate limits: free-tier limits are not fully publicly documented; validate empirically during phase 2; implement 24h TTL cache per package+version aggressively before hitting limits
- eBPF correlation rule threshold validation: 60-second windows and 2-occurrence triggers are research-derived starting points from the Nx Console postmortem timeline, not empirically validated values; plan structured false positive measurement during phase 4
- eslogger field name completeness: build the macOS Sentry parser against real eslogger output on macos-latest CI, not synthetic JSON constructed from partial documentation
- ETW buffer sizing: MinimumBuffers and MaximumBuffers values need empirical measurement under worst-case npm install event rates on Windows during phase 5 development
- Bubble Tea v2 teatest namespace: verify charm.land/x/exp/teatest availability before building TUI tests in phase 6; wrap in internal/beekeepertest regardless to isolate the unstable API
- Bumblebee schema evolution: v0.1.1 is current and the schema is actively evolving; pin to a specific schema version and add CI validation against a fixture catalog from phase 1

## Sources

### Primary (HIGH confidence)
- go.dev/doc/go1.25: Go 1.25 release notes
- pkg.go.dev/github.com/fsnotify/fsnotify: v1.10.1 docs, Windows ReadDirectoryChangesW caveats
- github.com/charmbracelet/bubbletea: v2.0.6 release, Windows issues #1601 and #1627
- blog.modelcontextprotocol.io/posts/2026-07-28-release-candidate/: July 2026 MCP spec RC
- github.com/perplexityai/bumblebee: NDJSON schema v0.1.0 and v0.1.1
- google.github.io/osv-scanner/: v2.3.8, offline mode, GCS download URLs
- github.com/slsa-framework/slsa-github-generator: v2.1.0 Go builder README
- goreleaser.com/blog/cosign-v3/: cosign v3 bundle migration
- learn.microsoft.com ETW documentation: ETW architecture, event loss, buffer properties
- learn.microsoft.com Named Pipe Security: Windows named pipe ACLs
- nx.dev/blog/nx-console-v18-95-0-postmortem: Nx Console postmortem and attack timeline
- meta-llama.github.io/PurpleLlama/LlamaFirewall/: LlamaFirewall official docs
- pnpm.io/supply-chain-security: pnpm v11 minimumReleaseAge and allowBuilds
- man7.org/linux/man-pages/man7/fanotify.7.html: fanotify kernel version matrix and mmap limitation
- github.com/iovisor/bcc/blob/master/docs/kernel-versions.md: eBPF kernel version feature matrix

### Secondary (MEDIUM confidence)
- github.com/cilium/ebpf/releases: v0.21.0 release (version HIGH, kernel-feature mapping MEDIUM)
- docs.socket.dev: Socket PURL endpoint and deprecated score endpoint; rate limits partially undocumented
- pkg.go.dev/github.com/tekert/golang-etw/etw: pure-Go ETW library (less widely used; no-CGO constraint confirmed)
- cybereason.com/blog/blue-teaming-on-macos-with-eslogger: eslogger analysis, field coverage, macOS gaps
- github.com/golang/go/issues/71497: encoding/json/v2 adoption timeline
- Various MCP gateway comparison sources (integrate.io, lunar.dev, dxheroes.io): competitive landscape verification

### Tertiary (LOW confidence, needs validation during implementation)
- ETW MinimumBuffers and MaximumBuffers optimal values: no published benchmarks found; empirical measurement required during phase 5
- eBPF correlation rule thresholds: derived from Nx Console postmortem timeline, not from controlled experiments
- Socket PURL endpoint free-tier rate limits: not fully publicly documented

---
*Research completed: 2026-05-26*
*Ready for roadmap: yes*
