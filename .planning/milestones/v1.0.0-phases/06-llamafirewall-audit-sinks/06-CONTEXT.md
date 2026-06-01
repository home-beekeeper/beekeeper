# Phase 6: LlamaFirewall + Audit Sinks — Context

**Gathered:** 2026-05-28
**Status:** Ready for planning
**Source:** PRD Express Path (beekeeper-prd.md §5.9, §7.1, §15 + REQUIREMENTS.md LLMF-01–06, AUDT-01–07)

<domain>
## Phase Boundary

Phase 6 delivers two parallel tracks:

**Track A — Audit Sinks:** The existing local NDJSON audit log gains log rotation, 30-day retention, filtering queries, multi-format export, and three optional remote sinks (syslog RFC 5424, OTLP, HTTPS POST). All remote sinks are off by default and require explicit opt-in config.

**Track B — LlamaFirewall Sidecar:** An optional Python 3.11+ sidecar running PromptGuard 2 (86M BERT), CodeShield, and experimental AlignmentCheck is supervised by Beekeeper. Tool outputs flowing into agent context (WebFetch results, file reads, MCP tool responses) are scanned for prompt injection; agent-generated code writes are evaluated by CodeShield; chain-of-thought content is optionally checked by AlignmentCheck.

Thirteen requirements in scope: LLMF-01 through LLMF-06, AUDT-01 (enhancement), AUDT-02 through AUDT-07.

Out of scope for this phase: TUI dashboard (Phase 8), `beekeeper diag` full display (Phase 9 CODE-06), macOS/Windows Sentry (Phase 7), SLSA provenance (Phase 7).

</domain>

<decisions>
## Implementation Decisions

### Architecture: Where New Code Lives

| Component | Package |
|-----------|---------|
| Log rotation + retention | `internal/audit/rotate.go` |
| Audit record query (streaming filter) | `internal/audit/query.go` |
| Audit export (ndjson, csv, otlp) | `internal/audit/export.go` |
| Sink interface + MultiSink | `internal/audit/sink.go` |
| Syslog sink (Unix only) | `internal/audit/syslog.go` (build linux,darwin) |
| OTLP sink (HTTP transport) | `internal/audit/otlp.go` |
| HTTPS POST sink | `internal/audit/http_sink.go` |
| LlamaFirewall IPC protocol | `internal/llamafirewall/proto.go` |
| LlamaFirewall IPC client | `internal/llamafirewall/client.go` (unix,darwin) + `client_windows.go` |
| LlamaFirewall supervisor | `internal/llamafirewall/supervisor.go` |
| LlamaFirewall latency tracking | `internal/llamafirewall/latency.go` |
| Python sidecar | `sidecar/llamafirewall_sidecar.py` |
| Sidecar requirements | `sidecar/requirements.txt` |
| Config extensions | `internal/config/config.go` (new `AuditConfig`, `LlamaFirewallConfig`) |
| CLI additions | `cmd/beekeeper/main.go` (audit query/export, llamafirewall commands) |

### AUDT-02: Log Rotation and Retention

**Rotation strategy:** Size-based trigger at configurable threshold (default 10 MB). On rotation:
1. Rename `beekeeper.ndjson` → `beekeeper.ndjson.1` (shift existing numbered archives up, drop oldest past `retention_days`).
2. Create new empty `beekeeper.ndjson` with `0600` permissions.
3. Re-apply `platform.SetOwnerOnly`.

**Retention:** Rotated files older than `retention_days` (default 30) are deleted on rotation. The current file is never deleted on retention grounds alone.

**Rotator location:** `internal/audit/rotate.go`. Called from `Writer.Write` when the file size exceeds the threshold — checked via `os.Stat` after every write (cheap: one syscall per write, which is already dominated by the write itself).

**Config keys:** `audit.max_size_bytes` (int64, default 10×1024×1024) and `audit.retention_days` (int, default 30).

