# Pitfalls Research

**Domain:** Agent runtime safety harness / security daemon (Go, eBPF, ETW, eslogger, MCP proxy)
**Researched:** 2026-05-26
**Confidence:** HIGH (most pitfalls verified against official docs, real-world tool postmortems, and community issue trackers)

---

## Critical Pitfalls

### Pitfall 1: Hook Handler Cold-Start Latency Compounds Under Heavy Sessions

**What goes wrong:**

The PRD targets sub-100ms p99 for `beekeeper check`. A fresh Go process startup is documented at ~10-30ms before any policy evaluation runs. That looks safe with single-digit milliseconds of evaluation cost. But Claude Code hooks spawn a subprocess per tool call, and at 50-200 tool calls per hour in a heavy coding session, the compound effect matters less for throughput than for perceived latency. The real failure mode is that `beekeeper check` is on the hot path for every single agent action — file reads, shell commands, MCP calls — meaning any regression above the budget blocks the agent's next step.

The documented worst case from real Claude Code hook deployments: a project with 11+ hooks across 9 lifecycle events caused 13-16 seconds overhead per prompt interaction (not per tool call — per prompt). The root cause was sequential Node.js subprocess spawning. Go starts faster than Node, but the architecture trap is the same: each hook invocation is a fresh process, and if policy evaluation touches disk (catalog lookup, baseline counters), OS file cache misses add latency that won't appear in synthetic benchmarks.

The second failure mode is the "fast in dev, slow in prod" trap. A developer benchmarks `beekeeper check` on a warm SSD with a small catalog. The actual user has a cold catalog cache (first run after boot), a spinning disk under I/O load from the agent's `npm install`, and a LlamaFirewall sidecar that's just been restarted. The p99 blows out. The agent stalls visibly. The user disables the hook.

**Why it happens:**

The sub-100ms target is achievable for the happy path — warm binary, small catalog in OS cache, no sidecar. It is not achievable without care for the cold-start case. Go's process startup adds 10-30ms before `main()` runs. JSON parsing of the tool call adds a few ms. The catalog lookup against an in-memory structure is fast, but loading the catalog from disk on first invocation adds real latency. The PRD correctly identifies this as an empirical question ("Latency budget under load... is unknown until prototyped"), but the pitfall is treating the p50 case as the p99 target.

**How to avoid:**

- Implement the catalog and policy engine as in-memory structures loaded once at process start, not re-parsed per invocation. Keep a persistent daemon (`beekeeper sentry` is already planned; the same pattern can serve as a policy cache daemon) that `beekeeper check` talks to via Unix domain socket / Windows named pipe IPC instead of loading state fresh each time.
- Benchmark `beekeeper check` specifically under the "cold OS file cache, large catalog, sidecar starting" case from v0.1.0. Set the latency regression gate at 100ms p95, not p50.
- The hook handler's stdin read has a 1MB cap (per PRD); enforce it early to avoid spending latency on oversized payloads before the bounds check kicks in.
- Test with a realistic catalog size. Bumblebee's `threat_intel/` is growing rapidly post-May 2026 campaigns. A catalog with 5,000 entries behaves differently than one with 50.
- Consider a persistent socket daemon as the v0.3.0 optimization path: `beekeeper check` sends a short IPC message to a warm `beekeeper daemon` process and gets a response. Cold start cost collapses to IPC overhead (~1ms). This is the pattern used by `gopls`, `rust-analyzer`, and LSP servers generally.

**Warning signs:**

- p99 latency in `beekeeper diag` above 80ms in a warm-cache test
- User bug reports: "agent feels slow" or "Claude keeps waiting before each tool"
- Integration tests that measure only average latency, not p99
- Benchmark environment using in-memory catalog, production environment loading from disk

**Phase to address:**

v0.1.0: Establish the latency benchmark baseline with realistic catalog sizes and document p50/p95/p99 in `beekeeper diag` output. v0.3.0: Implement persistent IPC daemon if cold-start latency exceeds budget under realistic loads.

---

### Pitfall 2: eBPF Kernel Version Fragmentation Silently Degrades Sentry

**What goes wrong:**

The PRD specifies fanotify + eBPF (`cilium/ebpf`) for Linux Sentry. This combination has hard kernel version dependencies that vary across features:

- `CAP_BPF` (granular eBPF capability) requires kernel **5.8+**. On 5.4 and earlier, you need `CAP_SYS_ADMIN` — a significantly broader privilege that many hardened systems deny.
- `FAN_MARK_FILESYSTEM` (efficient whole-filesystem fanotify mark, avoiding per-directory walking) requires kernel **4.20+**.
- `FAN_CREATE` / `FAN_DELETE` events for new file detection require kernel **5.1+**.
- eBPF CO-RE (Compile Once, Run Everywhere, needed for portable programs without headers) requires kernel **5.4+** with BTF enabled.
- Ring buffers (`BPF_MAP_TYPE_RINGBUF`) — more efficient than perf buffers for high-event-rate Sentry — require kernel **5.8+**.
- The `BPFize fanotify` integration (native BPF hooks on fanotify struct_ops) is still in-kernel-development as of 2025-2026.

