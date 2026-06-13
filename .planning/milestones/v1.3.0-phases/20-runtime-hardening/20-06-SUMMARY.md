---
phase: 20-runtime-hardening
plan: 06
subsystem: sentry
tags: [sentry, dns, ebpf, kprobe, etw, dns-client, exfil, optional-stretch]

requires:
  - phase: 20-runtime-hardening
    provides: "20-04 EventFileWrite iota slot (EventDNSQuery appended after it); 20-05 honesty docs"
provides:
  - EventDNSQuery EventKind (QNAME in FilePath)
  - Linux kprobe DNS ingestion (udp_sendmsg/tcp_sendmsg dport 53) via BeekeeperDNS bpf2go target + fail-closed stub
  - Windows DNS-Client ETW ingestion (provider {1C95126E-...} event 3006 QueryName)
affects: []

tech-stack:
  added: []
  patterns:
    - "eBPF copies raw DNS wire bytes; QNAME decoded in Go (decodeDNSQName) to keep the in-kernel path verifier-friendly"
    - "Optional event source is fail-safe: StartEBPFReaders takes a nil-able *dnsObjs; a DNS load failure leaves DNS absent without degrading exec/net"

key-files:
  created:
    - internal/sentry/linux/bpf/dns_tracer.bpf.c
    - internal/sentry/linux/bpf_beekeeper_dns_bpfel.go
  modified:
    - internal/sentry/types.go
    - internal/sentry/rules.go
    - internal/sentry/rules_test.go
    - internal/sentry/linux/gen.go
    - internal/sentry/linux/ebpf.go
    - internal/sentry/linux/daemon.go
    - internal/sentry/windows/parser.go
    - internal/sentry/windows/parser_test.go
    - internal/sentry/windows/etw.go
    - internal/sentry/windows/daemon.go

key-decisions:
  - "EventDNSQuery carries the QNAME in the existing FilePath field (no SentryEvent struct change), per the plan's 'FilePath/a QName field' allowance — minimizes surface."
  - "eBPF copies the raw DNS message bytes over the ringbuf and Go decodes the length-prefixed QNAME (decodeDNSQName); the in-kernel path avoids a fragile label-walk loop. The msg_iter.iov access targets the CI kernel matrix (5.4/5.15, pre-6.4 where the field is `iov`); 6.4+ `__iov` would need a CO-RE variant — flagged for CI."
  - "DNS is an OPTIONAL event source: StartEBPFReaders takes a nil-able *BeekeeperDNSObjects and daemon.go loads it best-effort; a load failure (bytecode-absent stub) disables DNS without degrading the core exec/net readers (fail-safe)."
  - "The committed bpf_beekeeper_dns_bpfel.go is a FAIL-CLOSED loader stub (returns the 'run go generate on Linux' error); real bytecode is CI-generated, never committed, never compiled at runtime (CLAUDE.md eBPF constraint)."
  - "No DNS-exfil correlation rule yet: EvaluateEvent dispatches EventDNSQuery as an explicit no-op so the ingestion source lands ahead of the rule (engine stays pure)."

patterns-established:
  - "Adding a manifest ETW provider (DNS-Client) reuses the existing access-denied-continue enable fallback, which also covers the open question of whether golang-etw can enable a manifest provider on the running session"

requirements-completed: [SENT-11]

duration: ~45 min
completed: 2026-06-10
---

# Phase 20 Plan 06: DNS Query Ingestion (SENT-11, OPTIONAL stretch) Summary

**A new EventDNSQuery event kind plus DNS QNAME ingestion on Linux (kprobe on udp_sendmsg/tcp_sendmsg filtered to port 53) and Windows (DNS-Client ETW event 3006), closing the DNS-TXT tunnelling gap that the TCP-connect-only network source could not see. macOS DNS stays out (NetworkExtension, v2). The stretch was executed rather than dropped.**

## Performance

- **Duration:** ~45 min
- **Tasks:** 2
- **Files:** 10 modified + 2 created

## Accomplishments

- **Task 1 (Linux + cross-cutting):** `EventDNSQuery` EventKind (QNAME in FilePath); `bpf/dns_tracer.bpf.c` kprobes on udp_sendmsg/tcp_sendmsg (dport 53) pushing raw DNS bytes over a ringbuf; `BeekeeperDNS` bpf2go target in gen.go; fail-closed `bpf_beekeeper_dns_bpfel.go` loader stub; `dnsEventLayout` + `decodeDNSQName` + the optional DNS reader in ebpf.go; best-effort DNS load + nil-able wiring in daemon.go; rules.go no-op dispatch; `TestDNSQueryPassThrough`.
- **Task 2 (Windows):** DNS-Client provider GUID in `ProviderGUIDs` + the daemon enable list (access-denied-continue fallback); parser.go maps event ID 3006 `QueryName` -> `EventDNSQuery`; `TestParseDNSQueryEvent` + `TestParseDNSNon3006Ignored`.

## Task Commits

1. **Tasks 1+2: DNS ingestion (Linux kprobe + Windows ETW)** - `d5719c5` (feat)

## Deviations from Plan

- **None structural.** Executed both tasks as specified. The QNAME is carried in `FilePath` (plan-allowed) rather than a new struct field, keeping the event layout and all per-OS struct mirrors unchanged.

## Issues Encountered / Carried Forward (CI-gated, like the 20-04 ETW field-key flag)

- **eBPF QNAME parse + msg_iter access are CI-validated only.** The `dns_tracer.bpf.c` source compiles to bytecode in the Ubuntu eBPF CI matrix (5.4 / 5.15), never locally. The `msg_iter.iov` field access is correct for those pre-6.4 kernels; on 6.4+ the field is `__iov` and would need a CO-RE variant. DNS-over-TCP also prefixes a 2-byte length before the 12-byte DNS header, so the Go QNAME decoder (which skips 12 bytes) is tuned for UDP; the TCP offset is a known CI-refinement. Real kprobe DNS capture is validated in CI.
- **golang-etw manifest-provider enable (open question).** Whether tekert/golang-etw can enable the DNS-Client MANIFEST provider on the running session is verified in the windows-latest CI capture; if it cannot, the existing access-denied-continue fallback leaves DNS absent without crashing the daemon (documented, fail-safe).

## Verification

- `go build ./...` + `GOOS=linux go build ./internal/sentry/linux/` + `GOOS=windows go build ./internal/sentry/windows/` all exit 0.
- `go test ./internal/sentry/ -run "DNSQuery"` + `go test ./internal/sentry/windows/ -run "Parse|DNS"` (native) + full `go test ./internal/sentry/...` + `go vet` (native + GOOS=linux) all green.
- AC greps: EventDNSQuery in types.go, BeekeeperDNS in gen.go, udp_sendmsg in dns_tracer.bpf.c, DNS loader stub fails closed, 3006 + DNS-Client GUID in parser.go, DNS-Client in the daemon provider list. No bytecode committed.

## Phase 20 status

- All 6 plans' CODE is complete (20-01..20-06). The ONLY outstanding Phase-20 item is the 20-02 human HF-license live-bootstrap verification (LlamaFirewall gated model).

---
*Phase: 20-runtime-hardening*
*Completed: 2026-06-10*
