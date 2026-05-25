# Architecture Research

**Domain:** Go multi-process security daemon — agent runtime safety harness
**Researched:** 2026-05-26
**Confidence:** HIGH (core Go patterns), MEDIUM (platform-specific kernel interfaces), HIGH (IPC authorization)

---

## Standard Architecture

### System Overview

```
+---------------------------------------------------------------------+
|                     UNPRIVILEGED USER TIER                          |
|                                                                     |
|  +------------------+  +-------------------+  +-----------------+  |
|  | beekeeper check  |  | beekeeper gateway |  | beekeeper watch |  |
|  | (ephemeral proc) |  | (long-lived MCP   |  | (fsnotify loop, |  |
|  | stdin JSON in    |  |  proxy daemon)    |  |  extension dirs)|  |
|  | exit 0/1 out     |  |                   |  |                 |  |
|  +--------+---------+  +---------+---------+  +--------+--------+  |
|           |                      |                      |           |
|           +----------------------+----------------------+           |
|                                  |                                  |
|                       +----------v----------+                       |
|                       |   Policy Engine     |                       |
|                       |   (in-process lib,  |                       |
|                       |   no goroutine       |                       |
|                       |   boundary)          |                       |
|                       +----------+----------+                       |
|                                  |                                  |
|          +-----------------------+-----------------------+          |
|          |                       |                       |          |
|  +-------v-------+    +----------v--------+   +---------v------+   |
|  | Catalog Store |    | Behavioral        |   | Audit Writer   |   |
|  | (in-memory    |    | Baseline          |   | (NDJSON sink,  |   |
|  |  hash index,  |    | (per-project      |   |  append-only,  |   |
|  |  mmap'd JSON) |    |  counters)        |   |  0600 perms)   |   |
|  +---------------+    +-------------------+   +----------------+   |
|                                                                     |
|           Unix socket / Windows named pipe (authorized)            |
+---------------------------------------------------------------------+
                              |
+---------------------------------------------------------------------+
|                   PRIVILEGED SENTRY TIER                            |
|                                                                     |
|  +------------------+  +------------------+  +------------------+  |
|  | Linux            |  | macOS            |  | Windows          |  |
|  | fanotify (file)  |  | eslogger stdin   |  | ETW consumer     |  |
|  | eBPF (proc+net)  |  | JSON decode      |  | (golang-etw,     |  |
|  | cilium/ebpf      |  |                  |  |  no-CGO variant) |  |
|  +--------+---------+  +--------+---------+  +--------+---------+  |
|           |                     |                     |             |
|           +---------------------+---------------------+             |
|                                 |                                   |
|                      +----------v----------+                        |
|                      | Process Correlation |                        |
|                      | Engine              |                        |
|                      | (rule eval, 5 rules |                        |
|                      |  v1, sliding window)|                        |
|                      +----------+----------+                        |
|                                 |                                   |
|              +------------------+------------------+                |
|              |                                     |                |
|    +---------v---------+               +----------v--------+        |
|    | IPC Server        |               | Audit Writer      |        |
|    | (Unix sock /      |               | (appends to same  |        |
|    |  named pipe,      |               |  NDJSON log)      |        |
|    |  SO_PEERCRED /    |               |                   |        |
|    |  pipe ACL auth)   |               +-------------------+        |
|    +-------------------+                                            |
+---------------------------------------------------------------------+
                              |
+---------------------------------------------------------------------+
|                  OPTIONAL PYTHON SIDECAR                            |
|                                                                     |
|  +-------------------+  supervised by beekeeper (unprivileged)      |
|  | LlamaFirewall     |  length-prefixed JSON over Unix socket /     |
|  | PromptGuard2      |  named pipe; fail-closed on crash            |
|  | CodeShield        |                                              |
|  +-------------------+                                              |
+---------------------------------------------------------------------+
```

### Component Responsibilities

| Component | Responsibility | Privilege | Lifetime |
|-----------|---------------|-----------|----------|
| `beekeeper check` | Read tool call JSON from stdin, evaluate policy, exit 0/1 | User | Ephemeral (per hook) |
| Policy Engine | Catalog match, release-age, lifecycle, path, egress, baseline | User | In-process library |
| Catalog Store | Immutable in-memory index of loaded threat intel; mmap for large catalogs | User | Daemon lifetime or lazy-loaded per check invocation |
| `beekeeper gateway` | Long-lived MCP proxy; apply policy in hot path per tool call | User | Service/daemon |
| `beekeeper watch` | fsnotify loop over extension dirs; trigger catalog match on new dirs | User | Service/daemon |
| `beekeeper catalogs watch` | HTTP fetch + delta detection; trigger scan on new entries | User | Service/daemon |
| Sentry daemon | Ingest OS kernel event streams; run process correlation rules | Root/System | Service/daemon |
| IPC server (in Sentry) | Accept commands from unprivileged CLI; authorize via SO_PEERCRED / pipe ACL | Root/System | Service/daemon |
| LlamaFirewall sidecar | PromptGuard2 + CodeShield inference; length-prefixed JSON IPC | User | Supervised subprocess |
| Audit Writer | Append NDJSON to `audit/beekeeper.ndjson`; enforce `0600`; optional remote sinks | User/System | Shared component |

