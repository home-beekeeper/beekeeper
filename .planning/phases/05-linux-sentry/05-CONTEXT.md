# Phase 5: Linux Sentry — Context

**Gathered:** 2026-05-27
**Status:** Ready for planning
**Source:** PRD Express Path (beekeeper-prd.md §5.5, §8.4, §15 + REQUIREMENTS.md SLNX-01–10)

<domain>
## Phase Boundary

Phase 5 delivers the Linux Sentry daemon — an opt-in privileged service that ingests OS-native event streams (eBPF + fanotify) and evaluates them against five narrow correlation rules targeting Nx Console-class attacks. Developers who run `beekeeper protect install` on Linux gain real-time credential exfiltration detection that operates outside the agent hook loop.

Ten requirements in scope (SLNX-01 through SLNX-10):

1. **Systemd service lifecycle** (`beekeeper protect install/uninstall/status`) — installs, enables, and manages the privileged Sentry daemon (SLNX-01).
2. **eBPF process + network ingestion** — process creation/exec events and outbound TCP connection events via `cilium/ebpf v0.21.0` with `bpf2go`-generated bindings; pre-compiled bytecode embedded at build time (SLNX-02, SLNX-04).
3. **fanotify file access ingestion** — reads of sensitive-path watchlist entries via `golang.org/x/sys/unix` with `CAP_SYS_ADMIN` (SLNX-03).
4. **Graceful degradation** — capability probe at startup via `cilium/ebpf/features`; degrade tiers on kernel < 5.15 / < 5.4; surface in `beekeeper protect status` (SLNX-05).
5. **Ring buffer architecture** — dedicated reader goroutine feeds buffered channel (cap 10 000) to correlation engine goroutine (SLNX-06).
6. **IPC with SO_PEERCRED** — Unix socket at `~/.beekeeper/sentry.sock`; peer credential verification via `unix.GetsockoptUcred` so only the installing user's CLI can command the daemon (SLNX-07).
7. **Five default correlation rules** — extension-host credential cluster, credential CLI burst, phone-home, fresh-extension behavior correlation, exfil signature fusion — each emits a Sentry alert NDJSON record (SLNX-08).
8. **7-day audit-only baseline period** — rules emit records but do not quarantine during baseline; configurable to 0 (immediate) or "indefinite" (SLNX-09).
9. **Privilege separation + seccomp-bpf** — eBPF loading is privileged; rule evaluation and event filtering drop capabilities; seccomp-bpf filters on unprivileged Sentry components (SLNX-10).
10. **CI eBPF matrix** — explicit Ubuntu 20.04 (kernel 5.4) and Ubuntu 22.04 (kernel 5.15) coverage; degradation tiers verified per-kernel.

Out of scope for this phase: macOS eslogger (Phase 7), Windows ETW (Phase 7), LlamaFirewall sidecar (Phase 6), TUI dashboard (Phase 8), IPC named pipes (Phase 7).

</domain>

<decisions>
## Implementation Decisions

### Architecture: Where New Code Lives

| Component | Package |
|-----------|---------|
| SentryEvent types + rule types | `internal/sentry/types.go` |
| Correlation engine (pure, no I/O) | `internal/sentry/rules.go` |
| Baseline store (7-day period) | `internal/sentry/baseline.go` |
| eBPF programs (C source) | `internal/sentry/linux/bpf/exec_tracer.bpf.c`, `network_tracer.bpf.c` |
| bpf2go-generated Go bindings | `internal/sentry/linux/bpf_beekeeper_exec_bpfel.go`, `bpf_beekeeper_net_bpfel.go` (auto-generated) |
| eBPF reader + process tree | `internal/sentry/linux/ebpf.go` |
| fanotify reader | `internal/sentry/linux/fanotify.go` |
| Capability probing | `internal/sentry/linux/probe.go` |
| Sentry daemon main loop | `internal/sentry/linux/daemon.go` |
| systemd install/uninstall | `internal/sentry/linux/systemd.go` |
| IPC server (daemon side) | `internal/ipc/server.go` |
| IPC client (CLI side) | `internal/ipc/client.go` |
| IPC protocol types | `internal/ipc/proto.go` |
| CLI wiring | `cmd/beekeeper/main.go` |

All `internal/sentry/linux/` files have `//go:build linux` at the top. The IPC package is Unix-gated (not Windows). The correlation engine in `internal/sentry/rules.go` and types in `internal/sentry/types.go` have no build tags — they compile on all platforms and are testable on Windows.

