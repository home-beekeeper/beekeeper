# Phase 10: Hook-Block Protocol Compliance & Multi-Harness Enforcement — Pattern Map

**Mapped:** 2026-06-05
**Files analyzed:** 14 new/modified files
**Analogs found:** 14 / 14

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/check/deny_render.go` | deny-renderer (new) | request-response | `internal/check/handler.go` (exitCodeFor, finalizeWithAC) | role-match |
| `internal/check/deny_render_test.go` | table-test (new) | request-response | `internal/check/paths_test.go` (cases/tc table pattern) | exact |
| `internal/hooks/hooks.go` | dispatch router (modify) | request-response | self (existing) | exact |
| `internal/hooks/claude_code.go` | per-harness installer (modify: update command string) | request-response | self (existing) | exact |
| `internal/hooks/cursor.go` | per-harness installer (fix: wrong event names) | request-response | `internal/hooks/claude_code.go` | exact |
| `internal/hooks/codex.go` | per-harness installer (fix: add features flag) | request-response | `internal/hooks/claude_code.go` | exact |
| `internal/hooks/augment.go` | per-harness installer (new) | request-response | `internal/hooks/claude_code.go` | exact |
| `internal/hooks/codebuddy.go` | per-harness installer (new) | request-response | `internal/hooks/claude_code.go` | exact |
| `internal/hooks/qwen.go` | per-harness installer (new) | request-response | `internal/hooks/claude_code.go` | exact |
| `internal/hooks/copilot.go` | per-harness installer (new) | request-response | `internal/hooks/cursor.go` + `claude_code.go` | role-match |
| `internal/hooks/gemini.go` | per-harness installer (new) | request-response | `internal/hooks/codex.go` | role-match |
| `internal/hooks/antigravity.go` | per-harness installer (new) | request-response | `internal/hooks/codex.go` | role-match |
| `internal/hooks/windsurf.go` | per-harness installer (new) | request-response | `internal/hooks/cursor.go` | role-match |
| `internal/hooks/cline.go` | per-harness installer (new, macOS/Linux only) | request-response | `internal/hooks/cursor.go` | role-match |
| `internal/hooks/hermes.go` | per-harness installer (new, fail-open harness) | request-response | `internal/hooks/codex.go` | role-match |
| `internal/hooks/opencode_plugin.go` | per-harness installer (new, plugin path) | request-response | `internal/hooks/gateway_targets.go` (printGuide pattern) | role-match |
| `internal/hooks/kilo_trae.go` | gateway-routing docs (new) | request-response | `internal/hooks/gateway_targets.go` | exact |
| `internal/hooks/hooks_test.go` | per-harness installer tests (modify: add new harnesses) | request-response | self (existing) | exact |
| `cmd/beekeeper/main.go` | CLI wiring (modify: add --hook flag to checkCmd) | request-response | self newCheckCmd() lines 245-308 | exact |
| `docs/harness-support-matrix.md` | support matrix docs (new) | — | `internal/hooks/gateway_targets.go` (printGuide text as template) | partial |

---

## Pattern Assignments

### `internal/check/deny_render.go` (deny-renderer, new)

**Analog:** `internal/check/handler.go`

**Core pattern — Decision struct and exitCodeFor** (`internal/check/handler.go` lines 78-83, 443-448):
```go
// Result is the outcome of a single check.
type Result struct {
    Decision policy.Decision
    ExitCode int
}

func exitCodeFor(d policy.Decision) int {
    if d.Allow {
        return exitAllow
    }
    return exitBlock
}
```

**New constants to add** (alongside existing `exitAllow=0`, `exitBlock=1`):
```go
// exitHookBlock is the exit code emitted when --hook <harness> is active and the
// decision is block. EVERY hook-capable harness (except Hermes) honors exit 2 as
// deny. Hermes is fail-open on exit codes; its block path is JSON-only.
const exitHookBlock = 2
```

**Core pattern — table-driven deny renderer** (greenfield; follows exitCodeFor's pure-function style):
```go
// HarnessID is the canonical name passed to --hook (matches installer target names).
type HarnessID string

const (
    HarnessClaudeCode  HarnessID = "claude-code"
    HarnessCursor      HarnessID = "cursor"
    HarnessCodex       HarnessID = "codex"
    HarnessAugment     HarnessID = "augment"
    // ... one const per harness
)