---

## Recommended Project Structure

```
beekeeper/
├── cmd/
│   └── beekeeper/
│       └── main.go              # Thin entrypoint: cobra root, OS signal wiring
├── internal/
│   ├── check/                   # Hook handler logic (beekeeper check)
│   │   ├── handler.go           # ReadStdin → PolicyEngine → Decision → Exit
│   │   └── handler_test.go
│   ├── policy/                  # Policy engine (pure library, no I/O)
│   │   ├── engine.go            # Evaluate() — catalog + release-age + lifecycle + path + egress
│   │   ├── catalog.go           # CatalogStore interface + corroboration logic
│   │   ├── releaseage.go        # Release-age policy
│   │   ├── lifecycle.go         # Lifecycle script policy
│   │   ├── paths.go             # Sensitive path blocklist
│   │   ├── egress.go            # Network egress policy
│   │   ├── baseline.go          # Behavioral baseline counters
│   │   └── engine_test.go       # Adversarial fixture corpus
│   ├── catalog/                 # Catalog loading, indexing, sync
│   │   ├── loader.go            # JSON parse + index build
│   │   ├── index.go             # In-memory hash index (map[ecosystem]map[package][]Entry)
│   │   ├── sync.go              # HTTP fetch, signature verify, delta detect
│   │   ├── watcher.go           # beekeeper catalogs watch daemon loop
│   │   └── signature.go         # Catalog signature verification
│   ├── gateway/                 # MCP proxy daemon
│   │   ├── server.go            # net.Listener, session lifecycle
│   │   ├── proxy.go             # Per-connection proxy goroutine
│   │   ├── policy_middleware.go # Policy evaluation in hot path
│   │   └── token.go             # Per-session token issuance + verification
│   ├── sentry/                  # Privileged daemon (build-tag separated)
│   │   ├── daemon.go            # Main event loop, IPC server
│   │   ├── rules.go             # Correlation rule evaluation
│   │   ├── sliding_window.go    # Time-windowed event accumulator
│   │   ├── linux/               # Linux-specific (build tag: linux)
│   │   │   ├── fanotify.go      # fanotify event ingestion
│   │   │   └── ebpf.go          # cilium/ebpf loader + perf reader
│   │   ├── darwin/              # macOS-specific (build tag: darwin)
│   │   │   └── eslogger.go      # eslogger subprocess + JSON decode
│   │   └── windows/             # Windows-specific (build tag: windows)
│   │       └── etw.go           # ETW session consumer
│   ├── ipc/                     # IPC client/server (cross-platform)
│   │   ├── server.go            # Unix socket (Linux/macOS) + named pipe (Windows)
│   │   ├── client.go            # Symmetric client
│   │   ├── auth_unix.go         # SO_PEERCRED authorization (build tag: !windows)
│   │   ├── auth_windows.go      # Named pipe ACL authorization (build tag: windows)
│   │   └── protocol.go          # Length-prefixed JSON message framing
│   ├── watch/                   # Extension directory watcher (fsnotify)
│   │   ├── watcher.go           # fsnotify setup, event fan-out
│   │   └── extension.go         # Parse extension manifest, trigger catalog match
│   ├── audit/                   # Audit log writer
│   │   ├── writer.go            # Append NDJSON, enforce permissions, rotate
│   │   └── sinks.go             # Syslog, OTLP, HTTPS POST sinks
│   ├── llamafirewall/           # LlamaFirewall sidecar supervisor
│   │   ├── supervisor.go        # Process start, crash restart, fail-closed
│   │   └── client.go            # Length-prefixed JSON IPC client
│   ├── config/                  # Layered config (system → user → project → env → flags)
│   │   └── config.go
│   ├── tui/                     # Bubble Tea TUI (beekeeper dashboard)
│   │   └── dashboard.go
│   └── selfdefense/             # beekeeper-self catalog check, reproducible build
│       └── selfcheck.go
├── bpf/                         # eBPF C programs (compiled via bpf2go)
│   ├── process_monitor.bpf.c
│   └── network_monitor.bpf.c
├── policies/                    # Default policy files (embedded via go:embed)
│   ├── lifecycle.json
│   └── sensitive_paths.json
├── testdata/                    # Adversarial fixture corpus
│   ├── toolcalls/               # Real malicious tool call patterns (May 2026 incidents)
│   └── catalogs/                # Synthetic catalog entries for unit tests
├── Makefile
├── go.mod
├── go.sum
└── .goreleaser.yaml
```

### Structure Rationale

- **`cmd/beekeeper/main.go` is thin:** Cobra root + subcommand wiring only. All business logic lives in `internal/`. This keeps the entry point testable via subprocess tests and makes it possible for `beekeeper check` to run its critical path with zero cobra overhead (cobra is only initialized at startup once).
- **`internal/policy/` is a pure library:** No I/O, no goroutines, no global state. Takes a `ToolCall` struct, returns a `Decision`. This is the hot path — it must be callable directly without any IPC or process boundary.
- **`internal/sentry/` uses build tags, not runtime conditionals:** Platform-specific kernel interfaces (`fanotify`, `eslogger`, ETW) are separated at compile time. The daemon binary for Linux does not link Windows ETW code. Build tags `//go:build linux`, `//go:build darwin`, `//go:build windows` on each platform subdirectory.
- **`bpf/` contains C source:** Compiled by `bpf2go` during `go generate`. Generated Go bindings land in `internal/sentry/linux/`. The C is a build-time dependency; the final binary embeds the eBPF bytecode via `go:embed`.
- **`policies/` embedded via `go:embed`:** Default policies are compiled into the binary. The catalog store overlays user-configured policies at runtime without modifying the embedded defaults.

