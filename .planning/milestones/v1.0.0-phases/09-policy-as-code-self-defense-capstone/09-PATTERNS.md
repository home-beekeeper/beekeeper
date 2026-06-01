# Phase 9: Policy as Code + Self-Defense Capstone - Pattern Map

**Mapped:** 2026-05-29
**Files analyzed:** 14 new/modified files
**Analogs found:** 14 / 14

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/policyloader/loader.go` | service | file-I/O + transform | `internal/catalog/osv.go` + `internal/config/config.go` | role-match |
| `internal/policyloader/validate.go` | utility | transform | `internal/catalog/sync.go` (ParseCatalogFile/ValidateSchemaVersion) | role-match |
| `internal/policyloader/loader_test.go` | test | file-I/O | `internal/catalog/loader_test.go` | exact |
| `internal/policyloader/testdata/*.json` | config | — | `testdata/catalog/*.json` (root testdata) | exact |
| `internal/catalog/selfcatalog.go` | service | request-response + file-I/O | `internal/catalog/osv.go` (HTTP fetch + disk cache + adapter) | exact |
| `internal/catalog/selfcatalog_test.go` | test | file-I/O | `internal/catalog/osv_test.go` + `state_test.go` | exact |
| `internal/check/diag.go` | utility | request-response | `internal/llamafirewall/latency.go` + `internal/catalog/state.go` | role-match |
| `internal/check/diag_windows.go` | utility | request-response | `cmd/beekeeper/protect_windows.go` (build-tagged) | exact |
| `internal/check/diag_other.go` | utility | request-response | `cmd/beekeeper/protect_other.go` (build-tagged stub) | exact |
| `internal/config/config.go` (modify) | config | file-I/O + transform | itself — extend existing `Load` + `Config` struct | self |
| `internal/llamafirewall/latency.go` (modify) | utility | transform | itself — add `P99()` next to `P95()` | self |
| `internal/catalog/state.go` (modify) | model | file-I/O | itself — extend `WatchState` with `SelfQuarantine` field | self |
| `cmd/beekeeper/main.go` (modify) | controller | request-response | itself — follow `newCatalogsCmd()` / `newQuarantineCmd()` pattern | self |
| `docs/THREAT-MODEL.md` | doc | — | none | no analog |

---

## Pattern Assignments

### `internal/policyloader/loader.go` (service, file-I/O + transform)

**Analog:** `internal/config/config.go` (JSON read + unmarshal + validate pattern) and `internal/catalog/osv.go` (cache directory traversal pattern)

**Imports pattern** (`internal/config/config.go` lines 14-19):
```go
import (
    "encoding/json"
    "errors"
    "fmt"
    "os"
)
```

**Core file-load pattern** (`internal/config/config.go` lines 157-184):
```go
func Load(path string) (Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            return Config{FailMode: FailModeClosed}, nil  // missing = default, not error
        }
        return Config{}, fmt.Errorf("read config %q: %w", path, err)
    }

    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return Config{}, fmt.Errorf("parse config %q: %w", path, err)
    }
    // ... field validation ...
    return cfg, nil
}
```

**Directory scan pattern for `ListPolicyFiles`** — follow `os.ReadDir` with empty-dir-as-OK (not-exist treated as empty, not error). Mirrors Pitfall 3 avoidance documented in RESEARCH.md.

**Struct definition pattern** (`internal/config/config.go` lines 89-121 for Config; each sub-struct has `json:` tags, `omitempty` for optional fields):
```go
type Config struct {
    FailMode string        `json:"fail_mode"`
    Socket   SocketConfig  `json:"socket"`
    Watch    *WatchSettings `json:"watch,omitempty"`
    // ...
}
```

**Key invariant:** `policyloader` MUST NOT import `internal/policy` for the purpose of performing I/O. `internal/policy` remains zero-import from policyloader on the I/O side; the loader converts to engine inputs before passing to `policy.Evaluate`.

---

### `internal/policyloader/validate.go` (utility, transform)

**Analog:** `internal/catalog/sync.go` (`ParseCatalogFile` + `ValidateSchemaVersion`) and `internal/catalog/sanity.go` (enum validation pattern)

**Schema version guard pattern** (mirrors `catalog.ParseCatalogFile`):
```go
// SupportedSchemaVersion is the only accepted schema_version value.
const SupportedSchemaVersion = "1"

func ValidateSchema(pf PolicyFile) []error {
    var errs []error
    if pf.SchemaVersion != SupportedSchemaVersion {
        errs = append(errs, fmt.Errorf("unsupported schema_version %q (want %q)", pf.SchemaVersion, SupportedSchemaVersion))
    }
    // Enum-validate rule_type for every rule
    for i, r := range pf.Rules {
        switch r.RuleType {
        case "release_age", "package_allowlist", "sensitive_path",
             "lifecycle_script_allowlist", "corroboration_threshold":
            // valid
        default:
            errs = append(errs, fmt.Errorf("rule[%d] %q: unknown rule_type %q", i, r.ID, r.RuleType))
        }
        // Reject execution-surface fields
        // (no "url" field, no "action": "exec" — action enum: "block"|"warn"|"allow")
    }
    return errs
}
```

**All-errors-not-just-first pattern** — collect all `errs` into a slice, return the slice (not just `errs[0]`). This matches the `policy validate` requirement to report all field errors.

---

### `internal/policyloader/loader_test.go` (test, file-I/O)

**Analog:** `internal/catalog/loader_test.go` (lines 1-60)

**Test structure pattern** (`internal/catalog/loader_test.go` lines 1-60):
```go
package catalog   // same-package white-box test

import (
    "os"
    "path/filepath"
    "testing"
)

func testdataDir() string {
    return filepath.Join("..", "..", "testdata", "catalog")
    // policyloader equivalent: filepath.Join("testdata")  (package-local testdata/)
}

func readFixture(t *testing.T, name string) []byte {
    t.Helper()
    data, err := os.ReadFile(filepath.Join(testdataDir(), name))
    if err != nil {
        t.Fatalf("read fixture %q: %v", name, err)
    }
    return data
}

func TestCatalogParse(t *testing.T) {
    cf, err := ParseCatalogFile(readFixture(t, "nx-console.json"))
    if err != nil {
        t.Fatalf("ParseCatalogFile: unexpected error: %v", err)
    }
    // ... field assertions ...
}

func TestUnknownSchemaVersion(t *testing.T) {
    data := []byte(`{"schema_version":"0.2.0","entries":[]}`)
    if _, err := ParseCatalogFile(data); err == nil {
        t.Fatal("ParseCatalogFile(schema_version 0.2.0): expected error, got nil")
    }
}
```

**Fixture location convention:** Policy loader uses `internal/policyloader/testdata/` (package-local, not root `testdata/`). `readFixture` resolves as `filepath.Join("testdata", name)` — no `../..` needed since policyloader is a new package.

**Adversarial fixture test names to use** (from RESEARCH.md validation map):
- `TestLoadPolicyFile` — valid file round-trip
- `TestValidateSchema_RejectsExec` — `"action": "exec"` rejected
- `TestValidateSchema_UnknownRuleType` — unknown rule_type rejected
- `TestListPolicyFiles_MissingDir` — missing dir → empty list, not error

---

### `internal/policyloader/testdata/*.json` (config fixtures)

**Analog:** `testdata/catalog/nx-console.json` (root testdata — catalog JSON fixtures)

Use the same format convention: minimal valid JSON files exercising one case each. File names map directly to test names:
- `valid_release_age.json`
- `valid_allowlist.json`
- `invalid_url_field.json`
- `invalid_exec_action.json`
- `invalid_unknown_rule_type.json`
- `invalid_schema_version.json`

---

### `internal/catalog/selfcatalog.go` (service, request-response + file-I/O)

**Analog:** `internal/catalog/osv.go` (HTTP fetch → disk cache → parse → `policy.MultiCatalogLookup` adapter)

**Imports pattern** (`internal/catalog/osv.go` lines 1-16):
```go
package catalog

import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "time"

    "github.com/mzansi-agentive/beekeeper/internal/policy"
)
```

**HTTP fetch + disk cache pattern** (`internal/catalog/osv.go` lines 118-160):
```go
func readOSVCache(cacheDir, ecosystem, pkg, version string) ([]Entry, bool, error) {
    path := osvCachePath(cacheDir, ecosystem, pkg, version)
    data, err := os.ReadFile(path)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            return nil, false, nil  // cache miss = not an error
        }
        return nil, false, fmt.Errorf("read osv cache %q: %w", path, err)
    }
    var ce osvCacheEntry
    if err := json.Unmarshal(data, &ce); err != nil {
        return nil, false, nil  // corrupt = treat as miss
    }
    if time.Since(ce.CachedAt) >= osvCacheTTL {
        return nil, false, nil  // expired = miss
    }
    return ce.Entries, true, nil
}

func writeOSVCache(cacheDir, ecosystem, pkg, version string, entries []Entry) error {
    path := osvCachePath(cacheDir, ecosystem, pkg, version)
    if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
        return fmt.Errorf("mkdir osv cache dir: %w", err)
    }
    ce := osvCacheEntry{CachedAt: time.Now().UTC(), Entries: entries}
    // ... writeFileAtomic(path, data) ...
}
```

**Atomic write pattern** (`internal/catalog/state.go` lines 76-92):
```go
func SaveState(path string, st WatchState) error {
    dir := filepath.Dir(path)
    if err := os.MkdirAll(dir, 0o700); err != nil {
        return fmt.Errorf("create state directory %q: %w", dir, err)
    }
    data, err := json.Marshal(st)
    if err != nil {
        return fmt.Errorf("marshal state: %w", err)
    }
    if err := writeFileAtomic(path, data); err != nil {
        return fmt.Errorf("write state %q: %w", path, err)
    }
    return nil
}
```

**MultiCatalogLookup adapter pattern** (`internal/catalog/multi.go` lines 75-125):
```go
type bumblebeeMultiAdapter struct{ idx *Index }

func (a *bumblebeeMultiAdapter) LookupAll(ecosystem, pkg string) []policy.CatalogMatch {
    e, ok := a.idx.Lookup(ecosystem, pkg)
    if !ok { return nil }
    // ... build []policy.CatalogMatch from entry ...
    return out
}
```

**MultiIndex extension pattern** (`internal/catalog/multi.go` lines 15-36):
```go
type MultiIndex struct {
    Bumblebee *Index
    OSV       policy.MultiCatalogLookup
    Socket    policy.MultiCatalogLookup
    // Phase 9: add BeeKeeperSelf policy.MultiCatalogLookup
}

func (m *MultiIndex) LookupAll(ecosystem, pkg string) []policy.CatalogMatch {
    var matches []policy.CatalogMatch
    if m.Bumblebee != nil { ... }
    if m.OSV != nil { matches = append(matches, m.OSV.LookupAll(ecosystem, pkg)...) }
    if m.Socket != nil { matches = append(matches, m.Socket.LookupAll(ecosystem, pkg)...) }
    // Phase 9: if m.BeeKeeperSelf != nil { matches = append(...) }
    return matches
}
```

**Version variable for self-catalog match** (`internal/version/version.go` lines 16-20):
```go
package version

var (
    Version = "dev"   // overwritten via -ldflags -X ...version.Version=v1.0.0
    Commit  = "none"
    Date    = "unknown"
)
```

**`SelfQuarantineState` extension to `WatchState`** (`internal/catalog/state.go` lines 38-41):
```go
// Existing:
type WatchState struct {
    Sources map[string]SourceState `json:"sources"`
    // Phase 9: add:
    // SelfQuarantine *SelfQuarantineState `json:"self_quarantine,omitempty"`
}
```

**Fail-closed vs. network-error branching note:** The selfcatalog check must distinguish `errIntegrity` (signature invalid → fail closed) from `errNetwork` (fetch failed → warn + continue). Use typed sentinel errors or `errors.As` — do NOT collapse them into a single `if err != nil { os.Exit(1) }`.

---

### `internal/catalog/selfcatalog_test.go` (test, file-I/O)

**Analog:** `internal/catalog/osv_test.go` and `internal/catalog/state_test.go`

**Table-driven HTTP mock pattern** (matches OSV test convention): inject a `*httptest.Server` whose handler returns fixture JSON; test multiple scenarios (match, no-match, invalid sig, network error, fresh cache, stale cache).

**WatchState extension test pattern** (`internal/catalog/state_test.go`): round-trip `SaveState` → `LoadState` with a `SelfQuarantine` field set; verify the field survives the JSON round-trip without corruption.

---

### `internal/check/diag.go` (utility, request-response)

**Analog:** `internal/llamafirewall/latency.go` (accessor pattern) + `internal/catalog/state.go` (`LoadState` call)

**DiagReport struct** — plain Go struct, no I/O methods, assembled by `CollectDiag(stateFile string) DiagReport`:
```go
// Mirrors LatencyTracker accessor pattern (internal/llamafirewall/latency.go lines 36–54).
// Mirrors SourceState struct (internal/catalog/state.go lines 17–34).
type DiagReport struct {
    HookLatencyP95MS    int64
    HookLatencyP99MS    int64
    SidecarLatencyP95MS int64
    CatalogSources      []CatalogSourceStatus
    ETWEventsLost       uint64  // populated by platform-specific eventsLost()
}

type CatalogSourceStatus struct {
    Name     string
    Degraded bool
    Count    int
    Hash     string
}
```

**GlobalHookTracker declaration** — package-level var in `internal/check/handler.go` (or diag.go), same pattern as `llamafirewall.GlobalLatencyTracker`:
```go
// GlobalHookTracker accumulates per-invocation latency samples for beekeeper diag.
// Initialized once; Record() called at the end of runCheck.
var GlobalHookTracker = &llamafirewall.LatencyTracker{}
```

**`runCheck` integration point** (`internal/check/handler.go` lines 77-208): add `Record` call at the bottom of `runCheck` before returning, measuring elapsed time from the function start:
```go
start := time.Now()
// ... existing runCheck body ...
GlobalHookTracker.Record(time.Since(start).Milliseconds())
return result
```

**Platform dispatch** — `CollectDiag` calls `eventsLost()` which is defined in `diag_windows.go` and `diag_other.go` (see below).

---

### `internal/check/diag_windows.go` (utility, request-response)

**Analog:** `cmd/beekeeper/protect_windows.go` lines 1-3 (build tag) and `internal/sentry/windows/etw.go` lines 16 + `daemon.go` line 233 (atomic read pattern)

**Build tag + import pattern** (`cmd/beekeeper/protect_windows.go` lines 1-23):
```go
//go:build windows

package check

import (
    "sync/atomic"
    windows "github.com/mzansi-agentive/beekeeper/internal/sentry/windows"
)

func eventsLost() uint64 {
    return atomic.LoadUint64(&windows.EventsLost)
}
```

**ETW EventsLost source** (`internal/sentry/windows/etw.go` line 16):
```go
var EventsLost uint64  // incremented atomically; read by diag_windows.go
```

---

### `internal/check/diag_other.go` (utility, request-response)

**Analog:** `cmd/beekeeper/protect_other.go` lines 1-4 (build tag stub)

```go
//go:build !windows

package check

func eventsLost() uint64 { return 0 }  // ETW not available on this platform
```

**Critical pair requirement:** Both `diag_windows.go` (`//go:build windows`) and `diag_other.go` (`//go:build !windows`) must exist. Omitting either produces a compile error. This is verified in `cmd/beekeeper/protect_*.go` which has four files: `protect_linux.go`, `protect_darwin.go`, `protect_windows.go`, `protect_other.go` (the last catches everything else).

---

### `internal/config/config.go` (modify — extend layered merge)

**Self-reference.** Existing patterns to extend:

**Current `Load` function** (lines 157-184) — the baseline single-file loader. `LoadLayered` calls `Load` per layer, never duplicates its parsing logic.

**Config struct** (lines 89-121) — the merged result type. Add a `SelfCatalog` field for Phase 9:
```go
type SelfCatalogConfig struct {
    URL    string `json:"url,omitempty"`    // defaults to official endpoint
    PubKey string `json:"pub_key,omitempty"` // base64 public key override
}

// In Config struct:
SelfCatalog SelfCatalogConfig `json:"self_catalog,omitempty"`
```

**`LoadLayered` extension point** — the Phase 1 comment on line 8 explicitly marks this:
```
// The full layered system→user→project→env→flag merge (CODE-05) lands in Phase 9
// and is out of scope here.
```

**FailMode validation** (lines 171-183) — the switch/case pattern to validate FailMode after merge is the template for `validate(cfg Config) (Config, error)` in `LoadLayered`:
```go
switch cfg.FailMode {
case FailModeClosed, FailModeOpen, FailModeWarn:
    // valid
default:
    return Config{}, fmt.Errorf("invalid fail_mode %q (want %q, %q, or %q)",
        cfg.FailMode, FailModeClosed, FailModeOpen, FailModeWarn)
}
```

**`merge` function pitfall** — bool fields (`LlamaFirewall.Enabled`) need pointer-or-sentinel to distinguish "not set" from "set to false". Use `*bool` in the merge-internal representation, or a separate `isSet` map. The `Save` function (lines 187-197) shows the `0600` permission convention for config writes.

---

### `internal/llamafirewall/latency.go` (modify — add `P99()`)

**Self-reference.** The existing `P95()` implementation (lines 36-54) is the exact template:

```go
// P95 — existing (lines 36–54):
func (t *LatencyTracker) P95() int64 {
    t.mu.Lock(); defer t.mu.Unlock()
    if t.count == 0 { return 0 }
    n := 100
    if !t.filled { n = t.head }
    buf := make([]int64, n)
    copy(buf, t.p95buf[:n])
    sort.Slice(buf, func(i, j int) bool { return buf[i] < buf[j] })
    idx := int(float64(n) * 0.95)
    if idx >= n { idx = n - 1 }
    return buf[idx]
}

// P99 — new, identical structure with 0.99:
func (t *LatencyTracker) P99() int64 {
    t.mu.Lock(); defer t.mu.Unlock()
    if t.count == 0 { return 0 }
    n := 100
    if !t.filled { n = t.head }
    buf := make([]int64, n)
    copy(buf, t.p95buf[:n])
    sort.Slice(buf, func(i, j int) bool { return buf[i] < buf[j] })
    idx := int(float64(n) * 0.99)
    if idx >= n { idx = n - 1 }
    return buf[idx]
}
```

Note: the ring buffer field is named `p95buf` — reuse the same field for P99 (it holds the last 100 samples regardless of percentile). No new struct field needed.

---

### `internal/catalog/state.go` (modify — add `SelfQuarantineState`)

**Self-reference.** The existing `SourceState` and `WatchState` pattern (lines 17-41) is the template for the extension:

**Existing `WatchState`** (lines 38-41):
```go
type WatchState struct {
    Sources map[string]SourceState `json:"sources"`
}
```

**Extension** — backward-compatible via `omitempty`:
```go
type WatchState struct {
    Sources        map[string]SourceState  `json:"sources"`
    SelfQuarantine *SelfQuarantineState    `json:"self_quarantine,omitempty"`
}

type SelfQuarantineState struct {
    Version string `json:"version"`
    EntryID string `json:"entry_id"`
    Reason  string `json:"reason"`
    FiredAt string `json:"fired_at"` // RFC3339
}
```

**`LoadState` nil-safety pattern** (lines 48-68): `LoadState` already guards `st.Sources == nil` after unmarshal. Add the same guard for `SelfQuarantine` — omitempty handles absent JSON; no extra guard needed since pointer is nil on absent.

**Atomic write pattern** (lines 76-92 via `writeFileAtomic`): `SaveState` already uses `writeFileAtomic`. Phase 9 self-quarantine writes go through the same `SaveState` call — no new write path needed.

---

### `cmd/beekeeper/main.go` (modify — add `newPolicyCmd` + `newDiagCmd`)

**Self-reference.** The existing registration pattern (lines 53-72) is the exact template:

**Root command registration** (lines 53-72):
```go
root.AddCommand(
    newVersionCmd(),
    newInitCmd(),
    newCheckCmd(),
    // ... existing ...
    newDashboardCmd(),
    // Phase 9: add newPolicyCmd(), newDiagCmd()
)
```

**Grouped subcommand pattern** (`newCatalogsCmd` lines 288-415 or `newQuarantineCmd` lines 552-701):
```go
func newPolicyCmd() *cobra.Command {
    policyCmd := &cobra.Command{
        Use:   "policy",
        Short: "Manage and test declarative policy files",
    }
    policyCmd.AddCommand(
        newPolicyValidateCmd(),  // beekeeper policy validate <file>
        newPolicyTestCmd(),      // beekeeper policy test <file> [--tool-call <json>]
        newPolicyListCmd(),      // beekeeper policy list
    )
    return policyCmd
}
```

**Standalone simple command pattern** (`newVersionCmd` lines 78-91 or `newSelftestCmd` lines 913-930):
```go
func newDiagCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "diag",
        Short: "Show system health: hook latency, sidecar latency, catalog freshness, ETW loss",
        Args:  cobra.NoArgs,
        RunE: func(cmd *cobra.Command, _ []string) error {
            // resolve stateFile from platform.StateDir()
            // call check.CollectDiag(stateFile)
            // format and print DiagReport to cmd.OutOrStdout()
            return nil
        },
    }
}
```

**`config.Load` call pattern** (repeated in every RunE, e.g. lines 157, 255, 443, 514): each RunE resolves `configPath` via `platform.ConfigPath()` then calls `config.Load(configPath)`. Phase 9 CODE-05 centralizes this in `PersistentPreRunE` on the root — but the per-subcommand calls remain until the centralization lands. Match the existing pattern for Phase 9 subcommands initially.

**Error return pattern** — every RunE wraps errors with `fmt.Errorf("action: %w", err)`. No naked `return err`.

---

## Shared Patterns

### Atomic file writes (all catalog + state writes)
**Source:** `internal/catalog/state.go` lines 76-92 via `writeFileAtomic`
**Apply to:** `internal/catalog/selfcatalog.go` (cache writes), `internal/catalog/state.go` (self-quarantine state)
```go
if err := writeFileAtomic(path, data); err != nil {
    return fmt.Errorf("write state %q: %w", path, err)
}
```

### Missing-file-is-OK pattern (all loaders)
**Source:** `internal/config/config.go` lines 159-162
**Apply to:** `internal/policyloader/loader.go` (`LoadPolicyFile`), `internal/catalog/selfcatalog.go` (cache read), `internal/policyloader/loader.go` (`ListPolicyFiles` — missing dir → empty list)
```go
if errors.Is(err, os.ErrNotExist) {
    return <zero-value>, nil  // missing = use defaults, not an error
}
```

### Build tag pair (all platform-split files)
**Source:** `cmd/beekeeper/protect_windows.go` line 1 + `cmd/beekeeper/protect_other.go` line 1
**Apply to:** `internal/check/diag_windows.go` + `internal/check/diag_other.go`

Windows file: `//go:build windows`
Non-Windows stub: `//go:build !windows`

Both files must be committed together. The non-Windows stub provides the function stub so non-Windows builds compile. Verified pattern from `internal/ipc/peer_linux.go` (`//go:build linux`) and `peer_darwin.go` (`//go:build darwin`).

### JSON struct tags convention
**Source:** `internal/config/config.go` lines 35-121, `internal/catalog/state.go` lines 17-41
**Apply to:** All new structs in `policyloader`, `selfcatalog`, `diag`, `config` extensions
```go
// Optional fields: omitempty
// Required fields: no omitempty
// Slices that default to empty: omitempty
// Pointers to sub-structs (optional sections): omitempty
```

### Error wrapping with context
**Source:** All `internal/` files — e.g. `internal/config/config.go` lines 162-163
**Apply to:** All new packages
```go
return Config{}, fmt.Errorf("read config %q: %w", path, err)
// Pattern: fmt.Errorf("<verb> <noun> %q: %w", identifier, err)
```

### Same-package white-box tests
**Source:** `internal/catalog/loader_test.go` line 1 (`package catalog`), `internal/policy/engine_test.go` line 1 (`package policy`)
**Apply to:** `internal/policyloader/loader_test.go`, `internal/catalog/selfcatalog_test.go`
- Use `package <pkgname>` (not `package <pkgname>_test`) for white-box access to unexported helpers
- Place test fixtures in `<package>/testdata/` (local to the package)

### Fake MultiCatalogLookup for tests
**Source:** `internal/policy/engine_test.go` lines 14-38
**Apply to:** `internal/policyloader/loader_test.go` (for `policy test` dry-run)
```go
type fakeMultiCatalog struct {
    matchesByKey map[string][]CatalogMatch
}
func (f fakeMultiCatalog) LookupAll(ecosystem, pkg string) []CatalogMatch {
    return f.matchesByKey[ecosystem+"::"+pkg]
}
```

### import path convention
**Source:** `internal/check/handler.go` lines 22-31
**Apply to:** All new files
```go
import (
    "github.com/mzansi-agentive/beekeeper/internal/catalog"
    "github.com/mzansi-agentive/beekeeper/internal/policy"
    // ...
)
```

---

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `docs/THREAT-MODEL.md` | doc | — | No existing threat-model documentation file; planner should use RESEARCH.md §Threat Model Documentation Plan for section structure |

---

## Metadata

**Analog search scope:** `internal/config/`, `internal/catalog/`, `internal/check/`, `internal/llamafirewall/`, `internal/policy/`, `internal/sentry/windows/`, `internal/ipc/`, `internal/version/`, `cmd/beekeeper/`
**Files scanned:** 22
**Pattern extraction date:** 2026-05-29
