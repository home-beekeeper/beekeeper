# Phase 1 / Plan 06 — Fail-Closed Hook Handler (Capstone) — Summary

**Plan:** `01-PLAN-hook-handler.md`
**Executed:** 2026-05-26
**Status:** Complete — all acceptance criteria met. `internal/config` (6 tests) and `internal/check` (12 tests + benchmark + selftest) pass; full suite (`go test ./...`) green; `go build ./...` and `go vet ./...` clean; `beekeeper selftest` prints `PASS: 7, FAIL: 0` and exits 0; `BenchmarkCheck` runs.
**Approach:** Task 1 config (test-alongside, immediately GREEN). Task 2 fail-closed handler with an injectable `catalogOpener` so the panic path is testable without a real index, wired into the `check` subcommand. Task 3 embedded adversarial corpus via `//go:embed`, wired into the `selftest` subcommand.

## What Was Built

This is the Phase 1 capstone: a developer can now route agent tool calls through `beekeeper check` and get an allow (exit 0) / block (exit non-zero) decision before any action takes effect, with a guarantee that any failure blocks rather than silently allows.

### Files created/modified

- `internal/config/config.go` (new) — `Config{FailMode}`, `Load`, `FailClosed`, mode constants
- `internal/config/config_test.go` (new) — 6 tests (missing→closed, open, warn, invalid→error, empty→closed, malformed→error)
- `internal/check/handler.go` (new) — `RunCheck`, `Result`, fail-closed core with hard caps
- `internal/check/handler_test.go` (new) — 12 tests covering allow / warn / every fail-closed path / fail-open / audit-on-every-path
- `internal/check/handler_bench_test.go` (new) — `BenchmarkCheck` against a ~200-entry index
- `internal/check/selftest.go` (new) — `RunSelftest`, embedded corpus runner
- `internal/check/selftest_test.go` (new) — `TestSelftestAllFixturesPass`
- `internal/check/corpus/fixtures.json` (new) — embedded adversarial corpus (6 evaluate fixtures)
- `cmd/beekeeper/main.go` (modified) — wired `check` and `selftest` subcommands (replaced both "not yet implemented" stubs); added `check`/`config` imports

## Key Interfaces Created

```go
// internal/config/config.go
type Config struct {
    FailMode string `json:"fail_mode"` // "closed" (default) | "open" | "warn"
}
func Load(path string) (Config, error) // missing file => Config{FailMode:"closed"}, nil
func (c Config) FailClosed() bool       // true unless FailMode is "open" or "warn"
const FailModeClosed = "closed"; FailModeOpen = "open"; FailModeWarn = "warn"

// internal/check/handler.go
type Result struct {
    Decision policy.Decision
    ExitCode int
}
// RunCheck reads stdin (1MB cap), evaluates under a 5s deadline + 256MB soft mem
// cap against the mmap index, writes an audit record on EVERY path, prints the
// Decision JSON to stdout, and returns a Result. NEVER a silent allow on failure.
func RunCheck(ctx context.Context, stdin io.Reader, cfg config.Config, indexPath, auditPath string) Result

// internal/check/selftest.go
func RunSelftest() (passed, failed int, err error) // err only on setup failure, not fixture mismatch
```

## RunCheck — fail-closed structure (HOOK-01/02/03/04)

1. `debug.SetMemoryLimit(256<<20)` — HOOK-04 256MB soft memory cap.
2. Top-level `defer recover()` → block (`failDecision`), honoring configured fail mode. Still writes a best-effort audit record on panic.
3. `context.WithTimeout(ctx, 5s)` — HOOK-04 execution cap; deadline re-checked after decode, after index open, and after evaluate.
4. `io.LimitedReader{N: 1MB+1}` — HOOK-04 1MB stdin cap; truncation detected by the extra cap byte (`limited.N <= 0`) for both malformed and valid-but-oversized inputs → block "stdin exceeds 1MB cap (fail-closed)".
5. `json.Decoder.Decode` failure → block "invalid tool call JSON (fail-closed)".
6. `catalog.OpenIndex(indexPath)` (HOOK-02, mmap, never cold JSON) failure → block "catalog index unavailable (fail-closed)"; closed via `io.Closer`.
7. `policy.Evaluate(toolCall, idx)` — pure synchronous call (`*catalog.Index` satisfies `policy.CatalogLookup`).
8. Exit mapping: `Allow==true` → 0 (covers Phase 1 single-source **warn**, which does NOT block — PLCY-01 deferred to Phase 2); `Allow==false` → 1. Fail-mode overrides apply ONLY to the failure paths above, never to a successful evaluate.
9. `finalize` is the single chokepoint: it audits (`audit.NewWriter` + `audit.FromDecision` with a crypto/rand 128-bit hex recordID + RFC3339 UTC timestamp), then prints the Decision JSON to stdout. Audit-write failure logs to stderr and NEVER downgrades a block to allow.

