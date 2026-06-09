# 18-05 Summary — Troubleshooting + Audit Log

**Plan:** 18-05-PLAN.md (Wave 2) · **Status:** ✅ Complete · **Date:** 2026-06-09
**Requirements:** DOCS-08, DOCS-09

## What was authored

- **`troubleshooting/index.mdx` (DOCS-08)** — rewritten around `beekeeper diag` as the primary diagnostic. **All three phantom commands removed**: `beekeeper hooks status` → `beekeeper diag`; `beekeeper catalogs rebuild` → `beekeeper catalogs sync`; the "getting help" `beekeeper status` → `beekeeper version` + `beekeeper diag`. Covers hook-not-firing (with the exit-1→2 note + changelog link), catalog sync, latency, self-quarantine, policy rejection, nudge fail-open detection (2s timeout), gateway, Sentry (Linux/systemd), LlamaFirewall, and the Windows `%APPDATA%` state-dir gotcha. `source_doc: docs/THREAT-MODEL.md, docs/nudge.md, cmd/beekeeper/diag.go`.
- **`audit-log/index.mdx` (DOCS-08/09)** — **corrected the false stub claims**: a single `beekeeper.ndjson` file with **no rotation/compression** (was "rotated daily and compressed"), and `beekeeper audit query`/`tail`/`export` replace the wrong `cat …$(date +%Y-%m-%d).ndjson` path. Documents `nudge audit` fields, record examples, and the field-scoped redaction caveat (Sentry-derived fields reach remote sinks verbatim). `source_doc: docs/THREAT-MODEL.md, cmd/beekeeper/main.go, cmd/beekeeper/nudge.go`.

## Verification

- `python tests/accuracy_spec.py`: **FULL SUITE GREEN — ALL ASSERTIONS PASSED** (AC-1/AC-2/AC-3 across all 8 sections now that the last source_doc fields + the troubleshooting phantom-command fix landed).
- troubleshooting greps: `hooks status`=0, `catalogs rebuild`=0, bare `status`=0, `beekeeper diag`=6, `/changelog/v1.3.0`=1.
- audit-log greps: `rotated daily`=0, `beekeeper.ndjson`=3, `beekeeper audit query`=2, dated-file `date +%Y-%m-%d`=0, `redact`=4.

## Commits

- `82e80b7` docs(18-05): Troubleshooting
- `b13fced` docs(18-05): Audit Log
