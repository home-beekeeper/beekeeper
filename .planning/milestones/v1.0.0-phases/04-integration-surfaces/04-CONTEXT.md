# Phase 4: Integration Surfaces - Context

**Gathered:** 2026-05-26
**Status:** Ready for planning
**Source:** PRD Express Path (beekeeper-prd.md §6 + REQUIREMENTS.md INTG-01–07)

<domain>
## Phase Boundary

Phase 4 delivers every major agent integration surface — hook installs, the MCP gateway daemon, the shim layer, and multi-agent observability — so that Claude Code, Cursor, Codex CLI, Continue, OpenCode, and OpenClaw can all be protected through a single install command.

Five surfaces in scope:

1. **Hook installer** (`beekeeper hooks install`) — writes PreToolUse/PostToolUse hooks to the per-agent settings file for Claude Code, Cursor, and Codex CLI (INTG-01, INTG-02).
2. **MCP gateway daemon** (`beekeeper gateway`) — stateless per-request HTTP proxy; applies the full policy engine inline to every proxied tool call; per-session token auth; localhost-only by default; JSON-RPC response correlation by `id` field (INTG-03, INTG-04, INTG-05).
3. **Shim layer** (`beekeeper shim install/uninstall`) — PATH-prepended wrapper scripts for npm, pnpm, pip, cargo, go, gem, composer, npx, pipx; each shim invokes `beekeeper check` and proxies to the real binary only if allowed (INTG-06).
4. **Multi-agent context propagation** — parent-child lineage in policy decisions and audit records; `beekeeper check` reads agent context from env or tool call JSON (INTG-07).
5. **Self-defense hardening** — MCP message parser fuzz tests (release gate for v0.6.0), per-session token auth, bounded message sizes, localhost-only binding, audit log field redaction defaults.

Out of scope for this phase: Sentry daemon (Phase 5), LlamaFirewall sidecar (Phase 6), TUI dashboard (Phase 8), ContextForge/MCPGuard plugin mode (v1.5 deliverable).

</domain>

<decisions>
## Implementation Decisions

### INTG-01 + INTG-02: Hook Installer (`beekeeper hooks install`)

