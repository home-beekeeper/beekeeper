# Phase 4: Integration Surfaces - Research

**Researched:** 2026-05-26
**Domain:** Agent hook formats, MCP July 2026 protocol, Go HTTP reverse proxy, JSON-RPC fuzz testing, PATH shim patterns, multi-agent context propagation
**Confidence:** HIGH (hook schemas verified from official docs; MCP spec verified from modelcontextprotocol.io blog; Go stdlib patterns verified from pkg.go.dev)

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**INTG-01 + INTG-02: Hook Installer (`beekeeper hooks install`)**
- New subcommand group `beekeeper hooks install --target <agent>` in `cmd/beekeeper/main.go`; business logic in `internal/hooks/`.
- Supported targets: `claude-code`, `cursor`, `codex`, `continue`, `opencode`, `openclaw`.
- **Claude Code** (`--target claude-code`): reads `~/.claude/settings.json` (JSONC), merges in PreToolUse and PostToolUse hook entries using the Phase 3 `editorinit.PatchSettings` pattern.
- **Cursor** (`--target cursor`): writes to `~/.cursor/hooks.json` (different schema from Claude Code).
- **Codex CLI** (`--target codex`): writes to `~/.codex/hooks.json`.
- Gateway-based targets (continue, opencode, openclaw): prints configuration instructions, no file write.
- Idempotent: re-running does not duplicate hooks.
- Backs up settings file before modifying.
- `--dry-run` flag: prints what would be written without modifying files.
- `beekeeper hooks uninstall --target <target>` to remove installed hooks.

**INTG-07: Multi-Agent Context Propagation**
- New `AgentContext` struct in `internal/policy/types.go` (pure, no I/O).
- Reads `BEEKEEPER_AGENT_ID`, `BEEKEEPER_PARENT_AGENT_ID`, `BEEKEEPER_AGENT_DEPTH` from environment.
- `internal/policy` `Evaluate` function receives `AgentContext`.
- `audit.AuditRecord` extended with `agent_id`, `parent_agent_id`, `agent_depth`, `agent_lineage` fields.
- Subagent calls cannot exceed the permission level of the parent.

**INTG-03 + INTG-04: MCP Gateway Daemon (`beekeeper gateway`)**
- Stateless per-request proxy per MCP July 2026 spec (no session state).
- JSON-RPC 2.0 over HTTP POST only (not SSE/WebSocket in this phase).
- Correlation by `id` field always; never by position.
- Default bind: `127.0.0.1:<port>`; exposing to `0.0.0.0` requires `--bind` flag AND `"allow_remote_gateway": true` in config.
- Per-session token: `crypto/rand` at gateway startup; stored in `~/.beekeeper/state.json` as `gateway_token`.
- Upstream MCP server URL required (`--upstream` flag or config).
- Policy: every `tools/call` method routes through `check.RunCheck` equivalent; fail-closed always.
- JSON-RPC error codes: -32001 (block), -32002 (internal error), -32700 (parse error).
- Package location: `internal/gateway/` (gateway.go, proxy.go, parser.go, policy.go).

**INTG-05: Continue / OpenCode / OpenClaw via Gateway**
- No new code for clients; `beekeeper hooks install --target continue|opencode|openclaw` prints guide.
- `beekeeper gateway token` prints current gateway token from state.json.
- `beekeeper gateway status` prints running status, bound address, masked token.

