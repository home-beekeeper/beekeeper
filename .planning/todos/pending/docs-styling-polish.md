---
type: todo
status: promoted
captured: 2026-06-09
promoted: 2026-06-09
promoted_to: Phase 18.1 — Docs Theme Restyle (DSYS-05)
source: maintainer (Phase 17 live review)
area: web/ docs (Fumadocs)
priority: medium
---

> **✅ PROMOTED 2026-06-09 → Phase 18.1 "Docs Theme Restyle" (DSYS-05).** Inserted into the v1.3.0 roadmap between Phase 18 (content) and Phase 19 (CI) on maintainer request. See `.planning/ROADMAP.md` Phase 18.1 and `.planning/phases/18.1-docs-theme-restyle/`. This file is retained for the capture/audit trail.

# Docs styling looks old / basic — polish the Fumadocs docs theme

**What:** During the Phase 17 SEO live review (localhost:3000), the maintainer flagged that the **Docs section styling "looks old and basic"** compared to the polished marketing home.

**Context:** The docs UI is the stock Fumadocs `DocsLayout` theme from Phase 13 (Docs Content Pipeline). It was wired functionally (sidebar, TOC, static search) but never given a design pass to match the Beekeeper design system (Phase 12 tokens: dark-first GitHub-dark palette, amber #e3b341 brand / teal #39c5cf interactive, Inter + JetBrains Mono, 1180px/60px chrome) or the marketing home's visual quality.

**Scope (not yet planned):** Customize the Fumadocs theme to match the brand — sidebar/nav styling, typography scale, code-block treatment, link/heading colors, spacing, and overall polish. Reuse the locked Phase-12 tokens and the marketing-home component language. Keep it dual-theme + reduced-motion safe (the Phase-12 `@theme inline` raw-token rule, the WCAG-AA constraints).

**NOT in scope here:** content authoring (that's Phase 18) — this is purely the docs *visual styling/theme*.

**Suggested home:** a dedicated docs-polish phase (via `/gsd-phase --insert` + `/gsd-plan-phase`), or fold into Phase 18 (Full Content Authoring) as a styling track. Decide at backlog review.

**Captured during:** Phase 17 (SEO & Static Assets) finalization — explicitly deferred ("just capture it for later") so it did not mix a docs redesign into the SEO phase.
