# Roadmap: Beekeeper v1.3.0 — Web Presence & Documentation

> Phases 1–5 = parked v1.1.0 Pollen; Phase 10 = shipped v1.3.0 seed — preserved by the orchestrator below / in git history.

## Milestones

- ✅ **v1.0.0 — Comprehensive Standalone Release** — Phases 1–11 (shipped 2026-06-01)
  Full per-phase detail archived in [`milestones/v1.0.0-ROADMAP.md`](milestones/v1.0.0-ROADMAP.md).
  Audit: PASSED — [`milestones/v1.0.0-MILESTONE-AUDIT.md`](milestones/v1.0.0-MILESTONE-AUDIT.md).
  Summary: [`MILESTONES.md`](MILESTONES.md).

- 🔄 **v1.1.0 — "Pollen"** — Phases 1–5 (PARKED at maintainer release checkpoint)
  Goal: Own Windows inventory compatibility via a bounded Apache-2.0 Bumblebee derivative so the Windows CI matrix goes fully green.

- ✅ **v1.2.0 — "Runtime Behavioral Hardening"** — Phases 6–9 (shipped 2026-06-04)
  Full per-phase detail archived in [`milestones/v1.2.0-ROADMAP.md`](milestones/v1.2.0-ROADMAP.md).
  Summary: [`MILESTONES.md`](MILESTONES.md).

- 🚧 **v1.3.0 — "Web Presence & Documentation"** — Phases 10–19 (in progress)
  Phase 10 (seed: hook-block + self-protection + $VAR hardening) shipped 2026-06-05.
  Phases 11–19: greenfield Next.js site under `web/`.

## Phases

<details>
<summary>✅ v1.0.0 Comprehensive Standalone Release (Phases 1–11) — SHIPPED 2026-06-01</summary>

- [x] Phase 1: Foundation + Hook Handler (6/6 plans)
- [x] Phase 2: Policy Engine + Multi-Source Catalogs (9/9)
- [x] Phase 3: Editor Extension Defense (5/5)
- [x] Phase 4: Integration Surfaces (5/5)
- [x] Phase 5: Linux Sentry (5/5)
- [x] Phase 6: LlamaFirewall + Audit Sinks (5/5)
- [x] Phase 7: Cross-Platform Sentry (5/5)
- [x] Phase 8: TUI Dashboard (9/9)
- [x] Phase 9: Policy as Code + Self-Defense Capstone (5/5)
- [x] Phase 10: Cross-Phase Integration Closure (1/1)
- [x] Phase 11: v1.0.0 PRD-Gap Closure (pre-push) (1/1)

</details>

### v1.1.0 "Pollen" — Windows Inventory Compatibility (PARKED)

- [x] **Phase 1: Fork Setup & Discipline** — tag `v0.1.1-pollen.1` shipped
- [~] **Phase 2: Windows Root Resolver** — code complete; tag deferred to M2 close
- [~] **Phase 3: Windows Path Representation** — code complete & verified 2026-06-02; tag deferred
- [~] **Phase 4: Windows Extension & MCP Coverage** — code complete & verified 2026-06-02; tag deferred
- [ ] **Phase 5: Contribution-Back & Milestone Close** — not started (parked)

<details>
<summary>✅ v1.2.0 Runtime Behavioral Hardening (Phases 6–9) — SHIPPED 2026-06-04</summary>

- [x] Phase 6: Corroboration Severity Hardening (3/3 plans)
- [x] Phase 7: Sensitive-Path Runtime Enforcement (3/3)
- [x] Phase 8: Package-Manager Nudge + Behavioral Test Suite (8/8)
- [x] Phase 9: v1.2.0 Tech-Debt Cleanup (5/5 +1 fix)

Full detail: [`milestones/v1.2.0-ROADMAP.md`](milestones/v1.2.0-ROADMAP.md).

</details>

### v1.3.0 "Web Presence & Documentation"

