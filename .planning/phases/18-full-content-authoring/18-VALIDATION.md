---
phase: 18
slug: full-content-authoring
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-09
---

# Phase 18 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Phase 18 is content authoring; its gate is **accuracy**, not behavior. DOCS-09 is
> verified by a pure-Python `web/tests/accuracy_spec.py` file-walk over
> `web/content/docs/**/*.mdx` (no browser, no build dependency for the walk itself),
> mirroring the existing `seo_spec.py` / `gfx_spec.py` / `home_spec.py` harness pattern,
> PLUS a mandatory human/agent accuracy review against `docs/THREAT-MODEL.md`.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Python 3.12 standalone spec scripts (project pattern — NOT pytest). New: `web/tests/accuracy_spec.py` (stdlib `glob`/`re` file-walk over `content/docs/**/*.mdx` — reads source MDX, not built `out/`, so it needs no `pnpm build` to run) |
| **Config file** | none — `accuracy_spec.py` is self-contained like the existing specs |
| **Quick run command** | `cd web && python tests/accuracy_spec.py` |
| **Full suite command** | `cd web && pnpm build && python tests/accuracy_spec.py && python tests/seo_spec.py && python tests/home_spec.py && python tests/gfx_spec.py` |
| **Estimated runtime** | accuracy_spec ~1–3s (source file walk); full suite ~40–70s incl. `pnpm build` |

---

## Sampling Rate

- **After every task commit:** `cd web && python tests/accuracy_spec.py` (source-MDX walk — no build needed; fast feedback as each section is authored)
- **After every plan wave:** Full suite command (accuracy + build + seo + home + gfx regression — proves the new content still builds the static export)
- **Before `/gsd-verify-work`:** Full suite green AND the human accuracy review (AC-5) signed off
- **Max feedback latency:** ~3 seconds for the per-task accuracy walk; ~70s for the full suite

---

## Per-Task Verification Map

> Task IDs are assigned by the planner. Rows below are the requirement→assertion
> contract each authored section must satisfy (DOCS-02..09 / SC-1..5). This phase has a
> minimal security surface — it documents the shipped binary, it does not change it —
> so the dominant "secure behavior" is *accuracy / honesty*: no overclaim, no phantom
> command, every unenforced feature labeled.

| Task (SC) | Plan | Wave | Requirement | Secure / Accuracy Behavior | Test Type | Automated Command | File Exists | Status |
|-----------|------|------|-------------|----------------------------|-----------|-------------------|-------------|--------|
| accuracy_spec harness (Wave 0) | 01 | 1 | DOCS-09 | N/A (infra) | infra | `python tests/accuracy_spec.py` (red until content lands) | ❌ W0 | ⬜ pending |
| Getting Started quickstart (SC-1) | — | — | DOCS-02 | no step references unshipped behavior; commands real (AC-3) | static-MDX assert + manual smoke | `python tests/accuracy_spec.py` | ❌ W0 | ⬜ pending |
| Installation (SC-2) | — | — | DOCS-03 | `go install` + Releases + cosign/SLSA commands are copyable & real (AC-3) | static-MDX assert | `python tests/accuracy_spec.py` | ❌ W0 | ⬜ pending |
| Configuration (SC) | — | — | DOCS-04 | unenforced features labeled (AC-2); nudge modes accurate to real validator | static-MDX assert | `python tests/accuracy_spec.py` | ❌ W0 | ⬜ pending |
| Security + known gaps co-located (SC-3) | — | — | DOCS-05 | `source_doc:` present (AC-1); gaps present (Hermes fail-open, Tier-3, release_age, --bind) | static-MDX assert | `python tests/accuracy_spec.py` | ❌ W0 | ⬜ pending |
| Integration point-of-use caveats (SC-4) | — | — | DOCS-06 | Hermes fail-open + Tier-3 UNGUARDED present at point-of-use (AC-1+AC-2) | static-MDX assert | `python tests/accuracy_spec.py` | ❌ W0 | ⬜ pending |
| CLI reference all subcommands/flags (SC-4) | — | — | DOCS-07 | no phantom commands (`hooks status`, `catalogs rebuild`, `check --input`) (AC-3) | static-MDX assert | `python tests/accuracy_spec.py` | ❌ W0 | ⬜ pending |
| Troubleshooting (SC) | — | — | DOCS-08 | uses real commands (`diag`, `version`) not phantom ones (AC-3) | static-MDX assert | `python tests/accuracy_spec.py` | ❌ W0 | ⬜ pending |
| source_doc: frontmatter on all Go-derived MDX (SC-5) | — | — | DOCS-09 | every Go-derived section carries `source_doc:` (AC-1) | static-MDX assert | `python tests/accuracy_spec.py` | ❌ W0 | ⬜ pending |
| Human accuracy review vs THREAT-MODEL.md (SC-5) | — | — | DOCS-09 | no claim stronger than the threat model asserts | manual | `checkpoint:human-verify` | N/A | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

### Assertion detail (what `accuracy_spec.py` proves)

- **AC-1 (`source_doc:` frontmatter):** every Go-derived `content/docs/**/index.mdx` (and any sub-page) has a non-empty `source_doc:` frontmatter field pointing at a real repo path (`docs/*.md`, `CLAUDE.md`, `README.md`, or a `cmd/`/`internal/` path). The spec asserts the referenced path(s) actually exist on disk.
- **AC-2 (unenforced labels):** any MDX file whose body mentions `release_age`, `minimumReleaseAge`, or `lifecycle_script_allowlist` MUST also contain "unenforced"/"not enforced" (case-insensitive) or the `<UnenforcedCallout` component tag — catches a config example added without its warning.
- **AC-3 (no phantom commands):** no MDX file mentions the confirmed-nonexistent surface: `beekeeper hooks status`, `beekeeper catalogs rebuild`, `beekeeper check --input`, `beekeeper status` (extend with any others confirmed in 18-RESEARCH.md §8). Guards against re-introducing the stub errors the research caught.
- **Build gate (separate):** `pnpm build` must exit 0 with the new content (run in the full suite, not inside `accuracy_spec.py`) — proves the MDX + any new callout component compile under the Next.js 16 static export.

---

## Wave 0 Requirements

- [ ] `web/tests/accuracy_spec.py` — AC-1 / AC-2 / AC-3 assertions over `content/docs/**/*.mdx` (the validation harness; red until the content + frontmatter land)

*Built first so the accuracy assertions exist before the authoring tasks; mirrors how `seo_spec.py` / `gfx_spec.py` were staged in Phases 16/17.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Accuracy review against `docs/THREAT-MODEL.md` (AC-5) | DOCS-09 | Semantic accuracy — "is this claim stronger than the threat model asserts?" cannot be reduced to a regex | Executor/maintainer reads each security-relevant docs section (security, integration, configuration, getting-started) side-by-side with `docs/THREAT-MODEL.md` and confirms no overclaim; planner adds a `checkpoint:human-verify` task at phase end |
| Getting Started quickstart actually reaches a working `beekeeper check` (SC-1) | DOCS-02 | End-to-end "follow the steps on a clean machine" is a human smoke test | Maintainer (or executor on the dev machine) follows the quickstart verbatim and confirms a `beekeeper check` invocation runs and returns a decision |
| Docs render correctly in the served site (sidebar/TOC/search still work with expanded content) | DOCS-01 regression | Visual/interaction — covered by Phase 19 E2E later, but a quick manual look at `pnpm build` + served `out/` is the v1.3.0 check | Maintainer browses the built docs locally; confirms nav + search return the new pages |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references (`accuracy_spec.py`)
- [ ] No watch-mode flags
- [ ] Feedback latency < 70s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