**INTG-06: Shim Layer (`beekeeper shim install/uninstall`)**
- Managed shim directory: `~/.beekeeper/shims/` (or `%APPDATA%\beekeeper\shims\` on Windows).
- Unix: shell scripts; Windows: `.cmd` batch files.
- Tools shimmed: npm, pnpm, pip, cargo, go, gem, composer, npx, pipx.
- Detects real binary via `exec.LookPath` excluding shim directory.
- `beekeeper shim status`: lists which tools are shimmed.

### Claude's Discretion
- Port selection: fixed default (e.g. 7837) with `--port` override and fallback-to-random if busy.
- State.json schema for gateway: separate gateway state struct vs extending `catalog.State`.
- Shim construction on Windows: `.cmd` batch files preferred over `.ps1` for broadest compatibility.
- Cursor hooks settings file path: `~/.cursor/hooks.json` (confirmed from Cursor docs).
- Codex CLI hooks path: `~/.codex/hooks.json` (confirmed from Codex docs).
- `beekeeper audit-record` PostToolUse command: implement minimal command reading PostToolUse stdin, writing `tool_result` audit record, exit 0 always.

### Deferred Ideas (OUT OF SCOPE)
- ContextForge / MCPGuard policy-plugin mode (v1.5 deliverable).
- macOS EndpointSecurity entitlement path (v2 deliverable).
- TypeScript/Bun hook scaffold libraries via npm (v1.5 deliverable).
- Gateway over SSE/WebSocket (future spec).
- Remote gateway exposure (bind 0.0.0.0) — explicit opt-in only.
- `beekeeper protect install` and Sentry daemon (Phase 5).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| INTG-01 | `beekeeper hooks install --target claude-code` — writes PreToolUse/PostToolUse hooks to `~/.claude/settings.json` | Claude Code settings.json hooks schema verified; uses existing `editorinit.PatchSettings` for the `hooks` key |
| INTG-02 | `beekeeper hooks install --target cursor` and `--target codex` | Cursor hooks.json schema and path verified; Codex hooks.json schema and path verified; both differ from Claude Code |
| INTG-03 | MCP gateway daemon — stateless per-request HTTP proxy, localhost-only, per-session token auth | MCP July 2026 spec verified; `httputil.ReverseProxy` pattern documented; token generation pattern established |
| INTG-04 | MCP gateway applies policy engine inline to every proxied tool call; correlation by `id` field | `check.RunCheck` reuse pattern documented; JSON-RPC 2.0 batch handling approach defined |
| INTG-05 | Continue, OpenCode, OpenClaw integrations via gateway mode | Config formats verified for all three; gateway token printing pattern defined |
| INTG-06 | Shim layer — wrapper scripts for npm/pnpm/pip/cargo/go/gem/composer/npx/pipx | Unix shell script pattern documented; Windows .cmd pattern documented; `exec.LookPath` exclusion pattern established |
| INTG-07 | Multi-agent observability — parent-child context propagation in policy decisions and audit records | `AgentContext` struct design from CONTEXT.md; env var reading pattern established; audit record extension documented |
</phase_requirements>

---

## Summary

Phase 4 delivers five integration surfaces on top of the Phase 1-3 infrastructure. The core challenge is that each surface has a different protocol and schema: Claude Code settings.json (JSONC, hooks merged into existing JSON), Cursor hooks.json (different schema with `permission`/`deny` output, different file path), Codex hooks.json (similar to Cursor but `~/.codex/`), the MCP gateway (HTTP proxy + JSON-RPC 2.0 policy interception), the shim layer (OS-specific wrapper scripts), and multi-agent context (pure type and env-var extensions).

The most significant new technical surface is the MCP gateway (`internal/gateway/`). The MCP July 2026 spec removes the `initialize`/`initialized` handshake entirely — the gateway is a pure per-request HTTP proxy with no session accumulation. JSON-RPC request/response correlation is always by `id` field per JSON-RPC 2.0 spec; the gateway must never assume ordering. The `httputil.ReverseProxy` `Rewrite` + `ModifyResponse` hook pattern is the correct Go stdlib approach; the body must be buffered in `Rewrite` to run policy evaluation before forwarding.

The MCP message parser fuzz test (`internal/gateway/parser.go` + `FuzzParseMessage`) is a v0.6.0 **release gate**: it must be present and passing before release, covering panic-safety and invalid-Level invariants. The existing `internal/policy/fuzz_test.go` (behind `-tags fuzz`) is the pattern to follow.

For the hook installer, the critical finding is that **Cursor and Codex do not use the Claude Code settings.json format** — they each have their own `hooks.json` files with a distinct schema. The `internal/hooks/` package must implement three separate writers: one for Claude Code's JSONC settings.json (reusing `editorinit.PatchSettings`), one for Cursor's hooks.json, and one for Codex's hooks.json.

**Primary recommendation:** Implement `internal/gateway/` as a thin HTTP server with a manual JSON-RPC body-read approach rather than `httputil.ReverseProxy`, because the gateway must read and evaluate the full request body before deciding whether to forward — which requires buffering the body, which `ReverseProxy.Rewrite` supports but at the cost of complexity. Use `httputil.ReverseProxy` with a custom `Rewrite` that buffers + evaluates + conditionally aborts forwarding via a sentinel error in `ErrorHandler`.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Claude Code hook install (INTG-01) | `internal/hooks/` (I/O adapter) | Calls `editorinit.PatchSettings` | File write operation; not in policy engine |
| Cursor hook install (INTG-02) | `internal/hooks/` (I/O adapter) | Direct JSON write to `~/.cursor/hooks.json` | Different schema from Claude Code; own writer |
| Codex hook install (INTG-02) | `internal/hooks/` (I/O adapter) | Direct JSON write to `~/.codex/hooks.json` | Similar to Cursor; own writer |
| MCP gateway HTTP server | `internal/gateway/gateway.go` | Calls `internal/gateway/proxy.go` | HTTP server + token auth; own package |
| MCP JSON-RPC parsing + policy | `internal/gateway/proxy.go` | Calls `internal/policy.Evaluate` | Request body parsing is pre-policy step |
| Bounded JSON-RPC parser | `internal/gateway/parser.go` | Fuzz-tested standalone | Isolated for fuzz test targeting |
| Upstream HTTP forwarding | `internal/gateway/proxy.go` | `net/http` client | Transparent proxy after policy allow |
| Shim script creation | `internal/shim/` (I/O adapter) | `exec.LookPath` for real binary | OS-level file write; not in policy |
| Multi-agent context types | `internal/policy/types.go` (extend, pure) | Read by `check.RunCheck` callers | Must stay pure; same package as ToolCall |
| Audit record lineage fields | `internal/audit/types.go` (extend) | Called by all decision surfaces | Existing package; extend AuditRecord |
| Gateway state (token + port) | `internal/gateway/state.go` | Follows `catalog.state.go` pattern | Separate state struct; not in catalog.State |
| `beekeeper audit-record` command | `cmd/beekeeper/main.go` | Reads stdin, writes audit record | Thin Cobra wiring; exit 0 always |

---

## Standard Stack

### Core (all stdlib — no new deps required for gateway or shim)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `net/http` | stdlib | MCP gateway HTTP server + upstream client | Already established; no external dep |
| `net/http/httputil` | stdlib | `ReverseProxy` for transparent upstream forwarding | Standard Go reverse proxy; handles hop-by-hop headers |
| `encoding/json` | stdlib | JSON-RPC message parsing, hook file writing | Consistent with all prior phase choices |
| `crypto/rand` | stdlib | Per-session gateway token generation (32 bytes → hex64) | Cryptographically secure; no external dep |
| `encoding/hex` | stdlib | Hex-encode the token (existing pattern in `check.newRecordID`) | Consistent with existing token encoding |
| `os/signal` | stdlib | `signal.NotifyContext(SIGINT, SIGTERM)` for gateway daemon | Same pattern as `catalogs watch` in `main.go` |
| `os/exec` | stdlib | `exec.LookPath` for shim real-binary detection | Same as Phase 3 Bumblebee lookup |
| `github.com/tidwall/jsonc` | v0.3.3 | JSONC parsing for Claude Code settings.json hook inject | Already in go.mod; `editorinit.PatchSettings` uses it |
| `github.com/spf13/cobra` | v1.10.2 | `hooks`, `gateway`, `shim` subcommand trees | Already in go.mod |

[VERIFIED: all listed packages are already in go.mod — no new dependencies required for Phase 4]

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `io` | stdlib | `io.LimitReader` for 1MB gateway body cap | Same cap as `check.RunCheck` stdin |
| `sync` | stdlib | `sync.Mutex` for gateway state file writes | Token rotation on restart |
| `path/filepath` | stdlib | Cross-platform shim directory paths | Same as all prior phases |
| `runtime` | stdlib | `runtime.GOOS` for Unix vs Windows shim selection | Same as Phase 3 watcher |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Manual gateway HTTP server | `httputil.ReverseProxy` alone | ReverseProxy cannot abort forwarding mid-`Rewrite` without a sentinel error trick; manual server is cleaner for policy-gated forwarding |
| `httputil.ReverseProxy.Rewrite` body buffering | `httputil.ReverseProxy.Director` | `Director` is deprecated in Go 1.25+; `Rewrite` is the supported API |
| 32-byte hex token (64 chars) | JWT or structured token | 32-byte crypto/rand hex is simpler, matches existing `newRecordID` pattern, and has no parsing overhead |
| `.cmd` batch shims on Windows | `.ps1` PowerShell shims | `.cmd` runs without execution policy restrictions; broadest compatibility |

**Installation (new deps): none required** — all Phase 4 packages use existing go.mod dependencies.

---

## Architecture Patterns

### System Architecture Diagram

```
                  PHASE 4: INTEGRATION SURFACES — DATA FLOW
                  ─────────────────────────────────────────

SURFACE 1: Hook Installer (beekeeper hooks install --target <agent>)
─────────────────────────────────────────────────────────────────────
  beekeeper hooks install --target claude-code
        │
        └→ internal/hooks/claude_code.go
                └→ editorinit.PatchSettings("~/.claude/settings.json", "hooks", hookMap)
                        └→ reads JSONC → strips comments → merges hooks key → atomic write

  beekeeper hooks install --target cursor
        │
        └→ internal/hooks/cursor.go
                └→ reads ~/.cursor/hooks.json → merges preToolUse entry → writes JSON
                        (schema: {version:1, hooks:{preToolUse:[{command,type,matcher}]}})

  beekeeper hooks install --target codex
        │
        └→ internal/hooks/codex.go
                └→ reads ~/.codex/hooks.json → merges PreToolUse entry → writes JSON
                        (schema: {hooks:{PreToolUse:[{matcher,hooks:[{type,command}]}]}})

  beekeeper hooks install --target continue|opencode|openclaw
        └→ internal/hooks/gateway_targets.go
                └→ prints formatted guide with token + port from state.json


SURFACE 2: MCP Gateway Daemon (beekeeper gateway)
──────────────────────────────────────────────────
  beekeeper gateway [--port 7837] [--upstream URL]
        │
        ├→ internal/gateway/gateway.go
        │       ├→ crypto/rand → 32-byte token → hex64 → store in state.json
        │       ├→ net.Listen("tcp", "127.0.0.1:<port>")
        │       └→ http.Server{Handler: gatewayHandler}
        │               └→ middleware: verify Authorization: Bearer <token>
        │
        └→ internal/gateway/proxy.go  (per-request handler)
                │
                ├→ io.LimitReader(r.Body, 1MB) → io.ReadAll → parseMessage()
                │
                ├→ method == "tools/call" ?
                │       YES → policy.Evaluate(tc, idx, thresholds)
                │               ├→ ALLOW: forward to upstream, return upstream response
                │               ├→ WARN: forward to upstream, inject _beekeeper_warning field
                │               └→ BLOCK: return JSON-RPC error {code:-32001, data:{decision,reason,rule_ids}}
                │
                └→ other methods → httputil.ReverseProxy transparent forward


SURFACE 3: Shim Layer (beekeeper shim install)
──────────────────────────────────────────────
  beekeeper shim install
        │
        └→ internal/shim/shim.go
                ├→ exec.LookPath("npm") → real path (excluding ~/.beekeeper/shims/)
                ├→ runtime.GOOS == "windows" ?
                │       YES → write ~/.beekeeper/shims/npm.cmd  (batch file)
                │       NO  → write ~/.beekeeper/shims/npm      (shell script, chmod +x)
                └→ print PATH instructions per detected shell


SURFACE 4: Multi-Agent Context (INTG-07)
─────────────────────────────────────────
  beekeeper check (reads env: BEEKEEPER_AGENT_ID, BEEKEEPER_PARENT_AGENT_ID, BEEKEEPER_AGENT_DEPTH)
        │
        ├→ build AgentContext{AgentID, ParentAgentID, Depth, Lineage}
        │
        ├→ policy.Evaluate(tc, idx, thresholds, agentCtx)  [extended signature]
        │       └→ lineage check: child cannot exceed parent policy level
        │
        └→ finalize() → audit.FromDecision() → AuditRecord{agent_id, parent_agent_id, ...}


SELF-DEFENSE: Fuzz Testing
───────────────────────────
  go test -tags fuzz -fuzz=FuzzParseMessage ./internal/gateway/...
        └→ internal/gateway/parser_fuzz_test.go
                └→ FuzzParseMessage(f *testing.F)
                        ├→ f.Add: valid JSON-RPC, malformed, oversized, batch
                        └→ parseMessage() must: never panic, always return typed error, respect 1MB cap
```

### Recommended Project Structure (Phase 4 additions)

```
internal/
  hooks/
    hooks.go              # HooksInstaller: Install(target, dryRun), Uninstall(target, dryRun)
    claude_code.go        # installClaudeCode(): PatchSettings-based JSONC hook inject
    cursor.go             # installCursor(): hooks.json merge
    codex.go              # installCodex(): hooks.json merge
    gateway_targets.go    # printGatewayGuide(target, token, port): Continue/OpenCode/OpenClaw
    hooks_test.go         # table-driven: idempotent, dry-run, backup, each target
    testdata/
      claude_settings.json   # fixture: existing settings with comments
      cursor_hooks.json      # fixture: existing cursor hooks
      codex_hooks.json       # fixture: existing codex hooks
  gateway/
    gateway.go            # Server struct, Start(), token gen+store, HTTP server setup
    proxy.go              # ServeHTTP: token verify, body read, method routing, forward
    parser.go             # ParseMessage([]byte) (JSONRPCMessage, error) — bounded, fuzz target
    policy.go             # applyPolicy(msg, idx, cfg) Decision — delegates to policy.Evaluate
    state.go              # GatewayState: GatewayToken, BoundPort, BoundAddr; LoadState/SaveState
    gateway_test.go       # httptest.Server; token auth; fail-closed paths
    proxy_test.go         # tools/call block; non-tool-call passthrough; warn injection
    parser_test.go        # table-driven: valid, malformed, oversized, batch, deep nesting
    parser_fuzz_test.go   # FuzzParseMessage — RELEASE GATE (go:build fuzz)
  shim/
    shim.go               # Install(shimDir, tools), Uninstall(shimDir), Status(shimDir)
    shim_unix.go          # writeShellScript() — build-tagged //go:build !windows
    shim_windows.go       # writeBatchFile() — build-tagged //go:build windows
    shim_test.go          # tempdir-based; verify script content, chmod, idempotency
cmd/beekeeper/
  main.go                 # EXTENDED: add hooks, gateway, shim subcommand trees; audit-record command
internal/policy/
  types.go                # EXTENDED: add AgentContext struct
internal/audit/
  types.go                # EXTENDED: add agent_id, parent_agent_id, agent_depth, agent_lineage fields
```

### Pattern 1: Claude Code `settings.json` Hook Injection

**What:** Use `editorinit.PatchSettings` to merge the `hooks` key into Claude Code's JSONC settings.json. The hook format was verified from the official Claude Code hooks documentation.

**Verified schema (Claude Code settings.json):**

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": ".*",
        "hooks": [
          {
            "type": "command",
            "command": "beekeeper check"
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": ".*",
        "hooks": [
          {
            "type": "command",
            "command": "beekeeper audit-record"
          }
        ]
      }
    ]
  }
}
```

[VERIFIED: code.claude.com/docs/en/hooks — `matcher` is a regex (empty/`".*"` matches all tools), `type` is `"command"`, `command` is the shell command. Exit code 0 = allow, exit code 2 = block (for PreToolUse). PostToolUse cannot block (tool already ran).]

**Key fields:**
- `matcher`: regex applied to `tool_name`. `".*"` matches all tools.
- `type`: must be `"command"` (other types: `"http"`, `"mcp_tool"`, `"prompt"`, `"agent"` — not needed here)
- `command`: the shell command; receives full tool call JSON on stdin

**Input stdin JSON to `beekeeper check`:**
```json
{
  "session_id": "abc123",
  "hook_event_name": "PreToolUse",
  "tool_name": "Bash",
  "tool_input": {"command": "npm install express"},
  "tool_use_id": "unique-id",
  "agent_id": "agent-123",
  "cwd": "/project",
  "transcript_path": "/path/to/transcript.jsonl"
}
```

[VERIFIED: code.claude.com/docs/en/hooks — confirmed `agent_id` field is present in hook stdin, enabling INTG-07 context propagation without additional config]

**Output stdout JSON (structured blocking):**
```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "Blocked by Beekeeper: corroborated catalog match"
  }
}
```

**Implementation: extend `editorinit.PatchSettings`**

```go
// Source: [VERIFIED: internal/editorinit/settings.go — existing PatchSettings function]
// internal/hooks/claude_code.go

func installClaudeCode(settingsPath string, dryRun bool) error {
    hookConfig := map[string]any{
        "PreToolUse": []any{
            map[string]any{
                "matcher": ".*",
                "hooks": []any{
                    map[string]any{
                        "type":    "command",
                        "command": "beekeeper check",
                    },
                },
            },
        },
        "PostToolUse": []any{
            map[string]any{
                "matcher": ".*",
                "hooks": []any{
                    map[string]any{
                        "type":    "command",
                        "command": "beekeeper audit-record",
                    },
                },
            },
        },
    }
    if dryRun {
        // print what would be written; do not touch the file
        return printDryRun(settingsPath, "hooks", hookConfig)
    }
    // Backup first
    if err := backupSettings(settingsPath); err != nil {
        return err
    }
    // PatchSettings uses editorinit.PatchSettings which is JSONC-safe (strips comments on read, atomic write)
    return editorinit.PatchSettings(settingsPath, "hooks", hookConfig)
}
```

**Idempotency:** `PatchSettings` unmarshals to `map[string]any`, overwrites the `hooks` key, and re-marshals. Re-running replaces the hooks key with the same value — no duplication possible.

### Pattern 2: Cursor `hooks.json` Schema

**What:** Cursor uses `~/.cursor/hooks.json` (not `settings.json`). The schema is different from Claude Code.

[VERIFIED: cursor.com/docs/hooks — `hooks.json` file at `~/.cursor/hooks.json` (user-level), `{version:1, hooks:{preToolUse:[...]}}`. Different field names from Claude Code.]

**Cursor hooks.json format:**

```json
{
  "version": 1,
  "hooks": {
    "preToolUse": [
      {
        "command": "beekeeper check",
        "type": "command",
        "timeout": 10,
        "matcher": ".*",
        "failClosed": true
      }
    ]
  }
}
```

**Key differences from Claude Code:**
- File: `~/.cursor/hooks.json` (not `~/.cursor/settings.json`)
- Key: `preToolUse` (camelCase, not `PreToolUse`)
- Schema: flat array of handler objects (no inner `hooks` array)
- Extra fields: `failClosed` (boolean — block on hook failure, critical for security), `loop_limit`
- Output: `{"permission": "allow|deny", "user_message": "...", "agent_message": "..."}`
- Exit code 2 blocks (same as Claude Code)

**stdin input to `beekeeper check` from Cursor:**
```json
{
  "tool_name": "Shell",
  "tool_input": {"command": "npm install express"},
  "tool_use_id": "abc123",
  "cwd": "/project",
  "conversation_id": "...",
  "hook_event_name": "preToolUse"
}
```

[VERIFIED: cursor.com/docs/hooks — Cursor uses `"Shell"` as tool name for shell execution (not `"Bash"`). The `agent_id` field is NOT present in Cursor hook stdin — INTG-07 lineage from Cursor must come from environment variables only.]

**File paths by platform:**
- User-level: `~/.cursor/hooks.json` (macOS/Linux: `$HOME/.cursor/hooks.json`; Windows: `%USERPROFILE%\.cursor\hooks.json`)
- Project-level: `<project>/.cursor/hooks.json` (installer targets user-level)
- Enterprise: `/Library/Application Support/Cursor/hooks.json` (macOS), `C:\ProgramData\Cursor\hooks.json` (Windows) — installer does not touch enterprise level

[VERIFIED: cursor.com/docs/hooks — confirmed paths above]

```go
// Source: [VERIFIED: cursor.com/docs/hooks schema]
// internal/hooks/cursor.go

type cursorHooksFile struct {
    Version int                    `json:"version"`
    Hooks   map[string][]cursorHook `json:"hooks"`
}

type cursorHook struct {
    Command    string `json:"command"`
    Type       string `json:"type"`
    Timeout    int    `json:"timeout,omitempty"`
    Matcher    string `json:"matcher,omitempty"`
    FailClosed bool   `json:"failClosed,omitempty"`
}

func installCursor(hooksPath string, dryRun bool) error {
    existing := cursorHooksFile{Version: 1, Hooks: make(map[string][]cursorHook)}

    if data, err := os.ReadFile(hooksPath); err == nil {
        _ = json.Unmarshal(data, &existing) // tolerate parse errors, start fresh
    }

    newHook := cursorHook{
        Command:    "beekeeper check",
        Type:       "command",
        Timeout:    10,
        Matcher:    ".*",
        FailClosed: true, // fail-closed: block on hook failure
    }

    // Idempotent: only add if not already present (match on Command field)
    if !containsCursorHook(existing.Hooks["preToolUse"], newHook.Command) {
        existing.Hooks["preToolUse"] = append(existing.Hooks["preToolUse"], newHook)
    }

    if dryRun {
        return printDryRun(hooksPath, "cursor-hooks", existing)
    }
    if err := backupSettings(hooksPath); err != nil {
        return err
    }
    out, _ := json.MarshalIndent(existing, "", "    ")
    return writeFileAtomic(hooksPath, out)
}
```

### Pattern 3: Codex CLI `hooks.json` Schema

**What:** Codex CLI uses `~/.codex/hooks.json` (user-level) or `<repo>/.codex/hooks.json` (project-level). The schema closely resembles Claude Code's nested format.

[VERIFIED: developers.openai.com/codex/hooks — `~/.codex/hooks.json` confirmed; events include `PreToolUse`, `PostToolUse`, `SessionStart`, `Stop`]

**Codex hooks.json format:**

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "^Bash$",
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/beekeeper check",
            "statusMessage": "Beekeeper policy check",
            "timeout": 10
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "^Bash$",
        "hooks": [
          {
            "type": "command",
            "command": "beekeeper audit-record"
          }
        ]
      }
    ]
  }
}
```

**Key similarities to Claude Code:** Nested `hooks` array structure, `matcher` regex, `type: "command"`. [VERIFIED: developers.openai.com/codex/hooks]

**Key differences from Claude Code:**
- File: `~/.codex/hooks.json` (separate file, not merged into a settings.json)
- Windows override: `commandWindows` field (string) for Windows-specific command path
- Trust model: Codex requires the user to explicitly trust each new hook definition; new hooks are skipped until trusted. The installer must inform the user they need to run Codex once to trust the hook.

**stdin input to `beekeeper check` from Codex:**
```json
{
  "session_id": "...",
  "turn_id": "...",
  "cwd": "/project",
  "hook_event_name": "PreToolUse",
  "tool_name": "Bash",
  "tool_input": {"command": "npm install express"}
}
```

[VERIFIED: developers.openai.com/codex/hooks — confirmed field names; `agent_id` is NOT present in Codex hook stdin]

### Pattern 4: MCP Gateway — Stateless Per-Request HTTP Proxy

**What:** The MCP July 2026 spec removes the `initialize`/`initialized` handshake. Every request is self-contained. The gateway is a stateless HTTP server that intercepts `tools/call` methods and applies policy inline.

[VERIFIED: blog.modelcontextprotocol.io/posts/2026-07-28-release-candidate/ — `initialize`/`initialized` handshake eliminated; `_meta` field on every request replaces handshake capabilities; `Mcp-Session-Id` header removed]

**MCP 2026-07-28 HTTP request format:**
```
POST /mcp HTTP/1.1
MCP-Protocol-Version: 2026-07-28
Mcp-Method: tools/call
Mcp-Name: search
Content-Type: application/json
Authorization: Bearer <gateway-token>

