# Phase 3: Editor Extension Defense - Research

**Researched:** 2026-05-26
**Domain:** Go filesystem watching, VS Code/Cursor/Windsurf extension interception, desktop notifications, JSONC parsing, Bumblebee CLI orchestration
**Confidence:** HIGH (core APIs verified; extension directory paths for Cursor partially inferred)

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**EDXT-01: Agent-Initiated CLI Intercept**
- Hook handler (`beekeeper check`) receives Bash tool call JSON; command contains editor extension install invocation.
- Recognized patterns: `code --install-extension <id>[@<version>]`, `code-insiders --install-extension`, `cursor --install-extension`, `windsurf --install-extension`; bulk forms (multiple `--install-extension` flags).
- Detection in `internal/policy` (pure, no I/O) as a new `ExtensionInstallCommand` recognition function.
- Routes to catalog matcher (corroboration semantics) and release-age check with `ecosystem: "editor-extension"`.
- Decision recorded in NDJSON with `rule_ids` citing EDXT-01, full catalog provenance.

**EDXT-02: File-Watcher Daemon (`beekeeper watch`)**
- New subcommand in `cmd/beekeeper/main.go`; business logic in `internal/watch/`.
- Uses **`fsnotify` v1.10.1** — explicit import `github.com/fsnotify/fsnotify`.
- Watches: `~/.vscode/extensions/`, `~/.cursor/extensions/`, `~/.windsurf/extensions/`, OpenVSX paths if discovered.
- Implementation: explicit `watcher.Add()` per directory — no recursive API.
- Windows NTFS: filter to `Create` events only.
- macOS/Linux: `Create` and `Rename` events.
- Daemon runs until signal; exponential backoff on transient errors; logs to audit log.

**EDXT-03: New Extension Handling**
- On `Create` event: parse `<dir>/package.json` for `publisher`, `name`, `version`, `displayName`.
- Catalog match via `internal/catalog.MultiCatalogLookup`, `ecosystem: "editor-extension"`.
- Release-age check via `internal/policy.ReleaseAgeEvaluate`; Marketplace timestamp via Open VSX / VS Code Marketplace REST APIs with 24h TTL cache.
- On catalog hit OR release-age block: emit `critical` audit record, desktop notification (configurable default on), quarantine (move to `~/.beekeeper/quarantine/extensions/<publisher>.<name>-<version>-<timestamp>/`, write `beekeeper-manifest.json`).
- On clean: write allow audit record.

**EDXT-04: `beekeeper scan` Orchestrator**
- New subcommand `beekeeper scan [--deep]`; logic in `internal/scan/`.
- Invokes Bumblebee CLI (`bumblebee scan --format ndjson`) via `exec.CommandContext`; supervises stdout.
- Merges Bumblebee NDJSON + Beekeeper-specific results into unified NDJSON stream.
- `--deep` passes depth flag to Bumblebee.
- Catalog-delta-triggered via Phase 2 `catalogs watch` daemon.
- Degrades gracefully if Bumblebee not installed: runs Beekeeper-only scan, logs `bumblebee_unavailable: true`.

**EDXT-05: Quarantine Workflow**
- Subcommands: `beekeeper quarantine list`, `quarantine restore <id>`, `quarantine purge`.
- Logic in `internal/quarantine/`; IDs are directory basenames under `~/.beekeeper/quarantine/`.
- `list`: parses `beekeeper-manifest.json` per item, prints table.
- `restore <id>`: moves item back to original extension directory; removes `beekeeper-manifest.json`; emits audit record.
- `purge`: removes all items; prompts for confirmation unless `--yes`; emits audit record per item.
- All operations emit NDJSON audit records with `rule_ids: ["EDXT-05"]`.

**EDXT-06: `beekeeper init` Editor Detection**
- Existing `beekeeper init` command (Phase 1) extended.
- Detection: check for executables in PATH (`code`, `cursor`, `windsurf`, `codium`) and extension directories at known paths.
- Offers (explicit consent, never writes without it): disable extension auto-update (writes `"extensions.autoUpdate": false` to VS Code / Cursor user settings JSON using `go-jsonc` parser to preserve comments), enable file-watcher for detected extension directories in Beekeeper config, set release-age threshold (default 1440 minutes).
- Idempotent: re-running does not duplicate settings.

**Architecture: Where New Code Lives**
| Component | Package |
|-----------|---------|
| Extension CLI command recognition | `internal/policy` (pure, added to Evaluate) |
| File-watcher daemon | `internal/watch/` |
| Desktop notifications | `internal/notify/` |
| Scan orchestrator | `internal/scan/` |
| Quarantine management | `internal/quarantine/` |
| Marketplace timestamp fetcher | `internal/catalog/` (new adapter, cache-first like OSV/Socket) |
| `beekeeper watch` subcommand | `cmd/beekeeper/main.go` |
| `beekeeper scan` subcommand | `cmd/beekeeper/main.go` |
| `beekeeper quarantine *` subcommands | `cmd/beekeeper/main.go` |

### Claude's Discretion
- Specific fsnotify event debounce window: suggested 500ms.
- Marketplace timestamp API selection (VS Code Marketplace vs. Open VSX REST API).
- `go-toast` vs. alternative for Windows desktop notifications — least transitive dependency footprint.
- Whether `beekeeper watch` runs as foreground daemon (PRD implies foreground for now; background is Phase 4).
- Test fixtures: synthetic `package.json` manifests in `internal/watch/testdata/`.

### Deferred Ideas (OUT OF SCOPE)
- True extension-loader hooking (requires editor-vendor cooperation).
- Background / daemonized `beekeeper watch` as a system service (Phase 4).
- OpenVSX path auto-detection beyond three primary editors.
- ContextForge / MCPGuard policy-plugin mode (Phase 4 / v1.5).
- Weighted corroboration for extensions (HARD-02, v2 requirement).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| EDXT-01 | Agent-initiated CLI intercept for all four editor install commands including bulk forms; routes through catalog + release-age | New `extractExtensionInstall` pure function in `internal/policy`; existing `Evaluate` extended; `editor-extension` ecosystem added to installPrefixes |
| EDXT-02 | File-watcher daemon via fsnotify v1.10.1; explicit per-directory `watcher.Add()`; platform-aware event filtering (Create-only on Windows) | fsnotify v1.10.1 API verified; non-existent directory handling pattern documented; debounce strategy documented |
| EDXT-03 | New extension handling: manifest parse → catalog match → release-age → quarantine + desktop notification + audit | Manifest schema verified; marketplace timestamp APIs documented; notification library selected (beeep v0.11.2); quarantine directory structure defined |
| EDXT-04 | `beekeeper scan` orchestrator invokes Bumblebee CLI; merges NDJSON; degrades if Bumblebee absent | Bumblebee CLI schema verified; exec.CommandContext pattern established; graceful degradation path documented |
| EDXT-05 | Quarantine workflow: list/restore/purge; beekeeper-manifest.json metadata; original path preservation | Quarantine directory structure and manifest schema documented; restore requires original_path field |
| EDXT-06 | `beekeeper init` editor detection and consent-based settings update; JSONC-safe settings.json editing | JSONC libraries evaluated (`tidwall/jsonc` for reading, custom writer for preserving comments); editor settings path documented |
</phase_requirements>

---

## Summary

Phase 3 adds three layers of editor extension defense on top of the Phase 2 policy engine and catalog infrastructure. The core challenge is ensuring each layer correctly uses the existing Phase 2 patterns rather than duplicating logic: the CLI intercept extends `internal/policy` (pure), the file-watcher builds on the Phase 2 `internal/catalog/watch.go` daemon pattern, and the marketplace timestamp adapter follows the OSV/Socket cache-first HTTP adapter pattern in `internal/catalog/`.

The most significant new technical surface is the filesystem watcher daemon (`internal/watch/`). The critical finding here is that **fsnotify v1.10.1's `watcher.Add()` requires the path to exist at watch time** — directories like `~/.cursor/extensions/` may not exist on machines that don't have Cursor installed. The `beekeeper watch` daemon must check for directory existence before calling `Add()`, use a polling loop to retry adding non-existent paths periodically, and log when a configured path is skipped. This is not optional error-handling; it is the only safe pattern.

For desktop notifications, `gen2brain/beeep` v0.11.2 is the correct choice. It is CGO-free on all platforms (D-Bus on Linux via pure-Go `godbus/dbus`; PowerShell/COM on Windows; `osascript` on macOS) and has a single, simple API call. The `nodbus` build tag is available to eliminate the `godbus/dbus` dep on Linux if the transitive dependency is undesirable, falling back to `notify-send`. The Windows implementation uses pure-Go COM API via `git.sr.ht/~jackmordaunt/go-toast` — no CGO.

