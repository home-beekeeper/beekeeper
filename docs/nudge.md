# Package Manager Nudge

Beekeeper's package manager nudge feature intercepts `npm install` (and related
commands) from autonomous agents and steers them toward `pnpm` (>=11.0.0) or
`bun` (>=1.3.0) when either is installed locally.  Both managers ship structural
supply-chain defenses that npm does not:

- **pnpm 11+** enables `minimumReleaseAge` (1-day hold on new packages),
  `blockExoticSubdeps`, and lifecycle-script allowlists by default.
- **Bun 1.3+** with `@socketsecurity/bun-security-scanner` routes every install
  through the Socket Security API, which catches typosquatting and malware in
  real time.

This is defense-in-depth: Beekeeper's catalog matching is *reactive* (depends on
threat-intel freshness); pnpm/Bun strict defaults are *proactive* (enforce structural
policy regardless of intel).  The two layers stack.

Beekeeper does **not** install or configure pnpm or bun.  Detection is read-only;
any rewriting (hard mode) is advisory — the agent receives the rewritten command
string; Beekeeper does not execute it.

---

## Version floors

| Package Manager | Minimum floor | Reason | Recommended |
|---|---|---|---|
| pnpm | `11.0.0` | `minimumReleaseAge` and `blockExoticSubdeps` on by default in 11.x | latest 11.x |
| bun | `1.3.0` | Security Scanner API stable from 1.3 | latest 1.x |
| Node.js (for pnpm 11) | `22.0.0` | pnpm 11 requires Node 22+; see caveat below | latest 22.x LTS |

### Node 22 Maintenance-LTS caveat

pnpm 11 requires Node.js >= 22.  Node 22 entered **Maintenance LTS** in April 2026
and remains supported until April 2027.  Node 24 is the current Active LTS and the
**recommended target for new setups**.

Beekeeper's version floor stays at 22.0.0 (pnpm 11 accepts it), but when Node < 22
is detected the nudge emits reason `node-incompatible-with-pnpm-11` and advises the
operator rather than rewriting — pnpm 11 would fail at install time on an older
runtime, so the rewrite would not help.

---

## Soft vs hard mode

### Soft mode (default)

Beekeeper emits an advisory message recommending the operator switch to pnpm or bun,
then **allows the original `npm install` to proceed** (exit 0).  The agent's
workflow is not interrupted.

Reason codes emitted in soft mode: `pnpm-available-soft`, `bun-available-soft`,
`bun-available-no-scanner`, `node-incompatible-with-pnpm-11`, `no-arg-install-soft`,
`sudo-passthrough`, `no-hardened-pm`.

### Hard mode (opt-in)

Beekeeper **rewrites** the agent's command to the pnpm/bun equivalent (e.g.
`npm install chalk@5.4.0` → `pnpm add chalk@5.4.0`) and surfaces the rewritten
string in the nudge decision.  The rewrite is advisory: Beekeeper does not execute
the rewritten command; the agent receives it and decides what to do.

Enable hard mode:

```sh
beekeeper config set nudge.mode hard
```

### `requireHardened` (opt-in)

When `require_hardened: true` is set, Beekeeper **blocks** `npm install` commands
when no hardened package manager is installed.  Default `false` — npm calls proceed
with an advisory message.

---

## Configuration

The nudge block lives under the `"nudge"` key in `~/.beekeeper/config.json`.
A project-level `.beekeeper/config.json` can override user config — for example,
setting `nudge.enabled: false` to opt out project-wide.

### Default `config.json` nudge block

The following block reflects `DefaultNudgeConfig()` defaults.  Copy this into your
`~/.beekeeper/config.json` (or let Beekeeper generate it on first run) and adjust
only the fields you want to override.

```json
{
  "nudge": {
    "enabled": true,
    "mode": "soft",
    "require_hardened": false,
    "preferred": "pnpm",
    "check_socket_scanner": true,
    "major_drift_check": {
      "enabled": true,
      "interval": "168h"
    },
    "version_floors": {
      "pnpm": "11.0.0",
      "bun": "1.3.0",
      "node": "22.0.0"
    }
  }
}
```

