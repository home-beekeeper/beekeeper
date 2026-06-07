# Feature Research

**Domain:** Developer-tool marketing + documentation website (OSS security CLI)
**Project:** Beekeeper v1.3.0 — Web Presence & Documentation
**Researched:** 2026-06-07
**Confidence:** HIGH (grounded in actual shipped product — PROJECT.md, THREAT-MODEL.md, harness-support-matrix.md — plus dev-tool landing page research and doc IA patterns from comparable OSS security tools)

> **Scope note:** This file supersedes the v1.2.0 runtime-hardening feature research. It covers the three new web surfaces: Marketing Home, Documentation (Fumadocs), and Changelog / Releases. The downstream consumer is the requirements definition and roadmap planner for v1.3.0. All content claims are anchored to shipped product facts.

---

## Surface 1: Marketing Home

### Table Stakes (Users Expect These)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Hero with headline + one-liner value prop | Any marketing page must answer "what is this?" in under 5 seconds; absence means bounce | LOW | "Real-time safety harness for autonomous coding agents." Already ship-ready from README.md line 3. |
| Primary install CTA in hero | Dev-tool users want an on-ramp without scrolling; dual CTA (install + docs) is the documented best practice for OSS tools (Evil Martians study) | LOW | Two CTAs: `go install github.com/bantuson/beekeeper@latest` (copy-to-clipboard chip) + "Read the docs" secondary. Primary styled boldly; secondary text-link. |
| Problem / origin story section | Grounds the tool in a concrete threat event rather than generic "AI safety" positioning | MEDIUM | The Nx Console compromise (May 2026, TeamPCP, ~3,800 GitHub-internal repos exfiltrated via trojanized extension) is a named, verifiable incident. Pull from PROJECT.md §Context. One paragraph, not a wall of text. |
| How-it-works section | Developers need the mental model before trusting a security interceptor | MEDIUM | 3-step linear flow: (1) Agent invokes tool call → (2) Beekeeper evaluates against corroborated threat intel → (3) Allow / Warn / Block / Quarantine. Short prose + icon-based step diagram. |
| Feature highlights (capability cards) | Visitors scan for specific capabilities; if they don't see "MCP gateway" or "audit log" they assume the feature doesn't exist | MEDIUM | Ground in shipped features only. Eight cards: hook interception, MCP gateway, corroboration engine, sensitive-path SPATH, LlamaFirewall, Sentry daemon, audit log, TUI dashboard. |
| Harness support matrix | Developers work with a specific agent; if their harness isn't listed they won't install | MEDIUM | 15 harnesses across 3 tiers already documented in `docs/harness-support-matrix.md`. Show tier icons. Include honest caveat: "Claude Code is the only live-verified harness." |
| Footer with GitHub, license, SECURITY.md links | Developers check license and disclosure policy before adopting OSS security tooling | LOW | Apache 2.0, GitHub repo, SECURITY.md, docs. |
| Responsive + accessible | Any 2026 web page is expected to work on mobile and for screen-reader users | LOW | shadcn/ui provides accessible primitives. Three.js hero needs `prefers-reduced-motion` static fallback and `aria-hidden` on the canvas element. |