For JSONC parsing (settings.json editing in EDXT-06), `tidwall/jsonc` v0.3.3 provides clean stripping of comments for reading. However, it cannot preserve comments when writing. Since EDXT-06 only needs to **add** a single key (`"extensions.autoUpdate": false`) and writing must preserve existing comments, the implementation should: (1) read and strip comments via `tidwall/jsonc`, (2) unmarshal to a map, (3) add/update the key, (4) marshal back to JSON, (5) write. Comments are lost on write — this is a known limitation that must be documented in the consent prompt. Alternatively, a targeted regex/string-patch approach can add the key without stripping comments.

**Primary recommendation:** Extend the existing Phase 2 patterns directly. No new architecture patterns are needed; this phase wires new consumers (watch daemon, scan orchestrator, quarantine manager) to existing `internal/catalog`, `internal/policy`, and `internal/audit` packages.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Extension CLI recognition (EDXT-01) | `internal/policy` (pure lib) | Called by `internal/check` handler | Must stay pure; same pattern as existing installPrefixes |
| File-watcher event loop (EDXT-02) | `internal/watch/` (daemon, I/O) | Calls `internal/catalog.MultiCatalogLookup` | I/O adapter tier; not in policy engine |
| Extension manifest parsing | `internal/watch/` (I/O) | Calls `internal/catalog` | filesystem read — adapter tier |
| Marketplace timestamp fetch (EDXT-03) | `internal/catalog/` (new adapter) | Cache-first like OSV/Socket adapters | Follows established I/O adapter pattern |
| Catalog match + release-age check | `internal/policy` (pure) | Pre-resolved by watch adapter | Policy engine consumes pre-resolved inputs |
| Desktop notification (EDXT-03) | `internal/notify/` | Called by `internal/watch/` handler | OS-level side effect; own package |
| Quarantine move (EDXT-03, EDXT-05) | `internal/quarantine/` | Called by watch + CLI subcommands | Filesystem operation; own package |
| Bumblebee scan orchestration (EDXT-04) | `internal/scan/` | `exec.CommandContext` caller | Subprocess supervision; not in policy |
| Editor detection + settings patch (EDXT-06) | `cmd/beekeeper/main.go` extended init | `internal/watch/` for dir registration | Thin Cobra wiring; logic in init handler |
| NDJSON audit records | `internal/audit/` (existing) | All new packages call WriteAudit | Existing package; extend record types |

---

## Standard Stack

### Core (phase-locked)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/fsnotify/fsnotify` | v1.10.1 | Filesystem event watching for `beekeeper watch` | Already in go.mod from Phase 2; explicitly pinned in CONTEXT.md |
| `github.com/gen2brain/beeep` | v0.11.2 | Cross-platform desktop notifications | CGO-free; pure Go D-Bus on Linux; COM/PowerShell on Windows; `osascript` on macOS; single `Notify()` call; BSD-2-Clause |
| `github.com/tidwall/jsonc` | v0.3.3 | JSONC parsing for VS Code settings.json | Zero external deps; MIT; strips comments for `json.Unmarshal`; fastest available |
| `net/http` | stdlib | Marketplace timestamp API queries | Already established pattern; no new dep |
| `encoding/json` | stdlib | Extension manifest parsing, quarantine manifest, Bumblebee NDJSON | Consistent with all Phase 1/2 choices |
| `os/exec` | stdlib | Bumblebee CLI subprocess invocation | Standard Go subprocess; no external dep needed |

[VERIFIED: fsnotify v1.10.1 published 2026-05-04 via proxy.golang.org — already in go.mod]
[VERIFIED: beeep v0.11.2 published Dec 11, 2025 via pkg.go.dev; CGO-free confirmed; Windows uses pure-Go go-toast]
[VERIFIED: tidwall/jsonc v0.3.3 confirmed via pkg.go.dev; 0 external deps; MIT license]

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `os/signal` | stdlib | SIGTERM/SIGINT handling in watch daemon | Same pattern as Phase 2 `catalogs watch` |
| `time` | stdlib | Debounce timer (`time.AfterFunc`), TTL cache | Standard; no external debounce library needed |
| `path/filepath` | stdlib | Cross-platform path construction for extension dirs | Same as all Phase 1/2 file operations |
| `strings` | stdlib | Extension install command parsing (pure policy) | No external dep needed |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `beeep` (gen2brain) | `go-toast` (Windows-only) | go-toast is Windows-only; beeep is cross-platform with equivalent Windows support |
| `beeep` (gen2brain) | `martinlindhe/notify` | martinlindhe/notify is less actively maintained; beeep has 557 importers and Dec 2025 release |
| `tidwall/jsonc` | `marcozac/go-jsonc` | Both strip comments for reading; tidwall has zero deps; marcozac adds complexity for no benefit in read-only use case |
| `tidwall/jsonc` + targeted string insertion | Full JSONC read/write preserving comments | VS Code settings.json comment preservation requires a custom AST-preserving writer; overkill for inserting one key; document comment loss in consent prompt |
| `exec.CommandContext` for Bumblebee | Importing Bumblebee as a library | Bumblebee is CLI-only (`go install` model); no importable library confirmed; exec is correct |
| `time.AfterFunc` debounce | External debounce library (`go-debounce`) | 10-15 lines of stdlib code; no external dep justified for a single debounce use case |

**Installation (new deps only):**
```bash
go get github.com/gen2brain/beeep@v0.11.2
go get github.com/tidwall/jsonc@v0.3.3
# fsnotify already in go.mod from Phase 2
```

---

## Architecture Patterns

### System Architecture Diagram

```
                    PHASE 3: EDITOR EXTENSION DEFENSE — DATA FLOW
                    ─────────────────────────────────────────────

LAYER 1: Agent CLI Intercept
─────────────────────────────
Agent tool call (Bash: "code --install-extension <id>")
        │
        ▼
  beekeeper check (stdin) → internal/check/handler.go (RunCheck)
        │
        ├→ policy.extractExtensionInstall()  [NEW: pure, internal/policy]
        │       └→ (ecosystem="editor-extension", publisher.name, version)
        │
        ├→ internal/catalog.MultiCatalogLookup.LookupAll("editor-extension", "publisher.name")
        │       └→ []CatalogMatch (corroboration semantics, Phase 2)
        │
        ├→ internal/catalog.FetchPublishAge (marketplace adapter) [NEW in internal/catalog/]
        │       └→ cache-first: Open VSX / VS Code Marketplace REST APIs, 24h TTL
        │
        └→ policy.EvaluateReleaseAge / policy.Evaluate  [existing + extended]
                └→ Decision → audit record (EDXT-01 rule_ids) → exit 0/1


LAYER 2: File-Watcher Daemon (beekeeper watch)
───────────────────────────────────────────────
  beekeeper watch (foreground, SIGTERM to stop)
        │
        ▼
  internal/watch/watcher.go
        │
        ├→ fsnotify.Watcher
        │       ├→ watcher.Add("~/.vscode/extensions/")   [if dir exists]
        │       ├→ watcher.Add("~/.cursor/extensions/")   [if dir exists]
        │       └→ watcher.Add("~/.windsurf/extensions/") [if dir exists]
        │
        │  event loop (goroutine)
        │       │
        │       ├→ filter: Create events only (+ Rename on Linux/macOS)
        │       ├→ filter: immediate child of watched dir only (depth=1)
        │       │
        │       └→ debounce (500ms time.AfterFunc) → handleNewExtension()
        │                       │
        │                       ├→ os.ReadFile("<dir>/package.json") → parse manifest
        │                       ├→ internal/catalog.MultiCatalogLookup.LookupAll(...)
        │                       ├→ internal/catalog.FetchMarketplaceAge(...)
        │                       ├→ policy.Evaluate() / policy.EvaluateReleaseAge()
        │                       │
        │                       ├→ [on clean]: internal/audit.Write (allow record, EDXT-02)
        │                       │
        │                       └→ [on hit/block]:
        │                               ├→ internal/audit.Write (critical record, EDXT-03)
        │                               ├→ internal/notify.Notify() [beeep wrapper]
        │                               └→ internal/quarantine.Move() → ~/.beekeeper/quarantine/extensions/
        │
        │  retry loop (goroutine, 30s poll)
        │       └→ retry watcher.Add() for directories that didn't exist at start


LAYER 3: Scan Orchestrator (beekeeper scan)
────────────────────────────────────────────
  beekeeper scan [--deep]
        │
        ├→ internal/scan/scanner.go
        │       ├→ exec.LookPath("bumblebee") [check if installed]
        │       │       ├→ found: exec.CommandContext → "bumblebee scan [--profile deep]"
        │       │       │       └→ read stdout NDJSON → merge into unified stream
        │       │       └→ not found: log bumblebee_unavailable:true, continue
        │       │
        │       └→ Beekeeper-own scan:
        │               ├→ scan ~/.vscode/extensions/, ~/.cursor/extensions/, ~/.windsurf/extensions/
        │               ├→ for each extension dir: parse package.json → catalog match + release-age
        │               └→ emit NDJSON records to stdout + audit log (EDXT-04 rule_ids)
        │
        └→ unified NDJSON stream → stdout (tee to audit log)


QUARANTINE MANAGEMENT (beekeeper quarantine)
─────────────────────────────────────────────
  ~/.beekeeper/quarantine/extensions/
        ├── publisher.name-version-<timestamp>/
        │       ├── [extension files]
        │       └── beekeeper-manifest.json  ← original_path + reason + timestamp + catalog provenance
        └── ...

  beekeeper quarantine list   → reads beekeeper-manifest.json per item → table
  beekeeper quarantine restore <id> → os.Rename(<quarantine>/<id>, original_path)
  beekeeper quarantine purge  → confirm prompt → os.RemoveAll per item
```