The fragmentation risk: Ubuntu 20.04 LTS ships kernel 5.4 by default (5.15 with HWE), Debian 11 ships 5.10, CentOS 8 ships 4.18. A developer on Ubuntu 20.04 without HWE kernel gets `CAP_SYS_ADMIN` requirement (broader privilege) and no ring buffers. The Sentry daemon may fail to load eBPF programs entirely and produce no error visible to the user.

fanotify has an additional limitation that never appears in synthetic tests: it does **not** report file accesses via `mmap()`, `msync()`, or `munmap()`. A malicious extension reading credential files via memory-mapped I/O would be invisible to fanotify-based monitoring.

**Why it happens:**

eBPF's feature matrix is a function of kernel version, kernel compile-time config (`CONFIG_BPF`, `CONFIG_BPF_SYSCALL`, `CONFIG_FANOTIFY`, `CONFIG_FANOTIFY_ACCESS_PERMISSIONS`), and distro-specific patches. The `cilium/ebpf` library provides good abstractions but cannot conjure missing kernel features. Projects using eBPF often test on their own kernel (likely recent Ubuntu or Fedora) and only discover fragmentation when users on older LTS kernels file issues.

**How to avoid:**

- Implement capability probing at `beekeeper protect install` time. Before loading any eBPF programs, check kernel version, BTF availability (`/sys/kernel/btf/vmlinux`), and specific required features. If the check fails, report exactly which features are missing and which kernel version would enable them.
- Implement graceful degradation tiers:
  - Tier 1 (kernel 5.8+, BTF): Full eBPF + fanotify, ring buffers. Full Sentry capability.
  - Tier 2 (kernel 5.1-5.7): fanotify with perf buffers, CAP_SYS_ADMIN required. Functional but noisier.
  - Tier 3 (kernel 4.20-5.0): fanotify only, no eBPF process events. File monitoring only, no network correlation.
  - Tier 4 (below 4.20): inotify fallback, no fanotify permission events, limited coverage. Surface in `beekeeper diag` as "degraded coverage."
- Document the mmap gap explicitly in the threat model: Sentry's file monitoring will not detect credential reads via memory-mapped I/O. This is a known limitation of the fanotify API, not a Beekeeper bug.
- Add kernel version to `beekeeper protect status` output and `beekeeper diag` so users can see what tier they're on.

**Warning signs:**

- CI tests passing on ubuntu-latest (typically 6.x) but Sentry failing silently on user-reported older kernels
- `beekeeper protect install` succeeding but Sentry generating zero events in the audit log
- User reports of Sentry rules never firing despite configured sensitive paths being accessed
- Missing `/sys/kernel/btf/vmlinux` (BTF not available, CO-RE will fail)

**Phase to address:**

v0.6.0 (Sentry Linux implementation): Build capability probing and degradation tiers from the first commit of Sentry. Do not build for the happy path first and add degradation later — by then the code assumes kernel 5.8+ throughout.

---

### Pitfall 3: Windows ETW Event Loss Under Load Creates Coverage Blind Spots

**What goes wrong:**

The PRD notes that ETW rate-limits under load and that Beekeeper will surface this in `beekeeper diag`. This is the right posture, but the practical impact is worse than "some events get dropped." ETW event loss under load is an architectural property of the system, not a transient glitch.

