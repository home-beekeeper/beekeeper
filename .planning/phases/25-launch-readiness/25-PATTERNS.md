# Phase 25: Launch Readiness — Pattern Map

**Mapped:** 2026-06-14
**Files analyzed:** 5 (4 new/modified code files + 1 docs edit)
**Analogs found:** 5 / 5

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/corpus/launch_e2e_test.go` | test | CRUD + event-driven | `internal/corpus/schema_lock_test.go` + `internal/sentry/rules_test.go` | exact (same package, same four-layer assertion pattern) |
| `internal/check/handler_test.go` | test (modify) | request-response | existing `BenchmarkRunCheck` (lines 1462–1511) + `internal/check/latency_persist_test.go` | exact (same file, same `runCheck` call pattern) |
| `internal/corpus/store_test.go` | test (modify) | static/CRUD | `internal/sentry/imports_test.go` (`TestRulesImportsArePure`) | exact (same import-purity gate pattern with `go/parser`) |
| `cmd/beekeeper/catalogs_daemon_test.go` | test (modify) | CRUD + event-driven | the existing `TestRunCatalogsSyncFirstResponder` in the same file (lines 69–end) | exact (extend in-place) |
| `docs/THREAT-MODEL.md` | docs (modify) | — | existing §1–§12 sections in the same file | exact (same section structure) |

---

## Pattern Assignments

### `internal/corpus/launch_e2e_test.go` (NEW — LAUNCH-02)

**Package declaration:** `package corpus` (white-box — same package as emitter, store, types)

**Analog:** `internal/corpus/schema_lock_test.go` (lines 1–412) for four-layer assertion structure;
`internal/sentry/rules_test.go` (lines 1–98) for synthetic `SentryEvent` + `EvaluateEvent` call pattern.

**Imports pattern** — model on `schema_lock_test.go` lines 17–25 + `emitter_test.go` lines 3–11:
```go
package corpus

import (
    "strings"
    "testing"
    "time"

    "github.com/bantuson/beekeeper/internal/audit"
    "github.com/bantuson/beekeeper/internal/config"
    "github.com/bantuson/beekeeper/internal/sentry"
)
```
Note: `internal/sentry` does NOT import `internal/corpus`, so this import direction is safe (no cycle).

**Synthetic event construction** — copy helper call pattern from `rules_test.go` lines 60–98:
```go
// From internal/sentry/rules_test.go lines 60–97
tree := editorTree()   // cursor(pid=1) → child(pid=100)
state := sentry.NewRuleState()
now := time.Now().UTC()