{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search","arguments":{...},"_meta":{...}}}
```

**What the gateway intercepts:** Any request whose JSON-RPC `method` field is `"tools/call"` (or `"tools/call"` — verify exact spelling against upstream). All other methods are passed through transparently.

**JSON-RPC 2.0 message types:**
```go
// Source: [VERIFIED: JSON-RPC 2.0 spec — https://www.jsonrpc.org/specification]
// internal/gateway/parser.go

type JSONRPCMessage struct {
    JSONRPC string          `json:"jsonrpc"` // always "2.0"
    ID      any             `json:"id"`      // string | number | null (for notifications: absent)
    Method  string          `json:"method,omitempty"`
    Params  json.RawMessage `json:"params,omitempty"`
    Result  json.RawMessage `json:"result,omitempty"`
    Error   *JSONRPCError   `json:"error,omitempty"`
}

type JSONRPCError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Data    any    `json:"data,omitempty"`
}
```

**Error codes for gateway-specific blocks:**
| Code | Meaning | When Used |
|------|---------|-----------|
| -32700 | Parse error | Malformed JSON or message exceeds 1MB |
| -32001 | Beekeeper block | Policy decision = block |
| -32002 | Beekeeper internal error | Policy engine panic, timeout, or index unavailable |
| -32600 | Invalid request | JSON-RPC shape invalid (no method field, wrong jsonrpc version) |

[VERIFIED: modelcontextprotocol.io spec — `-32002` was changed to `-32602` for missing resources in 2026-07-28 spec, but -32002 is still defined for server errors; Beekeeper's use of -32001 and -32002 as custom application codes is valid per JSON-RPC 2.0 which reserves -32000 to -32099 for implementation-defined server errors]

**Gateway daemon lifecycle (signal.NotifyContext pattern):**

```go
// Source: [VERIFIED: cmd/beekeeper/main.go — catalogs watch daemon is the canonical pattern]
// internal/gateway/gateway.go

func Start(ctx context.Context, cfg Config) error {
    // Generate per-session token
    var raw [32]byte
    if _, err := rand.Read(raw[:]); err != nil {
        return fmt.Errorf("generate token: %w", err)
    }
    token := hex.EncodeToString(raw[:])

    // Store token + port in state.json
    st := GatewayState{
        GatewayToken: token,
        BoundAddr:    cfg.BindAddr,
        BoundPort:    cfg.Port,
        StartedAt:    time.Now().UTC().Format(time.RFC3339),
    }
    if err := SaveGatewayState(cfg.StateFile, st); err != nil {
        return fmt.Errorf("save gateway state: %w", err)
    }

    listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.Port))
    if err != nil {
        // If port busy and fallback-to-random: net.Listen("tcp", "127.0.0.1:0")
        return fmt.Errorf("bind gateway: %w", err)
    }
    defer listener.Close()

    srv := &http.Server{
        Handler:      newGatewayHandler(cfg, token),
        ReadTimeout:  30 * time.Second,
        WriteTimeout: 35 * time.Second,
        IdleTimeout:  60 * time.Second,
    }

    go func() {
        <-ctx.Done()
        shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        _ = srv.Shutdown(shutCtx)
    }()

    return srv.Serve(listener)
}
```

### Pattern 5: Gateway Body-Read Policy Gate (Manual Proxy)

**What:** For `tools/call` methods, the gateway must read the full body, evaluate policy, then either forward or return an error. `httputil.ReverseProxy` is used for transparent passthrough of non-tool-call methods; tool-call methods are handled manually.

**Critical design decision:** Do NOT use `httputil.ReverseProxy` as the primary handler for tool-call methods. The `Rewrite` function receives the `*ProxyRequest`, not `http.ResponseWriter`, so you cannot write a JSON-RPC error response to the client from inside `Rewrite`. Use manual handling for the policy path and `ReverseProxy` for passthrough.

```go
// Source: [VERIFIED: pkg.go.dev/net/http/httputil — ReverseProxy.Rewrite signature; ModifyResponse]
// internal/gateway/proxy.go

func (h *gatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 1. Token auth: verify Authorization: Bearer <token>
    if !h.verifyToken(r) {
        writeJSONRPCError(w, nil, -32600, "unauthorized", nil)
        return
    }

    // 2. Read bounded body (1MB cap — same as check.RunCheck stdin cap)
    limited := io.LimitReader(r.Body, maxRequestBody+1)
    bodyBytes, err := io.ReadAll(limited)
    if err != nil || int64(len(bodyBytes)) > maxRequestBody {
        writeJSONRPCError(w, nil, -32700, "request body exceeds 1MB cap", nil)
        return
    }

    // 3. Parse JSON-RPC message (bounded parser — fuzz target)
    msg, parseErr := parser.ParseMessage(bodyBytes)
    if parseErr != nil {
        writeJSONRPCError(w, nil, -32700, "parse error: "+parseErr.Error(), nil)
        return
    }

    // 4. Route: tools/call → policy gate; everything else → transparent proxy
    if msg.Method == "tools/call" {
        h.handleToolCall(w, r, msg, bodyBytes)
        return
    }

    // 5. Transparent forward via ReverseProxy (body already consumed — replace)
    r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
    h.reverseProxy.ServeHTTP(w, r)
}

func (h *gatewayHandler) handleToolCall(w http.ResponseWriter, r *http.Request, msg parser.JSONRPCMessage, bodyBytes []byte) {
    // Build ToolCall for policy engine (extract from params.arguments)
    tc := extractToolCall(msg)

    // Policy evaluation (synchronous, inline — same as check.RunCheck)
    decision := policy.Evaluate(tc, h.idx, policy.DefaultCorroborationThresholds())

    switch decision.Level {
    case "block":
        writeJSONRPCError(w, msg.ID, -32001, "blocked by Beekeeper", map[string]any{
            "decision": decision.Level,
            "reason":   decision.Reason,
            "rule_ids": decision.RuleIDs,
        })
        h.writeAudit(tc, decision, "gateway")
        return

    case "warn":
        // Forward to upstream; inject warning field into response
        resp := h.forwardAndInjectWarning(r, bodyBytes, decision)
        h.writeAudit(tc, decision, "gateway")
        writeJSONResponse(w, resp)
        return

    default: // "allow"
        // Forward transparently
        r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
        h.reverseProxy.ServeHTTP(w, r)
        h.writeAudit(tc, decision, "gateway")
    }
}
```

**Fail-closed invariant:** Any panic in the policy engine is recovered in a deferred func (same as `check.runCheck`'s top-level recover), writing a -32002 error to the client.

### Pattern 6: Bounded JSON-RPC Parser (`internal/gateway/parser.go`)

**What:** The isolated parser that gets fuzz-tested. It enforces all size/depth bounds and is the sole entry point for untrusted bytes.

```go
// Source: [VERIFIED: encoding/json stdlib; CONTEXT.md bounds spec]
// internal/gateway/parser.go

const (
    maxRequestBody    = 1 << 20  // 1MB
    maxMethodLen      = 256      // INTG-03 spec: reject methods > 256 bytes
    maxBatchItems     = 50       // INTG-03 spec: max batch size
    maxRecursionDepth = 10       // INTG-03 spec: tool parameter schema depth
)

