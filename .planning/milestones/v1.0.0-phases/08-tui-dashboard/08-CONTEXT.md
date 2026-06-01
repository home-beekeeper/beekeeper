# Phase 8: TUI Dashboard — Context

**Gathered:** 2026-05-28 (updated post-prototype)
**Status:** Ready for planning
**Source:** PRD §7.2 + REQUIREMENTS.md TUI-01–10 + beekeeper-tui-prototype.html (LOCKED DESIGN REFERENCE)

> **DESIGN AUTHORITY:** `beekeeper-tui-prototype.html` in the repo root is the locked design reference.
> Every visual detail below is derived from it. Production TUI must be an exact replica.

<domain>
## Phase Boundary

Phase 8 delivers the Bubble Tea v2 TUI dashboard. The design is **calm-mode** — quiet until something requires attention. It is NOT a 7-panel tab grid. It is a single calm base screen that escalates automatically on critical events, with a command palette for all navigation.

### Core UX Model (from prototype — LOCKED)

1. **Calm base screen** — the normal state. Shows: brand + status line, health pips row, incident slot (empty when calm), hint row, keybar. Nothing more.
2. **Command palette** (`:` key) — the primary navigation surface. Typeahead filter across all commands organized in groups (scan / investigate / configure / system). Press `:`, type to filter, Enter to run.
3. **Panel overlays** — appear over a blurred base screen. Each panel covers the base. Closed with `Esc`. Panels: alerts, quarantine, catalogs, policy, audit tail, scan, help.
4. **Incident card** — when Sentry fires a critical, an incident card appears *inline* on the base screen (not as a panel overlay). The frame gets a red outline. Containment actions are keyboard-reachable directly.
5. **Toast notifications** — ephemeral bottom-center message for action feedback.

### Critical-mode escalation flow
Normal → `x` or Sentry fires → frame red outline, status turns critical red, sentry pip turns red+pulsing, incident card appears inline with process tree and action buttons → `Q` quarantine / `I` isolate / `d` details → resolved → frame returns to calm, "contained" status, incident card clears.

Out of scope for this phase: policy-as-code (Phase 9), `beekeeper-self` catalog checks (Phase 9), `beekeeper diag` (Phase 9).

</domain>

<decisions>
## Design System (LOCKED — derived from prototype CSS)

### Color Palette

```go
// All colors match the prototype's CSS custom properties exactly.
// Use lipgloss.Color("#hex") — requires true-color terminal support.

colorBg        = lipgloss.Color("#0b0f14")   // --bg
colorScreen    = lipgloss.Color("#0d1117")   // --screen
colorPanel     = lipgloss.Color("#11161d")   // --panel (panel bg)
colorPanel2    = lipgloss.Color("#161b22")   // --panel2 (titlebar, keybar, panelhead bg)
colorBorder    = lipgloss.Color("#2b3543")   // --border
colorBorderDim = lipgloss.Color("#1c242e")   // --border-dim (rule/hr)
colorFg        = lipgloss.Color("#c9d1d9")   // --fg (body text)
colorDim       = lipgloss.Color("#6e7681")   // --dim (secondary text)
colorDimmer    = lipgloss.Color("#454d57")   // --dimmer (tertiary text)
colorWhite     = lipgloss.Color("#ffffff")   // --white (emphasized)
colorRed       = lipgloss.Color("#f85149")   // --red (critical threat, firing)
colorCoral     = lipgloss.Color("#f0883e")   // --coral (block, quarantine action)
colorAmber     = lipgloss.Color("#e3b341")   // --amber (warn, degraded, brand)
colorGreen     = lipgloss.Color("#3fb950")   // --green (allowed, healthy)
colorTeal      = lipgloss.Color("#39c5cf")   // --teal (focus & interaction keys)
colorSelbg     = lipgloss.Color("#11233f")   // --selbg (selection background)
```

### Typography (4 sizes, no more — LOCKED)

