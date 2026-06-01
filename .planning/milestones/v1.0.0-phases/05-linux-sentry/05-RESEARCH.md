# Phase 5: Linux Sentry - Research

**Researched:** 2026-05-27
**Domain:** Linux eBPF (cilium/ebpf v0.21.0), fanotify, seccomp-bpf, systemd, SO_PEERCRED, capability management
**Confidence:** HIGH — all major claims verified against official docs, pkg.go.dev, and primary source code

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

| Component | Package |
|-----------|---------|
| SentryEvent types + rule types | `internal/sentry/types.go` |
| Correlation engine (pure, no I/O) | `internal/sentry/rules.go` |
| Baseline store (7-day period) | `internal/sentry/baseline.go` |
| eBPF programs (C source) | `internal/sentry/linux/bpf/exec_tracer.bpf.c`, `network_tracer.bpf.c` |
| bpf2go-generated Go bindings | `internal/sentry/linux/bpf_bpfel.go` (auto-generated) |
| eBPF reader + process tree | `internal/sentry/linux/ebpf.go` |
| fanotify reader | `internal/sentry/linux/fanotify.go` |
| Capability probing | `internal/sentry/linux/probe.go` |
| Sentry daemon main loop | `internal/sentry/linux/daemon.go` |
| systemd install/uninstall | `internal/sentry/linux/systemd.go` |
| IPC server (daemon side) | `internal/ipc/server.go` |
| IPC client (CLI side) | `internal/ipc/client.go` |
| IPC protocol types | `internal/ipc/proto.go` |
| CLI wiring | `cmd/beekeeper/main.go` |

- All `internal/sentry/linux/` files carry `//go:build linux` build tag
- IPC package: unix.go (build linux,darwin), stub.go (build windows)
- `internal/sentry/types.go` and `internal/sentry/rules.go`: no build tags — compile everywhere
- seccomp library: raw `golang.org/x/sys/unix` syscall path (no external seccomp library) to minimize deps
- Process tree: in-memory `map[uint32]ProcessNode`, GC'd entries older than 10 minutes
- IPC socket: `~/.beekeeper/sentry.sock`, chown'd to installing user with `0600`
- IPC framing: 4-byte big-endian uint32 length prefix + JSON payload, bounded at 64KB
- Degradation tiers: Tier 0 (>=5.15), Tier 1 (>=5.4 ring buf absent), Tier 2 (<5.4 or no CAP_BPF)
- Ring buffer channel capacity: 10,000 events, drops counted in `sentry_events_dropped`
- Sentry alert record: new `record_type: "sentry_alert"` / `"sentry_alert_baseline"` extending AuditRecord
- eBPF bytecode pre-compiled via bpf2go on Linux CI; generated `.go` and `.o` files committed to repo
- fanotify: always respond FAN_ALLOW (v1 is detection-only, never blocking)

### Claude's Discretion

All discretion items resolved in CONTEXT.md (see above). No open discretion areas.

### Deferred Ideas (OUT OF SCOPE)

- macOS eslogger Sentry — Phase 7
- Windows ETW Sentry — Phase 7
- IPC named pipes for Windows — Phase 7
- True kernel-mode syscall blocking — v3 roadmap
- Weighted corroboration for Sentry alerts — v2 roadmap
- TUI Sentry alerts panel — Phase 8
- `beekeeper diag` latency display — Phase 9
- AlertCheck (AlignmentCheck experimental) — Phase 6

</user_constraints>

---

## Summary

Phase 5 builds the Linux Sentry daemon: a privileged systemd service that ingests eBPF process/network events and fanotify file-access events, evaluates five correlation rules for Nx Console-class attacks, and communicates with the unprivileged CLI over a Unix socket authenticated by SO_PEERCRED. All five requirement areas have been verified against official Go package documentation and primary source code.

The primary challenge is multi-kernel portability: cilium/ebpf v0.21.0 handles CO-RE relocation at load time (a single pre-compiled `.bpf.o` works on 5.4–6.x kernels), but ring buffer (`BPF_MAP_TYPE_RINGBUF`) requires kernel 5.8+ and fanotify `FAN_REPORT_FID` requires 5.1+. The degradation tier logic uses `features.HaveMapType(ebpf.RingBuf)` and `unix.FanotifyInit` probes to select the correct code path at daemon startup. Ubuntu 20.04 is deprecated from GitHub Actions as of April 2025; the CI matrix must use LVH (Little VM Helper) with kernel images from `quay.io/lvh-images/` to test against 5.4 and 5.15.

**Primary recommendation:** Use `go tool bpf2go` (registered via `go get -tool`) for code generation, one `//go:generate` per C source file, with separate stems (`BeekeeperExec`, `BeekeeperNet`). Use `github.com/elastic/go-seccomp-bpf` for seccomp-bpf (pure Go, no CGO, handles `PR_SET_NO_NEW_PRIVS + SECCOMP_FILTER_FLAG_TSYNC`). Use `github.com/coreos/go-systemd/v22/daemon` for `sd_notify` READY=1. Use LVH GitHub Action (`cilium/little-vm-helper@v0.0.21`) with `image-version: 5.4-main` and `5.15-main` for CI kernel matrix.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| eBPF event ingestion (process/network) | Linux Sentry daemon (privileged) | — | Requires CAP_BPF/CAP_SYS_ADMIN; kernel-space eBPF program runs as ring buffer producer |
| fanotify file access ingestion | Linux Sentry daemon (privileged) | — | Requires CAP_SYS_ADMIN; fanotify fd is privileged |
| Correlation rule evaluation | `internal/sentry/rules.go` (pure function) | Daemon goroutine owns state | Pure function for testability; daemon goroutine manages RuleState lifetime |
| 7-day baseline state | `internal/sentry/baseline.go` | `~/.beekeeper/sentry-baseline.json` | In-process store + persisted JSON for restart survival |
| IPC authentication (SO_PEERCRED) | Daemon IPC server (`internal/ipc/server.go`) | — | Daemon verifies peer UID on each accepted connection |
| Capability drop + seccomp | Daemon startup (after eBPF load) | `internal/sentry/linux/privilege.go` | Privilege separation: load privileged, run unprivileged |
| systemd service lifecycle | `internal/sentry/linux/systemd.go` | `exec.Command("systemctl", ...)` | Unit file write + systemctl invocation; requires root at install time |
| Alert emission | `internal/audit/writer.go` (reused) | — | NDJSON sentry_alert records use existing audit writer |
| CLI communication (status/rules) | `internal/ipc/client.go` | — | Unprivileged CLI side; SO_PEERCRED verified by daemon |
| Degradation tier detection | `internal/sentry/linux/probe.go` | `cilium/ebpf/features` package | Probe at startup, set global tier, gate each ingestion path |

---

## 1. cilium/ebpf v0.21.0 API Patterns

### Version Confirmation

cilium/ebpf v0.21.0 was released 2026-03-05. It is the current latest release. [VERIFIED: github.com/cilium/ebpf/releases]

### 1.1 bpf2go — Code Generation

**Installing as a tool dependency (required for `go tool bpf2go`):**
```bash
go get -tool github.com/cilium/ebpf/cmd/bpf2go
```
This adds bpf2go to go.mod under the `tool` directive (Go 1.24+). [VERIFIED: pkg.go.dev/github.com/cilium/ebpf/cmd/bpf2go]

**`//go:generate` syntax (current recommended form):**
```go
//go:generate go tool bpf2go -type ProcessEvent -type NetworkEvent BeekeeperExec ./bpf/exec_tracer.bpf.c -- -D__TARGET_ARCH_x86 -I./bpf/headers
//go:generate go tool bpf2go -type ProcessEvent -type NetworkEvent BeekeeperNet ./bpf/network_tracer.bpf.c -- -D__TARGET_ARCH_x86 -I./bpf/headers
```