---

## Architectural Patterns

### Pattern 1: Ephemeral Process Hook Handler — No Pre-Warming Needed

**What:** `beekeeper check` is a fresh process spawned per agent tool call. It reads stdin, evaluates policy from an in-memory catalog built at startup, writes to the audit log, and exits.

**Startup budget analysis:**
- Bare Go binary hello-world: ~11ms measured (Replit benchmark)
- With real JSON catalog loading: 15-25ms estimated (depends on catalog size; mmap avoids full parse)
- JSON stdin decode + policy eval: ~1-5ms
- Audit log append (single write): ~1ms
- **Total realistic p95:** 20-40ms, well within the 100ms target

**Why pre-warming is not needed:** The 100ms budget is comfortable. Pre-warming (a long-lived daemon serving check requests) would add IPC round-trip complexity for minimal gain. The killer optimization is loading catalogs efficiently at startup — use `json.Decoder` streaming rather than `ioutil.ReadAll` + `json.Unmarshal`, and build the in-memory hash index once.

**When pre-warming would become necessary:** If catalog size grows to 50MB+ and cold parse takes >70ms. The mitigation is to pre-build and persist the hash index to a file (write once on `catalogs sync`, load with mmap on `check`). Flag this as a v0.3 optimization target once real catalog sizes are known.

**Example critical path:**

```go
// internal/check/handler.go
func Run() int {
    // 1. Load config (< 1ms if previously parsed to binary format)
    cfg := config.LoadCached()

    // 2. Open catalog index (mmap or in-process; pre-built by catalogs sync)
    idx := catalog.OpenIndex(cfg.CatalogPath) // < 5ms with mmap

    // 3. Decode tool call from stdin (bounded: max 1MB)
    var call toolcall.ToolCall
    if err := json.NewDecoder(io.LimitReader(os.Stdin, 1<<20)).Decode(&call); err != nil {
        audit.WriteFailure("stdin_decode_error", err)
        return 1 // fail-closed
    }

    // 4. Policy eval — pure function, no I/O
    decision := policy.Evaluate(call, idx, cfg.Policy)

    // 5. Audit
    audit.Write(decision)

    // 6. LlamaFirewall (only for relevant inputs, async write to pipe)
    if cfg.LlamaFirewall.Enabled && call.IsRelevantForPromptScan() {
        llamafirewall.CheckAsync(call, cfg)
    }

    if decision.Action == policy.Block {
        fmt.Fprintln(os.Stderr, decision.Reason)
        return 2
    }
    return 0
}
```

### Pattern 2: Catalog Index — Two-Level Hash Map, Not a Trie

**What:** The threat catalog is indexed as `map[Ecosystem]map[PackageKey][]Entry` where `PackageKey` is `package@version` normalized to lowercase. Lookup is O(1) amortized.

**Why not a trie or radix tree:** Go's built-in `map` outperforms trie implementations for exact-match lookups even at millions of entries (confirmed by multiple Go performance benchmarks — Go's map uses Robin Hood hashing with excellent cache locality for string keys). Prefix lookups are not needed for exact package-name matching.

**Version range matching:** Kept in a sorted slice per package, binary-searched. Version ranges are rare enough that linear scan over a small slice (typically <20 versions per CVE) is faster than building a full interval tree.

**Catalog load path:**

```go
// internal/catalog/index.go
type Index struct {
    // Primary: ecosystem -> package -> entries
    entries map[string]map[string][]Entry

    // Self-defense catalog (checked at startup)
    selfEntries []Entry

    mu sync.RWMutex // protects hot-reload on catalog sync
}

func (idx *Index) Lookup(ecosystem, pkg, version string) []Entry {
    idx.mu.RLock()
    defer idx.mu.RUnlock()

    pkgMap, ok := idx.entries[ecosystem]
    if !ok {
        return nil
    }
    entries, ok := pkgMap[normalizeKey(pkg)]
    if !ok {
        return nil
    }
    return matchVersions(entries, version) // binary search on sorted versions
}
```

**Hot-reload on catalog sync:** The catalog watcher builds a new index in a background goroutine, then atomically swaps the pointer under a `sync.RWMutex`. Readers (check handler subprocesses) always open the pre-built index file; the in-process index is only used by the long-lived gateway daemon.

### Pattern 3: Corroboration Engine — Separate from Matching

**What:** Catalog matching produces raw `[]Entry` (all hits across sources). Corroboration is a separate aggregation step that counts distinct sources and applies thresholds.

**Why separate:** Makes the matching logic independently testable and lets corroboration thresholds be configurable without touching the indexing code.