### Differentiators (Competitive Advantage)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Three.js hero: hive / agent-mediation visual | Makes "intercept between agent and action" tangible; signals craft and intentionality — differentiates from the README-as-website pattern common in OSS security tools | HIGH | Hive metaphor: hexagonal cells (each a tool call), agents (bees) routed through a central node (Beekeeper). Animation: packets route in → green glow on allow, red flash on deny. Lazy-load canvas. Static SVG fallback for `prefers-reduced-motion`. `aria-hidden` on canvas. Performance budget: <300KB compressed Three.js bundle, 60fps on mid-range GPU. |
| "Why corroboration?" explainer section | The 2FA principle for threat intel (1 source = warn, 2 = block, 3 = block+quarantine) is architecturally distinctive and easy to explain with a concrete scenario | MEDIUM | Use the "single attacker-controlled catalog" framing from THREAT-MODEL.md §3. Visual: three source logos (Bumblebee, OSV, Socket) + a corroboration score meter that fills as sources agree. No competitor has this specific design decision. |
| "Fail-closed by default" trust signal | Most security tools fail open; making fail-closed a headline feature reassures security-conscious users before they read the docs | LOW | Pull quote: "Any crash, timeout, or unavailability results in block, not allow. Benchmarked at ~3.58ms/op on Celeron N4020." Both the posture and the performance number are real. |
| Honesty / transparency block | Documenting known gaps on the marketing site is rare and builds developer trust faster than claiming perfection | MEDIUM | 3-4 bullet points: Tier-3 native tools are unguarded, Hermes is structurally fail-OPEN, command-chaining parse evasion exists, fanotify mmap gap on Linux. Each links to the full THREAT-MODEL.md entry. Rare for a marketing site; signals the project is trustworthy about its limits. |
| Supply-chain integrity callout | SLSA Level 3 + Sigstore + CycloneDX SBOM is a meaningful differentiator; most OSS doesn't have this | LOW | Visual badges with verification commands. Link to releases page for full cosign / slsa-verifier steps from THREAT-MODEL.md §7. |
| Three.js ambient accents in non-hero sections | Maintains visual identity across the page without re-loading the full hero scene | MEDIUM | Subtle: slow hex-grid particle field behind the corroboration section. A low-speed rotating node graph in the harness matrix header. Each is a small, isolated Three.js canvas — not the full hero renderer. |

### Anti-Features (Deliberately Exclude)

| Feature | Why Requested | Why Avoid | Alternative |
|---------|--------------|-----------|-------------|
| In-browser demo / playground | Users want to try before installing | Beekeeper requires filesystem access, a running catalog, and OS hooks — impossible to sandbox in a browser; a fake demo destroys trust for a security tool | Terminal GIF or 30-second screen recording of the credential-read block, embedded as a `<video>` below the hero fold |
| Blog section on home page | Signals active project | Out of scope per v1.3.0 milestone decision; adds content-maintenance burden with no clear ROI at this stage | GitHub releases and changelog page are the "active project" signal |
| GitHub star count / download badges | Social proof | Beekeeper has not been pushed to GitHub yet (local-only, never pushed); fabricating or inflating numbers is dishonest | Use the origin story (named incident) + supply-chain verification chain as trust signals; add star count after public launch |
| Pricing section | SaaS pattern | Apache 2.0 OSS; no pricing model | Single clear statement: "Free. Open source. Apache 2.0." in the footer or hero eyebrow |
| Newsletter / email capture | Lead generation | Not a SaaS company; introduces privacy obligations; adds nothing for an OSS CLI tool | GitHub "Watch" and "Star" as the follow mechanism; link to GitHub from hero |
| Animated background on every section | Visual richness | Performance regression on low-end machines, vestibular accessibility concern, distraction from content | Reserve Three.js for hero + 2 targeted ambient accents; all other sections are clean shadcn/ui |
| Video autoplay with sound | Modern feel | Autoplaying with audio is a UX anti-pattern; without audio it duplicates the Three.js animation | Click-to-play `<video>` element beneath the hero fold if a screen recording is needed |

---

## Surface 2: Documentation

### Information Architecture (recommended structure)

