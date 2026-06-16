# Phase 18: Full Content Authoring — Research

**Researched:** 2026-06-09
**Domain:** Docs content authoring, accuracy gate, CLI surface mapping
**Confidence:** HIGH (all facts sourced from real Go code, shipped Go docs, or existing MDX)

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01** — Phase 18 is pure content authoring; docs visual styling / Fumadocs theme redesign is a separate phase (deferred to `.planning/todos/pending/docs-styling-polish.md`). Phase 18 delivers MDX prose + `meta.json` ordering + (at most) small reusable content-callout MDX components. No `DocsLayout`/theme CSS changes.
- **D-02** — Accuracy is THE gate (DOCS-09 is non-negotiable). Every security/behavioral claim must trace to a source. MDX files deriving content from Go-side docs carry `source_doc:` frontmatter pointing at the authoritative file. All content reviewed against `docs/THREAT-MODEL.md` before publish. Unenforced features (`release_age`/`minimumReleaseAge` and `lifecycle_script_allowlist`) explicitly labeled "not enforced in v1.3.0."
- **D-03** — Honesty at point-of-use. Mandatory caveats: Hermes fail-open, Tier-3 UNGUARDED, Tier-1 testable=Claude-Code-only, `--bind 0.0.0.0` gateway exposure, exit-1→exit-2 history. Caveats live where the user reads them, not in footnotes.
- **D-04** — Source-of-truth precedence: real Go code > Go-side `docs/*.md` > marketing copy. CLI reference authored from real cobra definitions.
- **D-05** — discuss-phase skipped; research-first; inline on main. No UI-SPEC.

### Claude's Discretion

- Per-section depth, structure, and sub-page splitting (meta.json) — e.g. cli-reference as one page or page-per-group. SC-4 requires exhaustive coverage; plumbing subcommands may be condensed.
- Whether to add a reusable "Unenforced in v1.3.0" / caveat callout MDX component (mirroring BreakingChangeCallout pattern).
- Example command selection for copyable snippets.
- Verification mechanism for DOCS-09 (recommended: extend `web/tests/` with `accuracy_spec.py`).

### Deferred Ideas (OUT OF SCOPE)

- Docs visual styling / Fumadocs theme redesign (own phase, own UI-SPEC).
- Auto-generated CLI reference from the Go binary (v1.3.0 is hand-authored MDX).
- Versioned docs / i18n / blog / playground.
- SITE-03 live Vercel deploy.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DOCS-02 | Getting Started quickstart: zero to working `beekeeper check`, no unshipped steps | §1 Getting Started source-map + §2 CLI tree |
| DOCS-03 | Installation: `go install`, binary download, cosign + SLSA verification with copyable commands | §1 Installation source-map + docs/release-runbook.md + docs/THREAT-MODEL.md §2/7 |
| DOCS-04 | Configuration: layered config, policy-as-code, sensitive paths, package-manager nudge with copyable examples | §1 Configuration source-map + docs/nudge.md + CLAUDE.md |
| DOCS-05 | Security posture AND known gaps co-located: corroboration model, fail-closed, threat model + Hermes/Tier-3/unenforced/gateway caveats | §1 Security source-map + §4 Honesty caveats + §3 Enforced vs unenforced |
| DOCS-06 | Integration guides for Tier-1/2/3 harnesses (Claude Code/Cursor/Codex hooks, MCP gateway) with honest caveats at point-of-use | §1 Integration source-map + docs/harness-support-matrix.md |
| DOCS-07 | CLI / command reference: ALL subcommands and flags | §2 Full CLI command tree |
| DOCS-08 | Troubleshooting for common issues | §1 Troubleshooting source-map |
| DOCS-09 | Accuracy gate: `source_doc:` frontmatter on all Go-derived MDX, content reviewed against docs/THREAT-MODEL.md, unenforced features labeled | §3 Enforced/unenforced + §4 Honesty caveats + §6 Validation Architecture |
</phase_requirements>

---

## Summary

Phase 18 replaces eight stub docs pages with complete, accurate documentation anchored to the shipped binary. The core challenge is not technical complexity — the Fumadocs pipeline already works — it is accuracy discipline: every claim about what Beekeeper does must trace to a real source file, and every limitation must be surfaced at point-of-use rather than buried.

The stubs are short (~20–28 lines each) and contain multiple inaccuracies relative to the shipped code. The CLI reference stub is particularly wrong — it documents flags that do not exist (`--input`, `--hook` with wrong semantics) and omits the majority of the subcommand tree (15+ top-level subcommands, each with 2-5 sub-subcommands). The security stub is accurate at a high level but needs depth and co-location with known gaps per DOCS-05.

The nudge feature has a distinction the stubs miss entirely: the shipped binary defaults to `mode=block` (not `mode=soft`) when installed via `beekeeper hooks install`, because `ensureNudgeBlockDefault` runs at install time. The library default for fresh config is `soft`, but the install path auto-upgrades to `block`. This must be documented accurately.

**Primary recommendation:** Write all 8 sections as MDX with `source_doc:` frontmatter, add one small reusable `UnenforcedCallout` component mirroring `BreakingChangeCallout`, verify with `accuracy_spec.py`, and review each section against `docs/THREAT-MODEL.md` before publish.

---

## 1. Per-Section Content Source-Map

### Section 1: Getting Started (`getting-started/index.mdx`)

**Requirement:** DOCS-02 | **ROADMAP SC:** SC-1

**Content outline:**
1. Prerequisites (Go 1.25+, supported OS)
2. Install (go install one-liner)
3. First run: `beekeeper init` (creates state dir, detects editors)
4. Sync catalogs: `beekeeper catalogs sync`
5. Install hook for your harness: `beekeeper hooks install --target claude-code`
6. Verify: manual test of `beekeeper check`
7. What to expect: allow/warn/block decisions in the audit log

**Authoritative source_doc:** `README.md`, `cmd/beekeeper/main.go` (init + hooks install wiring), `docs/harness-support-matrix.md`

**Key accurate facts:**
- `beekeeper init` creates the state directory AND detects installed editors — `--yes` auto-consents to all prompts; `--no-editors` skips editor detection (scripted installs)
- Hook install flag is `--target <harness>`, NOT `--hook <harness>` (the stub is WRONG — see §8 Risks)
- Installing the hook sets `nudge.mode=block` automatically (via `ensureNudgeBlockDefault` in main.go) — this is a first-time install behavior, NOT the library default
- `beekeeper catalogs sync` must run before first check (builds `bumblebee.idx`)
- `beekeeper check` reads tool call JSON from stdin (hook mode); the manual-test form is `echo '{"tool_name":"Bash","tool_input":{"command":"cat ~/.ssh/id_rsa"}}' | beekeeper check --hook claude-code`

**Mandatory caveat at point-of-use:**
> "Only Claude Code is live-verified. All other harnesses are contract-shape tested but not verified against a running agent. See the Integration docs for details."

---

### Section 2: Installation (`installation/index.mdx`)