Terminal approximation via lipgloss styles (terminals don't have px sizes; use bold/dim as the signal):

```
t-display → Bold + colorAmber   (brand "BEEKEEPER" — the amber display word)
t-head    → Bold + colorWhite   (panel title text)
t-body    → Normal + colorFg    (main content)
t-micro   → Normal + colorDim   (timestamps, labels, keys, hints)
t-label   → Normal + colorDim + uppercase  (structural labels e.g. "PROCESS TREE")
w-bold    → Bold + colorWhite   (weight as binary signal — selected items)
```

### Layout: Calm Base Screen

```
┌─ titlebar ─────────────────────────────────────────────────────┐
│ ● ● ●  beekeeper dashboard — calm mode          HH:MM:SS       │
├────────────────────────────────────────────────────────────────┤
│                                                                  │
│  BEEKEEPER  all systems nominal · protecting 4 agents · …       │
│                                                                  │
│  ● hooks  ● gateway  ● sentry  ● catalogs fresh  last block 6m  │
│                                                                  │
│  [incident card — only visible in critical mode]                 │
│                                                                  │
│  ──────────────────────────────────────────────────────         │
│                                                                  │
│  :  command palette     !  jump to alerts     g  go to…         │
│  ?  help     q  quit                                             │
│  Beekeeper stays quiet until something needs you. Press : …      │
│                                                                  │
├─ keybar ───────────────────────────────────────────────────────┤
│ :  palette · !  alerts · g  go to · ?  help · q  quit          │
└────────────────────────────────────────────────────────────────┘
```

- Titlebar: `colorPanel2` bg, `colorBorder` bottom border. Three "traffic light" dots (●) in red/yellow/green. Title in `colorDim`. Clock in `colorDimmer` (right-aligned). Live clock, ticks every second.
- Status line: brand word `BEEKEEPER` in `colorAmber` + bold. Status message in `colorDim`. In critical mode: status message turns `colorRed` + bold.
- Health pips row: 8px Unicode circle `●` (filled) colored `colorGreen` normally, `colorRed` + pulsing spinner in critical, `colorAmber` for degraded. Labels in `colorDim`. "last block X ago" in `colorDimmer`.
- Incident slot: empty string in calm. Incident card (red-bordered box) in critical mode.
- Rule: thin horizontal line in `colorBorderDim`.
- Hint row: `colorDimmer` base. Keys in `colorTeal` + bold. Action labels in `colorDim`.
- Keybar: `colorPanel2` bg, `colorBorder` top border. Keys in `colorTeal`. Separators `·` in `colorDimmer`.

### Command Palette Overlay

Appears over blurred base (dim everything behind it). Centered vertically near top (roughly 20% from top).

```
┌────────────────── command palette ─────────────────────┐
│ : filter text▌                                          │
├─────────────────────────────────────────────────────────┤
│  SCAN                                                    │
│  ▸ scan now          run a deep bumblebee sweep         │
│  ▸ scan --quick      lockfiles + extensions only        │
│  ▸ scan history      view past scan results             │
│  INVESTIGATE                                             │
│  ▸ alerts            open the sentry alert log          │ ← selected (colorSelbg bg, teal left border)
│  ▸ quarantine        review held items                  │
│  ▸ audit tail        stream the raw event log           │
│  CONFIGURE                                               │
│  ▸ policy edit       tune rules & thresholds            │
│  ▸ catalogs          source status & sync               │
│  ▸ protect install   enable privileged sentry daemon    │
│  SYSTEM                                                  │
│  ▸ help              keybindings & concepts             │
│  ▸ quit              exit dashboard                     │
└─────────────────────────────────────────────────────────┘
```

- Border: `colorTeal`
- Input row: `colorPanel2` bg, prompt `:` in `colorTeal`, typed text in `colorFg`, blinking block cursor
- Group labels: `colorDim` + uppercase + letterSpacing (terminal: render as `"  SCAN  "` in `colorDim`)
- Items: arrow `▸` in `colorDimmer`, name in `colorFg` (min 20 chars padded), desc in `colorDim`
- Selected item: `colorSelbg` background, left-side `colorTeal` bar (in lipgloss: left border `colorTeal`), name bold + `colorWhite`, arrow `▸` in `colorTeal`
- Keys: `↑↓` navigate, `Enter` run, `Esc` close, typing filters

### Panel Overlay

Generic overlay system for: alerts, quarantine, catalogs, policy, audit tail, scan, help.

```
┌─ panel head ──────────────────────── count/meta ─┐
│ Panel Title                               3 items │
├──────────────────────────────────────────────────┤
│  [panel-specific content]                         │
│                                                   │
├──────────────────────────────────────────────────┤
│ ↑↓ select  · enter inspect  · q quarantine  · esc│
└──────────────────────────────────────────────────┘
```

- Panel border: `colorBorder` (normal), `colorRed` for alert panel in critical mode
- Panelhead: `colorPanel2` bg, `colorBorderDim` bottom border. Title bold + `colorWhite`. Count in `colorDimmer`.
- Panelbody: `colorPanel` bg.
- Panelfoot: `colorPanel2` bg, `colorBorderDim` top border. Keys in `colorTeal`. Labels in `colorDim`.

**Row items** (alerts, quarantine):
```
  HH:MM:SS   CRIT   ext-host-cred-cluster       read ~/.aws ~/.ssh
  [time]     [badge] [label]                     [meta]
```
- time: `colorDim` + micro
- badge: colored background chip (see badges below)
- label: `colorFg`; selected: bold + `colorWhite`
- meta: `colorDim` + micro + right-aligned
- Selected row: `colorSelbg` bg, `colorTeal` left border (2px)

**Padded panels** (catalogs, policy, help, audit): padding inside panelbody, text-only content.

### Badges (filled chips — terminal: [CRIT], [BLOCK], etc. with bg color)

```
b-crit  → bg colorRed,   fg "#0d1117"  — text: CRIT
b-block → bg colorCoral, fg "#0d1117"  — text: BLOCK
b-warn  → bg colorAmber, fg "#0d1117"  — text: WARN
b-ok    → bg colorGreen, fg "#0d1117"  — text: OK
b-held  → bg colorCoral, fg "#0d1117"  — text: HELD
```

In terminal: `lipgloss.NewStyle().Background(colorRed).Foreground(lipgloss.Color("#0d1117")).Bold(true).Padding(0,1).Render("CRIT")`

### Incident Card (inline in base screen — critical mode only)

```
┌─[CRITICAL] exfil-signature-fusion ─────── 14:21:54 · sentry · rule R5 ─┐
│ (red border, critglow gradient bg)                                        │
│ A process from the VS Code extension host read three credential           │
│ files and opened an outbound connection, within 4 minutes of installing   │
│ an extension flagged by 2 of 3 catalogs.                                  │
│                                                                            │
│ PROCESS TREE                                                               │
│ Code Helper (Plugin) pid 8821                                              │
│ └─ node extension.js pid 8847                                              │
│    ├─ read ~/.aws/credentials                                              │
│    ├─ read ~/.config/op/config                                             │
│    ├─ read ~/.ssh/id_ed25519                                               │
│    └─ POST 185.2.x.x:443  4.2KB                                           │
├────────────────────────────────────────────────────────────────────────────│
│ [Q] quarantine extension  [I] isolate process  [d] full record            │
│                                        rotate exposed creds after containment │
└────────────────────────────────────────────────────────────────────────────┘
```

- Border: `colorRed`
- Inc-head: CRITICAL badge, title bold + `colorWhite`, timestamp in `colorDim` right-aligned
- Inc-body: description in `colorFg`. "PROCESS TREE" label in `colorDim` + uppercase. Process tree:
  - Parent process: `colorDim`
  - Malicious process (node extension.js): `colorCoral`
  - File reads (`.aws`, `.ssh`, etc.): `colorRed`
  - Network: `colorRed`
  - PIDs: `colorDimmer`
- Inc-actions border-top: `colorRed` (dim)
- Action buttons: border `colorBorder`, bg `colorPanel`. Selected: border `colorTeal`, bg `colorSelbg`, text `colorWhite`. Danger key `colorCoral`, warn key `colorAmber`, info key `colorTeal`.
- Tail note: `colorDim` + right-aligned

### Toast notification (bottom center, ephemeral)

Renders at bottom. Success: `●` icon in `colorGreen`. Warn: `colorAmber`. Auto-clears after ~2.2 seconds. Renders as a small boxed line at bottom of screen.

### Panel Content Specs

**Alerts panel:** 4 rows (see prototype). critical mode adds extra CRIT row at top.
- Footer keys: `↑↓ select · enter inspect · q quarantine · esc close`

**Quarantine panel:** HELD rows.
- Footer keys: `r restore · p purge · enter details · esc close`

**Catalogs panel (padded):**
```
● bumblebee   threat_intel     synced 4m ago · +3 new entries
● osv         offline db       synced 6h ago · daily
● socket      public api       live query · rate-limited
● self        beekeeper-self   clean · this build not flagged

Enforcement requires 2 of 3 independent sources to agree.
A single source can warn but cannot block — the 2FA principle
applied to threat intelligence.
```
Footer: `s sync all · esc close`

**Policy panel (padded):**
```
corroboration    single → warn  two → enforce  three → quarantine
release-age      1440 min (24h) · npm pypi cargo gem composer
lifecycle        deny by default · allowlist 3 pkgs
sentry baseline  day 3 of 7 · audit-only until day 7
llamafirewall    enabled · sample 1.0

Declarative JSON, version-controlled, testable with
beekeeper policy test <file>
```
Footer: `e open in $EDITOR · t test · esc close`

**Audit tail panel (padded, micro text):**
```
{"t":"14:22:07","decision":"allow","tool":"Read","target":"main.go"}
{"t":"14:22:01","decision":"block","tool":"Bash","pkg":"chalk@5.4.0","rule":"release-age"}
{"t":"14:21:54","decision":"alert","rule":"R5","sev":"critical","sources":["bumblebee","osv"]}  ← colorRed
{"t":"14:21:50","decision":"allow","tool":"Edit","target":"prd.md"}
{"t":"14:21:43","decision":"allow","tool":"Read","target":"go.sum"}
```
Footer: `esc close`

**Scan panel (with progress animation):**
Deep/quick mode: shows animated progress steps, then results. History mode: shows past runs.
Progress steps: `▸` in `colorGreen` + step text in `colorDim`. Scan complete: `✓ scan complete` in `colorGreen` + results.

**Help panel (padded):**
```
NAVIGATION
:    open command palette (do anything)
!    jump straight to alerts
g    go-to menu
esc  close any overlay

CONCEPT
[explanation text in colorDim]
```

### Package Structure

```
internal/tui/
  model.go              # App (top-level tea.Model), mode routing (calm/palette/panel/critical)
  base.go               # calm base screen renderer (brand, status, health pips, hint, keybar)
  palette.go            # command palette overlay model + view
  panel.go              # generic panel overlay model + PanelContent interface
  incidents.go          # incident card model + view (critical mode)
  alerts_panel.go       # alerts panel content + row selection
  quarantine_panel.go   # quarantine panel content + keybindings
  catalogs_panel.go     # catalogs padded panel content
  policy_panel.go       # policy padded panel content
  audit_panel.go        # audit tail padded panel content
  scan_panel.go         # scan panel with progress animation
  help_panel.go         # help padded panel content
  styles.go             # all lipgloss styles derived from the locked color palette
  watcher.go            # audit log file-watcher (fsnotify parent-dir + 1s polling fallback)
  resize_windows.go     # 500ms poll, //go:build windows
  resize_other.go       # no-op, //go:build !windows
  toast.go              # ephemeral toast notification model
  model_test.go
  alerts_panel_test.go
  quarantine_panel_test.go
  catalogs_panel_test.go
  scan_panel_test.go
```

> **NOTE:** All panel files live directly in `internal/tui/` (flat layout, no `panels/` subdirectory).
> This avoids import cycles — all types are in package `tui` and can reference each other freely.

### App State Machine

```go
type mode int
const (
    modeCalm    mode = iota  // base screen
    modePalette              // command palette open
    modePanel                // a panel overlay is open
)

type panelKind string
const (
    panelAlerts     panelKind = "alerts"
    panelQuarantine panelKind = "quarantine"
    panelCatalogs   panelKind = "catalogs"
    panelPolicy     panelKind = "policy"
    panelAudit      panelKind = "audit"
    panelScan       panelKind = "scan"
    panelHelp       panelKind = "help"
)

type App struct {
    mode      mode
    critical  bool         // incident card visible
    panel     panelKind    // which panel is open (only meaningful in modePanel)
    palette   PaletteModel
    panelM    PanelModel   // current panel content
    incident  IncidentModel
    toast     ToastModel
    health    HealthState   // pip states, read from IPC + audit
    status    string       // status line message
    clock     time.Time
    width, height int
    adminMode bool
    auditPath string
}
```

### Commands (from prototype — LOCKED order and grouping)

```go
var commands = []Command{
    // grp: "scan"
    {Grp: "scan",        Name: "scan now",        Desc: "run a deep bumblebee sweep"},
    {Grp: "scan",        Name: "scan --quick",     Desc: "lockfiles + extensions only"},
    {Grp: "scan",        Name: "scan history",     Desc: "view past scan results"},
    // grp: "investigate"
    {Grp: "investigate", Name: "alerts",           Desc: "open the sentry alert log"},
    {Grp: "investigate", Name: "quarantine",       Desc: "review held items"},
    {Grp: "investigate", Name: "audit tail",       Desc: "stream the raw event log"},
    // grp: "configure"
    {Grp: "configure",   Name: "policy edit",      Desc: "tune rules & thresholds"},
    {Grp: "configure",   Name: "catalogs",         Desc: "source status & sync"},
    {Grp: "configure",   Name: "protect install",  Desc: "enable privileged sentry daemon"},
    // grp: "system"
    {Grp: "system",      Name: "help",             Desc: "keybindings & concepts"},
    {Grp: "system",      Name: "quit",             Desc: "exit dashboard"},
}
```

### Key Bindings (from prototype — LOCKED)

**Calm mode (no overlay):**
- `:` → open palette
- `!` → open alerts panel directly
- `?` → open help panel directly
- `g` or `G` → open palette pre-filtered with "go"
- `x` or `X` → trigger critical simulation (demo)
- `q` → quit

**Critical mode (incident card visible):**
- `Q` / `q` → quarantine extension (action 0)
- `I` / `i` → isolate process (action 1)
- `d` / `D` → full record (opens alerts panel)
- `↑`/`←` → select previous action
- `↓`/`→` → select next action
- `Enter` → run selected action

**Palette mode:**
- `↑`/`↓` → navigate items
- `Enter` → run selected command
- `Esc` → close palette
- `Backspace` → delete last char
- Any printable char → append to filter query

**Panel mode:**
- `Esc` → close panel
- `↑`/`↓` → scroll / navigate rows
- Panel-specific: `q` (alerts→quarantine), `r` (quarantine→restore), `p` (quarantine→purge), `s` (catalogs→sync)

</decisions>

<canonical_refs>
## Canonical References

- `beekeeper-tui-prototype.html` — THE locked design reference (in repo root)
- `CLAUDE.md` — Bubble Tea `charm.land/bubbletea/v2` import, Windows resize workaround
- `.planning/REQUIREMENTS.md` — TUI-01 through TUI-10
- `.planning/ROADMAP.md` — Phase 8 success criteria
- `.planning/phases/08-tui-dashboard/08-RESEARCH.md` — Bubble Tea v2 API findings (teatest incompatible, View() returns tea.View, KeyPressMsg not KeyMsg, fsnotify directory watch)

### Prior Phase Code to Consume
- `internal/audit/types.go` — AuditRecord fields
- `internal/audit/query.go` — NDJSON tail pattern for live feed
- `internal/quarantine/` — List/Restore/Purge
- `internal/ipc/client.go` + `internal/ipc/pipe_windows.go` — StatusResponse for health pips
- `internal/catalog/watch.go` — WatchState for catalog freshness
- `internal/scan/scanner.go` — Scan() for scan panel trigger
- `cmd/beekeeper/main.go` — add `newDashboardCmd()` here

</canonical_refs>

<specifics>
## Specific Implementation Notes

### Bubble Tea v2 API (from RESEARCH.md — CRITICAL)
- `View()` returns `tea.View`, NOT `string`. Use `tea.NewView(content)`.
- Key messages are `tea.KeyPressMsg` (not `tea.KeyMsg`). `msg.String()` for key text.
- Alt-screen: set `v.AltScreen = true` on the returned `tea.View` — not a program option.
- `teatest` is INCOMPATIBLE with v2 — use direct `Update()`/`viewString()` unit tests.
- fsnotify: watch the PARENT DIRECTORY, filter by `event.Name == auditPath`.

### Blurred base in overlay modes
When palette or panel is open, the base screen renders dimmed. In terminal: wrap all base content in a lipgloss style that sets foreground to `colorDimmer` (approximates the CSS `opacity:.32`). The palette/panel renders on top as a separate string via `lipgloss.Place()`.

### Scan progress animation
Steps print one per 480ms tick (match prototype). Use `tea.Tick(480*time.Millisecond, ...)` command. 4 defined steps, followed by completion message.

### Health pip pulse (critical mode)
Use spinner in sentry pip when critical. In calm mode: static `●` in `colorGreen`.

### Clock
Live clock using `tea.Every(time.Second, func(t time.Time) tea.Msg { return clockMsg(t) })`.

### Windows resize workaround (TUI-10)
`resize_windows.go` with `//go:build windows`: polls `term.GetSize` every 500ms, sends `tea.WindowSizeMsg`. `resize_other.go` with `//go:build !windows`: no-op `StartResizePoller`.

### Admin mode (`--admin`)
When `--admin` is set, the quarantine panel enables `r`/`p` actions, the policy panel enables `e`/`t` actions. Without `--admin`, those keys produce a toast "Use --admin to enable write actions."

### Self-defense
- TUI renders only from audit log data and IPC responses. No eval of audit content.
- Admin actions route through existing `internal/quarantine`, `internal/scan` packages.
- No network connections in TUI itself.

</specifics>

<deferred>
## Deferred

- `beekeeper diag` command → Phase 9
- `beekeeper-self` catalog live check in health pips → Phase 9
- Policy-as-code beyond on/off toggle → Phase 9 (CODE-01)
- SSH-accessible TUI → existing terminal over SSH works natively
- Web UI / desktop GUI → out of scope v1

</deferred>

---

*Phase: 08-tui-dashboard*
*Context updated: 2026-05-28 — design locked to beekeeper-tui-prototype.html; package structure updated to flat internal/tui/ layout (no panels/ subdirectory)*
