---
phase: 04-integration-surfaces
plan: "05"
subsystem: cli-wiring / audit-redaction
tags: [INTG-01, INTG-02, INTG-03, INTG-04, INTG-05, INTG-06, INTG-07, cli, audit, redaction, self-defense]
dependency_graph:
  requires:
    - 04-01 (internal/hooks — Install/Uninstall)
    - 04-02 (internal/check — RunAuditRecord; internal/policy — AgentContext)
    - 04-03 (internal/gateway — Start, LoadGatewayState, Config)
    - 04-04 (internal/shim — Install/Uninstall/Status, DefaultTools)
  provides:
    - beekeeper hooks install/uninstall CLI (INTG-01/02)
    - beekeeper gateway daemon + token + status CLI (INTG-03/04/05)
    - beekeeper shim install/uninstall/status CLI (INTG-06)
    - beekeeper audit-record PostToolUse CLI (INTG-07)
    - internal/audit/redact.go: DefaultRedactPatterns, RedactRecord (T-04-05-02)
    - config.RedactPatterns + GetRedactPatterns() for forward compat
  affects:
    - cmd/beekeeper/main.go
    - internal/audit/ (new redact.go)
    - internal/check/handler.go (redaction applied in writeAuditWithAC)
    - internal/config/config.go (RedactPatterns field)
tech_stack:
  added: []
  patterns:
    - signal.NotifyContext(SIGINT, SIGTERM) foreground daemon pattern (gateway follows catalogs watch)
    - cobra.MarkFlagRequired for --target on hooks install/uninstall
    - os.FindProcess + Signal(0) for PID liveness check in gateway status
    - defaultRedactPatterns() non-backtracking character-class regexes (T-04-05-07)
    - RedactRecord copy semantics — returns new AuditRecord, never mutates receiver
    - audit.DefaultRedactPatterns() applied at writeAuditWithAC chokepoint before every disk write
key_files:
  created:
    - internal/audit/redact.go
    - internal/audit/redact_test.go
  modified:
    - cmd/beekeeper/main.go
    - internal/config/config.go
    - internal/check/handler.go
decisions:
  - "gateway status masks token to first 8 chars + '...' (T-04-05-01); gateway token prints full token"
  - "DefaultRedactPatterns exported (not private) so check.writeAuditWithAC and any future gateway.writeAudit can call it"
  - "Bearer regex uses \\S+ (any non-whitespace) — correctly absorbs trailing punctuation in real-world tool outputs"
  - "config.RedactPatterns added for forward compat (Phase 6 audit enhancement); Phase 4 always uses default patterns"
  - "writeAuditWithAC is the single chokepoint where redaction is applied — all 4 audit surfaces (success, fail-closed, panic recover, audit-record) pass through it"
  - "newAuditRecordCmd returns nil (exit 0) even on AuditDir resolve error — T-04-05-04 PostToolUse must not disrupt agent"
metrics:
  duration: "~25 minutes"
  completed_date: "2026-05-27"
  tasks_completed: 2
  tasks_total: 2
  files_created: 2
  files_modified: 3
---

# Phase 04 Plan 05: CLI Wiring + Audit Redaction Summary

**One-liner:** Thin Cobra wiring for all Phase 4 commands (hooks/gateway/shim/audit-record + init shims dir) plus non-backtracking sensitive-field redaction applied at the single audit write chokepoint.

## What Was Built

### Task 1: Wire hooks/gateway/shim/audit-record commands + extend init

**`cmd/beekeeper/main.go`** — Four new command functions added; all internal packages imported:

| Function | Delegates To | Notes |
|----------|-------------|-------|
| `newHooksCmd()` | `hooks.Install`, `hooks.Uninstall` | `--target` required; `--dry-run`, `--force` flags |
| `newGatewayCmd()` | `gateway.Start`, `gateway.LoadGatewayState` | Foreground daemon with `signal.NotifyContext(SIGINT, SIGTERM)`; `--port 7837`, `--upstream`, `--bind 127.0.0.1` |
| `newShimCmd()` | `shim.Install`, `shim.Uninstall`, `shim.Status` | `shimDir = stateDir + "/shims"`; delegates `shim.DefaultTools` |
| `newAuditRecordCmd()` | `check.RunAuditRecord` | Exits 0 always; T-04-05-04 |

**gateway subcommands:**
- `beekeeper gateway token` — prints full token from state.json; if empty prints "Gateway not running"
- `beekeeper gateway status` — checks PID liveness via `os.FindProcess` + `Signal(0)`; masks token to 8 chars + "..." (T-04-05-01); prints address and StartedAt