// ParseMessage decodes a single JSON-RPC 2.0 request or batch from b.
// All bounds (maxRequestBody, maxMethodLen, maxBatchItems, maxRecursionDepth) are enforced.
// Returns a typed error for every invalid input — never panics.
func ParseMessage(b []byte) (JSONRPCMessage, error) {
    if len(b) == 0 {
        return JSONRPCMessage{}, &ParseError{Code: -32700, Msg: "empty body"}
    }

    // Detect batch (starts with '[') vs single (starts with '{')
    trimmed := bytes.TrimSpace(b)
    if len(trimmed) == 0 {
        return JSONRPCMessage{}, &ParseError{Code: -32700, Msg: "empty body after trim"}
    }

    if trimmed[0] == '[' {
        return parseAsBatch(trimmed)
    }
    return parseSingle(trimmed)
}

func parseSingle(b []byte) (JSONRPCMessage, error) {
    var msg JSONRPCMessage
    if err := json.Unmarshal(b, &msg); err != nil {
        return JSONRPCMessage{}, &ParseError{Code: -32700, Msg: "invalid JSON: " + err.Error()}
    }
    if msg.JSONRPC != "2.0" {
        return JSONRPCMessage{}, &ParseError{Code: -32600, Msg: "jsonrpc must be \"2.0\""}
    }
    if len(msg.Method) > maxMethodLen {
        return JSONRPCMessage{}, &ParseError{Code: -32600, Msg: "method name exceeds 256 bytes"}
    }
    if err := checkDepth(msg.Params, 0); err != nil {
        return JSONRPCMessage{}, &ParseError{Code: -32600, Msg: "params depth exceeds limit"}
    }
    return msg, nil
}

func parseAsBatch(b []byte) (JSONRPCMessage, error) {
    var batch []json.RawMessage
    if err := json.Unmarshal(b, &batch); err != nil {
        return JSONRPCMessage{}, &ParseError{Code: -32700, Msg: "invalid batch JSON"}
    }
    if len(batch) > maxBatchItems {
        return JSONRPCMessage{}, &ParseError{Code: -32600, Msg: "batch exceeds 50 items"}
    }
    // Return first item as the primary message; caller handles full batch iteration
    // For now: first-item approach; full batch fan-out is a gateway.go responsibility
    if len(batch) == 0 {
        return JSONRPCMessage{}, &ParseError{Code: -32600, Msg: "empty batch"}
    }
    return parseSingle(batch[0])
}

// checkDepth checks that a JSON value does not exceed maxRecursionDepth levels of nesting.
func checkDepth(raw json.RawMessage, depth int) error {
    if depth > maxRecursionDepth {
        return fmt.Errorf("depth limit exceeded")
    }
    if len(raw) == 0 {
        return nil
    }
    var v any
    if err := json.Unmarshal(raw, &v); err != nil {
        return nil // unmarshal errors are caught in parseSingle
    }
    return checkValueDepth(v, depth)
}
```

### Pattern 7: MCP Message Parser Fuzz Test (Release Gate)

**What:** The fuzz test for `internal/gateway/parser.go`. This is a **v0.6.0 release gate** — the test must exist and pass before release. Uses the same `-tags fuzz` build tag pattern as `internal/policy/fuzz_test.go`.

[VERIFIED: internal/policy/fuzz_test.go — existing pattern with `//go:build fuzz` tag and `f.Add` seed corpus]
[VERIFIED: go.dev/doc/security/fuzz/ — fuzz corpus directory at `testdata/fuzz/FuzzXxx/`, `f.Add` for inline seeds, `go test -tags fuzz -fuzz=FuzzParseMessage` to run]

```go
// Source: [VERIFIED: go.dev/doc/security/fuzz/ — exact API]
// internal/gateway/parser_fuzz_test.go

//go:build fuzz

package gateway

import (
    "testing"
)

// FuzzParseMessage is the RELEASE GATE fuzz test for the MCP message parser.
// Contract: ParseMessage must NEVER panic on any input and must ALWAYS return
// either a valid JSONRPCMessage or a typed *ParseError — no untyped panics,
// no out-of-bounds, no infinite loops.
//
// Run with: go test -tags fuzz -fuzz=FuzzParseMessage ./internal/gateway/...
// CI gate:  go test -tags fuzz -run=FuzzParseMessage ./internal/gateway/... (seed corpus only)
func FuzzParseMessage(f *testing.F) {
    // Seed corpus: representative and adversarial inputs
    f.Add([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"x","arguments":{}}}`))
    f.Add([]byte(`{"jsonrpc":"2.0","id":null,"method":"initialize","params":{}}`))
    f.Add([]byte(`[{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{}}]`))
    f.Add([]byte(`{}`))
    f.Add([]byte(`[]`))
    f.Add([]byte(``))
    f.Add([]byte(`null`))
    f.Add([]byte(`{"jsonrpc":"2.0","id":1,"method":"` + string(make([]byte, 300)) + `"}`)) // oversized method
    f.Add([]byte(`{"jsonrpc":"1.0","id":1,"method":"tools/call"}`))                          // wrong version
    // Deep nesting (depth=11, exceeds maxRecursionDepth=10)
    f.Add([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"a":{"b":{"c":{"d":{"e":{"f":{"g":{"h":{"i":{"j":{"k":"deep"}}}}}}}}}}}}`))
    // Batch with 51 items (exceeds maxBatchItems=50)
    batch := make([]byte, 0, 4096)
    batch = append(batch, '[')
    for i := 0; i < 51; i++ {
        if i > 0 {
            batch = append(batch, ',')
        }
        batch = append(batch, []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)...)
    }
    batch = append(batch, ']')
    f.Add(batch)

    f.Fuzz(func(t *testing.T, data []byte) {
        // ParseMessage must never panic regardless of input.
        // Any return (msg, nil) must have msg.JSONRPC == "2.0".
        // Any error return must be a *ParseError with a valid Code.
        msg, err := ParseMessage(data)
        if err != nil {
            pe, ok := err.(*ParseError)
            if !ok {
                t.Errorf("ParseMessage returned non-ParseError: %T: %v", err, err)
            }
            if pe.Code == 0 {
                t.Errorf("ParseError has zero Code for input %q", data)
            }
            return
        }
        // Success path: basic invariants
        if msg.JSONRPC != "2.0" {
            t.Errorf("ParseMessage returned non-2.0 jsonrpc: %q for input %q", msg.JSONRPC, data)
        }
    })
}
```

**Seed corpus directory (for CI without -fuzz flag):**
```
internal/gateway/testdata/fuzz/FuzzParseMessage/
    001_valid_tools_call
    002_null_id
    003_batch_single
    004_empty
    005_oversized_method
    006_wrong_version
    007_deep_nesting
    008_large_batch
```

Each corpus file format:
```
go test fuzz v1
[]byte("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{}}")
```

**CI integration:**
- `go test -tags fuzz -run=FuzzParseMessage ./internal/gateway/...` — runs seed corpus entries as regression tests (fast, no fuzzing engine)
- `go test -tags fuzz -fuzz=FuzzParseMessage -fuzztime=60s ./internal/gateway/...` — runs the fuzzing engine for 60 seconds (CI nightly or release gate check)

### Pattern 8: Per-Session Token Auth

**What:** Generate a 32-byte random token at gateway startup, store in state.json, require it on every request via `Authorization: Bearer <token>`.

```go
// Source: [VERIFIED: internal/check/handler.go — newRecordID() uses crypto/rand; same pattern]
// internal/gateway/gateway.go

func generateToken() (string, error) {
    var raw [32]byte // 256 bits of entropy
    if _, err := rand.Read(raw[:]); err != nil {
        return "", fmt.Errorf("crypto/rand failed: %w", err)
    }
    return hex.EncodeToString(raw[:]), nil // 64-char hex string
}

func verifyToken(r *http.Request, expected string) bool {
    const prefix = "Bearer "
    auth := r.Header.Get("Authorization")
    if len(auth) < len(prefix) {
        return false
    }
    candidate := auth[len(prefix):]
    // Constant-time comparison to prevent timing attacks
    return subtle.ConstantTimeCompare([]byte(candidate), []byte(expected)) == 1
}
```

**State.json extension (separate from `catalog.WatchState`):**

```go
// Source: [VERIFIED: internal/catalog/state.go — LoadState/SaveState pattern to follow]
// internal/gateway/state.go

// GatewayState is the persisted gateway daemon state, written atomically to
// ~/.beekeeper/state.json under the "gateway" key. It survives process restart
// and allows CLI commands (beekeeper gateway token, gateway status) to read
// the current token and port without connecting to the daemon.
type GatewayState struct {
    GatewayToken string `json:"gateway_token"` // 64-char hex; rotates on each gateway restart
    BoundAddr    string `json:"bound_addr"`    // e.g. "127.0.0.1"
    BoundPort    int    `json:"bound_port"`
    StartedAt    string `json:"started_at"`    // RFC3339
    PID          int    `json:"pid"`           // os.Getpid() for status check
}
```

**Design:** Store `GatewayState` as a separate top-level key `"gateway"` in `~/.beekeeper/state.json`, alongside the existing `"sources"` key from `catalog.WatchState`. This avoids a separate state file while keeping the schemas independent. Implement `LoadGatewayState` / `SaveGatewayState` following the exact `catalog.LoadState` / `catalog.SaveState` pattern (missing file = zero value, atomic write).

### Pattern 9: Shim Layer — Unix and Windows Scripts

**What:** Create wrapper scripts in `~/.beekeeper/shims/` that invoke `beekeeper check` before calling the real binary.

**Unix shell script (for npm, pip, etc.):**

```sh
#!/bin/sh
# beekeeper shim for npm — auto-generated, do not edit
# Real binary: /usr/local/bin/npm
beekeeper check <<EOF
{"tool_name":"Bash","tool_input":{"command":"npm $*"}}
EOF
_bk_exit=$?
if [ $_bk_exit -eq 0 ]; then
    exec "/usr/local/bin/npm" "$@"
fi
exit $_bk_exit
```

[VERIFIED: CONTEXT.md INTG-06 — exact shell script pattern; `exec` replaces the shell process to preserve exit code and signal handling]

**Windows .cmd batch file:**

```cmd
@echo off
echo {"tool_name":"Bash","tool_input":{"command":"npm %*"}} | beekeeper check
if %ERRORLEVEL% EQU 0 goto :run
exit /b %ERRORLEVEL%
:run
"C:\Program Files\nodejs\npm.cmd" %*
```

[VERIFIED: CONTEXT.md INTG-06 — .cmd batch file pattern; `%*` passes all args through]

**Real binary detection (excluding shim directory):**

```go
// Source: [VERIFIED: os/exec stdlib — LookPath searches PATH in order]
// internal/shim/shim.go

// findRealBinary finds the real binary for tool, excluding the Beekeeper shim directory.
// It temporarily removes shimDir from PATH, calls exec.LookPath, then restores PATH.
func findRealBinary(shimDir, tool string) (string, error) {
    origPath := os.Getenv("PATH")
    filteredPath := filterPathEntries(origPath, shimDir)

    os.Setenv("PATH", filteredPath)
    defer os.Setenv("PATH", origPath)

    return exec.LookPath(tool)
}

// filterPathEntries removes shimDir from the colon-separated (or semicolon on Windows) PATH.
func filterPathEntries(path, exclude string) string {
    sep := string(os.PathListSeparator) // ":" on Unix, ";" on Windows
    parts := strings.Split(path, sep)
    var filtered []string
    for _, p := range parts {
        if filepath.Clean(p) != filepath.Clean(exclude) {
            filtered = append(filtered, p)
        }
    }
    return strings.Join(filtered, sep)
}
```

**PATH instruction snippet (printed by `shim install`):**

```go
// Source: [ASSUMED] — standard PATH prepend instructions per shell
func printPathInstructions(shimDir string, out io.Writer) {
    fmt.Fprintf(out, "\nAdd the shim directory to the beginning of your PATH:\n\n")
    fmt.Fprintf(out, "  bash/zsh:   export PATH=%q:$PATH\n", shimDir)
    fmt.Fprintf(out, "  fish:       fish_add_path --prepend %q\n", shimDir)
    fmt.Fprintf(out, "  PowerShell: $env:PATH = %q + ';' + $env:PATH\n", shimDir)
    fmt.Fprintf(out, "\nAdd the appropriate line to your shell RC file (~/.bashrc, ~/.zshrc, etc.)\n")
}
```

