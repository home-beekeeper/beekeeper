---
phase: 01-foundation-hook-handler
plan: 05
type: execute
wave: 4
depends_on: [01, 04]
files_modified:
  - internal/audit/types.go
  - internal/audit/writer.go
  - internal/audit/writer_test.go
  - cmd/beekeeper/main.go
autonomous: true
requirements: [CTLG-07]
must_haves:
  truths:
    - "Every policy decision can be written as one NDJSON record to ~/.beekeeper/audit/beekeeper.ndjson"
    - "The audit log file is owner-only (0600 on Unix, owner-only DACL on Windows) from the first write"
    - "Audit records carry decision provenance: scanner_name beekeeper, agent, tool, decision, reason, rule_ids, catalog_matches"
    - "beekeeper audit tail streams the live audit log to the terminal"
    - "Permissions are re-applied on every open so a recreated file is never world-readable"
  artifacts:
    - path: "internal/audit/types.go"
      provides: "Bumblebee-compatible AuditRecord type"
      exports: ["AuditRecord", "FromDecision"]
    - path: "internal/audit/writer.go"
      provides: "Append-only NDJSON writer with owner-only permission enforcement"
      exports: ["Writer", "NewWriter", "Write"]
  key_links:
    - from: "internal/audit/writer.go"
      to: "internal/platform.SetOwnerOnly"
      via: "applied after every OpenFile of the audit log"
      pattern: "SetOwnerOnly"
    - from: "internal/audit/types.go"
      to: "internal/policy.Decision"
      via: "FromDecision maps a Decision to an AuditRecord"
      pattern: "policy\\.Decision"
    - from: "cmd/beekeeper/main.go"
      to: "internal/audit"
      via: "audit tail subcommand reads the log"
      pattern: "audit\\."
---

<objective>
Implement the Phase 1 NDJSON audit log: a Bumblebee-schema-compatible append-only writer that records one record per policy decision with owner-only file permissions, plus the `beekeeper audit tail` command to stream it. This is the minimum audit surface HOOK-01 requires (every decision must be logged); full audit sinks (syslog, OTLP, query/export, rotation) are Phase 6 and out of scope.

Purpose: A safety harness must produce a tamper-evident, private record of every allow/warn/block decision — including fail-closed decisions. Owner-only permissions prevent another local user from reading what packages/credentials passed through.
Output: `internal/audit` package (types + writer) and a working `beekeeper audit tail` subcommand. Runs in Wave 4 because it depends on the policy `Decision` type (plan 04) and the `platform.SetOwnerOnly` primitive (plan 01); the hook handler (plan 05→wave 5) consumes this writer.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/phases/01-foundation-hook-handler/01-CONTEXT.md
@.planning/phases/01-foundation-hook-handler/01-RESEARCH.md
@CLAUDE.md
@.planning/phases/01-foundation-hook-handler/01-01-SUMMARY.md
@.planning/phases/01-foundation-hook-handler/01-04-SUMMARY.md

<interfaces>
<!-- From plan 01: -->
```go
// internal/platform
func AuditDir() (string, error)
func SetOwnerOnly(path string) error
```
<!-- From plan 04: -->
```go
// internal/policy
type Decision struct {
    Allow bool; Level, Reason string; RuleIDs []string; CatalogMatches []CatalogMatch
}
type CatalogMatch struct { CatalogSource, EntryID, Ecosystem, Package, Version, Severity string; Signed bool }
type ToolCall struct { AgentName, ToolName string; ToolInput map[string]any }
```