// DenyOutput is the complete deny rendering for a blocked tool call under a
// specific harness contract. Callers write Stdout to os.Stdout, Stderr to
// os.Stderr, then call os.Exit(ExitCode).
type DenyOutput struct {
    Stdout   []byte // harness-specific JSON (nil = nothing to stdout)
    Stderr   []byte // human-readable reason (always non-nil on block)
    ExitCode int    // 2 for all hook-capable harnesses; special for Hermes
}

// RenderDeny is a pure, table-driven function: no I/O, no side effects.
// It maps (harness, decision) → DenyOutput. Tests call this directly.
// On allow/warn the caller should use the default path (exit 0, no JSON).
func RenderDeny(h HarnessID, d policy.Decision) DenyOutput {
    // table: harness → render function
}
```

**Imports pattern** (mirror handler.go):
```go
import (
    "encoding/json"
    "fmt"

    "github.com/home-beekeeper/beekeeper/internal/policy"
)
```

**Deny JSON shapes per family** (from RESEARCH.md §3):
```go
// Family A: nested hookSpecificOutput (Claude Code, Codex, CodeBuddy, Augment, Qwen)
type hookSpecificOutputDeny struct {
    HookSpecificOutput struct {
        HookEventName          string `json:"hookEventName"`
        PermissionDecision     string `json:"permissionDecision"`     // "deny"
        PermissionDecisionReason string `json:"permissionDecisionReason"`
    } `json:"hookSpecificOutput"`
}

// Family B: flat permissionDecision (Copilot)
type copilotDeny struct {
    PermissionDecision       string `json:"permissionDecision"`       // "deny"
    PermissionDecisionReason string `json:"permissionDecisionReason"`
}

// Family C: permission field (Cursor)
type cursorDeny struct {
    Permission   string `json:"permission"`    // "deny"
    UserMessage  string `json:"user_message"`
    AgentMessage string `json:"agent_message"`
}

// Family D: decision field (Gemini CLI)
type geminiDeny struct {
    Decision string `json:"decision"` // "deny"
    Reason   string `json:"reason"`
}

// Family E: dual defensive (Antigravity — MED-confidence docs)
type antigravityDeny struct {
    Decision           string `json:"decision"`            // "deny"
    PermissionDecision string `json:"permissionDecision"`  // "deny"
    DenyReason         string `json:"denyReason"`
}

// Family F: cancel (Cline)
type clineDeny struct {
    Cancel       bool   `json:"cancel"`        // true
    ErrorMessage string `json:"errorMessage"`
}

// Family G: Hermes fail-open — JSON is the ONLY block path (exit codes ignored)
type hermesDeny struct {
    Action  string `json:"action"`  // "block"
    Message string `json:"message"` // REQUIRED: non-empty
}

// Family H: Windsurf — exit 2 ONLY, no stdout JSON
// (DenyOutput.Stdout = nil)
```

---

### `internal/check/deny_render_test.go` (table-test, new)

**Analog:** `internal/check/paths_test.go` (lines 272-299) and `internal/check/handler_test.go`

**Table-test shape** (copy from `internal/check/paths_test.go` lines 272-280):
```go
cases := []struct {
    name     string
    harness  HarnessID
    decision policy.Decision
    wantExit int
    wantStdoutContains string
    wantStderrContains string
}{
    {
        name:     "claude-code block emits exit 2 + hookSpecificOutput deny",
        harness:  HarnessClaudeCode,
        decision: policy.Decision{Allow: false, Level: "block", Reason: "credential read"},
        wantExit: 2,
        wantStdoutContains: `"permissionDecision":"deny"`,
        wantStderrContains: "credential read",
    },
    // ... one row per harness per family
    {
        name:     "hermes block exit is 0 (fail-open harness — JSON is the block)",
        harness:  HarnessHermes,
        decision: policy.Decision{Allow: false, Level: "block", Reason: "policy"},
        wantExit: 0, // Hermes ignores exit code; JSON carries the block
        wantStdoutContains: `"action":"block"`,
    },
    {
        name:     "windsurf block exit 2 no stdout",
        harness:  HarnessWindsurf,
        decision: policy.Decision{Allow: false, Level: "block", Reason: "policy"},
        wantExit: 2,
        wantStdoutContains: "", // Windsurf: exit-2 only, no JSON
    },
    {
        name:     "allow path: no harness output",
        harness:  HarnessClaudeCode,
        decision: policy.Decision{Allow: true, Level: "allow"},
        wantExit: 0,
        wantStdoutContains: "",
    },
}

