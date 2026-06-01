# Phase 3: Editor Extension Defense - Context

**Gathered:** 2026-05-26
**Status:** Ready for planning
**Source:** PRD Express Path (beekeeper-prd.md §5.4 + REQUIREMENTS.md EDXT-01–06)

<domain>
## Phase Boundary

Phase 3 delivers all three layers of editor extension defense described in PRD §5.4:

1. **Agent-initiated CLI intercept** — Recognize `code/code-insiders/cursor/windsurf --install-extension` (including bulk forms) and route through the existing policy engine before the extension reaches disk.
2. **File-watcher daemon** — `beekeeper watch` detects GUI installs and auto-updates that bypass the CLI hook, matching every new extension directory against catalogs within seconds of appearance.
3. **Scan orchestrator** — `beekeeper scan` makes Bumblebee scans first-class, catalog-delta-triggered, and merges output into the unified NDJSON audit stream.

Also in scope: quarantine workflow (`list/restore/purge`), and `beekeeper init` editor auto-detection with consent-based auto-update disable and file-watcher setup.

Out of scope for this phase: blocking the extension loader itself (requires editor-vendor cooperation), Sentry process correlation (Phase 5), MCP gateway integration (Phase 4).

</domain>

<decisions>
## Implementation Decisions

### EDXT-01: Agent-Initiated CLI Intercept

- Hook handler (`beekeeper check`) receives a tool call JSON where `tool_name` is `Bash` (or equivalent) and the command contains an editor extension install invocation.
- Recognized patterns (must all route through catalog + release-age policy):
  - `code --install-extension <id>[@<version>]`
  - `code-insiders --install-extension <id>[@<version>]`
  - `cursor --install-extension <id>[@<version>]`
  - `windsurf --install-extension <id>[@<version>]`
  - Bulk forms: any command containing multiple `--install-extension` flags
- Detection lives in `internal/policy` (pure, no I/O) as a new `ExtensionInstallCommand` recognition function.
- After recognition, the policy engine routes to catalog matcher (corroboration semantics already in place from Phase 2) and release-age check with `ecosystem: "editor-extension"`.
- Decision recorded in NDJSON with `rule_ids` citing EDXT-01, full catalog provenance.

### EDXT-02: File-Watcher Daemon (`beekeeper watch`)