ev := sentry.SentryEvent{
    Kind:     sentry.EventFileAccess,
    PID:      100,
    PPID:     1,
    Exe:      "/usr/bin/some-tool",
    FilePath: "/home/user/.ssh/id_rsa",
    WallTime: now,
}
alerts := sentry.EvaluateEvent(ev, state, tree, emptyInventory(), defaultCfg(), noBaseline(), now)
```
IMPORTANT: `editorTree`, `emptyInventory`, `freshInventory`, `defaultCfg`, `noBaseline`, `hasAlert`
are defined in `internal/sentry/rules_test.go` as package-internal helpers — they are NOT exported.
The LAUNCH-02 test is in `package corpus`, not `package sentry`, so it CANNOT call those helpers
directly. Instead it must construct equivalent `sentry.SentryEvent` values inline, or duplicate
the minimal helper logic in `launch_e2e_test.go`. Pattern to copy for the process tree:
```go
// Inline equivalent of editorTree() for use from package corpus:
tree := map[uint32]sentry.ProcessNode{
    1:   {PID: 1, PPID: 0, Exe: "/usr/bin/cursor"},
    100: {PID: 100, PPID: 1, Exe: "/usr/bin/some-tool"},
}
inv := sentry.InventorySnapshot{RecentExtensions: map[string]time.Time{}}
```

**MapToCorpusRecord call pattern** — from `internal/corpus/emitter_test.go` lines 134–148
and `internal/corpus/fuzz_test.go` line 82:
```go
// From emitter_test.go lines 135–138
rec := audit.AuditRecord{
    RecordType:       "policy_decision",
    RecordID:         "test-record-id",
    Timestamp:        "2026-06-14T00:00:00Z",
    ScannerName:      "beekeeper",
    ToolName:         alert.RuleID,          // action_type proxy for sentry surface
    Decision:         "alert",
    SourceSurface:    "sentry",
    SentryRuleID:     alert.RuleID,
    SentryRuleName:   alert.Severity,
    CorroborationCount: 1,
    RulesetVersion:   "1.0",
    ClusterID:        "launch-02-cluster",
    Endpoint:         "check",
    SourcesAgreed:    []string{},
    SourcesDissented: []string{},
}
corpusRec := MapToCorpusRecord(rec, config.CorpusConfig{Enabled: true}, "test-repo-fp", "test-node")
```

**Four-layer assertion pattern** — copy from `schema_lock_test.go` lines 208–285
(the `fieldCheck` table structure and loop):
```go
// From schema_lock_test.go lines 208–285
type fieldCheck struct {
    name    string
    value   string
    wantAny bool
    allow   string
    skip    bool
    skipMsg string
}
checks := []fieldCheck{
    // Behavior layer
    {name: "source_surface", value: corpusRec.AuditRecord.SourceSurface, allow: "sentry"},
    {name: "sentry_rule_id",  value: corpusRec.AuditRecord.SentryRuleID, wantAny: true},
    // Decision layer
    {name: "decision", value: corpusRec.AuditRecord.Decision, allow: "alert"},
    // Outcome layer
    {name: "true_label", value: corpusRec.TrueLabel, allow: "unresolved"},
    // Context layer
    {name: "scope", value: string(corpusRec.Scope), allow: "org_only"},
    {name: "corpus_schema_version", value: corpusRec.CorpusSchemaVersion, allow: CorpusSchemaVersion},
}
for _, c := range checks {
    if c.skip { t.Logf("skip: %s — %s", c.name, c.skipMsg); continue }
    if c.wantAny {
        if c.value == "" { t.Errorf("LAUNCH-02 gap: %s is empty", c.name) }
    } else {
        if c.value != c.allow { t.Errorf("LAUNCH-02: %s = %q; want %q", c.name, c.value, c.allow) }
    }
}
```

**Envelope assertion pattern** — from `schema_lock_test.go` lines 370–395:
```go
// From schema_lock_test.go lines 370–395
envChecks := []struct{ fragment, desc string }{
    {`"action_hint":"watch_and_block"`, "action_hint must be watch_and_block"},
    {`"confidence_tier"`,               "confidence_tier must be present"},
    {`"behavior_signature_hash"`,       "behavior_signature_hash must be present"},
}
envJSON, _ := json.Marshal(corpusRec.PushEnvelope)
for _, ec := range envChecks {
    if !strings.Contains(string(envJSON), ec.fragment) {
        t.Errorf("LAUNCH-02: %s — %q not in envelope JSON: %s", ec.desc, ec.fragment, string(envJSON))
    }
}
// BehaviorSignatureHash must be 64-char hex (SCHEMA-06 gate, schema_lock_test.go line 396)
if len(corpusRec.PushEnvelope.Signature.BehaviorSignatureHash) != 64 {
    t.Errorf("BehaviorSignatureHash: want 64-char hex; got %d chars",
        len(corpusRec.PushEnvelope.Signature.BehaviorSignatureHash))
}
```

**Table-driven test skeleton** — model on `rules_test.go` SENTRY-001..008 naming convention:
```go
// TestAllSentryPatternsProduceMoatRecord (LAUNCH-02) — table over 8 patterns.
func TestAllSentryPatternsProduceMoatRecord(t *testing.T) {
    type sentryPatternCase struct {
        name      string
        ruleID    string
        buildEvent func(state *sentry.RuleState, now time.Time) []sentry.SentryAlert
    }
    cases := []sentryPatternCase{
        {name: "SENTRY-001 cred-file-access", ruleID: "SENTRY-001", buildEvent: ...},
        // ... 7 more rows, one per pattern
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            // 1. Produce alert via EvaluateEvent
            // 2. Build AuditRecord from alert
            // 3. Call MapToCorpusRecord
            // 4. Assert four layers + envelope
        })
    }
}
```

---

### `internal/check/handler_test.go` (MODIFY — LAUNCH-03)

**Package:** `package check` (white-box — same package as `runCheck`, `BenchmarkRunCheck`)

**Analog A — benchmark pattern** (`handler_test.go` lines 1462–1511, `BenchmarkRunCheck`):
```go
// From handler_test.go lines 1482–1511
func BenchmarkRunCheck(b *testing.B) {
    dir := b.TempDir()
    idxPath := buildTestIndexB(b, dir)
    corpusPath := filepath.Join(dir, "corpus", "bench-corpus.ndjson")
    if err := os.MkdirAll(filepath.Dir(corpusPath), 0o700); err != nil {
        b.Fatalf("MkdirAll corpus dir: %v", err)
    }
    cfg := config.Config{
        FailMode: config.FailModeClosed,
        Corpus: config.CorpusConfig{
            Enabled: true,
            Path:    corpusPath,
        },
    }
    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        // Use ReadFile tool — NOT Bash npm-install — to avoid pnpm nudge subprocess
        stdin := strings.NewReader(`{"agent_name":"a","tool_name":"ReadFile","tool_input":{"path":"/bench/fixture/beekeeper-test-file.txt"}}`)
        runCheck(context.Background(), stdin, cfg, idxPath, filepath.Join(dir, "audit.ndjson"), dir, defaultOpener, io.Discard)
    }
}
```
Copy this cfg + ReadFile stdin pattern EXACTLY for `TestBenchmarkRunCheckGate`. The
ReadFile-not-Bash constraint is documented at line 1504 and is non-negotiable (Pitfall 3).

**Analog B — latency ring P99 assertion** (`internal/check/diag_test.go` lines 88–101
and `latency_persist_test.go` lines 98–108):
```go
// From latency_persist_test.go lines 98–108
// (LatencyTracker is the in-process ring type; llamafirewall package owns it)
for _, ms := range samples {
    lt.Record(ms)
}
p99 := lt.P99()
if p99 == 0 {
    t.Fatal("p99 == 0, want non-zero")
}
```
For `TestBenchmarkRunCheckGate`, the pattern is: call `runCheck` N times, record
elapsed per call via `time.Since`, then assert `elapsedNs/N < 100_000_000`. Use
`testing.Short()` to skip if `-short` flag is passed (CI fast path).

**Analog C — offline block assertion** (`handler_test.go` lines 49–66, `TestHookHandlerAllow`):
```go
// From handler_test.go lines 49–66 (TestHookHandlerAllow) — offline is the default state
dir := t.TempDir()
idxPath := buildTestIndex(t, dir)   // mmap index from disk, no network sources configured
stdin := strings.NewReader(`{...}`) // ReadFile or known-malicious package
res := RunCheck(context.Background(), stdin, closedConfig(), idxPath, auditPathIn(t), t.TempDir())
if res.Decision.Allow {
    t.Fatal("Allow = true offline, want false (fail-closed)")
}
```
For `TestOfflineProtective`: seed the index with a known-malicious entry (use `buildTestIndex`),
use a stdin with that package, assert `!res.Decision.Allow`. No network sources needed —
tests run offline by default (Pitfall notes in RESEARCH.md §LAUNCH-03).

**Existing helpers to reuse** (`handler_test.go` lines 22–47):
```go
// buildTestIndex — handler_test.go lines 22–40
func buildTestIndex(t *testing.T, dir string) string { ... }