### Fail-mode semantics (`failDecision`)

| FailMode | On failure | Allow | Level |
|----------|-----------|-------|-------|
| `closed` (default) | block | false | block |
| `open` | allow (reduced security) | true | allow |
| `warn` | allow (reduced security) | true | warn |

`fail_open`/`fail_warn` reasons are suffixed `[fail_open: reduced security]`. The `Config.FailMode` doc comment states "open reduces security: failures allow instead of block".

## selftest fixtures (embedded via `//go:embed corpus/fixtures.json`)

Evaluated against a hermetic in-memory index (`catalog.BuildIndex` of two known entries: Nx Console `editor-extension:nrwl.angular-console@18.95.0` and `npm:shai-hulud@1.0.0`):

**Warn cases (catalog match, Level "warn", Allow true):**
1. direct shape `editor-extension:nrwl.angular-console@18.95.0`
2. direct shape `npm:shai-hulud@1.0.0`
3. command `npm install shai-hulud@1.0.0`

**Allow cases (Level "allow"):**
4. command `npm install express@4.18.2` (clean)
5. direct shape `editor-extension:nrwl.angular-console@18.100.0` (remediated version, not in catalog)
6. command `go get github.com/spf13/cobra@v1.10.2` (clean)

**Fail-closed case (routed through full `RunCheck`, not just `Evaluate`):**
7. malformed JSON `{bad json}` → block.

Total: 6 evaluate fixtures + 1 RunCheck fail-closed case = **7 PASS**.

> Note: the npm-command warn fixture targets `shai-hulud` (npm), not `nrwl.angular-console`. An `npm install` command resolves to ecosystem `npm`, whereas the Nx Console entry lives under `editor-extension`; matching it requires the direct-shape `ecosystem` field. The corpus reflects this ecosystem-keyed matching reality.

## Benchmark result (HOOK-01 latency)

`go test ./internal/check/... -run X -bench BenchmarkCheck -benchtime=10x`:

```
BenchmarkCheck-2   10   3575550 ns/op   (~3.58 ms/op)
```

Measured on an Intel Celeron N4020 @ 1.10GHz (low-end dev machine), full `RunCheck` path (stdin decode → mmap open → evaluate → audit write → stdout) against a 200-entry index. ~3.58ms is well under the sub-100ms p95 target (this excludes OS process cold-start, which the real subprocess invocation adds; cold-start is the dominant term per RESEARCH Pitfall 2 and is measured in CI on all three platforms).

## End-to-end verification (against the live synced Bumblebee catalog)

- Clean package `npm install express@4.18.2` → `{"Allow":true,"Level":"allow","Reason":"no catalog match"}`, exit 0.
- Real compromise `editor-extension:nrwl.angular-console@18.95.0` → `{"Allow":true,"Level":"warn","Reason":"bumblebee catalog match: stepsecurity-2026-05-18-vscode-nrwl-angular-console-compromised", CatalogMatches:[…critical…]}`, exit 0 (Phase 1 single-source warn).

## Acceptance Criteria Met

Task 1 (config):
- [x] `go test ./internal/config/... -count=1` exits 0
- [x] `Load` on a non-existent path → `Config{FailMode:"closed"}`, nil error
- [x] `Load` on invalid fail_mode → non-nil error
- [x] default is "closed"; doc comment notes "open" reduces security

Task 2 (handler):
- [x] `go test ./internal/check/... -count=1` exits 0; `go build ./...` clean
- [x] `TestFailClosedOnPanic`, `TestTimeoutFailClosed`, `TestStdinCapEnforced`, `TestMalformedJSONFailsClosed`, `TestMissingIndexFailsClosed` all assert block (Allow false, non-zero ExitCode)
- [x] handler uses `debug.SetMemoryLimit(256<<20)`, `io.LimitedReader` (1MB), `context.WithTimeout(5s)`, `catalog.OpenIndex`, and a top-level `recover()`
- [x] clean package → exit 0; Phase-1 catalog match → Level "warn", Allow true, CatalogMatch present
- [x] `TestFailOpenModeAllowsOnFailure` proves fail_open opts out of block (reduced security)
- [x] `BenchmarkCheck` exists and runs (`-bench BenchmarkCheck -benchtime=10x` exits 0)
- [x] audit record on every path (`audit.NewWriter`/`audit.FromDecision` in `finalize`); never downgrades a block on audit error (`TestAuditRecordWrittenOnEveryPath`, `TestMalformedJSONStillAudits`)
- [x] `check` subcommand calls `check.RunCheck` + `os.Exit(result.ExitCode)`; no "not yet implemented"