**One C source file per bpf2go invocation.** bpf2go v0.21.0 does not support multiple C source files in a single invocation without bpftool. The PR (#1762) adding multi-file support was merged but requires bpftool as an additional dependency — avoid this complexity. Instead, use two separate invocations with different ident stems. [VERIFIED: github.com/cilium/ebpf/pull/1762, pkg.go.dev/github.com/cilium/ebpf/cmd/bpf2go]

**Generated files per invocation** (ident = `BeekeeperExec`):
```
bpf_beekeeper_exec_bpfel.go   // little-endian (amd64, arm64)
bpf_beekeeper_exec_bpfeb.go   // big-endian (not needed for Beekeeper but generated)
bpf_beekeeper_exec_bpfel.o    // compiled eBPF bytecode embedded via go:embed
bpf_beekeeper_exec_bpfeb.o
```
All generated files have `//go:build linux` at the top. The `.o` files are embedded into the `.go` file via `//go:embed`. [VERIFIED: cilium/ebpf examples/ringbuffer/main.go]

**Key flags:**
| Flag | Purpose |
|------|---------|
| `-type <name>` | Generate Go declaration for C struct (repeatable) |
| `-cc clang` | C compiler (default: clang; env: `BPF2GO_CC`) |
| `-target bpf` | Single target instead of bpfel+bpfeb (use for Beekeeper: only little-endian matters) |
| `-no-global-types` | Skip auto-generating types for map keys/values (optional cleanup) |
| `-output-dir <dir>` | Where to write generated files |
| `--` | Separator; everything after is passed to clang |

**Note on `-target`:** Using `-target bpf` generates only one pair of files (generic BPF, not endian-specific). Using the default `bpfel,bpfeb` generates two pairs. For Beekeeper's amd64/arm64 target platforms, `-target bpf` is fine and reduces generated file count. [ASSUMED — based on bpf2go documentation; recommend verifying on CI]

### 1.2 Generated Struct Pattern and LoadAndAssign

bpf2go generates a struct named `<Ident>Objects` with `ebpf` tags, a `loadBeekeeperExecObjects` function, and a `close` method. The struct fields correspond to eBPF programs and maps declared in the C source. [VERIFIED: pkg.go.dev/github.com/cilium/ebpf, examples/ringbuffer/main.go]

```go
// Auto-generated (do not edit):
type BeekeeperExecObjects struct {
    BeekeeperExecPrograms
    BeekeeperExecMaps
}

type BeekeeperExecPrograms struct {
    TracepointSchedSchedProcessExec *ebpf.Program `ebpf:"tracepoint__sched__sched_process_exec"`
}

type BeekeeperExecMaps struct {
    Events *ebpf.Map `ebpf:"events"` // BPF_MAP_TYPE_RINGBUF
}
```

**Loading pattern:**
```go
// Source: cilium/ebpf examples/ringbuffer/main.go [VERIFIED]
objs := BeekeeperExecObjects{}
if err := loadBeekeeperExecObjects(&objs, nil); err != nil {
    // Inspect error for CollectionOptions.Programs.VerifierOptions.LogLevel
    // to get verifier log on failure
    log.Fatalf("loading eBPF objects: %v", err)
}
defer objs.Close()
```

The `loadBeekeeperExecObjects` function calls `CollectionSpec.LoadAndAssign` internally:
```go
// Direct use of CollectionSpec.LoadAndAssign [VERIFIED: pkg.go.dev/github.com/cilium/ebpf]
spec, err := ebpf.LoadCollectionSpecFromReader(bytes.NewReader(bpfBytes))
if err != nil {
    return err
}
return spec.LoadAndAssign(objs, &ebpf.CollectionOptions{...})
```

**Signature of `CollectionSpec.LoadAndAssign`:** [VERIFIED: pkg.go.dev/github.com/cilium/ebpf@v0.21.0]
```go
func (cs *CollectionSpec) LoadAndAssign(to interface{}, opts *CollectionOptions) error
```
`to` must be a pointer to a struct; fields are populated by `ebpf:"name"` tags. Accepts `*ebpf.Program` and `*ebpf.Map` fields.

### 1.3 Ring Buffer Reader

[VERIFIED: pkg.go.dev/github.com/cilium/ebpf@v0.21.0/ringbuf]

```go
import "github.com/cilium/ebpf/ringbuf"

// Create reader from the ring buffer map
rd, err := ringbuf.NewReader(objs.Events)  // objs.Events is *ebpf.Map of type RingBuf
if err != nil {
    return fmt.Errorf("opening ringbuf reader: %w", err)
}
defer rd.Close()

// Reader goroutine (from signal, close rd to unblock)
go func() {
    <-ctx.Done()
    rd.Close()
}()

var rec ringbuf.Record
for {
    if err := rd.ReadInto(&rec); err != nil {
        if errors.Is(err, ringbuf.ErrClosed) {
            return nil  // clean shutdown
        }
        log.Printf("ringbuf read error: %v", err)
        continue
    }
    // Parse raw bytes
    var event BeekeeperExecEvent  // generated by bpf2go from -type ProcessEvent
    if err := binary.Read(bytes.NewBuffer(rec.RawSample), binary.LittleEndian, &event); err != nil {
        log.Printf("parsing event: %v", err)
        continue
    }
    eventCh <- toSentryEvent(event)
}
```

**Key types:** [VERIFIED]
```go
type Record struct {
    RawSample []byte
    Remaining int
}

func NewReader(ringbufMap *ebpf.Map) (*Reader, error)
func (r *Reader) Read() (Record, error)
func (r *Reader) ReadInto(rec *Record) error  // preferred: reuses buffer
func (r *Reader) Close() error
func (r *Reader) SetDeadline(t time.Time)
func (r *Reader) Flush() error
```

### 1.4 Perf Buffer Fallback (Tier 1 — kernel 5.4–5.7)

Ring buffer (`BPF_MAP_TYPE_RINGBUF`) requires kernel 5.8+. On 5.4, use perf event array. [VERIFIED: pkg.go.dev/github.com/cilium/ebpf@v0.21.0/perf]

```go
import "github.com/cilium/ebpf/perf"

// perCPUBuffer: per-CPU buffer size in bytes (must be page-aligned, typically 4096 or 8192)
rd, err := perf.NewReader(objs.PerfEvents, 4096)

// perf.Record has same RawSample field; also has LostSamples counter
type Record struct {
    CPU         int
    RawSample   []byte
    LostSamples uint64  // non-zero if ring was full
    Remaining   int
}

func NewReader(array *ebpf.Map, perCPUBuffer int) (*Reader, error)
func (pr *Reader) Read() (Record, error)
func (pr *Reader) ReadInto(rec *Record) error
func (pr *Reader) Close() error
```

**C-side difference:** Perf event uses `BPF_MAP_TYPE_PERF_EVENT_ARRAY` and `bpf_perf_event_output()` instead of `BPF_MAP_TYPE_RINGBUF` and `bpf_ringbuf_reserve/submit`. The C source must have conditional compilation (`#ifdef PERF_BUFFER`) or two separate source files for ring vs perf. The easiest approach is to always emit to a ring buffer in the C code and fall back to the perf map only by switching the Go reader — but the C must also declare the perf map. Beekeeper's CONTEXT.md resolves this: declare both map types in C but only attach the appropriate reader based on tier.

### 1.5 Feature Probing

[VERIFIED: pkg.go.dev/github.com/cilium/ebpf@v0.21.0/features]

```go
import (
    "github.com/cilium/ebpf"
    "github.com/cilium/ebpf/features"
)

// Ring buffer availability (kernel 5.8+)
if err := features.HaveMapType(ebpf.RingBuf); err != nil {
    if errors.Is(err, ebpf.ErrNotSupported) {
        // Fall back to perf buffer (Tier 1)
    }
    // Other errors: log but continue
}

// Tracing program type (needed for CO-RE BTF tracepoints, kernel 5.5+)
if err := features.HaveProgramType(ebpf.Tracing); err != nil {
    if errors.Is(err, ebpf.ErrNotSupported) {
        // Fall back to raw tracepoint or kprobe
    }
}

// Kernel version (for informational display only; do NOT gate features on version)
if code, err := features.LinuxVersionCode(); err == nil {
    major := (code >> 16) & 0xFF
    minor := (code >> 8) & 0xFF
    // e.g., 5.15 = code 0x050F00
}
```

**Note:** `features.LinuxVersionCode()` documentation explicitly warns: "Do not use version to assume feature presence; always use feature probes instead." [VERIFIED: pkg.go.dev/github.com/cilium/ebpf@v0.21.0/features]

### 1.6 BTF CO-RE and vmlinux Fallback

[VERIFIED: pkg.go.dev/github.com/cilium/ebpf@v0.21.0/btf]

```go
import "github.com/cilium/ebpf/btf"

// Automatically reads /sys/kernel/btf/vmlinux (kernel 5.2+)
// Falls back to scanning filesystem for vmlinux ELFs if sysfs absent
spec, err := btf.LoadKernelSpec()
if err != nil {
    // errors.Is(err, ebpf.ErrNotSupported): BTF not available at all (very old kernels)
    // cilium/ebpf handles this transparently via pre-embedded BTF in the .o file
}
```

**cilium/ebpf handles BTF transparently:** When bpf2go compiles the C source, it embeds BTF into the ELF. At load time, `cilium/ebpf` uses this embedded BTF for CO-RE relocation — the caller does not need to call `btf.LoadKernelSpec()` directly. It is called internally by `LoadAndAssign` / `NewCollection`. [VERIFIED: cilium/ebpf source, ebpf-go.dev/concepts/rlimit]

### 1.7 rlimit.RemoveMemlock

```go
import "github.com/cilium/ebpf/rlimit"

// Call once at daemon startup before loading eBPF objects
// No-op on kernel 5.11+ (cgroup memory accounting)
// Requires CAP_SYS_RESOURCE on kernel < 5.11
if err := rlimit.RemoveMemlock(); err != nil {
    return fmt.Errorf("removing memlock: %w", err)
}
```

[VERIFIED: pkg.go.dev/github.com/cilium/ebpf@v0.21.0/rlimit]

### 1.8 Attaching Programs

[VERIFIED: pkg.go.dev/github.com/cilium/ebpf@v0.21.0/link]

```go
import "github.com/cilium/ebpf/link"

// Tracepoint: group="sched", name="sched_process_exec"
tp, err := link.Tracepoint("sched", "sched_process_exec",
    objs.BeekeeperExecPrograms.TracepointSchedSchedProcessExec, nil)
if err != nil {
    return fmt.Errorf("attaching exec tracepoint: %w", err)
}
defer tp.Close()

// Kprobe: for tcp_connect on kernel < 5.15
kp, err := link.Kprobe("tcp_connect", objs.BeekeeperNetPrograms.KprobeTcpConnect, nil)
if err != nil {
    return fmt.Errorf("attaching tcp_connect kprobe: %w", err)
}
defer kp.Close()

// Tracepoint for inet_sock_set_state (kernel >= 5.15 preferred)
tp2, err := link.Tracepoint("sock", "inet_sock_set_state",
    objs.BeekeeperNetPrograms.TracepointSockInetSockSetState, nil)
```

**Signatures:** [VERIFIED]
```go
func Tracepoint(group, name string, prog *ebpf.Program, opts *TracepointOptions) (Link, error)
func Kprobe(symbol string, prog *ebpf.Program, opts *KprobeOptions) (Link, error)
func Kretprobe(symbol string, prog *ebpf.Program, opts *KprobeOptions) (Link, error)

type Link interface {
    Close() error
    Update(*ebpf.Program) error
    Pin(string) error
    Unpin() error
    Detach() error
    Info() (*Info, error)
}
```

---

## 2. eBPF Program Patterns

### 2.1 Process Creation: sched_process_exec Tracepoint

[VERIFIED: github.com/mozillazg/hello-libbpfgo/blob/master/37-tracepoint-sched_process_exec/main.bpf.c]

Available on kernel 5.4+. The tracepoint fires after `execve` succeeds.

```c
//go:build ignore
// exec_tracer.bpf.c

#include "vmlinux.h"
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

#define EXE_MAX_LEN 256
#define ARGV_MAX_LEN 512

struct process_event {
    __u32 pid;
    __u32 ppid;
    __u32 uid;
    __u8  exe[EXE_MAX_LEN];
    __u8  cmdline[ARGV_MAX_LEN];
    __u64 ktime_ns;
};

// Ring buffer map (kernel 5.8+)
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 24);  // 16MB
} process_events SEC(".maps");

// Perf event array map (fallback, kernel 4.4+)
struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __uint(key_size, sizeof(__u32));
    __uint(value_size, sizeof(__u32));
} process_events_perf SEC(".maps");

SEC("tracepoint/sched/sched_process_exec")
int tracepoint__sched__sched_process_exec(
        struct trace_event_raw_sched_process_exec *ctx) {

    struct process_event *ev;
    ev = bpf_ringbuf_reserve(&process_events, sizeof(*ev), 0);
    if (!ev)
        return 0;

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();

    // PID (thread group ID = process ID in Linux terminology)
    ev->pid  = bpf_get_current_pid_tgid() >> 32;
    // PPID: read parent's tgid via CO-RE
    ev->ppid = BPF_CORE_READ(task, real_parent, tgid);
    ev->uid  = bpf_get_current_uid_gid() & 0xFFFFFFFF;
    ev->ktime_ns = bpf_ktime_get_ns();

    // Executable path from tracepoint context (__data_loc field)
    unsigned int fname_off = BPF_CORE_READ(ctx, __data_loc_filename) & 0xFFFF;
    bpf_probe_read_str(ev->exe, sizeof(ev->exe), (void *)ctx + fname_off);

    // Cmdline from task->mm->arg_start (process memory)
    void *arg_start = (void *)BPF_CORE_READ(task, mm, arg_start);
    void *arg_end   = (void *)BPF_CORE_READ(task, mm, arg_end);
    long arg_len = arg_end - arg_start;
    if (arg_len > ARGV_MAX_LEN)
        arg_len = ARGV_MAX_LEN;
    bpf_probe_read(ev->cmdline, arg_len, arg_start);

    bpf_ringbuf_submit(ev, 0);
    return 0;
}

char __license[] SEC("license") = "GPL";
```

**Key points:**
- `bpf_get_current_pid_tgid() >> 32` yields the TGID (process ID as seen by userspace). The lower 32 bits are the thread ID.
- `BPF_CORE_READ(task, real_parent, tgid)` works without `/sys/kernel/btf/vmlinux` because CO-RE relocations are applied at load time using the embedded BTF. [VERIFIED: cilium/ebpf CO-RE docs]
- `__data_loc_filename` is the dynamic tracepoint field locator; mask with `0xFFFF` to get the offset into the raw event buffer. [VERIFIED: mozillazg example]
- Cmdline args are null-separated in `mm->arg_start..arg_end`; `bpf_probe_read` copies them as-is (use `bytes.Split([]byte, []byte{0})` on the Go side to parse individual args).

### 2.2 Network Events: tcp_connect kprobe vs inet_sock_set_state Tracepoint

**Kernel availability:**
- `tcp_connect` kprobe: available from kernel 4.x. Fires in process context — PID is always the connecting process. Gets remote addr from `struct sock *sk`. [VERIFIED: Brendan Gregg TCP tracepoints blog]
- `sock/inet_sock_set_state` tracepoint: available kernel 4.16+. **Warning: may fire outside process context** on state changes triggered by the network stack, meaning `bpf_get_current_pid_tgid()` can return wrong PID. [VERIFIED: Brendan Gregg TCP tracepoints blog]

**Decision for Beekeeper:** Use `kprobe/tcp_connect` on ALL kernel versions. It always fires in process context and gives the correct PID. The `inet_sock_set_state` tracepoint is better for latency measurement (it fires at both SYN_SENT and ESTABLISHED) but worse for attribution. [ASSUMED — recommendation based on evidence; use kprobe for reliability]

```c
//go:build ignore
// network_tracer.bpf.c

#include "vmlinux.h"
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_endian.h>

struct network_event {
    __u32 pid;
    __u32 ppid;
    __u32 uid;
    __u32 saddr;    // source IPv4 (network byte order)
    __u32 daddr;    // dest IPv4 (network byte order)
    __u16 dport;    // dest port (network byte order)
    __u16 sport;    // source port (network byte order)
    __u8  is_ipv6;
    __u8  pad[3];
    __u8  saddr6[16];
    __u8  daddr6[16];
    __u64 ktime_ns;
};

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 24);
} network_events SEC(".maps");

SEC("kprobe/tcp_connect")
int kprobe__tcp_connect(struct pt_regs *ctx) {
    struct sock *sk = (struct sock *)PT_REGS_PARM1(ctx);

    struct network_event *ev;
    ev = bpf_ringbuf_reserve(&network_events, sizeof(*ev), 0);
    if (!ev)
        return 0;

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    ev->pid   = bpf_get_current_pid_tgid() >> 32;
    ev->ppid  = BPF_CORE_READ(task, real_parent, tgid);
    ev->uid   = bpf_get_current_uid_gid() & 0xFFFFFFFF;
    ev->ktime_ns = bpf_ktime_get_ns();

    __u16 family = BPF_CORE_READ(sk, __sk_common.skc_family);
    ev->dport = BPF_CORE_READ(sk, __sk_common.skc_dport);
    ev->sport = BPF_CORE_READ(sk, __sk_common.skc_num);

    if (family == AF_INET) {
        ev->is_ipv6 = 0;
        ev->saddr = BPF_CORE_READ(sk, __sk_common.skc_rcv_saddr);
        ev->daddr = BPF_CORE_READ(sk, __sk_common.skc_daddr);
    } else if (family == AF_INET6) {
        ev->is_ipv6 = 1;
        BPF_CORE_READ_INTO(ev->saddr6, sk,
            __sk_common.skc_v6_rcv_saddr.in6_u.u6_addr8);
        BPF_CORE_READ_INTO(ev->daddr6, sk,
            __sk_common.skc_v6_daddr.in6_u.u6_addr8);
    }

    bpf_ringbuf_submit(ev, 0);
    return 0;
}

char __license[] SEC("license") = "GPL";
```

**Go-side port conversion:** `bpf_ntohs(dport)` is needed — in Go: `binary.BigEndian.Uint16([]byte{byte(ev.Dport >> 8), byte(ev.Dport)})` or use `net.IP` parsing with `binary.BigEndian`.

### 2.3 Ring Buffer Map Declaration in C

[VERIFIED: github.com/cilium/ebpf/blob/v0.21.0/examples/ringbuffer/ringbuffer.c]

```c
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 24);  // Must be power of 2, page-aligned; 16MB
    __type(value, struct process_event);  // Optional type annotation for bpf2go -type
} events SEC(".maps");
```

Event submission pattern:
```c
// Reserve + submit (zero-copy, preferred)
struct process_event *ev = bpf_ringbuf_reserve(&events, sizeof(*ev), 0);
if (!ev) return 0;
// ... fill fields ...
bpf_ringbuf_submit(ev, 0);

// Or discard on error:
bpf_ringbuf_discard(ev, 0);
```

### 2.4 vmlinux.h Generation

For CO-RE, include `vmlinux.h` (generated from kernel BTF) instead of kernel headers. On CI runners:
```bash
bpftool btf dump file /sys/kernel/btf/vmlinux format c > internal/sentry/linux/bpf/vmlinux.h
```
Or use a pre-generated `vmlinux.h` from the cilium/ebpf headers directory. Commit the generated `vmlinux.h` to the repo so Windows compilation does not need BTF tooling. [ASSUMED — standard practice; confirm BTF availability on ubuntu-22.04 CI runners]

---

## 3. fanotify API

### 3.1 Function Signatures

[VERIFIED: pkg.go.dev/golang.org/x/sys@v0.30.0/unix, man7.org/linux/man-pages/man2/fanotify_init.2.html]

```go
import "golang.org/x/sys/unix"

// fanotify_init(2)
func FanotifyInit(flags uint, event_f_flags uint) (fd int, err error)

// fanotify_mark(2)
func FanotifyMark(fd int, flags uint, mask uint64, dirFd int, pathname string) (err error)
```

### 3.2 Key Constants

**FanotifyInit flags:**
```go
unix.FAN_CLASS_NOTIF    // Default class; no CAP_SYS_ADMIN required for notification events
unix.FAN_CLASS_CONTENT  // For permission events (FAN_OPEN_PERM, FAN_ACCESS_PERM); requires CAP_SYS_ADMIN
unix.FAN_NONBLOCK       // Return EAGAIN instead of blocking
unix.FAN_CLOEXEC        // Set O_CLOEXEC on the fd
unix.FAN_REPORT_FID     // Include inode info; kernel 5.1+
unix.FAN_REPORT_DFID_NAME  // Synonym for FAN_REPORT_DIR_FID|FAN_REPORT_NAME; kernel 5.9+
unix.FAN_REPORT_PIDFD   // Include process fd; kernel 5.15+
```

**FanotifyMark flags:**
```go
unix.FAN_MARK_ADD           // Add to existing mask
unix.FAN_MARK_REMOVE        // Remove from mask
unix.FAN_MARK_FLUSH         // Flush all marks
unix.FAN_MARK_DONT_FOLLOW   // Don't follow symlinks
unix.FAN_MARK_ONLYDIR       // Error if not directory
unix.FAN_MARK_MOUNT         // Mark the entire mount
unix.FAN_MARK_FILESYSTEM    // Mark the entire filesystem
```

**Event masks:**
```go
unix.FAN_ACCESS         // File was accessed (read)
unix.FAN_OPEN           // File was opened
unix.FAN_OPEN_PERM      // Permission check before open (requires FAN_CLASS_CONTENT)
unix.FAN_ACCESS_PERM    // Permission check before access (requires FAN_CLASS_CONTENT)
unix.FAN_CLOSE_WRITE    // File was closed (after write)
unix.FAN_CLOSE_NOWRITE  // File was closed (no write)
```

### 3.3 Kernel Version Differences

| Feature | Minimum Kernel | Notes |
|---------|---------------|-------|
| `FAN_CLASS_NOTIF` | 2.6.37 | Basic notification; no CAP_SYS_ADMIN for FAN_CLASS_NOTIF |
| `FAN_OPEN_PERM` | 2.6.37 | Requires `FAN_CLASS_CONTENT` |
| `FAN_REPORT_TID` | 4.20 | Report thread ID |
| `FAN_REPORT_FID` | 5.1 | Inode identity in events |
| `FAN_REPORT_DFID_NAME` | 5.9 | Dir FID + filename |
| `FAN_REPORT_PIDFD` | 5.15 | Process fd in events |

**Beekeeper degradation tier mapping:**
- Tier 0 (>=5.15): `FAN_CLASS_CONTENT|FAN_NONBLOCK|FAN_CLOEXEC|FAN_REPORT_FID|FAN_REPORT_PIDFD`
- Tier 1 (5.1–5.14): `FAN_CLASS_CONTENT|FAN_NONBLOCK|FAN_CLOEXEC|FAN_REPORT_FID`
- Tier 2 (2.6.37–5.0): `FAN_CLASS_CONTENT|FAN_NONBLOCK|FAN_CLOEXEC` (path-level marks only)

**Probing for FAN_REPORT_FID at startup:**
```go
// Try with FAN_REPORT_FID; if EINVAL, fall back without it
fd, err := unix.FanotifyInit(unix.FAN_CLASS_CONTENT|unix.FAN_NONBLOCK|unix.FAN_REPORT_FID, 0)
if err != nil {
    if errors.Is(err, unix.EINVAL) {
        // kernel < 5.1 or FAN_CLASS_CONTENT without FAN_REPORT_FID support
        fd, err = unix.FanotifyInit(unix.FAN_CLASS_CONTENT|unix.FAN_NONBLOCK, unix.O_RDONLY)
    }
    if err != nil {
        return fmt.Errorf("fanotify_init: %w", err)
    }
}
```

### 3.4 Adding Marks

```go
// Mark a specific file path (not filesystem-wide, to avoid false positives)
err = unix.FanotifyMark(fd,
    unix.FAN_MARK_ADD,
    unix.FAN_ACCESS|unix.FAN_OPEN_PERM,
    unix.AT_FDCWD,
    "/home/user/.ssh/id_rsa")

// OR mark a directory with recursive monitoring (use FAN_MARK_FILESYSTEM for FS-wide)
err = unix.FanotifyMark(fd,
    unix.FAN_MARK_ADD|unix.FAN_MARK_ONLYDIR,
    unix.FAN_ACCESS|unix.FAN_OPEN_PERM|unix.FAN_ONDIR,
    unix.AT_FDCWD,
    "/home/user/.ssh/")
```

### 3.5 Reading Events

[VERIFIED: man7.org/linux/man-pages/man7/fanotify.7.html, pkg.go.dev/golang.org/x/sys@v0.30.0/unix]

```go
type FanotifyEventMetadata struct {
    Event_len    uint32
    Vers         uint8
    Reserved     uint8
    Metadata_len uint16
    Mask         uint64
    Fd           int32   // -1 if FAN_REPORT_FID in use; file is identified by file_handle
    Pid          int32   // PID of the process that caused the event
}
```

**Reading loop:**
```go
buf := make([]byte, 4096)
for {
    n, err := unix.Read(fanFd, buf)
    if err != nil {
        if errors.Is(err, unix.EAGAIN) {
            // FAN_NONBLOCK: no events ready; use epoll or sleep
            break
        }
        return err
    }
    
    offset := 0
    for offset < n {
        meta := (*unix.FanotifyEventMetadata)(unsafe.Pointer(&buf[offset]))
        if meta.Event_len < unix.FAN_EVENT_METADATA_LEN {
            break // malformed
        }
        
        // Get file path via /proc/self/fd symlink
        if meta.Fd >= 0 {
            path, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", meta.Fd))
            if err == nil {
                // path is the full file path
            }
            unix.Close(int(meta.Fd)) // MUST close the event fd promptly
        }
        
        // Respond to permission events immediately
        if meta.Mask&unix.FAN_OPEN_PERM != 0 {
            resp := unix.FanotifyResponse{
                Fd:       meta.Fd,
                Response: unix.FAN_ALLOW,
            }
            buf2 := make([]byte, 8)
            binary.LittleEndian.PutUint32(buf2[0:4], uint32(resp.Fd))
            binary.LittleEndian.PutUint32(buf2[4:8], resp.Response)
            unix.Write(fanFd, buf2)
        }
        
        offset += int(meta.Event_len)
    }
}
```

**Critical:** The fanotify fd must be opened with `O_RDWR` (not `O_RDONLY`) if permission events are used, because responding to `FAN_OPEN_PERM` requires writing to the same fd. For notification-only (`FAN_ACCESS`, `FAN_OPEN`), `O_RDONLY` is sufficient. [VERIFIED: fanotify man page]

**`unix.FanotifyEventMetadata` constant sizes:**
```go
unix.FAN_EVENT_METADATA_LEN  // = unsafe.Sizeof(unix.FanotifyEventMetadata{}) = 24
```

### 3.6 Path Resolution

```go
// Resolve file path from event fd
func resolveEventPath(eventFd int32) string {
    // Use /proc/self/fd/<fd> to get the path (fd is already open in our process)
    link := fmt.Sprintf("/proc/self/fd/%d", eventFd)
    path, err := os.Readlink(link)
    if err != nil {
        // Fallback: /proc/<pid>/fd/<fd> using the event's PID
        return ""
    }
    return path
}
```

**Important:** The event fd in `meta.Fd` is a new file descriptor opened in the fanotify-receiving process's fd table. It must be closed after use. Do not confuse with `/proc/<pid>/fd/` of the originating process — `meta.Fd` is already accessible in the daemon's own fd namespace.

---

## 4. SO_PEERCRED Verification

[VERIFIED: pkg.go.dev/golang.org/x/sys@v0.30.0/unix]

### 4.1 Ucred Struct and GetsockoptUcred

```go
type Ucred struct {
    Pid int32  // NOTE: int32 in unix package (differs from libc pid_t)
    Uid uint32
    Gid uint32
}

func GetsockoptUcred(fd, level, opt int) (value *Ucred, err error)
```

### 4.2 Complete Pattern (daemon side)

```go
import (
    "net"
    "golang.org/x/sys/unix"
)

func verifyPeerUID(conn net.Conn, expectedUID uint32) error {
    unixConn, ok := conn.(*net.UnixConn)
    if !ok {
        return fmt.Errorf("not a Unix connection")
    }
    
    rawConn, err := unixConn.SyscallConn()
    if err != nil {
        return fmt.Errorf("getting raw conn: %w", err)
    }
    
    var ucred *unix.Ucred
    var innerErr error
    
    ctrlErr := rawConn.Control(func(fd uintptr) {
        ucred, innerErr = unix.GetsockoptUcred(
            int(fd),
            unix.SOL_SOCKET,
            unix.SO_PEERCRED,
        )
    })
    if ctrlErr != nil {
        return fmt.Errorf("control: %w", ctrlErr)
    }
    if innerErr != nil {
        return fmt.Errorf("getsockopt SO_PEERCRED: %w", innerErr)
    }
    
    if ucred.Uid != expectedUID {
        return fmt.Errorf("peer UID %d does not match expected %d", ucred.Uid, expectedUID)
    }
    return nil
}
```

### 4.3 Daemon Accept Loop Pattern

```go
listener, err := net.Listen("unix", sockPath)
// chmod 0600 so only the user can connect
if err := os.Chmod(sockPath, 0600); err != nil { ... }
// chown to the installing user
if err := os.Lchown(sockPath, installingUID, installingGID); err != nil { ... }

for {
    conn, err := listener.Accept()
    if err != nil { break }
    
    go func(c net.Conn) {
        defer c.Close()
        if err := verifyPeerUID(c, installingUID); err != nil {
            log.Printf("rejected connection: %v", err)
            return
        }
        handleIPCConn(c)
    }(conn)
}
```

---

## 5. Capability Drop

### 5.1 The Two-Struct Bug

The `unix.Capget` and `unix.Capset` functions with `LINUX_CAPABILITY_VERSION_3` write 64-bit capability fields across **two** `CapUserData` structs (lower 32 bits in `[0]`, upper 32 bits in `[1]`). Passing a single struct causes a heap overwrite. [VERIFIED: github.com/golang/go/issues/44312]

### 5.2 Correct Capget/Capset Pattern

[VERIFIED: pkg.go.dev/golang.org/x/sys@v0.30.0/unix, golang/go#44312]

```go
import "golang.org/x/sys/unix"

const (
    capBit = func(cap uintptr) (index int, mask uint32) {
        return int(cap / 32), uint32(1 << (cap % 32))
    }
)

func dropCapabilities(keepCaps []uintptr) error {
    hdr := unix.CapUserHeader{
        Version: unix.LINUX_CAPABILITY_VERSION_3,
        Pid:     0, // 0 = current process
    }
    var data [2]unix.CapUserData
    
    if err := unix.Capget(&hdr, &data[0]); err != nil {
        return fmt.Errorf("capget: %w", err)
    }
    
    // Zero out all capabilities
    data[0].Effective   = 0
    data[0].Permitted   = 0
    data[0].Inheritable = 0
    data[1].Effective   = 0
    data[1].Permitted   = 0
    data[1].Inheritable = 0
    
    // Re-enable only the kept caps
    for _, cap := range keepCaps {
        idx := int(cap / 32)
        bit := uint32(1 << (cap % 32))
        if idx == 0 {
            data[0].Effective   |= bit
            data[0].Permitted   |= bit
        } else {
            data[1].Effective   |= bit
            data[1].Permitted   |= bit
        }
    }
    
    return unix.Capset(&hdr, &data[0])
}

// Usage after eBPF programs are loaded:
err := dropCapabilities([]uintptr{
    unix.CAP_NET_ADMIN,        // needed for fanotify on network sockets
    unix.CAP_DAC_READ_SEARCH,  // needed for /proc/<pid>/fd/ symlink resolution
})
```

**Capability constants:** [VERIFIED: pkg.go.dev/golang.org/x/sys@v0.30.0/unix]
```go
unix.CAP_BPF              // uintptr = 39; load eBPF programs
unix.CAP_SYS_ADMIN        // uintptr = 21; general admin (also grants CAP_BPF on old kernels)
unix.CAP_NET_ADMIN        // uintptr = 12; network config
unix.CAP_DAC_READ_SEARCH  // uintptr = 2; bypass read permission checks
unix.CAP_SYS_RESOURCE     // uintptr = 24; needed for rlimit.RemoveMemlock on < 5.11
```

**CAP_BPF** was added in kernel 5.8. On kernels < 5.8, `CAP_SYS_ADMIN` is required to load eBPF. The daemon must check the kernel version or catch `EPERM` and retry with `CAP_SYS_ADMIN`. [ASSUMED — based on Linux kernel history; verify on 5.4 CI runner]

---

## 6. seccomp-bpf (Pure Go)

### 6.1 Recommended Library

**Use `github.com/elastic/go-seccomp-bpf`** — pure Go, no CGO, no libseccomp. [VERIFIED: pkg.go.dev/github.com/elastic/go-seccomp-bpf]

The CONTEXT.md decision says "raw `unix.Prctl` path" to avoid adding a dependency. However, elastic/go-seccomp-bpf is pure Go and well-maintained; the raw approach is error-prone. The planner should confirm which path to use. This research documents both.

### 6.2 elastic/go-seccomp-bpf (Recommended)

```go
import seccomp "github.com/elastic/go-seccomp-bpf"

filter := seccomp.Filter{
    NoNewPrivs: true,  // Sets PR_SET_NO_NEW_PRIVS=1 before loading
    Flag:       seccomp.FilterFlagTSync,  // Sync to all goroutine threads
    Policy: seccomp.Policy{
        DefaultAction: seccomp.ActionKillProcess,  // Kill on unrecognized syscall
        Syscalls: []seccomp.SyscallGroup{
            {
                Action: seccomp.ActionAllow,
                Names: []string{
                    "read", "write", "close",
                    "epoll_wait", "epoll_ctl",
                    "rt_sigreturn", "exit_group",
                    "socket", "accept", "accept4",
                    "recvfrom", "sendto", "sendmsg",
                    "openat", "fstat", "stat",
                    "getpid", "getsockopt",
                    "gettimeofday", "clock_gettime",
                    "mmap", "munmap", "brk",
                    "futex",  // required for Go runtime
                    "sched_yield",
                    "sigaltstack",
                    "getrandom",
                },
            },
        },
    },
}

if err := seccomp.LoadFilter(filter); err != nil {
    return fmt.Errorf("loading seccomp filter: %w", err)
}
```

**Key behavior:**
- `NoNewPrivs: true` calls `prctl(PR_SET_NO_NEW_PRIVS, 1)` before loading
- `FilterFlagTSync` uses `SECCOMP_FILTER_FLAG_TSYNC` to sync across all Go runtime threads
- Supports amd64, arm64, arm, 386 [VERIFIED]
- Does NOT require `CAP_SYS_ADMIN` when `NoNewPrivs=true` is set first [VERIFIED: seccomp man page]

**Warning about Go runtime syscalls:** The Go runtime uses a large number of syscalls (futex, clone, mmap, etc.). A too-restrictive allowlist will crash the process. Start with ActionAllow as default and block known-dangerous calls (ActionKillProcess on specific syscalls), or use a carefully tested allowlist. The Go runtime syscall list varies by version. [ASSUMED — based on general Go + seccomp knowledge; requires empirical testing on target kernel]

### 6.3 Raw Approach (no external library, per CONTEXT.md decision)

If the decision stands to avoid the external library, use this pattern:
```go
import (
    "syscall"
    "unsafe"
    "golang.org/x/sys/unix"
)

// Must call PR_SET_NO_NEW_PRIVS first (or have CAP_SYS_ADMIN)
func setNoNewPrivs() error {
    _, _, errno := syscall.Syscall6(syscall.SYS_PRCTL,
        unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0, 0)
    if errno != 0 {
        return errno
    }
    return nil
}

// Construct BPF filter as []unix.SockFilter
// Then wrap in unix.SockFprog and call prctl(PR_SET_SECCOMP, SECCOMP_MODE_FILTER, prog)
func applySeccompFilter(filter []unix.SockFilter) error {
    prog := &unix.SockFprog{
        Len:    uint16(len(filter)),
        Filter: &filter[0],
    }
    _, _, errno := syscall.Syscall(syscall.SYS_PRCTL,
        unix.PR_SET_SECCOMP,
        unix.SECCOMP_MODE_FILTER,
        uintptr(unsafe.Pointer(prog)))
    if errno != 0 {
        return errno
    }
    return nil
}
```

**Constructing SockFilter instructions manually** is complex and error-prone. The instruction format uses `unix.SockFilter{Code, Jt, Jf, K}` with `BPF_*` opcode constants. See kernel docs for the BPF filter format. This path is significantly more complex than using elastic/go-seccomp-bpf. [CITED: kernel.org/doc/html/v4.19/userspace-api/seccomp_filter.html]

### 6.4 CAP_SYS_ADMIN Requirement

`prctl(PR_SET_SECCOMP, SECCOMP_MODE_FILTER, prog)` does NOT require `CAP_SYS_ADMIN` after `prctl(PR_SET_NO_NEW_PRIVS, 1)` has been called. The no-new-privs bit allows loading seccomp filters without elevated privileges. [VERIFIED: kernel.org/doc/Documentation/prctl/seccomp_filter.txt]

---

## 7. systemd Integration

### 7.1 Detecting systemd

```go
import "os"

// Method 1: Check for /run/systemd/system directory (most reliable)
func isSystemdRunning() bool {
    fi, err := os.Stat("/run/systemd/system")
    return err == nil && fi.IsDir()
}

// Method 2: Check /proc/1/comm
func pid1IsSystemd() bool {
    comm, err := os.ReadFile("/proc/1/comm")
    if err != nil {
        return false
    }
    return strings.TrimSpace(string(comm)) == "systemd"
}
```

Use `isSystemdRunning()` (checks `/run/systemd/system`) as the primary method — it avoids reading arbitrary process comm names and is used by coreos/go-systemd. [VERIFIED: pkg.go.dev/github.com/coreos/go-systemd/util]

### 7.2 Unit File Template (from CONTEXT.md)

```ini
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

**`Type=notify` requires the daemon to send `READY=1` via sd_notify.** If the daemon never sends READY=1, systemd will wait up to the configured timeout and report failure. Use `Type=simple` for v1 if sd_notify is not implemented, or implement it (recommended). [VERIFIED: freedesktop.org/software/systemd/man/latest/sd_notify.html]

### 7.3 sd_notify Implementation

[VERIFIED: pkg.go.dev/github.com/coreos/go-systemd/v22/daemon]

```go
import "github.com/coreos/go-systemd/v22/daemon"

// After daemon is fully initialized (socket bound, eBPF loaded):
sent, err := daemon.SdNotify(false, daemon.SdNotifyReady) // sends "READY=1"
if err != nil {
    log.Printf("sd_notify failed: %v", err)
    // non-fatal: daemon continues running; systemd may time out
}
// sent == false means NOTIFY_SOCKET not set (not running under systemd)
// sent == true means notification was sent successfully
```

**`daemon.SdNotifyReady`** is the string constant `"READY=1"`. [VERIFIED]

**Alternative without adding go-systemd dependency:** Write directly to `NOTIFY_SOCKET`:
```go
func sdNotifyReady() error {
    socketPath := os.Getenv("NOTIFY_SOCKET")
    if socketPath == "" {
        return nil // not running under systemd
    }
    conn, err := net.Dial("unixgram", socketPath)
    if err != nil {
        return err
    }
    defer conn.Close()
    _, err = conn.Write([]byte("READY=1"))
    return err
}
```

This avoids adding the coreos/go-systemd dependency. The `NOTIFY_SOCKET` env var is set by systemd when `Type=notify` is configured. [VERIFIED: freedesktop.org/software/systemd/man/latest/sd_notify.html]

### 7.4 systemctl Commands via exec.Command

```go
import "os/exec"

func systemctlDaemonReload(ctx context.Context) error {
    cmd := exec.CommandContext(ctx, "systemctl", "daemon-reload")
    out, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("daemon-reload failed: %w\noutput: %s", err, out)
    }
    return nil
}

func systemctlEnableNow(ctx context.Context, unit string) error {
    cmd := exec.CommandContext(ctx, "systemctl", "enable", "--now", unit)
    out, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("enable --now %s failed: %w\noutput: %s", unit, out, out)
    }
    return nil
}