```go
// internal/policy/catalog.go
func Corroborate(matches []catalog.Entry, cfg CorroborationConfig) Decision {
    sources := make(map[string]struct{})
    for _, m := range matches {
        sources[m.CatalogSource] = struct{}{}
    }
    switch len(sources) {
    case 0:
        return Decision{Action: Allow}
    case 1:
        return Decision{Action: Warn, Sources: sources}
    case 2:
        return Decision{Action: Block, Sources: sources}
    default: // 3+
        return Decision{Action: BlockWithQuarantine, Sources: sources}
    }
}
```

### Pattern 4: MCP Gateway — Policy in the Hot Path via Middleware Chain

**What:** The gateway is a net.Listener accepting connections from MCP clients. Each connection gets a dedicated goroutine. Tool call requests are intercepted, policy-evaluated synchronously, then either forwarded or rejected before the upstream MCP server sees them.

**Connection lifecycle:**

```
Client connects
    → Token authentication (per-session token, checked before any other handling)
    → Handshake proxy (pass through to upstream MCP server)
    → Tool call intercept loop:
        read JSON-RPC request
        if method == "tools/call":
            policy.Evaluate() → Decision
            if Block: return error response to client, audit, continue loop
            else: forward to upstream, read response, audit, return to client
        else: forward unmodified
    → Session termination: clean up, audit session-end record
```

**Backpressure:** Use `context.WithTimeout` on each upstream forward. If the upstream MCP server is slow, the gateway's read deadline fires and the client gets an error. Do not buffer unboundedly — reject new connections when the active connection count exceeds the configured cap (default 10; a single developer machine is unlikely to have more than a handful of concurrent agent sessions).

**Policy evaluation must not block the goroutine loop:** The policy engine is a pure function call. LlamaFirewall calls are async with a configurable timeout; if the timeout fires, apply the `fail_closed` / `fail_open` / `fail_warn` policy. This keeps the goroutine serving other requests even when the Python sidecar is slow.

### Pattern 5: Sentry Event Loop — Platform-Specific Ingestion, Shared Rule Evaluation

**What:** The Sentry daemon has a platform-specific event ingestion layer and a platform-independent correlation engine. Events are normalized to a common `SentryEvent` struct before rule evaluation.

**Linux ingestion:**

```
fanotify fd (file events) ──► Go channel (buffered, 10k cap)
                                                               ──► Correlation engine
eBPF ring buffer (process + network) ──► Go channel (buffered)
```

The `perf.Reader` or `ringbuf.Reader` from `cilium/ebpf` runs in a dedicated goroutine that decodes kernel events and sends them to the channel. The correlation engine runs in a separate goroutine consuming from both channels via `select`. This keeps ingestion and evaluation decoupled — kernel events are never dropped because the evaluator is busy; the channel buffer absorbs bursts.

**eBPF program loading pattern (cilium/ebpf):**

```go
// internal/sentry/linux/ebpf.go
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang ProcessMonitor ../../bpf/process_monitor.bpf.c

func LoadAndAttach() (*EventSource, error) {
    if err := rlimit.RemoveMemlock(); err != nil {
        return nil, fmt.Errorf("remove memlock: %w", err)
    }
    objs := processMonitorObjects{}
    if err := loadProcessMonitorObjects(&objs, nil); err != nil {
        return nil, fmt.Errorf("load ebpf objects: %w", err)
    }
    // Attach to tracepoints (prefer over kprobes for stability across kernels)
    tp, err := link.Tracepoint("sched", "sched_process_exec", objs.TraceExec, nil)
    if err != nil {
        return nil, fmt.Errorf("attach tracepoint: %w", err)
    }
    rd, err := ringbuf.NewReader(objs.Events)
    if err != nil {
        return nil, fmt.Errorf("open ringbuf: %w", err)
    }
    return &EventSource{tp: tp, rd: rd}, nil
}
```

**macOS ingestion (eslogger):**

```go
// internal/sentry/darwin/eslogger.go
// eslogger streams EndpointSecurity events as NDJSON to stdout.
// Beekeeper spawns it as a subprocess and reads from its stdout pipe.
cmd := exec.CommandContext(ctx, "eslogger", "--format", "json",
    "exec", "open", "create", "network")
cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 0}}
stdout, _ := cmd.StdoutPipe()
go func() {
    dec := json.NewDecoder(bufio.NewReaderSize(stdout, 64*1024))
    for dec.More() {
        var ev ESEvent
        if err := dec.Decode(&ev); err != nil { continue }
        eventCh <- normalizeESEvent(ev)
    }
}()
```

**Windows ingestion (ETW):**

Use `tekert/golang-etw` (no CGO) rather than `bi-zone/etw` (requires CGO). The no-CGO constraint is explicit in the PRD — "no CGO for core." Subscribe to relevant ETW providers: `Microsoft-Windows-Kernel-Process` for process events, `Microsoft-Windows-Security-Auditing` for file access and network.

```go
// internal/sentry/windows/etw.go
// tekert/golang-etw consumer pattern (no CGO)
c := etw.NewConsumer(ctx)
c.FromProviderGUIDs(
    kernelProcessGUID,    // process create/exec
    securityAuditingGUID, // file open, network connect
)
c.ProcessEvents(func(e *etw.Event) {
    ev := normalizeETWEvent(e)
    if ev != nil {
        eventCh <- ev
    }
})
```