Task 3 (selftest):
- [x] `go test ./internal/check/... -run TestSelftest -count=1` exits 0 with `failed == 0`
- [x] `selftest.go` uses `//go:embed` (no runtime file dependency)
- [x] corpus has 3 warn + 3 allow + 1 malformed-JSON fail-closed case
- [x] `go run ./cmd/beekeeper selftest` prints `PASS: 7, FAIL: 0` and exits 0; exits non-zero if any fixture fails

Cross-cutting:
- [x] `go test ./internal/config/... ./internal/check/... -count=1` exits 0
- [x] `go test ./...` green (all packages)
- [x] `go run ./cmd/beekeeper selftest` exits 0

## Requirements Satisfied

- **HOOK-01** — `beekeeper check` reads stdin, evaluates via the mmap index, audits, and exits 0/non-zero with a structured JSON reason; benchmark in place toward sub-100ms p95.
- **HOOK-02** — catalog loaded via `catalog.OpenIndex` (mmap), never a cold JSON parse per invocation.
- **HOOK-03** — fails closed on crash/timeout/malformed/missing-index; `fail_open`/`fail_warn` are validated opt-ins defaulting to closed.
- **HOOK-04** — hard caps enforced: 1MB stdin (`io.LimitedReader`), 5s execution (`context.WithTimeout`), 256MB soft memory (`debug.SetMemoryLimit`).

## Threat Mitigations Implemented (from plan threat model)

- **T-06-01 (EoP — crash/timeout/error → silent allow)** — top-level `recover()` → block; deadline checks after decode/open/evaluate → block; all error paths return block; default fail mode "closed". Tests: `TestFailClosedOnPanic`, `TestTimeoutFailClosed`, `TestMissingIndexFailsClosed`.
- **T-06-02 (DoS — oversized stdin → OOM)** — `io.LimitedReader(1MB+1)` + truncation detection → block; 5s timeout; 256MB soft cap. Test: `TestStdinCapEnforced`.
- **T-06-03 (Tampering — malicious JSON)** — stdlib `encoding/json` decode; malformed → fail-closed block. Test: `TestMalformedJSONFailsClosed`.
- **T-06-04 (Tampering — corrupt/substituted index)** — `OpenIndex` validates magic/version (Plan 02); error → fail-closed block. Test: `TestMissingIndexFailsClosed`.
- **T-06-05 (Repudiation — decision not recorded)** — `finalize` writes one NDJSON record on every path (incl. pre-decode best-effort with empty agent/tool); audit-write failure never downgrades a block. Tests: `TestAuditRecordWrittenOnEveryPath`, `TestMalformedJSONStillAudits`.
- **T-06-06 (EoP — unknowingly running fail_open)** — `fail_mode "open"` requires explicit config, is documented as reducing security, defaults to closed, and is reflected in the decision reason. Test: `TestFailOpenModeAllowsOnFailure`.

## Deviations from the Plan

1. **Injectable `catalogOpener`.** `RunCheck` delegates to an unexported `runCheck(..., open catalogOpener)`; the default wraps `catalog.OpenIndex` (returning `*catalog.Index`, which satisfies both `policy.CatalogLookup` and `io.Closer`). Tests inject a panicking opener to exercise the top-level recover path deterministically without a corrupt-index fixture. No public-API change.
2. **`testdata/fixtures/oversized.json` not created.** Per the plan's own guidance ("prefer generating oversized input in the test via `bytes.Repeat` rather than committing a 1MB file"), `TestStdinCapEnforced` synthesizes a >1MB valid-JSON payload in-memory. No large binary committed.
3. **Embedded corpus lives in `internal/check/corpus/` (not `testdata/`).** `//go:embed` is excluded from `testdata` semantics confusion; `corpus/` is an ordinary embeddable directory and makes the binary-embedded nature explicit.
4. **`-race` is a CI-only gate here.** The Windows dev machine has no C compiler (gcc/clang), which `go test -race` (CGO) requires. Per CLAUDE.md ("cross-platform validated in CI only"), the full suite was run locally without `-race`; the `-race` matrix runs in CI (ubuntu/macos/windows). Local full suite (`go test ./...`) is green.
5. **Extra tests beyond the named set** — `TestLoadWarnMode`, `TestLoadMalformedJSONErrors`, `TestAuditRecordWrittenOnEveryPath`, `TestMalformedJSONStillAudits`, `TestNormalEvaluationWithinDeadline` — strengthen coverage of the audit-on-every-path and fail-mode contracts.

No out-of-scope features leaked in: no `beekeeper hooks install` (Phase 4), no full audit sinks (Phase 6), no `beekeeper protect` (Phase 5), no corroboration-based blocking (Phase 2).
