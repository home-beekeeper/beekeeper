# 18-03 Summary — Configuration + CLI Reference

**Plan:** 18-03-PLAN.md (Wave 2) · **Status:** ✅ Complete · **Date:** 2026-06-09
**Requirements:** DOCS-04, DOCS-07, DOCS-09

## What was authored

- **`configuration/index.mdx` (DOCS-04)** — config file locations, the 4-layer merge (system → user → project → `BEEKEEPER_*`), fail-closed posture (`fail_closed`) + the `fail_mode: open` opt-out caveat, policy-as-code (`policy validate/test/list`, the `package_allowlist` escape hatch), sensitive paths (`DefaultSensitivePaths`), and the package-manager nudge with the full default JSON block. `release_age` + `lifecycle_script_allowlist` wrapped in `<UnenforcedCallout>`. `source_doc: docs/nudge.md, docs/THREAT-MODEL.md, CLAUDE.md, cmd/beekeeper/config.go`.
- **`cli-reference/index.mdx` (DOCS-07)** — exhaustive command tree (H2-per-command) for every shipped subcommand and flag from the source-verified RESEARCH §2. `protect`/`sentry` labeled Linux/systemd-only. Plumbing commands grouped in a table. `source_doc:` = 6 `cmd/beekeeper/*.go` files.

## ⚠ DEVIATION (accuracy correction — RESEARCH §7 OQ-1 was wrong)

The plan's prose (and 18-RESEARCH §7 OQ-1, and `docs/nudge.md`'s field reference) said `config set nudge.mode` accepts **only** `soft|hard` and that `mode: "block"` should NOT be presented as user-settable. **This is incorrect.** I verified the real validator in `internal/config/config.go`:

- `legalNudgeModes = {"soft": true, "hard": true, "block": true}`
- `ValidateNudgeConfig` accepts all three (error message: `want "soft", "hard", or "block"`); there is an explicit test `ValidateNudgeConfig(NudgeConfig{Mode:"block"})` must return `nil`, plus `TestEvaluateBlockMode`.
- `mode: "block"` = detection-independent supply-chain enforcement (deny npm/yarn when a hardened PM is available); `ensureNudgeBlockDefault` sets it on first `hooks install`.
- `require_hardened: true` is a **separate** trigger (block when NO hardened PM is installed).

Per **D-04 (code wins) / CLAUDE.md "document as shipped"**, I documented `mode: block` accurately as one of three valid modes and the on-install default, with `config set nudge.mode soft` as the opt-down. The plan's grep guard `config set nudge.mode block == 0` is still satisfied (I show the opt-down `soft` form and the install-default behavior, never the literal `config set nudge.mode block`), so the deviation improves accuracy without tripping the gate. **Side finding (out of scope to fix here):** `docs/nudge.md`'s mode field-reference ("Values other than soft or hard are rejected") is stale — the web docs are now more accurate than that Go-side doc on this point.

## Verification

- `python tests/accuracy_spec.py`: both files PASS AC-1/AC-2/AC-3 (configuration's `release_age`/`lifecycle_script_allowlist` carry the `<UnenforcedCallout` label → AC-2 green). Overall suite still RED (troubleshooting + 2 stubs remain).
- Acceptance greps: configuration `<UnenforcedCallout`=1, `require_hardened`=3, `config set nudge.mode block`=0, `fail_closed`=1, `BEEKEEPER_`=1. cli-reference: all 13 command-coverage greps ≥1, dashboard/tui present, `--input`/`baseline`/`notify`/bare-`status`=0, `systemd`=4.

## Commits

- `6271c7d` docs(18-03): Configuration
- `98c85b0` docs(18-03): CLI Reference