Field reference:

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | bool | `true` | Master switch for the nudge feature. Set to `false` in a project `.beekeeper/config.json` to opt out project-wide (NUDGE-08 layered disable). |
| `mode` | string | `"soft"` | `"soft"` = advise + proceed (exit 0); `"hard"` = rewrite command and surface to agent. Values other than `"soft"` or `"hard"` are rejected by ValidateNudgeConfig. |
| `require_hardened` | bool | `false` | When `true`, block npm install when no hardened PM (pnpm or bun meeting the floor) is installed. Default `false` — npm calls proceed with advisory. |
| `preferred` | string | `"pnpm"` | When both pnpm and bun are installed and meet their floors, enter the `preferred` branch first. Must be `"pnpm"` or `"bun"`. |
| `check_socket_scanner` | bool | `true` | For bun: require `@socketsecurity/bun-security-scanner` in `bunfig.toml` before treating bun as hardened. Set to `false` to treat bun as hardened without the scanner (reduces security). |
| `major_drift_check.enabled` | bool | `true` | Enable the weekly pnpm/bun major-version drift check. Logs a `version_drift` audit record when a new major is detected. Does not auto-update floors. |
| `major_drift_check.interval` | string | `"168h"` | How often to check for a new pnpm/bun major version. Accepts Go duration strings (e.g. `"168h"` = 7 days, `"24h"` = daily). |
| `version_floors.pnpm` (`versionFloors.pnpm`) | string | `"11.0.0"` | Minimum acceptable pnpm version. Versions below this floor are not considered hardened. |
| `version_floors.bun` (`versionFloors.bun`) | string | `"1.3.0"` | Minimum acceptable bun version. Versions below this floor are not considered hardened. |
| `version_floors.node` (`versionFloors.node`) | string | `"22.0.0"` | Minimum Node.js version required for pnpm 11 compatibility. Below this floor, nudge emits `node-incompatible-with-pnpm-11` and advises without rewriting. |

---

## CLI surface

### `beekeeper nudge status`

Detects the local PM state (runs `pnpm/bun/node --version` with a 2-second
timeout each) and prints a human-readable summary alongside the active nudge
configuration.  Output is plain text, not NDJSON.

```
=== Package Manager State ===
  pnpm:  11.3.0 (hardened: yes)
  bun:   not installed
  node:  22.11.0

=== Active Nudge Configuration ===
  enabled:              true
  mode:                 soft
  preferred:            pnpm
  require_hardened:     false
  check_socket_scanner: true
  ...
```

### `beekeeper nudge check "<command>"`

Dry-runs the nudge decision for a given command string.  The command is **parsed
only** — never executed (T-08-27).  Detection runs `pnpm/bun/node --version` with
a 2-second timeout.

```sh
beekeeper nudge check "npm install chalk"
```

Output:
```
decision:  warn
reason:    pnpm-available-soft
action:    advise
rewritten: -
```

With `mode: hard`:
```
decision:  warn
reason:    pnpm-hard-rewrite
action:    rewrite
rewritten: pnpm add chalk
```

### `beekeeper nudge audit [--since=<duration|RFC3339>]`

Queries the Beekeeper audit log (`~/.beekeeper/audit/beekeeper.ndjson`) filtered to
`record_type:"nudge"` records and streams matching NDJSON lines to stdout.

```sh
beekeeper nudge audit --since=24h
beekeeper nudge audit --since=2026-06-01T00:00:00Z
```

### `beekeeper config set nudge.<key> <value>`

Changes a nudge configuration setting, validates the new value fail-closed
(an invalid value is rejected with no write), saves to `~/.beekeeper/config.json`,
and emits a `config_change` audit record (PRD §5.2, §10-17).

```sh
beekeeper config set nudge.mode hard
beekeeper config set nudge.enabled false
beekeeper config set nudge.preferred bun
beekeeper config set nudge.require_hardened true
beekeeper config set nudge.check_socket_scanner false
```

Invalid values are rejected immediately:
```sh
beekeeper config set nudge.mode aggressive
# Error: config set: validation failed (no write): invalid nudge mode "aggressive" (want "soft" or "hard")
```

---

## Audit records

Every nudge decision is logged as a `record_type:"nudge"` NDJSON record in
`~/.beekeeper/audit/beekeeper.ndjson`.  Key fields:

| Field | Description |
|---|---|
| `record_type` | `"nudge"` |
| `nudge_action` | Closed §9 enum: `advise` \| `proceed` \| `rewrite` \| `block` |
| `original_command` | The original agent command string |
| `rewritten_command` | The rewritten command (only when action=rewrite) |
| `reason_code` | Structured reason from `internal/nudge/reasons.go` |
| `pm_state` | Flattened PM state string for forensic provenance |

Config changes are logged as `record_type:"config_change"` records.

---

## Security notes

- **Fail-open detection**: PM detection (binary exec) is *fail-open by design*. A
  timeout or missing binary treats the PM as "not installed" and allows the agent's
  command to proceed.  This is the **documented soft-nudge exception** to the
  catalog/path fail-closed rule — a slow or absent binary must never block an agent.
- **Fail-closed config validation**: An invalid nudge config value (e.g. unknown mode)
  is rejected at load time and on `config set`.  No write happens on a validation
  failure (T-08-26).
- **No shell execution**: `nudge check` parses the command string and runs only
  fixed-argv `pnpm/bun/node --version` calls.  The operator's command string is never
  passed to a shell.