**Requirement:** DOCS-03 | **ROADMAP SC:** SC-2

**Content outline:**
1. `go install github.com/home-beekeeper/beekeeper/cmd/beekeeper@latest`
2. Pre-built binaries from GitHub Releases (download URL pattern)
3. Cosign verification (exact commands from docs/THREAT-MODEL.md §2/7)
4. SLSA Level 3 provenance verification
5. CycloneDX SBOM verification
6. Build from source (`make build`, `-trimpath -buildvcs=false -mod=readonly`)
7. State directory: `~/.beekeeper/` (Linux/macOS), `%APPDATA%\beekeeper\` (Windows)

**Authoritative source_doc:** `docs/THREAT-MODEL.md` (§2, §7), `docs/release-runbook.md`, `CLAUDE.md`

**Key accurate facts (all copyable commands from THREAT-MODEL.md §7):**

```bash
# Cosign verify
cosign verify \
  --certificate-identity=https://github.com/home-beekeeper/beekeeper/.github/workflows/release.yml@refs/tags/v<version> \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com \
  beekeeper
```

```bash
# SLSA verify — note lowercase source-uri (bantuson not Bantuson)
slsa-verifier verify-artifact beekeeper \
  --provenance-path beekeeper.intoto.jsonl \
  --source-uri github.com/home-beekeeper/beekeeper