- [x] **Phase 10: Hook-Block Protocol Compliance & Multi-Harness Enforcement** — SHIPPED 2026-06-05 (v1.3.0 seed)
- [ ] **Phase 11: Scaffold & Toolchain Isolation** — pnpm workspace, Next.js static-export app, Cloudflare Pages deploy
- [ ] **Phase 12: Design System** — shadcn/ui + Tailwind v4 + Fumadocs CSS + theme toggle + reduced-motion
- [ ] **Phase 13: Docs Content Pipeline** — fumadocs-mdx wiring, static Orama search, DocsLayout, first MDX
- [ ] **Phase 14: Changelog Pipeline** — versioned changelog route with breaking-change callout renderer
- [ ] **Phase 15: Marketing Home** — all marketing sections, static SVG hero, dual CTA, harness matrix
- [ ] **Phase 16: 3D Layer** — R3F hero + ambient accents behind dynamic(ssr:false), perf/a11y gates
- [ ] **Phase 17: SEO & Static Assets** — sitemap, robots.txt, finalized metadata, OG image
- [ ] **Phase 18: Full Content Authoring** — all docs + changelog prose, accuracy review gate
- [ ] **Phase 19: Test Suite & CI** — path-filtered web.yml, Vitest unit tests, Playwright E2E against out/

## Phase Details

### Phase 1: Fork Setup & Discipline

**Goal**: The `github.com/bantuson/pollen` module exists with correct Apache-2.0 attribution, renamed binary, reproducible builds + Sigstore signing, and CI that guards every subsequent change with a differential test and selftest on all three OSes. No Windows functionality yet — this phase proves fork hygiene before any Windows code lands.
**Repo locus**: Primarily `bantuson/pollen` (new repo). Beekeeper CI is not affected this phase.
**Depends on**: Nothing (first phase)
**Requirements**: FORK-01, FORK-02, FORK-03, FORK-04, PTEST-02, PTEST-03, SDEF-02
**Success Criteria** (what must be TRUE):

  1. `pollen` binary builds and runs on ubuntu/macos/windows from `go install github.com/bantuson/pollen/cmd/pollen@v0.1.1-pollen.1`; `pollen selftest` exits 0 on all three OSes
  2. The CI matrix (ubuntu/macos/windows, Go 1.25.x) runs `go vet`, `go test -race ./...`, and selftest green; the differential test asserts byte-for-byte identical NDJSON output between Pollen and upstream Bumblebee on Linux and macOS
  3. `LICENSE` is verbatim Apache-2.0; `NOTICE` names Perplexity/Bumblebee as origin; `CHANGES.md` records every delta; `UPSTREAM.md` records the pinned 40-char SHA with tag + date + verifier
  4. "Bumblebee" does not appear in any command name, package name, or README headline — only in NOTICE, README attribution paragraph, and UPSTREAM.md
  5. The `v0.1.1-pollen.1` GitHub release carries a Sigstore/cosign signature and a CycloneDX SBOM recording the upstream pinned commit

**Plans**: 5 plans (4 waves)

- [x] 01-01-PLAN.md — Wave 0: fork upstream @ pinned SHA, rewrite module path, rename cmd/bumblebee→cmd/pollen, trademark fixes, build + Windows cross-compile + selftest (FORK-01, FORK-04)
- [x] 01-02-PLAN.md — Wave 1: Apache-2.0 attribution (LICENSE/NOTICE/CHANGES/UPSTREAM), VERSION, empty threat_intel, full-repo trademark audit (FORK-02, FORK-04)
- [x] 01-03-PLAN.md — Wave 1: NDJSON normalization harness + TestDifferential vs pinned upstream + selftest 3-finding regression (PTEST-02, PTEST-03)
- [x] 01-04-PLAN.md — Wave 2: reproducible Makefile + goreleaser (cosign + CycloneDX SBOM), 3-OS CI matrix + differential + govulncheck, release.yml SLSA L3, THREAT-MODEL (FORK-03, SDEF-02)
- [x] 01-05-PLAN.md — Wave 3: create bantuson/pollen repo, green CI, tag + signed v0.1.1-pollen.1 release, verify signature + SBOM (FORK-03, SDEF-02)

### Phase 2: Windows Root Resolver

**Goal**: Pollen can discover all 8 package-manager roots on Windows using Windows environment variables, with the cross-platform parity test asserting equivalent detection counts against Linux.
**Repo locus**: Primarily `bantuson/pollen` — `cmd/pollen/roots_windows.go`
**Depends on**: Phase 1
**Requirements**: WRES-01, WRES-02, PTEST-01
**Success Criteria** (what must be TRUE):

  1. On a Windows CI runner with the standard fake-package fixture tree, `pollen scan` returns inventory records for all 8 ecosystems with non-empty paths
  2. The cross-platform parity test passes on all three OSes: same packages detected, same severity matches, equivalent record counts
  3. The differential test continues to pass on Linux and macOS
  4. `v0.1.1-pollen.2` is tagged and signed; Windows CI no longer skips root-resolver tests