### Pattern 10: Multi-Agent Context Propagation (INTG-07)

**What:** Extend `policy.ToolCall` and `policy.Evaluate` to accept an `AgentContext`; extend `audit.AuditRecord` with lineage fields; read context from environment in `check.RunCheck`.

```go
// Source: [VERIFIED: internal/policy/types.go — existing ToolCall and Decision structs]
// internal/policy/types.go — ADD AgentContext

// AgentContext carries multi-agent lineage from the caller to the policy engine.
// It is a pure struct (no I/O, no goroutines). Empty values mean root context.
type AgentContext struct {
    AgentID       string   // current agent session ID (from BEEKEEPER_AGENT_ID env)
    ParentAgentID string   // parent agent session ID (from BEEKEEPER_PARENT_AGENT_ID env)
    Depth         int      // nesting depth; 0 = root (from BEEKEEPER_AGENT_DEPTH env)
    Lineage       []string // ordered parent IDs from root to parent; caller-constructed
}
```

```go
// Source: [VERIFIED: internal/policy/engine.go — Evaluate signature]
// internal/policy/engine.go — EXTEND Evaluate signature

// Evaluate is extended to accept AgentContext. The context is used to:
// 1. Enforce that child agents cannot exceed parent policy level
// 2. Include lineage in the returned Decision for audit records
//
// Existing callers (check.RunCheck) must be updated to pass AgentContext.
// Zero value AgentContext{} is the safe default (root context, no lineage check).
func Evaluate(tc ToolCall, idx MultiCatalogLookup, t CorroborationThresholds, ac AgentContext) Decision {
    // ... existing logic unchanged ...
    // Add: lineage depth check
    if ac.Depth > maxAgentDepth {
        return Decision{
            Allow:  false,
            Level:  "block",
            Reason: fmt.Sprintf("agent depth %d exceeds maximum %d", ac.Depth, maxAgentDepth),
            RuleIDs: []string{"INTG-07"},
        }
    }
    // ... rest unchanged ...
}
```

**Reading AgentContext in `check.RunCheck`:**

```go
// Source: [VERIFIED: CONTEXT.md INTG-07 — env var names locked]
// internal/check/handler.go extension

func readAgentContext() policy.AgentContext {
    depth := 0
    if d := os.Getenv("BEEKEEPER_AGENT_DEPTH"); d != "" {
        depth, _ = strconv.Atoi(d)
    }
    lineage := []string{}
    if l := os.Getenv("BEEKEEPER_AGENT_LINEAGE"); l != "" {
        lineage = strings.Split(l, ",")
    }
    return policy.AgentContext{
        AgentID:       os.Getenv("BEEKEEPER_AGENT_ID"),
        ParentAgentID: os.Getenv("BEEKEEPER_PARENT_AGENT_ID"),
        Depth:         depth,
        Lineage:       lineage,
    }
}
```

**Audit record extension:**

```go
// Source: [VERIFIED: internal/audit/types.go — AuditRecord struct]
// internal/audit/types.go — EXTEND AuditRecord

type AuditRecord struct {
    // ... all existing fields unchanged ...

    // Phase 4 additions (INTG-07): multi-agent lineage
    AgentID       string   `json:"agent_id,omitempty"`
    ParentAgentID string   `json:"parent_agent_id,omitempty"`
    AgentDepth    int      `json:"agent_depth,omitempty"`
    AgentLineage  []string `json:"agent_lineage,omitempty"` // ordered parent IDs root→parent
}
```

**`beekeeper audit-record` command (PostToolUse hook):**

```go
// Source: [VERIFIED: CONTEXT.md — audit-record command spec]
// Reads PostToolUse JSON from stdin; writes tool_result audit record; exit 0 always.

// PostToolUse stdin JSON from Claude Code:
// {
//   "hook_event_name": "PostToolUse",
//   "tool_name": "Bash",
//   "tool_input": {"command": "npm test"},
//   "tool_output": {"exit_code": 0, "stdout": "...", "stderr": ""},
//   "tool_use_id": "unique-id"
// }
```

### Pattern 11: Gateway Fail-Closed with Panic Recovery

**What:** The gateway must apply the same top-level `defer recover()` pattern as `check.runCheck` to ensure policy panics result in a JSON-RPC error, not a silent forward.

```go
// Source: [VERIFIED: internal/check/handler.go — top-level recover pattern at line 93]
// internal/gateway/proxy.go

func (h *gatewayHandler) handleToolCall(w http.ResponseWriter, r *http.Request, msg parser.JSONRPCMessage, bodyBytes []byte) {
    // Fail-closed guard: any policy engine panic → JSON-RPC -32002 error
    defer func() {
        if rec := recover(); rec != nil {
            fmt.Fprintf(os.Stderr, "beekeeper gateway: recovered panic: %v\n", rec)
            writeJSONRPCError(w, msg.ID, -32002, "internal error (fail-closed)", nil)
        }
    }()

    // 500ms hard deadline for policy evaluation (CONTEXT.md: <100ms p95 target)
    evalCtx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
    defer cancel()

    tc := extractToolCall(msg)
    decision := policy.Evaluate(tc, h.idx, policy.DefaultCorroborationThresholds(), h.agentCtx(r))

    if evalCtx.Err() != nil {
        writeJSONRPCError(w, msg.ID, -32002, "policy timeout (fail-closed)", nil)
        return
    }
    // ... handle decision ...
}
```

### Pattern 12: Continue / OpenCode / OpenClaw Gateway Configuration

**What:** `beekeeper hooks install --target continue|opencode|openclaw` prints configuration instructions; no file is written.

**Continue.dev config format** (verified):
```yaml
# .continue/config.yaml (or ~/.continue/config.yaml)
mcpServers:
  - name: Beekeeper Gateway
    type: streamable-http
    url: http://127.0.0.1:7837/mcp
```
[VERIFIED: docs.continue.dev/customize/deep-dives/mcp — `type: streamable-http`, `url` field; auth via `env` injection]

Auth token must be passed as an environment variable (Continue does not support inline `Authorization` headers in config). The installer prints:
```
For Continue, set environment variable:
  export BEEKEEPER_GATEWAY_TOKEN=$(beekeeper gateway token)
```
And configure Continue's config.yaml to pass the token via env. (Continue's `${{ secrets.SECRET_NAME }}` syntax requires a secrets store; simpler is env var injection.)

**OpenCode config format** (verified):
```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "beekeeper": {
      "type": "remote",
      "url": "http://127.0.0.1:7837/mcp",
      "enabled": true,
      "headers": {
        "Authorization": "Bearer {env:BEEKEEPER_GATEWAY_TOKEN}"
      }
    }
  }
}
```
[VERIFIED: opencode.ai/docs/mcp-servers/ — `type: remote`, `url`, `headers` with `{env:VAR}` syntax confirmed]

**OpenClaw config format** (verified):
```json
{
  "mcp": {
    "servers": {
      "beekeeper": {
        "url": "http://127.0.0.1:7837/mcp",
        "transport": "streamable-http",
        "headers": {
          "Authorization": "Bearer <token>"
        }
      }
    }
  }
}
```
[VERIFIED: docs.openclaw.ai/cli/mcp — `url`, `transport: streamable-http`, `headers` with `Authorization` bearer confirmed]

### Anti-Patterns to Avoid

- **Anti-pattern: Using `httputil.ReverseProxy` as the sole handler for tool-call methods.** `ReverseProxy.Rewrite` cannot write a JSON-RPC error response to the client — it has no access to `http.ResponseWriter`. Handle tool-call methods manually; use `ReverseProxy` only for transparent passthrough of non-tool-call methods. [VERIFIED: pkg.go.dev/net/http/httputil — `Rewrite func(*ProxyRequest)` signature]

- **Anti-pattern: Correlating JSON-RPC responses by position.** Per MCP July 2026 spec and JSON-RPC 2.0, batch responses may arrive in any order. Always match by `id` field. [VERIFIED: blog.modelcontextprotocol.io/posts/2026-07-28-release-candidate/]

- **Anti-pattern: Assuming `initialize`/`initialized` handshake exists.** The MCP July 2026 spec removes this handshake entirely. Pass it through transparently (non-tool-call method) rather than implementing handshake state. [VERIFIED: MCP 2026-07-28 RC blog]

- **Anti-pattern: Treating the Claude Code hook `agent_id` field as available in Cursor/Codex hooks.** Claude Code passes `agent_id` in the hook stdin JSON. Cursor and Codex do NOT include `agent_id` in their hook stdin. INTG-07 context for Cursor/Codex must come exclusively from environment variables. [VERIFIED: cursor.com/docs/hooks stdin schema; developers.openai.com/codex/hooks stdin schema]

- **Anti-pattern: Writing Cursor hooks to `~/.cursor/settings.json`.** Cursor hooks live in `~/.cursor/hooks.json` with a completely different schema. `settings.json` is for editor/UI preferences, not hooks. [VERIFIED: cursor.com/docs/hooks]

- **Anti-pattern: Skipping `failClosed: true` in Cursor hook config.** If `failClosed` is false (the Cursor default), a hook crash is fail-open — the tool call proceeds. Beekeeper is a security tool; `failClosed: true` is required. [VERIFIED: cursor.com/docs/hooks — `failClosed` field description]

- **Anti-pattern: Setting `os.Setenv("PATH", ...)` globally in `findRealBinary`.** `os.Setenv` is process-wide and not goroutine-safe. Use `defer os.Setenv("PATH", origPath)` and ensure no concurrent goroutines read PATH during the lookup. For `beekeeper shim install` (a synchronous CLI command, not a daemon), this is safe. [ASSUMED — standard Go concurrency warning]

- **Anti-pattern: Using `httputil.ReverseProxy.Director` instead of `Rewrite`.** `Director` is deprecated in Go 1.25 and has known security issues (X-Forwarded-For preservation). Always use `Rewrite`. [VERIFIED: pkg.go.dev/net/http/httputil]

- **Anti-pattern: Storing gateway token in cleartext in `config.json`.** Token is runtime state, not user config — it belongs in `state.json` (created at runtime, `0600` permissions) not `config.json` (user-edited). [CITED: CONTEXT.md INTG-03]

- **Anti-pattern: Implementing JSON-RPC batch by forwarding all items to upstream then correlating.** The gateway must evaluate policy on each batch item individually before forwarding any of them. A batch containing one blocked and one allowed call must: block the blocked one, forward only the allowed one, and return both responses correlated by `id`. [CITED: CONTEXT.md INTG-04 — correlation by id]

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| HTTP reverse proxy | Custom TCP tunnel | `net/http/httputil.ReverseProxy` | Handles hop-by-hop headers, connection reuse, TLS; Go stdlib |
| JSONC settings.json edit | Custom comment-aware JSON writer | `editorinit.PatchSettings` (already built) | Existing Phase 3 code; tested; atomic write |
| Random token generation | `math/rand` token | `crypto/rand` (already used in `check.newRecordID`) | Cryptographically secure; same pattern as existing code |
| MCP client library | Custom MCP client | None needed — gateway is a transparent HTTP proxy | Gateway does not need to be an MCP client; it proxies HTTP |
| JSON-RPC library | github.com/sourcegraph/jsonrpc2 | `encoding/json` stdlib | One new dep for <50 lines of parsing; stdlib is sufficient |
| Constant-time token comparison | `==` string comparison | `crypto/subtle.ConstantTimeCompare` | Prevents timing attacks on token auth |

**Key insight:** Phase 4 is primarily wiring phase — the policy engine, audit logging, catalog lookup, and JSONC editing are all built. The new code is integrations (hook installer, gateway, shim) that delegate to existing infrastructure. No new algorithmic complexity is needed.

---

## Runtime State Inventory