### SLNX-07: IPC Protocol and SO_PEERCRED

- Unix socket path: `~/.beekeeper/sentry.sock` (platform.StateDir + "/sentry.sock")
- Daemon creates the socket with `net.Listen("unix", sockPath)` and `chmod 0600`
- On each accepted connection: `unix.GetsockoptUcred(int(conn.(*net.UnixConn).SyscallConn()), unix.SOL_SOCKET, unix.SO_PEERCRED)` — returns `(uid, gid, pid)` of the connecting process. Accept only if `ucred.Uid == daemonUID` (the UID that installed Beekeeper).
- Length-prefixed JSON framing: 4-byte big-endian uint32 length prefix followed by the JSON payload. Bounded at 64KB per message.
- IPC commands: `StatusRequest`, `RulesListRequest`, `RulesEnableRequest`, `RulesDisableRequest`
- IPC responses: `StatusResponse`, `RulesListResponse`, `ErrorResponse`
- `internal/ipc/` is shared between CLI and daemon; fuzz-tested (IPC protocol parser is a release gate per CLAUDE.md).

### SLNX-02 + SLNX-04: eBPF Programs

**Pre-compiled bytecode approach (CLAUDE.md requirement: "pre-compiled bytecode, embedded at build time via bpf2go — Never compile at runtime"):**
- eBPF C source lives in `internal/sentry/linux/bpf/`. Two programs:
  - `exec_tracer.bpf.c`: attaches to `tp/sched/sched_process_exec`; emits `ProcessEvent{pid, ppid, uid, exe[256], cmdline[512]}` to ring buffer.
  - `network_tracer.bpf.c`: attaches to `kprobe/tcp_connect` (kernel < 5.15) and `tp/sock/inet_sock_set_state` (kernel >= 5.15); emits `NetworkEvent{pid, ppid, saddr, daddr, dport, proto}` to ring buffer.
- `bpf2go` is invoked via `//go:generate` in `internal/sentry/linux/gen.go` — **one C source per invocation with distinct stems** (bpf2go v0.21.0 does not support multiple C sources in one invocation without bpftool):
  ```go
  //go:generate go tool bpf2go -type ProcessEvent -type NetworkEvent BeekeeperExec ./bpf/exec_tracer.bpf.c -- -D__TARGET_ARCH_x86 -I./bpf/headers
  //go:generate go tool bpf2go -type ProcessEvent -type NetworkEvent BeekeeperNet ./bpf/network_tracer.bpf.c -- -D__TARGET_ARCH_x86 -I./bpf/headers
  ```
- Generated files (`bpf_beekeeper_exec_bpfel.go`, `bpf_beekeeper_exec_bpfel.o`, `bpf_beekeeper_net_bpfel.go`, `bpf_beekeeper_net_bpfel.o`) are committed to the repo — they are the pre-compiled bytecode. No clang at runtime.
- The CI workflow runs `go generate ./internal/sentry/linux/` on Linux runners only (requires clang + libbpf-dev + kernel headers).
- Build tag: `//go:build linux` on all generated and hand-written files in this package.
- BTF CO-RE: the generated ELF has BTF embedded; `cilium/ebpf` loads it with `ebpf.LoadAndAssign` + `btf.LoadKernelSpec()`. On kernels without exposed `/sys/kernel/btf/vmlinux` (kernel < 5.5), the daemon falls back to a pre-embedded vmlinux BTF blob from `cilium/ebpf/internal/sys`.

### SLNX-03: fanotify File Access Events

- `fanotify_init(FAN_CLASS_NOTIF|FAN_NONBLOCK|FAN_CLOEXEC, O_RDONLY|O_LARGEFILE)` via `unix.FanotifyInit`.
- `fanotify_mark` with `FAN_MARK_ADD|FAN_MARK_FILESYSTEM`, `FAN_ACCESS|FAN_OPEN_PERM` on each sensitive-path watchlist entry (file-level marks, not filesystem-wide mounts — to avoid false positives from unrelated processes).
- On kernel < 5.15 (FAN_REPORT_FID not available): fall back to path-level marks without inode identity; mark as degraded tier 1.
- On kernel >= 5.15: use `FAN_REPORT_FID|FAN_REPORT_DFID_NAME` for richer event metadata.
- Event reading goroutine reads from the fanotify fd; extracts `pid`, `fd`→`/proc/<pid>/fd/<fd>` symlink resolution to get the full file path; emits `FileAccessEvent` to the buffered channel.
- Permission events: fanotify returns `FAN_OPEN_PERM`; the daemon always responds with `FAN_ALLOW` (v1 is detection-only, not blocking). The response must be sent promptly to avoid blocking the accessing process.