<!-- Contracts this plan CREATES — the hook handler (plan 05) consumes Writer + FromDecision: -->
```go
// internal/audit/types.go  (Bumblebee-compatible record per AUDT-01/CTLG-09 minimum)
type AuditRecord struct {
    RecordType     string             `json:"record_type"`     // "policy_decision"
    RecordID       string             `json:"record_id"`       // ULID/UUID-ish; caller-supplied or generated deterministically
    Timestamp      string             `json:"timestamp"`       // RFC3339; caller-supplied for purity/testability
    ScannerName    string             `json:"scanner_name"`    // always "beekeeper"
    AgentName      string             `json:"agent_name"`
    ToolName       string             `json:"tool_name"`
    Decision       string             `json:"decision"`        // allow|warn|block
    Reason         string             `json:"reason"`
    RuleIDs        []string           `json:"rule_ids"`
    CatalogMatches []CatalogProvenance `json:"catalog_matches"`
    Endpoint       string             `json:"endpoint"`        // "check" in Phase 1
}
type CatalogProvenance struct {
    CatalogSource string `json:"catalog_source"`
    EntryID       string `json:"entry_id"`
    Ecosystem     string `json:"ecosystem"`
    Package       string `json:"package"`
    Version       string `json:"version"`
    Severity      string `json:"severity"`
    Signed        bool   `json:"signed"`
}
// FromDecision maps a policy Decision + tool call + metadata into an AuditRecord.
func FromDecision(tc policy.ToolCall, d policy.Decision, recordID, timestamp string) AuditRecord

// internal/audit/writer.go
type Writer struct { /* holds path */ }
// NewWriter opens (creating if needed) the audit log at path and enforces owner-only perms.
func NewWriter(path string) (*Writer, error)
// Write appends one NDJSON record and re-enforces owner-only perms.
func (w *Writer) Write(rec AuditRecord) error
func (w *Writer) Close() error
```
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: AuditRecord type + FromDecision mapping</name>
  <read_first>
    - .planning/phases/01-foundation-hook-handler/01-CONTEXT.md (NDJSON Audit Log Phase 1 minimum fields)
    - .planning/phases/01-foundation-hook-handler/01-04-SUMMARY.md (policy Decision/CatalogMatch shapes as built)
    - CLAUDE.md (AUDT-01 schema, scanner_name "beekeeper")
  </read_first>
  <files>internal/audit/types.go, internal/audit/writer_test.go</files>
  <behavior>
    - FromDecision(allow decision) sets RecordType "policy_decision", ScannerName "beekeeper", Decision "allow", Endpoint "check", empty CatalogMatches
    - FromDecision(warn decision with one catalog match) maps the match into one CatalogProvenance with catalog_source, entry_id, ecosystem, package, version, severity, and signed copied through
    - FromDecision copies AgentName and ToolName from the ToolCall
    - FromDecision uses the caller-supplied recordID and timestamp verbatim (keeps mapping pure/testable; the hook handler supplies real values)
    - Marshalling an AuditRecord to JSON yields keys record_type, record_id, timestamp, scanner_name, agent_name, tool_name, decision, reason, rule_ids, catalog_matches, endpoint
  </behavior>
  <action>
    Create internal/audit/types.go (package `audit`) with `AuditRecord` and `CatalogProvenance` structs exactly as in the interfaces block, with the JSON tags shown. Implement `FromDecision(tc policy.ToolCall, d policy.Decision, recordID, timestamp string) AuditRecord`: set RecordType "policy_decision", RecordID and Timestamp from the args, ScannerName "beekeeper", AgentName/ToolName from tc, Decision from d.Level, Reason from d.Reason, RuleIDs from d.RuleIDs, Endpoint "check", and map each d.CatalogMatches element into a CatalogProvenance. Import internal/policy for the source types. This file may import encoding/json indirectly only via tags — FromDecision itself does no I/O.

    Add the FromDecision tests to writer_test.go (TestFromDecisionAllow, TestFromDecisionWarnWithMatch, TestAuditRecordJSONKeys — the last marshals and asserts the presence of every required key).
  </action>
  <verify>
    <automated>go test ./internal/audit/... -run "TestFromDecision|TestAuditRecordJSONKeys" -count=1 2>&1</automated>
  </verify>
  <acceptance_criteria>
    - `go test ./internal/audit/... -run TestFromDecision -count=1` exits 0
    - `internal/audit/types.go` AuditRecord has json tags `record_type`, `record_id`, `timestamp`, `scanner_name`, `agent_name`, `tool_name`, `decision`, `reason`, `rule_ids`, `catalog_matches`, `endpoint`
    - FromDecision sets ScannerName to the literal "beekeeper" and Endpoint to "check"
    - TestAuditRecordJSONKeys confirms all eleven JSON keys are present after marshalling
    - FromDecision maps each catalog match's Signed flag through to CatalogProvenance.Signed (CTLG-07 provenance)
  </acceptance_criteria>
  <done>A Bumblebee-compatible AuditRecord type exists and a pure FromDecision maps policy output into it, unit-tested.</done>
</task>