### Recommended Project Structure (Phase 3 additions)

```
internal/
  watch/
    watcher.go          # Watch() daemon loop, event handling, debounce, retry
    watcher_test.go     # table-driven tests with fake fsnotify events
    testdata/
      valid-extension/  # synthetic package.json for test fixtures
      malicious-extension/
  notify/
    notify.go           # Notify() wrapper around beeep.Notify(); configurable on/off
    notify_test.go
  scan/
    scanner.go          # Scanner: exec Bumblebee + Beekeeper-own scan, NDJSON merge
    scanner_test.go     # stub exec.LookPath/Command; verify merge logic
  quarantine/
    quarantine.go       # Move(), List(), Restore(), Purge(); manifest read/write
    quarantine_test.go  # table-driven; tempdir-based
  catalog/
    marketplace.go      # NEW: MarketplaceAdapter (Open VSX + VS Code Marketplace), cache-first
    marketplace_test.go # httptest.Server stubs
  policy/
    engine.go           # EXTENDED: extractExtensionInstall() in installPrefixes
    engine_test.go      # EXTENDED: TestExtensionInstallIntercept
cmd/beekeeper/
  main.go               # EXTENDED: add watch, scan, quarantine subcommands; extend init
~/.beekeeper/
  quarantine/
    extensions/         # quarantine items (created by EDXT-03 handler)
```

### Pattern 1: Extension Install Command Recognition (Pure Policy Extension)

**What:** Extend the existing `installPrefixes` table in `internal/policy/engine.go` with editor install command patterns. The `extractExtensionInstall` function is a new pure function that parses `code --install-extension publisher.name@version` into a `(ecosystem, pkg, version)` tuple.

**When to use:** All four editor CLIs; bulk forms (multiple `--install-extension` flags on one command line).

```go
// Source: [ASSUMED] — extends existing extractFromCommand pattern in engine.go

// editorInstallPatterns recognizes editor extension install commands.
// Must be checked BEFORE the generic installPrefixes table.
var editorInstallPatterns = []string{
    "code --install-extension",
    "code-insiders --install-extension",
    "cursor --install-extension",
    "windsurf --install-extension",
}

// extractExtensionInstall checks whether cmd is an editor extension install command.
// Returns (ecosystem="editor-extension", pkg="publisher.name", version, true)
// on match, or ("", "", "", false) if not an extension install.
// Handles bulk: "code --install-extension a.b --install-extension c.d" → first match only
// (each will be presented as a separate Bash invocation OR multi-install support is added).
func extractExtensionInstall(cmd string) (ecosystem, pkg, version string, ok bool) {
    lower := strings.ToLower(strings.TrimSpace(cmd))
    for _, pat := range editorInstallPatterns {
        idx := strings.Index(lower, pat)
        if idx < 0 {
            continue
        }
        rest := strings.TrimSpace(cmd[idx+len(pat):])
        token := firstPackageToken(rest) // existing helper: skip flags
        if token == "" {
            return "", "", "", false
        }
        name, ver := splitVersion(token) // existing helper: split at last "@"
        if name == "" {
            return "", "", "", false
        }
        return "editor-extension", normalize(name), ver, true
    }
    return "", "", "", false
}
```

**Integration point:** Call `extractExtensionInstall(cmd)` before the existing `extractFromCommand` in `engine.go extract()`. The function must be in `internal/policy` (pure — no I/O, no goroutines).

**Bulk form handling:** When a command contains multiple `--install-extension` flags, intercept the whole command and evaluate each publisher.name independently. In practice, the agent will normally call check per-command; bulk form evaluation can either evaluate all IDs and return the worst decision, or return the first block encountered.

### Pattern 2: File-Watcher Daemon with Non-Existent Directory Handling

**What:** The `beekeeper watch` daemon must handle directories that do not exist at startup (e.g., Cursor not installed). fsnotify returns an error on `watcher.Add()` for non-existent paths.

**Critical finding:** `fsnotify.Watcher.Add()` **requires the path to exist** at time of call. Attempting to add a non-existent path returns an OS-level "no such file or directory" error.

**Pattern:**

```go
// Source: [VERIFIED: fsnotify v1.10.1 docs — "Paths must exist on the filesystem"]
// internal/watch/watcher.go

// watchedDir tracks a single extension directory and whether it's currently watched.
type watchedDir struct {
    path    string
    active  bool
}

// Watch runs the beekeeper watch daemon. It watches all configured extension
// directories and handles extension installation events. Blocks until ctx is
// cancelled (SIGTERM/SIGINT).
func Watch(ctx context.Context, dirs []string, cfg WatchConfig, handler ExtensionHandler) error {
    w, err := fsnotify.NewWatcher()
    if err != nil {
        return fmt.Errorf("create watcher: %w", err)
    }
    defer w.Close()

    // Initial add: only directories that currently exist.
    pending := make([]watchedDir, 0, len(dirs))
    for _, dir := range dirs {
        expanded := expandHome(dir)
        if err := w.Add(expanded); err != nil {
            // Directory doesn't exist yet — watch for it via retry loop.
            pending = append(pending, watchedDir{path: expanded, active: false})
            log.Printf("watch: %q not found at startup, will retry", expanded)
            continue
        }
        log.Printf("watch: watching %q", expanded)
        pending = append(pending, watchedDir{path: expanded, active: true})
    }

    // Retry ticker: every 30s, attempt to add directories that weren't available.
    retryTicker := time.NewTicker(30 * time.Second)
    defer retryTicker.Stop()

    // Debounce map: path → pending timer. Reset on each new event within window.
    debounce := make(map[string]*time.Timer)

    for {
        select {
        case <-ctx.Done():
            return nil

        case <-retryTicker.C:
            for i, d := range pending {
                if d.active {
                    continue
                }
                if err := w.Add(d.path); err == nil {
                    log.Printf("watch: now watching %q", d.path)
                    pending[i].active = true
                }
            }

        case event, ok := <-w.Events:
            if !ok {
                return nil
            }
            if !shouldProcess(event) {
                continue
            }
            // Debounce: 500ms window to coalesce burst events from a single install.
            path := event.Name
            if t, exists := debounce[path]; exists {
                t.Stop()
            }
            debounce[path] = time.AfterFunc(500*time.Millisecond, func() {
                delete(debounce, path)
                handler.HandleNewExtension(ctx, path)
            })

        case err, ok := <-w.Errors:
            if !ok {
                return nil
            }
            log.Printf("watch: fsnotify error: %v", err)
        }
    }
}

// shouldProcess returns true when the event represents a new extension directory
// appearing at depth=1 under a watched extensions root.
// On Windows: Create events only (Write events fire for parent dir metadata updates).
// On macOS/Linux: Create and Rename events (atomic installs use rename).
func shouldProcess(event fsnotify.Event) bool {
    if runtime.GOOS == "windows" {
        return event.Has(fsnotify.Create)
    }
    return event.Has(fsnotify.Create) || event.Has(fsnotify.Rename)
}
```

**Key constraint:** The debounce map access must be confined to a single goroutine (the event loop). The `time.AfterFunc` callback runs in a separate goroutine — use a channel or sync to avoid the map race.

### Pattern 3: Extension Manifest Parsing

**What:** Parse `<extension-dir>/package.json` to extract identity fields. VS Code extension directories are named `publisher.name-version` on disk.

**Extension directory naming convention (on disk):**
- Format: `<publisher>.<name>-<version>` (e.g., `ms-python.python-2026.4.0`)
- Also present: `.obsolete` file (JSON, lists extensions marked for deletion by VS Code) and `extensions.json` (registry of all installed extensions)
- A valid extension directory contains `package.json` at its root

```go
// Source: [VERIFIED: VS Code extension manifest docs + disk layout research]
// internal/watch/manifest.go

// ExtensionManifest holds the identity fields from a VS Code extension package.json.
// These fields are mandatory per the VS Code extension manifest spec.
type ExtensionManifest struct {
    Publisher   string `json:"publisher"`
    Name        string `json:"name"`
    Version     string `json:"version"`
    DisplayName string `json:"displayName"` // optional but present in all marketplace extensions
}

// ParseManifest reads and parses the package.json in extensionDir.
// Returns ErrNoManifest if the directory doesn't contain a package.json
// (used to filter .obsolete, extensions.json, and other non-extension entries).
func ParseManifest(extensionDir string) (ExtensionManifest, error) {
    manifestPath := filepath.Join(extensionDir, "package.json")
    data, err := os.ReadFile(manifestPath)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            return ExtensionManifest{}, ErrNoManifest
        }
        return ExtensionManifest{}, fmt.Errorf("read manifest %q: %w", manifestPath, err)
    }
    var m ExtensionManifest
    if err := json.Unmarshal(data, &m); err != nil {
        return ExtensionManifest{}, fmt.Errorf("parse manifest %q: %w", manifestPath, err)
    }
    if m.Publisher == "" || m.Name == "" {
        return ExtensionManifest{}, ErrNoManifest // not a real extension
    }
    return m, nil
}
```