### AUDT-05: audit tail (enhancement)

The existing `tailAuditLog` in `cmd/beekeeper/main.go` already works (Phase 1/4 basic tail). In Phase 6 it gains a `--follow=false` flag (dump existing records and exit, no follow). No other changes to tail — it is correct as-is.

### AUDT-06: audit query

**Function signature:** `func Query(ctx context.Context, r io.Reader, opts QueryOpts, out io.Writer) error`

**QueryOpts:**
```go
type QueryOpts struct {
    Since    time.Time // zero = no lower bound
    Agent    string    // empty = no filter
    Tool     string    // empty = no filter
    Decision string    // empty = no filter  (allow|warn|block)
    Limit    int       // 0 = no limit
}
```

**Implementation:** Stream-reads NDJSON lines, unmarshals each to `AuditRecord`, applies filters, writes matching records as NDJSON to `out`. No full-file slurp. Handles malformed lines by skipping with a counter (reported at end if nonzero).

**CLI:** `beekeeper audit query [--since 2h|RFC3339] [--agent NAME] [--tool NAME] [--decision allow|warn|block] [--limit N]` — reads from the active audit log (falls back to numbered rotated files if `--since` spans a rotation boundary).

### AUDT-07: audit export

**Function signature:** `func Export(ctx context.Context, r io.Reader, opts ExportOpts, out io.Writer) error`

**ExportOpts:**
```go
type ExportOpts struct {
    Format string    // "ndjson" | "csv" | "otlp"
    QueryOpts        // embedded — same filter fields as query
}
```

**Formats:**
- `ndjson`: identical to query output (NDJSON records matching filter)
- `csv`: header row + one row per record; fields: `record_type,record_id,timestamp,scanner_name,agent_name,tool_name,decision,reason,rule_ids,endpoint`; `rule_ids` is pipe-joined. Uses `encoding/csv`.
- `otlp`: OTLP LogsData JSON format (manual encoding — no OTel SDK dependency). Schema: one `ResourceLogs` block with `scope_logs` containing one `LogRecord` per audit record, with `body.string_value` = JSON of the record and `attributes` carrying the key fields (decision, tool_name, agent_name) as `KeyValue` pairs.

**OTLP JSON structure:**
```json
{
  "resourceLogs": [{
    "resource": {"attributes": [{"key": "service.name", "value": {"stringValue": "beekeeper"}}]},
    "scopeLogs": [{
      "scope": {"name": "beekeeper/audit"},
      "logRecords": [
        {
          "timeUnixNano": "...",
          "body": {"stringValue": "<raw JSON line>"},
          "attributes": [
            {"key": "beekeeper.decision", "value": {"stringValue": "block"}},
            {"key": "beekeeper.tool_name", "value": {"stringValue": "Install"}},
            {"key": "beekeeper.agent_name", "value": {"stringValue": "claude-code"}}
          ]
        }
      ]
    }]
  }]
}
```

No external OTLP SDK required — this is hand-rolled JSON encoding for the log export case.

### AUDT-03, AUDT-04: Remote Sinks

**Sink interface** (`internal/audit/sink.go`):
```go
type Sink interface {
    Write(rec AuditRecord) error
    Close() error
}
```

**MultiSink**: fan-out, writes to all sinks in order, returns last non-nil error (does not short-circuit on error — all sinks receive every record).

**SyslogSink** (`internal/audit/syslog.go`, build tag `linux,darwin`):
- Uses stdlib `log/syslog` package. Priority: `syslog.LOG_LOCAL0 | syslog.LOG_INFO`.
- RFC 5424 format: the Go stdlib `log/syslog.Writer.Info()` writes RFC 3164-ish, but for RFC 5424 compliance we format the message manually as `<PRI>1 TIMESTAMP HOSTNAME APP-NAME PROCID MSGID [SD-ID fields] MSG` and write via `syslog.New(priority, tag).Write(formattedLine)`.
- Endpoint config: `audit.syslog_address` = `host:port` (UDP default). Format: `<proto>:<host>:<port>` where proto is `udp` or `tcp` (default `udp`).
- Stub file `internal/audit/syslog_stub.go` (build tag `windows`) returns `ErrSyslogNotSupported`.