### SLNX-05: Capability Probing and Degradation Tiers

Three tiers:
- **Tier 0 (Full)**: kernel >= 5.15, eBPF ring buffer + BTF CO-RE available, fanotify FAN_REPORT_FID. All five rules active.
- **Tier 1 (Degraded)**: kernel >= 5.4, eBPF perf buffer (not ring buffer), fanotify without FAN_REPORT_FID. Rules active but with reduced attribution fidelity. `beekeeper protect status` prints "Degraded: using perf buffer (ring buffer requires kernel 5.15+), fanotify without inode identity".
- **Tier 2 (Minimal)**: kernel < 5.4 or missing CAP_SYS_ADMIN/CAP_BPF. fanotify-only (process/network events unavailable). Rules limited to credential-access pattern only. `beekeeper protect status` prints "Degraded: eBPF unavailable (kernel < 5.4 or missing CAP_BPF), process/network events disabled".

Capability probing sequence at daemon startup (via `cilium/ebpf/features`):
1. `features.HaveMapType(ebpf.RingBuf)` → determines ring buffer availability.
2. `features.HaveProgramType(ebpf.Tracing)` → determines CO-RE tracing availability.
3. `unix.FanotifyInit(unix.FAN_CLASS_NOTIF|unix.FAN_REPORT_FID, 0)` → probe FAN_REPORT_FID support.
4. `unix.Capget()` for CAP_SYS_ADMIN, CAP_BPF.

### SLNX-06: Ring Buffer Architecture

```
[eBPF ring buffer / perf buffer]    [fanotify fd]
         │                                │
         ▼                                ▼
   [ebpfReader goroutine]       [fanotifyReader goroutine]
         │                                │
         ▼                                ▼
   ──────────── events chan (cap 10000, SentryEvent) ────────────
                              │
                              ▼
                  [correlationEngine goroutine]
                              │
                  ┌───────────┴───────────┐
                  ▼                       ▼
          [processTree store]    [ruleEvaluator (pure)]
                                          │
                                          ▼
                              [auditWriter — NDJSON alert records]
```

The buffered channel decouples ingestion rate from evaluation latency. If the channel reaches capacity, events are dropped and a counter is incremented (surfaced in `beekeeper diag` as `sentry_events_dropped`).

### SLNX-08: Five Default Correlation Rules

All rules share a `RuleState` struct tracking per-rule triggering windows. The correlation engine is a pure function — it takes an event + current state and returns (updated state, []SentryAlert). State is persisted in `~/.beekeeper/state.json` under the `"sentry"` key.

| Rule ID | Name | Trigger | Window | Severity |
|---------|------|---------|--------|----------|
| SENTRY-001 | ext-host-cred-cluster | editor-descended process reads ≥2 sensitive paths | 60s | critical |
| SENTRY-002 | ext-host-cred-cli-burst | editor-descended process spawns ≥2 credential CLIs | 60s | critical |
| SENTRY-003 | ext-host-phone-home | editor-descended process connects outbound to non-allowlisted domain | 10 min | high |
| SENTRY-004 | fresh-ext-correlation | any of SENTRY-001/002/003 fires AND a Bumblebee-tracked ext was installed ≤30 min ago | — | critical |
| SENTRY-005 | exfil-signature-fusion | sensitive read + outbound connection + same process descended from recently-installed ext, within 5 min | 5 min | critical |

Editor-descended = process whose ancestor chain includes a VS Code, Cursor, Windsurf, or Codium process.

Credential CLIs recognized: `gh`, `aws`, `op`, `vault`, `npm`, `gcloud`, `az`, `heroku`, `fly`, `vercel`, `netlify`, `supabase`.

Sensitive paths (read from policy.DefaultSensitivePaths — reuse Phase 2/4 sensitive path list).

### SLNX-09: Baseline Period

- Baseline state stored in `~/.beekeeper/sentry-baseline.json`: `{"started_at": "<RFC3339>", "duration_days": 7}`.
- During baseline: rules fire and emit NDJSON audit records of type `sentry_alert_baseline` but do NOT trigger desktop notifications or quarantine actions.
- After baseline: rules emit `sentry_alert` records and trigger desktop notifications (via `internal/notify`) on critical/high severity.
- Config keys: `sentry.baseline_days` (int, 0 = immediate enforcement, -1 = indefinite audit-only).

