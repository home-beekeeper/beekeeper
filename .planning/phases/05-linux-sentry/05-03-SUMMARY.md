---
plan: 05-03
status: complete
wave: 2
---
# 05-03 Summary: eBPF Event Ingestion

## Artifacts
- go.mod: cilium/ebpf v0.21.0, elastic/go-seccomp-bpf v1.6.0, bpf2go tool directive
- internal/sentry/linux/bpf/vmlinux.h — stub BTF header
- internal/sentry/linux/bpf/exec_tracer.bpf.c — sched_process_exec tracepoint → ring buffer
- internal/sentry/linux/bpf/network_tracer.bpf.c — kprobe/tcp_connect → ring buffer
- internal/sentry/linux/gen.go — //go:generate lines for bpf2go (linux only)
- internal/sentry/linux/bpf_beekeeper_exec_bpfel.go — stub generated bindings (linux)
- internal/sentry/linux/bpf_beekeeper_net_bpfel.go — stub generated bindings (linux)
- internal/sentry/linux/probe.go — ProbeTier/TierString; two-struct Capget pattern
- internal/sentry/linux/ebpf.go — StartEBPFReaders, StartProcessTreeBuilder, RemoveMemlock
- internal/sentry/linux/ebpf_test.go, probe_test.go — binary parse tests (linux)

## Verification
- go build ./... passes on Windows (linux files excluded by build tag)
- GOOS=linux GOARCH=amd64 go build ./... passes (cross-compile)
- GOOS=linux GOARCH=amd64 go vet ./internal/sentry/linux/... passes
- cilium/ebpf v0.21.0 in go.mod confirmed
- elastic/go-seccomp-bpf v1.6.0 in go.mod confirmed (v1.0.2 tag does not exist; v1.6.0 is latest)
- kprobe/tcp_connect in C source confirmed
- BPF_MAP_TYPE_RINGBUF in exec tracer confirmed
- bpf2go tool directive present as: tool github.com/cilium/ebpf/cmd/bpf2go

## Notes
- elastic/go-seccomp-bpf v1.0.2 tag does not exist upstream; v1.6.0 (latest) used instead.
- StartProcessTreeBuilder uses drop-on-full semantics for the tree channel (send-only
  channel cannot be drained from the writer side); a capacity-1 buffered channel is
  the recommended caller pattern.
- golang.org/x/net v0.46.0 added as indirect dependency (required by elastic/go-seccomp-bpf).