**OTLPSink** (`internal/audit/otlp.go`):
- HTTP POST with `Content-Type: application/json` to `audit.otlp_endpoint` (OTLP HTTP JSON transport, port 4318 by default).
- Batching: accumulates records in a slice, flushes on `Close()` or when batch reaches 100 records. Uses a mutex-protected buffer.
- Each flush POSTs one OTLP LogsData JSON payload (same format as the export `otlp` format).
- On flush error: logs to stderr, continues (fire-and-forget with error logging). Does not fail the local write.

**HTTPSink** (`internal/audit/http_sink.go`):
- POST each record as a single-record NDJSON body to `audit.https_endpoint`.
- `Content-Type: application/x-ndjson`.
- Timeout: 5 seconds per POST.
- On error: logs to stderr, continues (same fire-and-forget as OTLP).
- No batching for simplicity; add batching in v2 if throughput requires it.

**Sink initialization** (called from `newWriterCmd`-style or from `Writer` constructor):
- `internal/audit/sink.go` exports `NewMultiSink(cfg AuditConfig) (Sink, error)` which reads the config and creates the appropriate sinks.
- The local file sink is always created first (WriterSink wrapping the existing `Writer`).

**Config warnings:** When any non-file sink is enabled, Beekeeper prints a one-time warning to stderr on startup: `WARNING: audit data will leave this machine via [syslog|OTLP|HTTPS] sink configured at <endpoint>. Disable with audit.sinks in ~/.beekeeper/config.json.`

### LLMF-05: LlamaFirewall IPC Protocol

**Socket path:** `~/.beekeeper/llamafirewall.sock` (Unix/macOS)
**Named pipe:** `\\.\pipe\beekeeper-llamafirewall` (Windows)

**Protocol:** Length-prefixed JSON — identical framing to `internal/ipc/proto.go` (4-byte big-endian uint32 length + JSON payload, 1MB max message size for potentially large code content).

**Request types:**
```go
type ScanKind string
const (
    ScanPrompt    ScanKind = "scan_prompt"    // LLMF-02: PromptGuard 2
    ScanCode      ScanKind = "scan_code"      // LLMF-03: CodeShield
    ScanAlignment ScanKind = "scan_alignment" // LLMF-04: AlignmentCheck (experimental)
)

type ScanRequest struct {
    Kind      ScanKind `json:"kind"`
    Content   string   `json:"content"`   // text to scan
    Context   string   `json:"context,omitempty"` // tool name / surrounding context
    RequestID string   `json:"request_id"` // caller-generated UUID for correlation
}

type ScanResult string
const (
    ResultClean     ScanResult = "clean"
    ResultInjection ScanResult = "injection"    // for scan_prompt
    ResultUnsafe    ScanResult = "unsafe"        // for scan_code
    ResultHijacked  ScanResult = "hijacked"      // for scan_alignment
)

type ScanResponse struct {
    RequestID  string     `json:"request_id"`
    Result     ScanResult `json:"result"`
    Confidence float64    `json:"confidence"` // 0.0–1.0
    Reason     string     `json:"reason,omitempty"`
    LatencyMS  int64      `json:"latency_ms"` // inference time in the Python process
    Error      string     `json:"error,omitempty"` // non-empty means sidecar error
}
```

**1MB max message size** for LlamaFirewall (vs 64KB for Sentry IPC) because code files can be large.

**LlamaFirewall client** (`internal/llamafirewall/client.go`, build `linux,darwin`):
```go
type Client struct {
    conn    net.Conn
    timeout time.Duration
}
func Dial(sockPath string, timeout time.Duration) (*Client, error)
func (c *Client) Scan(req ScanRequest) (ScanResponse, error)
func (c *Client) Close() error
```