**ETW limitation to surface:** ETW rate-limits under high event volume. Beekeeper must surface degraded-mode indicators in `beekeeper diag` when the ETW session drops events (detectable via ETW session statistics).

### Pattern 6: IPC Authorization — SO_PEERCRED on Unix, ACL on Windows

**What:** The Unix socket between the unprivileged CLI and the privileged Sentry daemon uses `SO_PEERCRED` to verify that the connecting process is owned by the same UID as the installed Beekeeper user. Windows named pipes use a DACL restricting access to the installing user's SID.

**Unix (Linux + macOS):**

```go
// internal/ipc/auth_unix.go
func AuthorizeConn(conn *net.UnixConn, allowedUID uint32) error {
    raw, err := conn.SyscallConn()
    if err != nil { return err }
    var cred *unix.Ucred
    raw.Control(func(fd uintptr) {
        cred, err = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
    })
    if err != nil { return err }
    if cred.Uid != allowedUID {
        return fmt.Errorf("unauthorized: peer UID %d, expected %d", cred.Uid, allowedUID)
    }
    return nil
}
```

macOS does not support `SO_PEERCRED` directly; use `LOCAL_PEERPID` (`unix.GetsockoptInt(fd, unix.SOL_LOCAL, unix.LOCAL_PEERPID)`) to get the peer PID, then verify via `/proc`-equivalent (`kern.proc.pid.<pid>` sysctl).

**Windows:**

Named pipe server created with a DACL that grants `GENERIC_READ | GENERIC_WRITE` only to the installing user's SID. Use `golang.org/x/sys/windows` for the `CreateNamedPipe` call with a security descriptor. The `hectane/go-acl` package provides higher-level ACL manipulation.

**Protocol (both platforms):** Length-prefixed JSON. 4-byte big-endian length prefix, then JSON body. Maximum message size: 64KB (IPC commands are small — "quarantine item X", "reload config", "dump audit tail"). Reject oversized messages immediately.

### Pattern 7: fsnotify for Extension Directory Watching — Defense in Depth, Not Primary Security

**What:** `fsnotify` watches `~/.vscode/extensions/`, `~/.cursor/extensions/`, `~/.windsurf/extensions/` for new directory creation (new extension installs).

**Critical limitation to design around:** fsnotify does NOT currently implement a fanotify backend (as of 2026, the fsnotify roadmap shows fanotify as "Not yet"). It uses inotify on Linux (per-watch, not mount-wide), which has known event-loss scenarios:
- `inotify.max_user_watches` limit (default 8192) can be hit on machines with many directories
- Recursive watching is not supported — must explicitly add watches for new subdirectories
- Atomic file operations (temp file + rename) can cause RENAME events instead of CREATE

**Required pattern for recursive extension dir watching:**

```go
// internal/watch/watcher.go
// When a CREATE|RENAME event arrives for a directory path under the watched parent:
// 1. Immediately add a watch on the new directory
// 2. Walk the new directory to process any children already present (race window)
// 3. Trigger catalog match on any package.json found

watcher.Add(extensionRoot)
for ev := range watcher.Events {
    if ev.Op&(fsnotify.Create|fsnotify.Rename) != 0 {
        if isDir(ev.Name) {
            watcher.Add(ev.Name)          // add watch on new subdir
            scanExistingChildren(ev.Name) // handle race window
            go checkExtension(ev.Name)   // async catalog match
        }
    }
}
```

**Honest capability assessment:** fsnotify-based watching catches new installations but is not a security boundary. The race window (extension loads before Beekeeper quarantines it) is real and documented. This layer's value is speed of detection for catalog-known threats, not prevention. Sentry's fanotify-based monitoring (when elevated) provides the higher-assurance layer.

### Pattern 8: LlamaFirewall Sidecar Supervision

**What:** Beekeeper spawns the Python sidecar via `exec.Cmd`, monitors its health, and restarts on crash (up to 3 times with exponential backoff). Communication is length-prefixed JSON over a Unix domain socket / Windows named pipe.

**Fail-closed wiring:** The gateway and check handler have a configurable timeout on LlamaFirewall calls. If the timeout fires or the sidecar is unavailable, the decision falls back to the configured failure mode. The IPC client wraps `net.DialTimeout` with a 200ms default connect timeout.

**Supervision goroutine pattern:**

```go
// internal/llamafirewall/supervisor.go
func (s *Supervisor) Run(ctx context.Context) {
    backoff := 1 * time.Second
    for attempts := 0; attempts < 3; attempts++ {
        cmd := exec.CommandContext(ctx, "python3", "-m", "llamafirewall.server",
            "--socket", s.socketPath)
        if err := cmd.Start(); err != nil {
            s.markUnavailable()
            time.Sleep(backoff)
            backoff *= 2
            continue
        }
        s.markAvailable()
        attempts = 0 // reset on successful start
        cmd.Wait()   // blocks until exit
        s.markUnavailable()
        time.Sleep(backoff)
        backoff *= 2
    }
    s.markPermanentlyUnavailable() // 3 failures: stop retrying, log critical
}
```

---

## Data Flow

### Hook Handler Flow (Critical Path, ~20-40ms p95)