### SLNX-10: Privilege Separation

- The Sentry daemon starts as root (launched by systemd).
- After loading eBPF programs (requires CAP_BPF / CAP_SYS_ADMIN), the daemon drops all capabilities except CAP_NET_ADMIN (needed for fanotify on network sockets) and CAP_DAC_READ_SEARCH (needed for resolving `/proc/<pid>/fd/` symlinks).
- seccomp-bpf filter applied after capability drop: allowlist = `{read, write, close, epoll_wait, epoll_ctl, rt_sigreturn, exit_group, socket, accept, recv, send, openat, fstat, getpid, getsockopt}`. Using `github.com/elastic/go-seccomp-bpf` or raw `unix.PrctlSyscall + BPF_PROG_TYPE_SOCKET_FILTER` depending on what's available in the Go ecosystem as of this phase.
- `//go:build linux` on all privilege-separation code.

### SLNX-01: systemd Service Installation

Generated unit file (template, not hardcoded):
```
[Unit]
Description=Beekeeper Sentry Daemon
After=network.target
StartLimitIntervalSec=0

[Service]
Type=notify
ExecStart=/usr/local/bin/beekeeper sentry
Restart=on-failure
RestartSec=5s
User=root
CapabilityBoundingSet=CAP_SYS_ADMIN CAP_BPF CAP_NET_ADMIN CAP_DAC_READ_SEARCH
AmbientCapabilities=CAP_SYS_ADMIN CAP_BPF CAP_NET_ADMIN CAP_DAC_READ_SEARCH
SecureBits=keep-caps
ProtectSystem=strict
ProtectHome=read-only
NoNewPrivileges=false
RuntimeDirectory=beekeeper

[Install]
WantedBy=multi-user.target
```

`beekeeper protect install` steps:
1. Check if running as root (or prompt with sudo instructions).
2. Copy the `beekeeper` binary to `/usr/local/bin/beekeeper` if not already there.
3. Write the generated unit to `/etc/systemd/system/beekeeper-sentry.service`.
4. `systemctl daemon-reload`.
5. `systemctl enable --now beekeeper-sentry`.
6. Wait up to 5s for the socket at `~/.beekeeper/sentry.sock` to appear; verify with IPC status ping.

`beekeeper protect uninstall`:
1. `systemctl disable --now beekeeper-sentry`.
2. Remove `/etc/systemd/system/beekeeper-sentry.service`.
3. `systemctl daemon-reload`.
4. Remove `~/.beekeeper/sentry.sock`.

`beekeeper protect status`:
1. IPC `StatusRequest` → print degradation tier, active rule count, events processed, events dropped, baseline state, uptime.
2. On IPC failure: exec `systemctl is-active beekeeper-sentry` as fallback.

### Self-Defense Deliverables (per PRD §15, v0.6.0)

- Sentry daemon IPC authorization via `SO_PEERCRED` on Linux (SLNX-07) — same session as the policy hook handler fail-closed posture.
- Privilege separation within Sentry: eBPF loading privileged; rule eval + event filtering drop capabilities (SLNX-10).
- seccomp-bpf filters on unprivileged Sentry components after capability drop (SLNX-10).
- Fuzz tests extended to Sentry rule evaluator (IPC parser fuzz gates are release-blockers per CLAUDE.md §Phase 4).
- eBPF CI matrix: explicit Ubuntu 20.04 (kernel 5.4) and Ubuntu 22.04 (kernel 5.15) — not just ubuntu-latest.

### Claude's Discretion

**Resolved decisions for Phase 5:**