Windows client (`internal/llamafirewall/client_windows.go`) uses named pipes via `golang.org/x/sys/windows` (no CGO, pure Go).

**Fuzz test** (`internal/llamafirewall/proto_fuzz_test.go`) — same pattern as `internal/ipc/proto_fuzz_test.go`. This is a Phase 6 release gate (added to existing fuzz gates in CI).

### LLMF-01: Supervisor

**Supervisor** (`internal/llamafirewall/supervisor.go`):
```go
type Supervisor struct {
    PythonPath  string   // path to python3 executable
    SidecarPath string   // path to llamafirewall_sidecar.py
    SockPath    string   // Unix socket path
    MaxRetries  int      // default 3
    
    mu         sync.Mutex
    proc       *os.Process
    retries    int
    degraded   bool
    client     *Client
}

func NewSupervisor(cfg LlamaFirewallConfig, sockPath, sidecarPath string) *Supervisor
func (s *Supervisor) Start(ctx context.Context) error
func (s *Supervisor) Scan(ctx context.Context, req ScanRequest) (ScanResponse, error)
func (s *Supervisor) Stop() error
func (s *Supervisor) IsDegraded() bool
```

**Restart loop:** goroutine that watches for process exit; on exit, if retries < maxRetries, waits `min(pow(2, retries), 30)` seconds and relaunches. After maxRetries, sets `degraded = true`.

**Degraded behavior:** `Scan()` when degraded returns a synthetic response with `Result: ResultClean` (if fail mode is `open`) or returns `ErrSidecarUnavailable` (if fail mode is `closed`). Default: `fail_closed`.

**State persistence:** supervisor PID stored in `~/.beekeeper/state.json` under `"llamafirewall"` key for `beekeeper llamafirewall status`.

### LLMF-06: Sample Rate + Latency Tracking

**Sample rate:** `rand.Float64() < cfg.SampleRate` gate in `Supervisor.Scan()`. If not sampled, return synthetic `ResultClean` with `LatencyMS: 0`.

**Latency tracking** (`internal/llamafirewall/latency.go`):
```go
type LatencyTracker struct {
    mu     sync.Mutex
    count  int64
    sumMS  int64
    p95buf [100]int64  // ring buffer for last 100 latencies
    head   int
}
func (t *LatencyTracker) Record(ms int64)
func (t *LatencyTracker) P95() int64
func (t *LatencyTracker) Mean() float64
```

Supervisor holds a `*LatencyTracker`; updates on every non-sampled scan. Exposed via `LlamaFirewallStatus.P95LatencyMS` for CLI display.

### LLMF-02, LLMF-03, LLMF-04: Policy Integration

**Hook handler integration** (`internal/check/handler.go`):
- After a successful tool call result (PostToolUse path in the hook handler):
  - If result contains `web_search` or `read_file` tool responses: run `ScanPrompt`
  - If result contains `write_file` tool call with code: run `ScanCode`
- On injection detection: inject a structured warning record into the result; write `llmf_alert` audit record.
- AlignmentCheck: if `llamafirewall.alignment_check = true`, scan the tool input for goal-hijacking signals.

**Gateway integration** (`internal/gateway/policy.go`):
- After proxying an upstream MCP tool response: run `ScanPrompt` on the response body.
- On injection detection: replace the `result` field of the JSON-RPC response with a structured warning.

**AuditRecord additions** for Phase 6 (appended to `internal/audit/types.go`):
```go
// Phase 6 additions (LLMF-02, LLMF-03, LLMF-04)
LLMFScanned    bool       `json:"llmf_scanned,omitempty"`
LLMFScanKind   string     `json:"llmf_scan_kind,omitempty"`   // prompt|code|alignment
LLMFResult     string     `json:"llmf_result,omitempty"`      // clean|injection|unsafe|hijacked
LLMFConfidence float64    `json:"llmf_confidence,omitempty"`
LLMFLatencyMS  int64      `json:"llmf_latency_ms,omitempty"`
```

