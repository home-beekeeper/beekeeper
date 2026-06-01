# Phase 1: Fork Setup & Discipline - Context

**Gathered:** 2026-06-01
**Status:** Ready for planning
**Source:** PRD Express Path (`beekeeper-m2-prd.md` §11 M2.1) + cross-repo decision (this session)

<domain>
## Phase Boundary

Phase 1 establishes the **Pollen** repository — a bounded Apache-2.0 fork of upstream `perplexityai/bumblebee` — with correct attribution, reproducible+signed builds, and the CI guard rails (differential test + selftest matrix) **green before any Windows code lands**. No Windows functionality ships in this phase; its whole purpose is to prove the fork hygiene and the guards work. It ends with a tagged, signed release `v0.1.1-pollen.1`.

**Requirements covered:** FORK-01, FORK-02, FORK-03, FORK-04, PTEST-02, PTEST-03, SDEF-02.

**In scope:**
- Create the `pollen` repo (sibling dir + its own git), fork upstream at the pinned v0.1.1 commit
- Rename module path → `github.com/bantuson/pollen`; rename `cmd/bumblebee/` → `cmd/pollen/`; `pollen` CLI builds on ubuntu/macos/windows
- Legal/attribution: verbatim Apache-2.0 LICENSE, NOTICE, CHANGES.md, UPSTREAM.md (pinned commit + sync workflow), trademark discipline
- CI matrix (ubuntu/macos/windows, go 1.25.x): `go vet`, `go test -race ./...`, `pollen selftest`, versioned build; upstream's inherited tests pass on Linux/macOS
- Differential test: `pollen` output byte-for-byte identical to upstream `bumblebee` on Linux + macOS for a fixed fixture; runs every PR
- Reproducible builds (`-trimpath -buildvcs=false`) + Sigstore keyless signing (GitHub Actions OIDC) + CycloneDX SBOM; tag `v0.1.1-pollen.1`

**Out of scope (later phases):** any Windows resolver/path/extension code (P2–P4), beekeeper-side integration (P4–P5), `pollen-self` catalog entries (P5).
</domain>

<decisions>
## Implementation Decisions

### Repository & Cross-Repo Model (LOCKED this session)
- Pollen lives at **`C:\Users\Bantu\mzansi-agentive\pollen`** (sibling to beekeeper), as **its own git repository** — NOT vendored into beekeeper (PRD §5.1).
- GitHub home: **`github.com/bantuson/pollen`** (the GitHub handle is `bantuson`; `mzansi-agentive` was only ever local naming).
- **GSD tracks this milestone from beekeeper for now.** Phase planning artifacts (CONTEXT/PLAN/SUMMARY) live in `beekeeper/.planning/`. The **code** is created in and committed to `../pollen` via explicit `git -C ../pollen ...` operations in plan tasks. Beekeeper's own GSD commits cover only the planning artifacts. Whether Pollen later gets its own `.planning/` is revisited AFTER it exists (phases 2–4) — do not assume it here.
- Because pollen is outside beekeeper's worktree, executor tasks that produce pollen files MUST perform their own `git -C ../pollen add/commit` and must NOT rely on beekeeper's auto-commit for pollen code.

### Fork mechanics (PRD §5.1, §5.2)
- Pin to an upstream **commit** (not a branch): the v0.1.1 tag's 40-char SHA. Record it in `UPSTREAM.md` (`upstream: github.com/perplexityai/bumblebee`, `pinned commit: <SHA>`, `pinned tag: v0.1.1`, `pinned date`, `verified by: bantuson`).
- **Preserve upstream's directory structure** to keep future upstream merges tractable. The ONLY structural changes in this phase: module path rewrite and `cmd/bumblebee/` → `cmd/pollen/`. (`_windows.go` files come in P2–P4.)
- `VERSION` file: append `-pollen.N` suffix (e.g. `0.1.1-pollen.1`).
- `threat_intel/` ships **empty** except upstream selftest fixtures — catalogs flow through beekeeper's own `catalogs sync`, never duplicated in Pollen (PRD §6.3 "reference" decision). The `--exposure-catalog` CLI surface is preserved for compatibility but empty by default.
- **Zero non-stdlib dependencies added** beyond what upstream already has (keep the supply-chain surface minimal — PRD §10.1).

### Legal & attribution (PRD §7)
- `LICENSE`: verbatim upstream Apache-2.0, unmodified.
- `NOTICE`: the exact text in PRD §7.2 (attributes Perplexity/Bumblebee, states non-affiliation, points general users to upstream, references `github.com/bantuson/beekeeper`).
- `CHANGES.md`: the §7.3 format — Added / Renamed / Modified / Removed sections documenting every delta from the pinned commit.
- **Trademark discipline (FORK-04):** "Bumblebee" appears ONLY in attribution contexts (NOTICE, README "derived from" paragraph, UPSTREAM.md). Never in command names, package names, README headlines, or `cmd/`. Apache-2.0 §6 grants no trademark rights.

