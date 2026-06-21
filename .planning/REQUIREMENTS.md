# Requirements: v1.5.0 — Install Posture

> **Versioning:** GSD milestone **v1.5.0 "Install Posture"** (internal planning number, continues the v1.4.0 line; deliberately NOT "v1.1.0" — that string is the PARKED Pollen GSD milestone). **Ships publicly as release `v1.1.0`** (git tag, changelog, code version bump). No real `v1.1.0` git tag exists yet (Pollen's tags are `pollen`-suffixed), so the public tag is free.

**Source of truth:** `beekeeper-install-posture-prd.md` (repo root). This milestone retires the package-manager nudge and ships tool-agnostic install posture. Only the PRD's v1.0 / first-release scope (acceptance criteria 1-8) is in this milestone; PRD Layer 4 (config mutation) and the deep per-rule editor are roadmap.

**Two human gates (do not self-certify):**
- **Gate 1 — enforcement-boundary review.** After Layer 1 enforcement + the canonical boundary statement are written (end of Phase 27), present the enforcement map as implemented, the exact boundary copy, and any PRD divergence. Maintainer ratifies before propagation.
- **Gate 2 — release signing.** Release prepared fully (version, changelog, green CI, signed commits); the maintainer cuts and signs the final `v1.1.0` tag. The agent does not sign the release.

**Honesty standard:** the enforcement-boundary statement must appear wherever install posture is described (code, help text, docs, copy), held to the same standard as the harness-coverage table.

---

## v1.5.0 Requirements

### Install Posture — Layer 1 enforcement (IPST)
- [ ] **IPST-01**: A shipped default install posture (release-age <24h warn, lifecycle-script warn, git/remote-URL flag/warn) is enforced at the pre-exec hook via the existing warn/block/quarantine policy engine, on by default, with no configuration required. (AC1)
- [ ] **IPST-02**: Release-age enforcement catches a package version published under the configured window (default 24h, semver/registry-resolved, not pinned to any package manager's implementation) and surfaces the reason. (AC3)
- [ ] **IPST-03**: Lifecycle-script posture warns on an unapproved install script (aligned with npm v12's default), raisable to block per ecosystem (the raising itself is roadmap; the warn default ships). (AC1)
- [ ] **IPST-04**: Git and remote-URL dependency installs are detected and flagged/warned on first encounter with the reason surfaced (NEW rule; pkgparse gains remote-source detection + a pure policy evaluator). (AC1)
- [ ] **IPST-05**: Posture rules are inputs to the existing pure policy engine (new pure evaluators consumed by the check handler), not a parallel mechanism. (AC7)
- [ ] **IPST-06**: Sentry observes and audits installs it sees (agent or human-run), writing an audit record labeled detection, not prevention. (AC2)

### Install Posture — boundaries & honesty (IPBND)
- [ ] **IPBND-01**: The enforcement-boundary statement appears wherever install posture is described (code comments, help text, docs, web copy), to the harness-coverage honesty standard. (AC8)
- [ ] **IPBND-02**: The boundaries are documented exactly: prevention at the hook for hooked Tier-1 harnesses inheriting all tier caveats; observation + audit at Sentry for anything it sees including human installs, labeled detection not prevention; not a general MCP-gateway function; the package-manager shim for machine-wide human-install prevention is a labeled roadmap item. (AC2)

### Install Posture — Layer 2 read-only view (IPVIEW)
- [ ] **IPVIEW-01**: `beekeeper posture` (CLI + TUI) reads each detected package manager's actual config read-only and displays it side by side with Beekeeper's enforced posture, naming the gaps Beekeeper covers. Machine-wide. (AC4)
- [ ] **IPVIEW-02**: The posture view writes no package-manager config file — read-only guarantee, asserted by test. (AC4, AC6)

### Install Posture — Layer 3 scoped override + per-rule severity (IPOVR)
- [ ] **IPOVR-01**: A posture-rule decision at the hook (warn OR block) offers graduated scoped responses (allow once; allow always with a recorded reason; block this package) rather than a binary on/off. Operates on a WARN (the default), since the fail-soft default does not block. (AC5)
- [ ] **IPOVR-02**: Each override is written to the audit log as a scoped, recorded trust decision (allow-always persists via the existing policy overlay; nothing is a silent weakening). (AC5)
- [ ] **IPOVR-03**: A user can raise an individual posture rule from warn to block via layered config (per rule: release-age / lifecycle / git-remote). Untrusted (project) layers may only TIGHTEN (warn→block), never loosen, mirroring the fail_mode invariant. A definite violation uses the configured action; the unknown path (missing timestamp / registry error / timeout) stays fail-soft warn. (Maintainer-directed at Gate 1; pulls part of the PRD roadmap per-rule severity into v1.0.)

### Nudge removal & migration (NMIG)
- [ ] **NMIG-01**: The nudge feature and its steer-to-pnpm/Bun copy are removed from the product and docs (package, check adapter, gateway derivation, `beekeeper nudge` CLI, `config set nudge.*`, `ensureNudgeBlockDefault` npm/yarn-deny on install, layered nudge invariants, audit nudge fields). (Migration note)
- [ ] **NMIG-02**: The release-age logic is preserved and repositioned as the release-age posture rule (the pure `EvaluateReleaseAge` evaluator + catalog age adapter, now wired into the hook). (Migration note)
- [ ] **NMIG-03**: The home page "Agent safety" nudge bullet is replaced with the install-posture framing, plus the nudge-obsolescence note: npm v12 blocks install scripts by default, so the nudge's premise is removed and install posture is the tool-agnostic successor. No em dashes, sentence case. (DoD #7)
- [ ] **NMIG-04**: The existing package-manager shim is kept (not deleted), repointed off the nudge onto the posture-aware `beekeeper check`, and documented as roadmap/experimental rather than a headline v1.0 surface. (Scope decision; maintainer-approved)

### Release preparation (REL)
- [ ] **REL-01**: Version bumped to `v1.1.0` (code version + changelog) with the shipped install-posture feature and a roadmap note for the deferred layers (config mutation, deep per-rule editor, shim as a first-class surface). Signed commits through PRs to main per the branch ruleset. The final tag is signed by the maintainer at Gate 2. (DoD #8)

---

## Future Requirements (deferred to roadmap, marked as such)
- Deep per-rule per-ECOSYSTEM policy editing (custom release-age windows, warn/block/quarantine per rule PER ECOSYSTEM, per-project granular overrides, quarantine action) in the TUI policy editor. NOTE: the global per-rule warn→block opt-up moved INTO v1.0 as IPOVR-03 by maintainer decision; only the finer-grained per-ecosystem/per-project matrix and custom thresholds remain deferred. (PRD Layer 3 partial deferral)
- The package-manager shim as a first-class machine-wide human-install pre-exec enforcement surface (Surface 4). It exists today but is documented as roadmap/experimental, not promoted in v1.0. (PRD Surface 4)
- Config mutation: opt-in, reversible, audited, dry-run-by-default generation of recommended pm config that closes a posture gap. (PRD Layer 4)

## Out of Scope (explicit exclusions)
- **Building the shim as a new feature** — it already exists; the decision is keep + repoint + document as roadmap, never a new build. (Scope)
- **Config mutation of any package-manager file** (`.npmrc`, `package.json`, `pnpm-workspace.yaml`, `bunfig.toml`, etc.) by default or without an explicit, reversible, audited, opt-in action. Beekeeper enforces at its own surfaces and leaves config files alone. (AC6)
- **Deep per-rule per-ecosystem policy editing** in the TUI. (AC7)
- **The MCP gateway as a general install-enforcement surface.** Only an MCP-exposed install-type tool routes through it; the gateway's job is response scanning. (PRD Surface 3)
- Any roadmap-tier item carried in the PRD beyond v1.0 acceptance criteria 1-8.

## Traceability
Filled by ROADMAP.md (phases 26-31). Every REQ-ID maps to exactly one phase.
</content>
</invoke>