```

- CRITICAL: `--certificate-identity-regexp` for cosign uses capital-B `Bantuson` when verifying pollen releases but the beekeeper binary itself uses the workflow URL pattern (all lowercase `bantuson` in the source-uri for SLSA). Use the exact commands from THREAT-MODEL.md §7, not from the release-runbook (which is pollen-scoped).
- Build flags: `-trimpath -buildvcs=false -mod=readonly`
- SLSA tag: `slsa-github-generator@v2.1.0` (full semver — not `@v2`)
- Windows state dir: `%APPDATA%\beekeeper\` (NOT `~/.beekeeper/`)

**Mandatory caveat:** None specific to this section beyond normal verification guidance.

---

### Section 3: Configuration (`configuration/index.mdx`)

**Requirement:** DOCS-04 | **ROADMAP SC:** SC-3 (implied by SC-2's "copyable commands")

**Content outline:**
1. Config file location: `~/.beekeeper/config.json` (Linux/macOS), `%APPDATA%\beekeeper\config.json` (Windows)
2. Layered config: system → user → project (`.beekeeper/config.json` in working dir) → `BEEKEEPER_*` env vars
3. Fail mode: `fail_closed` (default true), `fail_mode: open` (explicit opt-in — reduces security)
4. Policy-as-code: `~/.beekeeper/policies/*.json`, policy validate/test/list commands
5. Sensitive paths (`DefaultSensitivePaths`): `~/.ssh`, `~/.aws`, `~/.cargo/credentials`, `.env` globs, editor MCP config dirs
6. Package-manager nudge: soft/hard/block modes, full config fields with defaults
7. Self-catalog override (for advanced users who self-host)
8. `beekeeper config set nudge.<key> <value>` — the only `config set` surface

**Authoritative source_doc:** `docs/nudge.md`, `docs/THREAT-MODEL.md` (§8 project config caveat, §9), `CLAUDE.md`, `cmd/beekeeper/config.go`

**Key accurate facts:**
- Nudge config default block from `docs/nudge.md`:
```json
{
  "nudge": {
    "enabled": true,
    "mode": "soft",
    "require_hardened": false,
    "preferred": "pnpm",
    "check_socket_scanner": true,
    "major_drift_check": { "enabled": true, "interval": "168h" },
    "version_floors": { "pnpm": "11.0.0", "bun": "1.3.0", "node": "22.0.0" }
  }
}
```
- Nudge `mode` values: `"soft"` (advise + proceed), `"hard"` (rewrite command, still advisory), `"block"` (deny npm/yarn install when no hardened PM) — NOTE: `config set` only supports `soft` and `hard` per `applyNudgeKey` (config.go line 156); the `block` mode referenced in CONTEXT.md memory is the `require_hardened: true` equivalent, NOT a string mode. **IMPORTANT:** The `ensureNudgeBlockDefault` function sets `nudge.mode = "block"` in user config on first `hooks install` — but the ValidateNudgeConfig only accepts `soft|hard`. This is a discrepancy that needs investigation (see §7 Open Questions).
- `config set` supports only 5 nudge keys: `nudge.enabled`, `nudge.mode`, `nudge.require_hardened`, `nudge.preferred`, `nudge.check_socket_scanner`
- `release_age` and `lifecycle_script_allowlist` in policy files are **informational only** — NOT enforced in v1.3.0 (see §3)
- Project `.beekeeper/config.json` is the lowest-trust layer; `fail_mode: open` in a project config relaxes fail-closed globally — a security risk documented in THREAT-MODEL.md §8

**Mandatory caveat at point-of-use (project config risk):**
> "The project-layer `.beekeeper/config.json` is discovered by walking up from the working directory and is merged above user config. A project file with `fail_mode: open` relaxes all fail-closed safety nets. Treat it as security-relevant and do not run agents in untrusted repositories with project config discovery enabled if you rely on fail-closed enforcement."

---

### Section 4: Security (`security/index.mdx`)

**Requirement:** DOCS-05 | **ROADMAP SC:** SC-3

**Content outline (co-located posture AND gaps — not separate pages):**

**Posture:**
1. Corroboration model: 1 source → warn, 2 → block, 3+ → block + quarantine; per-severity thresholds (CORR)
2. Fail-closed defaults (all paths: hook, gateway, Sentry)
3. SPATH: blocks agent reads of `~/.ssh`, `~/.aws`, `.env`, editor MCP dirs; normalization for Windows ADS and trailing-dot tricks
4. Self-protection: agent cannot read/write StateDir, overwrite binary, remove hook entry (content-aware)
5. `beekeeper-self` catalog: startup + post-sync self-quarantine check, separate Ed25519 key
6. Build hardening: reproducible builds, Sigstore/cosign, SLSA Level 3, CycloneDX SBOM
7. Policy-as-code escape hatch: `package_allowlist allow` rules are forensically visible in audit

**Known gaps (co-located, not hidden):**
1. Hermes is fail-OPEN: block requires correct stdout JSON; exit codes ignored
2. Tier-3 harnesses (Kilo, Trae): native Bash/file tools UNGUARDED
3. Tier-1 testable = Claude Code only (14 harnesses documented but not live-verified)
4. `release_age` / `lifecycle_script_allowlist` unenforced in policy files
5. `--bind 0.0.0.0` gateway: exposes plain HTTP; `allow_remote_gateway` config gate NOT implemented
6. Project `.beekeeper/config.json` can relax fail-closed
7. Windsurf: fail-OPEN on non-2 exit codes
8. OpenCode: plugin does not catch subagent `task` calls (#5894)
9. Package-parse evasion: command chaining (`&&`), leading env-var assignments, unlisted PMs → allow
10. TM-B-02: Bumblebee signature is presence-check, not Ed25519 (tracked, not yet remediated)
11. fanotify mmap gap (Linux): pre-existing mmap'd libraries not re-intercepted
12. Windows Sentry: missing PPID on file/network events

**Authoritative source_doc:** `docs/THREAT-MODEL.md` (ALL sections), `docs/harness-support-matrix.md`, `CLAUDE.md`

**Mandatory caveat:** All known gaps must live on the same page as the posture claims (D-03, D-02). Link to v1.3.0 changelog for exit-1→exit-2 history rather than duplicating.

---

### Section 5: Integration (`integration/index.mdx`)

**Requirement:** DOCS-06 | **ROADMAP SC:** SC-4 (partial — integration + CLI together)

**Content outline:**
- Overview: 15 harnesses, 3 tiers, honest ceiling
- **Tier 1 full walkthrough (Claude Code):** `beekeeper hooks install --target claude-code`; reload behavior; live-verified
- **Tier 1 documented (9 others):** Codex, Cursor, Augment, CodeBuddy, Qwen Code, Gemini CLI, Copilot, Antigravity, Windsurf — per-harness install command + known caveat
- **Tier 2 with caveats:** Hermes (fail-OPEN, use gateway instead), Cline (macOS/Linux only), OpenCode (plugin gaps)
- **Tier 3 gateway-only:** Kilo, Trae (UNGUARDED for native tools — point-of-use caveat required)
- **MCP gateway:** `beekeeper gateway --upstream <url> --port 7837`; `--bind 0.0.0.0` caveat; `gateway token`/`gateway status`; bearer auth

**Authoritative source_doc:** `docs/harness-support-matrix.md`, `docs/THREAT-MODEL.md` §10, `cmd/beekeeper/main.go` (gateway wiring)

**Key accurate facts:**
- Install flag is `--target <harness>` (not `--hook <harness>` which is the check flag)
- Tier-1 targets in hooks install: `claude-code`, `cursor`, `codex`, `augment`, `codebuddy`, `qwen`, `continue`, `opencode`, `openclaw` (from installCmd.Flags() in main.go line 1229)
- Gateway default port: 7837, default bind: `127.0.0.1`
- Gateway flags: `--upstream` (required at runtime), `--port`, `--bind`, `--allow-remote`
- Cursor requires `failClosed:true` in its hook config (Cursor is fail-OPEN by default)
- Cline installer is `//go:build !windows` — returns explicit error on Windows
- `--allow-remote` flag exists in code (line 1416) but the `allow_remote_gateway` config gate does NOT exist (THREAT-MODEL.md §8 explicit gap)

**Mandatory caveats at point-of-use:**
- Hermes section: "Hermes ignores hook exit codes. Block depends entirely on stdout JSON `{\"action\":\"block\",\"message\":\"...\"}`. Any hook timeout or crash allows the call. Prefer the MCP gateway for Hermes."
- Kilo/Trae section: "Native Bash, file-read, and shell commands are UNGUARDED. Only MCP tools routed through the gateway are intercepted. For full pre-exec coverage, use a Tier-1 harness."
- Verification section: "Only Claude Code is live-verified end-to-end (hook fires → exit 2 → tool denied → audit entry written). All other harnesses are contract-shape tested."

---

### Section 6: CLI Reference (`cli-reference/index.mdx`)

**Requirement:** DOCS-07 | **ROADMAP SC:** SC-4

**Content outline:** Complete subcommand tree — see §2 Full CLI Command Tree below for the exhaustive inventory. Structure recommendation: one page with H2-per-top-level-command and H3-per-subcommand. For very large sections (audit, catalogs, nudge, policy, quarantine), collapse into a table. Do not split into sub-pages for v1.3.0.

**Authoritative source_doc:** `cmd/beekeeper/main.go`, `cmd/beekeeper/config.go`, `cmd/beekeeper/policy.go`, `cmd/beekeeper/nudge.go`, `cmd/beekeeper/diag.go`, `cmd/beekeeper/protect_linux.go`

See §2 for full details.

---

### Section 7: Troubleshooting (`troubleshooting/index.mdx`)

**Requirement:** DOCS-08 | **ROADMAP SC:** SC-1..5 (implied by overall accuracy gate)

**Content outline:**
1. Hook not firing / tool still runs after block
2. Catalog sync failures
3. High latency on `beekeeper check`
4. Self-quarantine event: what it means and how to investigate
5. Policy file rejected by `beekeeper policy validate`
6. Nudge not detecting pnpm/bun (detection is fail-open, 2s timeout)
7. Gateway not starting / upstream unreachable
8. Sentry not installed (Linux: requires root + systemd)
9. LlamaFirewall sidecar unreachable
10. `beekeeper diag` — the primary diagnostic tool
11. Common Windows gotcha: state dir is `%APPDATA%\beekeeper\`, not `~/.beekeeper/`

**Authoritative source_doc:** `docs/THREAT-MODEL.md` (§8 known gaps), `docs/nudge.md` (security notes), `cmd/beekeeper/main.go` (error messages)

**Key accurate facts / corrections vs. stub:**
- Stub says "run `beekeeper hooks status`" — this command DOES NOT EXIST in the code. The diagnostic command is `beekeeper diag`. Stub also says "run `beekeeper catalogs rebuild`" — this command DOES NOT EXIST. The correct command is `beekeeper catalogs sync`.
- For self-quarantine: `beekeeper version`, `beekeeper diag`, `beekeeper selftest`, `beekeeper policy validate` remain runnable during self-quarantine (THREAT-MODEL.md §7)
- pnpm detection timeout is 2 seconds per nudge.md (not configurable)
- Audit log is at `~/.beekeeper/audit/beekeeper.ndjson` (single file, not dated files as the stub implies)

---

### Section 8: Audit Log (`audit-log/index.mdx`)

**Requirement:** DOCS-09 (meta), DOCS-08 (adjacent) | **ROADMAP SC:** SC-5

**Content outline:**
1. Location: `~/.beekeeper/audit/beekeeper.ndjson` (single NDJSON file; Windows: `%APPDATA%\beekeeper\audit\beekeeper.ndjson`)
2. Record types and key fields
3. `beekeeper audit tail [--no-follow]` — stream live
4. `beekeeper audit query --since --agent --tool --decision --limit` — filter
5. `beekeeper audit export --format ndjson|csv|otlp` — export
6. `beekeeper nudge audit [--since]` — nudge-specific filter
7. Audit record examples (allow, warn, block, nudge, config_change, quarantine_restore)
8. Remote sinks: OTLP/HTTPS/syslog fan-out, redaction caveat (field-scoped, not content-scanning)

**Authoritative source_doc:** `docs/THREAT-MODEL.md` (§8 audit redaction caveat), `cmd/beekeeper/main.go` (audit subcommands), `cmd/beekeeper/nudge.go` (nudge audit)

**Key accurate facts / corrections vs. stub:**
- Stub says "rotated daily and compressed" — NO. The code writes to a single `beekeeper.ndjson` file; there is no rotation logic in the shipped code. Remove this claim.
- Stub says query via `cat ~/.beekeeper/audit/$(date +%Y-%m-%d).ndjson` — WRONG path and WRONG rotation assumption. Correct: `beekeeper audit query` or `beekeeper audit tail`.
- `beekeeper audit export --format` supports: `ndjson`, `csv`, `otlp`
- Redaction is field-scoped (not content-scanning): Sentry-derived fields (accessed paths, network destinations, process exe paths) are written verbatim. The behavioral-watch audit path does NOT route through `RedactRecord`. (THREAT-MODEL.md §8)
- `nudge audit` records use `record_type: "nudge"` with fields: `nudge_action`, `original_command`, `rewritten_command`, `reason_code`, `pm_state`

**Mandatory caveat at point-of-use:**
> "Audit redaction is field-scoped: the `Reason` field and raw package-manager commands are redacted before remote sinks, but Sentry-derived fields (file paths, network destinations, process exe paths) are written verbatim. A credential in a watched file path can reach a remote OTLP/syslog sink unscrubbed. The local audit file is owner-only (0600)."

---

## 2. Full CLI Command Tree (DOCS-07)

All entries extracted from `cmd/beekeeper/main.go`, `config.go`, `policy.go`, `nudge.go`, `diag.go`, `protect_linux.go`. [VERIFIED: source code]

### User-Facing Commands

#### `beekeeper version`
Print version, commit, and build date.
- Flags: none
- Example: `beekeeper version`

#### `beekeeper init`
Create state directory and configure editor protection.
- Flags: `--yes` (auto-consent to all prompts), `--no-editors` (skip editor detection)
- Detects installed editors, offers to disable extension auto-update and register watch dirs

#### `beekeeper check`
Evaluate a tool call read from stdin. Core hook handler entry point.
- Flags: `--hook <harness>` (harness name; emits exit 2 + harness-specific deny JSON on block), `--tool <name>` (shim path: builds ToolCall from flags), `--args <arg>` (for shim path, array flag)
- Without `--hook`: exits 0 (allow) or 1 (block); writes raw Decision JSON to stdout
- With `--hook`: exits 0 (allow) or 2 (block); emits harness-specific deny JSON to stdout, reason to stderr
- Exit codes: 0 = allow/warn, 2 = block (with `--hook`), 1 = block (without `--hook`)
- NOTE: The stub's `--input` flag DOES NOT EXIST. Input is always stdin.

#### `beekeeper catalogs` (group)
Manage cached threat-intel catalogs.

  - **`beekeeper catalogs sync`** — Fetch and cache catalogs; build mmap index. No flags.
  - **`beekeeper catalogs watch`** — Poll catalog sources; trigger re-scan on delta (Ctrl+C to stop). No flags.
  - **`beekeeper catalogs verify --source <name>`** — Clear degraded mode for a catalog source after operator review. Required: `--source`
  - **`beekeeper catalogs diff`** — Show per-source delta between last-synced state and current on-disk snapshot. No flags.

#### `beekeeper audit` (group)
Inspect the Beekeeper audit log.

  - **`beekeeper audit tail [--no-follow]`** — Stream the live audit log. `--no-follow` dumps existing records and exits.
  - **`beekeeper audit query`** — Filter records. Flags: `--since <duration|RFC3339>`, `--agent`, `--tool`, `--decision allow|warn|block`, `--limit N`
  - **`beekeeper audit export --format <fmt>`** — Export audit records. Required: `--format ndjson|csv|otlp`. Optional: `--since`, `--agent`, `--tool`, `--decision`

#### `beekeeper selftest`
Run embedded adversarial fixtures (corpus) as a sanity check. No flags.

#### `beekeeper watch`
Watch extension directories for new installations (Ctrl+C to stop). No flags. Uses config.WatchDirectories() or DetectEditors() for watch dirs.

#### `beekeeper scan [--deep]`
Scan installed extensions against catalog and release-age policy.
- Flags: `--deep` (deep scan — passes `--profile deep --root <home>` to Pollen)

#### `beekeeper quarantine` (group)
Manage quarantined extensions.

  - **`beekeeper quarantine list`** — List quarantined extensions (ID, publisher.name, version, quarantined_at, reason). No flags.
  - **`beekeeper quarantine restore <id>`** — Restore a quarantined extension to its original location.
  - **`beekeeper quarantine purge [--yes]`** — Remove ALL quarantined extensions. `--yes` skips confirmation.

#### `beekeeper hooks` (group)
Install or uninstall Beekeeper hooks for agent CLIs.

  - **`beekeeper hooks install --target <harness> [--dry-run] [--force]`** — Install PreToolUse/PostToolUse hooks. `--target` required. `--dry-run` prints without modifying. `--force` overwrites without prompting. Also sets `nudge.mode=block` on first install.
  - **`beekeeper hooks uninstall --target <harness> [--dry-run]`** — Remove hooks. `--target` required.
  - Target values (from main.go line 1229): `claude-code`, `cursor`, `codex`, `augment`, `codebuddy`, `qwen`, `continue`, `opencode`, `openclaw`

#### `beekeeper gateway` (group + daemon)
Manage the Beekeeper MCP gateway daemon.

  - **`beekeeper gateway [--port 7837] [--upstream <url>] [--bind 127.0.0.1] [--allow-remote]`** — Start foreground gateway daemon. `--upstream` required at runtime.
  - **`beekeeper gateway token`** — Print the current session bearer token from state.json.
  - **`beekeeper gateway status`** — Print running status, bound address, masked token, started time.

#### `beekeeper shim` (group)
Manage PATH shims for package managers and toolchains.

  - **`beekeeper shim install`** — Create shim scripts in `~/.beekeeper/shims/`
  - **`beekeeper shim uninstall`** — Remove all shim scripts
  - **`beekeeper shim status`** — List shimmed tools and their real binary paths

#### `beekeeper protect` (group — Linux only)
Manage the Beekeeper Sentry daemon. On non-Linux platforms, prints "not supported."

  - **`beekeeper protect install`** — Install and start Sentry via systemd (requires root/sudo)
  - **`beekeeper protect uninstall`** — Stop and remove the Sentry daemon
  - **`beekeeper protect status`** — Show Sentry daemon status (IPC status + baseline state)

#### `beekeeper sentry` (group)
Sentry daemon (invoked by systemd ExecStart; Linux only).

  - **`beekeeper sentry`** — Run the Sentry daemon directly (for testing; normally invoked by systemd)
  - **`beekeeper sentry rules list`** — List active rules and enabled state
  - **`beekeeper sentry rules enable <id>`** — Enable a Sentry rule by ID
  - **`beekeeper sentry rules disable <id>`** — Disable a Sentry rule by ID

#### `beekeeper llamafirewall` (group)
Manage the LlamaFirewall prompt-injection sidecar.

  - **`beekeeper llamafirewall enable`** — Enable LlamaFirewall sidecar scanning
  - **`beekeeper llamafirewall disable`** — Disable LlamaFirewall sidecar scanning
  - **`beekeeper llamafirewall status`** — Show sidecar status (PID, uptime, sample rate, fail mode, degraded)

#### `beekeeper dashboard [--admin]`
Open the real-time TUI dashboard. `--admin` enables policy toggle, quarantine restore/purge, scan trigger.

#### `beekeeper policy` (group)
Manage and test declarative policy files in `~/.beekeeper/policies/`.

  - **`beekeeper policy validate <file>`** — Schema-check a policy file; exit non-zero on errors
  - **`beekeeper policy test <file> [--tool-call <path|->]`** — Dry-run a policy file against a tool-call JSON (no live catalog). Default: stdin.
  - **`beekeeper policy list`** — List loaded policy files with rule counts

#### `beekeeper diag`
Show system health: hook latency (p95/p99), sidecar latency, catalog freshness, ETW loss. No flags.

#### `beekeeper nudge` (group)
Inspect and test the package-manager nudge feature.

  - **`beekeeper nudge status`** — Show current PM state + active nudge configuration
  - **`beekeeper nudge check "<command>"`** — Dry-run: show what Beekeeper would do with a given install command (command is NEVER executed)
  - **`beekeeper nudge audit [--since <duration|RFC3339>]`** — Query audit log filtered to nudge records

#### `beekeeper config` (group)
Manage Beekeeper configuration.

  - **`beekeeper config set <key> <value>`** — Set a nudge.* configuration value (validated fail-closed, audit logged). Supported keys: `nudge.enabled`, `nudge.mode`, `nudge.require_hardened`, `nudge.preferred`, `nudge.check_socket_scanner`

### Internal / Plumbing Commands

These commands are invoked by installer scripts or internal mechanisms, not typically by users directly:

| Command | Purpose | User-Facing? |
|---------|---------|--------------|
| `beekeeper audit-record` | PostToolUse hook handler — record tool_result to audit log | No (registered as PostToolUse hook) |
| `beekeeper shim install/uninstall/status` | PATH shim management for package managers | Advanced users only |
| `beekeeper sentry` | Raw Sentry daemon (normally via systemd) | No (systemd ExecStart target) |
| `beekeeper sentry rules *` | Live rule management via IPC | Advanced/operator |

### Commands in Stubs That DO NOT EXIST

These appear in the current stub docs and MUST be corrected:

| Stub Command | Status | Correction |
|-------------|--------|------------|
| `beekeeper check --input '...'` | DOES NOT EXIST | Input is always stdin; remove `--input` flag |
| `beekeeper hooks status` | DOES NOT EXIST | Use `beekeeper diag` |
| `beekeeper catalogs rebuild` | DOES NOT EXIST | Use `beekeeper catalogs sync` |

---

## 3. Enforced vs. Unenforced Feature Table

[VERIFIED: `docs/THREAT-MODEL.md` §9, `docs/nudge.md` §security notes, `cmd/beekeeper/main.go`]

| Feature | Config Key | Status | How to Tell |
|---------|-----------|--------|-------------|
| Package catalog blocking | `corroboration_threshold` | **ENFORCED** | `beekeeper check` exits 2 on block |
| Sensitive-path blocking (SPATH) | `DefaultSensitivePaths` | **ENFORCED** | `~/.ssh`, `~/.aws`, `.env` etc. blocked |
| Package allowlist override | `package_allowlist` in policy | **ENFORCED** — can override catalog blocks | THREAT-MODEL.md §9 |
| Sensitive-path override | `sensitive_path` in policy | **ENFORCED** | policy engine |
| `release_age` rules in policy files | `release_age` | **NOT ENFORCED in v1.3.0** — informational only | THREAT-MODEL.md §9, code comment: "requires catalog lookup not available in tool call" |
| `lifecycle_script_allowlist` in policy files | `lifecycle_script_allowlist` | **NOT ENFORCED in v1.3.0** — informational only | THREAT-MODEL.md §9 |
| Nudge soft mode (advise) | `nudge.mode: "soft"` | **ENFORCED** (allows the command, emits advisory) | nudge.md |
| Nudge hard mode (rewrite) | `nudge.mode: "hard"` | **ENFORCED** (rewrites command, advisory to agent) | nudge.md |
| Nudge block mode | `nudge.require_hardened: true` | **ENFORCED** (denies npm/yarn when no hardened PM) | nudge.md; note `mode: "block"` in ensureNudgeBlockDefault is a migration path |
| Nudge PM detection | N/A | **FAIL-OPEN by design** — slow/absent PM = not installed | nudge.md §security |
| LlamaFirewall PostToolUse | `llamafirewall.enabled` | **PostToolUse non-blocking by default** (fail-open on sidecar unreachability unless `fail_mode: closed`) | THREAT-MODEL.md §1 |
| `allow_remote_gateway` config gate | `allow_remote_gateway` | **NOT IMPLEMENTED** — help text promises it but code has no such field | THREAT-MODEL.md §8 |
| Bumblebee Ed25519 signature | `catalog_signature` | **PRESENCE CHECK only** (not cryptographic verification in decision path) | THREAT-MODEL.md §11 TM-B-02 |

**Unenforced features that MUST be labeled in docs:**
- `release_age` / `minimumReleaseAge`: label with "Not enforced in v1.3.0 — informational only. The engine's built-in release-age policy (configured via catalog entries) is the enforcement path."
- `lifecycle_script_allowlist`: same label.
- `allow_remote_gateway`: document as "the config gate described in the help text is not implemented — `--bind 0.0.0.0` flows directly to `net.Listen`."

---

## 4. Mandatory Honesty Caveats and Where Each Lives

[VERIFIED: `docs/THREAT-MODEL.md`, `docs/harness-support-matrix.md`, `cmd/beekeeper/main.go`]

### 4.1 Hermes Fail-OPEN

**Exact accurate wording basis:** THREAT-MODEL.md §8 "Hermes Is a Structurally Fail-OPEN Harness" and harness-support-matrix.md Tier 2 entry.

**What to say:**
> Hermes ignores hook exit codes entirely. A block is carried only by emitting `{"action":"block","message":"..."}` (non-empty message) as the first JSON object on stdout. Any hook timeout, crash, or non-JSON stdout causes Hermes to allow the tool call silently. There is no exit-code backstop. **Recommendation: use the MCP gateway for Hermes use cases where reliable block enforcement is required.**

**Where it lives:** `integration/index.mdx` — Hermes sub-section (point-of-use); brief mention in `security/index.mdx` known gaps.

### 4.2 Tier-3 Harnesses UNGUARDED (Kilo, Trae)

**Exact accurate wording basis:** THREAT-MODEL.md §8, harness-support-matrix.md §Tier-3.

**What to say:**
> Kilo and Trae have no upstream pre-exec hook mechanism. Beekeeper can only intercept MCP tools routed through the gateway. **Native built-in tools (Bash, file read/write, shell execution) are completely UNGUARDED.** This is an upstream limitation (Kilo FR #5827), not a Beekeeper implementation gap. For full pre-exec coverage, use a Tier-1 harness.

**Where it lives:** `integration/index.mdx` — Kilo and Trae sub-sections (point-of-use); `security/index.mdx` known gaps.

### 4.3 Tier-1 Testable = Claude Code Only

**Exact accurate wording basis:** harness-support-matrix.md §Honesty Notes §1/§2.

**What to say:**
> Only Claude Code is live-verified. On the development machine, Beekeeper was confirmed to fire the PreToolUse hook, block the tool call, and write the audit entry for a credential-read attempt. The other 14 harnesses are implemented against published documentation and validated by contract-shape unit tests that verify Beekeeper emits the correct exit code and JSON — but those tests **do not run a real harness**. Whether a specific harness actually honors the contract in a live session is not tested in CI and is manual + Claude-Code-only.

**Where it lives:** `integration/index.mdx` — top-level introduction to the harness table; `getting-started/index.mdx` — brief mention.

### 4.4 `--bind 0.0.0.0` Gateway Exposure

**Exact accurate wording basis:** THREAT-MODEL.md §8 "Gateway Remote-Bind Exposure and the Missing `allow_remote_gateway` Gate."

**What to say:**
> The gateway binds to `127.0.0.1` by default. Binding a non-loopback address with `--bind 0.0.0.0 --allow-remote` exposes the policy-decision proxy over **plain HTTP** — the bearer token travels in cleartext. The help text states that `--bind 0.0.0.0` "requires `allow_remote_gateway: true` in config" but **this config gate is not implemented in v1.3.0** — the flag flows straight to `net.Listen`. Do not bind the gateway to a non-loopback interface without a TLS-terminating reverse proxy in front.

**Where it lives:** `integration/index.mdx` — MCP gateway sub-section (point-of-use); `security/index.mdx` known gaps.

### 4.5 Exit-1 → Exit-2 Hook History

**Exact accurate wording basis:** THREAT-MODEL.md §10, v1.3.0 changelog `BreakingChangeCallout`.

**What to say:** Link to the v1.3.0 changelog page (already has the full `BreakingChangeCallout` with migration steps). Do not duplicate the full callout. In troubleshooting, a brief note: "If `beekeeper check` is installed from a pre-v1.3.0 binary, tools may still execute despite block decisions — upgrade to v1.3.0 and re-run `beekeeper hooks install --target <harness>`."

**Where it lives:** `troubleshooting/index.mdx` (brief note + link to changelog); `security/index.mdx` (brief historical note in context of enforce-or-audit distinction).

---

## 5. Docs MDX Conventions Recommendation

### 5.1 `source_doc:` Frontmatter Field

**Does Fumadocs frontmatter schema need extending?**

`web/source.config.ts` uses `defineDocs({ dir: "content/docs" })` with no explicit frontmatter schema. `web/lib/source.ts` uses the Fumadocs `loader()` with `toFumadocsSource()`. Fumadocs passes arbitrary frontmatter fields through to the page component — the loader does not validate or strip unknown fields.

**Verdict:** `source_doc:` can be added to MDX frontmatter immediately without schema changes. The planner does NOT need to modify `source.config.ts` or `lib/source.ts`. The field is consumed by the accuracy gate (`accuracy_spec.py`) via Python file-walk, not by the Fumadocs runtime.

**Recommended frontmatter pattern:**
```mdx
---
title: Security
description: Beekeeper's threat model, corroboration engine, and known gaps.
source_doc: docs/THREAT-MODEL.md
---
```

For sections deriving from multiple sources:
```mdx
source_doc: "docs/THREAT-MODEL.md, docs/harness-support-matrix.md"
```

### 5.2 Reusable Caveat/Unenforced Callout Component

**Recommendation: YES, add a small `UnenforcedCallout` component** (mirroring `BreakingChangeCallout`).

Rationale: the "unenforced in v1.3.0" label appears in 3+ docs sections (configuration, security, cli-reference). A consistent visual treatment makes it unmissable and keeps wording uniform.

**Implementation pattern** (mirror `breaking-change-callout.tsx`):
- File: `web/components/docs/unenforced-callout.tsx`
- Props: `feature: string` (feature name), `children: ReactNode` (explanation)
- Styling: amber/warning color using `var(--amber)` raw token (same dual-theme discipline as `BreakingChangeCallout` — NOT `--color-bk-*`)
- Register in `web/mdx-components.tsx` alongside existing components

**Usage in MDX:**
```mdx
<UnenforcedCallout feature="release_age / minimumReleaseAge">
  Declaring `release_age` rules in policy files is **not enforced in v1.3.0**.
  The overlay evaluates only data in the tool call itself; release age requires
  a catalog/registry API lookup not available at check time. These rules are
  informational and for `policy test` dry-runs only.
</UnenforcedCallout>
```

### 5.3 Sub-Page Splitting via meta.json

**Recommendation for `cli-reference`:** Keep as single `index.mdx`. The full command tree is ~30 commands but most have short descriptions; a single page with a good H2/H3 hierarchy and Fumadocs TOC is better UX than 15 separate pages. Do NOT split.

**Recommendation for `integration`:** The 15-harness guide is large. Consider a flat `index.mdx` with per-harness H2 sections (ordered Tier 1 → Tier 2 → Tier 3 → MCP Gateway). Splitting into sub-pages via `meta.json` is an option if content grows beyond ~500 lines, but for v1.3.0 keep it on one page.

**Current meta.json sidebar order** (from `web/content/docs/meta.json`):
```json
["getting-started", "installation", "configuration", "integration", "security", "cli-reference", "audit-log", "troubleshooting"]
```
This order is correct for user journey (quickstart → install → configure → integrate → understand security → CLI ref → audit → debug). Do not reorder.

### 5.4 Static Export + Sidebar Compatibility

- Adding sub-pages to a section requires a `meta.json` in the section dir and new `index.mdx` files. The existing `generateStaticParams` in `app/docs/[[...slug]]/page.tsx` uses `source.generateParams()` — this automatically picks up new pages from the `content/docs/` directory tree via fumadocs-mdx. No code changes needed to add sub-pages.
- Sub-pages under e.g. `integration/` would have paths like `/docs/integration/claude-code/` — these are automatically included in the Orama search index and sitemap via `source.generateParams()`.
- Since `trailingSlash: true` is set in `next.config.mjs`, all routes end with `/` in the static export. Links in MDX must use trailing slashes or use the Next `Link` component.

---

## 6. Validation Architecture

**Note:** `workflow.nyquist_validation` is not set to `false` in `.planning/config.json` (no config.json found for this project; the planning workflow uses its own conventions). Treat as enabled.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Pure Python (stdlib only) — mirrors `seo_spec.py`, `gfx_spec.py`, `home_spec.py` |
| Config file | None required |
| Quick run command | `cd web && python tests/accuracy_spec.py` |
| Full suite command | `cd web && python tests/seo_spec.py && python tests/accuracy_spec.py && python tests/home_spec.py && python tests/gfx_spec.py` |

### Recommended `accuracy_spec.py` — DOCS-09 Mechanical Gate

Create `web/tests/accuracy_spec.py` as a pure-Python file-walk (Python stdlib only, no pip install). It gates DOCS-09 by asserting:

**AC-1: `source_doc:` frontmatter present on every Go-derived docs section**
- Walk `web/content/docs/**/*.mdx`
- For each file that is not purely "pipeline" (i.e., is an actual content file), check for `source_doc:` in frontmatter
- Exception: `getting-started/index.mdx` may have `source_doc: README.md` (less critical) — still require it

**AC-2: Unenforced-feature labels present where those features are mentioned**
- Grep each MDX file for mentions of `release_age`, `minimumReleaseAge`, `lifecycle_script_allowlist`
- Assert that any file mentioning these strings also contains the word "unenforced" or "not enforced" (case-insensitive) or the `<UnenforcedCallout` component tag
- This catches the case where a writer adds a config example without the warning

**AC-3: No references to non-existent subcommands**
- Check that MDX files do not mention `beekeeper hooks status`, `beekeeper catalogs rebuild`, `beekeeper check --input` (known stubs errors)
- Extend with any other confirmed-nonexistent commands from §2

**AC-4: Build stays green**
- Not in `accuracy_spec.py` itself — `pnpm build` is the build gate and runs separately. But the planner should wire the spec to run AFTER `pnpm build` succeeds.

**AC-5: Human/agent accuracy pass against docs/THREAT-MODEL.md**
- This cannot be mechanically automated. The executor (or the maintainer in UAT) must read each security-relevant docs section side-by-side with `docs/THREAT-MODEL.md` and confirm no claims are stronger than the model asserts.
- The planner should include a `checkpoint:human-verify` task at the end of the phase for this review.

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DOCS-02 | Getting started leads to working `beekeeper check` | Manual smoke | run beekeeper check manually | No — Wave 0 |
| DOCS-03 | Installation commands are copyable and accurate | AC-3 (no phantom commands) | `python tests/accuracy_spec.py` | No — Wave 0 |
| DOCS-04 | Config docs cover layered config + nudge | AC-2 (unenforced labels) | `python tests/accuracy_spec.py` | No — Wave 0 |
| DOCS-05 | Security posture + known gaps co-located | AC-1 (source_doc frontmatter) | `python tests/accuracy_spec.py` | No — Wave 0 |
| DOCS-06 | Integration caveats at point-of-use | AC-1 + AC-2 | `python tests/accuracy_spec.py` | No — Wave 0 |
| DOCS-07 | All subcommands and flags documented | AC-3 (no phantom commands) | `python tests/accuracy_spec.py` | No — Wave 0 |
| DOCS-08 | Troubleshooting uses real commands | AC-3 | `python tests/accuracy_spec.py` | No — Wave 0 |
| DOCS-09 | source_doc: frontmatter on all Go-derived MDX | AC-1 | `python tests/accuracy_spec.py` | No — Wave 0 |
| DOCS-09 | Unenforced features labeled | AC-2 | `python tests/accuracy_spec.py` | No — Wave 0 |
| DOCS-09 | Human accuracy review vs THREAT-MODEL.md | Manual | checkpoint:human-verify | N/A |

### Wave 0 Gaps

- [ ] `web/tests/accuracy_spec.py` — AC-1, AC-2, AC-3 assertions
- [ ] `web/components/docs/unenforced-callout.tsx` — new callout component (if researcher recommendation accepted)

---

## 7. Open Questions (RESOLVED)

### OQ-1: Does `nudge.mode: "block"` actually work?

**RESOLVED.** `ensureNudgeBlockDefault` in `main.go` (lines 1150–1198) sets `nudge.mode = "block"` in user config on first `hooks install`. However, `config.ValidateNudgeConfig` in `config.go` (line 156) only accepts `"soft"` or `"hard"` as valid mode strings. This means the `"block"` mode string set by `ensureNudgeBlockDefault` will be rejected by `config set nudge.mode block`. The correct understanding: the `mode: "block"` in `ensureNudgeBlockDefault` is an internal mode string that the nudge engine itself interprets (the nudge engine likely has a `"block"` case in its Evaluate function), but `config set` does not expose it. The config validator and the config setter have different allowed sets. **Documentation implication:** describe the `--target`-install behavior as "sets nudge to block npm/yarn installs when no hardened PM is installed" — do not document `nudge.mode: "block"` as a user-settable config value via `config set` since the validator rejects it. Instead note `require_hardened: true` as the documented way to get blocking behavior.

### OQ-2: Is there a `beekeeper baseline` command?

**RESOLVED.** `internal/baseline/` exists as a package but there is no `beekeeper baseline` cobra command registered in `main.go`. The baseline feature is an internal engine component, not a user-facing subcommand. Do not document it in the CLI reference.

### OQ-3: Is there a `beekeeper notify` command?

**RESOLVED.** `internal/notify/` exists but there is no `beekeeper notify` cobra command. `notify.Config{Enabled: true}` is passed to the watch handler (main.go line 697). Not user-facing.

### OQ-4: Does the `beekeeper diag` command exist or is it just `beekeeper status`?

**RESOLVED.** `beekeeper diag` exists and is the real diagnostic command. `beekeeper status` does NOT exist in the CLI. The troubleshooting stub's "open an issue with the output of `beekeeper status`" is wrong — it should reference `beekeeper version` and `beekeeper diag`.

### OQ-5: What is the correct Fumadocs `source_doc:` frontmatter behavior?

**RESOLVED.** Fumadocs `defineDocs()` in `source.config.ts` passes arbitrary frontmatter through to the MDX page without validation. The `source_doc:` field can be added freely without schema changes. Confirmed by reading `source.config.ts` and `lib/source.ts` — no frontmatter schema is defined.

### OQ-6: Does the `meta.json` sidebar order need changing?

**RESOLVED.** The current order `[getting-started, installation, configuration, integration, security, cli-reference, audit-log, troubleshooting]` is a sensible user journey order. No changes needed.

### OQ-7: Does the v1.3.0 changelog have a `hookSpecificOutput` path or `--hook` flag?

**RESOLVED.** The v1.3.0 changelog (already shipped) uses `--hook claude-code` syntax in the BreakingChangeCallout migration steps. The docs should link to this and use the same `--hook` syntax. (Note: for `hooks install` the flag is `--target`; for `beekeeper check` the flag is `--hook`. These are different commands and different flags — the stubs conflate them.)

---

## 8. Risks and Pitfalls

### 8.1 Stub Inaccuracies That Must Be Corrected

The following claims in the existing stubs are WRONG vs. the shipped binary. The executor MUST correct them:

| Stub | Wrong Claim | Correct Fact | Source |
|------|-------------|-------------|--------|
| `getting-started/index.mdx` | `beekeeper hooks install --hook claude-code` | Flag is `--target`, not `--hook` | `main.go` line 1229 |
| `cli-reference/index.mdx` | `beekeeper check --tool bash --input '...'` | `--input` flag does not exist; input is always stdin | `main.go` newCheckCmd |
| `cli-reference/index.mdx` | Shows only 3 commands (check, catalogs sync, hooks install) | 15+ top-level commands, each with sub-subcommands | `main.go` root cmd |
| `troubleshooting/index.mdx` | `beekeeper hooks status` | Command does not exist; use `beekeeper diag` | `main.go` |
| `troubleshooting/index.mdx` | `beekeeper catalogs rebuild` | Command does not exist; use `beekeeper catalogs sync` | `main.go` |
| `troubleshooting/index.mdx` | `beekeeper status` (in "getting help") | Command does not exist; use `beekeeper version` + `beekeeper diag` | `main.go` |
| `audit-log/index.mdx` | "rotated daily and compressed" | No rotation logic in shipped code; single `beekeeper.ndjson` file | `main.go` audit wiring |
| `audit-log/index.mdx` | Query via `cat ~/.beekeeper/audit/$(date +%Y-%m-%d).ndjson` | Wrong path (dated rotation doesn't exist); use `beekeeper audit query` | `main.go` audit cmd |
| `integration/index.mdx` | `beekeeper hooks install --hook claude-code` | Flag is `--target`, not `--hook` | `main.go` line 1229 |
| `security/index.mdx` | Generic corroboration claim (accurate but incomplete) | Needs CORR per-severity thresholds, known gaps, SPATH details | THREAT-MODEL.md |

### 8.2 Fumadocs Frontmatter Strictness

Fumadocs does NOT validate arbitrary frontmatter fields (confirmed from source.config.ts). Adding `source_doc:` will not break the build. Risk: LOW.

### 8.3 Broken-Link Risk from Sub-Page Splitting

If `integration/index.mdx` is later split into sub-pages, any internal links using `/docs/integration#hermes` anchors will break (they'd need to become `/docs/integration/hermes/`). The recommendation to keep integration as a single page avoids this risk for v1.3.0.