### Pattern 4: Marketplace Timestamp Adapter (Open VSX)

**What:** Fetch extension publish timestamps from Open VSX Registry. The adapter follows the exact same cache-first pattern as OSV and Socket in Phase 2.

**Open VSX API endpoint:**
```
GET https://open-vsx.org/api/{publisher}/{name}/{version}
Response field: "timestamp" → "2026-03-13T04:21:53.228120Z"
```

**VS Code Marketplace API endpoint (fallback):**
```
POST https://marketplace.visualstudio.com/_apis/public/gallery/extensionquery/
Accept: application/json;api-version=3.0-preview.1
Body: {"filters":[{"criteria":[{"filterType":7,"value":"publisher.name"}]}],"flags":914}
Response path: results[0].extensions[0].versions[0].properties[] where key=="Microsoft.VisualStudio.Services.Links.Source"
Response publishedDate: results[0].extensions[0].publishedDate ("2019-05-02T18:40:34.66Z")
```

**Cursor and Windsurf use Open VSX** (confirmed: Cursor docs state "Cursor uses the Open VSX extension registry"). This means Open VSX is the primary endpoint for both Cursor and Windsurf extensions; VS Code Marketplace is the fallback for VS Code-native extensions not on Open VSX.

```go
// Source: [VERIFIED: Open VSX API live call returning "timestamp" field; Cursor docs confirming Open VSX use]
// internal/catalog/marketplace.go

const (
    openVSXBaseURL        = "https://open-vsx.org/api"
    vscodeMarketplaceURL  = "https://marketplace.visualstudio.com/_apis/public/gallery/extensionquery/"
    marketplaceAgeCacheTTL = 24 * time.Hour
)

// FetchMarketplaceAge fetches the publish timestamp for a VS Code extension.
// Cache key: "marketplace/<publisher>/<name>/<version>.json" under cacheDir/marketplace-cache/.
// Strategy: try Open VSX first (used by Cursor/Windsurf); fall back to VS Code Marketplace.
// Both fail → write Missing:true entry; caller treats as fail-closed block (EDXT-03).
func FetchMarketplaceAge(
    ctx context.Context,
    client *http.Client,
    cacheDir, publisher, name, version string,
    now time.Time,
) (ageMinutes int64, missing bool, err error) {
    // Cache-first (same pattern as FetchPublishAge in age_cache.go)
    path := marketplaceCachePath(cacheDir, publisher, name, version)
    if entry, ok := readAgeCacheEntry(path); ok {
        if now.Sub(entry.CachedAt) < marketplaceAgeCacheTTL {
            if entry.Missing { return 0, true, nil }
            return int64(now.Sub(entry.PublishedAt).Minutes()), false, nil
        }
    }

    // Try Open VSX first.
    tsStr, err := fetchOpenVSXTimestamp(ctx, client, publisher, name, version)
    if err != nil {
        // Fall back to VS Code Marketplace.
        tsStr, err = fetchVSCodeMarketplaceTimestamp(ctx, client, publisher, name)
    }
    if err != nil {
        _ = writeAgeCacheEntry(path, ageCacheEntry{CachedAt: now, Missing: true})
        return 0, true, nil
    }
    // Parse + cache + return (same as FetchPublishAge)
    // ...
}

// fetchOpenVSXTimestamp queries GET https://open-vsx.org/api/{publisher}/{name}/{version}
// Returns the "timestamp" field value.
func fetchOpenVSXTimestamp(ctx context.Context, client *http.Client, publisher, name, version string) (string, error) {
    url := fmt.Sprintf("%s/%s/%s/%s", openVSXBaseURL, publisher, name, version)
    var resp struct {
        Timestamp string `json:"timestamp"`
        Error     string `json:"error"`
    }
    if err := fetchRegistryJSON(ctx, client, url, &resp); err != nil {
        return "", err
    }
    if resp.Error != "" {
        return "", fmt.Errorf("open-vsx: %s", resp.Error)
    }
    if resp.Timestamp == "" {
        return "", fmt.Errorf("open-vsx: no timestamp for %s.%s@%s", publisher, name, version)
    }
    return resp.Timestamp, nil
}
```

**Rate limiting:** Open VSX has no documented rate limit for public metadata queries. VS Code Marketplace is similarly undocumented for public queries. The 24h TTL cache makes rate limiting a non-issue in practice for normal check rates.

### Pattern 5: Desktop Notification Wrapper

**What:** Thin `internal/notify/` wrapper around `beeep.Notify()` that is configurable on/off. Notifications are best-effort — failure to send must never affect the security decision.

```go
// Source: [VERIFIED: beeep v0.11.2 API from pkg.go.dev]
// internal/notify/notify.go

package notify

import "github.com/gen2brain/beeep"

// Config controls whether desktop notifications are sent.
type Config struct {
    Enabled bool // default true; set to false to silence all notifications
}

// Notify sends a desktop notification if enabled. Any error is logged but
// never propagated — notifications are best-effort and must not affect decisions.
func Notify(cfg Config, title, message string) {
    if !cfg.Enabled {
        return
    }
    // Ignore error: notification failure does not block security decisions.
    _ = beeep.Notify(title, message, nil)
}
```

**CGO status:** Confirmed CGO-free on all platforms. On Linux uses `godbus/dbus` (pure Go D-Bus bindings) or `notify-send` CLI fallback. On Windows uses `git.sr.ht/~jackmordaunt/go-toast` (pure Go COM API) with PowerShell fallback. On macOS uses `osascript`.

**Build tag option:** Build with `-tags nodbus` on Linux to exclude `godbus/dbus` and use only `notify-send`.

### Pattern 6: Quarantine Manager

**What:** Move extension directories to quarantine, preserving original path for restore. `beekeeper-manifest.json` is the metadata file written to each quarantine entry.

```go
// Source: [ASSUMED] — design from CONTEXT.md EDXT-05 + EDXT-03 decisions
// internal/quarantine/quarantine.go

// Manifest is the metadata file written to each quarantine entry.
// It preserves the original_path so restore can put it back.
type Manifest struct {
    ID           string          `json:"id"`            // directory basename (the quarantine ID)
    Publisher    string          `json:"publisher"`
    Name         string          `json:"name"`
    Version      string          `json:"version"`
    OriginalPath string          `json:"original_path"` // full path before quarantine
    QuarantinedAt time.Time     `json:"quarantined_at"`
    Reason       string          `json:"reason"`        // "catalog-match" | "release-age"
    RuleIDs      []string        `json:"rule_ids"`
    CatalogMatches []CatalogMatchSummary `json:"catalog_matches,omitempty"`
}

// Move atomically moves an extension directory to the quarantine directory and
// writes beekeeper-manifest.json. Returns the quarantine ID.
func Move(quarantineDir, extensionPath string, m Manifest) (string, error) {
    id := fmt.Sprintf("%s.%s-%s-%d", m.Publisher, m.Name, m.Version, time.Now().UnixNano())
    destDir := filepath.Join(quarantineDir, "extensions", id)

    if err := os.MkdirAll(filepath.Dir(destDir), 0o700); err != nil {
        return "", err
    }

    // os.Rename is atomic on POSIX; on Windows it is NOT atomic when crossing
    // volumes. Since source (extensions/) and quarantine (~/.beekeeper/quarantine/)
    // are on the same volume in the typical setup, os.Rename works.
    // If cross-volume: fall back to copy+delete.
    if err := rename(extensionPath, destDir); err != nil {
        return "", fmt.Errorf("quarantine move: %w", err)
    }

    m.ID = id
    m.OriginalPath = extensionPath
    manifestPath := filepath.Join(destDir, "beekeeper-manifest.json")
    data, _ := json.MarshalIndent(m, "", "  ")
    if err := os.WriteFile(manifestPath, data, 0o600); err != nil {
        return id, fmt.Errorf("write manifest: %w", err)
    }
    return id, nil
}
```

**Cross-volume rename on Windows:** `os.Rename` fails across volumes on Windows. The quarantine directory `~/.beekeeper/quarantine/` is typically on the same drive as `%USERPROFILE%` — same volume as extensions. Cross-volume is an edge case (portable mode); document it as unsupported in Phase 3.

### Pattern 7: Bumblebee Scan Orchestration

**What:** Invoke Bumblebee CLI as a subprocess, read NDJSON from stdout, merge with Beekeeper-own results.

**Bumblebee CLI confirmed API:**
```bash
# Standard profile scan (scans global packages, extension dirs, etc.)
bumblebee scan

# Deep scan
bumblebee scan --profile deep --root ~/

# NDJSON output fields (record_type: "package" or "finding"):
# package: record_type, record_id, schema_version, scanner_name, ecosystem, package_name, version, source_file, confidence
# finding: record_type, record_id, finding_type, severity, catalog_id, ecosystem, package_name, version, evidence
```