- New subcommand group `beekeeper hooks install --target <agent>` in `cmd/beekeeper/main.go`; business logic in `internal/hooks/`.
- Supported targets: `claude-code`, `cursor`, `codex`, `continue`, `opencode`, `openclaw`.
- **Claude Code** (`--target claude-code`): reads `~/.claude/settings.json` (JSONC, may have comments → use Phase 3's go-jsonc approach to preserve comments), merges in:
  ```json
  {
    "hooks": {
      "PreToolUse": [
        {"matcher": ".*", "hooks": [{"type": "command", "command": "beekeeper check"}]}
      ],
      "PostToolUse": [
        {"matcher": ".*", "hooks": [{"type": "command", "command": "beekeeper audit-record"}]}
      ]
    }
  }
  ```
- **Cursor** (`--target cursor`): Cursor uses the same Claude Code hook mechanism via `~/.cursor/settings.json` or via the MCP gateway — installer writes the same PreToolUse/PostToolUse pattern to the Cursor settings file.
- **Codex CLI** (`--target codex`): Codex CLI has its own hook config path; installer writes the appropriate hook entry.
- For gateway-based targets (continue, opencode, openclaw): installer prints configuration instructions for pointing the MCP client at `beekeeper gateway` (127.0.0.1:port) rather than writing a settings file.
- Installer is idempotent: re-running does not duplicate hooks; merges safely with existing hook arrays.
- Backs up settings.json before modifying (copies to `settings.json.beekeeper-backup-<timestamp>`).
- `beekeeper hooks install --target <target> --dry-run`: prints what would be written without modifying any file.
- New `beekeeper hooks uninstall --target <target>` to remove installed hooks.

### INTG-07: Multi-Agent Context Propagation

- New `AgentContext` struct in `internal/policy/types.go` (pure, no I/O):
  ```go
  type AgentContext struct {
      AgentID       string   // current agent session ID
      ParentAgentID string   // parent agent session ID (empty if root)
      Depth         int      // nesting depth (0 = root)
      Lineage       []string // ordered parent IDs from root to parent
  }
  ```
- Tool call JSON extended: Beekeeper reads `BEEKEEPER_AGENT_ID`, `BEEKEEPER_PARENT_AGENT_ID`, `BEEKEEPER_AGENT_DEPTH` from environment when evaluating `beekeeper check`. Claude Code hooks automatically carry env from the hook invocation context.
- `internal/policy` `Evaluate` function receives `AgentContext` — pure, no I/O change.
- `audit.AuditRecord` extended with `agent_id`, `parent_agent_id`, `agent_depth`, `agent_lineage` fields.
- Subagent blocks escalate to parent context: if a subagent call is blocked, the audit record carries the full lineage so forensics can trace the parent chain.
- Policy: subagent calls cannot exceed the permission level of the parent (no child escapes parent policy level). Enforced via `Depth` + `Lineage` check in the policy engine.

### INTG-03 + INTG-04: MCP Gateway Daemon (`beekeeper gateway`)

**Spec: MCP July 2026 — stateless per-request proxy**
- No session state. No `initialize`/`initialized` handshake required by Beekeeper's proxy layer — pass through as-is to upstream.
- JSON-RPC 2.0 over HTTP (not SSE/WebSocket for this phase — the gateway accepts HTTP POST requests from MCP clients).
- Request/response correlation: always by `id` field of the JSON-RPC object. Never by position. Batch requests: process each element independently.

**Binding and auth:**
- Default bind: `127.0.0.1:<port>` (localhost-only). Exposing to `0.0.0.0` requires `--bind` flag AND an explicit `"allow_remote_gateway": true` in config (acknowledged opt-out of security).
- Per-session token: generated with `crypto/rand` at gateway startup; stored in `~/.beekeeper/state.json` as `gateway_token` alongside the bound port. Clients must supply `Authorization: Bearer <token>` on every request. Other local processes cannot talk to the gateway without the token. Token rotates on each gateway restart.
- Upstream MCP server URL: configured via `--upstream` flag or config; required — gateway refuses to start without it.

**Policy application:**
- Every inbound JSON-RPC call whose `method` starts with `tools/call` is routed through the full policy engine (same `check.RunCheck` path as `beekeeper check`).
- Policy evaluation is synchronous, inline with the request — not async. Target latency: sub-100ms p95 (same as hook handler).
- On allow: forward request to upstream, return upstream response.
- On warn: forward request to upstream; inject a `_beekeeper_warning` field into the JSON-RPC response.
- On block: do NOT forward. Return a structured JSON-RPC error to the client (error code -32001, structured `data` field with `decision`, `reason`, `rule_ids`).
- Other methods (non-tool-call): proxy transparently without policy evaluation.
- Fail-closed: gateway crash or policy engine panic → return JSON-RPC error -32002 (internal error, with beekeeper-specific context) to client; never silently forward.

**Protocol parsing hardening (self-defense):**
- Bounded message size: 1MB hard cap on inbound request body (same as hook handler stdin cap).
- Bounded recursion in tool parameter schemas: max depth 10.
- Bounded array sizes in batch requests: max 50 items per batch.
- Malformed JSON → return JSON-RPC parse error (-32700) immediately; never partially evaluate.
- Reject unknown method names exceeding 256 bytes.

**Architecture:**
- `internal/gateway/` package:
  - `gateway.go`: HTTP server setup, port binding, token generation/verification
  - `proxy.go`: per-request JSON-RPC parsing, method routing, upstream forwarding, response injection
  - `parser.go`: bounded JSON-RPC message parser (the thing that gets fuzz-tested)
  - `policy.go`: policy engine integration (delegates to `check.RunCheck` or equivalent)
- Upstream forwarding: standard `net/http` client with configurable timeout (default 30s).
- Daemon: `beekeeper gateway [--port N] [--upstream URL] [--bind addr]`; runs foreground with `signal.NotifyContext(SIGINT, SIGTERM)` (same pattern as catalogs watch).

### INTG-05: Continue / OpenCode / OpenClaw via Gateway

- No new code for the clients themselves — the gateway already handles any MCP client.
- `beekeeper hooks install --target continue` (and opencode, openclaw): prints a formatted guide showing the user how to set their MCP client's upstream URL to `http://127.0.0.1:<port>` and include the auth token.
- Token retrieval: `beekeeper gateway token` prints the current gateway token (reads from state.json).
- `beekeeper gateway status`: prints whether the gateway process is running (checks state.json pid), the bound address, and the current token (masked except last 8 chars).

### INTG-06: Shim Layer (`beekeeper shim install/uninstall`)

- New subcommand `beekeeper shim {install|uninstall}` in `cmd/beekeeper/main.go`; logic in `internal/shim/`.
- Managed shim directory: `~/.beekeeper/shims/` (or `%APPDATA%\beekeeper\shims\` on Windows).
- `shim install`: creates wrapper scripts for npm, pnpm, pip, cargo, go, gem, composer, npx, pipx.
  - **Unix (shell scripts)**:
    ```sh
    #!/bin/sh
    # beekeeper shim for npm
    beekeeper check <<EOF
    {"tool_name":"Bash","tool_input":{"command":"npm $*"}}
    EOF
    exit_code=$?
    if [ $exit_code -eq 0 ]; then
        exec "$(which -a npm | grep -v beekeeper | head -1)" "$@"
    fi
    exit $exit_code
    ```
    Written to `~/.beekeeper/shims/npm`, `chmod +x`.
  - **Windows (`.cmd` batch files)**:
    ```cmd
    @echo off
    echo {"tool_name":"Bash","tool_input":{"command":"npm %*"}} | beekeeper check
    if %ERRORLEVEL% EQU 0 goto :run
    exit /b %ERRORLEVEL%
    :run
    <real-npm-path> %*
    ```
    Written to `~/.beekeeper/shims/npm.cmd`.
  - Detects real binary path via `exec.LookPath` excluding the shim directory; stores in the shim script.
  - If a tool is not installed, its shim is skipped (not an error).
- `shim install` also prints shell RC instructions: "Add `~/.beekeeper/shims` to the beginning of your PATH" with shell-specific snippets for bash, zsh, fish, PowerShell.
- `shim uninstall`: removes all files in `~/.beekeeper/shims/`.
- Shims are idempotent: reinstalling overwrites existing shim files with the current real binary path.
- `beekeeper shim status`: lists which tools are shimmed and their real binary paths.

### Architecture: Where New Code Lives

| Component | Package |
|-----------|---------|
| Hook installer logic | `internal/hooks/` |
| Multi-agent context types | `internal/policy/types.go` (extend) |
| Audit record lineage fields | `internal/audit/types.go` (extend) |
| MCP gateway daemon | `internal/gateway/` |
| Shim creator/remover | `internal/shim/` |
| `beekeeper hooks` subcommands | `cmd/beekeeper/main.go` |
| `beekeeper gateway` subcommands | `cmd/beekeeper/main.go` |
| `beekeeper shim` subcommands | `cmd/beekeeper/main.go` |

### Claude's Discretion

**Resolved decisions (planning complete 2026-05-26):**

- **Port selection:** Default port 7837 with `--port` override and fallback to random port (`net.Listen("tcp", "127.0.0.1:0")`) when 7837 is busy. Actual bound port stored in state.json so `beekeeper gateway token` and `gateway status` always reference the correct address.
- **State.json schema:** Separate `GatewayState` struct stored under a `"gateway"` key in the top-level state.json, alongside the existing `"sources"` key from `catalog.WatchState`. Avoids a separate file while keeping schemas independent. Load/Save pattern mirrors `catalog.LoadState`/`catalog.SaveState`.
- **Windows shim format:** `.cmd` batch files (not `.ps1`) for broadest compatibility with cmd.exe without execution policy restrictions. CRLF line endings required.
- **Cursor hooks file:** `~/.cursor/hooks.json` (verified from cursor.com/docs/hooks). NOT `~/.cursor/settings.json`. Schema uses `preToolUse` (camelCase), `version: 1`, `failClosed: true` required.
- **Codex CLI hooks file:** `~/.codex/hooks.json` (verified from developers.openai.com/codex/hooks). Nested schema similar to Claude Code but separate file. Trust confirmation required on first Codex run after install.
- **`beekeeper audit-record` command:** Implemented in `internal/check/handler.go` as `RunAuditRecord(stdin io.Reader, auditPath string) int`. Reads PostToolUse JSON from stdin; writes `tool_result` audit record; returns 0 always. Registered as a Cobra subcommand in Plan 05 (cmd/beekeeper/main.go).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Architecture and Constraints
- `CLAUDE.md` — Architecture constraints (Go 1.25+, single binary, `internal/` structure, `internal/policy` pure, fail-closed, MCP gateway stateless per-request, correlate by `id` not position)
- `.planning/PROJECT.md` — Core decisions, locked choices
- `.planning/REQUIREMENTS.md` — INTG-01 through INTG-07 (verbatim requirement text)
- `.planning/ROADMAP.md` — Phase 4 success criteria, dependency on Phase 2

### Prior Phase Patterns (reuse directly)
- `.planning/phases/02-policy-engine-multi-source-catalogs/02-PATTERNS.md` — Pattern map for Phase 2 code
- `.planning/phases/03-editor-extension-defense/03-CONTEXT.md` — Phase 3 decisions (JSONC settings patch, editorinit patterns)
- `cmd/beekeeper/main.go` — Existing Cobra wiring patterns (catalogs watch daemon = canonical signal.NotifyContext foreground daemon)
- `internal/check/handler.go` — Policy engine invocation pattern (canonical consumer)
- `internal/config/config.go` — Config struct shape and extend-don't-replace pattern

### Key Source Files to Extend or Follow
- `internal/policy/types.go` — Add AgentContext and extend Decision with lineage fields
- `internal/audit/types.go` — Add agent_id, parent_agent_id, agent_depth, agent_lineage fields
- `internal/editorinit/settings.go` — JSONC patch pattern (reuse for Claude Code settings.json hook injection)
- `cmd/beekeeper/main.go` — Add hooks, gateway, shim subcommand trees

</canonical_refs>

<specifics>
## Specific Ideas

### MCP July 2026 Spec Notes (from PRD + CLAUDE.md)
- Stateless per-request proxy: no session accumulation. The gateway is a pure function of (request, policy engine state, upstream URL).
- `id` field correlation: JSON-RPC 2.0 requires responses match requests by `id`. Never assume position-based ordering. This is especially important for batch requests where upstream may reorder responses.
- The gateway does NOT need to implement the full MCP spec server; it is a transparent proxy that intercepts at the tool-call layer.

### Claude Code Settings.json Hook Format (PRD §6.1)
```json
{
  "hooks": {
    "PreToolUse": [
      {"matcher": ".*", "hooks": [{"type": "command", "command": "beekeeper check"}]}
    ],
    "PostToolUse": [
      {"matcher": ".*", "hooks": [{"type": "command", "command": "beekeeper audit-record"}]}
    ]
  }
}
```
The file is JSONC (may have comments). Use the existing go-jsonc approach from Phase 3 editorinit.

### Self-Defense Deliverables for v0.6.0 (per PRD §15)
- MCP gateway localhost-only binding by default; explicit acknowledgment required to expose remotely.
- MCP gateway per-session token authentication for all clients.
- Strict MCP protocol parsing: bounded message sizes and recursion limits.
- Fuzz testing extended to MCP message parser (and Sentry rule evaluator — Sentry is Phase 5, fuzz the parser now).
- Audit log sensitive field redaction patterns shipped with sensible defaults.
Note: fuzz tests for MCP message parser are a **release gate** (block v0.6.0 release if fuzz coverage is absent).

### `beekeeper audit-record` Command
The PostToolUse hook writes `beekeeper audit-record` as the command. This command should:
- Read PostToolUse JSON from stdin (tool name, result, exit code).
- Write an NDJSON audit record of type `tool_result` with the outcome.
- Exit 0 always (PostToolUse hooks that fail may disrupt the agent).
This is a simple command, can be implemented as part of the CLI wiring plan.

### Fail-Closed Gateway Semantics
The gateway must never forward a request if the policy engine panics, times out (>500ms hard cap), or returns an error. The fail-closed posture applies to the gateway the same way it applies to `beekeeper check`. The `fail_open` config flag applies here too.

</specifics>

<deferred>
## Deferred Ideas

- ContextForge / MCPGuard policy-plugin mode — v1.5 deliverable per PRD §16
- macOS EndpointSecurity entitlement path — v2 deliverable
- TypeScript/Bun hook scaffold libraries distributed via npm — v1.5 deliverable
- Gateway over SSE/WebSocket — current spec is HTTP POST; future MCP spec may add streaming
- Remote gateway exposure (bind 0.0.0.0) — deferred to "explicit opt-in" path
- `beekeeper protect install` and Sentry daemon — Phase 5
- Codex CLI hooks: if the CLI hook API is not yet publicly documented, emit a TODO comment and implement the file writer to an assumed path (empirically validated in research)

</deferred>

---

*Phase: 04-integration-surfaces*
*Context gathered: 2026-05-26 via PRD Express Path (beekeeper-prd.md §6 + REQUIREMENTS.md INTG-01–07)*
*Discretion resolved: 2026-05-26 during planning (port 7837, separate gateway state key, .cmd shims, ~/.cursor/hooks.json, ~/.codex/hooks.json, RunAuditRecord)*
