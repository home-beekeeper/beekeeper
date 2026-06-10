# Requirements: Beekeeper v1.3.0 — Web Presence & Documentation

**Defined:** 2026-06-07
**Core Value:** A hijacked or off-task agent cannot successfully act on the developer's machine without Beekeeper deciding to permit it.
**Milestone goal:** Ship Beekeeper's public-facing Next.js site — a distinctive marketing home and a complete documentation set — that takes a developer from "what is this" to installed, configured, and confident in its security posture.

> Greenfield web app under `web/` (in-repo, pnpm workspace, isolated from the Go module). Static export (Next.js 16 / Tailwind v4 / shadcn/ui / Fumadocs / React-Three-Fiber). See `.planning/research/SUMMARY.md` for the locked stack and pitfalls.

## v1 Requirements (this milestone)

### Site Foundation (SITE)

- [x] **SITE-01**: A developer can run the `web/` app locally (`pnpm dev`) and produce a successful static build (`pnpm build` → `out/`) with no server runtime ✅ Phase 11
- [x] **SITE-02**: The `web/` Node toolchain is isolated from the Go module (pnpm workspace; `pnpm install` never touches the Go root; `.source/` and build artifacts gitignored) ✅ Phase 11
- [ ] **SITE-03**: The static site deploys to a static host (**Vercel** — retargeted from Cloudflare Pages, maintainer decision 2026-06-08) and is reachable at a public URL — *DEFERRED out of Phase 15 (page build-verified locally; live deploy pending repo push / Vercel setup; static export retained)*

### Design System (DSYS)

- [x] **DSYS-01**: The site uses a shadcn/ui + Tailwind v4 design system with correct Fumadocs CSS integration — a single source of design tokens, no theming conflicts
- [x] **DSYS-02**: A visitor can switch between light and dark themes, persisted across visits, with no flash-of-wrong-theme
- [x] **DSYS-03**: The site honors `prefers-reduced-motion` site-wide (a reduced-motion provider gates animation and 3D)
- [x] **DSYS-04**: The UI meets WCAG 2.1 AA (contrast, keyboard navigation, visible focus) across both themes
- [x] **DSYS-05**: The documentation section adopts the Beekeeper design system (brand tokens, typography, chrome) and matches the marketing home's visual quality — dual-theme, reduced-motion, and WCAG-AA safe — without changing content ✅ Phase 18.1 (2026-06-09; quick task — command-card copy refinement deferred to backlog)

### Marketing Home (HOME)

- [x] **HOME-01**: A visitor sees a home hero with headline, subhead, and a dual CTA (copyable `go install` command + "Read the docs") ✅ Phase 15 (2026-06-08)
- [x] **HOME-02**: The home page explains the origin/problem (Nx Console compromise) and how Beekeeper works in 3 steps ✅ Phase 15 (2026-06-08)
- [x] **HOME-03**: The home page presents feature highlights covering only shipped capabilities (corroboration engine, fail-closed hooks, editor-extension defense, Sentry, LlamaFirewall, policy-as-code) ✅ Phase 15 (2026-06-08)
- [x] **HOME-04**: The home page shows the 15-harness support matrix with honest tier/verification caveats, linking to the integration docs ✅ Phase 15 (2026-06-08)
- [x] **HOME-05**: The home page surfaces an honesty / known-gaps callout linking to the security-posture docs (no overclaiming) ✅ Phase 15 (2026-06-08)

### 3D & Motion (GFX)

- [x] **GFX-01**: The home hero features an interactive Three.js centerpiece (hive / agent-mediation visual) loaded behind a client-only boundary that never breaks the static build
- [x] **GFX-02**: Ambient 3D/motion accents enhance marketing sections without harming readability
- [x] **GFX-03**: The 3D layer falls back to a static SVG (LCP-sized) under reduced-motion, low-power, or no-WebGL; the canvas is accessibility-invisible with an sr-only description
- [x] **GFX-04**: The home page meets a performance budget — Lighthouse LCP < 2.5s (SVG as LCP candidate), a bounded 3D bundle, and no leaked WebGL contexts across navigation

### Documentation (DOCS)

- [x] **DOCS-01**: A user can browse a Fumadocs-powered docs site with sidebar navigation, table of contents, and working static (Orama) search ✅ Phase 13 (2026-06-08)
- [x] **DOCS-02**: A new user can follow a Getting Started / Quickstart guide from zero to a working `beekeeper check`
- [x] **DOCS-03**: A user can follow installation docs (`go install` + GitHub Releases + cosign / SLSA verification)
- [x] **DOCS-04**: A user can learn to customize configuration (layered config, policy-as-code, sensitive paths, package-manager nudge) with copyable examples
- [x] **DOCS-05**: A user can understand Beekeeper's security posture (corroboration model, fail-closed defaults, threat model) **and** its known gaps/limitations, presented together
- [x] **DOCS-06**: A user can follow integration guides for supported harnesses (Claude Code / Cursor / Codex hooks, MCP gateway) with honest caveats at point-of-use (Hermes fail-open, Tier-3 unguarded)
- [x] **DOCS-07**: A user can consult a CLI / command reference for `beekeeper` subcommands and flags
- [x] **DOCS-08**: A user can find troubleshooting guidance for common issues
- [x] **DOCS-09**: Documentation is accurate to the shipped binary — every security claim cites its source (`source_doc:`), is reviewed against `docs/THREAT-MODEL.md` before publish, and unenforced features (`release_age`, lifecycle allowlist) are explicitly labeled

