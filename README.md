<p align="center">
  <img src="assets/readme-banner.gif" alt="Beekeeper: a fail-closed safety harness for autonomous coding agents" width="100%">
</p>

# Beekeeper

**Real-time safety harness for autonomous coding agents.**

Beekeeper intercepts agent tool calls before they execute and evaluates them
against unified threat intelligence. It is a single static Go binary with no
external runtime dependencies.

> **Status:** v1.3.0-seed (Phase 10 — Hook-Block Protocol Compliance &
> Multi-Harness Enforcement). Local-only; not yet pushed to GitHub.

---

## What it does

- **PreToolUse hook:** Runs `beekeeper check --hook <harness>` before each tool
  call. On block: exits 2 + emits harness-specific deny JSON so the harness
  refuses to run the tool.
- **MCP gateway:** Intercepts in-flight MCP tool calls via a local proxy
  (`http://127.0.0.1:7837/mcp`). Gateway-only path for harnesses without a
  pre-exec hook.
- **Catalog-based threat intelligence:** Corroboration-based scoring — 1 source
  = warn, 2 = block, 3 = block + quarantine.
- **Audit log:** NDJSON audit trail of every decision (allow/warn/block).
- **Package-manager nudge:** Detects `npm install`, `pip install`, etc. and
  checks packages against the threat catalog before they run.
- **Local adjudicated corpus (v1.4.0):** Confirmed incidents are recorded to a
  local, append-only, owner-only corpus. Off the hot path (only during
  `catalogs sync`), an adjudication engine assigns the outcome; a
  confirmed-malicious package is written to a local catalog overlay so that
  machine enforces it immediately, ahead of the upstream feed. Local-only:
  nothing leaves the machine (see
  [docs/THREAT-MODEL.md §13](docs/THREAT-MODEL.md)).

---

## Agent harness support

Beekeeper supports 17 agent harnesses across three tiers. See
[docs/harness-support-matrix.md](docs/harness-support-matrix.md) for the full
table with config locations, deny mechanisms, caveats, and verification status.

### Summary

| Tier | Harnesses | Coverage |
|------|-----------|---------|
| **Tier 1 — full hook-block** | Claude Code, Codex, Cursor, Augment, CodeBuddy, Qwen Code, Gemini CLI, Copilot, Antigravity, Windsurf | Pre-exec hook: exit 2 + per-harness deny JSON. All tool calls intercepted. |
| **Tier 2 — hook-block with caveats** | Hermes, Cline, OpenCode | Hook available but with known limitations (see below). |
| **Tier 3 — MCP gateway only** | Kilo, Trae, Continue, OpenClaw | MCP tools intercepted via gateway. **Native Bash/file tools UNGUARDED.** |

### Tier 2 caveats

- **Hermes**: fail-OPEN harness — exit codes are ignored. Block requires emitting
  `{"action":"block","message":"..."}` to stdout. Any hook timeout/crash allows
  the tool.
- **Cline**: **macOS/Linux ONLY** — no Windows support. The hook is an executable
  file; Windows cannot run Unix executable scripts in this way.
- **OpenCode**: JS plugin (`tool.execute.before`). Does not catch subagent `task`
  calls (#5894) or historically MCP calls (#2319).

### Tier 3 (Kilo, Trae, Continue, OpenClaw) — native tools unguarded

These four harnesses have no upstream pre-exec hook mechanism. Beekeeper can only
intercept MCP tools by routing them through the gateway. **Native built-in
tools (Bash, file read/write, shell commands) are UNGUARDED.** For Kilo and Trae
this is an upstream limitation (no hook); Continue and OpenClaw are wired through
their MCP client config (`~/.continue/config.yaml`, `~/.openclaw/config.json`),
so `beekeeper hooks install` prints the config rather than writing a hook file.
For full pre-exec coverage, use a Tier-1 harness.

See `beekeeper hooks install --target kilo` (or `trae` / `continue` / `openclaw`)
for gateway configuration instructions.

### Verification scope

**Only Claude Code is locally live-verified.** The other 16 harnesses are
implemented against their published documentation and validated by
contract-shape unit tests — these tests verify Beekeeper emits the correct
exit code and JSON, but do NOT run a real harness. Whether a harness actually
honors the hook contract is manual + Claude-Code-only.

The live-block procedure for each of the 16 non-Claude-Code harnesses is
enumerated, with sign-off fields, in
[docs/validation-register.md](docs/validation-register.md); the validation
posture (Tier A/B/C model + the coverage allowlist) is documented in
[docs/validation-posture.md](docs/validation-posture.md) so the coverage claim is
auditable.

Full honesty notes: [docs/harness-support-matrix.md#honesty-notes](docs/harness-support-matrix.md#honesty-notes)

---

## Quick start

```sh
# Build
go build ./cmd/beekeeper

# Sync threat catalog
beekeeper catalogs sync

# Install hook for Claude Code
beekeeper hooks install --target claude-code

# Check a tool call manually (stdin: tool call JSON)
echo '{"tool_name":"Bash","tool_input":{"command":"cat ~/.ssh/id_rsa"}}' \
  | beekeeper check --hook claude-code

# Start MCP gateway (for Tier-3 harnesses or as a proxy)
beekeeper gateway start

# Get gateway auth token
beekeeper gateway token
```

---

## Architecture

- **Single static binary** — `cmd/beekeeper/main.go` is thin Cobra wiring.
  All business logic lives in `internal/`.
- **`internal/policy`** — pure function library (no I/O, no goroutines). Called
  synchronously from hook handler, gateway middleware, and Sentry correlation.
- **`internal/corpus`** — local adjudicated-corpus loop. Append-only,
  owner-only, redaction-first, with no network imports (enforced by
  `TestCorpusStoreHasNoNetworkImports`). Adjudication runs off the hot path.
- **Fail-closed by default** — any crash, timeout, or unavailability in
  `beekeeper check` or the gateway results in block, not allow.
- **Hook handler (`beekeeper check`)** loads catalog via mmap. No cold-load
  per invocation.
- **MCP gateway** — stateless per-request proxy (MCP July 2026 spec).

See `CLAUDE.md` for detailed architecture constraints and key technical
decisions.

---

## Security

See [SECURITY.md](SECURITY.md) for the vulnerability disclosure policy.

---

## License

Beekeeper is licensed under the Apache License 2.0. See [LICENSE](LICENSE).

Beekeeper's threat intelligence is inspired by Perplexity's
[Bumblebee](https://github.com/perplexityai/bumblebee) (Apache-2.0). Bumblebee is
consumed as a runtime catalog source and, for the optional `beekeeper scan`
inventory, as an external scanner binary (preferred on macOS/Linux), with
[Pollen](https://github.com/home-beekeeper/pollen) as the Windows-support fallback.
Beekeeper bundles no Bumblebee or Pollen code; both are invoked as subprocesses.