**Note:** There is no `--format ndjson` flag — Bumblebee outputs NDJSON by default. The CONTEXT.md reference to `--format ndjson` should be treated as `bumblebee scan [--profile deep]` with stdout NDJSON output.

```go
// Source: [VERIFIED: github.com/perplexityai/bumblebee README + CLI schema]
// internal/scan/scanner.go

// runBumblebee invokes the bumblebee CLI and returns a channel of raw NDJSON lines.
// Returns (nil channel, false) if bumblebee is not installed.
func runBumblebee(ctx context.Context, deep bool) (<-chan []byte, bool) {
    beeKeeper, err := exec.LookPath("bumblebee")
    if err != nil {
        return nil, false // not installed — degrade gracefully
    }

    args := []string{"scan"}
    if deep {
        args = append(args, "--profile", "deep")
    }

    cmd := exec.CommandContext(ctx, beeKeeper, args...)
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return nil, false
    }
    if err := cmd.Start(); err != nil {
        return nil, false
    }

    ch := make(chan []byte, 100)
    go func() {
        defer close(ch)
        scanner := bufio.NewScanner(stdout)
        for scanner.Scan() {
            ch <- scanner.Bytes()
        }
        _ = cmd.Wait()
    }()
    return ch, true
}
```

### Pattern 8: JSONC-Safe Settings.json Edit (EDXT-06)

**What:** Write `"extensions.autoUpdate": false` to VS Code/Cursor user settings JSON without destroying existing content.

**Settings file paths:**
- VS Code (Linux/macOS): `~/.config/Code/User/settings.json`
- VS Code (Windows): `%APPDATA%\Code\User\settings.json`
- Cursor (Linux/macOS): `~/.config/Cursor/User/settings.json`
- Cursor (Windows): `%APPDATA%\Cursor\User\settings.json`
- Windsurf (Linux/macOS): `~/.config/Windsurf/User/settings.json`

**JSONC limitation:** `tidwall/jsonc.ToJSON()` strips comments for reading, but there is no JSONC writer that preserves comments. Two approaches:

**Approach A (recommended for simplicity):** Strip comments on read, unmarshal to `map[string]any`, set key, marshal to standard JSON (comments lost). Warn user in consent prompt.

**Approach B (comment-preserving):** Use a targeted line-search: find the closing `}` of the JSON and insert the key before it (raw string manipulation). Only safe for the simple case of adding a top-level key. Fragile — not recommended.

**Approach A implementation:**

```go
// Source: [VERIFIED: tidwall/jsonc API — ToJSON strips comments; json.Unmarshal then standard]
// In cmd/beekeeper/main.go (init command extension)

func patchEditorSettings(path string, key string, value any) error {
    existing := []byte("{}")
    if data, err := os.ReadFile(path); err == nil {
        existing = jsonc.ToJSON(data) // strips comments — documented limitation
    }

    var settings map[string]any
    if err := json.Unmarshal(existing, &settings); err != nil {
        return fmt.Errorf("parse settings %q: %w", path, err)
    }
    if settings == nil {
        settings = make(map[string]any)
    }
    settings[key] = value

    out, err := json.MarshalIndent(settings, "", "    ")
    if err != nil {
        return fmt.Errorf("marshal settings: %w", err)
    }

    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        return err
    }
    return writeFileAtomic(path, out)
}
```

**Consent prompt text (required):** "This will update your {Editor} settings.json to disable extension auto-update. NOTE: existing comments in settings.json will be removed. Proceed? [y/N]"

### Anti-Patterns to Avoid

- **Anti-pattern: Calling watcher.Add() on non-existent paths.** fsnotify returns an error and the path is never watched. Always check existence first; use a retry loop for paths that may appear later. [VERIFIED: fsnotify docs]

- **Anti-pattern: Watching parent directories instead of extension dirs directly.** Extension installs appear as new child directories of `~/.vscode/extensions/`. Watching the parent (`~/.vscode/`) generates too many unrelated events. Watch `~/.vscode/extensions/` directly.

- **Anti-pattern: Ignoring Windows `Write` events on extension directories.** On Windows NTFS, when a child directory is created inside a watched directory, the parent receives both `Create` (new child) and `Write` (parent metadata updated) events. Filter to `Create` only on Windows or the handler fires twice. [VERIFIED: fsnotify Windows docs]

- **Anti-pattern: Treating `beeep.Notify()` error as a fatal error.** Notification failure (notification daemon not running, no display, etc.) must never block the security decision. Wrap in best-effort pattern. [CITED: CONTEXT.md EDXT-03]

- **Anti-pattern: Setting `beekeeper-manifest.json` as the quarantine ID.** The quarantine ID must be the directory basename (e.g., `ms-python.python-2026.4.0-1716739234567890123`), not the manifest file path. [CITED: CONTEXT.md EDXT-05]

- **Anti-pattern: Hardcoding the Bumblebee binary name.** Use `exec.LookPath("bumblebee")` to find the binary; degrade gracefully with `bumblebee_unavailable: true` in the audit record. [CITED: CONTEXT.md EDXT-04]

- **Anti-pattern: Importing Bumblebee as a Go library.** Bumblebee is CLI-only (`go install` distribution model); it has no importable library API. All integration is via `exec.CommandContext`. [VERIFIED: perplexityai/bumblebee README]

- **Anti-pattern: Using recursive fsnotify watching.** fsnotify v1.10.1 does not expose a public recursive API. Use explicit `watcher.Add()` per directory. [VERIFIED: fsnotify docs]

- **Anti-pattern: Calling `I/O` inside `internal/policy` for extension recognition.** The `extractExtensionInstall` function must be pure. No file reads, no HTTP, no `os.Stat`. [CITED: CLAUDE.md pure-function constraint]

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cross-platform filesystem events | OS-specific inotify/kqueue/ReadDirectoryChangesW syscall wrappers | `fsnotify` v1.10.1 | Already in go.mod; cross-platform; handles Windows NTFS quirks |
| Desktop notifications | OS-specific AppleScript / PowerShell / D-Bus invocation | `gen2brain/beeep` v0.11.2 | Handles all three platforms; CGO-free; falls back gracefully |
| JSONC comment stripping | Custom comment-stripping regex | `tidwall/jsonc` v0.3.3 | Zero deps; handles nested comments and strings containing comment chars |
| Event debouncing | Manual goroutine with channel timeout | `time.AfterFunc` (stdlib) | 15 lines of stdlib; no external dep justified |
| Extension publish timestamp lookup | Custom date-string scraping | Open VSX REST API + VS Code Marketplace API | Documented, stable REST APIs; same cache-first pattern as Phase 2 |

**Key insight:** Phase 3 is a consumer phase — all the hard infrastructure (corroboration engine, release-age policy, audit logging, cache infrastructure) is already built in Phases 1 and 2. Phase 3 primarily adds new event sources (file system, CLI pattern) and new output channels (notifications, quarantine) that feed into existing systems.

---

## Common Pitfalls

### Pitfall 1: fsnotify Cannot Watch Non-Existent Directories

**What goes wrong:** `watcher.Add("~/.cursor/extensions/")` returns "no such file or directory" on machines where Cursor is not installed. The directory never gets watched, and the error is silently swallowed if the startup code doesn't handle it.

**Why it happens:** fsnotify requires the path to exist at time of `Add()`. This is a documented fundamental limitation.

**How to avoid:** Check path existence before `Add()`; log a warning and add to a retry list when missing; retry every 30 seconds in a background goroutine.

**Warning signs:** `beekeeper watch` starts without error but no events are received for Cursor extension directories; no log message indicating the directory was not found.

### Pitfall 2: Windows NTFS Generates `Write` Events on Parent Directory

**What goes wrong:** When VS Code installs a new extension into `~/.vscode/extensions/`, Windows NTFS emits: (1) `Create` for the new extension directory, and (2) `Write` for the parent extensions directory (metadata update). The handler fires twice, causing duplicate catalog lookups and potentially two quarantine moves for the same extension.

**Why it happens:** NTFS updates the last-write-time of a directory when its contents change. fsnotify requests `FILE_NOTIFY_CHANGE_LAST_WRITE` for the parent directory and receives the extra `Write` event.

**How to avoid:** On Windows, filter to `Create` events only using `runtime.GOOS == "windows"` check in `shouldProcess()`. The debounce timer also partially mitigates this.

**Warning signs:** Duplicate audit records for the same extension install on Windows; "already quarantined" errors when trying to move a directory that was already moved.

### Pitfall 3: Extension Directory Name vs. Extension ID Mismatch

**What goes wrong:** The directory name on disk (`ms-python.python-2026.4.0`) is used as the package identifier for catalog lookup, but the catalog stores entries keyed by `publisher.name` (without version). Passing the directory name directly returns no catalog match.

