---
phase: 01-foundation-hook-handler
plan: 06
type: execute
wave: 5
depends_on: [02, 04, 05]
files_modified:
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/check/handler.go
  - internal/check/handler_test.go
  - internal/check/handler_bench_test.go
  - internal/check/selftest.go
  - internal/check/selftest_test.go
  - cmd/beekeeper/main.go
  - testdata/fixtures/oversized.json
autonomous: true
requirements: [HOOK-01, HOOK-02, HOOK-03, HOOK-04]
must_haves:
  truths:
    - "beekeeper check reads tool call JSON from stdin and exits 0 (allow) or non-zero (block) with a structured reason"
    - "A crash, timeout, malformed input, or missing catalog index results in a block, never a silent allow"
    - "The handler loads the catalog via mmap (not cold JSON parse) and enforces a 1MB stdin cap and 5s execution deadline"
    - "fail_open and fail_warn are configurable opt-ins; fail_closed is the default"
    - "Every check writes one NDJSON audit record including fail-closed decisions"
    - "beekeeper selftest runs embedded allow/block/fail-closed fixtures and exits 0 when all pass"
    - "check latency is benchmarked against a realistic catalog toward the sub-100ms p95 target"
  artifacts:
    - path: "internal/check/handler.go"
      provides: "Fail-closed hook handler entry point"
      exports: ["RunCheck"]
    - path: "internal/config/config.go"
      provides: "User-level config with fail mode"
      exports: ["Load", "Config"]
    - path: "internal/check/selftest.go"
      provides: "Embedded adversarial-corpus fixture runner"
      exports: ["RunSelftest"]
  key_links:
    - from: "internal/check/handler.go"
      to: "internal/policy.Evaluate"
      via: "synchronous pure call"
      pattern: "policy\\.Evaluate"
    - from: "internal/check/handler.go"
      to: "internal/catalog.OpenIndex"
      via: "mmap RDONLY load per invocation"
      pattern: "OpenIndex"
    - from: "internal/check/handler.go"
      to: "internal/audit.Writer"
      via: "writes one record per decision including fail-closed"
      pattern: "audit\\."
    - from: "cmd/beekeeper/main.go"
      to: "internal/check.RunCheck"
      via: "check subcommand RunE with fail-closed exit mapping"
      pattern: "check\\.RunCheck"
---

<objective>
Implement the Phase 1 capstone: the `beekeeper check` hook handler. Read a tool call from stdin (1MB cap), load the catalog mmap index, call the pure policy engine under a 5s deadline, write an NDJSON audit record, and exit 0 (allow) or non-zero (block) with a structured reason — failing CLOSED on any crash, timeout, malformed input, or missing index. Also deliver the layered config (fail mode) and `beekeeper selftest`.

Purpose: This is the whole point of Phase 1 — a developer can protect their machine by routing agent tool calls through `beekeeper check` and getting allow/block decisions before any action takes effect, with a guarantee that failures block rather than silently allow.
Output: `internal/config`, `internal/check` (handler, selftest, benchmark), the fully wired `check` and `selftest` subcommands. Wave 5 — depends on catalog (02), policy (04), and audit (05).
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
@.planning/phases/01-foundation-hook-handler/01-02-SUMMARY.md
@.planning/phases/01-foundation-hook-handler/01-04-SUMMARY.md
@.planning/phases/01-foundation-hook-handler/01-05-SUMMARY.md

<interfaces>
<!-- From plan 01: platform.StateDir/CatalogDir/AuditDir/ConfigPath -->
<!-- From plan 02: -->
```go
// internal/catalog
func OpenIndex(path string) (*Index, error)
func (idx *Index) Lookup(ecosystem, pkg string) (Entry, bool)
func (idx *Index) Close() error
```
<!-- From plan 04: -->
```go
// internal/policy
func Evaluate(tc ToolCall, idx CatalogLookup) Decision
type ToolCall struct { AgentName, ToolName string; ToolInput map[string]any }
type Decision struct { Allow bool; Level, Reason string; RuleIDs []string; CatalogMatches []CatalogMatch }
```
<!-- From plan 05: -->
```go
// internal/audit
func NewWriter(path string) (*Writer, error)
func (w *Writer) Write(rec AuditRecord) error
func FromDecision(tc policy.ToolCall, d policy.Decision, recordID, timestamp string) AuditRecord
```

