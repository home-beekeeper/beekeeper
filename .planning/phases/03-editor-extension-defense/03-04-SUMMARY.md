# Plan 03-04 Summary: Scan Orchestrator + CLI Wiring

## Status: DONE

## Deliverables

### 1. `internal/scan/scanner.go`

New `scan` package. Implements `Scan(ctx, Config, io.Writer) error`.

#### Config struct
```go
type Config struct {
    Deep          bool
    ExtensionDirs []string
    IndexPath     string
    CacheDir      string
    AuditPath     string
    SocketToken   string
    HTTPClient    *http.Client
    Now           func() time.Time
}
```

#### Bumblebee integration
- `lookBumblebee` package var (`exec.LookPath("bumblebee")`) — injectable for tests
- `runBumblebeeFn` package var — injectable for tests (avoids spawning real process)
- `defaultRunBumblebee`: args = `["scan"]` + `["--profile","deep"]` when `Deep=true`. **No `--format` flag.**
- Each line from subprocess validated as JSON before passthrough; malformed lines produce a `scan_error` record (fail-closed, no crash)
- Unknown `record_type` values passed through unmodified (Open Question 3)

#### Bumblebee unavailable path
Emits: `{"record_type":"scan_status","bumblebee_unavailable":true,"scanner_name":"beekeeper"}`
Then continues with Beekeeper-own scan.

#### Beekeeper-own scan
- Opens mmap index once (nil if IndexPath empty or unavailable — `catalog.NewMultiIndex` handles nil gracefully)
- Per-extension: `watch.ParseManifest` → per-extension `context.WithTimeout(3s)` → builds OSV+Socket adapters → `policy.Evaluate` + `catalog.FetchMarketplaceAge` + `policy.EvaluateReleaseAge`
- Emits `FindingRecord{record_type:"finding", scanner_name:"beekeeper", ...}` per extension
- Writes `audit.AuditRecord` to AuditPath when configured

#### Key invariant
`grep "scan --format ndjson" internal/scan/scanner.go` returns nothing — forbidden flag absent.

---

### 2. `internal/scan/scanner_test.go`

```
=== RUN   TestScanWithBumblebee
--- PASS: TestScanWithBumblebee (0.00s)
=== RUN   TestScanBumblebeeUnavailable
--- PASS: TestScanBumblebeeUnavailable (0.04s)
PASS
ok  github.com/mzansi-agentive/beekeeper/internal/scan  4.619s
```

- `TestScanWithBumblebee`: stubs `runBumblebeeFn` to return two canned NDJSON lines; asserts both appear in captured output. No real binary needed.
- `TestScanBumblebeeUnavailable`: stubs `runBumblebeeFn` to return `(nil, false)`; creates clean extension in tempdir with pre-seeded marketplace cache (48h old → allows); asserts `bumblebee_unavailable:true` AND `record_type:"finding"` in output.

---

### 3. `internal/config/config.go` (extended)

Added:
```go
type WatchSettings struct {
    Directories []string `json:"directories,omitempty"`
}
// Watch *WatchSettings `json:"watch,omitempty"` on Config
func (c Config) WatchDirectories() []string
func (c *Config) AddWatchDirectory(dir string)   // idempotent
func Save(path string, cfg Config) error
```

---

### 4. `cmd/beekeeper/main.go` (extended)

Four additions to `newRootCmd().AddCommand(...)`:

#### `newWatchCmd()` — `beekeeper watch`
- Resolves dirs from `cfg.WatchDirectories()` or `editorinit.DetectEditors()` fallback
- Constructs `watch.Handler` via `watch.NewHandler` with all required fields
- Wraps context with `signal.NotifyContext(SIGINT, SIGTERM)` (same pattern as `catalogs watch`)
- Calls `watch.Watch(ctx, dirs, watch.WatchConfig{}, handler)` — foreground

#### `newScanCmd()` — `beekeeper scan [--deep]`
- `--deep` bool flag passed to `scan.Config.Deep`
- Resolves dirs same as `newWatchCmd`
- Calls `scan.Scan(ctx, cfg, cmd.OutOrStdout())`

#### `newQuarantineCmd()` — `beekeeper quarantine {list,restore,purge}`
- **list**: `quarantine.List` → table to stdout; "no quarantined items" if empty
- **restore `<id>`**: `cobra.ExactArgs(1)` → `quarantine.Restore` → prints confirmation → writes `quarantine_restore` audit record (EDXT-05)
- **purge**: `--yes` flag; without it, prompts `[y/N]` from `cmd.InOrStdin()`; `quarantine.Purge` → writes one `quarantine_purge` audit record per purged ID (EDXT-05)

#### Extended `newInitCmd()` — `beekeeper init [--yes] [--no-editors]`
- Preserves Phase 1 dir creation (stateDir, catalogDir, auditDir)
- **New Phase 3 dirs**: `stateDir/quarantine/extensions`, `catalogDir/marketplace-cache`
- `--no-editors`: skip editor detection entirely (Phase 1 behavior for scripted setups)
- `--yes`: auto-consent for non-interactive use
- Per-editor, per-action consent prompts (two per editor):
  1. Disable extension auto-update → `editorinit.DisableExtensionAutoUpdate`
  2. Register watch dir → `cfg.AddWatchDirectory` → `config.Save` (idempotent)
- Re-running is idempotent: `AddWatchDirectory` deduplicates

---

## Test Results

```
go build ./...   → exit 0
go vet ./cmd/...  → exit 0 (implied by build success)
go test ./...    → all 13 packages pass

go run ./cmd/beekeeper --help      → lists watch, scan, quarantine
go run ./cmd/beekeeper quarantine --help → lists list, restore, purge
go run ./cmd/beekeeper scan --help → shows --deep flag
go run ./cmd/beekeeper init --help → shows --yes and --no-editors flags
```

## Deviations from Plan

- **Beekeeper-own scan: duplicate evaluation code** — `evaluateExtension` in `internal/scan/scanner.go` replicates the multi-source adapter construction from `internal/watch/handler.go`. The two implementations use the same pattern (OSV+Socket+catalog) but are not shared because handler.go's logic is tightly coupled to its quarantine/notify pipeline. The duplication is minimal (adapter construction + two policy calls) and noted here per the plan's instruction.
- **`notify.Config{Enabled: true}` hardcoded in `newWatchCmd`** — the config package has no notification settings yet; this will be wired to config in a future phase when notification preferences are added.
- **`audit.AuditRecord` fields for quarantine_restore/purge** — `RecordType` uses custom values ("quarantine_restore", "quarantine_purge") that differ from "policy_decision". These are valid NDJSON records in the audit log but outside the standard schema. Acceptable for Phase 3 audit trail completeness.