<task type="auto">
  <name>Task 2: Owner-only NDJSON append writer + audit tail command</name>
  <read_first>
    - .planning/phases/01-foundation-hook-handler/01-RESEARCH.md (Pattern 4 Windows permissions, Pitfall 5 ACL not sticky on recreation, Claude's Discretion on rotation deferred)
    - internal/platform/perms_unix.go + perms_windows.go (SetOwnerOnly from plan 01)
    - internal/audit/types.go (AuditRecord from Task 1)
    - cmd/beekeeper/main.go (audit tail stub from plan 01)
  </read_first>
  <files>internal/audit/writer.go, internal/audit/writer_test.go, cmd/beekeeper/main.go</files>
  <action>
    Create internal/audit/writer.go. `NewWriter(path string) (*Writer, error)`: MkdirAll the parent dir, OpenFile with `os.O_APPEND|os.O_CREATE|os.O_WRONLY` and mode 0600 (the O_APPEND avoids file recreation per Pitfall 5), then immediately call `platform.SetOwnerOnly(path)` and return the error if non-nil. Keep the open *os.File on the Writer. `Write(rec AuditRecord) error`: json.Marshal the record, append a single line (record bytes + "\n") to the file, and call `platform.SetOwnerOnly(path)` again AFTER the write (Pitfall 5: re-apply on every write so a truncated/recreated file is never left world-readable on Windows). Flush/sync is acceptable but not required for Phase 1. `Close() error` closes the file. No rotation in Phase 1 (deferred to Phase 6 per Claude's Discretion).

    Wire the `audit tail` subcommand RunE in main.go: resolve the audit log path via `platform.AuditDir()` + "beekeeper.ndjson", open read-only, and stream existing lines to stdout, then follow appended lines (a simple poll loop reading new bytes since last offset, ~500ms interval, until interrupted). Keep it dependency-free (stdlib only). Replace the not-implemented stub from plan 01. Do NOT implement `audit query` or `audit export` — those are AUDT-06/07, Phase 6, out of scope.

    Add writer tests: TestNDJSONWriteAppends (write two records, read file back, assert two newline-delimited JSON lines that unmarshal), TestPermissionsEnforced (on Unix assert `os.Stat(path).Mode().Perm() == 0600` after NewWriter and after Write; on Windows assert NewWriter and Write return nil — DACL byte assertion out of scope), TestWriteReappliesPermsAfterWrite (Unix: chmod the file to 0644 between writes, call Write, assert it returns to 0600).
  </action>
  <verify>
    <automated>go test ./internal/audit/... -count=1 2>&1 && go build ./... 2>&1</automated>
  </verify>
  <acceptance_criteria>
    - `go test ./internal/audit/... -count=1` exits 0
    - `internal/audit/writer.go` opens the file with `os.O_APPEND|os.O_CREATE|os.O_WRONLY` (grep confirms O_APPEND)
    - `platform.SetOwnerOnly` is called in BOTH NewWriter and Write (Pitfall 5 re-application — grep shows at least two call sites)
    - On Unix, TestPermissionsEnforced asserts mode 0600 and TestWriteReappliesPermsAfterWrite proves perms are restored after an external chmod
    - `beekeeper audit tail` is wired to `internal/audit`/`platform.AuditDir` and no longer returns "not yet implemented"
    - No `audit query` or `audit export` subcommand exists (grep returns nothing) — deferred to Phase 6
  </acceptance_criteria>
  <done>Decisions append as owner-only NDJSON; `beekeeper audit tail` streams the live log; permissions survive file recreation.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| process → filesystem | The audit log records sensitive decision metadata and must be readable only by its owner |
| other local user → audit log | On multi-user machines, other users must not read what packages/tools passed through |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-05-01 | Information Disclosure | World-readable audit log leaking package/credential context | mitigate | `SetOwnerOnly` (0600 Unix / DACL Windows) applied in NewWriter and re-applied after every Write (Pitfall 5); TestPermissionsEnforced + TestWriteReappliesPermsAfterWrite |
| T-05-02 | Repudiation | Missing or partial decision records | mitigate | Append-only O_APPEND writer; every decision (including fail-closed) is written by the hook handler (plan 05); scanner_name/timestamp/record_id provenance per AUDT-01 |
| T-05-03 | Tampering | Audit log truncation/recreation resetting permissions on Windows | mitigate | O_APPEND avoids recreation; SetOwnerOnly re-applied on every write so a recreated file is re-locked (Pitfall 5) |
| T-05-04 | Denial of Service | Unbounded audit log growth | accept | Rotation/retention is AUDT-02/Phase 6 (out of scope); Phase 1 accepts unbounded local growth, documented |
</threat_model>

<verification>
- `go test ./internal/audit/... -count=1` exits 0
- `go build ./...` exits 0
- Manual: after a `beekeeper check`, `beekeeper audit tail` shows the decision record
- On Unix the audit file is mode 0600; on Windows the writer applies a DACL without error
</verification>

<success_criteria>
- Bumblebee-compatible NDJSON audit record per decision with full catalog provenance (AUDT-01 minimum, CTLG-07 signedness)
- Owner-only permissions from first write, re-applied on every write (cross-platform)
- Working `beekeeper audit tail`
- No Phase 6 audit features (query/export/sinks/rotation) leaked into Phase 1
</success_criteria>

<output>
After completion, create `.planning/phases/01-foundation-hook-handler/01-05-SUMMARY.md`
</output>