**Plans**: 4 plans (3 waves)

- [x] 02-01-PLAN.md — Wave 1: roots_windows.go (8-ecosystem Windows root table) (WRES-01, WRES-02)
- [x] 02-02-PLAN.md — Wave 2: roots_windows_test.go + flip the 6 Phase-2 skips (WRES-01, WRES-02)
- [x] 02-03-PLAN.md — Wave 2: parity_test.go + testdata/parity-fixture/ 8-ecosystem fixture (PTEST-01)
- [~] 02-04-PLAN.md — Wave 3: VERSION bump 0.1.1-pollen.2 + CHANGES.md (tag deferred to M2 close)

### Phase 3: Windows Path Representation

**Goal**: Every NDJSON record emitted by Pollen on Windows carries native Windows paths — backslash separators, drive letters, `endpoint.os="windows"`, correct `arch` and `username`, and empty `uid`.
**Repo locus**: `bantuson/pollen` — `internal/ecosystem/npm/npm.go`, `internal/endpoint/endpoint.go`
**Depends on**: Phase 2
**Requirements**: WPATH-01, WPATH-02
**Success Criteria** (what must be TRUE):

  1. A Windows CI `pollen scan` run produces NDJSON where `project_path` and `source_file` contain backslash separators and drive letters
  2. The `endpoint` record on Windows contains `os="windows"`, `arch` matching `runtime.GOARCH`, non-empty `username`, and empty `uid`
  3. Beekeeper's audit-log consumer parses a Windows-shaped Pollen NDJSON record without error
  4. `v0.1.1-pollen.3` is tagged and signed (deferred to M2 close)

**Plans**: 3 plans (2 waves)

- [x] 03-01-PLAN.md — Wave 1 (Pollen): WPATH-01 — filepath.FromSlash + Windows-gated unit tests
- [x] 03-02-PLAN.md — Wave 1 (Pollen): WPATH-02 — empty uid on Windows guard + endpoint tests
- [x] 03-03-PLAN.md — Wave 2 (both repos): Windows path-shape assertions, beekeeper round-trip test, VERSION/CHANGES bump

### Phase 4: Windows Extension & MCP Coverage + Beekeeper Compat Test

**Goal**: Pollen enumerates all Windows editor-extension directories, browser-extension profile paths, and MCP host-config files; and beekeeper's Pollen compatibility test runs on all three OSes with zero skips.
**Repo locus**: `bantuson/pollen` + beekeeper `internal/scan/`
**Depends on**: Phase 3
**Requirements**: WEXT-01, WEXT-02, WEXT-03, BKINT-01, PTEST-04
**Success Criteria** (what must be TRUE):

  1. On Windows CI, `pollen scan` detects fake VS Code family extensions under `%USERPROFILE%` fixture trees
  2. On Windows CI, `pollen scan` detects fake Chrome/Chromium/Edge/Brave and Firefox browser extensions
  3. On Windows CI, `pollen scan` finds fake MCP config files at Claude, Cursor, Windsurf, Cline, and Gemini CLI paths
  4. Beekeeper's Pollen compatibility test runs green on ubuntu/macos/windows with zero `t.Skip` calls
  5. `v0.1.1-pollen.4` is tagged and signed (deferred to M2 close)

**Plans**: 3 plans (2 waves)

- [x] 04-01-PLAN.md — Wave 1 (Pollen): WEXT-01/02/03 — browser + MCP roots + vscode-oss editor segment
- [x] 04-02-PLAN.md — Wave 1 (beekeeper): BKINT-01 rename + PTEST-04 TestPollenCompatibility
- [~] 04-03-PLAN.md — Wave 2 (Pollen): VERSION bump 0.1.1-pollen.4 + CHANGES.md (tag deferred)

**UI hint**: no

### Phase 5: Contribution-Back & Milestone Close