func systemctlDisableNow(ctx context.Context, unit string) error {
    cmd := exec.CommandContext(ctx, "systemctl", "disable", "--now", unit)
    out, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("disable --now %s failed: %w\noutput: %s", unit, out, out)
    }
    return nil
}

func systemctlIsActive(ctx context.Context, unit string) (bool, error) {
    cmd := exec.CommandContext(ctx, "systemctl", "is-active", unit)
    err := cmd.Run()
    if err == nil {
        return true, nil // exit code 0 = active
    }
    var exitErr *exec.ExitError
    if errors.As(err, &exitErr) {
        return false, nil // exit code != 0 = not active; not an error per se
    }
    return false, err // real error (command not found, etc.)
}
```

**Note:** `systemctl` must be run as root (or with `sudo`). The install command already requires the user to run `beekeeper protect install` as root.

---

## 8. CI Matrix — Kernel Versions

### 8.1 GitHub Actions Runner Kernels

[VERIFIED: github.com/actions/runner-images Ubuntu2204-Readme.md]

| Runner Label | Actual Kernel (May 2026) | eBPF Tier |
|-------------|--------------------------|-----------|
| `ubuntu-22.04` | 6.8.0-1052-azure | Tier 0 (full) |
| `ubuntu-24.04` | 6.8.0+ | Tier 0 (full) |
| `ubuntu-latest` | 22.04 image (= 6.8.0) | Tier 0 (full) |
| `ubuntu-20.04` | **DEPRECATED** — unavailable from 2025-04-15 | N/A |

**Ubuntu 20.04 runners are no longer available on GitHub Actions.** The CONTEXT.md references ubuntu-20.04 for kernel 5.4 testing — this approach is obsolete. [VERIFIED: github.blog/changelog/2025-01-15-github-actions-ubuntu-20-runner-image-brownout-dates-and-other-breaking-changes]

### 8.2 LVH (Little VM Helper) for Multi-Kernel Testing

[VERIFIED: github.com/cilium/little-vm-helper-images, ebpfchirp.substack.com]

LVH is the standard approach for eBPF multi-kernel CI, used by Cilium, Tetragon, and pwru. It runs a QEMU VM with a specific kernel inside the GitHub Actions runner.

**Available kernel versions in LVH:** [VERIFIED: github.com/cilium/little-vm-helper-images/blob/main/_data/kernels.json]
- `5.4` — linux-5.4.y stable
- `5.10` — linux-5.10.y stable
- `5.15` — linux-5.15.y stable
- `6.1`, `6.6`, `6.12`, `6.18` — stable
- `bpf-next` — bleeding edge

**GitHub Actions workflow using LVH:**
```yaml
jobs:
  test-sentry-kernel-5-4:
    runs-on: ubuntu-22.04
    steps:
      - uses: actions/checkout@v4
      - uses: cilium/little-vm-helper@v0.0.21
        with:
          image-version: "5.4-main"
          test-name: "sentry-degradation-tier-2"
          cmd: |
            go test -v -tags linux ./internal/sentry/... -run TestDegradationTier

  test-sentry-kernel-5-15:
    runs-on: ubuntu-22.04
    steps:
      - uses: actions/checkout@v4
      - uses: cilium/little-vm-helper@v0.0.21
        with:
          image-version: "5.15-main"
          test-name: "sentry-full-tier"
          cmd: |
            go test -v -tags linux ./internal/sentry/... -run TestFullTier