```
Agent invokes PreToolUse hook
    |
    v
OS spawns fresh beekeeper process (~5-10ms)
    |
    v
beekeeper check:
    Load config from disk (< 1ms, small JSON)
    Open catalog index (mmap, < 5ms)
    Decode stdin JSON (< 2ms, bounded 1MB)
    |
    v
policy.Evaluate():
    catalog.Lookup(ecosystem, pkg, version)    -- O(1) hash map
    corroborate(matches)                       -- O(sources) ~ns
    releaseAge.Check(publishedAt)              -- O(1) comparison
    lifecycle.Check(scripts)                  -- O(allowlist) ~ns
    paths.Check(targetPath)                   -- O(blocklist) ~ns
    egress.Check(url, size)                   -- O(allowlist) ~ns
    baseline.Check(tool, target)              -- O(1) counter lookup
    |
    v
Decision (allow / warn / block / block+quarantine)
    |
    +----> audit.Write() -- single append syscall
    |
    +----> if block: fmt.Fprintln(stderr, reason); exit 2
           if allow: exit 0
```

### Gateway MCP Proxy Flow (Per Tool Call)

```
Agent sends tools/call JSON-RPC
    |
    v
gateway.Proxy.readRequest()
    |
    v
Token verification (per-session, in-memory check)
    |
    v
policy.Evaluate() [same engine as check handler]
    |
    +-- if Block: return JSON-RPC error to client
    |             audit.Write(block record)
    |             continue loop (connection stays open)
    |
    +-- if Allow/Warn:
        forward to upstream MCP server (context with timeout)
        await response
        [if LlamaFirewall enabled: scan response for prompt injection, async with timeout]
        audit.Write(allow record, response metadata)
        return response to client
```

### Sentry Event Processing Flow (Continuous)

```
OS kernel events (fanotify / eslogger / ETW)
    |
    v
Platform ingestion goroutine
    normalize to SentryEvent{Pid, ParentPid, ExePath, Files, NetworkDst, Timestamp}
    |
    v
buffered channel (cap 10000) -- absorbs burst during npm install (thousands of events)
    |
    v
Correlation engine goroutine:
    ProcessTree.Update(event)          -- maintain pid→parent map
    SlidingWindow.Add(event)           -- 5-minute rolling window per process subtree
    for each rule:
        rule.Match(window, bumblebeeInventory) -- O(rules) = O(5) in v1
        if match: emit SentryAlert
    |
    v
SentryAlert
    |
    +----> audit.Write(critical record with full provenance)
    +----> IPC: notify connected CLI clients (TUI update)
    +----> optional: desktop notification
```

### Catalog Sync Flow (Hourly + Delta-Triggered)

```
Timer fires (default 1h) OR new catalog drop detected
    |
    v
HTTP fetch from upstream sources (Bumblebee threat_intel, OSV, Socket)
    |
    v
Signature verification (reject unsigned, warn on missing sig)
    |
    v
Sanity bounds check (reject deltas > configured max)
    |
    v
Build new catalog.Index in background goroutine
    |
    v
Atomic pointer swap (sync.RWMutex) -- zero-downtime hot reload
    |
    v
Delta analysis: new entries since last sync?
    if yes, any new entries matching installed packages?
        trigger beekeeper scan --deep (spawn Bumblebee subprocess)
```

---

## Component Boundaries

| Boundary | Interface | Direction | Authorization |
|----------|-----------|-----------|---------------|
| Agent → Hook handler | stdin (JSON) + exit code | Agent writes, handler reads | None (agent is trusted caller) |
| Agent → Gateway | TCP `127.0.0.1:N` (MCP JSON-RPC) | Bidirectional | Per-session token |
| CLI → Sentry daemon | Unix socket / named pipe | CLI commands → Sentry; alerts ← Sentry | SO_PEERCRED / pipe ACL |
| Gateway → Upstream MCP | TCP (MCP JSON-RPC) | Bidirectional | Upstream's own auth |
| Beekeeper → LlamaFirewall | Unix socket / named pipe | Beekeeper sends, sidecar responds | Socket ownership (same process tree) |
| All components → Audit log | File append | Write-only | OS file permissions (0600) |
| Beekeeper → Bumblebee | subprocess exec + stdout pipe | Beekeeper spawns, reads NDJSON output | Process ownership |
| Catalog sync → Upstream | HTTPS | Fetch only | TLS + catalog signatures |

---

## Scaling Considerations

This is a single-developer tool, not a distributed system. Scaling considerations are about resource efficiency on a single machine, not horizontal scaling.

| Load | Concern | Architecture Adjustment |
|------|---------|------------------------|
| 50-200 hook calls/hour (normal dev session) | Process spawn overhead | None needed; cold start is ~20ms, total cost negligible |
| 500+ hook calls/hour (heavy agent session) | Audit log write contention | Buffered writer with periodic flush (not per-call syscall); already planned |
| Large catalogs (>10MB threat_intel JSON) | Catalog parse time on `check` startup | Pre-build binary index on `catalogs sync`, mmap on load; target <5ms |
| npm install (1000s of fs events in seconds) | Sentry event buffer saturation | Buffered channel (10k cap) + event sampling in degraded mode |
| Multiple concurrent agents (MCP gateway) | Per-connection goroutine memory | Cap at 10 concurrent connections; each goroutine ~8KB stack |
| LlamaFirewall inference latency | Gateway p99 latency | Async with timeout; configurable sample rate for resource-constrained machines |

