# Feature Research

**Domain:** Agent runtime safety harness / developer workstation security
**Researched:** 2026-05-26
**Confidence:** HIGH (most claims verified against multiple live sources from May 2026)

---

## Competitive Landscape Summary

Before the feature breakdown, a precise account of what each existing tool actually provides — because the gaps are where Beekeeper lives.

### Bumblebee (Perplexity AI, released May 22, 2026)

Read-only inventory scanner. Single static Go binary. Three scan profiles (baseline, project, deep). Covers npm/pnpm/Yarn/Bun/PyPI/Go/RubyGems/Composer/MCP configs/editor extensions/browser extensions. Exposure catalog: minimal JSON with exact ecosystem/name/version matching. Schema-compatible NDJSON output. Apache 2.0.

**What it does not do:** No runtime enforcement. No package install interception. No hook integration with any agent. No network egress monitoring. No process correlation. No prompt injection detection. macOS/Linux only at v0.1.1; no Windows support shipped yet. No lifecycle script policy. No release-age policy. Read-only is the explicit design choice — Bumblebee answers "who has this package right now?" not "block this install."

**Gap Beekeeper fills:** Everything past detection. Beekeeper is the runtime enforcement layer on top of Bumblebee's inventory intelligence.

### LlamaFirewall (Meta, May 2025)

Python framework. Three scanners: PromptGuard 2 (86M BERT, 97.5% attack detection in proprietary dataset), AlignmentCheck (chain-of-thought deviation detection, experimental), CodeShield (Semgrep + regex insecure code detection, 8 languages).

**What it does not do:** No supply chain intelligence. No package install enforcement. No file system monitoring. No network egress control. No process correlation. No editor extension protection. No catalog matching. Pure semantic/LLM-layer defense — it does not know what npm packages are malicious. Requires Python + PyTorch (~400–800MB). No integration with agent hooks — users wire it themselves.

**Gap Beekeeper fills:** Supply chain enforcement, host-level process monitoring, structured hook integration. LlamaFirewall becomes Beekeeper's sidecar for the semantic layer Beekeeper does not implement.

### Socket Firewall Free (Socket, Sept 2025)

HTTP proxy wrapper around npm/yarn/pnpm/pip/Rust package fetches. Blocks confirmed malware (human-reviewed). Zero configuration. Warns on AI-flagged but unconfirmed packages. Only blocks what Socket has reviewed. Does not support private registries. Not designed for agent-initiated tool calls — designed for human-invoked package manager commands.

**What it does not do:** No MCP gateway integration. No hook integration. No file system or process monitoring. No release-age policy. No lifecycle script control. No editor extension protection. No prompt injection defense. Single-source intel (Socket only). Agent calling `npm install` through a tool call is not guaranteed to route through the proxy.

**Gap Beekeeper fills:** Agent-layer interception (the hook handler and shim layer). Multi-source corroboration. Release-age enforcement. Cross-ecosystem coverage. Editor extension enforcement.

### MCP Gateways (ContextForge/IBM, MCPX/Lunar, Bifrost/Maxim, MintMCP, TrueFoundry)

Access control and audit at the MCP protocol layer. Features shared across the category: OAuth/RBAC for MCP endpoints, audit logging of tool invocations, rate limiting, protocol translation (HTTP/SSE/stdio), telemetry (OpenTelemetry).

**What none of them do:** No threat intelligence matching against supply chain catalogs. No package install enforcement. No release-age policy. No lifecycle script control. No editor extension catalog matching. No OS-level process correlation (all enforce at application middleware layer, not kernel). No prompt injection detection (most). No behavioral baseline. No sensitive-path enforcement. IBM ContextForge documentation explicitly states: "AGT enforces governance at the application middleware layer, not at the OS kernel level." Gateway governance and developer workstation supply chain enforcement are entirely separate problems.

**Gap Beekeeper fills:** Everything below the MCP protocol layer — the OS, the package managers, the editor extensions, the process tree.

### Microsoft Agent Governance Toolkit (April 2026)

Python-based, MIT license. Sub-0.1ms p99 policy engine. OWASP Agentic Top 10 compliance. Framework adapters for LangChain, CrewAI, Google ADK, OpenAI Agents SDK, LangGraph, PydanticAI. Plugin signing (Ed25519). RL governance for training. Compliance grading (EU AI Act, HIPAA, SOC2).

**What it does not do:** No supply chain catalog matching. No package install enforcement. No developer workstation OS-level monitoring. No editor extension protection. No Windows support focus. No Go binary — Python-first. Designed for enterprise agent deployments (containers, cloud), not individual developer workstations. No Bumblebee integration.

**Gap Beekeeper fills:** Individual developer workstation focus, supply chain enforcement, editor extension protection, Windows support, Go single binary model.

### RAMPART + Clarity (Microsoft, May 2026)

RAMPART: pytest-style safety testing framework for agents (design-time and CI, not runtime). Clarity: design documentation tool. Both are pre-deployment and testing tools. No runtime enforcement.

**Gap Beekeeper fills:** Runtime enforcement. Everything that happens after deployment, during an actual agent session.