**Goal**: Windows additions are prepared as upstream-shaped PRs against `perplexityai/bumblebee`; beekeeper's full CI matrix is green on all three OSes; `pollen-self` entries protect against compromised Pollen releases; the upstream sync workflow is documented and operational.
**Repo locus**: `bantuson/pollen` + beekeeper
**Depends on**: Phase 4
**Requirements**: SYNC-01, SYNC-02, BKINT-02, PTEST-05, SDEF-01
**Success Criteria** (what must be TRUE):

  1. `UPSTREAM.md` documents a repeatable, step-by-step sync workflow a second maintainer could follow without prior context
  2. At least one upstream-shaped PR is open or contribution-back is documented as deferred with rationale
  3. Beekeeper's `go.mod` pins Pollen at an explicit version; all inventory-related tests pass on ubuntu/macos/windows with zero skips
  4. The Windows Sentry honeypot E2E test fires beekeeper's exfil-signature-fusion rule on the Windows CI runner
  5. `beekeeper-self` catalog contains `pollen-self` entries; `beekeeper selftest` passes with the extended catalog
  6. `v0.1.1-pollen.5` is tagged and signed — the milestone-complete tag

**Plans**: 5 plans (3 waves)

- [x] 05-01-PLAN.md — Wave 1 (beekeeper): PTEST-05 — Windows honeypot E2E (TestHoneypotExfilFusion)
- [x] 05-02-PLAN.md — Wave 1 (beekeeper): SDEF-01 — pollen-self entries in beekeeper-self catalog
- [x] 05-03-PLAN.md — Wave 1 (pollen): SYNC-01 UPSTREAM.md sync workflow; SYNC-02 contribution-back-deferred rationale
- [x] 05-04-PLAN.md — Wave 2 (beekeeper): BKINT-02 — CI go install Pollen pin + D-5 release runbook
- [ ] 05-05-PLAN.md — Wave 3 (CHECKPOINT, autonomous:false): maintainer pushes both repos + cuts four signed tags

> **SC2 relaxed (D-2):** No upstream contribution-back PRs against perplexityai/bumblebee this milestone. SYNC-02 is satisfied-by-documented-deferral in UPSTREAM.md (05-03).

### Phase 10: Hook-Block Protocol Compliance & Multi-Harness Enforcement

> **v1.3.0 seed — SHIPPED 2026-06-05.** Continues the live `.planning/phases/` numbering from v1.2.0 (9).

**Goal**: Beekeeper's PreToolUse hook actually blocks denied tool calls across supported agent harnesses — not merely detects and audits them. A live dogfood (2026-06-05) proved the shipped hook fires but the harness runs the tool anyway (exit 1 vs the required exit 2 / deny JSON). This phase adds the `beekeeper check --hook <harness>` deny adapter, fixes 15-harness installers, routes no-hook harnesses to the MCP gateway, and adds the missing release gate.
**Repo locus**: `internal/check`, `cmd/beekeeper`, `internal/hooks/*`, `internal/gateway`, `docs/`
**Depends on**: Phase 9 (shipped)
**Requirements**: HPC-01, HPC-02, HPC-03, HPC-04, HPC-05, HPC-06
**Success Criteria** (what must be TRUE):

  1. On Claude Code, a credential-read tool call is BLOCKED live (tool does not execute), verified end-to-end
  2. `beekeeper check --hook <harness>` emits the correct deny signal (exit 2 + per-harness JSON) for each Tier-1 harness, proven by unit tests
  3. Installers write the correct event names + config + feature flags per harness and never clobber a user's existing hooks
  4. No-hook harnesses (Kilo, Trae) documented + routed to the MCP gateway; OpenCode plugin shipped; Hermes/Cline/Windows caveats documented; Tier 1/2/3 support matrix published
  5. A release-gate test asserts the harness deny contract (exit 2 / deny JSON)

**Plans**: 6 plans across 5 waves

- [x] 10-01-PLAN.md — RenderDeny pure adapter + --hook flag + fixed Claude installer + deny-contract regression gate (HPC-01/03/06)
- [x] 10-02-PLAN.md — Live Claude Code end-to-end block re-proof (HPC-04)
- [x] 10-03-PLAN.md — Fix Cursor event-name bug + Codex features flag; add Augment/CodeBuddy/Qwen installers (HPC-02/03)
- [x] 10-04-PLAN.md — Copilot/Gemini/Antigravity/Windsurf installers, non-Claude deny families (HPC-02/03)
- [x] 10-05-PLAN.md — Hermes (fail-open JSON-only) + Cline (no-Windows) + OpenCode plugin (HPC-02/03/05)
- [x] 10-06-PLAN.md — Kilo/Trae MCP-gateway routing + honest Tier 1/2/3 support matrix (HPC-05)