### Changelog & Releases (CHG)

- [x] **CHG-01**: A visitor can read a versioned changelog (v1.0.0, v1.2.0, v1.3.0) with human-written release notes ✅ Phase 14 (2026-06-08)
- [x] **CHG-02**: Each release entry includes download + verification (cosign / SLSA / SBOM) guidance ✅ Phase 14 (2026-06-08)
- [x] **CHG-03**: The v1.3.0 entry prominently flags the exit-1 → exit-2 hook breaking change (red callout) ✅ Phase 14 (2026-06-08)

### SEO & Assets (SEO)

- [x] **SEO-01**: Each page emits correct static metadata (title / description / canonical), an OG / social card image, `sitemap.xml`, and `robots.txt`

### Quality & CI (QA)

- [ ] **QA-01**: A path-filtered web CI job (separate from Go CI) builds the static site and runs unit (Vitest) + E2E (Playwright against `out/`) tests + lint/format (Biome), gating merges
- [ ] **QA-02**: E2E tests verify the critical paths — home renders with hero fallback, docs navigation + search returns results, theme toggle, changelog pages build

### Full-System Validation (VAL) — Go core, Phase 21

> Added 2026-06-10. Net-new Go-core validation scope (relaxes the web-only fence for the
> pre-ship release gate). Validation tier model: **Tier A** = locally testable (target 100%,
> gate-enforced) · **Tier B** = platform/kernel/build-tag (CI matrix) · **Tier C** = irreducible
> manual/gated (documented register). Only Claude Code is live-verified; the other 16 harnesses
> get installer + deny-contract conformance locally and a manual live-block register.

- [ ] **VAL-01**: Every Go production file has a linked test OR a documented, reason-coded no-test allowlist entry (pure type/const/build-metadata + platform stubs); a coverage-gate check enforces zero silent gaps and closes the surfaced Tier-A gaps (`ipc` server/client + Windows-pipe peer, `check`/`watch`/`scan`/`gateway` `sanity.go`, `editorinit/lookup.go`, `hooks/protected.go`, TUI model logic)
- [ ] **VAL-02**: A local 17-harness conformance suite asserts, for every target, correct installer config output (keys, idempotency, backup-on-overwrite) and golden-file the exact per-harness deny contract (exit code + JSON/stdout), including the Hermes fail-open seam and Kilo/Trae UNGUARDED honesty
- [ ] **VAL-03**: A cross-platform CI matrix (ubuntu-20.04/kernel-5.4 + ubuntu-22.04/kernel-5.15 + macos-latest + windows-latest) runs build (native + 3 GOOS), vet, test, `-race` (CGO), eBPF generate+load (CI-only bytecode), eslogger (macOS), ETW (Windows), and Unix peer-cred auth — all green
- [ ] **VAL-04**: The fuzz suite (policy engine, IPC proto parser, catalog parser, MCP message parser, Sentry rule evaluator) runs in CI as a blocking release gate
- [ ] **VAL-05**: A Claude Code live end-to-end test proves a real block (canary `~/.ssh` + `~/.aws` read DENIED end-to-end) as the documented reference
- [ ] **VAL-06**: `docs/validation-register.md` enumerates all 16 non-Claude-Code harness live-block procedures + the LlamaFirewall gated-22M-model e2e, each with exact steps, expected result, and a sign-off — the documented release gate for what cannot be automated
- [ ] **VAL-07**: Documentation honesty — README harness count corrected (15→17 / 14→16) and the validation posture (no-test allowlist + Tier A/B/C model) documented so the coverage claim is auditable
- [ ] **VAL-08**: Self-defense — the coverage gate cannot be silently weakened (allowlist growth requires a reason code and fails closed otherwise); fuzz-gate failures block release

## Future Requirements (deferred)

- Interactive in-browser playground / tool-call decision demo (deferred — sandboxing an OS-hook security tool is infeasible and trust-risky)
- Per-page dynamic OG images (`next/og`) — single static OG image in v1.3.0; per-page deferred (no runtime in static export)
- Versioned docs (multiple product versions) — single current version for v1.3.0
- i18n / multi-language docs
- Community / blog section
- Auto-generated CLI reference from the Go binary (hand-authored MDX in v1.3.0)

## Out of Scope (explicit exclusions)