// closedConfig — handler_test.go line 42
func closedConfig() config.Config { return config.Config{FailMode: config.FailModeClosed} }

// auditPathIn — handler_test.go lines 44–47
func auditPathIn(t *testing.T) string { return filepath.Join(t.TempDir(), "audit", "beekeeper.ndjson") }
```
All three helpers are already in the same file; the new tests call them directly.

---

### `internal/corpus/store_test.go` (MODIFY — LAUNCH-04 static import gate)

**Package:** `package corpus` (existing file, add one new test function)

**Analog:** `internal/sentry/imports_test.go` (full file, 47 lines) — the exact same
`go/parser` + `go/token` AST-based import-purity gate pattern:
```go
// From internal/sentry/imports_test.go lines 1–47 (full file)
package sentry

import (
    "go/parser"
    "go/token"
    "os"
    "testing"
)

func TestRulesImportsArePure(t *testing.T) {
    forbidden := map[string]bool{
        "os":       true,
        "net/http": true,
        "io":       true,
        "sync":     true,
        "context":  true,
    }
    for _, path := range []string{"rules.go", "types.go"} {
        src, err := os.ReadFile(path)
        if err != nil { t.Fatalf("reading %s: %v", path, err) }
        fset := token.NewFileSet()
        f, err := parser.ParseFile(fset, path, src, parser.ImportsOnly)
        if err != nil { t.Fatalf("parsing %s: %v", path, err) }
        for _, imp := range f.Imports {
            p := imp.Path.Value
            if len(p) >= 2 { p = p[1 : len(p)-1] } // strip surrounding quotes
            if forbidden[p] {
                t.Errorf("%s imports forbidden package %q — ...", path, p)
            }
        }
    }
}
```

**Adapt to corpus/store_test.go for `TestCorpusStoreHasNoNetworkImports`:**
- Change `package sentry` → `package corpus` (file is already `package corpus`)
- Change `path` list: `[]string{"store.go"}` only (the one file under test)
- Change `forbidden` set to match LAUNCH-04 requirement:
  ```go
  forbidden := map[string]bool{
      "net":      true,
      "net/http": true,
      "os/exec":  true,
  }
  ```
- Error message should name STORE-03 and the LAUNCH-04 no-exfil guarantee

**No new imports needed** — `go/parser`, `go/token`, `os` are all stdlib already
used by `imports_test.go`. The existing `store_test.go` already imports `os` (line 6).

---

### `cmd/beekeeper/catalogs_daemon_test.go` (MODIFY — LAUNCH-01)

**Analog:** the existing `TestRunCatalogsSyncFirstResponder` in the same file (lines 69–end).
This is an in-place extension: add assertions #8–11 after the existing 7-point gate.

**Existing 7-assertion gate structure** (lines 186–end) — the new 4 assertions follow
the same numbered-comment pattern:
```go
// From catalogs_daemon_test.go lines 186–199
// ==============================================================
// EVALUATOR GATE — 7 assertions (FRB-01..05)
// ==============================================================