| Category | Items Introduced | Action Required |
|----------|-----------------|-----------------|
| Stored data | `gateway_token` in `~/.beekeeper/state.json` — 64-char hex, rotates on each restart | Gateway writes at startup; `beekeeper gateway token` reads it |
| Stored data | `~/.beekeeper/shims/` directory with shim scripts | `beekeeper shim install` creates; `shim uninstall` removes |
| Live service config | Gateway daemon PID in state.json (`pid` field) for `gateway status` check | Written at startup; stale if process crashed without cleanup |
| OS-registered state | PATH modification for shims — not OS-registered; user manually edits shell RC | Installer prints instructions; no automated RC modification |
| Secrets/env vars | `BEEKEEPER_GATEWAY_TOKEN` env var (printed for user, used in Continue/OpenCode config) | Not stored in code; user reads from `beekeeper gateway token` |
| Build artifacts | None — no new binaries; shim scripts reference the existing `beekeeper` binary | None |

**Hook files written by installer:**
| Target | File Written | Action Required |
|--------|-------------|-----------------|
| claude-code | `~/.claude/settings.json` (JSONC patch, hooks key) | Backup created automatically |
| cursor | `~/.cursor/hooks.json` (JSON, version+hooks keys) | Backup created automatically |
| codex | `~/.codex/hooks.json` (JSON, hooks key) | User must trust hook in Codex on next run |
| continue, opencode, openclaw | None — instructions printed | User edits their config manually |

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All builds | ✓ | 1.25 | — |
| `net/http` stdlib | INTG-03 gateway | ✓ | stdlib | — |
| `net/http/httputil` stdlib | INTG-03 transparent proxy | ✓ | stdlib | — |
| `crypto/rand` stdlib | INTG-03 token gen | ✓ | stdlib | — |
| `crypto/subtle` stdlib | Token comparison | ✓ | stdlib | — |
| `github.com/tidwall/jsonc` | INTG-01 Claude Code | ✓ | v0.3.3 (in go.mod) | — |
| `~/.claude/` directory | INTG-01 | ✓ on machines with Claude Code | — | Installer skips if dir missing; prints warning |
| `~/.cursor/` directory | INTG-02 cursor | ✓ on machines with Cursor | — | Installer skips if dir missing; `--force` creates it |
| `~/.codex/` directory | INTG-02 codex | ✓ on machines with Codex CLI | — | Installer skips if dir missing; `--force` creates it |
| MCP upstream server | INTG-03 gateway | ✗ (user-supplied) | — | Gateway refuses to start without `--upstream` |
| npm, pip, cargo, etc. | INTG-06 shims | Varies per machine | — | Skip shim if tool not in PATH; not an error |

**Missing dependencies with no fallback:**
- MCP upstream server URL — required at gateway startup; gateway refuses to start without it

**Missing dependencies with fallback:**
- Target editor directories (`~/.claude/`, `~/.cursor/`, `~/.codex/`) — installer skips gracefully
- Individual tools for shimming (npm, pip, etc.) — skip if not in PATH

---

## Common Pitfalls

### Pitfall 1: Cursor Uses `hooks.json`, Not `settings.json`

**What goes wrong:** Hook installer writes Beekeeper hooks to `~/.cursor/settings.json` (by analogy with Claude Code). Cursor ignores these hooks entirely; no interception occurs.

**Why it happens:** Claude Code stores all settings including hooks in `settings.json`. Cursor stores hooks separately in `hooks.json` with a completely different schema.

**How to avoid:** The `--target cursor` installer writes to `~/.cursor/hooks.json`. The installer for Claude Code and Cursor must use completely different code paths. [VERIFIED: cursor.com/docs/hooks]

**Warning signs:** `beekeeper hooks install --target cursor` succeeds but Cursor does not call `beekeeper check` on tool calls.

### Pitfall 2: Codex CLI Requires Hook Trust Confirmation

**What goes wrong:** Beekeeper writes `~/.codex/hooks.json` with the PreToolUse hook. Codex CLI skips the hook on the first run because the hook hash is unknown and untrusted.

**Why it happens:** Codex's trust model requires explicit user approval of each new hook definition. The trust decision is recorded against the hook's current hash. [VERIFIED: developers.openai.com/codex/hooks]

**How to avoid:** After `beekeeper hooks install --target codex`, print: "Codex requires you to trust this hook. Run Codex once and approve the hook prompt before Beekeeper interception takes effect."

**Warning signs:** Codex does not call `beekeeper check` after installation; no audit records appear.

### Pitfall 3: MCP Gateway Forwarding Before Policy Evaluation