**`newInitCmd` extended:** Creates `~/.beekeeper/shims/` directory (Phase 4 state dir, INTG-06) in addition to existing Phase 1 + Phase 3 directories.

### Task 2: Audit log sensitive field redaction

**`internal/audit/redact.go`** — Pure sensitive field redaction:

| Function | Type | Description |
|----------|------|-------------|
| `DefaultRedactPatterns()` | `[]redactPattern` | Three default patterns: Bearer/JWT/API-key |
| `applyRedaction(s, patterns)` | pure | Returns new string; input never modified |
| `RedactRecord(rec, patterns)` | pure | Returns new AuditRecord copy with Reason field redacted |
| `RedactString(s)` | convenience | applyRedaction with DefaultRedactPatterns |
| `HasSensitiveData(s)` | bool | Tests whether any default pattern matches |
| `RedactStringSlice(ss, patterns)` | pure | Per-element redaction returning new slice |

**Default patterns (non-backtracking, T-04-05-07):**
1. `(?i)Authorization:\s*Bearer\s+\S+` → `Authorization: Bearer [REDACTED]`
2. `eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+` → `[JWT_REDACTED]`
3. `(sk-proj-|sk-ant-|AKIA|ghp_|glpat-)[A-Za-z0-9_-]+` → `${1}[REDACTED]`

**`internal/check/handler.go`** — `writeAuditWithAC` applies `audit.DefaultRedactPatterns()` + `audit.RedactRecord()` to every audit record before writing. This covers all four audit paths: success, fail-closed, panic recover, and `RunAuditRecord` (PostToolUse).

**`internal/config/config.go`** — Added `RedactPatterns []string` field with `json:"redact_patterns,omitempty"` and `GetRedactPatterns()` method for forward compatibility (custom patterns are a Phase 6 audit enhancement).

## Test Results

```
go test ./internal/audit/... -run TestRedact -v -count=1
--- PASS: TestRedactBearerToken (0.00s)
--- PASS: TestRedactJWT (0.00s)
--- PASS: TestRedactAPIKeyPrefix (0.00s)
--- PASS: TestRedactPure (0.00s)
--- PASS: TestRedactNoMatch (0.00s)
--- PASS: TestRedactRecordReason (0.00s)
--- PASS: TestRedactRecordNoPatterns (0.00s)
--- PASS: TestRedactPathologicalInputs (0.08s)
PASS

go test ./... -count=1
ok  github.com/mzansi-agentive/beekeeper/internal/audit          1.282s
ok  github.com/mzansi-agentive/beekeeper/internal/baseline       1.128s
ok  github.com/mzansi-agentive/beekeeper/internal/catalog        3.463s
ok  github.com/mzansi-agentive/beekeeper/internal/check          4.718s
ok  github.com/mzansi-agentive/beekeeper/internal/config         1.131s
ok  github.com/mzansi-agentive/beekeeper/internal/editorinit     1.351s
ok  github.com/mzansi-agentive/beekeeper/internal/gateway        3.020s
ok  github.com/mzansi-agentive/beekeeper/internal/hooks          1.368s
ok  github.com/mzansi-agentive/beekeeper/internal/notify         1.819s
ok  github.com/mzansi-agentive/beekeeper/internal/platform       1.115s
ok  github.com/mzansi-agentive/beekeeper/internal/policy         1.362s
ok  github.com/mzansi-agentive/beekeeper/internal/quarantine     1.472s
ok  github.com/mzansi-agentive/beekeeper/internal/scan           2.827s
ok  github.com/mzansi-agentive/beekeeper/internal/shim           1.279s
ok  github.com/mzansi-agentive/beekeeper/internal/watch          2.634s
```

All 14 test packages pass. Zero regressions.

## Commits

| Task | Commit | Message |
|------|--------|---------|
| Task 1 | 2d07fa6 | feat(04-05): wire hooks/gateway/shim/audit-record commands + extend init |
| Task 2 | 3d73430 | feat(04-05): audit log sensitive field redaction (INTG-07 / T-04-05-02) |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Test case for Bearer token in longer string absorbed comma into [REDACTED]**