```
Getting Started
  └── Quickstart (< 5 minutes to first block)
  └── How Beekeeper works (conceptual)
Installation
  └── go install (recommended)
  └── GitHub Releases binary
  └── Reproducible build verification (make verify-release)
  └── Cosign / SLSA verification
Configuration
  └── Layered config (system → user → project → env → CLI flags)
  └── Config reference (all fields, defaults, valid values)
  └── fail_mode and its security implications
  └── Policy-as-code (policy files, beekeeper policy validate/test)
  └── Sensitive paths (DefaultSensitivePaths, extend / allowlist)
Integration Guides
  └── Tier 1 harnesses (Claude Code, Codex, Cursor, Augment, CodeBuddy, Qwen, Gemini CLI, Copilot, Antigravity, Windsurf)
  └── Tier 2 harnesses (Hermes, Cline, OpenCode) — caveats first
  └── Tier 3 harnesses (Kilo, Trae) — unguarded native tools warning first
  └── MCP gateway (start/stop, auth, localhost binding, remote-bind warning)
Security Posture
  └── Corroboration model (the 2FA principle)
  └── Fail-closed defaults
  └── Self-protection (agent cannot tamper with Beekeeper)
  └── Build and release pipeline (SLSA L3, Sigstore, SBOM, reproducible builds)
  └── beekeeper-self catalog
  └── Known gaps and explicit non-defenses
CLI Reference
  └── check
  └── catalogs (sync, watch)
  └── hooks (install, uninstall)
  └── gateway (start, token)
  └── audit (tail, query, export)
  └── policy (list, validate, test)
  └── scan
  └── diag
  └── selftest
  └── protect (install, uninstall)
  └── dashboard
  └── nudge
  └── version
Audit Log
  └── NDJSON schema
  └── Sinks (local, syslog, OTLP, HTTPS)
  └── Redaction scope and limitations
  └── beekeeper audit tail / query / export
Troubleshooting
  └── Hook not firing
  └── Catalog stale or degraded
  └── Self-quarantine event (verification steps)
  └── beekeeper diag output reference
  └── Windows state dir (%APPDATA%/beekeeper vs ~/.beekeeper)
```

### Table Stakes (Users Expect These)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Getting Started / Quickstart (< 5 min to first block) | First goal of any developer doc: get the user to a working state fast | MEDIUM | Three steps: (1) install, (2) `beekeeper catalogs sync`, (3) `beekeeper hooks install --target <harness>`. Must end with a verifiable outcome: paste the credential-read tool call JSON and see the block in terminal output. |
| Installation guide (all methods) | Multiple install paths serve different sub-audiences; missing any path alienates that group | LOW | Three paths: `go install` (recommended, Go developers), GitHub Releases binary download (non-Go users), `make verify-release` (reproducible build verification). `curl \| sh` documented as NOT recommended with honest trade-off explanation from PROJECT.md. |
| Cosign + SLSA verification steps | Security-conscious adopters verify binaries before running; a security tool that doesn't document this is self-undermining | LOW | Copy-paste commands from THREAT-MODEL.md §7 Steps 2 and 3: `cosign verify ...` and `slsa-verifier verify-artifact ...`. |
| Configuration reference (all fields) | Users need to know every config option with defaults and valid values; the layered config model must be explained explicitly | HIGH | Keys: `fail_mode` (default `closed`; explicit note that `open` reduces security), `corroboration_threshold`, `self_catalog.*`, `nudge.*`, per-ecosystem settings. Precedence order: system → user → project → env → CLI. Warning about project-layer relaxation from THREAT-MODEL.md §8. |
| Policy-as-code guide | Policy files are the primary customization surface and a selling-point differentiator | HIGH | Covers: `~/.beekeeper/policies/*.json` structure, `package_allowlist` rules, `sensitive_path` rules, `beekeeper policy validate`, `beekeeper policy test <file>`, escape-hatch semantics from THREAT-MODEL.md §9. Honest note: `release_age` and `lifecycle_script_allowlist` are declared in policy files but NOT enforced in v1 — do not imply otherwise. |
| Sensitive paths guide (SPATH) | Users want to know what credential paths are blocked by default and how to extend the list | LOW | `DefaultSensitivePaths` list: `~/.ssh`, `~/.aws`, `~/.cargo/credentials`, `.env` globs, editor MCP config dirs (Cursor/Windsurf). Windows ADS normalization and trailing-dot evasion noted. Allowlist escape hatch documented. |
| CLI command reference (all subcommands) | A CLI tool without a command reference is unusable for non-trivial configuration | HIGH | Every subcommand with flags, examples, and expected output format. At minimum: `check`, `catalogs sync/watch`, `hooks install/uninstall`, `gateway start/token`, `audit tail/query/export`, `policy list/validate/test`, `scan`, `diag`, `selftest`, `protect install`, `dashboard`, `nudge`, `version`. |
| Harness integration guide (all 15 harnesses) | Developers need per-harness setup instructions; 15-harness support is a selling point but also a complexity burden | HIGH | One sub-page per tier. Tier-1: full hook install with `--dry-run` preview. Tier-2: caveat-first (Hermes fail-OPEN warning must appear before config instructions; Cline Windows-only limitation must appear before config instructions). Tier-3: explicit "native tools UNGUARDED" warning in a red callout block before any config instructions. |
| MCP gateway guide | The gateway is the only enforcement path for Tier-3 harnesses and an alternative for others | MEDIUM | Start/stop, auth token flow, `127.0.0.1` default binding. Explicit warning about `--bind 0.0.0.0` from THREAT-MODEL.md §8: the `allow_remote_gateway` config gate is not yet implemented — the help text promises a gate that does not exist. |
| Troubleshooting guide | Users get stuck; without a troubleshooting section they abandon the tool | MEDIUM | Common scenarios: hook not firing (settings.json not merged), catalog index stale (`beekeeper catalogs sync`), self-quarantine event (point to THREAT-MODEL.md §7 verification steps), Windows `%APPDATA%/beekeeper` vs `~/.beekeeper` state dir confusion. |
| Full-text search | Any doc site over ~10 pages needs search to be usable | LOW | Fumadocs ships built-in search. Enable it. No additional implementation cost. |