// 1. corpus.ReadMaliciousRecords returns the seeded record (FRB-01 signal).
malicious, err := corpus.ReadMaliciousRecords(corpusPath)
if err != nil {
    t.Fatalf("[1] ReadMaliciousRecords error: %v", err)
}
found1 := false
for _, r := range malicious {
    if r.TrueLabel == "malicious" && r.PushEnvelope != nil &&
        r.PushEnvelope.Signature.PackageOrExtensionID == "npm:@nrwl/nx-console" {
        found1 = true
        // ... assertions continue
    }
}
```

**Seeded record shape** (lines 109–131) — the existing seed already has all four layers
populated with `TrueLabel = "malicious"`, `AdjudicationSource = "catalog_confirmation"`,
`ConfidenceTier = "enforce"`, `SourceCount = 2`, `ActionHint = corpus.ActionHintWatchAndBlock`.
Assertions #8–11 can operate on `malicious[0]` (or the `found1` record) directly.

**New assertions to add (pattern from RESEARCH.md §LAUNCH-01 checklist):**
```go
// 8. All four layers populated on the CorpusRecord (LAUNCH-01).
rec := malicious[0]
// Behavior layer
if rec.AuditRecord.SourceSurface == "" && rec.AuditRecord.ToolName == "" {
    t.Error("[8] behavior layer: both SourceSurface and ToolName are empty")
}
// Decision layer
if rec.AuditRecord.Decision == "" {
    t.Error("[8] decision layer: Decision is empty")
}
if rec.AuditRecord.CorroborationCount < 1 {
    t.Errorf("[8] decision layer: CorroborationCount = %d, want >= 1", rec.AuditRecord.CorroborationCount)
}
// Outcome layer
if rec.TrueLabel != "malicious" {
    t.Errorf("[8] outcome layer: TrueLabel = %q, want \"malicious\"", rec.TrueLabel)
}
if rec.AdjudicationSource == "" {
    t.Error("[8] outcome layer: AdjudicationSource is empty")
}
// Context layer
if string(rec.Scope) != "org_only" {
    t.Errorf("[8] context layer: Scope = %q, want \"org_only\"", string(rec.Scope))
}
if rec.CorpusSchemaVersion != "1.0" {
    t.Errorf("[8] context layer: CorpusSchemaVersion = %q, want \"1.0\"", rec.CorpusSchemaVersion)
}

