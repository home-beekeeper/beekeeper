# Phase 1 / Plan 05 — NDJSON Audit Logging — Summary

**Plan:** `01-PLAN-audit-logging.md`
**Executed:** 2026-05-26
**Status:** Complete — all acceptance criteria met. 6/6 audit tests pass; full suite (`go test ./...`) green; `go build ./...` and `go vet` clean.
**Commit:** `f5d6489` feat: Phase 1 Plan 05 — NDJSON audit writer, owner-only perms, audit tail command
**Approach:** Task 1 TDD (FromDecision mapping tests + types written together, GREEN immediately); Task 2 writer + `audit tail` wiring with accompanying tests.

## What Was Built

The `internal/audit` package: a Bumblebee-compatible, append-only NDJSON audit
log that records exactly one record per policy decision with owner-only file
permissions, plus the `beekeeper audit tail` command that streams it live. This
is the minimum audit surface HOOK-01 requires (every decision — including
fail-closed decisions — must be logged). The hook handler (Plan 06) consumes
`FromDecision` + `Writer`.

### Files created/modified

- `internal/audit/types.go` (new) — `AuditRecord`, `CatalogProvenance`, `FromDecision`
- `internal/audit/writer.go` (new) — `Writer`, `NewWriter`, `Write`, `Close`
- `internal/audit/writer_test.go` (new) — 6 tests (3 mapping, 3 writer)
- `cmd/beekeeper/main.go` (modified) — `audit tail` RunE wired to `internal/audit` log path + new `tailAuditLog` follow loop; replaced the Plan 01 "not yet implemented" stub

## Key Interfaces Created (Plan 06 / hook handler consumes ALL of these)

```go
// internal/audit/types.go
type AuditRecord struct {
    RecordType     string              `json:"record_type"`     // "policy_decision"
    RecordID       string              `json:"record_id"`
    Timestamp      string              `json:"timestamp"`       // RFC3339
    ScannerName    string              `json:"scanner_name"`    // always "beekeeper"
    AgentName      string              `json:"agent_name"`
    ToolName       string              `json:"tool_name"`
    Decision       string              `json:"decision"`        // allow|warn|block
    Reason         string              `json:"reason"`
    RuleIDs        []string            `json:"rule_ids"`
    CatalogMatches []CatalogProvenance `json:"catalog_matches"`
    Endpoint       string              `json:"endpoint"`        // "check" in Phase 1
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

// Pure mapping (no I/O, no wall clock): recordID and timestamp are caller-supplied.
func FromDecision(tc policy.ToolCall, d policy.Decision, recordID, timestamp string) AuditRecord

// internal/audit/writer.go
type Writer struct { /* path + open *os.File */ }
func NewWriter(path string) (*Writer, error) // MkdirAll(0700) + OpenFile(O_APPEND|O_CREATE|O_WRONLY,0600) + SetOwnerOnly
func (w *Writer) Write(rec AuditRecord) error // marshal + "\n" append, then SetOwnerOnly again (Pitfall 5)
func (w *Writer) Close() error
```

### FromDecision mapping (Phase 1)

| AuditRecord field | Source |
|-------------------|--------|
| RecordType | literal `"policy_decision"` |
| RecordID / Timestamp | caller-supplied verbatim (purity/testability) |
| ScannerName | literal `"beekeeper"` |
| AgentName / ToolName | `tc.AgentName` / `tc.ToolName` |
| Decision | `d.Level` (allow\|warn\|block) |
| Reason | `d.Reason` |
| RuleIDs | `d.RuleIDs` |
| CatalogMatches | each `d.CatalogMatches[i]` → `CatalogProvenance` (all 7 fields incl. `Signed`, CTLG-07) |
| Endpoint | literal `"check"` |

### `beekeeper audit tail`