### Differentiators (Competitive Advantage)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Security posture page (dedicated) | Most CLI tools bury security info in a single README section; a dedicated page signals the project treats this seriously and makes it easy to cite in a security review | MEDIUM | Covers: corroboration model (2FA principle), fail-closed defaults, self-protection (state dir, binary, hook entry), `beekeeper-self` catalog, build pipeline (SLSA L3, Sigstore, SBOM). Source content from THREAT-MODEL.md §1-7 and §9. |
| Known gaps / honest limitations (own sub-section of security posture) | Rare for a marketing-facing doc; builds trust with security-savvy readers who will find the gaps regardless | MEDIUM | Direct prose summaries of THREAT-MODEL.md §8: Tier-3 unguarded native tools, Hermes fail-OPEN, `--bind 0.0.0.0` gate not implemented, project-layer config can relax fail-closed, command-chaining parse evasion (TM-B-06), fanotify mmap gap (§5). Frame as "what we don't claim to do" — not as failures. |
| Self-protection documentation (dedicated section) | Unique shipped feature: the agent cannot tamper with Beekeeper's own config, binary, or hook entry. Operators need to understand what is and is not protected. | LOW | Content from THREAT-MODEL.md §9 "Self-Protection": state dir read+write block, binary write-block, content-aware hook-entry guard, CLI mutation block (config set, hooks install/uninstall, protect install/uninstall). Explicit note: human channels are unaffected (terminal, dashboard, /config). |
| Corroboration model conceptual explainer | The "2FA for threat intel" framing is the core architectural insight; explaining it in docs (not just marketing) helps operators set `corroboration_threshold` correctly | LOW | Short prose + threshold table from THREAT-MODEL.md §3. Include: per-severity CORR thresholds (CORR-01/02), degraded-source suppression, sanity bounds, the coordinated false-positive poisoning attack surface (§4) with mitigations. |
| Audit log guide with redaction scope honesty | Operators forwarding logs off-host need to know the redaction scope; the limitation (field-scoped, not content-scanning) is documented in THREAT-MODEL.md §8 | MEDIUM | Cover: local 0600 NDJSON, syslog/OTLP/HTTPS opt-in sinks with "data leaving this machine" warning, `audit tail/query/export`, redaction limitation (Sentry-derived fields and network destinations are written verbatim; behavioral-watch path does not route through RedactRecord). |
| Per-OS notes surfaced inline (not in a separate "Windows" page) | Windows state dir is `%APPDATA%/beekeeper`. ETW vs fanotify vs eslogger. Cline Windows limitation. These should appear at the point of use. | LOW | Inline callout blocks (`:::note[Windows]`) in relevant sections. Cost is authoring discipline. |

### Anti-Features (Deliberately Exclude)