// 9. Envelope: BehaviorSignatureHash is 64-char hex (LAUNCH-01).
if rec.PushEnvelope == nil {
    t.Fatal("[9] PushEnvelope is nil")
}
if len(rec.PushEnvelope.Signature.BehaviorSignatureHash) != 64 {
    t.Errorf("[9] BehaviorSignatureHash: got %d chars, want 64",
        len(rec.PushEnvelope.Signature.BehaviorSignatureHash))
}

// 10. Envelope: ConfidenceTier = "enforce", SourceCount = 2 (LAUNCH-01).
if rec.PushEnvelope.ConfidenceTier != "enforce" {
    t.Errorf("[10] ConfidenceTier = %q, want \"enforce\"", rec.PushEnvelope.ConfidenceTier)
}
if rec.PushEnvelope.SourceCount != 2 {
    t.Errorf("[10] SourceCount = %d, want 2", rec.PushEnvelope.SourceCount)
}

// 11. Envelope: ActionHint = ActionHintWatchAndBlock (LAUNCH-01).
if rec.PushEnvelope.ActionHint != corpus.ActionHintWatchAndBlock {
    t.Errorf("[11] ActionHint = %q, want ActionHintWatchAndBlock", rec.PushEnvelope.ActionHint)
}
```

**Comment block to update** — the existing `// EVALUATOR GATE — 7 assertions` comment
must be updated to `// EVALUATOR GATE — 11 assertions (FRB-01..05 + LAUNCH-01)`.

---

### `docs/THREAT-MODEL.md` (MODIFY — LAUNCH-04 docs)

**Analog:** existing `## 1.` through `## 12.` section headers in the same file.
The current document has 12 numbered sections; LAUNCH-04 adds `## 13`.

**Section structure to copy** — from the existing `## 8. Known Gaps` section pattern:
each section opens with a one-paragraph summary, then uses a bulleted or sub-headed list
for named items. For §13 the three gaps must use the EXACT strings from REQUIREMENTS.md:

```markdown
## 13. Adjudicated Corpus (Local Loop) — v1.4.0

The v1.4.0 corpus is local-first, append-only, and owner-only (0600 on Unix; DACL
on Windows). No corpus data leaves the machine in v1: `StoreSink.Write` has no
`net`, `net/http`, or `os/exec` import (verified by `TestCorpusStoreHasNoNetworkImports`
in `internal/corpus/store_test.go`). The no-exfil property is also required by
STORE-03 in `.planning/REQUIREMENTS.md`.

Three residual gaps are acknowledged as out-of-scope for v1.4.0:

### SENTRY-008 CI-runner OIDC theft

When tokens are stolen from CI runner process memory on a machine where no
editor or host agent is running, Beekeeper's ancestry gate (`isEditorDescendant`)
cannot observe the event. Architectural mitigation only: hardened token scoping
and short TTLs at the CI level.

### GitHub API dead-drop exfil

A malicious agent that has already exfiltrated secrets can create a private GitHub
repo or gist as a dead-drop via the GitHub API. A host tool cannot reliably
distinguish a legitimate `gh repo create` from a malicious one using stolen
credentials. This channel is out of host scope.

### DNS-tunnel ingested-but-undetected

DNS query events are captured on Linux (eBPF `udp_sendmsg`/`tcp_sendmsg` kprobe)
and Windows (ETW DNS-Client), but `EvaluateEvent`'s `EventDNSQuery` case is an
explicit no-op — no correlation rule currently consumes these events. DNS-TXT
tunneling is an undetected exfil channel until a rule is written.
```

