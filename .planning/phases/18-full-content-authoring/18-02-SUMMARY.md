# 18-02 Summary — Getting Started + Installation

**Plan:** 18-02-PLAN.md (Wave 2) · **Status:** ✅ Complete · **Date:** 2026-06-09
**Requirements:** DOCS-02, DOCS-03, DOCS-09

## What was authored

- **`getting-started/index.mdx` (DOCS-02, SC-1)** — full quickstart: prerequisites → `go install` → `beekeeper init` (`--yes`/`--no-editors`) → `catalogs sync` → `hooks install --target claude-code` → manual `beekeeper check` via stdin → audit. Fixed the stub's `--hook` → `--target` error. Documents that first install sets `nudge.mode=block` (supply-chain enforcement) with the `config set nudge.mode soft` opt-down. Ends with the Claude-Code-only live-verified caveat linking `/docs/integration/`. `source_doc: README.md, docs/harness-support-matrix.md`.
- **`installation/index.mdx` (DOCS-03, SC-2)** — `go install`, GitHub Releases, the EXACT cosign + SLSA + SBOM commands copied verbatim from `docs/THREAT-MODEL.md` §7 (lowercase `bantuson` in the SLSA `--source-uri`, workflow URL in the cosign identity), `make verify-release`, build-from-source, and a state-dir table. `source_doc: docs/THREAT-MODEL.md, docs/release-runbook.md, CLAUDE.md`.

## Verification

- `python tests/accuracy_spec.py`: both files PASS AC-1 (source_doc present, all referenced paths exist) and AC-3 (no phantom commands). Overall suite still RED (expected — 6 stubs remain).
- Acceptance greps: getting-started `--target`=1 / `--hook`-install=0 / `beekeeper check`=6 / live-verified=2 / `--input`=0; installation cosign=1 / slsa=1 / go-install=1 / lowercase source-uri=1 / %APPDATA=1.

## Notes

- Honest framing preserved (repo unpushed → releases may not resolve yet); commands are exactly the ones the pipeline signs.
- All JSON/command examples kept inside fenced blocks to avoid the MDX `${...}`/`<...>` interpolation trap (Phase-14 lesson).

## Commits

- `5fea39d` docs(18-02): Getting Started
- `057b4cf` docs(18-02): Installation (commit-message prefix has a cosmetic "18-03..." typo; file is installation/index.mdx)