**Warning payload injected into agent context on injection detection:**
```json
{
  "beekeeper_alert": true,
  "alert_type": "prompt_injection",
  "confidence": 0.97,
  "reason": "PromptGuard 2 detected indirect prompt injection attempt in tool output",
  "original_content_redacted": true
}
```

### Config Extensions

New fields in `internal/config/config.go`:

```go
type AuditConfig struct {
    Sinks         []string `json:"sinks,omitempty"`          // ["file","syslog","otlp","https"]
    SyslogAddress string   `json:"syslog_address,omitempty"` // proto:host:port or host:port
    OTLPEndpoint  string   `json:"otlp_endpoint,omitempty"`  // https://collector:4318
    HTTPSEndpoint string   `json:"https_endpoint,omitempty"` // arbitrary HTTPS POST URL
    RetentionDays int      `json:"retention_days,omitempty"` // default 30
    MaxSizeBytes  int64    `json:"max_size_bytes,omitempty"` // default 10MB
}

type LlamaFirewallConfig struct {
    Enabled          bool    `json:"enabled"`
    SampleRate       float64 `json:"sample_rate,omitempty"`       // 0.0-1.0, default 1.0
    FailMode         string  `json:"fail_mode,omitempty"`         // closed|open|warn
    CodeShield       bool    `json:"codeshield,omitempty"`        // default true when enabled
    AlignmentCheck   bool    `json:"alignment_check,omitempty"`   // experimental
    CodeShieldAction string  `json:"codeshield_action,omitempty"` // warn|block
    PythonPath       string  `json:"python_path,omitempty"`       // default "python3"
}
```

`Config` struct gains `Audit AuditConfig` and `LlamaFirewall LlamaFirewallConfig` fields.

### New CLI Commands

```
beekeeper audit query [--since <dur|RFC3339>] [--agent NAME] [--tool NAME] [--decision allow|warn|block] [--limit N]
beekeeper audit export --format ndjson|csv|otlp [--since ...] [--agent ...] [--tool ...] [--decision ...]
beekeeper audit tail [--no-follow]

beekeeper llamafirewall enable
beekeeper llamafirewall disable
beekeeper llamafirewall status
```

### Self-Defense Deliverables (per PRD §15, v0.9.0)

- LlamaFirewall sidecar fail-closed by default on crash/unavailability (LLMF-01, LLMF-05)
- Remote audit sinks explicitly opt-in with documented warning (AUDT-03, AUDT-04)
- Audit log rotation enforces `0600` permissions on the new file (AUDT-02)
- LlamaFirewall IPC fuzz test as Phase 6 release gate
- HTTPS POST sink uses Go's stdlib TLS (system trust store) — no custom cert pinning in v1, which is documented as a known limitation in comments

### Claude's Discretion

**Resolved decisions for Phase 6:**