for _, tc := range cases {
    t.Run(tc.name, func(t *testing.T) {
        out := RenderDeny(tc.harness, tc.decision)
        if out.ExitCode != tc.wantExit { ... }
        if tc.wantStdoutContains != "" && !strings.Contains(string(out.Stdout), tc.wantStdoutContains) { ... }
        if tc.wantStderrContains != "" && !strings.Contains(string(out.Stderr), tc.wantStderrContains) { ... }
    })
}
```

**Imports pattern** (mirror handler_test.go lines 1-17):
```go
package check

import (
    "strings"
    "testing"

    "github.com/home-beekeeper/beekeeper/internal/policy"
)
```

---

### `cmd/beekeeper/main.go` — `newCheckCmd()` modification

**Analog:** self, `newCheckCmd()` lines 245-308. The `--tool` flag is the exact precedent for how a new optional flag branches the stdin construction without breaking the default path.

**Existing flag wiring pattern** (lines 284-308):
```go
// Shim invocation: build ToolCall JSON from flags using json.Marshal.
var stdin io.Reader = os.Stdin
if toolName != "" {
    tc := map[string]any{ ... }
    data, merr := json.Marshal(tc)
    ...
    stdin = strings.NewReader(string(data))
}

result := check.RunCheck(cmd.Context(), stdin, cfg, indexPath, auditPath, catalogDir)
os.Exit(result.ExitCode)
```

**New `--hook` flag pattern** (add alongside existing `--tool`/`--args` flags):
```go
var hookTarget string
cmd.Flags().StringVar(&hookTarget, "hook", "", "Harness name for hook invocations (emits exit 2 + harness-specific deny JSON on block)")

// In RunE, after result := check.RunCheck(...):
if hookTarget != "" && !result.Decision.Allow {
    out := check.RenderDeny(check.HarnessID(hookTarget), result.Decision)
    if len(out.Stdout) > 0 {
        fmt.Fprint(os.Stdout, string(out.Stdout))
    }
    if len(out.Stderr) > 0 {
        fmt.Fprint(os.Stderr, string(out.Stderr))
    }
    os.Exit(out.ExitCode) // exit 2 (or 0 for Hermes)
}
// default path: unchanged (exit 1 = block, exit 0 = allow)
os.Exit(result.ExitCode)
```

**Constraint:** default (no `--hook`) behavior is UNCHANGED — raw Decision JSON to stdout, exit 0/1. The `--hook` branch only activates when the flag is set AND the decision is block. On allow/warn with `--hook` set, exit 0 and emit nothing harness-specific (CONTEXT.md constraint 3).

---

### `internal/hooks/hooks.go` (modify: add 12 new targets)

**Analog:** self. Pattern to replicate is the `switch target` in `InstallTo` and `UninstallTo` (lines 65-87, 101-129).

**New target constants** (add alongside existing TargetClaudeCode, TargetCursor, TargetCodex):
```go
const (
    TargetAugment     = "augment"
    TargetAntigravity = "antigravity"
    TargetCline       = "cline"
    TargetCodeBuddy   = "codebuddy"
    TargetCopilot     = "copilot"
    TargetGemini      = "gemini"
    TargetHermes      = "hermes"
    TargetKilo        = "kilo"
    TargetOpenCode    = "opencode"   // already exists (gateway); needs update to add plugin path
    TargetQwen        = "qwen"
    TargetTrae        = "trae"
    TargetWindsurf    = "windsurf"
)
```

**Switch extension pattern** (copy shape of lines 65-87):
```go
case TargetAugment:
    settingsPath := augmentSettingsPath(homeDir)
    return installAugment(settingsPath, dryRun, out)
// ... one case per new harness
case TargetKilo, TargetTrae:
    return printGatewayGuide(target, out)