---

## v1.3.0 Web Presence & Documentation — Phase Details (Phases 11–19)

### Phase 11: Scaffold & Toolchain Isolation

**Goal**: A developer can run and build the `web/` Next.js app locally, and the Node toolchain is fully isolated from the Go module with zero cross-contamination.
**Depends on**: Phase 10 (v1.3.0 seed shipped)
**Requirements**: SITE-01, SITE-02
**Success Criteria** (what must be TRUE):
  1. `pnpm dev` starts in `web/` and serves a page at localhost without errors
  2. `pnpm build` completes and emits a non-empty `web/out/` directory with `index.html`
  3. `pnpm install` in `web/` never modifies `go.mod`, `go.sum`, or any Go-module file
  4. `.next/`, `out/`, `.source/`, and `node_modules/` are all gitignored; no build artifacts appear in `git status`
**Plans**: TBD
**UI hint**: yes

### Phase 12: Design System

**Goal**: The site has a unified design system — shadcn/ui + Tailwind v4 + Fumadocs CSS — with a working theme toggle, reduced-motion gate, and WCAG 2.1 AA compliance verified in both themes.
**Depends on**: Phase 11
**Requirements**: DSYS-01, DSYS-02, DSYS-03, DSYS-04
**Success Criteria** (what must be TRUE):
  1. `pnpm build` succeeds with the correct `globals.css` import order (tailwindcss → shadcn.css → preset.css → @source → @theme) and Fumadocs components render in correct colors in both light and dark themes
  2. A visitor can switch between light and dark themes via the toggle; the chosen theme persists across page reloads with no flash-of-wrong-theme
  3. Setting `prefers-reduced-motion: reduce` in browser preferences disables all animation and 3D site-wide (verified by the ReducedMotionProvider)
  4. Both light and dark themes pass WCAG 2.1 AA color-contrast ratios and all interactive elements are reachable by keyboard with visible focus indicators
**Plans**: TBD
**UI hint**: yes

### Phase 13: Docs Content Pipeline

**Goal**: A visitor can browse a Fumadocs-powered documentation site with sidebar navigation, table of contents, and working static search — all served from pre-built static files with no server runtime.
**Depends on**: Phase 12
**Requirements**: DOCS-01
**Success Criteria** (what must be TRUE):
  1. `pnpm build` emits `out/docs/` with HTML pages for every docs route; `out/api/search/index.json` is non-empty (Orama static index generated)
  2. The docs sidebar lists all top-level sections (getting-started, installation, configuration, integration, security, cli-reference, audit-log, troubleshooting) in the correct order
  3. The table of contents panel renders correctly for a docs page with multiple headings
  4. The search dialog opens, a query returns at least one result, and clicking a result navigates to the correct page
**Plans**: TBD
**UI hint**: yes

### Phase 14: Changelog Pipeline

**Goal**: A visitor can read versioned release notes (v1.0.0, v1.2.0, v1.3.0) with per-release download and verification guidance, and the v1.3.0 entry displays a red breaking-change callout.
**Depends on**: Phase 12
**Requirements**: CHG-01, CHG-02, CHG-03
**Success Criteria** (what must be TRUE):
  1. `pnpm build` emits `out/changelog/v1.0.0/`, `out/changelog/v1.2.0/`, and `out/changelog/v1.3.0/` as separate static HTML pages
  2. Each changelog page includes copyable cosign/SLSA verification commands and a link to the corresponding GitHub Release
  3. The v1.3.0 changelog page displays a prominently styled red callout for the exit-1 to exit-2 breaking change with a migration note
**Plans**: TBD
**UI hint**: yes

### Phase 15: Marketing Home