---

## Anti-Patterns

### Anti-Pattern 1: Long-Running beekeeper check via Socket

**What people do:** Run `beekeeper check` as a long-lived daemon that accepts hook requests over a local socket, to amortize startup cost.

**Why it's wrong for Beekeeper:** It introduces an IPC boundary on the critical path (every hook call), adds a persistent process that must be managed, and creates a new attack surface (the socket). The startup cost (~20ms) doesn't justify the added complexity. If startup cost becomes a real problem (measurable, not theoretical), the right solution is the binary catalog index approach — not a socket daemon.

**Do this instead:** Keep `beekeeper check` ephemeral. Optimize catalog loading (mmap + pre-built binary index). Measure actual p95 latency before adding architecture.

### Anti-Pattern 2: Shared In-Process State Between check Invocations

**What people do:** Use init() functions or package-level globals to cache state between `beekeeper check` invocations within the same process (doesn't apply to ephemeral model, but relevant if someone implements the socket daemon anti-pattern above).

**Why it's wrong:** Each hook invocation is a separate process. Package-level state is per-process and does not persist. Attempting to share state across invocations requires a daemon, which brings all the complexity above.

**Do this instead:** Treat each `beekeeper check` invocation as stateless. The catalog index is the only "state" — load it fast from disk on each startup.

### Anti-Pattern 3: Policy Logic in the Gateway Proxy Layer

**What people do:** Implement policy as middleware specific to the gateway, separate from the hook handler's policy logic.

**Why it's wrong:** Policy divergence. The hook handler and gateway must enforce identical policy for the same tool call. Having two policy implementations means they will drift.

**Do this instead:** `internal/policy` is a pure library. Both `internal/check` and `internal/gateway/policy_middleware.go` import and call `policy.Evaluate()`. One implementation, two consumers.

### Anti-Pattern 4: Fanotify for Extension Directory Watching

**What people do:** Use fanotify (privileged) for the extension directory watcher to avoid fsnotify's inotify limitations.

**Why it's wrong for Beekeeper:** The extension watcher runs unprivileged. fanotify requires `CAP_SYS_ADMIN`. Using it for the watcher would force users to run the watcher as root, which defeats the unprivileged-tier value proposition.

**Do this instead:** fsnotify (inotify) for the unprivileged watcher, accepting its limitations. The Sentry daemon's fanotify monitoring (elevated, opt-in) provides the higher-assurance layer. Document the race window honestly. Implement the recursive watch pattern (watch new subdirectories as they appear) to minimize the gap.

### Anti-Pattern 5: Blocking the Correlation Engine Goroutine on Audit Writes

**What people do:** Call `audit.Write()` synchronously inside the Sentry correlation engine goroutine.

**Why it's wrong:** Audit writes involve disk I/O (fsync on critical events) and optional network I/O (remote sinks). Blocking the correlation engine means events pile up in the channel during slow I/O, eventually saturating the buffer.

**Do this instead:** Audit writes in the Sentry path go through a dedicated audit goroutine with a buffered channel. The correlation engine sends `AuditRecord` structs to the channel and continues immediately. The audit goroutine drains the channel and batches writes. Critical events (severity: critical) bypass batching and force a flush.

### Anti-Pattern 6: TCP localhost for Gateway Authentication

**What people do:** Bind the gateway to `127.0.0.1` and treat localhost as a trust boundary.

**Why it's wrong:** Any process on the machine can connect to a localhost TCP socket. A compromised package's postinstall script can talk to the gateway without a token.

**Do this instead:** Per-session token authentication even on localhost (already in the PRD). The token is issued by `beekeeper` at agent setup time, written to the agent's config, and checked on every new connection. Additionally, consider offering Unix socket mode for the gateway (eliminates the network stack entirely; socket file permissions enforce access control).

---

## Build Order Implications

The architecture has clear dependency layers. Build order should follow dependencies, not features:

### Layer 0 — Foundation (must exist before anything else)
1. `internal/config` — layered config loading
2. `internal/audit` — NDJSON writer, 0600 permissions, local file sink
3. `internal/catalog` — JSON loader + two-level hash index (without sync)

### Layer 1 — Core Policy Engine (enables check handler and gateway)
4. `internal/policy` — pure policy evaluation library (depends on catalog.Index)
5. `internal/check` — hook handler (depends on policy, catalog, audit)
6. `cmd/beekeeper` — Cobra root + check subcommand wiring

This is the v0.1.0 deliverable: a working `beekeeper check` with Bumblebee catalog matching and basic audit logging.

### Layer 2 — Extended Policy + Sync (v0.3.0)
7. `internal/catalog/sync` — HTTP fetch, signature verify, delta detect
8. `internal/catalog/watcher` — daemon loop for `beekeeper catalogs watch`
9. `internal/watch` — fsnotify extension directory watcher
10. Policy extensions in `internal/policy`: release-age, lifecycle, paths, egress, baseline

### Layer 3 — Gateway + Sidecar (v0.6.0)
11. `internal/ipc` — cross-platform IPC (Unix socket + named pipe, with auth)
12. `internal/gateway` — MCP proxy daemon (depends on policy, ipc, audit)
13. `internal/llamafirewall` — sidecar supervisor (depends on ipc)
14. `internal/sentry/linux` — fanotify + eBPF ingestion (depends on ipc, audit)
15. `bpf/` — eBPF C programs (compiled via bpf2go as part of layer 3)

### Layer 4 — Cross-Platform Sentry (v0.9.0)
16. `internal/sentry/darwin` — eslogger-based ingestion
17. `internal/sentry/windows` — ETW-based ingestion (tekert/golang-etw, no CGO)

### Layer 5 — TUI + Policy as Code (v1.0.0)
18. `internal/tui` — Bubble Tea dashboard (reads from audit log NDJSON, no privileged channel)
19. Policy as code testing: `beekeeper policy test`, `beekeeper policy validate`
20. `internal/selfdefense` — beekeeper-self catalog check

---

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| Bumblebee | subprocess exec; read NDJSON stdout | Pin Bumblebee version in go.mod or verify hash of downloaded binary |
| OSV database | HTTP fetch of offline DB; cilium/ebpf not relevant here | Use osv-scanner's offline DB mechanism; cache in `~/.beekeeper/catalogs/` |
| Socket public API | HTTPS GET; rate-limited free tier | Cache responses locally; retry with exponential backoff |
| MCP servers | TCP JSON-RPC (MCP protocol) | Standard MCP client library; wrap with policy middleware |
| LlamaFirewall | Unix socket / named pipe; length-prefixed JSON | Beekeeper owns the process lifecycle; sidecar does not outlive Beekeeper |
| Syslog | RFC 5424 over UDP/TCP | Use `log/syslog` from stdlib (available on Linux/macOS; Windows needs a shim) |
| OpenTelemetry | OTLP gRPC or HTTP | `go.opentelemetry.io/otel` — add only if audit sink is configured |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| check ↔ policy | Direct function call (same process) | No channel, no goroutine boundary; must be synchronous |
| gateway ↔ policy | Direct function call (same goroutine) | Policy eval is on the hot path; must not block on I/O |
| sentry ↔ correlation engine | Buffered Go channel (cap 10000) | Decouples kernel ingestion rate from rule evaluation rate |
| correlation engine ↔ audit | Buffered Go channel | Decouples audit I/O from event processing latency |
| CLI ↔ sentry daemon | Unix socket / named pipe (length-prefixed JSON) | Authorized via SO_PEERCRED / pipe ACL; protocol is strictly command/response |
| beekeeper ↔ llamafirewall | Unix socket / named pipe (length-prefixed JSON) | Beekeeper is the client; timeout on every call; fail-closed/open/warn configurable |

---

## Sources

- Go project layout community standard: https://github.com/golang-standards/project-layout (MEDIUM — community convention, not official Go spec)
- Cobra CLI framework: https://github.com/spf13/cobra (HIGH — official repo, widely used in Kubernetes, GitHub CLI, Hugo)
- Go binary startup characteristics: https://replit.com/blog/golang-performance — measured ~11ms for bare binary, 277ms for a program with 25MB eager map literal (avoidable); https://eblog.fly.dev/startfast.html — parallelization and lazy init strategies
- cilium/ebpf library: https://pkg.go.dev/github.com/cilium/ebpf — pure Go, no CGO, ring buffer + perf reader patterns (HIGH)
- cilium/ebpf usage patterns: https://deepwiki.com/cilium/ebpf/6-examples-and-usage-patterns (MEDIUM)
- fanotify Go wrapper: https://github.com/opcoder0/fanotify — notification-only in current state, requires CAP_SYS_ADMIN (HIGH — official repo)
- fsnotify limitations: https://github.com/fsnotify/fsnotify — fanotify backend "Not yet", inotify watch limits, no recursive watching (HIGH — official repo)
- ETW Go library (CGO, older): https://pkg.go.dev/github.com/bi-zone/etw — requires mingw-w64 + CGO (HIGH)
- ETW Go library (no CGO): https://pkg.go.dev/github.com/tekert/golang-etw/etw — pure Go, batched consumer pattern (MEDIUM — less widely used)
- SO_PEERCRED in Go: https://blog.jbowen.dev/2019/09/using-so_peercred-in-go/ and https://github.com/joeshaw/peercred (MEDIUM — pattern is stable, platform coverage varies)
- Windows named pipe ACLs: https://learn.microsoft.com/en-us/windows/win32/ipc/named-pipe-security-and-access-rights (HIGH — official Microsoft docs)
- Go map vs trie performance: https://jmoiron.net/blog/go-performance-tales/ — Go's built-in map outperforms trie at exact-match lookups (MEDIUM)
- Cross-platform IPC in Go: https://github.com/james-barrow/golang-ipc — Unix socket + named pipe abstraction (MEDIUM — smaller project)
- Tetragon architecture overview: https://tetragon.io/ (MEDIUM — limited architectural detail available without source inspection)
- MCP gateway patterns: https://aigateway.envoyproxy.io/blog/mcp-implementation/ and https://github.com/microsoft/mcp-gateway (MEDIUM — emerging ecosystem, patterns not yet settled)

---

*Architecture research for: Beekeeper — Go multi-process agent runtime safety harness*
*Researched: 2026-05-26*