**Why it happens:** VS Code's disk naming convention appends `-<version>` to the `publisher.name` format. The catalog lookup key is `publisher.name` (no version); the version is passed separately.

**How to avoid:** Always parse `package.json` to extract `publisher`, `name`, and `version` as separate fields. Never split the directory name; `package.json` is authoritative.

**Warning signs:** Zero catalog matches even for known-malicious extensions; "no catalog match" audit records for extensions that should trigger.

### Pitfall 4: Open VSX `timestamp` Field Is Last-Updated, Not Published

**What goes wrong:** Open VSX's `timestamp` field represents the last time the extension metadata was updated on Open VSX, not the original publication date on VS Code Marketplace. An extension published years ago may have a recent `timestamp` if it was re-synced to Open VSX.

**Why it happens:** Open VSX mirrors extensions from VS Code Marketplace. The `timestamp` reflects the sync/update time on Open VSX, not the original author's publish date.

**How to avoid:** For release-age policy, accept this limitation. The `timestamp` from Open VSX is the best available signal without scraping VS Code Marketplace for every extension. Document this as a known limitation: Open VSX timestamp may be newer than the actual first-publish date, so release-age blocks may be less effective for extensions that were recently re-synced. For the Nx Console attack vector (18-minute window), a freshly published extension would have a fresh Open VSX timestamp regardless.

**Warning signs:** Extension published months ago triggers a release-age block because it was recently re-synced to Open VSX.

### Pitfall 5: Quarantine `os.Rename` Fails Across Volumes on Windows

**What goes wrong:** `os.Rename("C:\\Users\\user\\.vscode\\extensions\\ext-dir", "D:\\beekeeper\\quarantine\\extensions\\ext-dir")` fails with "invalid cross-device link" if the quarantine directory is on a different drive.

**Why it happens:** `os.Rename` on Windows is implemented as `MoveFileExW(MOVEFILE_REPLACE_EXISTING)`, which fails across volumes.

