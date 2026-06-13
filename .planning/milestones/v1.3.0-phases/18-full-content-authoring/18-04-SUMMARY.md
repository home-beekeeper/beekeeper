# 18-04 Summary — Security + Integration

**Plan:** 18-04-PLAN.md (Wave 2) · **Status:** ✅ Complete · **Date:** 2026-06-09
**Requirements:** DOCS-05, DOCS-06, DOCS-09

## What was authored

- **`security/index.mdx` (DOCS-05, SC-3)** — posture (corroboration model, fail-closed defaults, SPATH, self-protection, beekeeper-self, build hardening) **co-located on the same page** with the full known-gaps list (Hermes fail-open, Tier-3 UNGUARDED, Claude-Code-only verification, `--bind 0.0.0.0` + the not-implemented `allow_remote_gateway` gate, project `fail_mode:open`, Windsurf/OpenCode gaps, package-parse evasion, catalog poisoning, TM-B-02 presence-check, Linux fanotify mmap, Windows PPID). `release_age`/`lifecycle_script_allowlist` in `<UnenforcedCallout>`. exit-1→2 history links `/changelog/v1.3.0/` (not duplicated). Authored against the full `docs/THREAT-MODEL.md`, claims kept no stronger than the model. `source_doc: docs/THREAT-MODEL.md, docs/harness-support-matrix.md, CLAUDE.md`.
- **`integration/index.mdx` (DOCS-06, SC-4)** — Tier 1 → 2 → 3 → gateway, with each caveat at point-of-use: Hermes fail-open in the Hermes section, Kilo/Trae UNGUARDED in their sections, Claude-Code-only live-verified in the intro/table, `--bind 0.0.0.0` (gate not implemented) in the gateway section. Uses `hooks install --target` (fixed the stub `--hook`); gateway documented at `127.0.0.1:7837` with `--upstream`/`token`/`status`. `source_doc: docs/harness-support-matrix.md, docs/THREAT-MODEL.md, cmd/beekeeper/main.go`.

## Verification

- `python tests/accuracy_spec.py`: both files PASS AC-1/AC-2/AC-3.
- security greps: fail-open=3, unguarded=1, release_age=2, 0.0.0.0=1, `<UnenforcedCallout`=1, quarantine=3, `/changelog/v1.3.0`=1.
- integration greps: `--target`=14, `--hook`-install=0, fail-open=3, unguarded=3, live-verified=2, 0.0.0.0=1, 7837=2, 127.0.0.1=2.

## Notes

- Gateway invocation documented as the source-verified `beekeeper gateway --upstream <url> --port 7837` (README's `gateway start` form is stale; code wins per D-04).
- These two pages are the primary target of the 18-06 maintainer AC-5 accuracy review.

## Commits

- `4fffac0` docs(18-04): Security
- `933c85e` docs(18-04): Integration