| Feature | Why Requested | Why Avoid | Alternative |
|---------|--------------|-----------|-------------|
| Aspirational / not-yet-shipped feature documentation | Completeness | THREAT-MODEL.md and PROJECT.md explicitly flag deferred items: weighted corroboration, `beekeeper-self` live hosting E2E, `allow_remote_gateway` config gate. Documenting these as present creates false confidence and is a security risk | Mark deferred items "planned" in a callout, or omit until shipped; update docs on release |
| Interactive config builder | Developer UX | High complexity for low gain; the config schema is simple enough to document as a reference table + example JSON | Copyable JSON examples with inline comments |
| Version-switcher (multi-version docs) | Completeness | Only two shipped milestones; version-switching adds nav complexity before it is warranted | Single current docs + changelog page; add version-switcher when there are 3+ major versions with breaking changes |
| AI chatbot / "ask the docs" widget | Discoverability | Significant ongoing cost (API fees, hallucination risk); especially dangerous for a security tool where a wrong answer can leave users unprotected | Excellent search (Fumadocs built-in) + clear IA |
| Downloadable PDF of docs | Enterprise compliance | PDF goes stale immediately; adds a build step with no clear consumer | Link to the GitHub repo for offline access |

---

## Surface 3: Changelog / Releases

### Table Stakes (Users Expect These)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Per-version release notes (prose) | Developers need to know what changed before upgrading a security tool; a raw git log is not acceptable | LOW | v1.0.0 and v1.2.0 notes. Write prose summaries from `milestones/` archives: what shipped, what was fixed, what changed. Use REQ-IDs in a footnote for cross-reference; lead with user-facing language. |
| Download table per release (all platforms) | Binary distribution is the product; missing a platform entry leaves that group stranded | LOW | Linux amd64/arm64, macOS Intel + Apple Silicon, Windows amd64. Link to GitHub Releases page. Do not host binaries on the marketing site. |
| Cosign verification command per release | Security-conscious users verify before running; a security tool that buries this is self-undermining | LOW | Static code block per release. Command from THREAT-MODEL.md §7 Step 2: `cosign verify --certificate-identity=... --certificate-oidc-issuer=...`. |
| SLSA provenance link + verification command per release | SLSA Level 3 is a meaningful supply-chain security signal; it should be surfaced at the point of download | LOW | `slsa-verifier verify-artifact ...` + link to `.intoto.jsonl` on GitHub Releases. THREAT-MODEL.md §7 Step 3. |
| SBOM link per release | Enterprise adopters often require SBOM for compliance | LOW | Link to `beekeeper.cyclonedx.json` on GitHub Releases with a one-line `jq` inspection command from THREAT-MODEL.md §7 Step 4. |
| Release date and version number as headings | Basic navigation — users scan for "latest" and "the version I'm on" | LOW | Heading format: `v1.2.0 — Runtime Behavioral Hardening — 2026-06-04`. |

### Differentiators (Competitive Advantage)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| "Security changes in this release" callout block | Security tools have security-relevant changes that need to stand apart from feature changes; operators need to prioritize upgrades | LOW | Distinct visual callout (amber/warning) per release entry. v1.0.0: fail-closed landed (top-level panic-recover → block), SLSA L3 + Sigstore shipped. v1.2.0: F1 critical-malware warn→block, F2 credential-reads ALLOW→BLOCK, F3 pnpm/bun catalog bypass closed. v1.3.0: exit-1→exit-2 fix (silent-allow defect that affected all harnesses). |
| "Known gaps in this release" subsection | Mirrors the threat model's honesty; helps operators decide whether gaps are acceptable for their threat model before upgrading | LOW | Per-release list of accepted gaps from the relevant threat model version. Short bullets, each linking to THREAT-MODEL.md for the full description. |
| Reproducible build verification instructions | Allows any user to confirm the released binary matches the tagged source — this is the most rigorous supply-chain verification | LOW | `make verify-release VERSION=X.Y.Z` from THREAT-MODEL.md §7 Step 1. |
| Migration notes (breaking changes explicitly flagged) | Operators need to know if upgrading requires config changes | LOW | v1.0.0: no prior version to migrate from. v1.2.0: no breaking config changes. v1.3.0: **breaking change** — hook exit code changed from 1 to 2; any wrapper script that tested for exit 1 to detect a block will no longer work. Must be called out prominently in a red callout block. |

### Anti-Features (Deliberately Exclude)