- **Interactive demo / playground** — cannot safely sandbox a filesystem + OS-hook tool in the browser; a fake demo destroys trust for a security product
- **Blog** — out of milestone scope
- **AI chatbot over docs** — hallucination risk is unacceptable for security content
- **SSR / ISR / server runtime** — static export only (locked)
- **CMS / headless backend** — content is MDX in-repo
- **Changing the Go product** — web phases (11–19) are web-only and do not modify Go behavior. **Exception: Phase 21 (Full-System Validation)** adds Go test coverage + a cross-platform CI matrix and MAY apply coverage-surfaced fixes, as the pre-ship release gate (added 2026-06-10)

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| SITE-01 | Phase 11 — Scaffold & Toolchain Isolation | ✅ Complete (2026-06-07) |
| SITE-02 | Phase 11 — Scaffold & Toolchain Isolation | ✅ Complete (2026-06-07) |
| SITE-03 | Phase 15 — Marketing Home (deploy deferred; retargeted Cloudflare→Vercel) | Deferred (page in Phase 15; live deploy pending) |
| DSYS-01 | Phase 12 — Design System | ✅ Complete (2026-06-08) |
| DSYS-02 | Phase 12 — Design System | ✅ Complete (2026-06-08) |
| DSYS-03 | Phase 12 — Design System | ✅ Complete (2026-06-08) |
| DSYS-04 | Phase 12 — Design System | ✅ Complete (2026-06-08) |
| DSYS-05 | Phase 18.1 — Docs Theme Restyle | ✅ Complete (2026-06-09; quick task; command-card copy split → backlog) |
| HOME-01 | Phase 15 — Marketing Home | ✅ Complete (2026-06-08) |
| HOME-02 | Phase 15 — Marketing Home | ✅ Complete (2026-06-08) |
| HOME-03 | Phase 15 — Marketing Home | ✅ Complete (2026-06-08) |
| HOME-04 | Phase 15 — Marketing Home | ✅ Complete (2026-06-08) |
| HOME-05 | Phase 15 — Marketing Home | ✅ Complete (2026-06-08) |
| GFX-01 | Phase 16 — 3D Layer | ✅ Complete (2026-06-09) |
| GFX-02 | Phase 16 — 3D Layer | ✅ Complete (2026-06-09) |
| GFX-03 | Phase 16 — 3D Layer | ✅ Complete (2026-06-09) |
| GFX-04 | Phase 16 — 3D Layer | ✅ Complete (2026-06-09) |
| DOCS-01 | Phase 13 — Docs Content Pipeline | ✅ Complete (2026-06-08) |
| DOCS-02 | Phase 18 — Full Content Authoring | ✅ Complete (2026-06-09) |
| DOCS-03 | Phase 18 — Full Content Authoring | ✅ Complete (2026-06-09) |
| DOCS-04 | Phase 18 — Full Content Authoring | ✅ Complete (2026-06-09) |
| DOCS-05 | Phase 18 — Full Content Authoring | ✅ Complete (2026-06-09) |
| DOCS-06 | Phase 18 — Full Content Authoring | ✅ Complete (2026-06-09) |
| DOCS-07 | Phase 18 — Full Content Authoring | ✅ Complete (2026-06-09) |
| DOCS-08 | Phase 18 — Full Content Authoring | ✅ Complete (2026-06-09) |
| DOCS-09 | Phase 18 — Full Content Authoring | ✅ Complete (2026-06-09) |
| CHG-01 | Phase 14 — Changelog Pipeline | ✅ Complete (2026-06-08) |
| CHG-02 | Phase 14 — Changelog Pipeline | ✅ Complete (2026-06-08) |
| CHG-03 | Phase 14 — Changelog Pipeline | ✅ Complete (2026-06-08) |
| SEO-01 | Phase 17 — SEO & Static Assets | ✅ Complete (2026-06-09) |
| QA-01 | Phase 19 — Test Suite & CI | Planned & verified 2026-06-10 (web) |
| QA-02 | Phase 19 — Test Suite & CI | Planned & verified 2026-06-10 (web) |
| VAL-01 | Phase 21 — Full-System Validation & CI Calibration | Not started |
| VAL-02 | Phase 21 — Full-System Validation & CI Calibration | Not started |
| VAL-03 | Phase 21 — Full-System Validation & CI Calibration | Not started |
| VAL-04 | Phase 21 — Full-System Validation & CI Calibration | Not started |
| VAL-05 | Phase 21 — Full-System Validation & CI Calibration | Not started |
| VAL-06 | Phase 21 — Full-System Validation & CI Calibration | Not started |
| VAL-07 | Phase 21 — Full-System Validation & CI Calibration | Not started |
| VAL-08 | Phase 21 — Full-System Validation & CI Calibration | Not started |

**Coverage:** 32/32 requirements mapped — 100% coverage, no orphans. *(DSYS-05 added 2026-06-09 with Phase 18.1.)*
