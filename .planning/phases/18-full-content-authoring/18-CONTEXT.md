# Phase 18: Full Content Authoring - Context

**Gathered:** 2026-06-09
**Status:** Ready for planning
**Source:** Inline capture (discuss-phase skipped by maintainer choice — Phase 13/14/15/16/17 precedent)

<domain>
## Phase Boundary

Phase 18 replaces the **stub docs** (8 sections of ~20–28 lines each, authored as functional placeholders in Phase 13's pipeline work) with the **complete, accurate documentation set**. It is the content-authoring phase: every documentation path a developer can follow — quickstart → installation → configuration → security posture → integration → CLI reference → troubleshooting → audit log — is written to be **accurate to the shipped binary**, with security claims cited to source and unenforced features explicitly labeled.

**IN scope (from ROADMAP SC-1..5 / DOCS-02..09):**
- **DOCS-02** — Getting Started / Quickstart: zero → a working `beekeeper check` invocation, no steps referencing unshipped behavior.
- **DOCS-03** — Installation: `go install`, GitHub Releases binary download, **and** cosign + SLSA verification, all with copyable commands.
- **DOCS-04** — Configuration: layered config, policy-as-code, sensitive paths, package-manager nudge — with copyable examples.
- **DOCS-05** — Security posture **co-located with known gaps**: corroboration model, fail-closed defaults, threat model summary **alongside** the limitations (Hermes fail-open, Tier-3 unguarded, `release_age`/lifecycle allowlist unenforced, `--bind 0.0.0.0` caveat).
- **DOCS-06** — Integration guides for supported harnesses (Claude Code / Cursor / Codex hooks, MCP gateway) with **honest caveats at point-of-use**.
- **DOCS-07** — CLI / command reference for `beekeeper` subcommands and flags.
- **DOCS-08** — Troubleshooting for common issues.
- **DOCS-09** — Accuracy gate: every MDX file deriving content from a Go-side doc carries a `source_doc:` frontmatter field; all content reviewed against `docs/THREAT-MODEL.md` before publish; unenforced features (`release_age`/`minimumReleaseAge`, `lifecycle_script_allowlist`) explicitly labeled as not-enforced-in-v1.3.0.

**OUT of scope (explicit):**
- **Docs visual styling / Fumadocs theme redesign** — the "docs look old/basic" concern (`.planning/todos/pending/docs-styling-polish.md`) is DEFERRED to its own phase with its own UI-SPEC + ui-review gate (maintainer decision 2026-06-09). Phase 18 touches **content** (`web/content/docs/**/*.mdx`, `meta.json` ordering, and at most small content-callout MDX components), NOT the Fumadocs `DocsLayout`/theme CSS. Despite ROADMAP "UI hint: yes", this phase carries no UI-SPEC.
- **Changelog content** — already authored in Phase 14 (v1.0.0 / v1.2.0 / v1.3.0). Not re-touched here.
- **Any Go product change** — the binary is documented **as shipped**, never modified to match a doc. If docs and code disagree, the code wins and the doc is corrected.
- **Marketing home / SEO / 3D** — Phases 15/16/17, complete.

</domain>

<decisions>
## Implementation Decisions (locked)

### D-01 — Phase 18 is pure content authoring; styling is a separate phase
The deferred "docs styling looks old/basic" item is NOT folded in (maintainer decision 2026-06-09, backlog-review choice). Phase 18 deliverables are MDX prose + `meta.json` navigation ordering + (at most) small reusable content-callout components. The Fumadocs theme/`DocsLayout` redesign is a future phase. This keeps the demanding DOCS-09 accuracy review uncontaminated by visual-review concerns.

### D-02 — Accuracy is THE gate (DOCS-09 is non-negotiable)
Every security/behavioral claim must trace to a source. MDX files deriving content from a Go-side doc carry `source_doc:` frontmatter pointing at the authoritative file (e.g. `docs/THREAT-MODEL.md`, `docs/harness-support-matrix.md`, `docs/nudge.md`, `CLAUDE.md`, or a specific `cmd/`/`internal/` path). All content is reviewed against `docs/THREAT-MODEL.md` before publish. Unenforced features are explicitly labeled "not enforced in v1.3.0" — specifically `release_age`/`minimumReleaseAge` and `lifecycle_script_allowlist` (these are documented-but-informational, never blocked). Document the binary **as shipped**.

### D-03 — Honesty at point-of-use (reuse the established framing)
Caveats live where the user reads them, not buried in a footnote. Mandatory point-of-use caveats:
- **Hermes fail-open** — surfaced in the integration guide for Hermes (Tier-2).
- **Tier-3 unguarded** — Kilo / Trae have no enforceable hook; their integration guidance says "UNGUARDED" honestly (per `docs/harness-support-matrix.md`).
- **Harness tiers** — Tier-1 *testable* = Claude Code only; 9 others are Tier-1 *documented*; do not overclaim universal protection.
- **`--bind 0.0.0.0`** gateway exposure caveat in the MCP gateway docs.
- **exit-1 → exit-2 hook history** — referenced in security/troubleshooting where relevant (already flagged in the v1.3.0 changelog; do not duplicate, link).
Reuse the honest voice already shipped in the marketing home and changelog.

### D-04 — Source-of-truth precedence
Authoritative, in order: the actual Go code (`cmd/beekeeper/*.go`, `internal/**`) and the Go-side `docs/*.md`. The CLI reference (DOCS-07) is authored by reading the **real cobra command definitions and flags**, not from memory. Where `docs/THREAT-MODEL.md` and any prior marketing claim disagree, the threat model wins.

### D-05 — Process: discuss skipped, research-first, plan + execute inline on main
discuss-phase skipped (maintainer choice, Phase 13–17 precedent). **Research-first chosen** — the researcher's job here is a *ground-truth content source-map*: the real subcommand/flag tree, the enforced-vs-unenforced feature list, every claim's `source_doc:`, and the honest caveats — exactly what the DOCS-09 accuracy gate consumes. No UI-SPEC (D-01). Execution will run **inline on main** (subagents lack node/pnpm — the executor needs a live `pnpm build`); research/plan/plan-check run as subagents (they only read files + write markdown).

### Claude's Discretion
- **Per-section depth, structure, and sub-page splitting** — e.g. whether `cli-reference` becomes one page or a page-per-command-group, whether `integration` splits per-harness, via `meta.json`. SC-4 requires "all subcommands and flags," so the CLI reference must be exhaustive, but plumbing/internal subcommands (e.g. `ipc`, `shim`, `pkgparse`, `editorinit`, `platform`) may be grouped/condensed under a clearly-labeled section rather than given equal weight to user-facing ones.
- **Whether to add a small reusable content-callout MDX component** (e.g. an "Unenforced in v1.3.0" / "Caveat" callout) — reuse the Phase-14 `breaking-change-callout.tsx` pattern and register it in `mdx-components.tsx` if it earns its keep; otherwise plain MDX admonitions.
- **Example command selection** for copyable snippets (must be real, runnable, and match the shipped flags).
- **Verification mechanism for the accuracy gate** — recommended: extend the existing pure-Python `web/tests/*_spec.py` harness with an `accuracy_spec.py` that file-walks `content/docs/**` asserting (a) every Go-derived section has `source_doc:` frontmatter, (b) unenforced-feature labels are present where those features are mentioned, (c) no references to unshipped behavior, and that `pnpm build` stays green — PLUS a human/agent accuracy pass against `docs/THREAT-MODEL.md`. Final mechanism is the researcher's + planner's call.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents (researcher, planner, executor) MUST read these before working.**

### Phase scope & requirements
- `.planning/ROADMAP.md` — Phase 18 section (goal, SC-1..5, Depends on Phase 13/14/15)
- `.planning/REQUIREMENTS.md` — DOCS-02 through DOCS-09 (full text)

### Authoritative product truth (source_doc: targets)
- `docs/THREAT-MODEL.md` — **the accuracy spine**; every security claim is reviewed against this before publish
- `docs/harness-support-matrix.md` — the 15-harness honesty doc (Tier-1 testable = Claude Code; Tier-2 = Hermes/Cline/OpenCode; Tier-3 = Kilo/Trae UNGUARDED)
- `docs/nudge.md` — package-manager nudge behavior (DOCS-04); note `minimumReleaseAge` baseline = 1440 min and is informational/unenforced
- `docs/release-runbook.md` — release/signing flow (DOCS-03 cosign + SLSA verification)
- `CLAUDE.md` — locked architecture decisions, corroboration thresholds, fail-closed default, self-defense
- `cmd/beekeeper/*.go` + `internal/**` — the real CLI surface for DOCS-07 (subcommands: check, catalog, config, hooks, gateway, scan, audit, baseline, nudge, policy, quarantine, sentry, tui, watch, version, protect, llamafirewall, notify, + plumbing ipc/shim/pkgparse/editorinit/platform)
- `README.md` — honest headline framing (no overclaiming universal protection)

### web/ docs stack & conventions (do not regress)
- `web/content/docs/**/index.mdx` + `web/content/docs/**/meta.json` — the 8 stubs to replace and their nav ordering: getting-started, installation, configuration, security, integration, cli-reference, troubleshooting, audit-log
- `web/content/docs/meta.json` — top-level docs sidebar order
- `web/source.config.ts` + `web/lib/source.ts` — fumadocs-mdx docs collection + loader (frontmatter schema lives here if `source_doc:` needs registering)
- `web/mdx-components.tsx` — where MDX components are registered (e.g. a new caveat callout)
- `web/components/changelog/breaking-change-callout.tsx` — the reusable callout pattern to mimic for any content callout
- `web/content/changelog/v1.3.0/index.mdx` — the already-shipped honest framing + the exit-1→exit-2 breaking-change callout (link, don't duplicate)
- `beekeeper-docs.html` (repo root) — brand/visual reference (for callout styling only, if a component is added — NOT a theme redesign)

### Process / tracking
- `.planning/STATE.md` — hand-managed tracking (do NOT trust phase-number SDK state verbs; frontmatter-corruption caveat applies)
- `.planning/phases/17-seo-static-assets/17-CONTEXT.md` — format precedent for this file
- `.planning/todos/pending/docs-styling-polish.md` — the DEFERRED styling concern (out of scope here; do not action)

</canonical_refs>

<specifics>
## Specific Ideas

- The 8 docs sections already exist with frontmatter (`title`, `description`) and correct `meta.json` wiring — Phase 18 **expands the body**, adds `source_doc:` where applicable, and may add sub-pages. Do not break the existing Fumadocs routing.
- DOCS-05 explicitly requires security posture **and** known gaps **presented together** (co-located) — not a posture page that hides the limitations elsewhere.
- DOCS-09's "unenforced features" list is concrete: `release_age`/`minimumReleaseAge` and `lifecycle_script_allowlist`. Memory + STATE confirm these are informational-only in the shipped binary.
- The nudge is **advise/warn by default** but gained a detection-independent `mode=block` (DENY npm/yarn install, offer pnpm+bun) — DOCS-04 must describe both modes accurately.
- Self-defense is shipped (config-as-secret: agent can't read/write StateDir, overwrite binary, or remove its hook entry) — DOCS-05 should cover it accurately.
- Windows state dir is `%APPDATA%/beekeeper`, NOT `~/.beekeeper` (the existing configuration stub already gets this right — keep it).

</specifics>

<deferred>
## Deferred Ideas

- **Docs visual styling / Fumadocs theme redesign** — own phase, own UI-SPEC (`.planning/todos/pending/docs-styling-polish.md`).
- **Auto-generated CLI reference from the Go binary** — v1.3.0 is hand-authored MDX (REQUIREMENTS "Future"); auto-gen deferred.
- **Versioned docs / i18n / blog / playground** — out of milestone scope (REQUIREMENTS).
- **SITE-03 live Vercel deploy** — still deferred (separate track).

</deferred>

---

*Phase: 18-full-content-authoring*
*Context gathered: 2026-06-09 via inline capture (discuss-phase skipped; research-first chosen)*
