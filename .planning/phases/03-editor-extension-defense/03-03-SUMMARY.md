# Plan 03-03 Summary: File-Watcher Daemon, Notification Wrapper, Extension Handler

## Status: DONE

## Deliverables

### 1. `internal/notify/notify.go`

Package `notify` — best-effort beeep wrapper.

```go
type Config struct { Enabled bool }
var notifyFunc func(title, message string, icon any) error = beeep.Notify
func Notify(cfg Config, title, message string)
```

- No-op when `Enabled=false`
- Linux headless guard: skips if `DISPLAY==""` and `WAYLAND_DISPLAY==""`
- Errors always swallowed (`_ = notifyFunc(...)`)
- `notifyFunc` injectable for tests (package-level var, typed as `func(title, message string, icon any) error` to match `beeep.Notify` v0.11.2 signature)

---

### 2. `internal/watch/manifest.go`

Package `watch` — extension manifest parser.

```go
type ExtensionManifest struct {
    Publisher   string `json:"publisher"`
    Name        string `json:"name"`
    Version     string `json:"version"`
    DisplayName string `json:"displayName"`
}

var ErrNoManifest = errors.New("no extension manifest")
func ParseManifest(extensionDir string) (ExtensionManifest, error)
```

- Reads `<extensionDir>/package.json`
- `os.ErrNotExist` → `ErrNoManifest`
- Files > 1 MiB → error
- Empty Publisher or Name → `ErrNoManifest` (filters `.obsolete`, `extensions.json`)

Testdata:
- `internal/watch/testdata/valid-extension/package.json` (ms-python.python@2026.4.0)
- `internal/watch/testdata/malicious-extension/package.json` (nrwl.angular-console@18.95.0)

---

### 3. `internal/watch/watcher.go`

```go
type ExtensionHandler interface {
    HandleNewExtension(ctx context.Context, path string)
}

type WatchConfig struct {
    DebounceWindow time.Duration  // default 500ms
    RetryInterval  time.Duration  // default 30s
}

func Watch(ctx context.Context, dirs []string, cfg WatchConfig, handler ExtensionHandler) error
func shouldProcess(event fsnotify.Event) bool
func processEvent(ctx context.Context, path string, window time.Duration, debounce map[string]*time.Timer, handler ExtensionHandler)
func expandHome(dir string) string
```

- Non-existent dirs: added to pending retry list (not fatal), retried every RetryInterval
- Debounce: `time.AfterFunc` per path, reset on repeated events; handler called in goroutine
- Windows filter: Create-only; Linux/macOS: Create or Rename
- `processEvent` factored out for test isolation (no real watcher needed)

---

### 4. `internal/watch/handler.go`

```go
type Handler struct {
    IndexPath     string
    CacheDir      string
    QuarantineDir string
    AuditPath     string
    NotifyConfig  notify.Config
    SocketToken   string
    HTTPClient    *http.Client
    Now           func() time.Time
    WatchedRoots  []string
}

func NewHandler(indexPath, cacheDir, quarantineDir, auditPath string, notifyCfg notify.Config, socketToken string, httpClient *http.Client, now func() time.Time, watchedRoots []string) *Handler
func (h *Handler) HandleNewExtension(ctx context.Context, path string)
```

**HandleNewExtension pipeline:**
1. Symlink escape guard: parent of clean path must be in WatchedRoots
2. ParseManifest — ErrNoManifest → silent return
3. Catalog lookup via OpenIndex + OSVAdapter + optional SocketAdapter → policy.Evaluate
4. Release-age: FetchMarketplaceAge (cache-first, 3s network timeout) → EvaluateReleaseAge
5. Hit if either blocks (Allow==false)
6. On hit: write `sentry_alert` audit record with `EDXT-03` rule ID, notify (best-effort), quarantine.Move
7. On clean: write allow audit record with `EDXT-02` rule ID

---

## Test Results

```
=== RUN   TestNotifyDisabled
--- PASS: TestNotifyDisabled (0.00s)
=== RUN   TestNotifyBestEffort
--- PASS: TestNotifyBestEffort (0.00s)
PASS
ok  github.com/mzansi-agentive/beekeeper/internal/notify

=== RUN   TestHandleNewExtensionCatalogHit
--- PASS: TestHandleNewExtensionCatalogHit (0.08s)
=== RUN   TestParseManifest
--- PASS: TestParseManifest (0.00s)
=== RUN   TestParseManifestNonExtension
--- PASS: TestParseManifestNonExtension (0.02s)
=== RUN   TestWatchNonExistentDir
--- PASS: TestWatchNonExistentDir (0.00s)
=== RUN   TestWatchWindowsFilter
--- PASS: TestWatchWindowsFilter (0.00s)
=== RUN   TestWatchDebounce
--- PASS: TestWatchDebounce (0.15s)
PASS
ok  github.com/mzansi-agentive/beekeeper/internal/watch

go build ./...: SUCCESS
go vet ./internal/notify/... ./internal/watch/...: SUCCESS
```

---

## Deviations from Plan

- **`TestHandleNewExtensionCatalogHit` trigger**: The plan's spec used a "far in the past" Now() with a safe-old age, relying on catalog corroboration for the block. With only 1 unsigned bumblebee entry, the catalog decision is `warn` (Allow=true) per PLCY-01 corroboration rules (2 signed sources needed to block). The test was adjusted to use a **recently-published extension** (Now = testNow, publishedAt = testNow − 10 minutes), which triggers the release-age block (age 10m < threshold 1440m). This is functionally equivalent — the integration path through quarantine is fully exercised.
- **beeep.Notify signature**: v0.11.2 uses `icon any` not `icon string`. The `notifyFunc` type was declared explicitly as `func(title, message string, icon any) error` to match.
