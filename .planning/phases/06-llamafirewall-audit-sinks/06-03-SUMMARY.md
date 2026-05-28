---
phase: "06"
plan: "03"
subsystem: audit
tags: [audit, sinks, otlp, syslog, https, config, AUDT-03, AUDT-04]
dependency_graph:
  requires: [06-01, 06-02]
  provides: [audit.Sink, audit.MultiSink, audit.OTLPSink, audit.HTTPSink, audit.SyslogSink, config.AuditConfig, config.LlamaFirewallConfig]
  affects: [internal/audit, internal/config]
tech_stack:
  added: [log/syslog (stdlib), net/http (stdlib), sync.Mutex, OTLPv1 LogsData JSON]
  patterns: [Fan-out sink pattern, fire-and-forget remote sinks, batch-flush OTLP, RFC-5424 syslog framing]
key_files:
  created:
    - internal/audit/sink.go
    - internal/audit/sink_test.go
    - internal/audit/otlp.go
    - internal/audit/http_sink.go
    - internal/audit/syslog.go
    - internal/audit/syslog_stub.go
  modified:
    - internal/audit/writer.go
    - internal/config/config.go
decisions:
  - "AuditConfig lives in internal/config (not audit) — audit/sink.go imports it to avoid duplicating the struct; no import cycle exists because config imports only stdlib."
  - "ErrSyslogNotSupported is a var (not a sentinel value const) so errors.Is works cross-package on both build tags."
  - "OTLPSink reuses the otlpLogRecord/otlpStringVal/otlpKV types already defined in export.go by being in the same package — no duplication needed."
  - "Writer.Write fan-out holds the Writer mutex during sink.Write calls; remote sinks (OTLPSink, HTTPSink) manage their own internal locking so they are safe under the Writer mutex."
  - "Remote sink flush errors are fire-and-forget (logged to stderr, nil returned) so a remote collector outage never affects the local NDJSON audit trail (fail-closed principle preserved for local writes)."
metrics:
  duration: "~20 minutes"
  completed: "2026-05-28"
  tasks_completed: 4
  files_created: 6
  files_modified: 2
  tests_added: 10
---

# Phase 6 Plan 03: Audit Sinks + Config Extensions Summary

Implements the Sink interface and fan-out MultiSink for the audit package, plus three remote sink implementations (SyslogSink, OTLPSink, HTTPSink). Extends Writer with mutex + rotation + fan-out, and adds AuditConfig + LlamaFirewallConfig to the config package. Closes AUDT-03 (syslog RFC 5424) and AUDT-04 (OTLP + HTTPS POST).

## What Was Built

**internal/audit/sink.go** — Sink interface, WriterSink wrapper, MultiSink fan-out (no short-circuit on error), NewMultiSink constructor that wires file+optional remote sinks from config.AuditConfig. Includes ErrSyslogNotSupported handling and remote-sink data-egress warning.

**internal/audit/otlp.go** — OTLPSink that batches up to 100 AuditRecords and POSTs OTLP LogsData JSON to a configurable endpoint. Auto-flushes at 100 records; flushes remainder on Close. Flush errors are fire-and-forget.

**internal/audit/http_sink.go** — HTTPSink that POSTs each AuditRecord as a single application/x-ndjson line. Per-record, 5-second timeout, fire-and-forget on error.

**internal/audit/syslog.go** (linux || darwin) — SyslogSink using log/syslog. RFC 5424 framing with facility LOG_LOCAL0, severity LOG_INFO. Address format: "proto:host:port" or "host:port" (UDP default).

**internal/audit/syslog_stub.go** (windows) — Stub that returns ErrSyslogNotSupported from NewSyslogSink; NewMultiSink in sink.go treats this as a skip-with-warning rather than a fatal error.

**internal/audit/writer.go** — Extended with sync.Mutex (safe for concurrent hook-handler calls), maxBytes rotation threshold (calls Rotate() post-write), and sinks []Sink fan-out. NewWriterWithOptions added; NewWriter preserved for backward compatibility. Close() now closes all additional sinks.

**internal/config/config.go** — AuditConfig struct (sinks list, syslog/otlp/https addresses, retention, max-size) and LlamaFirewallConfig struct (enabled, sample-rate, fail-mode, codeshield, alignment-check, python-path) added before Config struct. Both added as fields on Config with omitempty. Accessor methods: AuditRetentionDays(), AuditMaxSizeBytes(), LlamaFirewallEnabled(), LlamaFirewallSampleRate().

## Test Results

10 tests added in sink_test.go, all passing:

- TestMultiSinkFanout — both sinks receive every record
- TestMultiSinkContinuesOnError — no short-circuit; last error returned
- TestMultiSinkCloseAll — Close called on all sinks
- TestWriterSinkDelegates — delegates to real Writer
- TestNewMultiSinkFileOnly — file-only config works end-to-end
- TestOTLPFlushOnClose — 3 records flushed on Close; OTLP shape validated
- TestOTLPBatching — 100-record auto-flush fires at least one POST
- TestHTTPSinkPostsNDJSON — Content-Type header + valid JSON body verified
- TestHTTPSinkContinuesOnError — bad endpoint returns nil (fire-and-forget)
- TestSyslogNotSupportedStubOnWindows — ErrSyslogNotSupported returned on Windows

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical functionality] AuditConfig referenced in audit package**

- **Found during:** Task 1 implementation
- **Issue:** The plan spec placed AuditConfig in internal/config but also showed NewMultiSink(auditPath, AuditConfig) in the audit package — audit package had no access to the type. Using an inline duplicate would cause divergence.
- **Fix:** sink.go imports "github.com/mzansi-agentive/beekeeper/internal/config" and uses config.AuditConfig. No import cycle exists (config imports only stdlib). sink_test.go updated to use config.AuditConfig{} accordingly.
- **Files modified:** internal/audit/sink.go, internal/audit/sink_test.go

## Known Stubs

None — all sink implementations are fully wired with real I/O.

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: data-egress | internal/audit/sink.go | NewMultiSink fans audit records to configurable remote endpoints (syslog/OTLP/HTTPS); egress warning printed to stderr on startup when remote sinks are active |

## Self-Check: PASSED

- internal/audit/sink.go — FOUND
- internal/audit/sink_test.go — FOUND
- internal/audit/otlp.go — FOUND
- internal/audit/http_sink.go — FOUND
- internal/audit/syslog.go — FOUND
- internal/audit/syslog_stub.go — FOUND
- internal/audit/writer.go — FOUND (modified)
- internal/config/config.go — FOUND (modified)
- All 10 sink tests: PASS
- All 8 config tests: PASS
- go build ./...: PASS
- go vet ./internal/audit/...: PASS