```

**gatewayTargets map update** (add Kilo, Trae; remove OpenCode if it gets a plugin installer):
```go
var gatewayTargets = map[string]bool{
    TargetContinue: true,
    TargetKilo:     true,
    TargetTrae:     true,
    TargetOpenClaw: true,
}
```

---

### `internal/hooks/claude_code.go` (modify: update command string)

**Analog:** self. Change `claudePreCommand` from `"beekeeper check"` to `"beekeeper check --hook claude-code"` so the installed hook emits exit 2 + hookSpecificOutput JSON on block.

```go
const (
    claudePreCommand  = "beekeeper check --hook claude-code"  // was: "beekeeper check"
    claudePostCommand = "beekeeper audit-record"              // unchanged
)
```

All merge/uninstall helpers use `claudePreCommand` as the sentinel string, so changing the constant propagates correctly to idempotency detection and targeted uninstall — no other logic changes needed.

---

### `internal/hooks/cursor.go` (fix: wrong event names + deny JSON)

**Analog:** self. Critical bug: `existing.Hooks["preToolUse"]` → must be three separate event keys.

**Current broken pattern** (line 79 — writes non-existent event):
```go
existing.Hooks["preToolUse"] = append(existing.Hooks["preToolUse"], beekeeperHook)
```

**Fixed pattern** (replace with correct Cursor events per RESEARCH.md):
```go
// Cursor v1.7+ hook events (NOT "preToolUse" which does not exist in Cursor).
// Each event maps to its own array of hook entries in hooks.json.
for _, event := range []string{"beforeShellExecution", "beforeMCPExecution", "beforeReadFile"} {
    if !containsCursorHookByCommand(existing.Hooks[event], "beekeeper check --hook cursor") {
        existing.Hooks[event] = append(existing.Hooks[event], cursorHook{
            Command:    "beekeeper check --hook cursor",
            Type:       "command",
            Timeout:    10,
            Matcher:    ".*",
            FailClosed: true, // REQUIRED: Cursor is fail-OPEN by default
        })
    }
}
```

**Uninstall fix** (iterate all three event keys, not just "preToolUse").

---

### `internal/hooks/codex.go` (fix: add `[features] hooks=true`)

**Analog:** self. The hooks.json write is correct structure; the missing piece is a TOML config patch.

**New helper** (add `ensureCodexFeaturesFlag` alongside existing install logic):
```go
// codexConfigPath returns the path to Codex's config.toml.
func codexConfigPath(homeDir string) string {
    return homeDir + "/.codex/config.toml"
}

// ensureCodexFeaturesFlag ensures [features]\nhooks=true is present in
// ~/.codex/config.toml. Idempotent. Required for Codex hook execution (PR #18385).
func ensureCodexFeaturesFlag(configPath string, out io.Writer) error { ... }
```

**Updated command string:**
```go
func beekeeperCodexPreToolUse() codexHookEntry {
    return codexHookEntry{
        Matcher: ".*",
        Hooks: []codexHookCmd{
            {Type: "command", Command: "beekeeper check --hook codex", Timeout: 10},
        },
    }
}
```

---

### `internal/hooks/augment.go`, `codebuddy.go`, `qwen.go` (new — Claude Code family)

**Analog:** `internal/hooks/claude_code.go` (exact pattern — same hookSpecificOutput schema)

These three harnesses use the identical Claude Code schema (nested `hookSpecificOutput.permissionDecision:"deny"`, `PreToolUse` event, settings.json merge via `editorinit.PatchSettings`).

**Imports pattern** (copy claude_code.go lines 1-11):
```go
package hooks