**What goes wrong:** The gateway uses `httputil.ReverseProxy` as the sole handler. The `Rewrite` function evaluates policy and tries to abort forwarding by writing an error to... nothing (there's no `http.ResponseWriter` in `Rewrite`). The request is forwarded anyway.

**Why it happens:** `Rewrite func(*ProxyRequest)` has no return value and no access to `http.ResponseWriter`. You cannot abort forwarding from inside `Rewrite`. [VERIFIED: pkg.go.dev/net/http/httputil]

**How to avoid:** Handle `tools/call` methods manually in `ServeHTTP` before `ReverseProxy.ServeHTTP` is ever called. Use `ReverseProxy` only for transparent passthrough of non-tool-call methods.

**Warning signs:** Policy blocks are logged but the request is still forwarded to upstream; the agent receives the upstream response instead of the JSON-RPC -32001 error.

### Pitfall 4: JSON-RPC `id` Field Type Handling

**What goes wrong:** JSON-RPC 2.0 allows `id` to be a string, integer, or null. Go's `json.Unmarshal` into `int` fails for string IDs; unmarshaling into `string` fails for integer IDs. The gateway returns -32700 parse error for valid requests with integer IDs.

**Why it happens:** JSON-RPC 2.0 spec allows `string | number | null` for `id`. Go requires explicit `any` or `json.RawMessage` to handle all three types. [VERIFIED: json-rpc 2.0 spec]

**How to avoid:** Use `ID any` (or `ID json.RawMessage`) in the `JSONRPCMessage` struct. When constructing the error response, echo back the same `id` field value (including null for notifications). [VERIFIED: parser.go pattern above]

**Warning signs:** Requests with integer `id` values fail with parse errors; Claude Code uses integer IDs by default.

### Pitfall 5: `findRealBinary` PATH Race in Daemon Contexts

**What goes wrong:** In a future phase where `beekeeper shim install` is run from a daemon context, the `os.Setenv("PATH", filteredPath)` + `exec.LookPath` + `os.Setenv("PATH", origPath)` sequence is not goroutine-safe. A concurrent goroutine may read PATH during the temporary modification.

**Why it happens:** `os.Setenv` modifies the process-wide environment, visible to all goroutines.

**How to avoid:** For the Phase 4 CLI command (synchronous, single-goroutine), this is safe. Add a `sync.Mutex` if this logic is ever called from a goroutine. Alternatively, manually split PATH, check each entry for the tool binary, and skip the shim directory — no `os.Setenv` needed.

**Warning signs:** Shim install occasionally finds the shim binary as "the real binary" (pointing the shim at itself), creating an infinite loop when npm is called.

### Pitfall 6: Gateway Token File Permissions

**What goes wrong:** `state.json` is written with `0644` permissions. Another local process reads the gateway token and authenticates to the gateway.

**Why it happens:** `os.WriteFile(path, data, 0644)` is the default. The token provides "security" against other local processes only if the file is owner-read-only.

**How to avoid:** Write `state.json` with `0600` permissions (same as `config.json` — `config.Save` uses `0600`). On Windows, use `platform.SetOwnerOnly` (same as Phase 2 audit log pattern). [CITED: CONTEXT.md — per-session token is the local process protection mechanism]

**Warning signs:** `ls -la ~/.beekeeper/state.json` shows `-rw-r--r--` (world-readable).

### Pitfall 7: MCP July 2026 `initialize` Passthrough vs. Handling

**What goes wrong:** The gateway intercepts `initialize` and `initialized` methods, thinking it needs to maintain handshake state. These methods do not exist in the MCP July 2026 spec. If a client sends them (compatibility mode), the gateway blocks them with a "method not found" error, breaking the connection.

**Why it happens:** Confusion between MCP 2025-11-25 (had handshake) and MCP 2026-07-28 (handshake removed).

**How to avoid:** Treat all non-`tools/call` methods as transparent passthrough. If `initialize` appears (from an older client), pass it to upstream unchanged — let upstream decide. Do not handle it in the gateway. [VERIFIED: MCP 2026-07-28 RC]

**Warning signs:** MCP clients using older spec cannot connect to the gateway; `initialize` requests return errors.

### Pitfall 8: Shim Script Line Ending on Windows

**What goes wrong:** The `.cmd` batch file is written with LF line endings (Unix). Windows `cmd.exe` requires CRLF line endings. The batch file fails to execute.

**Why it happens:** Go's `os.WriteFile` writes bytes as-is. Go string literals use `\n` (LF).

**How to avoid:** When building Windows `.cmd` shim content, use `\r\n` line endings. Use a `strings.ReplaceAll(content, "\n", "\r\n")` pass before writing on Windows, or explicitly construct the content with `\r\n`. [ASSUMED — Windows cmd.exe CRLF requirement; verified by CONTEXT.md noting Windows as primary dev machine]

**Warning signs:** `npm.cmd` is created but executing it in cmd.exe produces no output or "The syntax of the command is incorrect."

---

## Code Examples

### Gateway Token Verification Middleware

```go
// Source: [VERIFIED: crypto/subtle.ConstantTimeCompare; net/http stdlib]
// internal/gateway/proxy.go

const bearerPrefix = "Bearer "

func (h *gatewayHandler) verifyToken(r *http.Request) bool {
    auth := r.Header.Get("Authorization")
    if !strings.HasPrefix(auth, bearerPrefix) {
        return false
    }
    candidate := auth[len(bearerPrefix):]
    return subtle.ConstantTimeCompare([]byte(candidate), []byte(h.token)) == 1
}

func writeJSONRPCError(w http.ResponseWriter, id any, code int, msg string, data any) {
    resp := map[string]any{
        "jsonrpc": "2.0",
        "id":      id,
        "error": map[string]any{
            "code":    code,
            "message": msg,
            "data":    data,
        },
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK) // JSON-RPC errors use HTTP 200
    _ = json.NewEncoder(w).Encode(resp)
}
```

### Hook Installer: Backup Before Modify

```go
// Source: [VERIFIED: os stdlib; CONTEXT.md backup requirement]
// internal/hooks/hooks.go

func backupSettings(path string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            return nil // nothing to back up
        }
        return err
    }
    ts := time.Now().Format("20060102-150405")
    backupPath := path + ".beekeeper-backup-" + ts
    return os.WriteFile(backupPath, data, 0o644)
}
```

### Gateway State: Load/Save Pattern

```go
// Source: [VERIFIED: internal/catalog/state.go — LoadState/SaveState canonical pattern]
// internal/gateway/state.go

type topLevelState struct {
    // Preserve existing sources key from catalog.WatchState
    Sources map[string]catalog.SourceState `json:"sources,omitempty"`
    // Gateway state as a nested key
    Gateway *GatewayState `json:"gateway,omitempty"`
}

func LoadGatewayState(path string) (GatewayState, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            return GatewayState{}, nil
        }
        return GatewayState{}, fmt.Errorf("read state %q: %w", path, err)
    }
    var top topLevelState
    if err := json.Unmarshal(data, &top); err != nil {
        return GatewayState{}, fmt.Errorf("parse state %q: %w", path, err)
    }
    if top.Gateway == nil {
        return GatewayState{}, nil
    }
    return *top.Gateway, nil
}

func SaveGatewayState(path string, gw GatewayState) error {
    // Load existing state to preserve sources key
    data, _ := os.ReadFile(path)
    var top topLevelState
    _ = json.Unmarshal(data, &top) // tolerate missing file

    top.Gateway = &gw
    out, err := json.MarshalIndent(top, "", "  ")
    if err != nil {
        return fmt.Errorf("marshal state: %w", err)
    }
    return writeFileAtomic(path, out)
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| MCP initialize/initialized handshake per connection | No handshake; capabilities in `_meta` on every request | MCP 2026-07-28 spec | Gateway needs no session state; simpler, scales on HTTP infrastructure |
| MCP session via `Mcp-Session-Id` header | Session header removed entirely | MCP 2026-07-28 spec | Gateway is truly stateless; any server instance can handle any request |
| `httputil.ReverseProxy.Director` | `httputil.ReverseProxy.Rewrite` | Go 1.22+ | Fixes X-Forwarded-For spoofing; hop-by-hop headers removed before Rewrite |
| Claude Code only hook system | Cursor (hooks.json v1), Codex (hooks.json), OpenAI (hooks) | 2025-2026 | Each agent CLI now has its own hook mechanism; no single schema |

**Deprecated/outdated:**
- `httputil.ReverseProxy.Director`: deprecated in Go 1.22, removed guidance in 1.25 docs — use `Rewrite` instead.
- MCP SSE transport for client-to-server: Streamable HTTP POST is the standard for MCP 2026-07-28.
- MCP `initialize` / `initialized` methods: removed from the 2026-07-28 spec.

---

## Open Questions

1. **MCP July 2026 exact `tools/call` method name spelling**
   - What we know: MCP spec uses `tools/call` (slash-separated). The gateway intercepts on this exact string.
   - What's unclear: Whether MCP clients send `tools/call` or `tools.call` or `tool_call` — the spec says `tools/call`.
   - Recommendation: Intercept on `"tools/call"` (verified from MCP spec examples). Add a test that passes through `"tools/list"` and `"resources/list"` without policy evaluation.

2. **`beekeeper check` structured JSON output vs exit-code-only**
   - What we know: Claude Code hooks can parse `hookSpecificOutput.permissionDecision` from stdout JSON (verified). Exit code 2 also blocks.
   - What's unclear: Whether `beekeeper check` should emit the structured `hookSpecificOutput` JSON for richer denial messages in Claude Code, or rely solely on exit code 2 for blocking.
   - Recommendation: Emit structured JSON output (the existing `json.Marshal(d)` call in `finalize` already writes the Decision struct). Add a `hookSpecificOutput` wrapper in the output for Claude Code compatibility. This is a low-risk addition.

3. **Cursor `agent_id` propagation for INTG-07**
   - What we know: Cursor's hook stdin does NOT include `agent_id`. Claude Code's hook stdin DOES include `agent_id`. [VERIFIED from respective docs]
   - What's unclear: Whether Cursor exports any session/agent identifier as an environment variable that could be used for INTG-07 lineage tracking.
   - Recommendation: For Cursor, INTG-07 lineage must come exclusively from `BEEKEEPER_AGENT_ID` / `BEEKEEPER_PARENT_AGENT_ID` environment variables set by the user or orchestration layer. Document as a known limitation.

4. **Gateway default port 7837 availability**
   - What we know: Port 7837 is not a well-known registered port.
   - What's unclear: Whether another common tool uses this port in developer environments.
   - Recommendation: Use 7837 as default. Implement fallback-to-random-port (`net.Listen("tcp", "127.0.0.1:0")`) when 7837 is busy. Store the actual bound port in state.json so `beekeeper gateway token` and `gateway status` read the correct address.

5. **MCP gateway `warn` injection field name**
   - What we know: On a warn decision, the gateway forwards to upstream and injects a warning field into the response. The field name `_beekeeper_warning` is from the CONTEXT.md spec.
   - What's unclear: Whether MCP clients silently ignore unknown fields in tool responses, or whether the injected field causes parse errors in some clients.
   - Recommendation: Use `_beekeeper_warning` (underscore prefix signals a vendor extension). MCP JSON-RPC responses are parsed by the client; unknown fields in `result` should be ignored per JSON parsing norms. Test with actual Claude Code client.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `beekeeper check` structured JSON output (`hookSpecificOutput`) is compatible with Claude Code hook parsing | Pattern 1 | If Claude Code ignores the JSON and only uses exit codes, the structured output is harmless but unused |
| A2 | Cursor's `preToolUse` hook runs the command with the project directory as cwd, accessible to `beekeeper` binary in PATH | Pattern 2 | If cwd is wrong or beekeeper not in PATH, hook silently fails; `failClosed: true` would then block all tool calls |
| A3 | MCP clients (Claude Code, Continue, OpenClaw) silently ignore unknown fields in JSON-RPC `result` objects | Open Question 5 | If any client fails on unknown fields, the `_beekeeper_warning` injection breaks warn-level forwarding |
| A4 | Gateway port 7837 is not commonly used by other developer tools on Windows | Open Question 4 | If port is in use, gateway startup fails; fallback-to-random mitigates this |
| A5 | The Codex CLI trust confirmation for new hooks is a one-time action per hook hash | Pattern 3 | If trust is per-session, users must re-trust on every Codex start; installer must warn more prominently |
| A6 | `state.json` top-level structure is a flat JSON object that can hold both `"sources"` (catalog.WatchState) and `"gateway"` keys without conflict | Pattern 8 / Gateway State | If `catalog.LoadState` uses strict unmarshaling that rejects unknown keys, adding `"gateway"` breaks catalog state loading; but `json.Unmarshal` ignores unknown fields by default |
| A7 | Windows `.cmd` batch files require CRLF line endings | Pitfall 8 | LF-only .cmd files may fail silently in some Windows configurations; needs validation on `windows-latest` CI |
| A8 | `BEEKEEPER_AGENT_LINEAGE` env var (comma-separated parent IDs) is sufficient for lineage propagation without a separate `--lineage` flag | Pattern 10 | If parent IDs contain commas, splitting is wrong; recommend URL-encoding or base64 in a future revision |

---

## Project Constraints (from CLAUDE.md)

- **Go 1.25+, single static binary, no CGO in core** — All Phase 4 packages use only stdlib. No new external dependencies required. `net/http/httputil`, `crypto/rand`, `crypto/subtle` are all stdlib.
- **`internal/policy` must be pure (no I/O)** — `AgentContext` is a pure struct. The `Evaluate` signature extension adds `AgentContext` as a value parameter — no I/O added. Reading env vars (`os.Getenv`) happens in `internal/check/handler.go` (I/O tier), not in policy.
- **Fail closed by default** — Gateway panic recovery writes -32002 JSON-RPC error; policy timeout writes -32002 error. `fail_open` config applies to the gateway the same way as `check.RunCheck`.
- **MCP gateway: stateless per-request** — No session accumulation. Token is per-startup, not per-request-session. Confirmed compatible with MCP 2026-07-28 spec.
- **Correlate JSON-RPC responses by `id` field, never by position** — `JSONRPCMessage.ID any` preserves the exact id type for echo-back in error responses.
- **Bubble Tea: `charm.land/bubbletea/v2`** — Not used in Phase 4 (TUI is Phase 8).
- **eBPF: pre-compiled bytecode** — Not used in Phase 4 (Sentry is Phase 5).
- **ETW: `tekert/golang-etw`** — Not used in Phase 4.
- **Windows primary dev machine** — Shim layer has explicit `shim_windows.go` / `shim_unix.go` with `//go:build` tags. `.cmd` batch files for Windows. CRLF line endings for Windows batch files.
- **Cobra wiring is thin** — All hook, gateway, shim business logic in `internal/hooks/`, `internal/gateway/`, `internal/shim/`. `cmd/beekeeper/main.go` adds only the subcommand wiring.
- **No WSL integration tests** — Gateway tests use `httptest.Server` stubs; no live MCP upstream calls.
- **Reproducible builds** — No new go.mod deps; existing deps unchanged. `go mod verify` continues to pass.
- **Fuzz tests for MCP message parser are a RELEASE GATE** — `FuzzParseMessage` must exist in `internal/gateway/parser_fuzz_test.go` with `-tags fuzz` build tag. CI seed corpus check: `go test -tags fuzz -run=FuzzParseMessage ./internal/gateway/...` must pass as part of CI and release gate.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go standard `testing` package + `go test` |
| Config file | None — `go test ./...` discovers automatically |
| Quick run command | `go test ./internal/... -count=1` |
| Full suite command | `go test -race -count=1 ./...` (CI-only) |
| Fuzz seed run command | `go test -tags fuzz -run=FuzzParseMessage ./internal/gateway/...` |
| Fuzz active run command | `go test -tags fuzz -fuzz=FuzzParseMessage -fuzztime=60s ./internal/gateway/...` |

### Phase Requirements to Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| INTG-01 | `installClaudeCode` merges hooks key into existing JSONC settings.json | unit | `go test ./internal/hooks/... -run TestInstallClaudeCode -v` | ❌ Wave 0 |
| INTG-01 | `installClaudeCode` is idempotent (no duplication on re-run) | unit | `go test ./internal/hooks/... -run TestInstallClaudeCodeIdempotent -v` | ❌ Wave 0 |
| INTG-01 | `installClaudeCode --dry-run` prints without modifying | unit | `go test ./internal/hooks/... -run TestInstallClaudeCodeDryRun -v` | ❌ Wave 0 |
| INTG-01 | Backup file created before modification | unit | `go test ./internal/hooks/... -run TestInstallBackup -v` | ❌ Wave 0 |
| INTG-02 | `installCursor` writes correct hooks.json schema (version:1, hooks.preToolUse, failClosed:true) | unit | `go test ./internal/hooks/... -run TestInstallCursor -v` | ❌ Wave 0 |
| INTG-02 | `installCursor` is idempotent | unit | `go test ./internal/hooks/... -run TestInstallCursorIdempotent -v` | ❌ Wave 0 |
| INTG-02 | `installCodex` writes correct hooks.json schema | unit | `go test ./internal/hooks/... -run TestInstallCodex -v` | ❌ Wave 0 |
| INTG-02 | `installCodex` is idempotent | unit | `go test ./internal/hooks/... -run TestInstallCodexIdempotent -v` | ❌ Wave 0 |
| INTG-03 | Gateway binds to 127.0.0.1 by default (not 0.0.0.0) | integration | `go test ./internal/gateway/... -run TestGatewayLocalOnlyBind -v` | ❌ Wave 0 |
| INTG-03 | Request without Authorization header returns -32600 error | unit | `go test ./internal/gateway/... -run TestGatewayUnauthorized -v` | ❌ Wave 0 |
| INTG-03 | Request with wrong token returns -32600 error | unit | `go test ./internal/gateway/... -run TestGatewayWrongToken -v` | ❌ Wave 0 |
| INTG-03 | Request body > 1MB returns -32700 error | unit | `go test ./internal/gateway/... -run TestGatewayOversizedBody -v` | ❌ Wave 0 |
| INTG-03 | Malformed JSON body returns -32700 error | unit | `go test ./internal/gateway/... -run TestGatewayMalformedJSON -v` | ❌ Wave 0 |
| INTG-03 | Token stored in state.json with 0600 permissions | unit | `go test ./internal/gateway/... -run TestGatewayStatePermissions -v` | ❌ Wave 0 |
| INTG-04 | tools/call with blocked package returns -32001 JSON-RPC error | integration | `go test ./internal/gateway/... -run TestGatewayBlocksToolCall -v` | ❌ Wave 0 |
| INTG-04 | tools/call with allowed package forwards to upstream | integration | `go test ./internal/gateway/... -run TestGatewayAllowsToolCall -v` | ❌ Wave 0 |
| INTG-04 | Non-tool-call method is proxied transparently (no policy eval) | unit | `go test ./internal/gateway/... -run TestGatewayPassthrough -v` | ❌ Wave 0 |
| INTG-04 | Policy engine panic → -32002 error (fail-closed) | unit | `go test ./internal/gateway/... -run TestGatewayFailClosed -v` | ❌ Wave 0 |
| INTG-04 | JSON-RPC id field echoed back correctly (string, int, null) | unit | `go test ./internal/gateway/... -run TestGatewayIDCorrelation -v` | ❌ Wave 0 |
| INTG-04 (fuzz) | FuzzParseMessage — never panics, always typed error | fuzz | `go test -tags fuzz -run=FuzzParseMessage ./internal/gateway/...` | ❌ Wave 0 |
| INTG-04 (fuzz) | FuzzParseMessage — method > 256 bytes rejected | fuzz seed | `go test -tags fuzz -run=FuzzParseMessage ./internal/gateway/...` | ❌ Wave 0 |
| INTG-04 (fuzz) | FuzzParseMessage — batch > 50 items rejected | fuzz seed | `go test -tags fuzz -run=FuzzParseMessage ./internal/gateway/...` | ❌ Wave 0 |
| INTG-04 (fuzz) | FuzzParseMessage — depth > 10 rejected | fuzz seed | `go test -tags fuzz -run=FuzzParseMessage ./internal/gateway/...` | ❌ Wave 0 |
| INTG-06 | `shim install` creates executable shell script for npm on Unix | unit | `go test ./internal/shim/... -run TestShimInstallUnix -v` | ❌ Wave 0 |
| INTG-06 | `shim install` creates .cmd batch file for npm on Windows | unit | `go test ./internal/shim/... -run TestShimInstallWindows -v` | ❌ Wave 0 |
| INTG-06 | Shim script points to real binary (not itself) | unit | `go test ./internal/shim/... -run TestShimRealBinary -v` | ❌ Wave 0 |
| INTG-06 | `shim uninstall` removes all shim files | unit | `go test ./internal/shim/... -run TestShimUninstall -v` | ❌ Wave 0 |
| INTG-06 | `shim install` is idempotent (overwrites existing shims) | unit | `go test ./internal/shim/... -run TestShimIdempotent -v` | ❌ Wave 0 |
| INTG-07 | `AgentContext` with depth > maxAgentDepth → block decision | unit | `go test ./internal/policy/... -run TestAgentContextDepthBlock -v` | ❌ Wave 0 |
| INTG-07 | `AuditRecord` contains agent_id, parent_agent_id, agent_depth | unit | `go test ./internal/audit/... -run TestAuditRecordAgentContext -v` | ❌ Wave 0 |
| INTG-07 | `readAgentContext` reads env vars correctly | unit | `go test ./internal/check/... -run TestReadAgentContext -v` | ❌ Wave 0 |
| audit-record | `beekeeper audit-record` writes tool_result record; exits 0 always | unit | `go test ./... -run TestAuditRecordCommand -v` | ❌ Wave 0 |
| audit-record | `beekeeper audit-record` exits 0 even on malformed stdin | unit | `go test ./... -run TestAuditRecordMalformedStdin -v` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/... -count=1` (< 30 seconds on Windows)
- **Per wave merge:** `go test -race -count=1 ./...` + `go test -tags fuzz -run=FuzzParseMessage ./internal/gateway/...` (CI-only)
- **Phase gate:** Full test suite green + fuzz seed corpus passing on all 3 platforms before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/hooks/hooks_test.go` — TestInstallClaudeCode, TestInstallCursor, TestInstallCodex (all idempotent + dryrun variants)
- [ ] `internal/hooks/testdata/claude_settings.json` — synthetic fixture with JSONC comments
- [ ] `internal/hooks/testdata/cursor_hooks.json` — existing Cursor hooks fixture
- [ ] `internal/hooks/testdata/codex_hooks.json` — existing Codex hooks fixture
- [ ] `internal/gateway/gateway_test.go` — TestGatewayLocalOnlyBind, TestGatewayUnauthorized, TestGatewayWrongToken, TestGatewayOversizedBody, TestGatewayStatePermissions
- [ ] `internal/gateway/proxy_test.go` — TestGatewayBlocksToolCall, TestGatewayAllowsToolCall, TestGatewayPassthrough, TestGatewayFailClosed, TestGatewayIDCorrelation
- [ ] `internal/gateway/parser_test.go` — table-driven parser bounds tests
- [ ] `internal/gateway/parser_fuzz_test.go` — FuzzParseMessage (RELEASE GATE)
- [ ] `internal/gateway/testdata/fuzz/FuzzParseMessage/` — 8 seed corpus files
- [ ] `internal/shim/shim_test.go` — TestShimInstallUnix, TestShimInstallWindows, TestShimRealBinary, TestShimUninstall, TestShimIdempotent
- [ ] `internal/policy/engine_test.go` — EXTEND: TestAgentContextDepthBlock
- [ ] `internal/audit/types_test.go` — EXTEND: TestAuditRecordAgentContext
- [ ] `internal/check/handler_test.go` — EXTEND: TestReadAgentContext

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | Yes — gateway per-session token auth | `crypto/rand` 32-byte token; `subtle.ConstantTimeCompare`; `0600` state.json |
| V3 Session Management | Yes — gateway token rotation on restart | Token rotates on each `beekeeper gateway` start; no long-lived tokens |
| V4 Access Control | Yes — localhost-only binding; per-session token | Default bind `127.0.0.1`; `allow_remote_gateway` explicit opt-out |
| V5 Input Validation | Yes — JSON-RPC parsing, hook file inputs, shim tool names | `ParseMessage` with all bounds; 1MB cap; maxMethodLen=256; maxBatchItems=50; maxRecursionDepth=10 |
| V6 Cryptography | Yes — token generation | `crypto/rand`; never `math/rand` |
| V7 Error Handling | Yes — gateway fail-closed; hook installer backup | Top-level recover; -32002 on panic; backup before modify |

### Known Threat Patterns for Phase 4 Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Token brute-force by local process | Elevation of Privilege | 256-bit token space; `subtle.ConstantTimeCompare`; `0600` state.json |
| Oversized JSON-RPC body causing OOM | DoS | 1MB `io.LimitReader` cap (same as `check.RunCheck`) |
| Deeply nested JSON parameters causing stack overflow | DoS | `maxRecursionDepth=10` in `ParseMessage` |
| Batch request DoS (10,000 items) | DoS | `maxBatchItems=50` hard cap |
| MCP method name injection with embedded newlines | Tampering | `maxMethodLen=256`; string comparison in router |
| Shim pointing to itself (infinite loop) | Tampering/DoS | PATH filtering in `findRealBinary` removes shim dir before LookPath |
| Hook installer writing to wrong file (path traversal in `--target`) | Tampering | `--target` is an enum (claude-code, cursor, codex, ...); no user-supplied path |
| Gateway token leaked via process list (`ps aux`) | Information Disclosure | Token in state.json (not in process args); state.json is `0600` |
| Agent context depth bypass (`BEEKEEPER_AGENT_DEPTH=-1`) | Elevation of Privilege | `max(0, depth)` normalization; negative depth treated as 0 (root) |
| JSON-RPC response injection via upstream MCP server | Tampering | Gateway validates own responses; `_beekeeper_warning` inject only on warn-path |
| `.cmd` batch file with special characters in real binary path | Tampering | Quote the real binary path in the batch file |

---

## Sources

### Primary (HIGH confidence)
- `code.claude.com/docs/en/hooks` — Full Claude Code hooks schema: PreToolUse/PostToolUse format in settings.json, matcher semantics, exit codes (0=allow, 2=block), stdin fields including `agent_id`, `tool_use_id`, structured output format (`hookSpecificOutput.permissionDecision`). Fetched 2026-05-26.
- `cursor.com/docs/hooks` — Cursor hooks.json schema: file location (`~/.cursor/hooks.json`), version:1, `preToolUse` (camelCase), `failClosed` field, stdin JSON fields (no agent_id), output format (`permission: allow|deny`). Platform-specific enterprise paths. Fetched 2026-05-26.
- `developers.openai.com/codex/hooks` — Codex CLI hooks schema: `~/.codex/hooks.json`, `PreToolUse` (PascalCase like Claude Code but separate file), `commandWindows` field, trust requirement. Fetched 2026-05-26.
- `blog.modelcontextprotocol.io/posts/2026-07-28-release-candidate/` — MCP July 2026 spec: `initialize`/`initialized` removed, `Mcp-Session-Id` removed, `_meta` per-request, HTTP POST + Streamable HTTP transport, `server/discover` replaces handshake. Fetched 2026-05-26.
- `pkg.go.dev/net/http/httputil` — `ReverseProxy.Rewrite func(*ProxyRequest)` signature, `ModifyResponse`, `ErrorHandler`, `ServeHTTP`, deprecation of `Director`. Fetched 2026-05-26.
- `go.dev/doc/security/fuzz/` — Fuzz test API: `FuzzXxx(*testing.F)`, `f.Add`, `f.Fuzz`, corpus directory `testdata/fuzz/FuzzXxx/`, run command `-fuzz=FuzzXxx`, CI regression with `-run=FuzzXxx`. Fetched 2026-05-26.
- `docs.continue.dev/customize/deep-dives/mcp` — Continue config format: `mcpServers`, `type: streamable-http`, `url` field. Fetched 2026-05-26.
- `opencode.ai/docs/mcp-servers/` — OpenCode config: `opencode.json`, `mcp.<name>.type: remote`, `url`, `headers` with `{env:VAR}` syntax. Fetched 2026-05-26.
- `docs.openclaw.ai/cli/mcp` — OpenClaw config: `mcp.servers.<name>.url`, `transport: streamable-http`, `headers.Authorization`. Fetched 2026-05-26.
- `internal/check/handler.go` — Existing fail-closed pattern (top-level recover, 1MB stdin cap, 5s timeout, crypto/rand token ID). Read 2026-05-26.
- `internal/editorinit/settings.go` — `PatchSettings` JSONC-safe patch pattern (tidwall/jsonc + atomic write). Read 2026-05-26.
- `internal/catalog/state.go` — `LoadState`/`SaveState` pattern for state.json. Read 2026-05-26.
- `internal/policy/fuzz_test.go` — Existing fuzz test pattern with `//go:build fuzz` tag. Read 2026-05-26.
- `go.mod` — Confirmed all Phase 4 packages already in go.mod (no new deps required). Read 2026-05-26.

### Secondary (MEDIUM confidence)
- `developers.openai.com/codex/config-reference` — Codex `config.toml` inline hooks (`hooks.<Event>` TOML tables), `commandWindows` override for Windows. Fetched 2026-05-26.
- `json-rpc.org/specification` — JSON-RPC 2.0 `id` field types (string | number | null), error object format, batch request/response ordering. Standard reference.

### Tertiary (LOW confidence)
- `github.com/weykon/agent-hooks` — Unified hook registration library (not used, but shows community convergence on PreToolUse/PostToolUse pattern across Claude Code, Cursor, Codex, Windsurf, Kiro). Checked 2026-05-26.
- `agenticcontrolplane.com/blog/codex-cli-hooks-reference` — Confirms Codex CLI trust requirement for new hooks; community blog (not official doc). Checked 2026-05-26.

---

## Metadata

**Confidence breakdown:**
- Claude Code hook schema: HIGH — verified from official docs.code.claude.com
- Cursor hooks.json schema: HIGH — verified from official cursor.com/docs/hooks
- Codex CLI hooks.json schema: HIGH — verified from official developers.openai.com/codex/hooks
- MCP July 2026 spec (stateless, no handshake): HIGH — verified from official modelcontextprotocol.io blog
- Go httputil.ReverseProxy API: HIGH — verified from pkg.go.dev
- Go fuzz test API: HIGH — verified from go.dev/doc/security/fuzz
- Continue/OpenCode/OpenClaw MCP config formats: HIGH — verified from official docs
- Gateway token security approach (crypto/rand + subtle.ConstantTimeCompare): HIGH — established Go security pattern
- Shim script content and PATH filtering logic: MEDIUM — design from CONTEXT.md, stdlib approach verified
- AgentContext struct and env var names: HIGH — locked in CONTEXT.md
- Windows .cmd CRLF requirement: ASSUMED — standard Windows constraint, needs CI validation

**Research date:** 2026-05-26
**Valid until:** 2026-06-25 (30 days; MCP spec is the most volatile area — check for RC changes before planning finalizes)