**Note on verbatim strings** — REQUIREMENTS.md lists the three gap names verbatim as
"SENTRY-008 CI-runner OIDC theft", "GitHub API dead-drop exfil", and
"DNS-tunnel ingested-but-undetected". These exact strings must appear as sub-headers
or in bold so a grep gate (`TestThreatModelNames` or equivalent) can locate them.

**Header update** — the document's opening "Covers" or version line (currently v1.3.0)
must be updated to include v1.4.0.

---

## Shared Patterns

### Four-layer field-by-field assertion
**Source:** `internal/corpus/schema_lock_test.go` lines 208–285
**Apply to:** `launch_e2e_test.go` (LAUNCH-02) and the LAUNCH-01 extension in `catalogs_daemon_test.go`

Pattern: use a `[]fieldCheck` slice with `{name, value, wantAny, allow, skip, skipMsg}` fields
and a single range loop that calls `t.Errorf`. This is explicitly preferred over reflection
(RESEARCH.md §Don't Hand-Roll: "Reflection is fragile; field-by-field is readable").

### Go/parser AST import-purity gate
**Source:** `internal/sentry/imports_test.go` (full 47-line file)
**Apply to:** `internal/corpus/store_test.go` (`TestCorpusStoreHasNoNetworkImports`)

Pattern: `os.ReadFile(path)` → `parser.ParseFile(..., parser.ImportsOnly)` → range over
`f.Imports`, strip quotes from `imp.Path.Value`, check against `forbidden` map.
No `exec.Command("go", "list", ...)` — the AST approach is cheaper and avoids subprocess
overhead (RESEARCH.md §LAUNCH-04 Pitfall 4 also confirms direct import check is sufficient).

### Corpus-enabled config for runCheck tests
**Source:** `internal/check/handler_test.go` lines 1486–1498
**Apply to:** `TestBenchmarkRunCheckGate` in `handler_test.go`

```go
cfg := config.Config{
    FailMode: config.FailModeClosed,
    Corpus: config.CorpusConfig{
        Enabled: true,
        Path:    corpusPath,  // under t.TempDir()
    },
}
```
Always pair with `ReadFile` tool input (not Bash npm-install) to avoid nudge subprocess.

### BEEKEEPER_HOME environment redirect
**Source:** `cmd/beekeeper/catalogs_daemon_test.go` lines 84–97
**Apply to:** any test in `cmd/beekeeper/` that drives `runCatalogsSync` or `platform.StateDir()`

```go
home := t.TempDir()
t.Setenv("BEEKEEPER_HOME", home)
stateDir := filepath.Join(home, "beekeeper")
// MkdirAll stateDir, catalogDir, auditDir, corpusDir
```

---

## No Analog Found

All five files have close analogs in the codebase. No file is left without a pattern.

---

## Metadata

**Analog search scope:** `internal/corpus/`, `internal/sentry/`, `internal/check/`, `cmd/beekeeper/`, `docs/`
**Files read:** `schema_lock_test.go`, `store_test.go`, `emitter_test.go`, `fuzz_test.go`,
`imports_test.go` (sentry), `rules_test.go` (sentry), `handler_test.go` (check),
`latency_persist_test.go` (check), `catalogs_daemon_test.go`
**Pattern extraction date:** 2026-06-14