**Goal**: A visitor sees a complete marketing home page — hero with dual CTA, origin story, how-it-works, feature highlights for shipped capabilities, the 15-harness support matrix, and an honesty callout — all as server-rendered static content with a static SVG hero (no 3D yet).
**Depends on**: Phase 12
**Requirements**: HOME-01, HOME-02, HOME-03, HOME-04, HOME-05, SITE-03
**Success Criteria** (what must be TRUE):
  1. The home page hero displays the headline, subhead, a copyable `go install` command chip, and a "Read the docs" link — all visible above the fold on a 1280px viewport
  2. The page explains the Nx Console compromise origin story and presents a 3-step how-it-works flow
  3. Feature cards cover only the six shipped capabilities (corroboration engine, fail-closed hooks, editor-extension defense, Sentry, LlamaFirewall, policy-as-code) with no aspirational claims
  4. The harness support matrix shows all 15 harnesses with honest tier labels and live-verification caveats, linking to the integration docs
  5. An honesty callout is visible on the home page linking to the security-posture / known-gaps documentation
  6. The static site is deployed to Cloudflare Pages and reachable at a public URL
**Plans**: TBD
**UI hint**: yes

### Phase 16: 3D Layer

**Goal**: The home hero features an interactive Three.js hive centerpiece and ambient 3D accents that load client-only behind a dynamic(ssr:false) boundary, meet the Lighthouse LCP < 2.5s budget, and fall back gracefully to a static SVG under reduced-motion, low-power, or no-WebGL conditions.
**Depends on**: Phase 15
**Requirements**: GFX-01, GFX-02, GFX-03, GFX-04
**Success Criteria** (what must be TRUE):
  1. `pnpm build` succeeds with no R3F/three/drei imports visible in server-rendered output; the R3F chunk loads lazily in the browser (verified: `<canvas>` element appears only after JS hydration)
  2. With `prefers-reduced-motion: reduce` or WebGL unavailable, the hero renders the static SVG fallback and no canvas is mounted — the page is fully usable
  3. The `<canvas>` element has `aria-hidden="true"` and a visible sr-only description; keyboard navigation is unaffected by the 3D layer
  4. Lighthouse audit against `out/index.html` shows LCP < 2.5s (SVG as LCP candidate) and no WebGL context leaks across navigation cycles
**Plans**: TBD
**UI hint**: yes

### Phase 17: SEO & Static Assets

**Goal**: Every page emits correct static metadata, a shared OG/social card image, and the site produces `sitemap.xml` and `robots.txt` as part of the static build.
**Depends on**: Phase 15
**Requirements**: SEO-01
**Success Criteria** (what must be TRUE):
  1. Every page in `out/` has a `<title>`, `<meta name="description">`, and canonical `<link rel="canonical">` in its static HTML
  2. Every page references the shared OG image (1200×630 PNG); social-card preview renders correctly when the URL is pasted into Twitter/LinkedIn
  3. `out/sitemap.xml` lists all public page URLs and `out/robots.txt` allows all crawlers and references the sitemap
**Plans**: TBD
**UI hint**: no

### Phase 18: Full Content Authoring

**Goal**: A developer can follow every documentation path — quickstart to CLI reference to security posture to integration guides to troubleshooting — with content that is accurate to the shipped binary: every security claim cites its source, every unenforced feature is explicitly labeled, and known gaps are presented alongside the security posture.
**Depends on**: Phase 13, Phase 14, Phase 15
**Requirements**: DOCS-02, DOCS-03, DOCS-04, DOCS-05, DOCS-06, DOCS-07, DOCS-08, DOCS-09
**Success Criteria** (what must be TRUE):
  1. A new user can follow the Getting Started quickstart from zero to a working `beekeeper check` invocation, with no steps referencing unshipped behavior
  2. The installation docs cover `go install`, binary download from GitHub Releases, and cosign + SLSA verification with copyable commands
  3. The security posture and known-gaps pages are co-located and explicitly document: Hermes fail-open, Tier-3 unguarded tools, `release_age`/`lifecycle_script_allowlist` unenforced in v1.3.0, and the `--bind 0.0.0.0` gateway caveat
  4. Every integration guide for Tier-1/2/3 harnesses includes honest caveats at point-of-use; the CLI reference covers all subcommands and flags
  5. Every MDX file that derives content from a Go-side doc has a `source_doc:` frontmatter field, and all content has been reviewed against `docs/THREAT-MODEL.md` before publish
**Plans**: TBD
**UI hint**: yes

### Phase 19: Test Suite & CI