### Self-defense (PRD §10.2)
- Reproducible builds: `-trimpath -buildvcs=false -mod=readonly` (mirror beekeeper's `.goreleaser.yaml` + `Makefile`).
- Sigstore/cosign **keyless** signing via GitHub Actions OIDC — the cert identity will be `github.com/bantuson/pollen/.github/workflows/...` (new repo; do NOT copy beekeeper's identity string).
- CycloneDX **SBOM** per release (syft, as in beekeeper's `.goreleaser.yaml`), recording the source upstream commit.
- Two-account release approval discipline noted for the tag step (same as beekeeper).

### Differential & selftest (PRD §9.1, §9.2)
- `pollen selftest` (renamed from `bumblebee selftest`) runs upstream's embedded fixture tests; passes on all three OSes.
- Differential test runs BOTH `pollen` and upstream `bumblebee` against the same fixture on Linux + macOS and asserts byte-for-byte identical NDJSON. **Determinism risk:** NDJSON may carry timestamps, host-specific fields, or nondeterministic ordering — RESEARCH must establish how upstream makes output deterministic (or what normalization the differential harness needs). This is the load-bearing guard for every later phase.

### Claude's Discretion
- Exact CI job names / file layout of `.github/workflows/ci.yml` and the release workflow (mirror beekeeper's conventions where sensible).
- How the differential test obtains the upstream `bumblebee` binary in CI (build from the pinned upstream commit vs download a release artifact).
- Test-fixture directory layout for the differential/selftest inputs.
- Whether to add an `upstream` git remote in the pollen repo for the sync workflow now or in P5.
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone PRD (authoritative scope)
- `beekeeper-m2-prd.md` §4 (scope), §5 (architecture + module structure), §6 (upstream sync discipline + threat_intel reference decision), §7 (legal/attribution: LICENSE/NOTICE/CHANGES/trademark), §9.1–9.2 (selftest + differential test), §10 (self-defense), §11 M2.1 (this phase's sub-phase definition)

### Upstream source (the fork base)
- `github.com/perplexityai/bumblebee` @ tag **v0.1.1** — VERIFY actual structure: `cmd/`, `internal/` layout, module path, NDJSON output package, selftest mechanism, existing `.github/workflows/`, `go.mod`, version ldflags var. PRD §5.2 ASSUMES a layout (`cmd/bumblebee/`, `internal/resolver/`, `internal/output/ndjson.go`) but this must be confirmed against the real repo, not assumed.

### Beekeeper self-defense patterns to mirror (same repo)
- `.goreleaser.yaml` — repro build flags, cosign signing, syft SBOM stanza
- `.github/workflows/release.yml` — cosign keyless OIDC pattern (adapt identity to pollen)
- `.github/workflows/ci.yml` — 3-OS matrix + go test -race shape
- `Makefile` — build flag conventions
- `docs/THREAT-MODEL.md` — cosign verify-command doc pattern (write the pollen equivalent)
</canonical_refs>

<specifics>
## Specific Ideas

- First tagged release: `v0.1.1-pollen.1` — proves fork hygiene works with NO Windows code yet (PRD §11 M2.1).
- CI matrix axis: `os: [ubuntu-latest, macos-latest, windows-latest]`, `go: ['1.25.x']` (PRD §9.6).
- Per-OS CI jobs: `go vet`, `go test -race ./...`, `pollen selftest`, build with `-ldflags "-X main.Version=..."`; differential test on Linux + macOS only.
- M2.1 CI establishes the matrix on three OSes with **Windows tests explicitly skipped with structured reasons** (Windows functionality arrives P2+) — the skip is intentional and labeled, not silent rot.
</specifics>

<deferred>
## Deferred Ideas

- Windows root resolver, path representation, extension/MCP coverage — Phases 2–4.
- Beekeeper-side `internal/inventory/` integration + compat test + honeypot — Phases 4–5.
- `pollen-self` catalog entries in `beekeeper-self` — Phase 5 (needs stable pollen versions to reference).
- Upstream contribution-back PRs + full sync execution — Phase 5 (UPSTREAM.md *documents* the workflow now; *executing* a sync is later).
- Pollen binary distribution — out of scope (source-only via `go install`); deferred → DIST-01.
</deferred>

---

*Phase: 01-fork-setup-discipline*
*Context gathered: 2026-06-01 via PRD Express Path + cross-repo decision*