ETW uses a fixed-size circular buffer per session. When the consumer (Beekeeper's Sentry daemon) cannot drain the buffer as fast as the provider writes events, the oldest events are overwritten. The events that get dropped are the high-rate, low-value ones (file access in a hot loop from `npm install`) — but also potentially the high-value, low-frequency ones (credential file read from an extension host process) if the drop timing is unlucky.

Additional ETW constraints for Beekeeper:

- Windows supports only **one active NT Kernel Logger session** at any time. If Windows Defender, Carbon Black, or another EDR is already consuming the NT Kernel Logger, Beekeeper cannot also subscribe without coordination. This is not a rate-limit; it is an architectural exclusion. The `Microsoft-Windows-Threat-Intelligence` provider (which exposes process injection events) has similar single-consumer restrictions.
- Per-session buffer sizes are configurable (`MaximumBuffers`, `BufferSize`) but require the Sentry daemon to set them at session creation. Default values are often too small for high-event-rate workloads.
- ETW is also a documented EDR blind-spot class: attackers can patch ETW in-process (etw.dll) to suppress events from their own process. This is not a Beekeeper bug but limits what Sentry can detect when an adversary is in the same process as the legitimate application (e.g., a compromised dependency loaded into the extension host process).

**Why it happens:**

ETW was designed as a diagnostic tracing mechanism, not a real-time security event bus. The session model, buffer architecture, and single-consumer constraints were designed for profiling workloads, not adversarial environments. Security tools have adopted ETW because it is the canonical Windows kernel event source, but its diagnostic-tool heritage creates structural gaps.

**How to avoid:**

- At `beekeeper protect install` on Windows, probe for existing NT Kernel Logger consumers and document which providers will be shared vs. exclusive.
- Set explicit `MinimumBuffers` and `MaximumBuffers` values when creating the ETW session, sized for worst-case `npm install` event rates (empirically measurable in v0.9.0 development).
- Implement an event loss counter in the Sentry daemon using ETW's built-in `EventsLost` session statistic. Surface this counter in `beekeeper diag` and in the TUI system health panel with a clear "detection coverage degraded" warning when it exceeds a configurable threshold.
- Document the single-consumer NT Kernel Logger constraint in the user-facing installation guide. If another EDR is present, recommend checking for provider conflicts with `logman query -ets`.
- For the extension-host credential cluster rule specifically, fall back to polling-based sensitive-path monitoring (ReadDirectoryChangesW on the credential directories) as a complement to ETW process events on Windows. This survives ETW consumer conflicts.

**Warning signs:**

- `EventsLost` counter in ETW session statistics above zero during `npm install`
- Sentry audit log shows gap in process events during file-heavy operations
- `beekeeper protect install` on a machine with an existing EDR failing silently or producing no Sentry events
- TUI showing "Sentry active" but zero process events in a 5-minute window during active development

**Phase to address:**

v0.9.0 (Windows Sentry implementation). Set the `EventsLost` surfacing requirement as a release gate criterion, not a backlog item.

---

### Pitfall 4: eslogger on macOS Cannot Catch In-Memory Credential Theft

**What goes wrong:**

The PRD correctly limits v1 macOS Sentry to eslogger and documents this as a known gap. The pitfall is underestimating how specific the gap is relative to the Nx Console attack class.

eslogger exposes 82 ES events including process exec, file open/read, socket connections, and launch item creation. These events are the correct signals for the Nx Console attack pattern (credential file read + outbound connection from an editor-descended process). eslogger **would** have generated events covering the Nx Console compromise:

- `ES_EVENT_TYPE_NOTIFY_OPEN` for reads of `~/.config/gh/hosts.yml` (GitHub token)
- `ES_EVENT_TYPE_NOTIFY_CREATE` for the persistence artifacts written to disk
- Socket-level events for the HTTPS exfiltration connections

However, eslogger has specific gaps that matter for future attack variants:

1. **In-memory access via Cocoa APIs**: Screenshot capture via `NSScreen` or `CGWindowListCreateImage` does not generate file events. A malicious extension doing screen scraping to harvest secrets from other windows is invisible to eslogger.
2. **Keychain access**: Reading from the macOS Keychain via Security framework APIs does not necessarily generate file events in the locations eslogger monitors. Keychain access events are a separate API surface requiring the `EndpointSecurity` entitlement directly.
3. **Memory injection**: Code injected into an existing process via `task_for_pid` or similar Mach port exploitation has no distinct ES event. The injected code inherits the parent process identity, so eslogger would attribute reads to the legitimate parent (VS Code extension host) rather than the injected payload.
4. **Performance**: eslogger outputs complex JSON to stdout; at high event rates, the consumer (Beekeeper Sentry on macOS) must drain stdout faster than events arrive or events are lost. This is the same class of problem as ETW event loss but in the pipe buffer.

The eslogger documentation gap is also a pitfall: "many field names are self-explanatory, but others are not." Building correct event parsers requires significant trial and error on a real macOS system.

**Why it happens:**

eslogger is a diagnostic tool that streams EndpointSecurity events to stdout. It does not expose the full EndpointSecurity framework capability (which requires the `com.apple.developer.endpoint-security.client` entitlement from Apple). The gap is not a bug; it is an intentional scoping of what the entitlement-free path can access.

**How to avoid:**

- In `beekeeper protect status` on macOS, explicitly list the monitoring gaps (Keychain, in-memory, injected code) so users understand the coverage.
- Handle eslogger's stdout pipe carefully: use a buffered reader with explicit backpressure signaling. If the pipe buffer fills, Beekeeper Sentry should log a warning and increment a coverage-degraded counter.
- For the Nx Console attack class specifically: eslogger coverage is adequate. The 18-minute window attack would generate `ES_EVENT_TYPE_NOTIFY_OPEN` events on `~/.config/gh/hosts.yml` and socket events for the exfiltration. The correlation rule would fire.
- Plan the EndpointSecurity entitlement application for v2 with realistic timeline expectations: Apple's developer entitlement review is weeks to months for new applicants, and approval is not guaranteed for OSS tools without commercial backing. Do not block v1 on this.
- Test the eslogger parser against actual macOS event output (not synthetic JSON) in CI using `macos-latest`. Field names and structures change across macOS versions.

**Warning signs:**

- eslogger integration tests that parse synthetic JSON rather than actual eslogger output
- Pipe buffer overflow errors in the macOS Sentry implementation under high event rate
- Parser failures on event types added in newer macOS versions
- Community reports of Sentry missing events on macOS that were visible in the audit log on Linux

**Phase to address:**

v0.9.0 (macOS Sentry implementation). Add macOS-specific coverage gap documentation to `beekeeper protect status` output as a release requirement.

---

### Pitfall 5: Catalog Corroboration 2-of-N Is Vulnerable to Coordinated Catalog Poisoning via DoS

**What goes wrong:**

The corroboration model (1 source = warn, 2 sources = enforce, 3 sources = enforce + quarantine) is a well-reasoned defense against a compromised single catalog source. The attack surface it does not address: an attacker who poisons two sources with false-positive entries targeting a legitimate package to trigger enforcement against users of that package.

Two realistic attack vectors:

1. **False-positive poisoning for disruption**: An attacker compromises one catalog source and injects entries for widely-used, legitimate packages (React, Lodash, FastAPI). Single-source = warn. If the same attacker or a separate campaign simultaneously poisons a second source, the threshold crosses to enforce. Users across all Beekeeper deployments are blocked from installing those packages. The attacker doesn't need to compromise the third source for the attack to be disruptive.

2. **Catalog source attrition**: The corroboration model's implicit assumption is that all three active catalog sources (Bumblebee, OSV, Socket) are independently operated. If two of the three can be influenced by the same threat actor, the 2FA metaphor breaks down. This is not hypothetical: TeamPCP compromised Checkmarx, Trivy, and Bitwarden simultaneously — the very pattern of multi-vendor compromise.

The sanity bounds on catalog deltas (from Section 12.3 of the PRD) partially mitigate this by triggering degraded mode on sudden large delta influxes. But a targeted attack injecting a small number of high-impact false entries for specific packages would not trigger the delta bounds.

The PRD's user-configurable thresholds (lower for extensions, higher for Go modules) reduce the blast radius but don't prevent it. A deployment that drops to single-source enforcement for extensions would be vulnerable to a single-source poisoning attack on extensions specifically.

**Why it happens:**

The 2-of-N model assumes independent catalog sources are independently operated with independently secured infrastructure. The May 2026 incidents demonstrated that independent vendors can be compromised through shared attack vectors (same threat actor, similar social engineering patterns). The independence assumption is probabilistically valid but not guaranteed.

**How to avoid:**

- Implement weighted corroboration for v1.1 where configurable source trust weights are maintained per ecosystem. Bumblebee (maintained by the same community responding to real incidents) has higher baseline trust than an unsigned user-provided catalog.
- Add a "source health" monitor: if a catalog source's entry velocity suddenly spikes (many new entries in a short window for a single ecosystem), flag those entries as suspect and require a higher corroboration bar for them to trigger enforcement.
- The `beekeeper-self` pattern (Section 12.6) should include a "beekeeper-catalogs" feed that publishes known-bad catalog entries — a second-order catalog for the catalogs themselves. This creates a revocation mechanism for poisoned catalog entries.
- For the false-positive DoS case: include a community override mechanism where entries flagged as false positives by multiple independent users can be downgraded to warn-only pending investigation, without requiring a full catalog release.
- Document explicitly in the threat model which corroboration scenarios Beekeeper does and does not protect against.

**Warning signs:**

- A sudden wave of enforcement blocks for widely-used, low-suspicion packages not previously flagged
- Catalog delta counts spiking on a source that has historically been stable
- Multiple users reporting the same package blocked with only 2-source corroboration where one source is a recently-added catalog

**Phase to address:**

v0.3.0 (corroboration model implementation): Build source health monitoring and delta sanity bounds from the first corroboration implementation. v0.9.0: Design the catalog-revocation feed concept. v1.0.0: Document the known attack surface against the corroboration model in the public threat model.

---

### Pitfall 6: MCP Proxy Correctness Is Harder Than It Appears

**What goes wrong:**

The MCP gateway (`beekeeper gateway`) is a long-running proxy between MCP clients (agents) and upstream MCP servers. Writing a correct MCP proxy has specific failure modes beyond generic HTTP proxy correctness:

1. **Request ID correlation across multiplexed calls**: MCP JSON-RPC 2.0 allows batched requests where the response array order is not guaranteed to match the request order. A proxy that assumes sequential request-response pairing and forwards by order will corrupt ID-to-response mappings under concurrent tool calls. The spec requires correlating by `id` field, not position.

2. **Capability negotiation forwarding**: The MCP handshake (client `initialize` → server `initialize` response → `initialized` notification) negotiates protocol version and capabilities. A proxy must maintain separate capability sets for the client-facing side and each upstream-server-facing side. Forwarding the upstream server's capabilities verbatim to the client breaks if the proxy itself does not support all advertised features (e.g., if the upstream advertises `streaming: true` but the proxy buffers everything).

3. **SSE/Streamable HTTP transport complexity**: As of MCP protocol 2024-11-05, standalone SSE transport is deprecated in favor of Streamable HTTP. A proxy talking to upstream servers using the older SSE transport and presenting Streamable HTTP to clients (or vice versa) requires active transport bridging, not just message forwarding. This is non-trivial and a common source of "works with one client, fails with another" bugs.

4. **Per-session token enforcement with connection reuse**: The PRD specifies per-session token auth for the gateway. If a client reuses a TCP connection across sessions (which HTTP/1.1 keepalive and HTTP/2 both do), the token validation must be per-request, not per-connection. A proxy that validates the token at connection establishment and then trusts all subsequent requests on that connection is vulnerable to token reuse attacks.

5. **Message size and recursion bounds**: Tool definitions in MCP can include arbitrarily nested JSON schemas. Without explicit recursion depth limits on the JSON-RPC parser, a malicious upstream server (or a compromised agent feeding a malicious tool call response back through the gateway) can trigger stack overflow in the proxy's parser.

**Why it happens:**

MCP is a relatively new protocol (2024-2025) with active spec changes (the SSE → Streamable HTTP transition being the major one). Proxy implementations written against an earlier spec version quietly break when clients or servers upgrade. The protocol's flexibility (batching, streaming, capability negotiation) makes a correct implementation significantly more work than a naive forward-everything proxy.

**How to avoid:**

- Test the gateway against both stdio-mode upstream servers and HTTP-mode upstream servers from v0.6.0. Do not prototype with only one transport.
- Implement request ID correlation tables explicitly — never assume response ordering matches request ordering.
- Handle the capability negotiation as a three-party negotiation: (1) what the upstream server advertises, (2) what Beekeeper's proxy can bridge, (3) what the proxy should advertise to the client. The intersection of (1) and (2), not the raw upstream capabilities.
- Fuzz the MCP message parser against malformed JSON, deeply nested schemas, oversized payloads, and malformed `id` fields from the first gateway implementation.
- Pin to the stable MCP spec version (2025-11-25 or later) and add a spec version check in CI: the gateway integration tests should fail if the pinned spec version drifts from what the upstream SDK produces.

**Warning signs:**

- Gateway works with Claude Code but fails with Cursor (different client implementations expose different edge cases)
- Tool calls work but tool list responses are empty or corrupt after the first request in a session
- Memory usage growing monotonically in the gateway daemon (leak in the request correlation table)
- Parser panics in CI fuzz testing before v0.6.0 release

**Phase to address:**

v0.6.0 (gateway implementation). Fuzz testing is a release gate, not a backlog item.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Load catalog from disk per `beekeeper check` invocation instead of IPC to warm daemon | Simpler v0.1.0 implementation | p99 latency blows out under cold-cache conditions; users disable the hook | Acceptable for v0.1.0 only if p99 is measured and a persistent daemon is on the v0.3.0 roadmap |
| Build Sentry only for the happy-path kernel (5.8+) | Faster v0.6.0 delivery | Silent Sentry failure on Ubuntu 20.04 LTS (large user base); capability probing much harder to add retroactively | Never acceptable for a published release |
| Skip per-request MCP token validation, validate only at connection establishment | Simpler gateway implementation | Token reuse attack surface; connection-reuse clients bypass auth for subsequent requests | Never acceptable |
| Use union-of-bad catalog matching (any source = enforce) instead of corroboration | Simpler catalog engine, faster detection | A single compromised source triggers enforcement against legitimate packages; users disable Beekeeper | Never acceptable; the corroboration model is a core design principle |
| Treat all catalog sources as equal weight | Simpler corroboration logic | User-provided unsigned catalogs trigger enforcement with equal weight to Bumblebee; easy to exploit | Acceptable for v0.3.0 only if source weights are on the roadmap for v0.6.0 |
| Audit log without sensitive field redaction | Simpler first implementation | Audit log becomes a credential store; reads of `.env` files capture API key values | Acceptable for v0.1.0 if redaction patterns are added before any remote sink is enabled |
| Single SLSA provenance workflow that signs everything | Simpler CI | Public Sigstore transparency log exposes repository structure; private modules in go.sum visible | Acceptable for OSS; document the transparency log disclosure explicitly |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Claude Code PreToolUse hooks | Assuming hook stdin is always well-formed JSON; malformed tool calls crash the handler | Validate JSON on entry; fail-closed on parse error; include parse error in audit log |
| Bumblebee `threat_intel/` catalog | Treating the catalog schema as stable; Bumblebee is actively evolving its schema | Pin to a specific Bumblebee schema version; add schema validation in CI against a fixture catalog |
| Socket public API | Assuming free-tier rate limits are sufficient for a catalog sync daemon polling hourly | Measure actual request rate under catalog sync; implement exponential backoff; cache aggressively |
| OSV offline DB | Assuming `osv-scanner`'s offline DB format is stable across OSV-Scanner versions | Pin `osv-scanner` version in CI; validate DB schema in the catalog sync test |
| fsnotify on Windows | Assuming ReadDirectoryChangesW fires for all file creates in extension directories; symlinks and junction points are not always reported | Test extension directory watcher specifically against VS Code's extension install mechanism on Windows, which uses junction points |
| LlamaFirewall sidecar via Unix socket | Assuming the socket survives Go's `exec.Command` supervision across macOS and Windows | Named pipes on Windows, Unix sockets on macOS/Linux; the abstraction is not transparent; test both paths |
| MCP protocol version | Assuming MCP clients all implement the same protocol version; Claude Code and Cursor may differ | Negotiate version per-connection; handle version mismatch gracefully rather than treating it as a fatal error |
| Sigstore + GitHub Actions OIDC | Referencing the slsa-github-generator action by short tag (`@v2`) instead of full semver (`@v2.0.0`) | The SLSA verifier requires full `@vX.Y.Z` tags; short tags cause verification failures even if the action runs successfully |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Per-invocation catalog disk I/O in `beekeeper check` | p99 latency 200ms+ on first call after boot; noticeable agent stutter | Implement warm-daemon IPC pattern by v0.3.0; benchmark with cold OS file cache | From first use on a machine that just booted |
| Sentry eBPF ring buffer undersized for `npm install` storms | ETW/eBPF event loss counter increments; credential-access events dropped in the noise | Tune ring buffer size based on worst-case `npm install` event rate; measure in v0.6.0 dev | During any `npm install` with many packages |
| eslogger stdout pipe saturation on macOS | Pipe buffer fills; eslogger drops events silently; Sentry audit log has gaps | Dedicated high-priority goroutine draining eslogger stdout; explicit pipe buffer size | During `npm install` or Bumblebee scan triggering many file events |
| LlamaFirewall sidecar sample-every-call at high tool call rate | CPU spike; sidecar queue backs up; `beekeeper check` latency grows | Implement configurable sampling rate (default 1.0, reduce for resource-constrained machines); surface sidecar queue depth in `beekeeper diag` | When agent is in a tight tool-call loop (e.g., batch file processing) |
| Baseline counter file contention | Multiple `beekeeper check` processes racing to update baseline counters simultaneously | Use append-only log for raw events; compact counters asynchronously; do not O_TRUNC on every invocation | At 50+ tool calls/hour concurrent agent sessions |
| Catalog delta scan triggering full Bumblebee scan on every new `threat_intel/` entry | CPU spike every time a catalog entry is added upstream; adds to battery drain | Batch catalog delta triggers; scan only affected ecosystems; debounce with minimum interval | When upstream catalog is actively updated (post-incident response period) |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Audit log file permissions set to 0644 instead of 0600 on Unix | Any local process can read the audit log, which contains sensitive path access patterns and partial credential shapes | Set 0600 permissions at file creation; verify in `beekeeper selftest`; re-verify on startup |
| MCP gateway binding to 0.0.0.0 by default instead of 127.0.0.1 | Gateway becomes a network-accessible proxy; any host on the LAN can submit tool calls | Default bind to 127.0.0.1; require explicit config and acknowledgment flag to expose remotely; this is documented as a PRD requirement but easy to accidentally break during development |
| IPC socket between Sentry daemon and CLI world-readable | Any local process can send commands to the privileged Sentry daemon | Use `SO_PEERCRED` on Linux; named pipe ACLs on Windows; test IPC authorization in unit tests, not just integration tests |
| Catalog entries with external URL references executed at match time | Catalog injection escalates from "update JSON" to "execute arbitrary code" | Validate that no catalog field triggers external network requests during matching; whitelist catalog schema fields strictly; reject unknown fields |
| Per-session gateway token stored in agent config file | Token file readable by any process running as the same user | Store token in memory only; generate fresh token per gateway session; if persistence is needed, use 0600-mode token file |
| `beekeeper check` accepting arbitrarily large stdin | Memory exhaustion from adversarially large tool call payloads | Hard-cap stdin at 1MB (already in PRD); enforce before allocating the read buffer, not after |
| Go `os/exec` subprocess in policy engine for catalog actions | Code injection from malicious catalog entry if any exec path is influenced by catalog data | Never use `os/exec` in the policy engine hot path; catalog data is strictly JSON matching, never executed |

---

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Sentry fires on developer's own routine credential access | Alert fatigue within days; user disables Sentry | 7-day baseline mode (already in PRD) is correct; additionally, make the baseline period's audit records visible in the TUI so users can see what's being learned |
| Block message says "blocked by policy" without actionable context | User doesn't know how to unblock; files a bug assuming Beekeeper is broken | Block messages must include: which rule fired, which catalog source corroborated, what user action resolves it (add to allowlist, wait for release age, etc.) |
| `beekeeper protect install` requires elevation without explanation | User skips elevation, gets unprivileged tier, wonders why Sentry events don't appear | `beekeeper init` should offer a clear explanation of what elevation adds and what the user gets without it, with a "not now" option that records the choice |
| Dashboard showing "all green" when a catalog source is stale | User trusts the system is fully armed when it's operating on 3-day-old threat intel | Stale catalog indicator must be visible even when no active threats are detected; color-code by staleness duration |
| `beekeeper check` failing silently (binary not found, bad PATH) | Agent proceeds without policy evaluation; user doesn't know protection is disabled | Claude Code hook failure (non-zero exit without stdout) should produce a visible warning in the agent's output; document this expected behavior in the integration guide |
| Extension quarantine removing an extension the developer needs | Workflow disruption; user disables the extension watcher to get work done | Quarantine should move to `~/.beekeeper/quarantine/`, not delete; surface a "restore" action immediately in the TUI; include a "why was this quarantined" explanation with the catalog provenance |

---

## "Looks Done But Isn't" Checklist

- [ ] **Hook handler latency:** Measure p99 latency under cold-cache conditions, not just warm-cache average — verify with `beekeeper diag` on a freshly-booted machine with a realistic catalog size
- [ ] **Sentry Linux coverage:** Verify Sentry generates events for mmap-based file access (it won't; fanotify limitation) — document this gap explicitly rather than leaving it implicit
- [ ] **Sentry macOS eslogger:** Verify the Beekeeper eslogger consumer handles the pipe backpressure case (high-event-rate periods) — test during a full `npm install` invocation
- [ ] **ETW event loss:** Verify `EventsLost` counter is surfaced in `beekeeper diag` and TUI on Windows — confirm it increments during simulated high-event-rate scenarios
- [ ] **MCP gateway concurrent requests:** Verify request ID correlation is correct under concurrent tool calls — test with a client that issues multiple calls simultaneously
- [ ] **Reproducible builds:** Verify binary hashes match between two independent builds from the same commit — do not rely on `trimpath` alone; match the exact Go toolchain version including default GOROOT
- [ ] **Catalog signature verification:** Verify that Beekeeper refuses to load a catalog with an invalid signature — test with a deliberately corrupted signature, not just an absent one
- [ ] **Fail-closed default:** Verify that a `beekeeper check` process kill (SIGKILL, not graceful) causes Claude Code to block the tool call, not allow it — test the actual hook exit code behavior
- [ ] **Audit log permissions:** Verify audit log is created 0600 on first write — check on both Linux and macOS; Windows ACLs need separate verification
- [ ] **SLSA verification path:** Verify `make verify-release` actually fails when given a tampered binary — test against a binary with a single byte changed

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Hook handler latency regression discovered post-v0.1.0 | MEDIUM | Implement persistent IPC daemon in v0.3.0; add latency regression gate to CI to prevent recurrence |
| eBPF kernel incompatibility reported by users on older kernels | MEDIUM | Add capability probing and graceful degradation tiers; requires testing on the affected kernel versions (GitHub Actions `ubuntu-latest` is not sufficient) |
| ETW event loss discovered in production on Windows | LOW | Tune buffer sizes and expose `EventsLost` counter; configuration change, no code rewrite |
| MCP gateway message ordering bug corrupting tool call responses | HIGH | Gateway requires protocol-level rewrite of the correlation table; will break existing integrations during fix; add to fuzz test corpus to prevent recurrence |
| Catalog poisoning false-positive DoS affecting users | MEDIUM | Emergency: publish downgrade entry in `beekeeper-self` catalog for the affected catalog source entries; longer-term: implement source health monitoring |
| SLSA provenance breaks due to slsa-github-generator version mismatch | LOW | Pin action to full `@vX.Y.Z` semver; re-run workflow; no binary changes required |
| Beekeeper binary reproducibility fails (different hashes on independent build) | MEDIUM | Identify non-determinism source (Go toolchain version, CGO, build path); fix build flags; re-release; update `BUILDING.md` with exact reproduction steps |
| Audit log 0644 permissions discovered (credential exposure risk) | LOW | `chmod 0600` the audit file; add permissions check to `beekeeper selftest`; re-release with the fix |
| LlamaFirewall sidecar supervisor crash loop blocking agent workflow | LOW | Fail-open mode already in PRD; user can disable sidecar temporarily; fix underlying crash; no data loss |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Hook handler cold-start latency | v0.1.0: benchmark baseline; v0.3.0: persistent IPC if needed | `beekeeper diag` shows p99 < 100ms on cold-cache machine with 5,000-entry catalog |
| eBPF kernel version fragmentation | v0.6.0: capability probing at install | `beekeeper protect status` shows correct tier on kernel 5.4, 5.8, 6.x; Sentry generates expected events or explicit "degraded" notice |
| ETW event loss on Windows | v0.9.0: buffer tuning + EventsLost surfacing | `beekeeper diag` shows EventsLost = 0 during simulated `npm install`; or surfaces "detection degraded" when loss occurs |
| eslogger coverage gaps on macOS | v0.9.0: gap documentation + backpressure handling | `beekeeper protect status` lists coverage gaps; eslogger pipe drain survives `npm install` stress test |
| Catalog corroboration poisoning | v0.3.0: delta sanity bounds; v0.9.0: source health monitoring | Inserting 1,000 false entries into a single catalog source triggers degraded mode, not enforcement |
| MCP proxy correctness | v0.6.0: fuzz testing as release gate | Gateway fuzz test corpus covers batched requests, oversized messages, malformed IDs; concurrent tool calls return correct ID correlation |
| Go reproducible builds non-determinism | v0.1.0: `make verify-release` target | Two independent builds from same git commit and same Go toolchain version produce identical SHA-256 hashes |
| Sentry false positive alert fatigue | v0.6.0: baseline mode implementation; v1.0.0: TUI baseline visibility | 7-day audit-only baseline before rules promote; user can see what baseline has learned |
| SLSA Level 3 workflow friction | v0.9.0: full SLSA implementation | `make verify-release` passes; slsa-verifier accepts the provenance attestation independently |
| Single binary shared privilege surface | All phases: IPC authorization | `SO_PEERCRED` check rejects connection from process owned by different UID; test in unit tests |

---

## Sources

- Claude Code hooks latency real-world data: [Hooks causing ~20s latency · ruvnet/ruflo Issue #1530](https://github.com/ruvnet/ruflo/issues/1530)
- Claude Code hooks dispatcher pattern: [Claude Code Hooks: Why Each of My 95 Hooks Exists](https://blakecrosley.com/blog/claude-code-hooks)
- Claude Code hooks official reference: [Hooks reference - Claude Code Docs](https://code.claude.com/docs/en/hooks)
- eBPF kernel version requirements: [BPF Features by Linux Kernel Version - iovisor/bcc](https://github.com/iovisor/bcc/blob/master/docs/kernel-versions.md)
- eBPF portability across kernel versions: [Why Does My eBPF Program Work on One Kernel but Fail on Another?](https://labs.iximiuz.com/tutorials/portable-ebpf-programs-46216e54)
- Ubuntu eBPF unprivileged disable: [Unprivileged eBPF disabled by default - Ubuntu Community Hub](https://discourse.ubuntu.com/t/unprivileged-ebpf-disabled-by-default-for-ubuntu-20-04-lts-18-04-lts-16-04-esm/27047)
- fanotify mmap limitation: [fanotify(7) Linux manual page](https://www.man7.org/linux/man-pages/man7/fanotify.7.html)
- fanotify FAN_MARK_FILESYSTEM kernel requirements: [Linux fanotify for Real-Time Filesystem Security Monitoring](https://www.systemshardening.com/articles/linux/linux-fanotify-security-monitoring/)
- ETW architecture and event loss: [EVENT_TRACE_PROPERTIES - Microsoft Learn](https://learn.microsoft.com/en-us/windows/win32/api/evntrace/ns-evntrace-event_trace_properties)
- ETW buffer tuning: [Adjusting Buffer Settings for ETW - Microsoft Learn](https://learn.microsoft.com/en-us/archive/blogs/visualizeparallel/adjusting-buffer-settings-for-event-tracing-for-windows-etw)
- ETW EDR design issues: [Design issues of modern EDRs: bypassing ETW-based solutions - Binarly](https://www.binarly.io/blog/design-issues-of-modern-edrs-bypassing-etw-based-solutions)
- eslogger analysis: [Blue Teaming on macOS with eslogger - Cybereason](https://www.cybereason.com/blog/blue-teaming-on-macos-with-eslogger)
- EndpointSecurity framework overview: [Endpoint Security In a macOS World - Huntress](https://www.huntress.com/blog/endpoint-security-in-a-macos-world)
- Nx Console compromise postmortem: [Postmortem: Nx Console v18.95.0 supply-chain compromise](https://nx.dev/blog/nx-console-v18-95-0-postmortem)
- False positive security tool adoption failure: [Are Your Security Tools Crying Wolf? - CYDEF](https://cydef.io/resources/are-your-security-tools-crying-wolf/)
- Falco false positive real-world data: [False positives in GKE · falcosecurity/falco Issue #439](https://github.com/falcosecurity/falco/issues/439)
- SLSA Level 3 for Go on GitHub Actions: [Achieving SLSA 3 Compliance with GitHub Actions - GitHub Blog](https://github.blog/security/supply-chain-security/slsa-3-compliance-with-github-actions/)
- SLSA adoption challenges: [SLSA Provenance Part 3: Adoption Challenges - Legit Security](https://www.legitsecurity.com/blog/slsa-provenance-blog-series-part3-challenges-of-adopting-slsa-provenance)
- Go reproducible builds: [Reproducing Go binaries byte-by-byte - Filippo Valsorda](https://words.filippo.io/reproducing-go-binaries-byte-by-byte/)
- Go build non-determinism issue: [cmd/go: builds not reproducible · golang/go Issue #36230](https://github.com/golang/go/issues/36230)
- Privilege separation model: [Examining OpenSSH Sandboxing and Privilege Separation - JFrog](https://jfrog.com/blog/examining-openssh-sandboxing-and-privilege-separation-attack-surface-analysis/)
- MCP transport failure modes: [MCP Transport: Architecture, Boundaries, and Failure Modes - pgEdge](https://www.pgedge.com/blog/mcp-transport-architecture-boundaries-and-failure-modes)
- MCP JSON-RPC message structure: [JSON-RPC Message Structure - apxml](https://apxml.com/courses/getting-started-model-context-protocol/chapter-1-architecture-and-fundamentals/json-rpc-message-structure)

---
*Pitfalls research for: Beekeeper agent runtime safety harness*
*Researched: 2026-05-26*