**How to avoid:** Detect cross-volume case (different drive letter prefix) and fall back to copy+delete. In Phase 3, document as unsupported (quarantine directory must be on same drive as extensions). The typical case (user home on C:, quarantine in `%APPDATA%\beekeeper\quarantine\`) is same-volume.

**Warning signs:** Quarantine operations fail on machines with extensions on a different drive than `%APPDATA%`.

### Pitfall 6: `beeep` Notify on Linux Fails When No Display Server Is Running

**What goes wrong:** `beeep.Notify()` returns an error when run in a non-GUI context (SSH session, CI, headless server). On Linux, D-Bus notification requires a running desktop session.

**Why it happens:** `notify-send` and D-Bus both require a running notification daemon. In headless environments, neither is available.

**How to avoid:** Always treat `beeep.Notify()` error as best-effort; log the error to stderr but never propagate it. Notification failure is expected in headless environments. Consider checking for `DISPLAY` or `WAYLAND_DISPLAY` env vars before attempting notification.

**Warning signs:** `beekeeper watch` exits on headless Linux servers because notification error is treated as fatal.

### Pitfall 7: `beekeeper scan` Bumblebee Output Has No `--format ndjson` Flag

**What goes wrong:** Running `bumblebee scan --format ndjson` returns a "unknown flag" error; `beekeeper scan` subprocess fails immediately.

**Why it happens:** Bumblebee outputs NDJSON by default. There is no `--format` flag. The CONTEXT.md reference to `--format ndjson` is an artifact of the PRD drafting process — the actual CLI was confirmed without this flag.

**How to avoid:** Use `bumblebee scan [--profile deep]` only. Read stdout directly as NDJSON.

**Warning signs:** `beekeeper scan` always logs "bumblebee error: exit status 1" even when Bumblebee is installed.

---

## Code Examples

### Extension Install Command Recognition

```go
// Source: [ASSUMED] — extends existing installPrefixes in internal/policy/engine.go
// Pure function; no I/O. Follows the existing extractFromCommand pattern.

// In internal/policy/engine.go — add to extract() before existing command check:
func extract(input map[string]any) (ecosystem, pkg, version string, ok bool) {
    if input == nil {
        return "", "", "", false
    }
    // Direct shape (unchanged from Phase 2).
    if eco, pkgRaw, ok2 := directPackage(input); ok2 {
        ver, _ := input["version"].(string)
        return eco, normalize(pkgRaw), strings.TrimSpace(ver), true
    }
    // Command shape.
    if cmd, ok2 := input["command"].(string); ok2 {
        // Check editor extension patterns first (more specific).
        if eco, pkg, ver, ok3 := extractExtensionInstall(cmd); ok3 {
            return eco, pkg, ver, true
        }
        return extractFromCommand(cmd)
    }
    return "", "", "", false
}

// extractExtensionInstall parses editor extension install commands.
// Returns ecosystem="editor-extension" on match.
func extractExtensionInstall(cmd string) (ecosystem, pkg, version string, ok bool) {
    lower := strings.ToLower(strings.TrimSpace(cmd))
    patterns := []string{
        "code --install-extension ",
        "code-insiders --install-extension ",
        "cursor --install-extension ",
        "windsurf --install-extension ",
    }
    for _, pat := range patterns {
        idx := strings.Index(lower, pat)
        if idx < 0 {
            continue
        }
        rest := strings.TrimSpace(cmd[idx+len(pat):])
        token := firstPackageToken(rest)
        if token == "" {
            continue
        }
        name, ver := splitVersion(token)
        if name == "" {
            continue
        }
        return "editor-extension", normalize(name), ver, true
    }
    return "", "", "", false
}
```

### Open VSX Timestamp Fetch (Verified Working)

```go
// Source: [VERIFIED: Open VSX API call returning "timestamp" field for ms-python/python/2026.4.0]
// internal/catalog/marketplace.go

type openVSXResponse struct {
    Timestamp string `json:"timestamp"` // "2026-03-13T04:21:53.228120Z"
    Error     string `json:"error"`     // set on error responses
}

func fetchOpenVSXTimestamp(ctx context.Context, client *http.Client, publisher, name, version string) (string, error) {
    url := fmt.Sprintf("https://open-vsx.org/api/%s/%s/%s", publisher, name, version)
    var resp openVSXResponse
    if err := fetchRegistryJSON(ctx, client, url, &resp); err != nil {
        return "", err
    }
    if resp.Error != "" {
        return "", fmt.Errorf("open-vsx error: %s", resp.Error)
    }
    if resp.Timestamp == "" {
        return "", fmt.Errorf("open-vsx: no timestamp for %s.%s@%s", publisher, name, version)
    }
    return resp.Timestamp, nil  // parse via time.Parse(time.RFC3339Nano, resp.Timestamp)
}
```

### Quarantine Manifest Structure

```go
// Source: [ASSUMED] — design from CONTEXT.md EDXT-05

// beekeeper-manifest.json written at: ~/.beekeeper/quarantine/extensions/<id>/beekeeper-manifest.json
type QuarantineManifest struct {
    ID            string    `json:"id"`             // e.g., "ms-python.python-2026.4.0-1716739234"
    Publisher     string    `json:"publisher"`
    Name          string    `json:"name"`
    Version       string    `json:"version"`
    DisplayName   string    `json:"display_name"`
    OriginalPath  string    `json:"original_path"`  // CRITICAL: restore target
    QuarantinedAt time.Time `json:"quarantined_at"`
    Reason        string    `json:"reason"`         // "catalog-match" | "release-age"
    RuleIDs       []string  `json:"rule_ids"`
    AuditRecordID string    `json:"audit_record_id"` // links to NDJSON audit record
}
```

### Debounce Pattern (Stdlib-Only)

```go
// Source: [VERIFIED: standard Go debounce pattern using time.AfterFunc]
// internal/watch/watcher.go

// debounceMap is confined to the watcher goroutine (no mutex needed).
type debounceMap struct {
    timers map[string]*time.Timer
}

func (d *debounceMap) reset(key string, delay time.Duration, fn func()) {
    if t, ok := d.timers[key]; ok {
        t.Stop()
    }
    d.timers[key] = time.AfterFunc(delay, func() {
        // NOTE: AfterFunc callback runs in a NEW goroutine.
        // Pass path as parameter to avoid closure over loop variable.
        fn()
    })
}

// Usage in event loop (single goroutine — no mutex needed on debounceMap):
const debounceDelay = 500 * time.Millisecond

case event, ok := <-w.Events:
    if !ok { return nil }
    if !shouldProcess(event) { continue }
    // Capture path before AfterFunc callback executes.
    p := event.Name
    dm.reset(p, debounceDelay, func() { handleExt(ctx, p) })
```

---

## Runtime State Inventory

This phase introduces new runtime state that is NOT file-based and must be explicitly tracked.

| Category | Items Introduced | Action Required |
|----------|-----------------|-----------------|
| Disk directories | `~/.beekeeper/quarantine/extensions/` (created on first quarantine event), `~/.beekeeper/catalogs/marketplace-cache/` (created by marketplace adapter on first lookup) | `beekeeper init` should create these; first-run code creates on demand |
| Live service config | None — `beekeeper watch` is a foreground process (no service registration in Phase 3) | None |
| OS-registered state | None (Phase 3 watch daemon is foreground only; no systemd/launchd/Windows Service registration) | None |
| Secrets / env vars | None — marketplace APIs are public; no auth tokens required for Open VSX | None |
| Build artifacts | None — no new binaries beyond the existing `beekeeper` binary | None |

**Phase 2 state unchanged:** All Phase 2 state (OSV cache, Socket cache, age cache, baselines, state.json) is unaffected by Phase 3.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `api.socket.dev/v0/purl` | Phase 2 (existing) | ✓ (requires token) | — | Degrade gracefully (unchanged from Phase 2) |
| `open-vsx.org/api` | EDXT-03 marketplace timestamps | ✓ (public, no auth) | — | Fall back to VS Code Marketplace API |
| `marketplace.visualstudio.com/_apis/public/gallery/extensionquery/` | EDXT-03 fallback | ✓ (public, no auth for basic queries) | — | If both fail: treat as missing timestamp, fail-closed |
| `bumblebee` CLI | EDXT-04 | NOT installed by default | — | Degrade: Beekeeper-only scan, log `bumblebee_unavailable: true` |
| Go toolchain | All builds | ✓ (Phase 1 verified) | 1.25 | — |
| `fsnotify` v1.10.1 | EDXT-02 | ✓ (in go.mod from Phase 2) | v1.10.1 | — |
| `gen2brain/beeep` | EDXT-03 notifications | ✓ (go get) | v0.11.2 | If install fails: skip notifications, document as degraded |
| `tidwall/jsonc` | EDXT-06 settings edit | ✓ (go get) | v0.3.3 | — |

**Missing dependencies with no fallback:** None (all critical paths degrade gracefully).

**Missing dependencies with fallback:** Bumblebee CLI (most critical — EDXT-04 degrades to Beekeeper-only scan without it).

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go standard `testing` package + `go test` |
| Config file | None — `go test ./...` discovers automatically |
| Quick run command | `go test ./internal/... -count=1` |
| Full suite command | `go test -race -count=1 ./...` (CI-only; requires CGO) |

### Phase Requirements to Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| EDXT-01 | `code --install-extension ms-python.python@2026.4.0` → ecosystem="editor-extension", pkg="ms-python.python" | unit | `go test ./internal/policy/... -run TestExtensionInstallExtract -v` | ❌ Wave 0 |
| EDXT-01 | Bulk form: `code --install-extension a.b --install-extension c.d` → intercept | unit | `go test ./internal/policy/... -run TestExtensionInstallBulk -v` | ❌ Wave 0 |
| EDXT-01 | cursor/windsurf/code-insiders variants recognized | unit | `go test ./internal/policy/... -run TestExtensionInstallVariants -v` | ❌ Wave 0 |
| EDXT-02 | Watch daemon starts; skips non-existent directory without crashing | unit | `go test ./internal/watch/... -run TestWatchNonExistentDir -v` | ❌ Wave 0 |
| EDXT-02 | Create event triggers handleNewExtension with correct path | unit | `go test ./internal/watch/... -run TestWatchCreateEvent -v` | ❌ Wave 0 |
| EDXT-02 | Windows: Write events on watched dir are filtered (Create-only) | unit | `go test ./internal/watch/... -run TestWatchWindowsFilter -v` | ❌ Wave 0 |
| EDXT-02 | Debounce: burst of 10 Create events → one handleNewExtension call | unit | `go test ./internal/watch/... -run TestWatchDebounce -v` | ❌ Wave 0 |
| EDXT-03 | ParseManifest returns correct publisher/name/version from package.json | unit | `go test ./internal/watch/... -run TestParseManifest -v` | ❌ Wave 0 |
| EDXT-03 | ParseManifest returns ErrNoManifest for .obsolete and non-extension dirs | unit | `go test ./internal/watch/... -run TestParseManifestNonExtension -v` | ❌ Wave 0 |
| EDXT-03 | Catalog hit → quarantine move + audit record emitted | integration | `go test ./internal/watch/... -run TestHandleNewExtensionCatalogHit -v` | ❌ Wave 0 |
| EDXT-03 | FetchMarketplaceAge returns correct ageMinutes for known extension | unit (httptest) | `go test ./internal/catalog/... -run TestFetchMarketplaceAge -v` | ❌ Wave 0 |
| EDXT-03 | FetchMarketplaceAge 24h cache hit skips HTTP | unit | `go test ./internal/catalog/... -run TestMarketplaceAgeCacheHit -v` | ❌ Wave 0 |
| EDXT-04 | `beekeeper scan` runs Bumblebee when installed; merges NDJSON | integration | `go test ./internal/scan/... -run TestScanWithBumblebee -v` | ❌ Wave 0 |
| EDXT-04 | `beekeeper scan` degrades gracefully when Bumblebee not in PATH | unit | `go test ./internal/scan/... -run TestScanBumblebeeUnavailable -v` | ❌ Wave 0 |
| EDXT-05 | `quarantine list` parses all beekeeper-manifest.json files | unit | `go test ./internal/quarantine/... -run TestQuarantineList -v` | ❌ Wave 0 |
| EDXT-05 | `quarantine restore <id>` moves extension back to original_path | unit | `go test ./internal/quarantine/... -run TestQuarantineRestore -v` | ❌ Wave 0 |
| EDXT-05 | `quarantine purge --yes` removes all items; emits audit records | unit | `go test ./internal/quarantine/... -run TestQuarantinePurge -v` | ❌ Wave 0 |
| EDXT-06 | patchEditorSettings writes `extensions.autoUpdate:false` without destroying other keys | unit | `go test ./... -run TestPatchEditorSettings -v` | ❌ Wave 0 |
| EDXT-06 | patchEditorSettings is idempotent (re-running doesn't duplicate key) | unit | `go test ./... -run TestPatchEditorSettingsIdempotent -v` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/... -count=1` (< 30 seconds on Windows)
- **Per wave merge:** `go test -race -count=1 ./...` (CI-only)
- **Phase gate:** Full test suite green on all 3 platforms before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/policy/engine_test.go` — extend: TestExtensionInstallExtract, TestExtensionInstallBulk, TestExtensionInstallVariants
- [ ] `internal/watch/watcher_test.go` — TestWatchNonExistentDir, TestWatchCreateEvent, TestWatchWindowsFilter, TestWatchDebounce
- [ ] `internal/watch/manifest_test.go` — TestParseManifest, TestParseManifestNonExtension
- [ ] `internal/watch/handler_test.go` — TestHandleNewExtensionCatalogHit (integration with tempdir)
- [ ] `internal/catalog/marketplace_test.go` — TestFetchMarketplaceAge, TestMarketplaceAgeCacheHit (httptest.Server stubs)
- [ ] `internal/scan/scanner_test.go` — TestScanWithBumblebee, TestScanBumblebeeUnavailable
- [ ] `internal/quarantine/quarantine_test.go` — TestQuarantineList, TestQuarantineRestore, TestQuarantinePurge
- [ ] `internal/watch/testdata/valid-extension/package.json` — synthetic fixture
- [ ] `internal/watch/testdata/malicious-extension/package.json` — synthetic fixture

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | Single-user local binary |
| V3 Session Management | No | Stateless; watch daemon has no sessions |
| V4 Access Control | Yes — quarantine directory, marketplace cache | `0700` dirs + `0600` files via `platform.SetOwnerOnly` (same as Phase 2) |
| V5 Input Validation | Yes — extension manifest parsing, Bumblebee NDJSON lines | `json.Unmarshal` with sized limit; manifest field validation (empty publisher/name rejection) |
| V6 Cryptography | No new crypto | — |
| V7 Error Handling | Yes — all new I/O paths | Fail-closed on marketplace timestamp unavailability; best-effort for notifications |

### Known Threat Patterns for Phase 3 Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Malicious `package.json` with path traversal in publisher/name fields | Tampering | `filepath.Base()` sanitization on all attacker-controlled fields used in path construction; reject empty publisher/name |
| Extension directory named `../../etc/passwd` to escape quarantine dir | Tampering | Use only `filepath.Base(extensionDir)` as the quarantine ID component; verify quarantine path stays under quarantine root before rename |
| Marketplace timestamp poisoning via MITM | Tampering | HTTPS; 24h TTL cache; missing timestamp → fail-closed block (conservative) |
| fsnotify event injection via symlink | Tampering | Verify event.Name is a real child of the watched directory (filepath.Dir check) before processing |
| Bumblebee subprocess stdout injection (compromised bumblebee binary) | Tampering | `exec.LookPath` finds binary in PATH — document that PATH integrity is assumed; fail-closed if NDJSON parse fails |
| Quarantine bypass via directory rename after Create event but before Move | TOCTOU | Window is milliseconds; acceptable for Phase 3; Phase 5 Sentry provides deeper process-level protection |
| Notification spam (100 extensions installed rapidly) | DoS | Debounce reduces to one notification per extension; notifications are best-effort |

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Extension scanning only at install time (via package manager hook) | File-system event watching detects GUI installs and auto-updates | Phase 3 design | Closes the GUI install gap — the exact vector in the Nx Console attack |
| Single CLI check (blocking) for package installs | Three-layer defense: CLI intercept + file-watcher + scheduled scan | Phase 3 design | Defense in depth; CLI intercept can be bypassed by direct GUI use; watcher catches it |
| Bumblebee used standalone | `beekeeper scan` orchestrates Bumblebee + applies Beekeeper policies on top | Phase 3 | Unified NDJSON output stream; Beekeeper catalog-delta triggers automated scan |

**Deprecated/outdated:**
- `bumblebee scan --format ndjson`: This flag does not exist; Bumblebee outputs NDJSON by default. Do not include `--format ndjson` in any plan task.

---

## Open Questions

1. **Cursor extension directory path on Windows**
   - What we know: Cursor is based on VS Code; it uses Open VSX; community forum posts confirm `~/.cursor/extensions/` on Linux/macOS.
   - What's unclear: Windows path — is it `%USERPROFILE%\.cursor\extensions\` or `%APPDATA%\Cursor\extensions\`?
   - Recommendation: Default to `%USERPROFILE%\.cursor\extensions\` (mirrors VS Code's `%USERPROFILE%\.vscode\extensions\` convention) and document as needing empirical validation on Windows. A `beekeeper init` detection step can discover the actual path by checking both locations.

2. **VS Code extension directory on Windows: junction point behavior**
   - What we know: Some users create symlinks/junctions to redirect `~/.vscode/extensions/` to another drive. fsnotify v1.10.1 uses `ReadDirectoryChangesW` which follows junctions transparently.
   - What's unclear: Whether fsnotify correctly reports events when the extensions dir itself is a junction pointing elsewhere. Phase 2 research flagged this as needing live testing.
   - Recommendation: Document as a known limitation; add a `beekeeper watch --test` mode that creates and detects a synthetic event. If the junction-point case is common in production, add a junction-detection step to `beekeeper init` that warns the user.

3. **Bumblebee scan NDJSON record_type field for findings vs. packages**
   - What we know: Bumblebee outputs records with `record_type: "package"` and `record_type: "finding"`. The finding records include `severity`, `catalog_id`, and `evidence` fields.
   - What's unclear: Whether the `record_type` field is consistently spelled this way across all Bumblebee versions, and whether additional record types exist.
   - Recommendation: Parse `record_type` from each line; pass through any unknown record types unmodified to the unified NDJSON stream (future-proof).

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Cursor extension directory on Windows is `%USERPROFILE%\.cursor\extensions\` (mirroring VS Code convention) | Pattern 2, EDXT-02 | If Cursor uses `%APPDATA%\Cursor\extensions\`, the watch daemon misses Cursor extensions on Windows; `beekeeper init` detection must check both |
| A2 | Open VSX `timestamp` field is ISO 8601 format parseable by `time.RFC3339Nano` | Pattern 4 | If format differs, timestamp parse fails → treated as missing → fail-closed block on all extensions |
| A3 | Bumblebee outputs NDJSON by default with no `--format` flag required | Pattern 7, Pitfall 7 | If Bumblebee requires an explicit output format flag, subprocess integration fails silently |
| A4 | `gen2brain/beeep` v0.11.2 is CGO-free on Windows (uses pure-Go go-toast) | Standard Stack | If any dependency introduces CGO, the single static binary constraint is violated |
| A5 | VS Code Marketplace `publishedDate` field in extensionquery response is the initial publish date (not last updated) | Pattern 4 | If publishedDate is last-update time, release-age check is still effective (would catch very new updates) but semantics differ |
| A6 | `os.Rename` across `~/.vscode/extensions/` → `~/.beekeeper/quarantine/` is same-volume on typical Windows installations | Pattern 6 | If the user has extensions on a different drive, rename fails; needs copy+delete fallback |

---

## Project Constraints (from CLAUDE.md)

- **Go 1.25+, single static binary, no CGO in core** — `gen2brain/beeep` v0.11.2 is CGO-free confirmed. `tidwall/jsonc` has zero deps. All new packages must be pure Go.
- **`internal/policy` must be pure** — `extractExtensionInstall` and all Phase 3 policy additions go in `internal/policy` with no I/O. Marketplace adapter goes in `internal/catalog/`.
- **Fail closed by default** — marketplace timestamp unavailable → `TimestampMissing: true` → `EvaluateReleaseAge` blocks. Catalog unavailable → fail-closed (existing behavior).
- **Windows primary dev machine** — all new code must compile on Windows. No `syscall.SIGHUP` without build tag guard. Quarantine rename is cross-volume edge case (documented, not fixed in Phase 3).
- **mmap catalog loaded by `beekeeper check`** — the file-watcher daemon (`beekeeper watch`) must NOT reload the mmap catalog on every event. It should use the HTTP-based adapters (OSV, Socket, marketplace) for catalog lookup, consistent with the multi-source aggregator architecture.
- **Cobra wiring is thin** — all watch/scan/quarantine business logic in `internal/` packages; `cmd/beekeeper/main.go` is path resolution + struct wiring only.
- **No WSL integration tests** — marketplace HTTP tests use `httptest.Server` stubs; no live API calls in unit tests.
- **Reproducible builds** — `go get github.com/gen2brain/beeep@v0.11.2` and `go get github.com/tidwall/jsonc@v0.3.3` must be pinned in `go.mod` with `go mod verify` passing.

---

## Sources

### Primary (HIGH confidence)
- `pkg.go.dev/github.com/fsnotify/fsnotify@v1.10.1` — full API: Watcher, Add(), Events channel, Op bitmask (Create/Write/Remove/Rename/Chmod), platform-specific behavior (Windows Write events on parent directories), non-existent path limitation — fetched 2026-05-26
- `open-vsx.org/api/ms-python/python/2026.4.0` — live API response confirming `timestamp` field format: "2026-03-13T04:21:53.228120Z" (ISO 8601) — verified 2026-05-26
- `pkg.go.dev/github.com/gen2brain/beeep@v0.11.2` — API: Notify(), platform implementations, CGO-free status, godbus/dbus dependency, nodbus build tag — fetched 2026-05-26
- `pkg.go.dev/github.com/tidwall/jsonc@v0.3.3` — API: ToJSON(), zero deps, cannot preserve comments on write — fetched 2026-05-26
- `github.com/perplexityai/bumblebee` README — CLI schema: scan flags (--profile, --root, --ecosystem), NDJSON record fields, no --format flag, CLI-only (no importable library) — fetched 2026-05-26
- Phase 1/2 implementation files — `internal/policy/engine.go`, `internal/catalog/age_cache.go`, `internal/catalog/registry.go`, `internal/check/handler.go`, `internal/catalog/multi.go` — verified existing interfaces and patterns — 2026-05-26
- `go.mod` — confirmed fsnotify v1.10.1 NOT yet in go.mod (Phase 2 research referenced it but it was not added); must be added in Phase 3

### Secondary (MEDIUM confidence)
- `cursor.com/docs/configuration/extensions` — Cursor uses Open VSX registry (not VS Code Marketplace); extension directory path not documented in available content
- `windsurf.com` community sources — Windsurf extensions stored at `~/.windsurf/extensions/` (mirrors VS Code convention)
- `code.visualstudio.com/api/references/extension-manifest` — required fields: publisher, name, version, engines; extension directory naming convention (publisher.name-version)
- `marketplace.visualstudio.com` extensionquery API — `publishedDate` field confirmed in response; `api-version=3.0-preview.1` header required
- `github.com/gen2brain/beeep/blob/master/notify_windows.go` — imports `git.sr.ht/~jackmordaunt/go-toast` (pure Go COM); no CGO directives visible

### Tertiary (LOW confidence)
- Cursor Windows extension path (`%USERPROFILE%\.cursor\extensions\`) — inferred from VS Code convention; not directly confirmed from Cursor documentation
- Open VSX `timestamp` semantics (last-sync vs. first-publish) — inferred from understanding of how Open VSX mirrors from VS Code Marketplace; not explicitly documented

---

## Metadata

**Confidence breakdown:**
- Standard stack (fsnotify, beeep, tidwall/jsonc): HIGH — all verified via pkg.go.dev and official docs
- Extension manifest schema: HIGH — verified from VS Code official docs
- Open VSX API schema: HIGH — verified via live API call (timestamp field confirmed)
- Bumblebee CLI schema: HIGH — verified from official README (no --format flag; NDJSON default)
- Cursor extension dir path (macOS/Linux): MEDIUM — community-confirmed `~/.cursor/extensions/`
- Cursor extension dir path (Windows): LOW — inferred from VS Code convention; needs empirical validation
- VS Code Marketplace publishedDate semantics: MEDIUM — confirmed present in API response; format confirmed

**Research date:** 2026-05-26
**Valid until:** 2026-06-25 (30 days; stable APIs; beeep pre-v1.0 so watch for breaking changes)