```

**LVH image format:** `<kernel_version>-main` (e.g., `5.4-main`, `5.15-main`). [VERIFIED: cilium/little-vm-helper action.yaml]

### 8.3 CO-RE Portability

eBPF programs compiled with CO-RE (which bpf2go enables by default via BTF relocation) are portable across kernel versions **as long as the required kernel features are available** (ring buffer, tracepoints, etc.). A single `.bpf.o` compiled on Ubuntu 22.04 (kernel 6.8) can be loaded on kernel 5.4. The CO-RE relocation engine (built into cilium/ebpf) adjusts struct field offsets at load time using the target kernel's BTF. [VERIFIED: ebpf-go.dev concepts docs, aquasec.com CO-RE article]

**Requirement:** The target kernel must have BTF enabled (`CONFIG_DEBUG_INFO_BTF=y`) OR have `/sys/kernel/btf/vmlinux` available. Ubuntu 20.04+ kernels have BTF. RHEL 8+ kernels have BTF. Very old/minimal kernels may not. [VERIFIED: cilium/ebpf btf.LoadKernelSpec docs]

**Fallback when BTF absent:** `btf.LoadKernelSpec()` falls back to scanning the filesystem for vmlinux ELFs. If no BTF is available at all, CO-RE relocations that reference kernel structs will fail at load time — the daemon must catch this and degrade to Tier 2 (minimal). [VERIFIED: pkg.go.dev/github.com/cilium/ebpf@v0.21.0/btf]

---

## 9. Key Pitfalls

### Pitfall 1: Single CapUserData with LINUX_CAPABILITY_VERSION_3
**What goes wrong:** Passing `&data` (a single `unix.CapUserData`) to `Capget`/`Capset` with `Version: unix.LINUX_CAPABILITY_VERSION_3` writes beyond the struct bounds — heap corruption.
**Why it happens:** V3 capabilities are 64-bit; the kernel writes into two consecutive `CapUserData` structs.
**How to avoid:** Always `var data [2]unix.CapUserData` and pass `&data[0]`. [VERIFIED: golang/go#44312]
**Warning signs:** Capabilities appear to be set but behavior is wrong; or process crashes mysteriously after capset.

### Pitfall 2: Fanotify fd Not Opened with O_RDWR for Permission Events
**What goes wrong:** `FAN_OPEN_PERM` events require writing a `FanotifyResponse` to the fanotify fd. If the fd was opened with `O_RDONLY`, the write fails.
**Why it happens:** The standard `event_f_flags` is `O_RDONLY`; permission events need write access to the same fd.
**How to avoid:** When using `FAN_OPEN_PERM`, pass `unix.O_RDWR` or `unix.O_WRONLY` as `event_f_flags` in `FanotifyInit`. Alternatively, use a separate write fd by keeping the fanotify fd in `O_RDWR` mode.
**Warning signs:** `write: bad file descriptor` when responding to permission events; target processes block indefinitely.

### Pitfall 3: Failure to Close Event Fds from Fanotify
**What goes wrong:** Each fanotify event with `meta.Fd >= 0` opens a new fd in the receiving process. If not closed promptly, fd exhaustion occurs.
**Why it happens:** fanotify grants the receiver a live fd to the accessed file. This is intentional for content inspection but must be closed after use.
**How to avoid:** Call `unix.Close(int(meta.Fd))` immediately after path resolution in the event loop.
**Warning signs:** "too many open files" error after the daemon runs for several minutes.

### Pitfall 4: FAN_OPEN_PERM Without Timely Response Blocks the Accessing Process
**What goes wrong:** A process trying to open a watched file is blocked until the daemon sends `FAN_ALLOW` or `FAN_DENY`. If the daemon is slow or crashes, the accessing process hangs indefinitely.
**Why it happens:** `FAN_OPEN_PERM` is a synchronous permission check — it blocks the kernel's file open syscall.
**How to avoid:** (1) Always respond immediately with `FAN_ALLOW` in v1 (detection-only). (2) Use a separate, high-priority goroutine for permission responses. (3) In the event loop, respond before emitting to the correlation engine channel. (4) If the daemon crashes, systemd restarts it but any pending permission events will unblock (kernel times them out). [ASSUMED — based on fanotify man page semantics; verify timeout behavior]

### Pitfall 5: Ring Buffer `max_entries` Must Be a Power of 2 and Page-Aligned
**What goes wrong:** `loadBeekeeperExecObjects` returns `EINVAL` or `EPERM`.
**Why it happens:** `BPF_MAP_TYPE_RINGBUF` requires `max_entries` to be a power of 2 AND a multiple of the page size (4096).
**How to avoid:** Use `1 << 24` (16MB) or any power-of-2 that's also divisible by 4096. `1 << 12` = 4096 (minimum); `1 << 24` = 16MB (generous).
**Warning signs:** `LoadAndAssign` fails with `EINVAL` even though the C source looks correct.

### Pitfall 6: `bpf_get_current_pid_tgid() >> 32` Returns TGID, Not PID
**What goes wrong:** Process tree shows duplicate or incorrect PIDs; multi-threaded programs show thread IDs instead of process IDs.
**Why it happens:** The function returns `(tgid << 32 | tid)`; `>> 32` shifts out the TID. TGID = process ID as seen by userspace.
**How to avoid:** This is correct for tracking processes. The lower 32 bits (TID) are only needed for per-thread tracking. Store TGID as `pid` everywhere.

### Pitfall 7: inet_sock_set_state Fires Outside Process Context
**What goes wrong:** `bpf_get_current_pid_tgid()` returns 0 or a wrong PID in the `inet_sock_set_state` tracepoint handler.
**Why it happens:** TCP state machine transitions (e.g., ESTABLISHED → CLOSE_WAIT) happen in softirq or timer context, not in the process's kernel context.
**How to avoid:** Use `kprobe/tcp_connect` for capturing the initiating connection — it always fires in process context. Reserve `inet_sock_set_state` for state tracking only (not for PID attribution).

### Pitfall 8: Go Runtime Syscalls Not in seccomp Allowlist
**What goes wrong:** Daemon crashes with SIGSYS (signal 31) shortly after applying seccomp filter.
**Why it happens:** Go runtime uses syscalls like `futex`, `clone`, `mmap`, `epoll_create1`, `sigaltstack`, `getrandom` that are not in a hand-written minimal allowlist.
**How to avoid:** Use `elastic/go-seccomp-bpf` with default action `ActionAllow` + blocklist of specific dangerous calls, OR carefully audit the Go runtime syscalls for the target architecture before building a custom allowlist.
**Warning signs:** Process exits with signal 31 (SIGSYS); `strace` shows seccomp-killed call.

### Pitfall 9: bpf2go go:generate Run on Non-Linux Host
**What goes wrong:** `go generate ./internal/sentry/linux/` fails on Windows with "clang: command not found" or "linux headers not found".
**Why it happens:** bpf2go requires clang and Linux kernel headers (for BTF).
**How to avoid:** Run `go generate` only in Linux CI. Commit generated files (`bpf_bpfel.go`, `bpf_bpfeb.go`, `*.bpf.o` embedded via go:embed) to the repo. The `//go:build linux` tag on all generated files ensures they compile on Windows without executing the C code path.