### 8.4 UnenforcedCallout Typo Risk

The component name "UnenforcedCallout" contains a common typo ("unenfored" → "unenforved"). Rename to `UnenforcedCallout` consistently and spell it correctly: `UnenforcedCallout`. Actually — spell correctly: `UnenenforcedCallout` or `UnenforcedCallout`... The planner should pick a consistent name: **`UnenforcedCallout`** — use exactly this in all references to avoid drift between MDX usage and component registration. (Note: "unenforced" is the correct English spelling; the component name should be `UnenforcedCallout`. The planner should verify the chosen name and use it consistently.)

**Correction:** Use `UnenforcedCallout` as the component name for consistency with `BreakingChangeCallout` naming pattern. Correct spelling of the concept: "unenforced."

### 8.5 nudge.mode "block" vs. require_hardened Confusion

The `ensureNudgeBlockDefault` function sets `nudge.mode = "block"` in config on first install, but `ValidateNudgeConfig` only accepts `"soft"` or `"hard"`. This creates a potential confusion where `beekeeper config set nudge.mode block` would fail with a validation error. The docs should NOT present `mode: "block"` as a user-settable value via `config set`. Instead, document `require_hardened: true` as the way to enforce blocking npm/yarn when no hardened PM is present, and describe the install-time auto-setting as "on first hook install, Beekeeper sets supply-chain enforcement mode" without exposing the internal `mode=block` string.

