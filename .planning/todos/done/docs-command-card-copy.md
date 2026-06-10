---
type: todo
status: pending
captured: 2026-06-09
source: maintainer (Phase 18.1 review)
area: web/ docs (Fumadocs MDX content)
priority: low
---

# Docs: one-command-per-card copy for multi-command command-field blocks

**What:** In the docs, multi-command fenced blocks render as a SINGLE Fumadocs code card with ONE copy button that copies ALL the commands at once. For lists of *alternative* commands (where the reader picks one), copy-all is wrong — each independent command should be its own card with its own copy button.

**Where (the offenders, as of Phase 18 content):**
- `web/content/docs/cli-reference.mdx` — the `catalogs` group, the `audit` group, the `config set` 5-key list, the `hooks install`/`uninstall` pair, the `gateway` group, etc. (each is one fenced ```bash block with multiple commands).
- `web/content/docs/integration.mdx` — the Tier-1 "documented" install list (`--target codex` / `cursor` / `augment` / …).

**Fix:** split each independent/alternative command into its OWN fenced block (Fumadocs gives each block its own copy button). Genuine run-in-order sequences (e.g. the getting-started quickstart steps) may stay grouped. Optionally introduce a small "command card" MDX component if a denser treatment is wanted.

**Why deferred:** Phase 18.1 (Docs Theme Restyle, DSYS-05) was closed on the three visual gripes (white border, no accents, sidebar duplication) per maintainer "mark complete to move on." This copy-behavior refinement is a content re-chunk, captured separately. Maintainer flagged it as important during the Phase-18 review — do not drop it.

**Suggested home:** a quick task (`/gsd-quick`-style, inline) over cli-reference.mdx + integration.mdx; or fold into Phase 19 prep. Decide at backlog review.

---

## ✅ RESOLVED 2026-06-10

Split each fenced block of **pick-one / alternative** commands into one fenced
block per command (Fumadocs renders each as its own card with its own copy
button), preserving inline `# comments`. Genuine **sequences** and benign
read-only **gather-sets** were deliberately left grouped (the rule exempts
run-in-order steps; copy-all of those is correct, not harmful).

**Split (alternatives — copy-all would have produced a wrong combined command):**
- `cli-reference.mdx` — catalogs, audit, quarantine, hooks (install/uninstall),
  gateway, shim, protect, sentry, llamafirewall (enable/disable/status), policy,
  nudge, and the `config set` 5-key list (12 blocks).
- `integration.mdx` — the Tier-1 "documented" install list (codex/cursor/augment/
  codebuddy/qwen/gemini/copilot), the Tier-3 install list (kilo/trae/continue/
  openclaw), and the MCP-gateway block (run/token/status).
- `configuration.mdx` — the policy validate/test/list block and the `config set`
  5-key block.
- `audit-log.mdx` — the reading (tail vs tail --no-follow), querying (two
  examples), and exporting (ndjson/csv/otlp) blocks.

**Kept grouped (sequence or benign gather-set, NOT pick-one):**
- `installation.mdx` — `git clone` → `cd` → `make build` (build sequence).
- `troubleshooting.mdx` — `go install`→`hooks install` (upgrade sequence),
  `sudo protect install`→`protect status` (install→verify), and the two read-only
  diagnostic gather-sets (`version`/`diag`/`selftest`/`policy validate` to
  investigate; `version`+`diag` to attach to a bug report). Copying these as a set
  is the intended use.

No new MDX component was introduced (the straightforward per-block split is the
faithful fix; the optional "command card" component was not needed). Getting-
started's numbered quickstart steps were already one command per step.

**Verification:** `pnpm build` exit 0; `accuracy_spec` (no phantom commands; the
command set is unchanged, only re-chunked) + `seo_spec` + `home_spec` + `gfx_spec`
all PASS. No Go changed. A post-edit sweep confirms the only remaining
multi-command blocks are the intentionally-kept sequences/gather-sets.