### Pitfall 10: ubuntu-20.04 Runner Unavailable (CI Matrix Obsolete)
**What goes wrong:** CI job fails immediately with "The ubuntu-20.04 runner image has been retired."
**Why it happens:** GitHub deprecated ubuntu-20.04 runners effective April 15, 2025.
**How to avoid:** Use LVH (`cilium/little-vm-helper@v0.0.21`) with `image-version: "5.4-main"` running inside an `ubuntu-22.04` host runner. [VERIFIED: github.blog changelog]

### Pitfall 11: CAP_BPF Not Available on Kernel < 5.8
**What goes wrong:** Capability drop fails; bitmask manipulation uses wrong bit index.
**Why it happens:** `CAP_BPF` (capability number 39) was added in kernel 5.8. On 5.4, loading eBPF requires `CAP_SYS_ADMIN`.
**How to avoid:** Check `features.HaveProgramType(ebpf.Kprobe)` to infer kernel capability model, OR catch `EPERM` and retry with `CAP_SYS_ADMIN` held. Drop `CAP_BPF` from the keep-set on < 5.8.

---

## 10. go.mod Changes

Current go.mod has `golang.org/x/sys v0.30.0` (sufficient for fanotify, SO_PEERCRED, Capset). The following need to be added:

```
require (
    github.com/cilium/ebpf v0.21.0
    github.com/elastic/go-seccomp-bpf v1.0.2
    github.com/coreos/go-systemd/v22 v22.5.0
)
```