| Feature | Why Requested | Why Avoid | Alternative |
|---------|--------------|-----------|-------------|
| Auto-generated changelog from commit messages | Low effort, looks complete | Commit messages are written for developers, not for operators deciding whether to upgrade a security tool; raw git log is noise | Hand-authored prose summaries with REQ-ID footnotes |
| RSS feed for releases (hosted) | Automation / monitoring | Adds implementation complexity; GitHub Releases already provides an Atom feed at the standard URL | Link to the GitHub Releases Atom feed from the releases page |
| "What's coming" / roadmap section on releases page | Transparency | Beekeeper has no committed public roadmap for future milestones; speculative roadmaps create expectation debt and can mislead users about security posture | Link to GitHub Discussions or open issues instead |

---

## Feature Dependencies

```
Marketing Home
    └── Three.js hero ──requires──> performance budget decision + static SVG fallback (build-time)
    └── Harness matrix ──pulls-from──> docs/harness-support-matrix.md (content already exists)
    └── Corroboration explainer ──content-from──> THREAT-MODEL.md §3
    └── Supply-chain callout ──links-to──> Releases page (verification commands)
    └── Honesty block ──links-to──> Security Posture doc page

Documentation
    └── Getting Started ──requires──> Installation guide (prerequisite; do not merge)
    └── Installation guide ──links-to──> Releases page (download links)
    └── Policy-as-code guide ──requires──> Configuration reference (layered config must come first)
    └── Harness integration guide ──requires──> MCP gateway guide (Tier-3 harness pages reference gateway)
    └── Security posture page ──requires──> Known gaps section (ship together — honesty requires both)
    └── Sentry guide ──requires──> Installation guide (protect install is a post-install step)

Releases Page
    └── Per-version notes ──pulls-from──> milestones/ archive in repo
    └── Verification commands ──content-from──> THREAT-MODEL.md §7
    └── v1.3.0 migration note ──documents──> exit-1→exit-2 breaking change
```

### Dependency Notes

- **Security posture page requires known gaps section:** Publishing security properties without the limitations creates false confidence. These must ship together; do not ship the posture page as a stub while the gaps section is in progress.
- **Getting Started requires Installation as a prerequisite page:** Quickstart references the install; keep them separate so users can return to Installation independently (e.g., to run cosign verify on a new machine).
- **Three.js hero requires a static SVG fallback:** The canvas is an enhancement. For `prefers-reduced-motion` users and slow connections the page must be fully usable without it. Build the SVG first; add the Three.js canvas on top.
- **Tier-3 harness pages require the MCP gateway guide to exist:** Kilo and Trae integration pages must link to the gateway guide as the enforcement path. The gateway guide cannot be deferred while harness pages are being written.
- **Honest limitations must appear at the point of use, not only in the threat model:** The Hermes fail-OPEN caveat belongs in the Hermes integration page header — not just in a separate "known gaps" section. Duplication is intentional.

---

## MVP Definition

### Launch With (v1.3.0 target)

- [x] Marketing home — hero (Three.js), value prop, problem/origin, how-it-works, feature highlights, harness matrix, install CTA, fail-closed + corroboration callouts, supply-chain integrity callout, honesty block
- [x] Documentation — Getting Started, Installation (all methods + cosign + SLSA verify), Configuration reference (layered config + fail_mode warning), Policy-as-code guide (with v1 enforcement limitations noted), Sensitive paths, CLI reference (all subcommands), Harness integration (all 15 with tier caveats at point-of-use), MCP gateway (with remote-bind warning), Security posture page (corroboration + fail-closed + self-protection + build pipeline + known gaps), Troubleshooting, Audit log guide
- [x] Releases page — v1.0.0 and v1.2.0 prose notes, download table (all platforms), cosign/SLSA/SBOM per release, security-changes callout, v1.3.0 exit-code migration note

### Add After Validation (v1.x)

- [ ] Sentry / protected mode guide — complex, OS-specific content requiring stable feature; add when Sentry is live-verified cross-platform and the fanotify mmap gap (THREAT-MODEL.md §5) has a definitive status update
- [ ] `beekeeper diag` output field reference — low priority until adoption drives questions
- [ ] Per-OS callout discipline audit — review all pages for Windows-specific notes after first user feedback cycle

### Future Consideration (v2+)

