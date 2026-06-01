# Phase 8: TUI Dashboard — Research

**Researched:** 2026-05-28
**Domain:** Bubble Tea v2 (`charm.land/bubbletea/v2`), Lip Gloss v2, Bubbles v2, fsnotify v1.10.1, golang.org/x/term
**Confidence:** HIGH — all module versions verified via Go module proxy; API details verified against pkg.go.dev and source

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- Import path: `charm.land/bubbletea/v2` (NOT `github.com/charmbracelet/bubbletea`)
- Also add: `charm.land/lipgloss/v2` (styling), `charm.land/bubbles/v2` (spinner, textinput, viewport, table)
- `golang.org/x/term` for resize workaround
- Windows resize workaround: polling goroutine via `golang.org/x/term.GetSize` every 500ms, sending synthetic `tea.WindowSizeMsg` (Bubble Tea v2 issue #1601)
- Package structure: `internal/tui/` flat — `model.go`, `base.go`, `palette.go`, `panel.go`, `incidents.go`, `toast.go`, `*_panel.go` files; NO `panels/` subdirectory (avoids import cycles)
- Data sources: audit log NDJSON tail, state.json, config.json, quarantine index, IPC client for health pips
- Refresh: fsnotify Write events primary; 1s polling fallback; stateTick 5s for state/config/quarantine; healthTick 10s for IPC+gateway probe
- Layout: calm base screen + command palette overlay + panel overlays + inline incident card (NOT tab grid)
- Admin mode: `--admin` flag; routes actions through existing internal packages
- Note: `teatest` is INCOMPATIBLE with Bubble Tea v2 — use direct Update()/viewString() unit tests

### Claude's Discretion

All discretion items resolved in CONTEXT.md. No open discretion areas.

### Deferred Ideas (OUT OF SCOPE)

- SSH-accessible TUI (system SSH handles it; nothing Beekeeper-specific)
- `beekeeper diag` command (Phase 9, CODE-06)
- `beekeeper-self` catalog checks in health panel (Phase 9, CTLG-04)
- Policy-as-code toggles beyond enable/disable (Phase 9)
- Web UI / desktop GUI (out of scope for v1)

</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TUI-01 | `beekeeper dashboard` — Bubble Tea v2, single screen, sshable; event-driven refresh via file watcher, 1s polling fallback | Module verified at `charm.land/bubbletea/v2 v2.0.6`; `tea.NewProgram` + `tea.Program.Run()` is the entry point |
| TUI-02 | Live activity feed panel — tool calls in real time with decision indicator, filterable | `audit.Query` + `bufio.Scanner` tail pattern already in `internal/audit/query.go`; fsnotify Write event triggers re-tail |
| TUI-03 | Sentry alerts panel — severity-color-coded, expandable detail | `AuditRecord` sentry fields already in `internal/audit/types.go`; `bubbles/v2/viewport` for scrollable expandable rows |
| TUI-04 | Catalog freshness panel — per-source last sync, delta count, stale indicator | `internal/catalog/watch.go` WatchState struct; 5s polling ticker; `bubbles/v2/table` for tabular display |
| TUI-05 | Scan status panel — last scan timestamp, findings count, one-key scan trigger | `internal/scan` package; `s` key in admin mode dispatches scan via existing package |
| TUI-06 | Active policies panel — loaded policy files with rule counts; drill-down | `internal/config.Load()` for policy file list; `bubbles/v2/table` + viewport for expand |
| TUI-07 | Quarantine panel — items with restore and purge actions | `internal/quarantine.QuarantineManager`; admin mode dispatches typed commands |
| TUI-08 | System health panel — Sentry/gateway/LlamaFirewall status | `internal/ipc.Client` for `StatusResponse`; 10s polling ticker; degrades gracefully on no daemon |
| TUI-09 | Admin mode (`--admin` flag) — policy toggling and quarantine actions | All actions route through existing internal packages; TUI dispatches typed commands only |
| TUI-10 | Windows resize workaround — polling goroutine sends synthetic `WindowSizeMsg` every 500ms | Confirmed: issue #1601 still open in v2.0.6; `term.GetSize(int(os.Stdout.Fd()))` is the correct API |

</phase_requirements>

---

## Summary

Phase 8 builds a single-screen Bubble Tea v2 terminal dashboard consumed by the `beekeeper dashboard` command. The TUI is a pure read consumer of existing internal packages (audit, config, quarantine, ipc, catalog) plus a new `internal/tui/` package. No privileged operations, no new business logic.

All module versions have been verified against the Go module proxy as of 2026-05-28. The Charm ecosystem has migrated to `charm.land/` vanity domains for v2. The `charm.land/bubbletea/v2 v2.0.6` module is at a stable release with no retracted versions (beta1 was retracted). The v2 API has breaking changes from v0.x, most importantly: `View()` now returns `tea.View` (a struct, not a string), terminal feature toggles moved to `tea.View` fields, and key messages split into `tea.KeyPressMsg` / `tea.KeyReleaseMsg`.

The Windows resize regression (issue #1601) is confirmed still present in v2.0.6. The last four patch releases (v2.0.3–v2.0.6) contain no Windows resize fix. The polling workaround specified in CONTEXT.md is the correct approach. The `golang.org/x/term.GetSize(fd int) (width, height int, err error)` signature is confirmed.

For snapshot testing, `github.com/charmbracelet/x/exp/teatest` (pseudo-version `v0.0.0-20260527151214-009e6338d40d`) is the standard utility, but it depends on `github.com/charmbracelet/bubbletea v1.3.5` (v1, NOT v2). A v2-compatible testing path exists via `tea.WithWindowSize(w, h)` for headless testing without teatest, and direct `Model.Update()` unit tests bypass the need for a test runner entirely. This is a critical finding that affects the snapshot test approach.

**Primary recommendation:** Use `charm.land/bubbletea/v2 v2.0.6` with `charm.land/lipgloss/v2 v2.0.3` and `charm.land/bubbles/v2 v2.1.0`. Watch the audit log's parent directory (not the file itself) with fsnotify and filter by filename. Test panels with direct `Update()` unit tests; avoid `github.com/charmbracelet/x/exp/teatest` until a v2-compatible version ships.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| TUI rendering and event loop | `internal/tui` (frontend process) | — | Bubble Tea owns the terminal; all rendering is pure string composition in `View()` |
| Audit log tail / NDJSON parsing | `internal/tui/watcher.go` (consumer) | `internal/audit` (existing reader logic) | TUI owns the file-watch goroutine; `audit.AuditRecord` struct is the data model |
| State / config / quarantine reads | `internal/tui` panel models (read-only) | Existing `internal/config`, `internal/quarantine` | TUI calls existing package APIs on timers; no new business logic |
| Sentry / gateway daemon status | `internal/tui/health.go` (IPC consumer) | `internal/ipc.Client` | Health panel is the only panel with an IPC dependency; must degrade gracefully |
| Admin actions (toggle, restore, purge, scan) | Existing internal packages | `internal/tui/admin.go` (dispatcher) | TUI dispatches typed commands to `internal/policy`, `internal/quarantine`, `internal/scan`; validation is in those packages |
| Windows resize polling | `internal/tui/resize.go` (goroutine) | `golang.org/x/term.GetSize` | OS-specific workaround isolated to its own file; sends synthetic `tea.WindowSizeMsg` |
| Color palette / shared styles | `internal/tui/styles.go` | `charm.land/lipgloss/v2` | All style constants in one file to ensure consistent theming across panels |
| Cobra command wiring | `cmd/beekeeper/main.go` | — | `newDashboardCmd()` added to root; thin wiring only, no business logic |

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `charm.land/bubbletea/v2` | v2.0.6 | TUI framework: event loop, Model/Update/View, message routing | Only v2-stable release with `charm.land/` vanity path; required by CLAUDE.md constraint |
| `charm.land/lipgloss/v2` | v2.0.3 | Terminal styling: colors, borders, padding, layout composition | v2 companion to bubbletea v2; same `charm.land/` vanity domain |
| `charm.land/bubbles/v2` | v2.1.0 | UI components: viewport, table, textinput, spinner | v2 companion; provides production-grade scrolling, table, and input widgets |
| `golang.org/x/term` | v0.43.0 | `term.GetSize()` for Windows resize polling workaround | Standard library extension; already available as transitive dep via bubbletea |
| `github.com/fsnotify/fsnotify` | v1.10.1 | Audit log file-change notifications | Already in go.mod; no new dependency needed |

[VERIFIED: Go module proxy — `go list -m -json X@latest` for each package, 2026-05-28]

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/charmbracelet/x/exp/teatest` | pseudo-v0 (20260527) | Snapshot / golden-file testing for TUI panels | Only if a v2-compatible release ships; see Pitfall 3 below |

[VERIFIED: Go module proxy, 2026-05-28]

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `charm.land/bubbletea/v2` | `github.com/rivo/tview` | tview is not TEA-pattern; would require architectural rewrite; excluded by CLAUDE.md |
| `charm.land/bubbles/v2/viewport` | Custom scrolling panel | Viewport handles all edge cases (mouse wheel, half-page, GotoTop/Bottom, line-by-line); hand-rolling costs 2–3 days of debugging |
| `charm.land/bubbles/v2/table` | ASCII table via fmt.Sprintf | Table handles focus, selection highlight, key bindings, and resize correctly; fmt.Sprintf cannot |
| `golang.org/x/term` | `os.Stdout` + ioctl | `term.GetSize` is portable across Windows/Linux/macOS; raw ioctl is platform-specific |

**Installation:**

```bash
go get charm.land/bubbletea/v2@v2.0.6
go get charm.land/lipgloss/v2@v2.0.3
go get charm.land/bubbles/v2@v2.1.0
go get golang.org/x/term@v0.43.0
```

**Note on `golang.org/x/term`:** The module is already a transitive dependency via `charm.land/bubbletea/v2` (which pulls `github.com/charmbracelet/x/term v0.2.2`, itself importing `golang.org/x/term`). Adding it as a direct dependency pins the version explicitly, which is correct per CLAUDE.md's pinned-deps requirement.

---

## Architecture Patterns

### System Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                   beekeeper dashboard                       │
│                  (internal/tui/model.go)                    │
│                                                             │
│  tea.Program ─── Init() ──► [fsnotify watcher goroutine]   │
│       │                     [1s fallback ticker]            │
│       │                     [5s state ticker]               │
│       │                     [10s health ticker]             │
│       │                     [500ms resize poller (Windows)] │
│       │                                                     │
│  Update() ◄──── tea.Msg ────────────────────────────────── │
│       │                                                     │
│       │  NewRecordsMsg ──► FeedPanel.Update()               │
│       │                 ──► SentryPanel.Update()            │
│       │  StateTickMsg  ──► CatalogsPanel.Update()           │
│       │                ──► ScanPanel.Update()               │
│       │                ──► PoliciesPanel.Update()           │
│       │                ──► QuarantinePanel.Update()         │
│       │  HealthTickMsg ──► HealthPanel.Update() ──► IPC     │
│       │  WindowSizeMsg ──► All panels (resize layout)       │
│       │  KeyPressMsg   ──► focused panel + tab routing      │
│       │                                                     │
│  View() ──► lipgloss layout composition ──► terminal       │
│                                                             │
│  Data sources (read-only):                                  │
│  ~/.beekeeper/audit/beekeeper.ndjson  ◄── fsnotify/ticker  │
│  ~/.beekeeper/state.json              ◄── 5s ticker        │
│  ~/.beekeeper/config.json             ◄── 5s ticker        │
│  ~/.beekeeper/quarantine/             ◄── 5s ticker        │
│  internal/ipc.Client                  ◄── 10s ticker       │
└─────────────────────────────────────────────────────────────┘
```

### Recommended Project Structure

```
internal/tui/
  model.go        # App (top-level Model), tea.Model impl, tab routing, WindowSizeMsg handling
  feed.go         # FeedPanel — live activity model+view
  sentry.go       # SentryPanel — alerts model+view
  catalogs.go     # CatalogsPanel — freshness model+view
  scan.go         # ScanPanel — status + trigger model+view
  policies.go     # PoliciesPanel — active rules model+view
  quarantine.go   # QuarantinePanel — items + admin actions model+view
  health.go       # HealthPanel — daemon status model+view
  watcher.go      # audit log file-watcher (fsnotify + polling fallback)
  resize.go       # Windows resize polling goroutine
  styles.go       # lipgloss color palette, shared styles
  admin.go        # admin-mode key bindings and action dispatcher
  model_test.go   # top-level model tests
  feed_test.go    # FeedPanel unit tests
  sentry_test.go  # SentryPanel unit tests
  (one _test.go per panel)
```

### Pattern 1: Bubble Tea v2 Model Interface

**What:** Every model (top-level `App` and each `*Panel`) implements `tea.Model`. In v2, `View()` returns `tea.View` not `string`. Terminal features are declared on `tea.View` fields, not program options.

**When to use:** All panel models.

```go
// Source: pkg.go.dev/charm.land/bubbletea/v2 + UPGRADE_GUIDE_V2.md
import tea "charm.land/bubbletea/v2"

type App struct {
    panels    []tea.Model
    focused   int
    width     int
    height    int
    adminMode bool
}

func (a App) Init() tea.Cmd {
    cmds := make([]tea.Cmd, 0, len(a.panels))
    for _, p := range a.panels {
        cmds = append(cmds, p.Init())
    }
    return tea.Batch(cmds...)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        a.width, a.height = msg.Width, msg.Height
        // re-layout all panels
    case tea.KeyPressMsg:  // NOTE: KeyPressMsg not KeyMsg in v2
        switch msg.String() {
        case "tab":
            a.focused = (a.focused + 1) % len(a.panels)
        case "q", "ctrl+c":
            return a, tea.Quit
        }
    }
    // delegate to focused panel
    var cmd tea.Cmd
    a.panels[a.focused], cmd = a.panels[a.focused].Update(msg)
    return a, cmd
}

func (a App) View() tea.View {     // Returns tea.View, NOT string
    v := tea.NewView(a.renderLayout())
    v.AltScreen = true              // Declare alt screen on the View struct
    return v
}
```

[VERIFIED: pkg.go.dev/charm.land/bubbletea/v2, UPGRADE_GUIDE_V2.md]

### Pattern 2: Program Startup

**What:** `tea.NewProgram` takes fewer options in v2. Alt screen, mouse mode, and window title are declared in `View()`. The `tea.WithWindowSize(w, h)` option is useful for headless testing.

```go
// Source: pkg.go.dev/charm.land/bubbletea/v2@v2.0.6
func runDashboard(adminMode bool) error {
    m := tui.NewApp(adminMode)
    p := tea.NewProgram(m)  // No options needed; View() declares AltScreen

    // Start Windows resize polling BEFORE p.Run() blocks
    if runtime.GOOS == "windows" {
        tui.StartResizePoller(p)
    }

    _, err := p.Run()  // Blocks until tea.Quit
    return err
}
```

[VERIFIED: pkg.go.dev/charm.land/bubbletea/v2]

### Pattern 3: Key Input Handling (v2 breaking change)

**What:** v2 replaced `tea.KeyMsg` with `tea.KeyPressMsg`. Field names changed: `msg.Type` → `msg.Code`, `msg.Runes` → `msg.Text` (now `string`), `msg.Alt` → `msg.Mod.Contains(tea.ModAlt)`. Space bar returns `"space"` not `" "`.

```go
// Source: UPGRADE_GUIDE_V2.md
case tea.KeyPressMsg:   // NOT tea.KeyMsg
    switch msg.String() {
    case "tab":
        // cycle focused panel
    case "shift+tab":
        // cycle backwards
    case "j", "down":
        // scroll in focused panel
    case "k", "up":
        // scroll in focused panel
    case "enter":
        // expand in sentry/policies panel
    case "space":       // NOTE: "space" not " "
        // toggle rule in admin mode
    case "f":
        // filter activity feed
    case "q", "ctrl+c":
        return m, tea.Quit
    }
```

[VERIFIED: UPGRADE_GUIDE_V2.md, pkg.go.dev/charm.land/bubbletea/v2]

### Pattern 4: Polling Tickers

**What:** Use `tea.Tick` for one-shot timers, `tea.Every` for wall-clock-aligned recurring ticks. For dashboard refresh, simple `tea.Tick` loops suffice.

```go
// Source: pkg.go.dev/charm.land/bubbletea/v2
type stateTickMsg time.Time
type healthTickMsg time.Time

func stateTickCmd() tea.Cmd {
    return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
        return stateTickMsg(t)
    })
}

func healthTickCmd() tea.Cmd {
    return tea.Tick(10*time.Second, func(t time.Time) tea.Msg {
        return healthTickMsg(t)
    })
}

// In App.Update:
case stateTickMsg:
    // re-read state.json, config.json, quarantine index
    return a, stateTickCmd()  // re-arm
```

[VERIFIED: pkg.go.dev/charm.land/bubbletea/v2]

### Pattern 5: Windows Resize Polling Workaround (TUI-10)

**What:** Bubble Tea v2 switched Windows input to VT mode (`ENABLE_VIRTUAL_TERMINAL_INPUT`), dropping `WINDOW_BUFFER_SIZE_EVENT` delivery. There is no official fix in v2.0.0–v2.0.6. Poll `term.GetSize` every 500ms and send synthetic `tea.WindowSizeMsg`.

```go
// Source: CONTEXT.md + confirmed via issue #1601 analysis
// internal/tui/resize.go
//go:build windows

package tui

import (
    "os"
    "time"

    tea "charm.land/bubbletea/v2"
    "golang.org/x/term"
)

// StartResizePoller starts a background goroutine that polls terminal
// dimensions every 500ms and sends synthetic WindowSizeMsg to p.
// This is the workaround for bubbletea v2 issue #1601: resize events
// are not delivered on Windows Terminal (VT input mode drops
// WINDOW_BUFFER_SIZE_EVENT).
func StartResizePoller(p *tea.Program) {
    go func() {
        ticker := time.NewTicker(500 * time.Millisecond)
        defer ticker.Stop()
        for range ticker.C {
            w, h, err := term.GetSize(int(os.Stdout.Fd()))
            if err != nil {
                continue
            }
            p.Send(tea.WindowSizeMsg{Width: w, Height: h})
        }
    }()
}
```

The stub for non-Windows platforms:

```go
// internal/tui/resize.go (build constraint: !windows)
//go:build !windows

package tui

import tea "charm.land/bubbletea/v2"

// StartResizePoller is a no-op on platforms with native resize signals.
func StartResizePoller(p *tea.Program) {}
```

[VERIFIED: issue #1601 confirmed active; term.GetSize signature verified at pkg.go.dev/golang.org/x/term@v0.43.0]

### Pattern 6: Lipgloss v2 Styles

**What:** Lipgloss v2 module path is `charm.land/lipgloss/v2`. The API is method-chained `lipgloss.NewStyle()`. Border styles are functions (`lipgloss.RoundedBorder()`). Color palettes are defined as package-level vars.

```go
// Source: pkg.go.dev/charm.land/lipgloss/v2@v2.0.3
// internal/tui/styles.go
import "charm.land/lipgloss/v2"

var (
    // Decision colors
    colorAllow = lipgloss.Color("#00FF87")   // green
    colorWarn  = lipgloss.Color("#FFFF00")   // yellow
    colorBlock = lipgloss.Color("#FF0000")   // red
    colorMuted = lipgloss.Color("241")       // ANSI 256 gray

    // Severity colors (sentry panel)
    colorCritical = lipgloss.Color("#FF0000")
    colorHigh     = lipgloss.Color("#FF8700")
    colorMedium   = lipgloss.Color("#FFFF00")
    colorLow      = lipgloss.Color("#87AFFF")

    // Panel border styles
    stylePanelActive = lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("63"))  // purple highlight

    stylePanelInactive = lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(colorMuted)

    styleDecisionAllow = lipgloss.NewStyle().Foreground(colorAllow).Bold(true)
    styleDecisionWarn  = lipgloss.NewStyle().Foreground(colorWarn).Bold(true)
    styleDecisionBlock = lipgloss.NewStyle().Foreground(colorBlock).Bold(true)
)
```

Layout composition:

```go
// lipgloss.JoinHorizontal / JoinVertical for two-column layout
left := lipgloss.JoinVertical(lipgloss.Left, feedPanel, sentryPanel)
right := lipgloss.JoinVertical(lipgloss.Left, scanPanel, policiesPanel, quarantinePanel, healthPanel, catalogsPanel)
full := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
```

[VERIFIED: pkg.go.dev/charm.land/lipgloss/v2@v2.0.3]

### Pattern 7: Viewport Component (bubbles/v2)

**What:** `charm.land/bubbles/v2/viewport` provides a scrollable panel. Note: `viewport.Model.View()` returns `string`, not `tea.View`. Embed it inside your panel's `View()` which returns `tea.View`.

```go
// Source: pkg.go.dev/charm.land/bubbles/v2@v2.1.0/viewport
import "charm.land/bubbles/v2/viewport"

type FeedPanel struct {
    vp      viewport.Model
    records []audit.AuditRecord
}

func NewFeedPanel(width, height int) FeedPanel {
    vp := viewport.New(
        viewport.WithWidth(width),
        viewport.WithHeight(height),
    )
    vp.MouseWheelEnabled = true
    return FeedPanel{vp: vp}
}

func (p FeedPanel) Update(msg tea.Msg) (FeedPanel, tea.Cmd) {
    var cmd tea.Cmd
    p.vp, cmd = p.vp.Update(msg)
    return p, cmd
}

// View() returns string (embedded in parent's tea.View)
func (p FeedPanel) View() string {
    return p.vp.View()
}

// Call when new records arrive
func (p *FeedPanel) SetContent(records []audit.AuditRecord) {
    p.records = records
    p.vp.SetContent(p.renderRecords())
    p.vp.GotoBottom()  // Auto-scroll to latest
}
```

Key viewport API:
- `viewport.New(opts ...Option) Model` — constructor with functional options
- `WithWidth(w int)`, `WithHeight(h int)` — dimension options
- `m.SetContent(s string)` — replace content
- `m.SetContentLines(lines []string)` — replace content from slice
- `m.GotoBottom()` — auto-scroll to newest record
- `m.ScrollUp(n)`, `m.ScrollDown(n)`, `m.PageUp()`, `m.PageDown()` — programmatic scroll
- `m.MouseWheelEnabled = true` — enable mouse wheel
- `m.SoftWrap = true` — wrap long lines instead of horizontal scroll

[VERIFIED: pkg.go.dev/charm.land/bubbles/v2@v2.1.0/viewport]

### Pattern 8: Table Component (bubbles/v2)

**What:** `charm.land/bubbles/v2/table` for catalog freshness, active policies, quarantine panels.

```go
// Source: pkg.go.dev/charm.land/bubbles/v2@v2.1.0/table
import "charm.land/bubbles/v2/table"

cols := []table.Column{
    {Title: "Source",    Width: 15},
    {Title: "Last Sync", Width: 20},
    {Title: "Delta",     Width: 8},
    {Title: "Next Sync", Width: 20},
    {Title: "Status",    Width: 8},
}

rows := []table.Row{
    {"bumblebee", "2026-05-28 14:00", "12", "2026-05-28 15:00", "OK"},
    // ...
}

t := table.New(
    table.WithColumns(cols),
    table.WithRows(rows),
    table.WithHeight(8),
    table.WithFocused(true),
)

// In Update:
t, cmd = t.Update(msg)

// In View (returns string, embed in parent's tea.View content):
return t.View()

// Get selected row for expand/action:
selected := t.SelectedRow()
```

[VERIFIED: pkg.go.dev/charm.land/bubbles/v2@v2.1.0/table]

### Pattern 9: Spinner Component (bubbles/v2)

**What:** Use for scan-in-progress and IPC-pending states.

```go
// Source: pkg.go.dev/charm.land/bubbles/v2@v2.1.0/spinner
import "charm.land/bubbles/v2/spinner"

type ScanPanel struct {
    spinner spinner.Model
    loading bool
}

func NewScanPanel() ScanPanel {
    s := spinner.New(spinner.WithSpinner(spinner.Dot))
    return ScanPanel{spinner: s}
}

func (p ScanPanel) Init() tea.Cmd {
    return p.spinner.Tick()  // arm spinner tick loop
}

func (p ScanPanel) Update(msg tea.Msg) (ScanPanel, tea.Cmd) {
    switch msg.(type) {
    case spinner.TickMsg:
        if p.loading {
            var cmd tea.Cmd
            p.spinner, cmd = p.spinner.Update(msg)
            return p, cmd
        }
    }
    return p, nil
}
```

[VERIFIED: pkg.go.dev/charm.land/bubbles/v2@v2.1.0/spinner]

### Pattern 10: fsnotify Audit Log Watcher

**What:** Watch the parent directory of the audit log file and filter for `fsnotify.Write` events on the specific file. Do NOT watch the file directly (fsnotify documents this as unreliable for append-only logs on some platforms).

```go
// Source: github.com/fsnotify/fsnotify v1.10.1 README + backend_windows.go
// internal/tui/watcher.go
import (
    "github.com/fsnotify/fsnotify"
    tea "charm.land/bubbletea/v2"
)

type newRecordsMsg []audit.AuditRecord

func watchAuditLog(p *tea.Program, auditPath string) error {
    w, err := fsnotify.NewWatcher()
    if err != nil {
        return err
    }
    defer w.Close()

    // Watch the parent directory, not the file itself.
    // Audit log is append-only so no atomic rename concern,
    // but directory-level watch is more reliable cross-platform.
    dir := filepath.Dir(auditPath)
    if err := w.Add(dir); err != nil {
        return err
    }

    var offset int64  // tracks last-read position for tail
    for {
        select {
        case event, ok := <-w.Events:
            if !ok {
                return nil
            }
            // Filter to our file + Write events only
            if event.Name == auditPath && event.Has(fsnotify.Write) {
                records, newOffset := tailFrom(auditPath, offset)
                offset = newOffset
                if len(records) > 0 {
                    p.Send(newRecordsMsg(records))
                }
            }
        case <-w.Errors:
            // Non-fatal; 1s fallback ticker covers gaps
        }
    }
}
```

**Windows NTFS note:** On NTFS, `ReadDirectoryChangesW` buffers events; under high write load the 64KB default buffer may overflow. Use `w.AddWith(dir, fsnotify.WithBufferSize(256000))` if burst writes are expected. The audit log write rate is low (one record per policy decision), so the default buffer is adequate. [VERIFIED: fsnotify v1.10.1 README + backend_windows.go source]

### Pattern 11: textinput Component (for filter/search bar)

```go
// Source: pkg.go.dev/charm.land/bubbles/v2@v2.1.0/textinput
import "charm.land/bubbles/v2/textinput"

type FilterBar struct {
    input textinput.Model
    active bool
}

func NewFilterBar() FilterBar {
    ti := textinput.New()
    ti.Placeholder = "filter by decision, agent, tool..."
    return FilterBar{input: ti}
}

// Focus when 'f' pressed in FeedPanel
func (fb *FilterBar) Activate() tea.Cmd {
    fb.active = true
    return fb.input.Focus()  // returns tea.Cmd for cursor blink
}
```

[VERIFIED: pkg.go.dev/charm.land/bubbles/v2@v2.1.0/textinput]

### Anti-Patterns to Avoid

- **Use `tea.KeyMsg` instead of `tea.KeyPressMsg`:** v2 removed `tea.KeyMsg` as a concrete type; the switch will silently not match any key events. Use `tea.KeyPressMsg`.
- **Return `string` from `View()`:** v2 `Model.View()` signature is `View() tea.View`. Returning a `string` is a compile error. Use `tea.NewView(content)` to wrap the rendered string.
- **Pass `tea.WithAltScreen()` to `tea.NewProgram()`:** This option was removed. Set `v.AltScreen = true` on the `tea.View` returned from `View()`.
- **Watch the audit log file directly with `watcher.Add(auditPath)`:** fsnotify documentation explicitly warns against file-level watches for append-only scenarios. Watch the parent directory and filter by `event.Name`.
- **Import `github.com/charmbracelet/bubbletea` (v1):** The CLAUDE.md constraint requires `charm.land/bubbletea/v2`. Mixing import paths causes two copies of the type system in the binary, and `tea.Model` from v1 is not the same interface as from v2.
- **Embed `charm.land/bubbles/v2/spinner.Model` as `tea.Model`:** The spinner does not implement `tea.Model` directly (no `Init()` method); it is a component embedded in parent models, with its `Tick()` seeded from the parent's `Init()`.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Scrollable text panels | Custom scroll + string slice | `bubbles/v2/viewport` | Handles mouse wheel, half-page, GotoBottom, content sizing, FillHeight, and per-line styling |
| Tabular data display | `fmt.Sprintf` aligned columns | `bubbles/v2/table` | Handles column widths, cursor tracking, focus highlight, key bindings, and viewport scrolling |
| Terminal styling | ANSI escape sequences by hand | `charm.land/lipgloss/v2` | Handles color downsampling (256→16→none), border composition, JoinHorizontal/JoinVertical, and automatic width measurement |
| Single-line text input | `bufio.Scanner` or raw key accumulation | `bubbles/v2/textinput` | Handles Unicode, cursor movement, paste, backspace, and validation callbacks |
| Loading indicator | Custom spinner string rotation | `bubbles/v2/spinner` | Handles frame timing, multiple presets, and integration with tea.Tick |
| Terminal size detection | `os.Stdout.Fd()` + ioctl | `golang.org/x/term.GetSize()` | Portable across Windows/Linux/macOS; correct fd handling |
| Layout composition | Manual string concatenation with padding | `lipgloss.JoinHorizontal` + `JoinVertical` | Handles ANSI-aware width, alignment (Top/Center/Bottom), and variable-height panels |

**Key insight:** The Bubbles library exists precisely because scrollable, keyboard-navigable panels have dozens of edge cases (off-by-one scroll, mouse vs keyboard interaction, focus management, resize). Every item in this table represents 1–5 days of debugging if hand-rolled.

---

## Common Pitfalls

### Pitfall 1: `View()` Returns `string` (v1 pattern, v2 compile error)

**What goes wrong:** Code written for v0.x/v1 returns `string` from `View()`. In v2, `View()` must return `tea.View`. The Go compiler catches this at compile time as an interface satisfaction failure.

**Why it happens:** Most Bubble Tea tutorials and examples online reference v0.x/v1 APIs. The `charm.land/bubbletea/v2` module only appeared in February 2026; most Stack Overflow and blog answers predate it.

**How to avoid:** Always use `return tea.NewView(content)` where `content` is the rendered string. For sub-components (viewport, table, spinner), their `View()` returns `string` — wrap it in the parent's `tea.NewView()`.

**Warning signs:** Compile error `cannot use string as tea.View` or `does not implement tea.Model (wrong type for method View)`.

### Pitfall 2: `tea.KeyMsg` Switch Miss (v2 breaking change)

**What goes wrong:** `case tea.KeyMsg:` in the type switch never matches in v2; all key events arrive as `tea.KeyPressMsg` (or `tea.KeyReleaseMsg`). The panel silently ignores all keyboard input.

**Why it happens:** v2 changed `KeyMsg` from a concrete struct to an interface covering both press and release. `tea.KeyPressMsg` is the concrete type for key-down events.

**How to avoid:** Use `case tea.KeyPressMsg:` for all keyboard handling. Access the key via `msg.String()` (unchanged). Check modifiers via `msg.Mod.Contains(tea.ModAlt)` instead of `msg.Alt`.

**Warning signs:** No keyboard response in the TUI; tab panel cycling does not work.

### Pitfall 3: `github.com/charmbracelet/x/exp/teatest` Depends on Bubbletea v1

**What goes wrong:** Adding `github.com/charmbracelet/x/exp/teatest` as a test dependency pulls `github.com/charmbracelet/bubbletea v1.3.5` into the module graph. The v1 `tea.Model` interface (`View() string`) conflicts with the v2 interface (`View() tea.View`). Tests that pass `tui.App{}` (a v2 model) to `teatest.NewTestModel()` (which expects a v1 model) fail to compile.

**Why it happens:** The `teatest` package has not been updated to support v2. As of 2026-05-28 (latest pseudo-version 20260527), its `go.mod` depends on `github.com/charmbracelet/bubbletea v1.3.5`. The v2 testing framework issue (#1654) was opened 2026-04-01 and remains open with no accepted solution.

**How to avoid:** Do NOT add `github.com/charmbracelet/x/exp/teatest` as a dependency. Instead:

1. **Direct `Update()` unit tests** (no framework needed):
   ```go
   func TestFeedPanelUpdate(t *testing.T) {
       p := NewFeedPanel(80, 24)
       records := []audit.AuditRecord{{Decision: "block", ToolName: "bash"}}
       p.SetContent(records)
       rendered := p.View()  // returns string (sub-component)
       if !strings.Contains(rendered, "block") {
           t.Errorf("expected block indicator in feed, got: %s", rendered)
       }
   }
   ```

2. **`tea.WithWindowSize(w, h)` for headless program tests** (v2 built-in):
   ```go
   func TestAppRender(t *testing.T) {
       m := NewApp(false)
       p := tea.NewProgram(m, tea.WithWindowSize(120, 40))
       // p.Run() would block; test Init and first View instead
       m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
       _ = m2.(App).View()  // should not panic
   }
   ```

**Warning signs:** `ambiguous import: found package charm.land/bubbletea/v2 in multiple modules` or type errors on `tea.Model`.

### Pitfall 4: `watcher.Add(auditFilePath)` Instead of Parent Directory

**What goes wrong:** Watching the audit log file directly misses `Write` events when the log rotation creates a new file (the old inode is replaced), or when the OS coalesces multiple appends into one notification. On Windows, file-level watches via `ReadDirectoryChangesW` are even less reliable for rapidly-appended files.

**Why it happens:** fsnotify's API accepts any path, including files, without error. The limitation is documented only in the FAQ ("Watching a file doesn't work well").

**How to avoid:** `watcher.Add(filepath.Dir(auditLogPath))` and filter events by `event.Name == auditLogPath && event.Has(fsnotify.Write)`. The 1s fallback ticker catches any missed events.

**Warning signs:** Activity feed stops updating after a log rotation or extended TUI session on Windows.

### Pitfall 5: `tea.WithAltScreen()` Removed in v2

**What goes wrong:** `tea.NewProgram(m, tea.WithAltScreen())` causes a compile error in v2 — `tea.WithAltScreen` is not defined.

**Why it happens:** All terminal feature toggles moved from program options to `tea.View` fields in v2.

**How to avoid:**
```go
// v2 correct: declare in View()
func (a App) View() tea.View {
    v := tea.NewView(a.render())
    v.AltScreen = true
    return v
}

// WRONG (v1 pattern, compile error in v2):
p := tea.NewProgram(m, tea.WithAltScreen())
```

**Warning signs:** `undefined: tea.WithAltScreen` compile error.

### Pitfall 6: `WindowSizeMsg` Extra Messages Are Not No-Ops by Default

**What goes wrong:** The resize poller sends a `tea.WindowSizeMsg` every 500ms. If `App.Update()` calls `SetContent` or re-renders all panels on every `WindowSizeMsg`, this causes 2 full re-renders per second even when the window hasn't changed, degrading performance.

**Why it happens:** The resize poller specification in CONTEXT.md notes "Bubble Tea ignores same-size `WindowSizeMsg`" — this refers to Bubble Tea not sending the message *to the terminal* unchanged, but your `Update()` still receives it.

**How to avoid:** Track last known dimensions and early-return from `WindowSizeMsg` handling when dimensions are unchanged:
```go
case tea.WindowSizeMsg:
    if msg.Width == a.width && msg.Height == a.height {
        return a, nil  // no layout change needed
    }
    a.width, a.height = msg.Width, msg.Height
    // ... re-layout panels
```

**Warning signs:** CPU usage noticeably elevated while TUI is idle on Windows.

---

## Code Examples

### Confirmed: `go.mod` additions required

```bash
# Run from beekeeper repo root — verified via go list -m -json
go get charm.land/bubbletea/v2@v2.0.6
go get charm.land/lipgloss/v2@v2.0.3
go get charm.land/bubbles/v2@v2.1.0
go get golang.org/x/term@v0.43.0
```

The resulting `go.mod` additions:

```
require (
    charm.land/bubbles/v2 v2.1.0
    charm.land/bubbletea/v2 v2.0.6
    charm.land/lipgloss/v2 v2.0.3
    golang.org/x/term v0.43.0
    // ... existing deps unchanged
)
```

### Confirmed: `term.GetSize` Signature

```go
// Source: pkg.go.dev/golang.org/x/term@v0.43.0
// Returns the visible dimensions of the given terminal.
func GetSize(fd int) (width, height int, err error)

// Usage:
w, h, err := term.GetSize(int(os.Stdout.Fd()))
```

[VERIFIED: pkg.go.dev/golang.org/x/term@v0.43.0]

### Confirmed: fsnotify Write Event Check

```go
// Source: fsnotify v1.10.1 README
if event.Has(fsnotify.Write) {  // NOT: event.Op&fsnotify.Write != 0
    // ...
}
```

The `Has()` method is the current API (v1.6+). The bitfield check still works but `Has()` is idiomatic.

[VERIFIED: fsnotify v1.10.1 README]

### Confirmed: `newDashboardCmd()` pattern

```go
// cmd/beekeeper/main.go (extend existing file)
root.AddCommand(newDashboardCmd())

func newDashboardCmd() *cobra.Command {
    var adminMode bool
    cmd := &cobra.Command{
        Use:   "dashboard",
        Short: "Open the real-time TUI dashboard",
        Args:  cobra.NoArgs,
        RunE: func(cmd *cobra.Command, _ []string) error {
            return tui.Run(cmd.Context(), adminMode)
        },
    }
    cmd.Flags().BoolVar(&adminMode, "admin", false,
        "Enable admin mode (policy toggle, quarantine actions)")
    return cmd
}
```

[ASSUMED: Cobra pattern is consistent with existing `newXCmd()` functions in main.go — verified against live source, pattern is lock-step]

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `github.com/charmbracelet/bubbletea` import | `charm.land/bubbletea/v2` import | v2.0.0 (Feb 2026) | Build fails with old import path; CLAUDE.md locks to new path |
| `View() string` | `View() tea.View` | v2.0.0 | Interface change; sub-components still return `string` |
| `tea.WithAltScreen()` program option | `view.AltScreen = true` field | v2.0.0 | Program options removed; declare in View() |
| `tea.KeyMsg` concrete type | `tea.KeyPressMsg` / `tea.KeyReleaseMsg` | v2.0.0 | `msg.Type` → `msg.Code`; `msg.Runes` → `msg.Text`; `msg.Alt` → `msg.Mod.Contains(tea.ModAlt)` |
| `tea.EnterAltScreen` command | `view.AltScreen = true` field | v2.0.0 | Command removed |
| `tea.HideCursor` command | `view.Cursor = nil` field | v2.0.0 | Command removed |
| `charm.land/bubbles` (v1 path) | `charm.land/bubbles/v2` | March 2026 | Same vanity domain, /v2 suffix added |

**Deprecated:**

- `github.com/charmbracelet/bubbletea` (v1): still published and maintained, but not compatible with v2 type system. Do not use in this codebase.
- `github.com/charmbracelet/lipgloss` (v1): replaced by `charm.land/lipgloss/v2 v2.0.3`. Not compatible.
- `github.com/charmbracelet/bubbles` (v1): replaced by `charm.land/bubbles/v2 v2.1.0`. Not compatible.
- `github.com/charmbracelet/x/exp/teatest`: v1-only; no v2-compatible release as of 2026-05-28.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `tea.NewProgram(m)` with no options is the correct v2 startup (options moved to View fields) | Pattern 2 | Compile error or missing features if some options still required |
| A2 | `cobra.Command.RunE` pattern for `newDashboardCmd()` matches existing command style in main.go | Code Examples | Style deviation only; no functional risk |
| A3 | Bubble Tea v2 `WindowSizeMsg` is still sent once at startup (initial terminal size) | Pattern 5 / Pitfall 6 | If not sent at startup, all panels start at zero size until first resize |
| A4 | `charm.land/bubbles/v2/spinner.Tick()` seeds the spinner animation when called from parent's `Init()` | Pattern 9 | Spinner stays frozen if seeding mechanism differs |
| A5 | The audit log is append-only (never rewritten atomically); therefore watching the parent directory for `Write` events is sufficient without handling `Rename`/`Create` | Pattern 10 | Feed stops after log rotation if rotation uses rename-and-recreate |

**Highest risk:** A5. If `internal/audit/rotate.go` uses rename-then-create (atomic rotation), the watcher must also handle `fsnotify.Create` on the audit file to resume tailing. The 1s fallback ticker mitigates this — the feed will resume within 1 second even if the watcher loses the file.

---

## Open Questions

1. **Atomic log rotation in `internal/audit/rotate.go`**
   - What we know: `rotate.go` exists; typical rotation strategies either truncate-in-place or rename-and-recreate.
   - What's unclear: Which strategy does Beekeeper use? If rename-and-recreate, the file-watch inode is lost.
   - Recommendation: Read `internal/audit/rotate.go` before implementing `watcher.go`. Add `fsnotify.Create` handling if rotation recreates the file.

2. **`charm.land/bubbles/v2/viewport.Model.Init()` method**
   - What we know: The viewport `Update()` and `View()` are documented. `Init()` is not shown in pkg.go.dev.
   - What's unclear: Whether `viewport.Model` fully satisfies `tea.Model` or is a sub-component only.
   - Recommendation: Treat viewport as a sub-component (embed, delegate `Update()` calls, use `View()` output as string). Do not try to use it as a standalone `tea.Model` passed to `tea.NewProgram`.

3. **IPC `StatusResponse` for gateway daemon status**
   - What we know: `ipc.StatusResponse` covers Sentry daemon fields (PID, Tier, RulesActive, EventsDropped, etc.). No gateway-specific fields present.
   - What's unclear: How the health panel gets gateway port and connection count (TUI-08 requires these).
   - Recommendation: Either extend `ipc.StatusResponse` with gateway fields, or have the health panel read `~/.beekeeper/state.json` for gateway port and poll `GET /health` on the gateway's local port.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.25 | Build | Yes | go1.25.0 | — |
| `charm.land/bubbletea/v2` | TUI framework | Yes (downloadable) | v2.0.6 | — |
| `charm.land/lipgloss/v2` | TUI styling | Yes (downloadable) | v2.0.3 | — |
| `charm.land/bubbles/v2` | TUI components | Yes (downloadable) | v2.1.0 | — |
| `golang.org/x/term` | Resize poller | Yes (transitive) | v0.43.0 | — |
| `github.com/fsnotify/fsnotify` | Audit log watcher | Yes (in go.mod) | v1.10.1 | — |
| Windows Terminal | TUI rendering on dev machine | Yes | Windows 11 | — |
| `github.com/charmbracelet/x/exp/teatest` | Snapshot tests | BLOCKED (v1 only) | pseudo-v0 20260527 | Direct `Update()` unit tests |

**Missing dependencies with no fallback:** None.

**Missing dependencies with fallback:**

- `github.com/charmbracelet/x/exp/teatest` — depends on bubbletea v1, incompatible with v2. Fallback: direct `Model.Update()` unit tests and `tea.WithWindowSize` headless test pattern. Issue #1654 tracks a v2 testing framework; monitor for updates.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (no external framework) |
| Config file | none (go test works natively) |
| Quick run command | `go test ./internal/tui/... -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TUI-01 | `beekeeper dashboard` starts and renders | smoke | `go test ./internal/tui/... -run TestAppInit` | Wave 0 |
| TUI-02 | FeedPanel renders allow/warn/block with color indicators | unit | `go test ./internal/tui/... -run TestFeedPanel` | Wave 0 |
| TUI-03 | SentryPanel renders severity-coded alerts, expandable | unit | `go test ./internal/tui/... -run TestSentryPanel` | Wave 0 |
| TUI-04 | CatalogsPanel renders per-source freshness table | unit | `go test ./internal/tui/... -run TestCatalogsPanel` | Wave 0 |
| TUI-05 | ScanPanel renders last scan time and triggers scan in admin mode | unit | `go test ./internal/tui/... -run TestScanPanel` | Wave 0 |
| TUI-06 | PoliciesPanel renders policy file list; drill-down shows rules | unit | `go test ./internal/tui/... -run TestPoliciesPanel` | Wave 0 |
| TUI-07 | QuarantinePanel lists items; admin mode enables restore/purge | unit | `go test ./internal/tui/... -run TestQuarantinePanel` | Wave 0 |
| TUI-08 | HealthPanel renders daemon status; degrades gracefully on no IPC | unit | `go test ./internal/tui/... -run TestHealthPanel` | Wave 0 |
| TUI-09 | Admin mode key bindings dispatch to correct internal packages | unit | `go test ./internal/tui/... -run TestAdminMode` | Wave 0 |
| TUI-10 | Resize poller sends WindowSizeMsg; no-op on same size (Pitfall 6) | unit | `go test ./internal/tui/... -run TestResizePoller` | Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./internal/tui/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** `go test ./... -count=1` green + `go vet ./...` clean before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `internal/tui/` directory — create package
- [ ] `internal/tui/model_test.go` — covers TUI-01, TUI-09, TUI-10
- [ ] `internal/tui/feed_test.go` — covers TUI-02
- [ ] `internal/tui/sentry_test.go` — covers TUI-03
- [ ] `internal/tui/catalogs_test.go` — covers TUI-04
- [ ] `internal/tui/scan_test.go` — covers TUI-05
- [ ] `internal/tui/policies_test.go` — covers TUI-06
- [ ] `internal/tui/quarantine_test.go` — covers TUI-07
- [ ] `internal/tui/health_test.go` — covers TUI-08

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | TUI is local process; no authentication surface |
| V3 Session Management | No | No session state in TUI |
| V4 Access Control | Partial | Admin mode gated by `--admin` flag; all actions routed through internal packages with their own validation |
| V5 Input Validation | Yes | Filter bar input via `bubbles/v2/textinput` (bounds, validation callback); admin actions pass typed Go structs (not raw strings) to internal packages |
| V6 Cryptography | No | TUI reads only; no cryptographic operations |

### Known Threat Patterns for TUI

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| NDJSON injection via crafted audit log content | Spoofing, Tampering | `json.Unmarshal` into typed `AuditRecord` struct; rendered as lipgloss strings (no eval or shell execution); injection surface is display-only |
| Quarantine path traversal in admin mode | Tampering | `internal/quarantine.QuarantineManager.Restore/Purge` validates paths; TUI passes quarantine item IDs (not raw paths) |
| Audit log TOCTOU on rotation | Denial of Service | 1s fallback ticker resumes tail within 1 second; non-blocking |
| IPC client spoofing (health panel) | Spoofing | IPC socket path is `~/.beekeeper/sentry.sock`; `unix.GetsockoptUcred` peer verification is in `ipc/server.go` (not TUI's responsibility) |

**Self-defense summary (from CONTEXT.md):**
- TUI renders audit log content as display strings; no eval, no shell execution from record content.
- Admin mode actions call typed Go functions in existing packages; TUI never constructs raw shell commands.
- No new network connections in the TUI process.
- `beekeeper dashboard` requires no elevation.

---

## Sources

### Primary (HIGH confidence)

- `pkg.go.dev/charm.land/bubbletea/v2@v2.0.6` — Model interface, tea.View, tea.Program, tea.WindowSizeMsg, tea.Batch/Tick/Every/Quit, Program.Run signature
- `pkg.go.dev/charm.land/lipgloss/v2@v2.0.3` — NewStyle, color API, border styles, JoinHorizontal/JoinVertical
- `pkg.go.dev/charm.land/bubbles/v2@v2.1.0/viewport` — viewport.Model, New(), SetContent, GotoBottom, scrolling API
- `pkg.go.dev/charm.land/bubbles/v2@v2.1.0/table` — table.Model, Column, Row, New(), WithColumns/WithRows, SelectedRow
- `pkg.go.dev/charm.land/bubbles/v2@v2.1.0/spinner` — spinner.Model, Tick(), presets
- `pkg.go.dev/charm.land/bubbles/v2@v2.1.0/textinput` — textinput.Model, Focus, Value, SetValue
- `pkg.go.dev/golang.org/x/term@v0.43.0` — GetSize(fd int) (width, height int, err error)
- Go module proxy (`go list -m -json X@latest`) — verified versions for all five modules
- `github.com/fsnotify/fsnotify@v1.10.1/README.md` — directory-watch recommendation, fsnotify.Write, Windows NTFS notes, CHANGELOG
- `charm.land/bubbletea/v2/UPGRADE_GUIDE_V2.md` — v2 breaking changes (View type, KeyPressMsg, removed options)

### Secondary (MEDIUM confidence)

- `github.com/charmbracelet/bubbletea/discussions/1374` — v2 What's New overview (confirms tea.WithWindowSize for testing)
- `github.com/charmbracelet/bubbletea/issues/1601` — Windows resize regression; confirmed active in v2.0.6
- `github.com/charmbracelet/bubbletea/releases` — v2.0.3–v2.0.6 release notes; no Windows resize fix
- `pkg.go.dev/github.com/charmbracelet/x/exp/teatest` — teatest API; go.mod shows bubbletea v1.3.5 dependency (INCOMPATIBLE with v2)
- `github.com/charmbracelet/bubbletea/issues/1654` — v2 testing framework proposal; open as of 2026-05-28

### Tertiary (LOW confidence)

- WebSearch results for fsnotify Windows Write event behavior under load — corroborated by fsnotify README and backend_windows.go source

---

## Metadata

**Confidence breakdown:**

- Module versions: HIGH — verified via `go list -m -json` against live Go module proxy
- Bubble Tea v2 API (Model, Program, View): HIGH — verified via pkg.go.dev official documentation
- Lipgloss v2 API: HIGH — verified via pkg.go.dev
- Bubbles v2 component APIs: HIGH — verified via pkg.go.dev per-package
- Windows resize bug status: HIGH — issue #1601 confirmed active; v2.0.3–v2.0.6 release notes contain no fix
- `term.GetSize` signature: HIGH — verified via pkg.go.dev
- fsnotify file vs directory watch behavior: HIGH — verified via source README and backend_windows.go
- `teatest` v2 incompatibility: HIGH — go.mod of teatest module directly shows bubbletea v1.3.5 dependency
- Direct `Update()` test approach as teatest alternative: MEDIUM — standard Go testing practice, not explicitly Charm-documented

**Research date:** 2026-05-28
**Valid until:** 2026-07-28 (stable libraries; watch for bubbletea v2.0.7+ and teatest v2-compatible release)