- New subcommand `beekeeper watch` in `cmd/beekeeper/main.go`; business logic in `internal/watch/`.
- Uses **`fsnotify` v1.10.1** — explicit import `github.com/fsnotify/fsnotify`.
- Watches these directories (platform-aware paths):
  - `~/.vscode/extensions/` (also `%USERPROFILE%\.vscode\extensions\` on Windows)
  - `~/.cursor/extensions/`
  - `~/.windsurf/extensions/`
  - OpenVSX paths if discovered by `beekeeper init`
- Implementation: explicit `watcher.Add()` per directory — no recursive API (fsnotify has no cross-platform recursive watch).
- On Windows NTFS: filter to `Create` events only (Windows generates additional `Write` events on directory population that are not meaningful for new-extension detection).
- On macOS FSEvents and Linux inotify: `Create` and `Rename` events (extensions sometimes appear via atomic rename).
- Daemon runs until signal; recovers from transient errors with exponential backoff, logs to audit log.

### EDXT-03: New Extension Handling

When a `Create` event fires on a watched directory's immediate child:
1. **Manifest parse**: read `<dir>/package.json`, extract `publisher`, `name`, `version`, `displayName`.
2. **Catalog match** via existing `internal/catalog.MultiCatalogLookup` — same corroboration semantics as Phase 2; `ecosystem: "editor-extension"`.
3. **Release-age check** via existing `internal/policy.ReleaseAgeEvaluate` — same 24h default; Marketplace publish timestamp fetched via Open VSX / VS Code Marketplace REST APIs with 24h TTL cache.
4. **On catalog hit OR release-age block**:
   - Emit `critical` audit record to NDJSON log (record_type: `sentry_alert`, rule_ids: `["EDXT-03"]`).
   - Desktop notification: configurable (default on); cross-platform via `internal/notify/` wrapping OS-native mechanisms (notify-send on Linux, AppleScript on macOS, Windows toast via `go-toast`).
   - Quarantine: move extension directory to `~/.beekeeper/quarantine/extensions/<publisher>.<name>-<version>-<timestamp>/`; write manifest snapshot as `<quarantine-dir>/beekeeper-manifest.json`.
5. **On clean check**: write allow record to audit log.

### EDXT-04: `beekeeper scan` Orchestrator

- New subcommand `beekeeper scan [--deep]` in `cmd/beekeeper/main.go`; logic in `internal/scan/`.
- Invokes Bumblebee CLI (`bumblebee scan --format ndjson`) via `exec.CommandContext`; supervises stdout.
- Merges Bumblebee NDJSON output with Beekeeper-specific rule results (catalog + release-age on installed extensions) into a unified NDJSON stream written to stdout and to audit log.
- `--deep` flag passes corresponding depth flag to Bumblebee invocation.
- Scan is triggered automatically by `catalogs watch` daemon when new `threat_intel/` entries arrive (existing hook from Phase 2 `internal/catalog/watch.go` emits a scan trigger event).
- If Bumblebee is not installed, scan degrades gracefully: runs Beekeeper-only scan, logs `bumblebee_unavailable: true` in scan audit record.

### EDXT-05: Quarantine Workflow

- Three subcommands added: `beekeeper quarantine list`, `quarantine restore <id>`, `quarantine purge`.
- Logic in `internal/quarantine/`; IDs are directory basenames under `~/.beekeeper/quarantine/`.
- `list`: reads quarantine directory, parses `beekeeper-manifest.json` per item, prints table to stdout: ID, publisher, name, version, quarantined-at, reason.
- `restore <id>`: moves item back to original extension directory; removes `beekeeper-manifest.json`; emits audit record.
- `purge`: removes all items in quarantine directory; prompts for confirmation unless `--yes` flag; emits audit record per item.
- All quarantine operations emit NDJSON audit records with `rule_ids: ["EDXT-05"]`.

### EDXT-06: `beekeeper init` Editor Detection

- Existing `beekeeper init` command (Phase 1) extended to detect installed editors.
- Detection: check for executables in PATH (`code`, `cursor`, `windsurf`, `codium`) and for extension directories at known paths.
- Offers (each with explicit user consent, never writes without it):
  1. Disable extension auto-update in the editor's JSON settings file — sets `"extensions.autoUpdate": false` in VS Code / Cursor user settings JSON; writes setting using `go-jsonc` parser to preserve comments.
  2. Enable the file-watcher for detected extension directories — adds directories to Beekeeper config's `watch.directories` list.
  3. Set release-age threshold for new extensions (default: 1440 minutes / 24h).
- No editor setting is touched without explicit per-editor consent prompt.
- Init changes are idempotent: re-running does not duplicate settings.

### Architecture: Where New Code Lives

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

- Specific fsnotify event debounce window (extension installations generate multiple events; a short debounce avoids duplicate catalog lookups). Suggested: 500ms.
- Marketplace timestamp API selection: VS Code Marketplace API vs. Open VSX REST API; use the one matching the detected extension publisher format. Cache key: `publisher.name@version`.
- `go-toast` vs. alternative for Windows desktop notifications — use whatever has the least transitive dependency footprint.
- Whether `beekeeper watch` runs as a foreground daemon (blocking) or can be backgrounded — PRD implies foreground for now; background mode is Phase 4 / platform service work.
- Test fixtures: use synthetic `package.json` manifests in `internal/watch/testdata/`.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Architecture and Constraints
- `CLAUDE.md` — Architecture constraints (Go 1.25+, single binary, `internal/` structure, `internal/policy` pure, fail-closed, mmap catalog loading)
- `.planning/PROJECT.md` — Core decisions, locked choices
- `.planning/REQUIREMENTS.md` — EDXT-01 through EDXT-06 (verbatim requirement text)
- `.planning/ROADMAP.md` — Phase 3 success criteria, dependency on Phase 2

### Phase 2 Patterns (reuse directly)
- `.planning/phases/02-policy-engine-multi-source-catalogs/02-PATTERNS.md` — Pattern map for Phase 2 code
- `.planning/phases/02-policy-engine-multi-source-catalogs/02-CONTEXT.md` — Phase 2 decisions (corroboration engine, OSV/Socket adapters, watch daemon)
- `.planning/phases/02-policy-engine-multi-source-catalogs/02-RESEARCH.md` — Technical research (fsnotify behavior, platform event types)

### Key Source Files to Extend
- `internal/policy/` — Pure policy engine; EDXT-01 extension CLI recognition goes here
- `internal/catalog/` — Catalog adapters; Marketplace timestamp adapter follows OSV/Socket patterns
- `internal/check/` — Hook handler entry point; must recognize extension install commands
- `cmd/beekeeper/main.go` — Cobra CLI wiring; add `watch`, `scan`, `quarantine` subcommands

</canonical_refs>

<specifics>
## Specific Ideas

### PRD Threat Model Context (PRD §11.1)
Beekeeper defends against compromised editor extensions via three layers. The Nx Console compromise (May 2026, 18-minute Marketplace window, ~3,800 GitHub repos exfiltrated) is the canonical example. Phase 3 closes:
- CLI install path (Layer 1)
- GUI / auto-update path (Layer 2)
- Scheduled scan path (Layer 3)

### fsnotify Platform Notes (from EDXT-02 requirement text)
- Use explicit `watcher.Add()` per directory — no recursive API cross-platform
- Windows NTFS: filter `Create` events only
- `fsnotify` v1.10.1 is the pinned version

### Release-Age for Extensions
PRD §5.4: "extensions less than 24 hours old at publish time are blocked unless the publisher is allowlisted." This mirrors PLCY-02 semantics already implemented; Phase 3 adds `editor-extension` as an ecosystem.

### Self-Defense Work in Phase 3
No new self-defense deliverables are explicitly called out for Phase 3 in the PRD phasing section. Carry forward: all existing self-defense infrastructure (reproducible builds, Sigstore, Renovate) remains in place. Any new dependencies (fsnotify, notification library) must be pinned in `go.mod` and reviewed per the SFDF constraints.

</specifics>

<deferred>
## Deferred Ideas

- True extension-loader hooking (requires editor-vendor cooperation) — out of scope per PRD §11.2
- Background / daemonized `beekeeper watch` as a system service — Phase 4 (integration surfaces installs it)
- OpenVSX path auto-detection beyond the three primary editors — can be added to `beekeeper init` incrementally
- ContextForge / MCPGuard policy-plugin mode — Phase 4 / v1.5 deliverable
- Weighted corroboration for extensions — deferred per HARD-02 (v2 requirement)

</deferred>

---

*Phase: 03-editor-extension-defense*
*Context gathered: 2026-05-26 via PRD Express Path (beekeeper-prd.md §5.4)*