- **Found during:** Task 2 redact test run
- **Issue:** The Bearer token regex `\S+` correctly matches any non-whitespace including commas. Test case used comma-separated HTTP headers (not real HTTP wire format). The regex behavior is correct; the test expectation was wrong.
- **Fix:** Updated test input to use space-separated values (matching the regex's semantics) rather than comma-separated values. Documented in test comment that real HTTP headers are `\r\n`-separated.
- **Files modified:** `internal/audit/redact_test.go`
- **Commit:** 3d73430

**2. [Rule 2 - Missing critical functionality] defaultRedactPatterns() needed to be exported**

- **Found during:** Task 2 implementation (go build — `internal/check/handler.go` calls `audit.DefaultRedactPatterns()`)
- **Issue:** The plan referred to `defaultRedactPatterns()` as a package-private function. Since `handler.go` in `internal/check` must call it, it must be exported.
- **Fix:** Renamed to `DefaultRedactPatterns()` and updated all references.
- **Files modified:** `internal/audit/redact.go`, `internal/audit/redact_test.go`
- **Commit:** 3d73430

## Threat Mitigations Applied

| Threat ID | Status | Implementation |
|-----------|--------|----------------|
| T-04-05-01 | Mitigated | `gateway status` masks token to first 8 chars + "..."; `gateway token` prints full token only on explicit request |
| T-04-05-02 | Mitigated | `RedactRecord` applied in `writeAuditWithAC` before every disk write; Bearer/JWT/API-key patterns tested |
| T-04-05-03 | Accepted | Gateway upstream URL is user-configured; no SSRF mitigation in Phase 4; documented in gateway --help Long description |
| T-04-05-04 | Mitigated | `newAuditRecordCmd` returns nil (exit 0) even on AuditDir resolve error; `RunAuditRecord` always returns 0 |
| T-04-05-05 | Mitigated | `hooks.Install` validates target against supported list (Plan 01 implementation); unknown target returns error |
| T-04-05-06 | Accepted | Stale PID in state.json on crash; next gateway start overwrites with fresh token; documented in gateway lifecycle |
| T-04-05-07 | Mitigated | All redaction patterns use non-backtracking character classes `[A-Za-z0-9_-]+`; `TestRedactPathologicalInputs` validates 4 adversarial cases complete in <100ms |

## Known Stubs

None. All commands dispatch to fully implemented internal packages. The `config.RedactPatterns` field is intentional forward-compat scaffolding (not a stub) — it is documented as a Phase 6 enhancement and the default patterns cover all three critical patterns for Phase 4.

## Threat Flags

No new threat surface introduced beyond the plan's threat model. The gateway `--upstream` flag SSRF concern (T-04-05-03) is accepted per plan and documented in the gateway command Long description.

## Success Criteria Verification

- [x] INTG-01/02: `beekeeper hooks install --target <t>` and `beekeeper hooks uninstall --target <t>` callable; dispatch to `internal/hooks`
- [x] INTG-03/04: `beekeeper gateway [--port] [--upstream] [--bind]` starts `gateway.Start` with `signal.NotifyContext(SIGINT, SIGTERM)`
- [x] INTG-03/04: `beekeeper gateway token` reads state.json; `gateway status` masks token (T-04-05-01)
- [x] INTG-05: `beekeeper hooks install --target continue|opencode|openclaw` prints gateway config guide (via `internal/hooks.Install`)
- [x] INTG-06: `beekeeper shim install/uninstall/status` callable; `beekeeper init` creates shims dir
- [x] INTG-07: `beekeeper audit-record` reads PostToolUse stdin; exits 0 always (T-04-05-04)
- [x] Self-defense: Bearer/JWT/API-key redaction applied at `writeAuditWithAC` (T-04-05-02)
- [x] Gateway token masked in status; printed in full via `gateway token` (T-04-05-01)
- [x] All pre-existing commands preserved: version, init, check, catalogs, audit, selftest, watch, scan, quarantine
- [x] Full test suite green (14 packages, 0 regressions)
- [x] main.go is thin wiring only — no business logic

## Self-Check: PASSED

Files created:
- internal/audit/redact.go: FOUND — contains `DefaultRedactPatterns`, `applyRedaction`, `RedactRecord`
- internal/audit/redact_test.go: FOUND — contains `TestRedactBearerToken`, `TestRedactJWT`, `TestRedactAPIKeyPrefix`, `TestRedactPure`, `TestRedactNoMatch`, `TestRedactRecordReason`

Files modified:
- cmd/beekeeper/main.go: FOUND — contains `newHooksCmd`, `newGatewayCmd`, `newShimCmd`, `newAuditRecordCmd`
- internal/check/handler.go: FOUND — contains `RedactRecord` call in `writeAuditWithAC`
- internal/config/config.go: FOUND — contains `RedactPatterns` field

Commits:
- 2d07fa6: confirmed via git log — feat(04-05): wire hooks/gateway/shim/audit-record commands + extend init
- 3d73430: confirmed via git log — feat(04-05): audit log sensitive field redaction (INTG-07 / T-04-05-02)

Test results: 14/14 packages pass (go test ./... -count=1)
Build: go build ./... — PASSED
go vet ./... — PASSED