---

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist in any credible agent safety harness. Missing these = dismissed as a toy or prototype.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| One-command install and hook wiring | The Nx Console attack proved that friction is the enemy of adoption. If setup takes more than 5 minutes, developers skip it. Every successful security tool in 2026 (Socket Firewall Free, pnpm v11 defaults) succeeds because setup is near-zero. | LOW | `beekeeper init` + `beekeeper hooks install --target claude-code` must Just Work. |
| Real-time package install enforcement | pnpm v11 ships minimumReleaseAge and allowBuilds by default. Developers using pnpm already have this. Extending it to agents and other package managers is the obvious next step. Without it, Beekeeper cannot claim to be a runtime harness. | MEDIUM | Must cover npm, pip, cargo, gem, composer, go. The shim layer is the fallback for agent shell-out. |
| Multi-source threat intelligence matching | After the TeamPCP wave hit Checkmarx, Trivy, SAP, TanStack, and Bitwarden simultaneously, developers do not trust single-source intel. Corroboration is table stakes for credibility. | MEDIUM | Bumblebee + OSV + Socket. Single-source warn, two-source enforce. |
| Audit log of all policy decisions | Every developer who installs a security tool expects to be able to answer "what did Beekeeper block and why?" after an incident. NDJSON local file is the minimum viable output. | LOW | NDJSON, 0600 permissions, auto-rotate. |
| Sensitive path protection | `.ssh/`, `.aws/`, `.env` protection is expected behavior for any tool that claims to guard against agent misuse. This is documented behavior in the OWASP Agentic Top 10 and in every MCP security checklist published in 2026. | LOW | Blocklist is well-understood. The complexity is in correctly classifying dynamic paths like project-local `.env.*`. |
| Claude Code hooks integration | As of May 2026, Claude Code is the dominant autonomous coding agent. PreToolUse hooks are the documented integration surface. Any agent safety tool that does not integrate with Claude Code hooks is not credible to the primary target audience. | LOW | `beekeeper hooks install --target claude-code` writes to `~/.claude/settings.json`. |
| Fail-closed default on crash or timeout | The developer community in 2026 knows about fail-open as a security anti-pattern. Falco, Tetragon, and enterprise EDR all fail-closed or fail-warn. A security tool that fails open is dismissed as insecure by any developer who has read the supply chain postmortems. | LOW | Hard timeout (500ms cap), crash → block. `fail_open` documented as reducing security. |
| Policy understandable by the developer | JSON policy files are the 2026 standard (OPA Rego, Cedar, YAML rules all appear in Microsoft's toolkit). Policy that cannot be read, tested, and version-controlled is not adopted. `beekeeper policy test` and `beekeeper policy validate` are expected. | MEDIUM | Declarative JSON, dry-run support, layered config (system → user → project → env → CLI). |
| Catalog freshness visible to the user | After observing that Bumblebee catalog entries are the key signal, users want to know "when was my threat intel last updated?" Stale intel equals false confidence. The TUI or `beekeeper diag` must surface catalog age prominently. | LOW | Per-source timestamps, red indicator on stale threshold. |
| Release-age policy for package installs | pnpm v11 ships 24h minimumReleaseAge by default. The Mini Shai-Hulud worm (detected in ~12 hours), the debug/chalk attack (resolved in ~2.5 hours), and the TanStack attack (caught within hours) all validate that 24h cooldown blocks the majority of worm campaigns. Developers who have read these postmortems expect this. | LOW | Default 24h, configurable per-ecosystem. The research is clear: this is near-zero false positive for stable packages, high blocking rate for attacks. |
| Lifecycle script enforcement | pnpm v11's `allowBuilds` model is documented and adopted. The npm CLI has the fewest consumer-side protections. A harness that doesn't enforce lifecycle scripts on npm is leaving the largest attack surface open. | MEDIUM | allowlist-only default. The complexity is cross-ecosystem: `preinstall`, `postinstall`, `install` differ per ecosystem. |
| SECURITY.md and responsible disclosure from day one | After TeamPCP specifically targeted security tools (Checkmarx, Trivy, Bitwarden), developers evaluate whether a security tool has its own security posture. An empty or missing SECURITY.md is a credibility killer. | LOW | Template exists; the process matters more than the document. |
| Sigstore-signed releases | The Nx Console payload contained full Sigstore integration and could generate valid provenance for malicious npm packages. Developers who absorbed that postmortem now look for Sigstore verification as proof that a security tool takes its own integrity seriously. | LOW | GitHub Actions OIDC → no long-lived key to steal. Documented in every release. |

### Differentiators (Competitive Advantage)

Features that set Beekeeper apart. Not universally expected yet, but high-signal differentiators that would drive adoption among security-conscious developers.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Corroboration-based catalog matching (2-source enforce, not union-of-bad) | Every competing tool uses union semantics: if any source says bad, block. Corroboration is the 2FA principle for threat intel — a compromised source cannot push a false enforcement rule without also compromising a second independent vendor. No existing tool in this space ships this. Research confidence: HIGH (verified against all competitors above). | MEDIUM | Per-ecosystem threshold configuration. Single source → warn. Two sources → enforce. Three sources → enforce + quarantine recommendation. |
| Editor extension enforcement across all three layers (agent CLI + file-watcher + scheduled scan) | All MCP gateways and agent frameworks completely miss the editor extension attack surface. Bumblebee catches existing installed extensions on scheduled scan. No tool intercepts agent-initiated `--install-extension` commands, watches extension directories for GUI/auto-update installs, and correlates release-age on new extensions. The Nx Console attack vector (11-18 minutes live, thousands of installs) is categorically unaddressed by every competing tool. | HIGH | Three enforcement layers must all be present to cover the full attack surface. Missing any one leaves a known gap from the Nx Console postmortem. |
| Sentry daemon: OS-level process correlation for extension-host attacks | Zero competing tools on the developer workstation market (as of May 2026) detect the Nx Console pattern at the process level: extension process descending from editor → reads credential paths → initiates outbound connection, within a 5-minute window. Microsoft's AGT explicitly does not do OS-level monitoring. Bumblebee does not do runtime detection. MCP gateways operate above the OS. This is genuinely novel for this market segment. | HIGH | fanotify + eBPF (Linux), eslogger (macOS, known limitations), ETW (Windows). The 7-day audit-only baseline period is essential to ship this without alarming false positive rates. |
| MCP gateway with threat intelligence enforcement inline | All existing MCP gateways (ContextForge, MCPX, Bifrost) provide access control and audit but zero threat intelligence matching. None of them know whether the package the agent is about to install was published 15 minutes ago or has two corroborating catalog matches. Beekeeper's gateway applies the full policy engine to every MCP tool call. This is the unique architectural combination: MCP proxy + supply chain intelligence. | HIGH | Requires the gateway to parse and understand tool call semantics for package installs, file access, and network egress. |
| LlamaFirewall sidecar integration (supervised, fail-closed) | PromptGuard 2 detects 97.5% of prompt injection attacks in Meta's dataset. No competing tool on the developer workstation integrates this at the hook level with automatic fail-closed behavior on sidecar crash. The fail-closed property is critical — a crashed or hung sidecar that fails open is worse than no sidecar (gives false confidence). | HIGH | Python IPC over named pipe or Unix socket. Supervised process lifecycle. Fail-closed default is the differentiating property. |
| Multi-agent parent-child policy inheritance | As developers chain agents (subagents, orchestrator-worker patterns), the threat model changes: a child agent may attempt operations the parent agent could not. The parent's policy context must propagate to children with explicit inheritance rules. No existing developer workstation tool addresses this. Microsoft's AGT does address it but in a container/cloud context, not on individual workstations. | HIGH | Requires agent identity and session context threading through the hook and gateway layers. |
| Behavioral baseline per project (counter-based, not ML) | ML-based anomaly detection is either resource-heavy or requires training data. Counter-based baselines with explicit thresholds are auditable, transparent, and require zero training period. Developers can read the baseline file and understand why a deviation fired. This is the right tradeoff for developer tooling in 2026 — not a black box. | MEDIUM | Rolling window frequency counters per (tool, target_pattern). Threshold in policy file. |
| Windows as first-class platform from day one | Bumblebee shipped macOS/Linux only at v0.1.1. Microsoft's AGT is Python-first and cloud-focused. No existing agent safety tool ships Windows support as a primary target. Given that a large fraction of developers use Windows and VS Code (the primary attack surface from the Nx Console incident), first-class Windows support is a real differentiator. | HIGH | ETW for Sentry. ReadDirectoryChangesW for file watcher. Go cross-compilation makes the binary straightforward; the platform-specific daemons are the complexity. |
| Self-defense: beekeeper-self catalog (recursive compromise detection) | After TeamPCP specifically compromised security vendors (Checkmarx, Trivy), security-conscious developers will ask "how do I know Beekeeper isn't compromised?" The beekeeper-self catalog — a separately hosted, separately signed feed of known-bad Beekeeper releases — answers this with a verifiable, auditable mechanism. No existing developer security tool ships self-compromise detection at this level. | MEDIUM | Separate hosting, separate signing key, consulted on every startup and catalog sync. Documents the honest limitation: single maintainer = single point of failure until a second maintainer joins. |
| Shim layer covering all package managers uniformly | Socket Firewall Free uses an HTTP proxy, which covers fetch-based installs but misses shell-out cases. pnpm's protections only apply when using pnpm. Beekeeper's shim layer (PATH symlinks for npm, pnpm, pip, cargo, go, gem, composer, npx, pipx) ensures coverage even when an agent shells out directly to a package manager without going through an MCP tool call. | MEDIUM | The shim is defense-in-depth, not the primary surface. Primary risk: an agent that detects shims and bypasses them (documented threat model item). |
| Catalog delta-triggered automatic re-scan | When a new threat intel entry lands in Bumblebee's upstream catalog, Beekeeper sweeps the local machine within the sync interval (default 1h), without any manual action. The detection latency for novel supply chain campaigns collapses from "hours after the user notices" to "minutes after catalog publishes." No competing tool ships this automatic catalog-delta-to-scan pipeline. | MEDIUM | Catalog sync daemon watches upstream delta, triggers Bumblebee with newly-cataloged ecosystems. This is the architectural differentiator that makes the continuous catalog sync valuable beyond mere freshness checking. |
| TUI dashboard (legibility without a web UI) | A security tool that only writes NDJSON is not evaluable at a glance. The TUI makes Beekeeper legible for humans without requiring a browser, a cloud account, or a separate dashboard server. `beekeeper dashboard` over SSH is a concrete workflow that no competing tool supports natively. Bubble Tea is mature and CGO-free. | MEDIUM | Single screen, event-driven refresh from audit log. Read-only by default, --admin for policy toggling. This is also a portfolio-quality deliverable for the solo developer. |

### Anti-Features (Things to Deliberately Not Build or Ship as Defaults)

Features that appear valuable but actively harm adoption, security posture, or the project's own integrity.

| Feature | Why Requested | Why Problematic | Better Approach |
|---------|---------------|-----------------|-----------------|
| Kernel-mode syscall blocking in v1 | Developers want true prevention, not just detection. Blocking is better than alerting. | Kernel-mode interceptors from a new, solo-maintained open source project are a privilege escalation attack surface on thousands of developer machines. The Sentry daemon is already privileged; adding syscall blocking requires macOS EndpointSecurity entitlement (uncertain for indie OSS) and introduces complexity that a solo developer cannot safely maintain, fuzz, and respond to when a CVE is found. Detection that compresses the breach window from hours to seconds is valuable without the maintenance burden of a kernel-mode enforcer. | Ship detect-and-alert in v1. v3 is the right target for eBPF-based syscall blocking, after the project has external security review, co-maintainers, and production operating experience. Document this explicitly in the threat model. |
| General-purpose rule engine (Falco-equivalent) | "Let me write my own rules for anything" is a power-user request. | Beekeeper's value is in its narrow, high-confidence, low-false-positive default ruleset. A general-purpose rule engine shifts the burden of rule quality to users, who are not security researchers. The community experience with Falco default rules (high false positive rates in developer environments) is the cautionary data point. False positives kill adoption faster than missing features do. Falco alerts on legitimate config updates; developers learn to ignore it. | Provide a narrow, tuned default ruleset. Allow users to add JSON-schema rules from the same schema. Explicitly do not provide a full expression language or hook system for arbitrary rule logic in v1. |
| Union-of-bad catalog semantics | "More coverage is better — if any source says bad, block it." | A compromised catalog source can push false enforcement entries. Single-source enforcement means one vendor compromise blocks legitimate packages for all Beekeeper users. The Sonatype 2026 report documented 20,362 false positives from vulnerability intelligence alone. Alert fatigue from over-blocking is the documented root cause of tool abandonment. pnpm's allowBuilds model failed to gain adoption in npm because its false positive rate on legitimate native modules was too high. | Corroboration-based semantics: single source warns, two sources enforce, three sources quarantine-recommend. Per-ecosystem threshold configuration. This is a unique differentiator, not a compromise. |
| `curl | sh` as the recommended install path | Lowest friction install. Every popular CLI tool offers it. | The Nx Console postmortem explicitly identified that the attacker poisoned the install path of trusted tools. A security tool distributed via an unverified shell script is self-defeating. The documentation should state this honestly with the specific risk. | `go install` with module checksum verification, or download + Sigstore verification. Document both. A `curl | sh` script may exist but must be labeled non-recommended with an explicit risk explanation. |
| Remote audit log sink enabled by default | "Centralize my audit trail in the cloud" is a legitimate enterprise need. | The audit log captures which files agents access, which tools they call, command fragments. An always-on remote sink leaks sensitive metadata from the developer's machine without explicit awareness. If the remote endpoint is compromised, the attacker has a reconnaissance feed for every developer running Beekeeper. | Remote sinks (syslog, OTLP, HTTPS POST) off by default. Opt-in with explicit configuration and a documented awareness message in the TUI ("audit data will leave this machine"). |
| Elevation required at first install | Maximum protection from the first run. | New OSS tool from a solo developer asking for admin/root on first install is a trust ask before the user has any basis for trusting the project. The historical pattern: developers either reject the tool or grant elevation while mentally de-trusting it (meaning they won't investigate alerts). Both outcomes are worse than opt-in elevation after the user has evaluated the unprivileged tier. | Unprivileged tier covers 80% of the value. `beekeeper protect install` is the explicit, documented, opt-in elevation path. `beekeeper init` may prompt to consider it after setup, but must not require it. |
| Sandbox/microVM orchestration in v1 | Running agents in isolated sandboxes is the ultimate defense. | Firecracker and gVisor integration requires significant kernel-version constraints, platform-specific work, and adds startup latency to every agent session. Solo developer, v1 scope. The value of sandboxing is real but the implementation complexity is incompatible with shipping a credible v1. | Document as the v3 roadmap. The policy engine + Sentry covers the practical threat classes without sandboxing overhead. |
| LlamaFirewall enabled by default | Maximum protection from the first run. | 400–800MB resident memory to hold PromptGuard 2 in RAM. On a developer's laptop with 16GB RAM and other tools running, this is a noticeable cost. Enabling it by default before the user knows its value and has opted into the resource cost will generate negative first impressions. Developers with resource-constrained machines will disable it, and a disabled feature is worse than an opt-in feature (gives false confidence). | Disabled by default. `beekeeper init` presents it as an option with the memory cost stated explicitly. Enable with a single config line. |
| Desktop GUI or web UI | More familiar to non-CLI users. | A web UI requires a local HTTP server, which is a new attack surface on the developer's machine. A desktop GUI requires Electron or similar, which bloats the single-binary model and adds CGO or cross-language complexity. The TUI via Bubble Tea is single-binary, no CGO required, SSH-accessible. Most developers who would use Beekeeper are comfortable with a TUI. | TUI only in v1. If a web UI becomes a clear adoption driver after v1, add it in v2 with explicit security review of the local HTTP server attack surface. |
| AI/ML-based anomaly classifier for tool calls | "Detect novel attacks without catalog entries" is the dream. | A local LLM-based anomaly classifier requires either a model large enough to be useful (memory cost) or a remote API call (privacy risk and latency). Counter-based behavioral baselines with explicit thresholds are auditable and have no model inference cost. ML anomaly detection has well-documented false positive problems in security contexts (see: every UEBA product that trained users to ignore alerts). | Counter-based baseline in v1 (transparent, auditable, low resource). Local LLM classifier is the v3 target, after operating experience establishes what the baseline misses. |
| Allowlisting via broad glob patterns as a default trust model | "Trust everything from publisher X" is convenient. | Broad publisher allowlists are exactly the trust model the TeamPCP attack exploited. `nrwl.*` was a trusted publisher until it wasn't. Allowlists should be specific and version-pinned, not broad. Shipping broad allowlist patterns as the default config trains users into a trust model that the threat landscape has directly invalidated. | Allowlists are specific by default (explicit package + version or explicit package + allowlist flag). Publisher-level allowlists require explicit user acknowledgment of the risk. |
| Shipping Sentry correlation rules without the 7-day audit-only baseline | Immediate protection from day one. | The Sentry rules fire on process lineage + file access + network connection patterns. On a fresh install, Beekeeper does not know the user's legitimate baseline (e.g., a developer whose workflow genuinely involves reading `~/.aws/credentials` in a known-good tool). False positives in the first week train users to dismiss alerts permanently. Falco's reputation for high false positives in dev environments is the cautionary example. | 7-day audit-only baseline period where all Sentry rules are observation-only. Rules promote to configured severity after baseline. Users can set baseline period to 0 (immediate) or "indefinite" (always audit-only). Ship conservative defaults. |
| Weighted corroboration (some sources count more than others) | "Bumblebee is more reliable than a user catalog — its match should count for more." | Empirically untested. Weighted systems are harder to reason about and harder to audit. If Source A counts as 1.5 votes and Source B as 0.5 votes, the effective threshold is now opaque. Transparent equal-vote corroboration is auditable: every decision can be traced to exactly which sources agreed. | Ship unweighted equal-vote corroboration in v1. Revisit weighting in v2 based on operating experience and false positive data per source. |

---

## Feature Dependencies

```
[Catalog sync daemon]
    └──requires──> [Multi-source catalog matching]
                       └──enables──> [Corroboration semantics]
                       └──enables──> [Catalog delta-triggered scan]

[Hook handler: beekeeper check]
    └──requires──> [Policy engine]
                       └──requires──> [Catalog matching]
                       └──requires──> [Release-age policy]
                       └──requires──> [Lifecycle script policy]
                       └──requires──> [Sensitive path policy]

[MCP Gateway daemon]
    └──requires──> [Policy engine]
    └──requires──> [Session token auth]
    └──enables──> [Cursor / Codex / Continue / OpenCode integrations]

[Sentry daemon]
    └──requires──> [beekeeper protect install (elevation)]
    └──requires──> [Behavioral baseline] (to tune false positive rates)
    └──enables──> [Extension-host credential cluster rule]
    └──enables──> [Exfil signature fusion rule]
    └──enhances──> [Editor extension enforcement] (adds process correlation to file-watcher)

[Editor extension enforcement]
    └──requires──> [File-watcher daemon] (for GUI installs)
    └──requires──> [Catalog matching] (for catalog hits)
    └──requires──> [Release-age policy] (for new-extension age check)
    └──enhances──> [Sentry daemon] (fresh-extension behavior correlation rule)

[LlamaFirewall sidecar]
    └──requires──> [Hook handler] (to route tool outputs for scanning)
    └──requires──> [IPC layer: Unix socket / named pipe]
    └──conflicts──> [Resource-constrained environments] (400-800MB RAM)

[TUI dashboard]
    └──requires──> [NDJSON audit log] (reads from audit log, no privileged channel)
    └──requires──> [Policy engine] (for active policies panel)
    └──enhances──> [Sentry daemon] (Sentry alerts panel)
    └──enhances──> [Catalog sync] (freshness panel)
    └──enhances──> [Quarantine workflow] (quarantine panel)

[Shim layer]
    └──requires──> [Hook handler / beekeeper check] (shims delegate to check)
    └──conflicts──> [Agents that detect shims and bypass them] (documented gap)

[Quarantine workflow]
    └──requires──> [Catalog matching] (trigger condition)
    └──requires──> [File-watcher daemon] (for extension quarantine)
    └──requires──> [TUI dashboard] (restore/purge UI)

[Behavioral baseline]
    └──requires──> [Audit log] (baselines read from policy decision history)
    └──enhances──> [Hook handler] (adds deviation-based warnings)
    └──enhances──> [Sentry daemon] (reduces false positive rate on fresh install)

[beekeeper-self catalog]
    └──requires──> [Catalog sync daemon]
    └──requires──> [Separate hosting + signing key]
    └──enhances──> [Self-defense posture] (recursive compromise detection)

[Multi-turn exfiltration detection]
    └──requires──> [Hook handler: PostToolUse]
    └──requires──> [Rolling output buffer per session]
    └──enhances──> [Output filtering] (adds statistical signal to pattern matching)
```

### Dependency Notes

- **Sentry requires elevation:** Protected mode is opt-in. The unprivileged tier (hooks, gateway, file-watcher, catalog sync) delivers meaningful coverage without Sentry. Sentry adds the class of threats (extension-host attacks) that bypass the agent loop entirely.
- **TUI requires audit log but not daemons:** The TUI is a read consumer of the NDJSON audit log. It can display historical data even when daemons are not running. This makes it useful for investigation without requiring all components to be live.
- **Shim layer conflicts with agents that detect PATH manipulation:** Documented in the threat model. The shim is defense-in-depth, not a primary enforcement surface. If an agent detects and bypasses the shim, the gateway and hooks are the primary surfaces.
- **LlamaFirewall conflicts with resource-constrained machines:** 400–800MB is a real cost. The optional/disabled-by-default design is a consequence of this conflict.
- **Corroboration semantics require multiple catalog sources:** Single-source Beekeeper (Bumblebee only) loses the 2FA property. OSV and Socket must be present for the corroboration model to provide its claimed security guarantee. The v0.1.0 milestone ships single-source with this limitation documented; v0.3.0 adds OSV and Socket for full corroboration.
- **7-day Sentry baseline depends on audit log:** The baseline learns from observing policy decisions over time. Fresh installs have no baseline, which is why audit-only mode for the first 7 days is essential — it collects data without firing false positives.

---

## MVP Definition

### Launch With (v0.1.0)

The minimum that is personally useful to the developer building it. Dogfoods on a real machine from day one.

- [x] `beekeeper check` hook handler (reads tool call from stdin, policy eval, exits allow/block, sub-100ms target)
- [x] `beekeeper hooks install --target claude-code` writes to `~/.claude/settings.json`
- [x] Bumblebee `threat_intel/` catalog sync and matching (single-source, warn on single match)
- [x] Release-age policy for npm and PyPI (24h default)
- [x] Basic NDJSON audit log to local file (0600 permissions)
- [x] `beekeeper catalogs sync`, `beekeeper audit tail`
- [x] Reproducible builds, Sigstore signing, pinned deps, SECURITY.md (self-defense minimum)
- [x] Fail-closed default with documented resource limits

Why this minimum: protects the builder's machine. Validates the hook integration pattern. Proves the catalog matching architecture works. Generates real audit data. Does not require elevation.

### Add After Validation (v0.3.0–v0.6.0)

- [ ] OSV and Socket catalog sources (enables corroboration semantics) — add when single-source coverage proves insufficient against real threats
- [ ] Lifecycle script policy for npm, pip, cargo, gem, composer — add when an agent-triggered lifecycle script exploit occurs or is reported
- [ ] Editor extension CLI parsing + file-watcher daemon — add after v0.1.0 validates hook architecture; this is the next highest-risk surface post-Nx Console
- [ ] Cursor and Codex CLI hook integration — add when user demand confirms multi-agent coverage matters
- [ ] Sensitive path policy (blocklist for ~/.ssh/, ~/.aws/, .env) — add in v0.3.0; low complexity, high visibility
- [ ] MCP gateway daemon — add when users report agents that cannot be covered by hook integration alone
- [ ] Sentry daemon (Linux fanotify + eBPF) — add in v0.6.0 after hook architecture is proven; requires 7-day baseline period before enabling rules
- [ ] TUI dashboard — add in v1.0.0; the legibility that makes Beekeeper evaluable at a glance

### Future Consideration (v2+)

- [ ] SARIF export for security team workflows — defer until enterprise/team use cases are confirmed
- [ ] Cross-host correlation for team deployments — defer until solo developer use case is proven
- [ ] macOS EndpointSecurity entitlement — defer; Apple entitlement process is uncertain for indie OSS; eslogger covers v1
- [ ] Local LLM-based anomaly classifier — defer to v3; counter-based baseline is sufficient for v1
- [ ] Sandbox/microVM orchestration — defer to v3; threat model shows hooks + Sentry cover the practical threat classes
- [ ] eBPF-based syscall blocking (true prevention) — defer to v3; detection with 5-minute window compression is the v1 value proposition
- [ ] SLSA Level 3 provenance — v0.9.0 target per the phasing plan; Sigstore covers v0.1.0

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| `beekeeper check` hook handler | HIGH | LOW | P1 |
| Claude Code hooks integration | HIGH | LOW | P1 |
| Bumblebee catalog matching | HIGH | LOW | P1 |
| Release-age policy (npm, PyPI) | HIGH | LOW | P1 |
| NDJSON audit log | HIGH | LOW | P1 |
| Fail-closed default | HIGH | LOW | P1 |
| Sigstore signing + reproducible builds | HIGH | LOW | P1 |
| SECURITY.md + disclosure process | HIGH | LOW | P1 |
| OSV + Socket catalog sources | HIGH | MEDIUM | P1 |
| Corroboration semantics | HIGH | MEDIUM | P1 |
| Sensitive path policy | HIGH | LOW | P1 |
| Lifecycle script policy (cross-ecosystem) | HIGH | MEDIUM | P1 |
| Editor extension CLI enforcement | HIGH | MEDIUM | P1 |
| File-watcher daemon (extension dirs) | HIGH | MEDIUM | P1 |
| Shim layer (PATH wrappers) | MEDIUM | MEDIUM | P2 |
| MCP gateway daemon | HIGH | HIGH | P2 |
| Behavioral baseline (counter-based) | MEDIUM | MEDIUM | P2 |
| Catalog delta-triggered re-scan | HIGH | MEDIUM | P2 |
| Sentry daemon (Linux) | HIGH | HIGH | P2 |
| Sentry daemon (macOS, eslogger) | HIGH | HIGH | P2 |
| Sentry daemon (Windows, ETW) | HIGH | HIGH | P2 |
| LlamaFirewall sidecar integration | MEDIUM | HIGH | P2 |
| TUI dashboard | HIGH | MEDIUM | P2 |
| Policy as code (test + validate) | HIGH | MEDIUM | P2 |
| Multi-agent parent-child policy | MEDIUM | HIGH | P2 |
| beekeeper-self catalog | HIGH | MEDIUM | P2 |
| Output filtering + multi-turn exfil detection | MEDIUM | MEDIUM | P2 |
| ContextForge/MCPGuard policy-plugin mode | LOW | HIGH | P3 |
| SARIF export | LOW | LOW | P3 |
| Cross-host team correlation | LOW | HIGH | P3 |
| Local LLM anomaly classifier | LOW | HIGH | P3 |
| Sandbox/microVM orchestration | LOW | HIGH | P3 |

**Priority key:**
- P1: Must have, ship in v0.1.0–v0.3.0
- P2: Should have, ship in v0.3.0–v1.0.0
- P3: Nice to have, v2+ consideration

---

## Competitor Feature Analysis

| Feature | Bumblebee | LlamaFirewall | Socket Firewall Free | MCP Gateways (ContextForge/MCPX/Bifrost) | Microsoft AGT | Beekeeper |
|---------|-----------|---------------|----------------------|-------------------------------------------|---------------|-----------|
| Supply chain catalog matching | Yes (read-only) | No | Yes (Socket only, human-reviewed) | No | Partial (plugin signing only) | Yes (multi-source, corroboration) |
| Runtime enforcement (block, not just detect) | No | No | Yes (package fetch layer) | Partial (MCP protocol layer only) | Partial (app middleware layer) | Yes (hook + gateway + shim) |
| Agent hook integration | No | No | No | Yes (MCP only) | Yes (framework adapters) | Yes (PreToolUse + PostToolUse) |
| Release-age policy | No | No | No | No | No | Yes (24h default, all ecosystems) |
| Lifecycle script enforcement | No | No | No | No | No | Yes (allowlist-only, cross-ecosystem) |
| Editor extension enforcement | Scan (post-install) | No | No | No | No | Yes (3 layers: CLI + file-watcher + scan) |
| OS-level process correlation | No | No | No | No | No (app layer only) | Yes (Sentry, opt-in) |
| Prompt injection detection | No | Yes (PromptGuard 2) | No | No | Yes (semantic classifier) | Yes (LlamaFirewall sidecar) |
| Sensitive path policy | No | No | No | Partial (RBAC on tools) | Partial | Yes |
| Behavioral baseline | No | No | No | No | No | Yes (counter-based) |
| Multi-source corroboration | No (single source) | N/A | No (single source) | No | No | Yes (2FA for threat intel) |
| Windows support | No (macOS/Linux only) | Yes | Yes | Varies (mostly cloud) | Yes (Azure-focused) | Yes (primary dev platform) |
| Single static binary | Yes | No (Python) | No (Node.js proxy) | Varies | No (Python) | Yes (Go) |
| TUI dashboard | No | No | No | No (web UI) | No | Yes (Bubble Tea) |
| Policy as code (testable) | No | No | No | Partial (YAML/OPA in AGT) | Yes (YAML/OPA/Cedar) | Yes (declarative JSON) |
| Self-compromise detection | No | No | No | No | No | Yes (beekeeper-self catalog) |
| Audit log (NDJSON, local) | Yes | No | No | Partial (varies by gateway) | No | Yes (schema-compatible with Bumblebee) |

---

## Sentry Process Correlation Rules: Documentation Quality Assessment

This section answers Question 5 from the research brief: what's well-documented vs. novel vs. risky to ship as defaults.

### Well-Documented (HIGH confidence, safe to ship as defaults)

**Extension-host credential cluster rule.** Process descended from editor reads 2+ sensitive-path files in 60 seconds. This is essentially the Nx Console attack signature. The Nx postmortem provides direct empirical validation. ARMO's eBPF detection research confirms process lineage analysis for parent-child relationships is the correct technique. False positive risk: LOW when the sensitive-path watchlist is specific. A legitimate developer tool (e.g., 1Password CLI reading `~/.config/op/`) should be allowlisted, not suppressed by rule tuning.

**Exfil signature fusion rule.** Sensitive path read + outbound network connection + same process descending from recently-installed extension, within 5 minutes. This is the exact Nx Console attack pattern in structured form. The attack executed within seconds of the developer opening a workspace. The 5-minute correlation window is conservative — the actual attack occurred in seconds. Well-documented source material. Safe to ship as default.

### Novel (MEDIUM confidence, requires baseline period before promoting to enforcement)

**Extension-host phone-home rule.** Process descended from editor initiates outbound connection to domain not in allowlist within 10 minutes of extension activation. The concept is sound; the implementation challenge is the allowlist. Legitimate extensions make network calls (telemetry, update checks, API calls). The allowlist must be pre-populated with known-good domains or the false positive rate will be unacceptable. Falco's experience with similar "new outbound connection" rules in production environments shows high noise until environment-specific tuning is applied. Ship with 7-day audit-only baseline. Do not promote to enforcement without an initial allowlist of common legitimate extension domains.

**Extension-host credential CLI burst rule.** Process descended from editor spawns 2+ known credential CLIs within 60 seconds. The attack behavior is documented in the Nx Console postmortem (the payload specifically targeted `gh auth`, AWS credentials, npm tokens, 1Password). The challenge: a developer running a setup script legitimately might invoke multiple credential CLIs in rapid succession. The 60-second window and 2-occurrence threshold are tunable starting points, not validated ground truth. Ship as audit-only default, promote to warn after baseline.

**Fresh-extension behavior correlation rule.** Any of the above fire AND a Bumblebee-tracked extension was installed/updated within 30 minutes. This is the highest-confidence compound rule because it requires two independent signals to fire. The cross-correlation with Bumblebee inventory is the differentiating element. Risk: Bumblebee inventory must be current for the correlation to be meaningful. If the catalog sync is stale, the rule degrades gracefully (it still fires on the process pattern; it just loses the catalog attribution). Safe to ship as default with the compound condition.

### Risky (LOW confidence, do NOT ship as enforcement defaults in v1)

**Network egress allowlist enforcement at the Sentry level.** Blocking outbound connections from editor processes based on domain allowlists requires either a very permissive allowlist (low value) or aggressive tuning (high false positive rate that breaks legitimate editor features). VS Code extensions legitimately call dozens of domains: Marketplace APIs, telemetry, language server updates, extension-specific APIs. Any approach narrower than "alert on unexpected new domains" requires per-developer baseline learning that is not feasible at v1 ship. Ship as audit-only, never as enforcement default.

**Agent process identity via process lineage only.** Inferring that a process is "the agent" from process lineage is reliable when the agent is a known binary (Claude Code, Cursor). It becomes unreliable when agents are invoked from scripts, CI, or non-standard paths. Misidentifying a non-agent process as an agent means applying agent-specific rules incorrectly. The safe approach: identify agents by path allowlist (documented agent binary locations), not purely by lineage.

**Multi-turn exfiltration detection via entropy scoring.** Rolling entropy on output buffers sounds good but generates significant false positives on legitimate high-entropy content: base64-encoded images, compressed payloads, cryptographic outputs from legitimate tools. Entropy scoring alone is not a sufficient signal. This should be paired with pattern matching (known API key prefixes, JWT format, private key headers) rather than used standalone. Ship the pattern matching; don't ship raw entropy thresholds as enforcement defaults.

---

## Sources

- Perplexity Bumblebee GitHub: https://github.com/perplexityai/bumblebee
- LlamaFirewall official docs: https://meta-llama.github.io/PurpleLlama/LlamaFirewall/
- LlamaFirewall paper: https://arxiv.org/abs/2505.03574
- Socket Firewall Free docs: https://docs.socket.dev/docs/socket-firewall-free
- TrueFoundry MCP security tools overview: https://www.truefoundry.com/blog/best-mcp-security-tools
- Integrate.io MCP gateways comparison: https://www.integrate.io/blog/best-mcp-gateways-and-ai-agent-security-tools/
- Lunar.dev open source MCP gateways: https://www.lunar.dev/post/the-best-open-source-mcp-gateways-in-2026
- DX Heroes MCP governance landscape: https://dxheroes.io/insights/mcp-governance-landscape-early-2026
- IBM ContextForge GitHub: https://github.com/IBM/mcp-context-forge
- Microsoft Agent Governance Toolkit GitHub: https://github.com/microsoft/agent-governance-toolkit
- Microsoft AGT announcement: https://opensource.microsoft.com/blog/2026/04/02/introducing-the-agent-governance-toolkit-open-source-runtime-security-for-ai-agents/
- Microsoft RAMPART + Clarity: https://www.microsoft.com/en-us/security/blog/2026/05/20/introducing-rampart-and-clarity-open-source-tools-to-bring-safety-into-agent-development-workflow/
- Nx Console compromise (StepSecurity): https://www.stepsecurity.io/blog/nx-console-vs-code-extension-compromised
- Nx Console compromise (Hacker News): https://thehackernews.com/2026/05/compromised-nx-console-18950-targeted.html
- Nx Console postmortem (Nx Blog): https://nx.dev/blog/nx-console-v18-95-0-postmortem
- Nx Console GHSA advisory: https://github.com/nrwl/nx-console/security/advisories/GHSA-c9j4-9m59-847w
- Aikido: GitHub breached via VS Code extension: https://www.aikido.dev/blog/github-breached-vs-code-extension
- pnpm supply chain security: https://pnpm.io/supply-chain-security
- pnpm 11 minimumReleaseAge announcement: https://cybersecuritynews.com/pnpm-11-turns-on-minimum-release-age/
- Mondoo npm supply chain 2026: https://mondoo.com/blog/npm-supply-chain-security-package-manager-defenses-2026
- Sonatype 2026 Software Supply Chain Report: https://www.sonatype.com/state-of-the-software-supply-chain/2026/vulnerability-management
- MCP security supply chain crisis: https://cyberstrategyinstitute.com/mcp-security-supply-chain-crisis/
- ARMO AI agent escape detection: https://www.armosec.io/blog/ai-agent-escape-detection/
- Palo Alto eBPF AI security: https://www.paloaltonetworks.com/blog/network-security/beginners-guide-to-ai-security-with-ebpf/
- Claude Code hooks documentation: https://code.claude.com/docs/en/agent-sdk/hooks
- eslogger macOS documentation: https://keith.github.io/xcode-man-pages/eslogger.1.html
- Cybereason blue teaming with eslogger: https://www.cybereason.com/blog/blue-teaming-on-macos-with-eslogger
- OpenClaw agent attack surface (VentureBeat): https://venturebeat.com/security/one-command-open-source-repo-ai-agent-backdoor-openclaw-supply-chain-scanner
- OWASP Agentic Top 10 compliance (Microsoft): https://github.com/microsoft/agent-governance-toolkit/blob/main/docs/OWASP-COMPLIANCE.md
- 2026 OSSRA Report (Black Duck): https://www.blackduck.com/blog/open-source-trends-ossra-report.html

---
*Feature research for: agent runtime safety harness (Beekeeper)*
*Researched: 2026-05-26*
