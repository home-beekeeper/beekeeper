# Phase 15: Marketing Home - Context

**Gathered:** 2026-06-08
**Status:** Ready for planning
**Source:** Maintainer decisions captured during `/gsd-plan-phase 15` (discuss-phase, research, and UI-SPEC all skipped by choice; the `beekeeper-docs.html` mockup IS the design contract)

<domain>
## Phase Boundary

Phase 15 builds the public **marketing home page** for the Beekeeper site (`web/app/page.tsx` + section components), codifying the maintainer's complete mockup (`beekeeper-docs.html`, repo root) on top of the locked Phase-12 design system. Executable scope = **HOME-01..05**, build-verified locally (`pnpm build` → `out/`).

**SITE-03 (the live deploy) is DEFERRED out of this phase.** The page is build-verified locally; the actual deploy + public URL is pending (see D-02).

Static **SVG** hero only — the interactive 3D hero is Phase 16.
</domain>

<decisions>
## Implementation Decisions

### D-01 — Design source: codify the mockup directly
- The authoritative design is `beekeeper-docs.html` (repo root, 1225 lines). Codify it DIRECTLY into React/Next server components. **No separate UI-SPEC.md** (maintainer choice — the mockup is more concrete than a generated spec).
- The Phase-12 design system is the styling foundation: `web/app/globals.css` raw theme tokens (light/dark, `var(--bg/--fg/--amber/--teal/--red/--coral)`), shadcn components, Inter/JetBrains fonts, ReducedMotionProvider. Map the mockup's inline styles onto these tokens — do NOT hardcode hex or reintroduce dark-only `--color-bk-*` (the Phase-12 dual-theme trap).
- Server-rendered static content (no client runtime beyond small interactions like a copy chip). Must work in both light and dark themes.

### D-02 — Deploy target CHANGED: Cloudflare Pages → Vercel; deploy DEFERRED
- SITE-03 host is **retargeted from Cloudflare Pages → Vercel** (maintainer decision 2026-06-08).
- **KEEP Next.js static export (`output:'export'`)** — Vercel serves the static `out/` (via `vercel deploy` or git integration). NO architecture change; Phases 16 (3D), 17 (SEO), 19 (E2E-against-`out/`) are unaffected.
- The **live deploy is DEFERRED** out of Phase 15: build + verify the page locally this phase. The actual Vercel deploy + public URL (SITE-03 / ROADMAP SC-6) is pending repo push (`bantuson/beekeeper` is parked in the v1.1.0 runbook) and Vercel account setup. Phase 15 verifies on the built page, NOT a live URL. The planner does NOT plan the live deploy; it MAY keep `out/` Vercel-deployable and note the deploy steps, but must not add speculative deploy config that isn't needed.

### D-03 — Content accuracy is a trust obligation (no aspirational claims)
- **HOME-03** feature cards cover ONLY the six SHIPPED capabilities: corroboration engine, fail-closed hooks, editor-extension defense, Sentry, LlamaFirewall, policy-as-code. No unshipped/aspirational claims.
- **HOME-04** harness support matrix = all 15 harnesses with HONEST tier labels + live-verification caveats, sourced from `docs/harness-support-matrix.md` (Tier 1 testable = Claude Code live-verified; the rest Tier 1 documented / Tier 2 / Tier 3 unguarded). Links to the integration docs.
- **HOME-05** honesty / known-gaps callout sourced from `docs/THREAT-MODEL.md` + known gaps (Hermes fail-open, Tier-3 unguarded MCP-only, `release_age`/lifecycle-allowlist unenforced in v1.3.0, gateway `--bind 0.0.0.0` caveat). Links to the security-posture docs. No overclaiming.
- **HOME-01** hero: headline + subhead + a copyable `go install github.com/bantuson/beekeeper/cmd/beekeeper@latest` command chip + a "Read the docs" link, all above the fold @1280px. The install command is canonical but not yet resolvable (repo unpushed) — frame honestly, same as the changelog GitHub-release links.
- **HOME-02** origin/problem: the Nx Console compromise story + a 3-step how-it-works flow (the mockup's "Protected in 60 seconds" section).

### D-04 — Reconcile the mockup against required sections
- The mockup covers: hero, feature cards, threat/origin ("Real attacks Beekeeper catches"), "Two layers working together", "Protected in 60 seconds" (3-step), FAQ, CTA banner. The planner MUST verify which HOME requirements the mockup already satisfies and **ADD any missing required section** — in particular HOME-04 (15-harness matrix) and HOME-05 (honesty/known-gaps callout) if the mockup lacks them — styled consistently with the mockup + design system.

### D-05 — Process
- discuss-phase, research, and UI-SPEC all skipped by maintainer choice. Inputs are the mockup, the HOME requirements, and the shipped docs.
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Design
- `beekeeper-docs.html` (repo root) — the complete marketing-home mockup to codify (D-01)
- `.planning/phases/12-design-system/12-UI-SPEC.md` — locked design system contract
- `web/app/globals.css` — token cascade (light/dark; use raw theme tokens, NOT `--color-bk-*`)
- `web/app/layout.tsx`, `web/app/providers.tsx` — root layout, theme/reduced-motion providers

### Content accuracy (cite these; do NOT invent claims)
- `docs/harness-support-matrix.md` — authoritative 15-harness tier table (HOME-04)
- `docs/THREAT-MODEL.md` — security posture + known gaps (HOME-05)
- `README.md` — honest headline / shipped capabilities (HOME-03)
- `web/content/docs/` and `web/content/changelog/` — existing accurate prose to link to

### Patterns to reuse
- `web/app/docs/layout.tsx`, `web/app/changelog/layout.tsx` — layout/nav patterns
- `web/components/ui/` (shadcn button/badge/separator/tooltip)
- `web/components/changelog/verify-commands.tsx` — the CopyButton pattern (for the hero `go install` chip)
</canonical_refs>

<specifics>
## Specific Ideas
- The mockup's section eyebrows already follow the Phase-12 semantic color roles (amber = brand, red = threat, teal = interactive, coral = response).
- The home page replaces the current scaffold `web/app/page.tsx`.
- Header nav should reach Docs (`/docs/getting-started`) and Changelog (`/changelog`) — both exist and resolve (Phase 13/14).
</specifics>

<deferred>
## Deferred Ideas
- **SITE-03** live Vercel deploy + public URL (ROADMAP SC-6) — DEFERRED (pending repo push / Vercel setup). Page is build-verified locally this phase.
- 3D hero (Phase 16); SEO/OG/sitemap/robots (Phase 17); full docs prose (Phase 18); web CI + Playwright E2E (Phase 19).
</deferred>

---

*Phase: 15-marketing-home*
*Context gathered: 2026-06-08 via maintainer decisions during `/gsd-plan-phase 15` (discuss/research/UI-SPEC skipped)*