- **eBPF bytecode generation**: `bpf2go` invoked only on Linux CI; generated files (`bpf_bpfel.go`, `bpf_bpfel.o`) committed to the repo as build artifacts. This allows the Go module to compile on Windows (the platform-gated `//go:build linux` guard prevents the C source from being compiled on Windows, but the committed `.go` file compiles fine as it's also `//go:build linux` tagged).
- **seccomp library**: Use `github.com/elastic/go-seccomp-bpf v1.0.2` (pure Go, no CGO). Apply with `FilterFlagTSync` for Go runtime thread safety and `NoNewPrivs: true`. Default action `ActionAllow` + blocklist of dangerous syscalls — safer than a custom allowlist because the Go runtime syscall set is not stable and an overly-narrow allowlist causes SIGSYS crashes (RESEARCH §9 Pitfall 8). The raw `unix.Prctl + SockFilter` path is avoided.
- **Process tree**: Maintained as an in-memory map `map[uint32]ProcessNode` keyed by PID; GC'd entries older than 10 minutes to prevent unbounded growth. Stored in the daemon goroutine — not shared across goroutines; the correlation engine receives a snapshot copy.
- **Sentry alert record type**: New `record_type: "sentry_alert"` (or `"sentry_alert_baseline"` during baseline) extending the existing `AuditRecord` struct with additional fields: `sentry_rule_id`, `process_pid`, `process_exe`, `process_parent_chain []string`, `files_accessed []string`, `network_destinations []string`, `correlated_extension string`.
- **IPC socket location**: `~/.beekeeper/sentry.sock` (in the state directory, not `/run/`). The systemd service runs as root but the socket file is `chown`'d to the installing user with `0600` so only that user's processes can connect. This resolves the root-daemon + user-CLI communication pattern without needing DBUS or other IPC.
- **Windows build**: All `internal/sentry/linux/` code is strictly `//go:build linux`. The `internal/ipc/` package has a `unix.go` (build linux,darwin) and stub `stub.go` (build windows). `internal/sentry/types.go` and `internal/sentry/rules.go` have no build tags — they compile everywhere and are fully unit-testable on Windows.
- **CI matrix for eBPF**: Add two new CI workflow jobs using LVH (`cilium/little-vm-helper@v0.0.21`) on `ubuntu-22.04` hosts: `test-sentry-kernel-5-4` (image-version: "5.4-main") and `test-sentry-kernel-5-15` (image-version: "5.15-main"). ubuntu-20.04 runners are deprecated (removed April 2025); must use LVH for old-kernel testing. Both LVH jobs must be required for a release build — if no `release-gate` aggregation job exists in ci.yml, create one.
- **IPC fuzz test**: Located at `internal/ipc/proto_fuzz_test.go` with `FuzzIPCMessage`. This is a release gate (blocks v0.6.0 tag if fuzz coverage is absent), same category as the MCP parser fuzz from Phase 4.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Architecture and Constraints
- `CLAUDE.md` — eBPF: pre-compiled bytecode embedded at build time via bpf2go (never compile at runtime); ETW: tekert/golang-etw (NOT bi-zone/etw); Windows primary dev machine means all Linux-specific code behind `//go:build linux`
- `.planning/PROJECT.md` — Core decisions, locked choices
- `.planning/REQUIREMENTS.md` — SLNX-01 through SLNX-10 (verbatim requirement text)
- `.planning/ROADMAP.md` — Phase 5 success criteria, dependency on Phase 4

### Prior Phase Patterns (reuse directly)
- `internal/catalog/watch.go` — Canonical foreground daemon pattern with `signal.NotifyContext(SIGINT, SIGTERM)`
- `internal/gateway/gateway.go` — Bounded JSON parser, fail-closed pattern, per-session token, channel-based architecture
- `internal/policy/path.go` — Sensitive path list (reuse `DefaultSensitivePaths` or equivalent)
- `internal/audit/types.go` — AuditRecord struct to extend with sentry_alert fields
- `internal/audit/writer.go` — NDJSON writer pattern (reuse for Sentry alert records)
- `internal/notify/notify.go` — Desktop notification pattern (reuse for Sentry alert notifications)
- `internal/baseline/store.go` — Baseline counter pattern (reference for sentry baseline state)
- `internal/check/handler.go` — Policy engine invocation + fail-closed top-level recover()
- `cmd/beekeeper/main.go` — Cobra wiring patterns; add `protect`, `sentry` subcommand trees here

### External Libraries (new for Phase 5)
- `github.com/cilium/ebpf v0.21.0` — eBPF loading, ring buffer reader, BTF CO-RE, feature probing
- `github.com/cilium/ebpf/cmd/bpf2go` — eBPF bytecode generation (dev dependency, invoked via `go generate`)
- `golang.org/x/sys v0.30.0` (already in go.mod) — unix.FanotifyInit, unix.GetsockoptUcred, unix.Capset, unix.Prctl

</canonical_refs>

<specifics>
## Specific Ideas

### eBPF ProcessEvent Schema (C struct, shared between C and Go via bpf2go)
```c
struct process_event {
    __u32 pid;
    __u32 ppid;
    __u32 uid;
    __u8  exe[256];
    __u8  cmdline[512];
    __u64 ktime_ns;
};
```

### SentryEvent Go Type (internal/sentry/types.go — no build tag, compiles everywhere)
```go
type EventKind uint8
const (
    EventProcessCreate EventKind = iota
    EventFileAccess
    EventNetworkConnect
)

type SentryEvent struct {
    Kind      EventKind
    PID       uint32
    PPID      uint32
    UID       uint32
    Exe       string
    Cmdline   string
    FilePath  string      // EventFileAccess only
    DstAddr   net.IP      // EventNetworkConnect only
    DstPort   uint16      // EventNetworkConnect only
    KTimeNS   uint64      // kernel monotonic timestamp
    WallTime  time.Time   // wall clock at ingestion
}
```

### SentryAlert NDJSON Record (extends AuditRecord concept)
```json
{
    "record_type": "sentry_alert",
    "record_id": "...",
    "timestamp": "...",
    "scanner_name": "beekeeper",
    "sentry_rule_id": "SENTRY-005",
    "sentry_rule_name": "exfil-signature-fusion",
    "severity": "critical",
    "baseline_mode": false,
    "process_pid": 12345,
    "process_exe": "/home/user/.cursor/resources/app/node_modules/.bin/cursor-helper",
    "process_parent_chain": ["cursor", "cursor-helper"],
    "files_accessed": ["~/.ssh/id_rsa", "~/.aws/credentials"],
    "network_destinations": ["52.14.222.1:443"],
    "correlated_extension": "nrwl.nx-console@18.95.0",
    "quarantine_recommended": true
}
```

### Degradation Tier Status Output
```
$ beekeeper protect status
Beekeeper Sentry — Active (PID 98234, uptime 3h42m)
Kernel:     5.4.0-generic (Ubuntu 20.04)
Tier:       Degraded (Tier 1)
Reason:     Ring buffer requires kernel 5.15+ (using perf buffer); FAN_REPORT_FID requires kernel 5.15+ (using path-level marks)
Rules:      5/5 active (audit-only baseline, 4d remaining)
Events:     142,391 processed, 0 dropped
IPC socket: /home/user/.beekeeper/sentry.sock
```

### Process Ancestor Check (critical for rule evaluation)
```go
func isEditorDescendant(pid uint32, tree map[uint32]ProcessNode) bool {
    editorNames := map[string]bool{
        "code": true, "code-insiders": true,
        "cursor": true, "windsurf": true, "codium": true,
    }
    for pid != 0 {
        node, ok := tree[pid]
        if !ok {
            break
        }
        if editorNames[filepath.Base(node.Exe)] {
            return true
        }
        pid = node.PPID
    }
    return false
}
```

### CI Matrix Addition (for eBPF degradation testing)
```yaml
# In .github/workflows/ci.yml — add two new jobs:
test-sentry-kernel-5-4:
  runs-on: ubuntu-20.04   # kernel 5.4
  steps:
    - uses: actions/checkout@v4
    - run: go test -v -tags linux ./internal/sentry/... -run TestDegradationTier

test-sentry-kernel-5-15:
  runs-on: ubuntu-22.04   # kernel 5.15
  steps:
    - uses: actions/checkout@v4
    - run: go test -v -tags linux ./internal/sentry/... -run TestFullTier
```

</specifics>

<deferred>
## Deferred Ideas

- macOS eslogger Sentry — Phase 7
- Windows ETW Sentry — Phase 7
- IPC named pipes for Windows — Phase 7 (with SWIN-05)
- True kernel-mode syscall blocking — v3 roadmap
- Weighted corroboration for Sentry alerts — v2 roadmap (HARD-02)
- TUI Sentry alerts panel — Phase 8 (TUI-03)
- `beekeeper diag` latency display — Phase 9 (CODE-06, but sentry_events_dropped counter wired here, just not surfaced in diag until Phase 9)
- AlertCheck (AlignmentCheck experimental) — Phase 6 (LLMF-04)
- macOS EndpointSecurity entitlement — v2 roadmap

</deferred>

---

*Phase: 05-linux-sentry*
*Context gathered: 2026-05-27 via PRD Express Path (beekeeper-prd.md §5.5, §8.4, §15 + REQUIREMENTS.md SLNX-01–10)*
*Discretion resolved: 2026-05-27 during context synthesis*