- **OTLP library**: No external OTel SDK. Manual JSON encoding of OTLP LogsData format for both the live OTLP sink and the export command. This avoids a 200+ package transitive dependency tree and keeps the binary lightweight. The OTLP HTTP JSON protocol is stable and simple to encode manually.
- **Syslog on Windows**: `log/syslog` is not available on Windows. The syslog sink is `//go:build linux darwin` only. `internal/audit/syslog_stub.go` (build windows) exports `NewSyslogSink` returning `ErrSyslogNotSupported`. Config validation rejects `audit.sinks: ["syslog"]` on Windows with a clear error.
- **Log rotation race**: rotation is not safe for concurrent writers. The `Writer` struct holds a mutex; `Rotate()` acquires it, closes the old file, renames, opens new file, releases. The daemon path (Sentry alerts, gateway audit writes) always goes through the same `Writer` instance, so no inter-process race.
- **LlamaFirewall Windows IPC**: Named pipe support via `golang.org/x/sys/windows` (`CreateNamedPipe`, `ConnectNamedPipe`, `CreateFile` for client side). No CGO. Build-tagged `//go:build windows` in `internal/llamafirewall/client_windows.go`.
- **Python sidecar discovery**: Default `python_path: "python3"`. On Windows default `python_path: "python"`. The sidecar Python file is discovered relative to the beekeeper binary path (`filepath.Dir(os.Executable())/../sidecar/llamafirewall_sidecar.py`) with a fallback to `~/.beekeeper/sidecar/llamafirewall_sidecar.py` for user-installed sidecars.
- **LlamaFirewall not available in CI**: The Python sidecar integration tests use a Go mock server (not the real Python), allowing CI to test supervision logic, IPC framing, fail-closed behavior, and sample-rate gating without requiring a Python environment with PromptGuard 2 installed.
- **Audit query spanning rotated files**: `beekeeper audit query --since` with a time that spans a rotation boundary reads from the oldest matching rotated file first. Query logic detects `--since` time and checks `beekeeper.ndjson.N` files in order.
- **OTLP sink batching**: batch up to 100 records; flush on `Close()`. Flush is also triggered on process exit via the supervisor's cleanup goroutine.
- **LlamaFirewall enable/disable**: `beekeeper llamafirewall enable` sets `llamafirewall.enabled: true` in config and optionally starts the sidecar process (if config has `python_path` and sidecar path). `disable` sets enabled to false and sends SIGTERM to the sidecar if running.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Architecture and Constraints
- `CLAUDE.md` — Single static binary, no CGO in core, `internal/policy` pure, fail-closed default
- `.planning/PROJECT.md` — Core decisions, locked choices
- `.planning/REQUIREMENTS.md` — LLMF-01–06, AUDT-01–07 verbatim
- `.planning/ROADMAP.md` — Phase 6 success criteria, dependency on Phase 5