- Resolves `platform.AuditDir()` + `beekeeper.ndjson`, opens **read-only**.
- Returns a meaningful error if the log does not exist yet (rather than the old stub error).
- Streams existing content via `io.Copy`, then follows appended bytes with a ~500ms `time.Ticker` + `ReadAt(offset)` loop until `cmd.Context()` is cancelled (Ctrl+C). Stdlib only — no new dependencies.

## Acceptance Criteria Met

Task 1:
- [x] `go test ./internal/audit/... -run TestFromDecision -count=1` exits 0
- [x] AuditRecord has all 11 JSON tags (`record_type`…`endpoint`)
- [x] FromDecision sets ScannerName `"beekeeper"`, Endpoint `"check"` (literals)
- [x] `TestAuditRecordJSONKeys` asserts all 11 keys present (and exactly 11)
- [x] Each catalog match's `Signed` flag flows through to `CatalogProvenance.Signed` (CTLG-07)

Task 2:
- [x] `go test ./internal/audit/... -count=1` exits 0 (6/6)
- [x] `writer.go` opens with `os.O_APPEND|os.O_CREATE|os.O_WRONLY` (grep confirms O_APPEND)
- [x] `platform.SetOwnerOnly` called in BOTH `NewWriter` (line 38) and `Write` (line 64) — Pitfall 5 re-application
- [x] Unix: `TestPermissionsEnforced` asserts 0600 after NewWriter and after Write; `TestWriteReappliesPermsAfterWrite` proves perms restored to 0600 after an external `chmod 0644`
- [x] `beekeeper audit tail` wired to `internal/audit`/`platform.AuditDir`; no longer "not yet implemented"
- [x] No `audit query` / `audit export` subcommand exists (grep returns nothing) — deferred to Phase 6

Cross-cutting:
- [x] `go build ./...` exits 0
- [x] `go test ./...` green (audit, catalog, platform, policy all ok)
- [x] `go vet ./internal/audit/... ./cmd/...` clean

## Requirements Satisfied

- **CTLG-07** — `CatalogProvenance.Signed` carries each match's signedness verbatim into the audit record, preserving provenance.
- **AUDT-01 (Phase 1 minimum)** — Bumblebee-compatible per-decision NDJSON record with full catalog provenance; local owner-only file sink.

## Threat Mitigations Implemented

- **T-05-01 (Information Disclosure — world-readable log)** — `SetOwnerOnly` (0600 Unix / DACL Windows) applied in `NewWriter` and re-applied after every `Write`; proven by `TestPermissionsEnforced` + `TestWriteReappliesPermsAfterWrite`.
- **T-05-02 (Repudiation — partial records)** — append-only `O_APPEND` writer; `FromDecision` produces one complete record per decision; provenance fields (scanner_name/timestamp/record_id) populated.
- **T-05-03 (Tampering — perms reset on recreation)** — `O_APPEND` avoids recreation; `SetOwnerOnly` re-applied on every write so a recreated/loosened file is re-locked (Pitfall 5).
- **T-05-04 (DoS — unbounded growth)** — accepted; rotation/retention is Phase 6, out of scope (documented).

## Deviations from the Plan

1. **`tailAuditLog` helper in `main.go`.** The follow loop is implemented as a small unexported function in `cmd/beekeeper/main.go` (stdlib `io.Copy` + `time.Ticker` + `ReadAt`), keeping it dependency-free per the plan. It uses `io`/`context`/`errors`/`time` (new `context`, `io`, `errors` imports added). This is CLI plumbing, not business logic — consistent with the "thin Cobra wiring" constraint (no policy/catalog logic added to cmd/).
2. **`audit tail` returns a friendly error when the log is absent** rather than streaming an empty file, so a user who runs `tail` before any `check` gets actionable guidance. No scope change.
3. **`TestAuditRecordJSONKeys` also asserts the key count is exactly 11**, catching accidental extra/renamed fields — a strict superset of the plan's "all 11 present" requirement.

No Phase 6 features leaked in: no rotation, no syslog/OTLP/HTTPS sinks, no `audit query`, no `audit export`. Only the minimal Phase 1 writer + `audit tail`.
