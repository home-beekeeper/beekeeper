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