<!-- Contracts this plan CREATES: -->
```go
// internal/config/config.go
type Config struct {
    FailMode string `json:"fail_mode"` // "closed" (default) | "open" | "warn"
}
func Load(path string) (Config, error) // missing file => defaults (FailMode "closed"); never errors on absence

// internal/check/handler.go
type Result struct { Decision policy.Decision; ExitCode int }
// RunCheck reads stdin, evaluates, audits, and returns a Result. NEVER returns a silent allow on failure.
func RunCheck(ctx context.Context, stdin io.Reader, cfg config.Config, indexPath, auditPath string) Result

// internal/check/selftest.go
func RunSelftest() (passed int, failed int, err error)
```
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Layered config loader (fail mode)</name>
  <read_first>
    - .planning/phases/01-foundation-hook-handler/01-CONTEXT.md (Fail-Closed Architecture, fail_open/fail_warn opt-ins, config schema Claude's Discretion)
    - CLAUDE.md (Fail closed by default; fail_open documented as reducing security)
    - internal/platform/dirs.go (ConfigPath from plan 01)
  </read_first>
  <files>internal/config/config.go, internal/config/config_test.go</files>
  <action>
    Create internal/config/config.go (package `config`) with the `Config` struct (FailMode string, json tag "fail_mode"). Phase 1 is user-level config only (full layered system→user→project→env→flag merge is CODE-05, Phase 9, out of scope). `Load(path string) (Config, error)`: if the file does not exist, return `Config{FailMode: "closed"}` with nil error (absence is normal, defaults apply). If it exists, read + json.Unmarshal; if FailMode is empty after unmarshal, default it to "closed"; validate FailMode is one of "closed"/"open"/"warn" and return an error otherwise. Add a doc comment on the FailMode field stating that "open" reduces security (CLAUDE.md). Provide a helper `(c Config) FailClosed() bool` returning true unless FailMode is "open" or "warn".

    Write config_test.go: TestLoadMissingFileDefaultsClosed, TestLoadOpenMode, TestLoadInvalidModeErrors, TestEmptyModeDefaultsClosed.
  </action>
  <verify>
    <automated>go test ./internal/config/... -count=1 2>&1</automated>
  </verify>
  <acceptance_criteria>
    - `go test ./internal/config/... -count=1` exits 0
    - `Load` on a non-existent path returns `Config{FailMode:"closed"}` and a nil error
    - `Load` on a config with an invalid fail_mode returns a non-nil error
    - The default fail mode is "closed" (fail-closed by default, CLAUDE.md)
    - A doc comment notes that fail_mode "open" reduces security
  </acceptance_criteria>
  <done>User-level config loads a fail mode defaulting to closed, with open/warn as validated opt-ins.</done>
</task>

<task type="auto">
  <name>Task 2: Fail-closed hook handler with stdin/timeout caps + check subcommand</name>
  <read_first>
    - .planning/phases/01-foundation-hook-handler/01-RESEARCH.md (Pattern 1 Fail-Closed Hook Handler, anti-pattern fail_open default, HOOK-01/02/03/04 rows, Security Domain STRIDE table)
    - internal/policy/engine.go + internal/catalog/index.go + internal/audit/writer.go (consumed contracts)
    - CLAUDE.md (Fail closed by default; mmap load; hard caps)
    - cmd/beekeeper/main.go (check stub from plan 01)
  </read_first>
  <files>internal/check/handler.go, internal/check/handler_test.go, internal/check/handler_bench_test.go, cmd/beekeeper/main.go, testdata/fixtures/oversized.json</files>
  <action>
    Create internal/check/handler.go (package `check`) implementing `RunCheck(ctx, stdin, cfg, indexPath, auditPath) Result` per RESEARCH Pattern 1. Structure:
    - A top-level `defer recover()` that, on any panic, sets the Result to a block decision (Decision{Allow:false, Level:"block", Reason:"internal error (fail-closed)"}, ExitCode non-zero) — UNLESS cfg says fail-open/warn (then map per fail mode), so failures still honor the configured fail mode but default to block.
    - Wrap stdin in `io.LimitReader(stdin, 1<<20)` (1MB, HOOK-04). After decode, detect truncation: if the limited reader hit exactly the cap with more data pending, treat as oversized → block (read one extra byte test, or compare bytes read to the limit). On oversized → block with reason "stdin exceeds 1MB cap (fail-closed)".
    - `ctx, cancel := context.WithTimeout(ctx, 5*time.Second)` (HOOK-04 execution cap); check ctx.Err() before emitting the decision; on deadline exceeded → block "execution timeout (fail-closed)".
    - json.Decode the tool call into policy.ToolCall; on decode error → block "invalid tool call JSON (fail-closed)".
    - `catalog.OpenIndex(indexPath)` (mmap RDONLY, HOOK-02 — never cold JSON parse); on error (missing/corrupt index) → block "catalog index unavailable (fail-closed)". defer idx.Close().
    - `policy.Evaluate(toolCall, idx)` — pure synchronous call.
    - Map Decision to exit code: in Phase 1 a "warn" decision keeps Allow=true → exit 0 (single-source warn does not block yet, PLCY-01 Phase 2); "block" → non-zero; "allow" → 0. Apply fail-mode overrides ONLY to failure-path decisions, never to a successful evaluate.
    - Write the audit record: open `audit.NewWriter(auditPath)`, build `audit.FromDecision(toolCall, decision, recordID, time.Now().UTC().Format(time.RFC3339))` (generate recordID, e.g. crypto/rand hex or a UUID-shaped string), and Write it. Audit-write failures must NOT downgrade a block to allow (log to stderr, keep the decision). EVERY path including fail-closed paths writes an audit record where a tool call was decoded; for pre-decode failures, write a best-effort record with empty agent/tool.
    - Print the structured Decision as JSON to stdout before returning.

    Wire the `check` subcommand RunE in main.go: resolve indexPath (`platform.CatalogDir()`+"bumblebee.idx") and auditPath (`platform.AuditDir()`+"beekeeper.ndjson"), `config.Load(platform.ConfigPath())`, call `check.RunCheck(cmd.Context(), os.Stdin, cfg, indexPath, auditPath)`, and `os.Exit(result.ExitCode)`. Replace the not-implemented stub.

    Create testdata/fixtures/oversized.json conceptually (a generator or a >1MB file) — prefer generating oversized input in the test via bytes.Repeat rather than committing a 1MB file; create a small placeholder note file only if needed.

    Write handler_test.go: TestHookHandlerAllow (clean package fixture → ExitCode 0), TestHookHandlerBlockOnCatalogMatch — NOTE: in Phase 1 a catalog match is warn→allow(exit 0); instead assert the decision Level is "warn" and a CatalogMatch is present (rename to TestCatalogMatchWarns), TestFailClosedOnPanic (inject a panic via a fake that panics → ExitCode non-zero, Allow false), TestTimeoutFailClosed (pass an already-cancelled/expired context → block), TestStdinCapEnforced (feed >1MB → block), TestMalformedJSONFailsClosed (garbage stdin → block), TestMissingIndexFailsClosed (non-existent indexPath → block), TestFailOpenModeAllowsOnFailure (cfg FailMode "open" + missing index → Allow true, documents the reduced-security path). Build a tiny real index in a temp dir for the allow/warn cases using catalog.BuildIndex.

    Write handler_bench_test.go: BenchmarkCheck builds a realistic index (use the testdata catalog fixtures or synthesize ~200 entries) and benchmarks RunCheck on a single tool call, so latency toward the sub-100ms p95 target (HOOK-01) is measurable in CI.
  </action>
  <verify>
    <automated>go test ./internal/check/... -count=1 2>&1 && go build ./... 2>&1</automated>
  </verify>
  <acceptance_criteria>
    - `go test ./internal/check/... -count=1` exits 0
    - TestFailClosedOnPanic, TestTimeoutFailClosed, TestStdinCapEnforced, TestMalformedJSONFailsClosed, TestMissingIndexFailsClosed all assert a block (Allow false, non-zero ExitCode)
    - `internal/check/handler.go` uses `io.LimitReader(stdin, 1<<20)`, `context.WithTimeout(ctx, 5*time.Second)`, and `catalog.OpenIndex` (HOOK-02/04) and contains a top-level `recover()`
    - A clean package tool call yields ExitCode 0; a Phase-1 catalog match yields Level "warn" with Allow true (single-source warn, PLCY-01 deferred)
    - TestFailOpenModeAllowsOnFailure proves fail_open opts out of block on failure (reduced-security path)
    - `BenchmarkCheck` exists and runs (`go test ./internal/check/... -run X -bench BenchmarkCheck -benchtime=10x` exits 0)
    - The handler writes an audit record on every decision path (grep confirms `audit.NewWriter` and `audit.FromDecision` in handler.go) and never downgrades a block to allow on audit-write error
    - `beekeeper check` subcommand calls `check.RunCheck` and `os.Exit(result.ExitCode)` (no "not yet implemented")
  </acceptance_criteria>
  <done>`beekeeper check` evaluates stdin tool calls via the mmap index under hard caps, fails closed on every failure mode, audits every decision, and exits 0/non-zero with a structured reason.</done>
</task>

<task type="auto">
  <name>Task 3: Embedded adversarial-corpus selftest + selftest subcommand</name>
  <read_first>
    - .planning/phases/01-foundation-hook-handler/01-RESEARCH.md (Adversarial Corpus Phase 1 Regression Fixtures, beekeeper selftest design)
    - .planning/phases/01-foundation-hook-handler/01-CONTEXT.md (selftest design: 3-5 allow + 3-5 block cases, must pass on all three platforms)
    - internal/check/handler.go (RunCheck from Task 2)
    - cmd/beekeeper/main.go (selftest stub from plan 01)
  </read_first>
  <files>internal/check/selftest.go, internal/check/selftest_test.go, cmd/beekeeper/main.go</files>
  <action>
    Create internal/check/selftest.go with `RunSelftest() (passed, failed int, err error)`. Use Go `embed` to embed a set of fixture tool-call JSON blobs (the adversarial corpus from RESEARCH) directly in the binary — no runtime file dependency (CONTEXT: embedded, not separate files at runtime). For each fixture, build an in-memory/temp catalog index containing the known Bumblebee entries (Nx Console editor-extension, a mini-shai-hulud npm package), run the evaluation against the embedded fixtures, and compare the actual Decision against the expected outcome encoded alongside each fixture. Expected outcomes per RESEARCH adversarial corpus:
    - Block/warn cases (expect a catalog match, Level "warn" in Phase 1): editor-extension nrwl.angular-console@18.95.0; an exact npm package from mini-shai-hulud; direct editor-extension:nrwl.angular-console:18.95.0 shape.
    - Allow cases (expect Level "allow"): npm express@4.18.2; remediated editor-extension nrwl.angular-console@18.100.0; go get github.com/spf13/cobra@v1.10.2.
    - Fail-closed cases: malformed JSON → block; (oversized handled in handler tests).
    Count passes/failures; return failed>0 as a non-nil error or let the caller decide exit code.

    Wire the `selftest` subcommand RunE: call `check.RunSelftest()`, print "PASS: N, FAIL: M", and exit non-zero if M>0 or err!=nil. Replace the not-implemented stub.

    Write selftest_test.go: TestSelftestAllFixturesPass asserts `failed == 0` and `err == nil` on the embedded corpus (this is the regression gate that must pass on all three CI platforms).
  </action>
  <verify>
    <automated>go test ./internal/check/... -run TestSelftest -count=1 2>&1 && go run ./cmd/beekeeper selftest 2>&1</automated>
  </verify>
  <acceptance_criteria>
    - `go test ./internal/check/... -run TestSelftest -count=1` exits 0 with `failed == 0`
    - `internal/check/selftest.go` uses `//go:embed` (fixtures embedded in the binary, no runtime file dependency)
    - The corpus includes at least 3 catalog-match (warn) cases and at least 3 allow cases plus a malformed-JSON fail-closed case (RESEARCH adversarial corpus)
    - `go run ./cmd/beekeeper selftest` prints a PASS/FAIL summary and exits 0 when all fixtures pass
    - The `selftest` subcommand exits non-zero if any fixture fails
  </acceptance_criteria>
  <done>`beekeeper selftest` runs the embedded May-2026 adversarial corpus and exits 0 only when every fixture produces its expected decision.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| agent → beekeeper check (stdin) | The tool call JSON on stdin is fully attacker-influenced (a hijacked agent controls it) |
| catalog index file → handler | The mmap'd index is read on every invocation; corruption must not crash into a silent allow |
| config file → fail mode | fail_open is a deliberate security-reducing opt-in |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-06-01 | Elevation of Privilege | Crash/timeout/error in check producing a silent allow | mitigate | Top-level `recover()` → block; context deadline → block; all error paths return block; default fail mode "closed"; tests TestFailClosedOnPanic/TestTimeoutFailClosed/TestMissingIndexFailsClosed (HOOK-03) |
| T-06-02 | Denial of Service | Oversized stdin causing OOM | mitigate | `io.LimitReader(stdin, 1<<20)` + oversized detection → block; 5s `context.WithTimeout` (HOOK-04); TestStdinCapEnforced |
| T-06-03 | Tampering | Malicious JSON crafted to exploit the decoder or evade evaluation | mitigate | stdlib `encoding/json` strict decode; malformed → fail-closed block (TestMalformedJSONFailsClosed); evaluation is the pure policy engine (plan 04) |
| T-06-04 | Tampering | Corrupt/substituted catalog index | mitigate | OpenIndex validates magic/version (plan 02) and errors → fail-closed block (TestMissingIndexFailsClosed) |
| T-06-05 | Repudiation | Decision not recorded | mitigate | Every decision path writes an NDJSON audit record (plan 05); audit-write failure never downgrades a block to allow |
| T-06-06 | Elevation of Privilege | Operator unknowingly running fail_open | mitigate | fail_mode "open" requires explicit config and is documented as reducing security; default is "closed"; TestFailOpenModeAllowsOnFailure makes the behavior explicit/tested |
</threat_model>

<verification>
- `go test ./internal/config/... ./internal/check/... -count=1` exits 0
- `go test -race -count=1 ./...` exits 0 (full suite, all packages from all plans)
- `go run ./cmd/beekeeper selftest` exits 0
- `BenchmarkCheck` runs and reports per-op latency toward the sub-100ms p95 target (HOOK-01)
- End-to-end: `beekeeper catalogs sync` then `echo '<toolcall>' | beekeeper check` returns the expected exit code and structured reason
</verification>

<success_criteria>
- `beekeeper check` reads stdin, evaluates via the mmap index, audits, and exits 0/non-zero with a structured reason (HOOK-01, HOOK-02)
- Fails closed on crash/timeout/malformed/missing-index; fail_open and fail_warn are validated opt-ins defaulting to closed (HOOK-03)
- Hard caps enforced: 1MB stdin, 5s execution (HOOK-04)
- `beekeeper selftest` runs the embedded adversarial corpus and gates regressions on all three platforms
- Latency benchmark in place toward the sub-100ms p95 target
</success_criteria>

<output>
After completion, create `.planning/phases/01-foundation-hook-handler/01-06-SUMMARY.md`
</output>