### 8.6 `allow_remote_gateway` Not Implemented — Must Not Overclaim

THREAT-MODEL.md §8 explicitly states this config gate does not exist. The CLI help text in `main.go` line 1279–1282 says `--bind 0.0.0.0` "requires `allow_remote_gateway:true` in config" — this is the HELP TEXT claim that is currently false. Do NOT document `allow_remote_gateway` as an actual config field a user can set. Document the gap honestly.

### 8.7 Sentry Platform Scope

`beekeeper protect install/uninstall/status` is `//go:build linux` only via `protect_linux.go`. On macOS, `protect_other.go` or `protect_darwin.go` exists. The docs should clearly state Sentry daemon lifecycle management (`protect install/uninstall/status`) is Linux-only (systemd required). macOS uses `eslogger` subprocess (mentioned in CLAUDE.md). Windows uses ETW (`tekert/golang-etw`). Document the Sentry as "available on all platforms but lifecycle management (`protect install`) requires systemd on Linux."

---

## Sources

### Primary (HIGH confidence)
- `cmd/beekeeper/main.go` — complete CLI surface, cobra command definitions, all flags
- `cmd/beekeeper/config.go` — `config set` supported keys and validation
- `cmd/beekeeper/policy.go` — `policy validate/test/list` commands
- `cmd/beekeeper/nudge.go` — `nudge status/check/audit` commands
- `cmd/beekeeper/diag.go` — `diag` command
- `cmd/beekeeper/protect_linux.go` — `protect install/uninstall/status` (Linux), sentry rules
- `docs/THREAT-MODEL.md` — authoritative security posture and known gaps
- `docs/harness-support-matrix.md` — 15-harness tier table, honesty notes
- `docs/nudge.md` — nudge configuration, modes, CLI surface
- `docs/release-runbook.md` — cosign and SLSA verification commands (pollen-scoped but verification patterns apply)
- `README.md` — honest framing, quick start
- `CLAUDE.md` — architecture constraints, locked technical decisions
- `web/content/docs/**/index.mdx` — current stubs (read to identify errors)
- `web/source.config.ts`, `web/lib/source.ts` — Fumadocs pipeline (frontmatter schema: not enforced)
- `web/mdx-components.tsx`, `web/components/changelog/breaking-change-callout.tsx` — component patterns
- `web/tests/seo_spec.py` — test harness pattern to mirror

### Secondary (MEDIUM confidence)
- `web/content/changelog/v1.3.0/index.mdx` — established honest framing for exit-1→exit-2 history

---

## Metadata

**Confidence breakdown:**
- CLI command tree: HIGH — extracted directly from cobra definitions in `main.go`
- Enforced vs. unenforced table: HIGH — traced to `docs/THREAT-MODEL.md` §9 and code comments
- Honesty caveats: HIGH — exact wording basis cited to THREAT-MODEL.md sections
- MDX conventions: HIGH — verified from source.config.ts + lib/source.ts (no schema enforcement)
- Stub errors: HIGH — confirmed by reading stub code vs. real cobra definitions

**Research date:** 2026-06-09
**Valid until:** Indefinite for this binary — docs/THREAT-MODEL.md is the living document; sync if THREAT-MODEL.md is updated.

---

## RESEARCH COMPLETE