**Goal**: A path-filtered web CI job gates merges with lint, type-check, unit tests, a static build, and E2E tests against the `out/` directory — completely isolated from Go CI.
**Depends on**: Phase 16, Phase 17, Phase 18
**Requirements**: QA-01, QA-02
**Success Criteria** (what must be TRUE):
  1. The `.github/workflows/web.yml` job triggers only on `web/**` and `pnpm-workspace.yaml` changes, never on Go file changes; Go CI never triggers on web changes
  2. The CI job runs Biome lint/format check, `tsc --noEmit`, Vitest unit tests, `pnpm build`, and Playwright E2E against `out/` — all as required gates before merge
  3. Playwright E2E tests verify: home page renders with SVG hero fallback, docs navigation and search return results, theme toggle persists across reload, all three changelog version pages build and render their headings
**Plans**: TBD
**UI hint**: no

## Progress

| Phase | Milestone | Plans | Status | Completed |
|-------|-----------|-------|--------|-----------|
| 1. Foundation + Hook Handler | v1.0.0 | 5/5 | Complete | 2026-05-26 |
| 2. Policy Engine + Multi-Source Catalogs | v1.0.0 | 4/4 | Complete | 2026-05-27 |
| 3. Editor Extension Defense | v1.0.0 | 5/5 | Complete | 2026-05-26 |
| 4. Integration Surfaces | v1.0.0 | 3/3 | Complete | 2026-05-27 |
| 5. Linux Sentry | v1.0.0 | 5/5 | Complete | 2026-05-28 |
| 6. LlamaFirewall + Audit Sinks | v1.0.0 | 5/5 | Complete | 2026-06-03 |
| 7. Cross-Platform Sentry | v1.0.0 | 5/5 | Complete | 2026-05-28 |
| 8. TUI Dashboard | v1.0.0 | 9/9 | Complete | 2026-05-29 |
| 9. Policy as Code + Self-Defense Capstone | v1.0.0 | 5/5 | Complete | 2026-05-29 |
| 10. Cross-Phase Integration Closure | v1.0.0 | 6/6 | Complete | 2026-06-05 |
| 11. v1.0.0 PRD-Gap Closure (pre-push) | v1.0.0 | 1/1 | Complete | 2026-06-01 |
| **1. Fork Setup & Discipline** | **v1.1.0** | **5/5** | **Complete** | **2026-05-26** |
| **2. Windows Root Resolver** | **v1.1.0** | **3/4** | **Code complete — release deferred to M2 close** | **—** |
| **3. Windows Path Representation** | **v1.1.0** | **3/3** | **Code complete & verified — release deferred** | **2026-06-02** |
| **4. Windows Extension & MCP Coverage** | **v1.1.0** | **3/3** | **Code complete & verified — release deferred** | **2026-06-02** |
| **5. Contribution-Back & Milestone Close** | **v1.1.0** | **0/5** | **PARKED** | **—** |
| **6. Corroboration Severity Hardening** | **v1.2.0** | **3/3** | **Complete** | **2026-06-03** |
| **7. Sensitive-Path Runtime Enforcement** | **v1.2.0** | **3/3** | **Complete** | **2026-06-04** |
| **8. Package-Manager Nudge + Behavioral Test Suite** | **v1.2.0** | **8/8** | **Complete** | **2026-06-04** |
| **9. v1.2.0 Tech-Debt Cleanup** | **v1.2.0** | **5/5** | **Complete** | **2026-06-04** |
| **10. Hook-Block Protocol Compliance** | **v1.3.0** | **6/6** | **Complete (seed)** | **2026-06-05** |
| **11. Scaffold & Toolchain Isolation** | **v1.3.0** | **0/TBD** | **Not started** | **—** |
| **12. Design System** | **v1.3.0** | **0/TBD** | **Not started** | **—** |
| **13. Docs Content Pipeline** | **v1.3.0** | **0/TBD** | **Not started** | **—** |
| **14. Changelog Pipeline** | **v1.3.0** | **0/TBD** | **Not started** | **—** |
| **15. Marketing Home** | **v1.3.0** | **0/TBD** | **Not started** | **—** |
| **16. 3D Layer** | **v1.3.0** | **0/TBD** | **Not started** | **—** |
| **17. SEO & Static Assets** | **v1.3.0** | **0/TBD** | **Not started** | **—** |
| **18. Full Content Authoring** | **v1.3.0** | **0/TBD** | **Not started** | **—** |
| **19. Test Suite & CI** | **v1.3.0** | **0/TBD** | **Not started** | **—** |