import (
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "os"

    "github.com/home-beekeeper/beekeeper/internal/editorinit"
)
```

**Install function shape** (copy `installClaudeCode` lines 120-149, substituting path/command):
```go
func installAugment(settingsPath string, dryRun bool, out io.Writer) error {
    // identical body to installClaudeCode but with:
    // - augmentPreCommand  = "beekeeper check --hook augment"
    // - augmentPostCommand = "beekeeper audit-record"
    // - uses mergeClaudeHookEntry (same schema)
}
```

**Path helpers:**
```go
func augmentSettingsPath(homeDir string) string  { return homeDir + "/.augment/settings.json" }
func codebuddySettingsPath(homeDir string) string { return homeDir + "/.codebuddy/settings.json" }
func qwenSettingsPath(homeDir string) string     { return homeDir + "/.qwen/settings.json" }
```

---

### `internal/hooks/copilot.go` (new — flat permissionDecision family)

**Analog:** `internal/hooks/claude_code.go` (merge pattern) + `internal/hooks/cursor.go` (typed struct)

Copilot uses a flat (non-nested) `{"permissionDecision":"deny","permissionDecisionReason":"..."}` on stdout, OR exit 2. Settings file is `~/.copilot/settings.json` or `.github/hooks/*.json`.

**Key divergence from claude_code.go:** event key is `"preToolUse"` (camelCase, same as the broken Cursor — but this is correct for Copilot). Deny rendered by `RenderDeny(HarnessCopilot, d)` produces flat JSON.

**Install shape** (same `editorinit.PatchSettings` merge pattern as claude_code.go; command updated):
```go
const copilotPreCommand = "beekeeper check --hook copilot"
```

---

### `internal/hooks/gemini.go` (new — Gemini-native decision field)

**Analog:** `internal/hooks/codex.go` (codexHooksFile struct → adapt to Gemini's `hooks` array in settings.json)

Gemini CLI uses `BeforeTool` event, `{"decision":"deny","reason":"..."}` stdout JSON, OR exit 2. Matcher = regex on tool name.

**Struct shape** (adapt codexHooksFile):
```go
type geminiHooksFile struct {
    Hooks []geminiHookEntry `json:"hooks"`
}

type geminiHookEntry struct {
    Event   string `json:"event"`   // "BeforeTool"
    Matcher string `json:"matcher"` // regex
    Command string `json:"command"`
}
```

---

### `internal/hooks/antigravity.go` (new — dual-defensive deny fields)

**Analog:** `internal/hooks/claude_code.go` (merge + editorinit.PatchSettings)

Config location: `~/.gemini/antigravity` or `.agents/hooks.json`. Because the deny-field name is MED-confidence (docs conflict), `RenderDeny` emits BOTH `decision:"deny"` AND `permissionDecision:"deny"` + `denyReason`.

---

### `internal/hooks/windsurf.go` (new — exit-2-only, no JSON)

**Analog:** `internal/hooks/cursor.go` (typed struct, hooks.json file)

Windsurf uses `~/.codeium/windsurf/hooks.json`, events: `pre_run_command` / `pre_mcp_tool_use` / `pre_read_code`. **Exit 2 ONLY — no stdout JSON deny form.** Windows uses `powershell` key; Linux/macOS use `command` key.

**Struct shape:**
```go
type windsurfHooksFile struct {
    Hooks map[string][]windsurfHook `json:"hooks"`
}

type windsurfHook struct {
    Command    string `json:"command,omitempty"`     // Linux/macOS
    PowerShell string `json:"powershell,omitempty"`  // Windows
    Timeout    int    `json:"timeout,omitempty"`
}
```

**Path helper:**
```go
func windsurfHooksPath(homeDir string) string {
    return homeDir + "/.codeium/windsurf/hooks.json"
}
```

---

### `internal/hooks/cline.go` (new — macOS/Linux only, executable file installer)

**Analog:** `internal/hooks/cursor.go` (install/uninstall shape); BUT the install target is an executable file, not a JSON config.

Cline hook = executable file named `PreToolUse` (no extension) in `.clinerules/hooks/` (project-local) or `~/Documents/Cline/Rules/Hooks/` (global).

**Key difference:** writes an executable shell script, not JSON.

**Build constraint** (add at top of file):
```go
//go:build !windows
// +build !windows
```

**Install shape:**
```go
func installCline(hooksDir string, dryRun bool, out io.Writer) error {
    hookPath := filepath.Join(hooksDir, "PreToolUse")
    script := "#!/bin/sh\nbeekeeper check --hook cline\n"
    // write hookPath with mode 0o755 (executable)
    // backup if exists
}
```

---

### `internal/hooks/hermes.go` (new — fail-open harness, JSON-ONLY block path)

**Analog:** `internal/hooks/codex.go` (YAML config + hooks file); key difference: exit codes are IGNORED by Hermes, so `RenderDeny` must always emit `{"action":"block","message":"..."}` to stdout regardless of other harnesses.

**Config path:** `~/.hermes/config.yaml` (YAML, not JSON — use `gopkg.in/yaml.v3` or manual string patch).

**Render contract:**
```go
// HarnessHermes special case in RenderDeny:
// ExitCode = 0 (exit codes ignored by Hermes)
// Stdout   = {"action":"block","message":"<reason>"} (REQUIRED non-empty message)
// Stderr   = reason (best-effort)
```

---

### `internal/hooks/opencode_plugin.go` (new — plugin guide)

**Analog:** `internal/hooks/gateway_targets.go` (`printGuide` pattern, lines 13-125)

OpenCode uses a JS plugin (`tool.execute.before` hook, `throw new Error(...)`) rather than a CLI hook or JSON config. Ship a plugin template to `~/.config/opencode/plugins/beekeeper.js` that shells to `beekeeper check --hook opencode` and throws on deny.

**Shape** (mirror `printOpenCodeGuide` → upgrade to `installOpenCodePlugin`):
```go
func installOpenCodePlugin(pluginDir string, dryRun bool, out io.Writer) error {
    pluginPath := filepath.Join(pluginDir, "beekeeper.js")
    pluginContent := openCodePluginTemplate // JS template string
    // write file; backup if exists; print caveat about subagent bypass
}
```

---

### `internal/hooks/kilo_trae.go` (new — MCP gateway only)

**Analog:** `internal/hooks/gateway_targets.go` (exact printGuide pattern)

Kilo and Trae have no pre-exec hook (Kilo: open FR #5827; Trae: MCP-only). Installer = print guide routing to MCP gateway. Shape identical to `printContinueGuide`/`printOpenCodeGuide`.

---

### `internal/hooks/hooks_test.go` (modify: add new harness tests)

**Analog:** self. Each new harness installer follows the existing test shape:

**Per-harness test shape** (copy `TestInstallClaudeCode` lines 59-109 and `TestInstallClaudeCodePreservesExistingHooks` lines 413-493):
```go
func TestInstallAugment(t *testing.T) {
    t.Run("from_absent", func(t *testing.T) {
        dir := t.TempDir()
        settingsPath := filepath.Join(dir, "settings.json")
        var buf bytes.Buffer
        if err := installAugment(settingsPath, false, &buf); err != nil {
            t.Fatalf("installAugment: %v", err)
        }
        m := readJSON(t, settingsPath)
        if _, ok := m["hooks"]; !ok {
            t.Fatal("expected hooks key")
        }
    })
    t.Run("preserves_existing_hooks", func(t *testing.T) {
        // copy TestInstallClaudeCodePreservesExistingHooks shape
    })
    t.Run("idempotent", func(t *testing.T) { ... })
    t.Run("dry_run", func(t *testing.T) { ... })
    t.Run("uninstall_only_removes_beekeeper", func(t *testing.T) { ... })
}
```

**Cursor fix test** (add to existing `TestInstallCursor`):
```go
t.Run("correct_event_names", func(t *testing.T) {
    // Assert hooks.json contains "beforeShellExecution", NOT "preToolUse"
    var f cursorHooksFile
    data, _ := os.ReadFile(hooksPath)
    json.Unmarshal(data, &f)
    if _, ok := f.Hooks["preToolUse"]; ok {
        t.Fatal("preToolUse must NOT be written — it does not exist in Cursor")
    }
    for _, event := range []string{"beforeShellExecution", "beforeMCPExecution", "beforeReadFile"} {
        if len(f.Hooks[event]) == 0 {
            t.Fatalf("expected beekeeper hook under event %q", event)
        }
    }
})
```

---

## Shared Patterns

### Merge-not-clobber + targeted uninstall
**Source:** `internal/hooks/claude_code.go` (entire file — `mergeClaudeHookEntry`, `removeClaudeHookEntry`, `claudeEntriesContainCommand`)
**Apply to:** ALL new settings.json-style harness installers (augment, codebuddy, qwen, copilot, codex fix, antigravity)

The critical pattern (lines 84-107):
```go
// mergeClaudeHookEntry — MERGE, never overwrite. Idempotent.
func mergeClaudeHookEntry(existing any, cmd string, entry map[string]any) []any {
    arr, _ := existing.([]any)
    if claudeEntriesContainCommand(arr, cmd) {
        return arr
    }
    return append(arr, entry)
}

// removeClaudeHookEntry — remove ONLY beekeeper entries, preserve others.
func removeClaudeHookEntry(existing any, cmd string) ([]any, int) { ... }
```

For harnesses using typed structs (cursor.go, codex.go) the equivalent is the `containsXxxHookByCommand` + filtered-append idiom at lines 79, 127-135 (cursor.go).

### Atomic file write + backup
**Source:** `internal/hooks/hooks.go` (`backupSettings` lines 138-153, `writeFileAtomic` lines 162-178)
**Apply to:** ALL file-writing harness installers

```go
if err := backupSettings(settingsPath); err != nil {
    return err
}
// ... compute merged content ...
return writeFileAtomic(settingsPath, data)
```

### editorinit.PatchSettings for settings.json
**Source:** `internal/hooks/claude_code.go` lines 129-149 (`editorinit.ReadSettings` → merge → `editorinit.PatchSettings`)
**Apply to:** Augment, CodeBuddy, Qwen, Copilot (all use `~/.*/settings.json` with same JSON shape)

```go
settings, err := editorinit.ReadSettings(settingsPath) // JSONC-safe, returns {} on ErrNotExist
// ... merge hooks ...
return editorinit.PatchSettings(settingsPath, "hooks", hooks) // atomic, MkdirAll, only sets "hooks" key
```

### Gateway guide pattern
**Source:** `internal/hooks/gateway_targets.go` (`printGatewayGuide` dispatch + per-target `fmt.Fprintf` blocks, lines 13-125)
**Apply to:** Kilo, Trae (no pre-exec hook → MCP gateway); OpenCode print fallback

### Cobra flag pattern
**Source:** `cmd/beekeeper/main.go` `newCheckCmd()` lines 245-308
**Apply to:** The `--hook` flag addition. Pattern: declare `var hookTarget string`, add `cmd.Flags().StringVar(...)`, branch in RunE without touching the default path.

### Table test structure
**Source:** `internal/check/paths_test.go` lines 272-299
**Apply to:** `internal/check/deny_render_test.go` (one row per harness × deny-family)

---

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `docs/harness-support-matrix.md` | support matrix | — | No existing markdown docs with harness tiers; content derived from RESEARCH.md §3 tiers |

---

## Metadata

**Analog search scope:** `internal/hooks/`, `internal/check/`, `cmd/beekeeper/`
**Files read:** 9 source files + CONTEXT.md + RESEARCH.md
**Pattern extraction date:** 2026-06-05

---

## PATTERN MAPPING COMPLETE

**Phase:** 10 - hook-block-protocol-compliance-and-multi-harness-enforcement
**Files classified:** 20
**Analogs found:** 19 / 20

### Coverage
- Files with exact analog: 9 (claude_code.go family + hooks_test.go + hooks.go + main.go + deny_render_test.go)
- Files with role-match analog: 10 (all new non-Claude-family installers mapped to cursor.go or codex.go)
- Files with no analog: 1 (docs/harness-support-matrix.md)

### Key Patterns Identified
- All settings.json-style harness installers copy the `mergeClaudeHookEntry` / `removeClaudeHookEntry` / `editorinit.PatchSettings` trinity from `internal/hooks/claude_code.go`
- Typed-struct harnesses (Cursor, Windsurf, Gemini) copy the `containsXxxHookByCommand` + filtered-append pattern from `internal/hooks/cursor.go` and `codex.go`
- The `--hook` flag in `cmd/beekeeper/main.go` follows the exact `--tool` flag precedent (lines 284-308): optional flag, branch in RunE, default path unchanged
- `RenderDeny` in `internal/check/deny_render.go` is a pure, table-driven function with no I/O — mirrors the `exitCodeFor` pure-function style already in handler.go
- Table tests in `deny_render_test.go` copy the `cases := []struct{ name, ... }` pattern from `internal/check/paths_test.go` lines 272-299

### File Created
`.planning/phases/10-hook-block-protocol-compliance-and-multi-harness-enforcement/10-PATTERNS.md`

### Ready for Planning
Pattern mapping complete. Planner can now reference analog patterns in PLAN.md files.