**Tool dependency (bpf2go — dev time only, invoked via `go tool`):**
```
tool (
    github.com/cilium/ebpf/cmd/bpf2go
)
```

**go get commands:**
```bash
# Core eBPF library
go get github.com/cilium/ebpf@v0.21.0

# bpf2go as a tool dependency (Go 1.24+ tool directive)
go get -tool github.com/cilium/ebpf/cmd/bpf2go@v0.21.0

# seccomp-bpf (pure Go, no CGO)
go get github.com/elastic/go-seccomp-bpf@v1.0.2

# systemd sd_notify (for Type=notify READY=1)
go get github.com/coreos/go-systemd/v22@v22.5.0
```

**If CONTEXT.md raw-syscall decision is kept for seccomp:** omit `github.com/elastic/go-seccomp-bpf`.
**If sd_notify is hand-rolled:** omit `github.com/coreos/go-systemd/v22` (it's only needed for `daemon.SdNotify`; see section 7.3 for the 10-line alternative).

**golang.org/x/sys already at v0.30.0** — verify this includes all needed fanotify constants (`FAN_REPORT_FID`, `FAN_REPORT_DFID_NAME`, `FAN_REPORT_PIDFD`). These are available in v0.30.0 (added in earlier minor versions). [ASSUMED — versions not individually verified; run `grep FAN_REPORT .` in the vendor directory to confirm]

**Version verification:**
```bash
npm view github.com/cilium/ebpf  # not applicable — use go module proxy
go list -m -versions github.com/cilium/ebpf@latest  # verify v0.21.0 is available
```
[VERIFIED via github.com/cilium/ebpf/releases: v0.21.0 released 2026-03-05]

---

## Code Examples (Additional Reference)

### Complete daemon startup sequence

```go
// internal/sentry/linux/daemon.go
//go:build linux

package linux

import (
    "context"
    "github.com/cilium/ebpf/rlimit"
    "github.com/cilium/ebpf/features"
    "github.com/cilium/ebpf"
)

func RunDaemon(ctx context.Context, cfg *config.Config) error {
    // 1. Remove memlock limit (no-op on 5.11+)
    if err := rlimit.RemoveMemlock(); err != nil {
        return fmt.Errorf("memlock: %w", err)
    }

    // 2. Probe capabilities
    tier := probeTier()  // returns Tier0, Tier1, or Tier2

    // 3. Load eBPF objects (only on Tier 0/1)
    var execObjs BeekeeperExecObjects
    var netObjs  BeekeeperNetObjects
    if tier <= Tier1 {
        if err := loadBeekeeperExecObjects(&execObjs, nil); err != nil {
            return fmt.Errorf("loading exec eBPF: %w", err)
        }
        defer execObjs.Close()
        // ... attach tracepoints / kprobes ...
    }

    // 4. Drop capabilities after loading
    if err := dropCapabilities(keepCaps(tier)); err != nil {
        return fmt.Errorf("capability drop: %w", err)
    }

    // 5. Apply seccomp filter
    if err := applySeccomp(); err != nil {
        return fmt.Errorf("seccomp: %w", err)
    }

    // 6. Initialize fanotify
    fanFd, err := initFanotify(tier, cfg.Policy.SensitivePaths)
    if err != nil {
        return fmt.Errorf("fanotify: %w", err)
    }
    defer unix.Close(fanFd)

    // 7. Start IPC server
    ipcSrv, err := ipc.NewServer(cfg.SockPath, installingUID)
    if err != nil {
        return fmt.Errorf("ipc server: %w", err)
    }
    defer ipcSrv.Close()

    // 8. Notify systemd READY
    sdNotifyReady()

    // 9. Start goroutines
    events := make(chan sentry.SentryEvent, 10000)
    go ebpfReaderLoop(ctx, execObjs.Events, netObjs.Events, events, tier)
    go fanotifyReaderLoop(ctx, fanFd, events)
    go correlationEngineLoop(ctx, events, cfg, auditWriter)

    <-ctx.Done()
    return nil
}
```

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go test (stdlib) |
| Config file | none (inferred from go.mod) |
| Quick run command | `go test -tags linux ./internal/sentry/...` |
| Full suite command | `go test -race -tags linux ./internal/sentry/... ./internal/ipc/...` |

### Phase Requirements to Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SLNX-01 | systemd unit file written correctly | unit | `go test ./internal/sentry/linux/ -run TestUnitFile` | Wave 0 |
| SLNX-02 | eBPF exec events ingested with pid/ppid/exe | integration (LVH) | `go test -tags linux ./internal/sentry/linux/ -run TestExecTracer` | Wave 0 |
| SLNX-03 | fanotify fires on sensitive path read | integration (LVH) | `go test -tags linux ./internal/sentry/linux/ -run TestFanotify` | Wave 0 |
| SLNX-04 | eBPF network events ingested with dst addr | integration (LVH) | `go test -tags linux ./internal/sentry/linux/ -run TestNetTracer` | Wave 0 |
| SLNX-05 | Degradation tier selected correctly | unit | `go test -tags linux ./internal/sentry/linux/ -run TestProbeTier` | Wave 0 |
| SLNX-06 | Ring buffer channel drop counter increments | unit | `go test ./internal/sentry/ -run TestChannelDrop` | Wave 0 |
| SLNX-07 | SO_PEERCRED rejects wrong UID | unit | `go test -tags linux ./internal/ipc/ -run TestPeerCredRejection` | Wave 0 |
| SLNX-08 | All 5 correlation rules fire on crafted events | unit | `go test ./internal/sentry/ -run TestCorrelationRules` | Wave 0 |
| SLNX-09 | baseline_mode=true during first 7 days | unit | `go test ./internal/sentry/ -run TestBaselineMode` | Wave 0 |
| SLNX-10 | Capability drop leaves only expected caps | integration (LVH) | `go test -tags linux ./internal/sentry/linux/ -run TestCapabilityDrop` | Wave 0 |

### IPC Fuzz Gate (Release Blocker)

```
internal/ipc/proto_fuzz_test.go — FuzzIPCMessage
```
Must pass `go test -fuzz=FuzzIPCMessage -fuzztime=60s ./internal/ipc/` before v0.6.0 tag.

### Wave 0 Gaps (Tests to Create)

- [ ] `internal/sentry/rules_test.go` — covers SLNX-08 (5 correlation rules)
- [ ] `internal/sentry/baseline_test.go` — covers SLNX-09
- [ ] `internal/sentry/linux/probe_test.go` — covers SLNX-05 (mocked feature probes)
- [ ] `internal/sentry/linux/systemd_test.go` — covers SLNX-01 (unit file generation)
- [ ] `internal/ipc/proto_test.go` + `proto_fuzz_test.go` — covers SLNX-07
- [ ] `internal/sentry/types_test.go` — covers SentryEvent struct round-trip

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | SO_PEERCRED UID verification on Unix socket |
| V3 Session Management | yes | Length-prefixed JSON framing, 64KB bound per message |
| V4 Access Control | yes | Capability drop after eBPF load; seccomp-bpf allowlist |
| V5 Input Validation | yes | IPC message parser fuzz test; bounded event channel |
| V6 Cryptography | no | No crypto in Sentry daemon itself |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Unauthorized CLI control of Sentry | Elevation of Privilege | SO_PEERCRED UID check on every accepted connection |
| Daemon resource exhaustion via event flood | Denial of Service | Buffered channel cap 10,000; drop counter; event fd closed promptly |
| IPC message injection (malformed length prefix) | Tampering | 4-byte big-endian length prefix; 64KB bound; fuzz test required |
| Capability re-escalation after drop | Elevation of Privilege | `NoNewPrivileges=false` in unit but capability bounding set limits; seccomp blocks `capset(2)` after filter applied |
| Agent exploiting fanotify permission timeout | Tampering | Always respond FAN_ALLOW immediately; timeout handled by kernel |
| Sentry self-compromise via eBPF verifier bypass | Tampering | Pre-compiled bytecode, no runtime compilation; eBPF verifier run at load time |

---

## Environment Availability

| Dependency | Required By | Available on dev (Win) | Available on CI | Fallback |
|------------|------------|------------------------|----------------|---------|
| clang | bpf2go code generation | No | ubuntu-22.04 via apt | CI-only; generated files committed |
| libbpf-dev | bpf2go C compilation | No | ubuntu-22.04 via apt | CI-only |
| Linux kernel >= 5.4 | eBPF loading | No | LVH 5.4-main | LVH for kernel-specific tests |
| Linux kernel >= 5.15 | Ring buffer, FAN_REPORT_FID | No | ubuntu-22.04 (6.8) | LVH 5.15-main |
| systemd | protect install/uninstall | No | ubuntu-22.04 | Detection via /run/systemd/system |

All Linux-specific code is `//go:build linux` — the module compiles on Windows without any of these dependencies.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `-target bpf` generates a single generic object file (not endian-specific) | §1.1 | May generate bpfel+bpfeb anyway; no functional impact, just extra files |
| A2 | `kprobe/tcp_connect` always fires in process context and gives correct PID | §2.2 | If wrong, network events have wrong PID attribution; fall back to `task_struct` walk |
| A3 | Go runtime syscall set on amd64 is stable enough for a static seccomp allowlist | §6 | If wrong, daemon SIGSYSes after seccomp applied; mitigated by using ActionAllow+blocklist instead of allowlist+kill |
| A4 | `CAP_BPF` (cap 39) present in `unix` package constants for all relevant kernel targets | §5 | If constant is wrong, capability manipulation silently operates on wrong bit; verify with `unix.CAP_BPF` value at runtime |
| A5 | `golang.org/x/sys v0.30.0` includes FAN_REPORT_FID and FAN_REPORT_PIDFD constants | §10 | If constants missing, fanotify code won't compile; upgrade golang.org/x/sys |
| A6 | LVH image-version `"5.4-main"` and `"5.15-main"` are valid and available from quay.io/lvh-images/ | §8.2 | If image names changed, CI matrix fails; check current tags in the LVH registry |
| A7 | FAN_OPEN_PERM permission events time out in the kernel if the daemon crashes | §9 Pitfall 4 | If they don't time out, target processes hang permanently until daemon restart |

---

## Open Questions

1. **seccomp: external library vs raw syscall**
   - What we know: CONTEXT.md says "raw unix.Prctl path" but elastic/go-seccomp-bpf is simpler and more correct for Go runtime compatibility.
   - What's unclear: Whether the project prefers zero external deps for this or is willing to add elastic/go-seccomp-bpf.
   - Recommendation: Use elastic/go-seccomp-bpf unless dependency-zero is a hard requirement.

2. **sd_notify: coreos/go-systemd vs hand-rolled**
   - What we know: The 10-line hand-rolled `NOTIFY_SOCKET` approach works identically; coreos/go-systemd adds watchdog support.
   - What's unclear: Whether watchdog pings are desired in v1.
   - Recommendation: Hand-roll for v1 (avoids dependency); add coreos/go-systemd in a later phase when watchdog is needed.

3. **vmlinux.h generation**
   - What we know: bpf2go generates or requires vmlinux.h for CO-RE; it can be committed to the repo.
   - What's unclear: Whether to use the cilium/ebpf bundled headers or generate fresh from the ubuntu-22.04 CI kernel.
   - Recommendation: Use cilium/ebpf bundled headers for v1; the committed version in `internal/sentry/linux/bpf/headers/` is sufficient for the struct definitions needed.

4. **Process tree snapshot consistency**
   - What we know: The correlation engine receives a snapshot copy of the process tree from the daemon goroutine.
   - What's unclear: Whether a simple `maps.Clone()` is sufficient or if a read-write mutex is needed.
   - Recommendation: Single goroutine owns the map; pass snapshot copy via channel message to the correlation engine. No mutex needed with this design.

---

## Sources

### Primary (HIGH confidence)
- `pkg.go.dev/github.com/cilium/ebpf@v0.21.0` — LoadAndAssign, ringbuf, perf, features, rlimit, link, btf packages
- `pkg.go.dev/github.com/cilium/ebpf@v0.21.0/features` — HaveMapType, HaveProgramType, LinuxVersionCode
- `pkg.go.dev/github.com/cilium/ebpf@v0.21.0/ringbuf` — Reader, NewReader, ReadInto, Record
- `pkg.go.dev/github.com/cilium/ebpf@v0.21.0/perf` — perf.Reader, NewReader
- `pkg.go.dev/github.com/cilium/ebpf@v0.21.0/link` — Tracepoint, Kprobe, Link interface
- `pkg.go.dev/github.com/cilium/ebpf@v0.21.0/btf` — LoadKernelSpec fallback behavior
- `pkg.go.dev/github.com/cilium/ebpf@v0.21.0/rlimit` — RemoveMemlock
- `github.com/cilium/ebpf/blob/v0.21.0/examples/ringbuffer/ringbuffer.c` — ring buffer C pattern
- `github.com/cilium/ebpf/blob/v0.21.0/examples/ringbuffer/main.go` — complete Go ring buffer consumer
- `github.com/cilium/ebpf/releases/tag/v0.21.0` — release date, breaking changes
- `pkg.go.dev/github.com/cilium/ebpf/cmd/bpf2go` — bpf2go flags and usage
- `man7.org/linux/man-pages/man2/fanotify_init.2.html` — FAN_* flags with kernel version annotations
- `man7.org/linux/man-pages/man7/fanotify.7.html` — event structures, FanotifyResponse
- `pkg.go.dev/golang.org/x/sys@v0.30.0/unix` — FanotifyInit, FanotifyMark, GetsockoptUcred, Capget, Capset
- `pkg.go.dev/github.com/elastic/go-seccomp-bpf` — Filter, Policy, LoadFilter
- `github.com/cilium/little-vm-helper-images/blob/main/_data/kernels.json` — LVH kernel versions
- `github.com/cilium/little-vm-helper/blob/main/action.yaml` — LVH GitHub Action inputs

### Secondary (MEDIUM confidence)
- `github.com/golang/go/issues/44312` — CapUserData two-struct bug (official Go issue tracker)
- `github.com/mozillazg/hello-libbpfgo/blob/master/37-tracepoint-sched_process_exec/main.bpf.c` — exec tracepoint C source
- `github.com/cilium/ebpf/releases` — confirmed v0.21.0 released 2026-03-05
- `pkg.go.dev/github.com/coreos/go-systemd/v22/daemon` — SdNotify signature, constants
- `freedesktop.org/software/systemd/man/latest/sd_notify.html` — READY=1 protocol, NOTIFY_SOCKET
- `github.blog/changelog/2025-01-15-github-actions-ubuntu-20-runner-image-brownout-dates-and-other-breaking-changes` — ubuntu-20.04 deprecation
- `github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2204-Readme.md` — ubuntu-22.04 kernel 6.8.0

### Tertiary (LOW confidence — marked [ASSUMED])
- TCP connect kprobe always fires in process context (verified via Brendan Gregg TCP tracepoints article; LOW because no unit test confirms behavior on 5.4)
- Go runtime syscall set is stable for seccomp allowlist (LOW — highly environment-specific)
- CAP_BPF not available on kernel < 5.8 (LOW — based on kernel commit history, not direct test)

---

## Metadata

**Confidence breakdown:**
- cilium/ebpf API (bpf2go, ringbuf, features): HIGH — verified against pkg.go.dev and source
- eBPF C patterns (exec tracepoint, tcp kprobe): MEDIUM — verified against examples; runtime behavior on 5.4 unconfirmed until CI
- fanotify API: HIGH — verified against man pages and golang.org/x/sys docs
- SO_PEERCRED: HIGH — exact signatures from pkg.go.dev
- Capability drop: HIGH — verified against golang/go#44312 and pkg.go.dev
- seccomp-bpf: MEDIUM — elastic/go-seccomp-bpf verified; Go runtime syscall list is ASSUMED
- systemd integration: HIGH — sd_notify protocol verified against freedesktop.org
- CI matrix: HIGH — ubuntu-20.04 deprecation confirmed; LVH kernel availability verified

**Research date:** 2026-05-27
**Valid until:** 2026-08-27 (90 days; stable APIs; LVH image version string may change sooner)