- [ ] Version-switcher for docs — when there are 3+ major versions with breaking changes
- [ ] AI-assisted search — only if a solution exists that cannot hallucinate dangerous security advice

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Hero with headline + install CTA | HIGH | LOW | P1 |
| Getting Started / Quickstart | HIGH | MEDIUM | P1 |
| CLI command reference | HIGH | HIGH | P1 |
| Harness integration guide (all 15) | HIGH | HIGH | P1 |
| Security posture page + known gaps | HIGH | MEDIUM | P1 |
| Releases page with verification per release | HIGH | LOW | P1 |
| Configuration + policy-as-code guide | HIGH | HIGH | P1 |
| Problem / origin story | MEDIUM | LOW | P1 |
| Corroboration + fail-closed callouts on home | MEDIUM | LOW | P1 |
| Three.js hero visual | MEDIUM | HIGH | P1 (brand differentiator — in-scope per milestone) |
| Harness support matrix on home | MEDIUM | MEDIUM | P1 |
| MCP gateway guide | MEDIUM | MEDIUM | P1 |
| Troubleshooting guide | HIGH | MEDIUM | P1 |
| Audit log guide + redaction scope honesty | MEDIUM | MEDIUM | P2 |
| Honesty / transparency block on home | MEDIUM | LOW | P2 |
| Three.js ambient accents (non-hero) | LOW | MEDIUM | P2 |
| Security changes callout on releases page | MEDIUM | LOW | P2 |
| Sentry / protected-mode guide | MEDIUM | HIGH | P3 |
| `beekeeper diag` output reference | LOW | LOW | P3 |

---

## Competitor Reference (Documentation IA Patterns)

| Pattern | Seen In | Beekeeper Approach |
|---------|---------|-------------------|
| "Getting Started" as first doc page with a verifiable end state | Trivy, Gemini CLI, AWS CLI, cosign | Same — explicit 5-minute quickstart ending with a confirmed block in the terminal |
| Per-harness / per-platform integration sub-pages | Trivy (CI integrations), Sigstore (per-registry) | Per-harness sub-pages under "Integration Guides" with tier labeling and caveats at the top of each page |
| Dedicated security / threat model page | cosign (signing model), Sigstore (security model) | Dedicated "Security Posture" page sourced from THREAT-MODEL.md |
| "Known limitations" in docs (not hidden in a footnote) | OWASP tools, Falco | Integrated into Security Posture as a required sub-section — not a footnote |
| Releases page with per-release verification commands | cosign GitHub, SLSA tooling | Same pattern — per-release cosign + SLSA + SBOM |
| Dual CTA in hero (primary + secondary) | Trivy, Tailscale, Neon | `go install` (primary) + "Read the docs" (secondary) |
| Trust via concrete incident rather than logos | n/a (uncommon) | Origin story (Nx Console, May 2026) plays the same role as a customer logo strip — more compelling for a new project with no named users yet |
| Per-OS inline callout blocks | Homebrew docs, Nix manual | `:::note[Windows]` callout blocks in relevant sections rather than a separate "Windows" page |

---

## Sources

- PROJECT.md (Beekeeper project context, shipped features, origin story, constraints)
- docs/THREAT-MODEL.md (security posture content, verification path §7, known gaps §8)
- docs/harness-support-matrix.md (15-harness tier structure, deny mechanisms, honest caveats)
- README.md (harness tier table, quick start commands)
- [Evil Martians: We studied 100 dev tool landing pages](https://evilmartians.com/chronicles/we-studied-100-devtool-landing-pages-here-is-what-actually-works-in-2025) — hero/trust/feature section patterns (2025, still current)
- [Trivy landing page](https://trivy.dev/) — OSS security CLI site structure reference
- [Fumadocs documentation framework](https://www.fumadocs.dev/docs) — docs IA and search capability
- [Three.js forum — hero section patterns](https://discourse.threejs.org/t/website-interactive-3d-hero-scene-and-more/28004) — visual role of 3D hero, performance considerations
- [LogRocket — hero section best practices](https://blog.logrocket.com/ux-design/hero-section-examples-best-practices/) — dual CTA pattern

---

*Feature research for: Beekeeper v1.3.0 Web Presence & Documentation*
*Researched: 2026-06-07*