### Prior Phase Patterns (reuse directly)
- `internal/audit/writer.go` — existing `Writer` (extend, don't replace; add rotation + sink dispatch)
- `internal/audit/types.go` — `AuditRecord` struct (append new LLMF fields here)
- `internal/audit/redact.go` — `RedactRecord`, `DefaultRedactPatterns`
- `internal/ipc/proto.go` — length-prefix framing pattern for LlamaFirewall IPC (COPY pattern, don't import)
- `internal/ipc/server.go` — SO_PEERCRED pattern (reference)
- `internal/config/config.go` — Config struct pattern; extend with `AuditConfig` + `LlamaFirewallConfig`
- `internal/catalog/watch.go` — foreground daemon pattern with `signal.NotifyContext`
- `internal/gateway/policy.go` — policy integration point for gateway; add LLMF scan here
- `internal/check/handler.go` — hook handler integration point; add LLMF scan here
- `cmd/beekeeper/main.go` — Cobra wiring; extend `newAuditCmd()`, add `newLlamaFirewallCmd()`
- `internal/sentry/baseline.go` — restart/backoff pattern reference for supervisor

### External Libraries (new for Phase 6)
- `golang.org/x/sys/windows` (already in transitive deps) — Named pipe for Windows LlamaFirewall IPC
- No new Go dependencies required — OTLP encoding is manual, syslog uses stdlib, HTTP uses stdlib `net/http`
- Python sidecar deps: `llama-firewall`, `torch`, `transformers`, `fastapi` (or raw `socket`) — `sidecar/requirements.txt`

</canonical_refs>

<specifics>
## Specific Ideas

### Rotated File Naming Convention
```
~/.beekeeper/audit/
├── beekeeper.ndjson       ← current (active writes)
├── beekeeper.ndjson.1     ← most recent rotation
├── beekeeper.ndjson.2
└── beekeeper.ndjson.3     ← oldest kept (based on retention_days)
```

### Writer with Rotation Integration
```go
// Writer extended for Phase 6
type Writer struct {
    path     string
    file     *os.File
    mu       sync.Mutex
    maxBytes int64    // 0 = no rotation
    sinks    []Sink   // additional sinks (syslog, OTLP, HTTPS)
}

func (w *Writer) Write(rec AuditRecord) error {
    w.mu.Lock()
    defer w.mu.Unlock()
    // ... marshal + write to file ...
    // check size, rotate if needed
    // fan-out to sinks (non-blocking best-effort)
}
```

### LlamaFirewall Status Output
```
$ beekeeper llamafirewall status
LlamaFirewall Sidecar — Active (PID 45123, uptime 2h15m)
Python:     /usr/bin/python3
Models:     PromptGuard 2 (loaded), CodeShield (loaded), AlignmentCheck (disabled)
Sample Rate: 1.00 (scan all)
P95 Latency: 42ms
Fail Mode:  closed
Scans:      1,234 total — 1,230 clean, 4 flagged
Degraded:   false
```

### audit query CLI example
```
$ beekeeper audit query --since 2h --decision block
{"record_type":"policy_decision","timestamp":"2026-05-28T14:23:01Z","decision":"block",...}
{"record_type":"policy_decision","timestamp":"2026-05-28T15:01:44Z","decision":"block",...}
Found 2 matching records.
```

### audit export CSV example
```
$ beekeeper audit export --format csv --since 24h > report.csv
record_type,record_id,timestamp,scanner_name,agent_name,tool_name,decision,reason,rule_ids,endpoint
policy_decision,abc123,2026-05-28T14:23:01Z,beekeeper,claude-code,Install,block,...,bumblebee-catalog-match,check
```

### RFC 5424 Syslog Format
```
<134>1 2026-05-28T14:23:01.000Z myhost beekeeper 12345 policy_decision [beekeeper decision="block" tool="Install" agent="claude-code"] {"record_type":"policy_decision",...}
```
Priority: `LOG_LOCAL0 | LOG_INFO` = 134.

### Python Sidecar Architecture
```python
# sidecar/llamafirewall_sidecar.py
# Socket server serving ScanRequest / ScanResponse JSON over Unix socket.
# Loads PromptGuard 2 on startup, CodeShield on startup.
# AlignmentCheck loaded only if enabled flag is set in first request or config.
# Responds to each request synchronously (no async needed — Go supervisor
# serializes requests per connection).
```

### CI Gate Addition
```yaml
# In .github/workflows/ci.yml — add to fuzz-test-gate job:
- name: Smoke fuzz LlamaFirewall IPC
  run: go test -run FuzzLlamaFirewallProto -fuzztime=5s ./internal/llamafirewall/...
```

</specifics>

<deferred>
## Deferred Ideas

- macOS eslogger Sentry — Phase 7
- Windows ETW Sentry — Phase 7
- SLSA Level 3 + SBOM — Phase 7
- TUI System Health panel (shows LlamaFirewall status) — Phase 8 (TUI-08)
- `beekeeper diag` full latency display — Phase 9 (CODE-06)
- Weighted corroboration — v2 roadmap (HARD-02)
- Audit log analytics and trend detection — v2 roadmap (DIST-06)
- Named pipe batching/multiplexing for Windows LlamaFirewall IPC — v2
- LlamaFirewall sampling with stratified sampling (always scan high-risk tools) — v2
- macOS EndpointSecurity entitlement — v2 roadmap

</deferred>

---

*Phase: 06-llamafirewall-audit-sinks*
*Context gathered: 2026-05-28 via PRD Express Path (beekeeper-prd.md §5.9, §7.1, §15 + REQUIREMENTS.md LLMF-01–06, AUDT-01–07)*
*Discretion resolved: 2026-05-28 during context synthesis*
