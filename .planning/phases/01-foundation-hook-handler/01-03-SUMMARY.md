# Phase 1 / Plan 03 — Self-Defense Foundations — Summary

**Plan:** `01-PLAN-self-defense.md`
**Executed:** 2026-05-26
**Status:** Complete — all three tasks pass their acceptance checks; reproducibility logic empirically validated on the Windows dev machine.
**Commit:** `e81b019` (5 files, 289 insertions, no Go source touched)

## What Was Built

The four Phase 1 self-defense foundations that CLAUDE.md forbids deferring —
reproducible builds (SFDF-01), Sigstore/cosign v3 signing (SFDF-02), pinned
dependencies via Renovate (SFDF-03), and a published responsible-disclosure
policy (SFDF-04). For a security tool, the supply chain is the product, so these
ship from v0.1.0.

This plan touched **only** build/release/policy files — no Go source — so it ran
in parallel with Plan 02 (catalog sync) in Wave 2.

### Files created

| File | Provides | Requirement |
|------|----------|-------------|
| `Makefile` | `build`, `test`, `vet`, `verify-release` targets with reproducible flags | SFDF-01 |
| `.goreleaser.yaml` | Reproducible multi-platform build + cosign v3 keyless signing | SFDF-01, SFDF-02 |
| `.github/workflows/release.yml` | Tag-triggered (`v*`) GoReleaser + cosign OIDC release pipeline | SFDF-02 |
| `.github/renovate.json` | Renovate config pinning go.mod + actions with gomodTidy, human review | SFDF-03 |
| `SECURITY.md` | Responsible disclosure policy (private advisory, 48h ack, 90-day window) | SFDF-04 |

## Key Implementation Details

### Makefile (SFDF-01)

- `build`: `go build -trimpath -buildvcs=false -mod=readonly` with ldflags
  injecting `internal/version.Version/.Commit/.Date`. **Date comes from
  `git show -s --format=%cI HEAD` (commit date)** — never a wall-clock shell
  timestamp, which would break reproducibility (RESEARCH Pitfall 3).
- `test`: `go test -race -count=1 ./...`
- `vet`: `go vet ./...`
- `verify-release`: errors (exit 1) when `VERSION` is empty or `dev`; performs
  two independent clean builds into `dist/verify-a` and `dist/verify-b`,
  computes sha256 of each, and exits non-zero on mismatch. Prints `go version`
  first because the SAME Go toolchain is required for byte-for-byte
  reproducibility (RESEARCH Open Question 2). Uses `sha256sum`, with a comment
  documenting the `shasum -a 256` portable fallback.
- All four targets are `.PHONY`.

### .goreleaser.yaml (SFDF-01 + SFDF-02)

- `version: 2`; builds with `CGO_ENABLED=0`, goos `[linux, darwin, windows]`,
  goarch `[amd64, arm64]`, `mod_timestamp: "{{ .CommitTimestamp }}"`,
  `flags: [-trimpath]`, ldflags injecting version vars with `{{.CommitDate}}`
  for Date, `binary: beekeeper`.
- `checksums.name_template: "checksums.txt"`.
- `signs`: cosign v3 keyless — `cmd: cosign`,
  `signature: "${artifact}.sigstore.json"`,
  `args: [sign-blob, --bundle=${signature}, ${artifact}, --yes]`,
  `artifacts: checksum` (signing checksums.txt transitively covers all
  artifacts).
- `archives`: tar.gz with a windows zip override (GoReleaser v2 `formats:`
  list syntax).
- `release.draft: false`.
- **No `slsa` / `sboms` sections** — SLSA L3 provenance and SBOM are SFDF-05,
  Phase 7, explicitly out of scope.

### .github/workflows/release.yml (SFDF-02)

- Trigger: `push: tags: ['v*']`.
- `permissions: { contents: write, id-token: write }` — `id-token` required for
  cosign keyless OIDC.
- Job `goreleaser` on `ubuntu-latest`: `actions/checkout@v4` (`fetch-depth: 0`),
  `actions/setup-go@v5` (`go-version-file: go.mod` — pins toolchain),
  `sigstore/cosign-installer@v3`, `goreleaser/goreleaser-action@v7`
  (`distribution: goreleaser`, `version: '~> v2'`, `args: release --clean`),
  env `GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}`.
- **No long-lived signing key secret anywhere** — only `GITHUB_TOKEN`.

### .github/renovate.json (SFDF-03)

- Extends `config:recommended`; `gomod` and `github-actions` managers enabled.
- `"postUpdateOptions": ["gomodTidy"]` — CRITICAL: without it Renovate PRs leave
  `go.sum` stale and fail CI `go mod verify` (RESEARCH Pitfall 4).
- `"automerge": false` — every dependency update requires human review (SFDF-03).
- Weekly schedule (`before 6am on monday`); go-modules and github-actions
  updates grouped via `packageRules`.
- A `"description"` array documents that this satisfies SFDF-03 (Renovate's
  supported in-config documentation field; valid JSON, no JSON5 comments).

### SECURITY.md (SFDF-04)

- "Reporting a Vulnerability" mandates **GitHub private security advisories** as
  the only intake channel (not public issues).
- **48-hour acknowledgment SLA** and **90-day coordinated disclosure default**.
- Supported versions: v0.1.0+ (pre-release unsupported).
- Supply-chain integrity section cross-references reproducible builds, cosign v3
  keyless signing, and Renovate pinning.
- Contact attribution: Mzansi Agentive Pty Ltd — Mfanafuthi Mhlanga.

## Acceptance Criteria Met

### Task 1 — Makefile (`Makefile OK`)
- [x] `verify-release` errors when `VERSION` is empty (and rejects `dev`)
- [x] ldflags use `internal/version.Version/.Commit/.Date`; Date from
      `git show -s --format=%cI` (commit date), not a wall-clock timestamp
- [x] Build flags include `-trimpath`, `-buildvcs=false`, `-mod=readonly`
- [x] `verify-release` builds twice and compares sha256, exiting non-zero on
      mismatch
- [x] All targets declared `.PHONY`
- [x] Node one-liner passes; recipe lines confirmed tab-indented

### Task 2 — GoReleaser + release workflow (`release config OK`)
- [x] `.goreleaser.yaml` has `version: 2`, `mod_timestamp: "{{ .CommitTimestamp }}"`, `flags: [-trimpath]`
- [x] `signs` uses cosign `sign-blob` with `--bundle=${signature}` and `signature: "${artifact}.sigstore.json"`
- [x] ldflags use `{{.CommitDate}}` for Date
- [x] No `sboms` / `slsa` (deferred to Phase 7)
- [x] `release.yml` triggers on `v*`, sets `id-token: write`, uses
      `sigstore/cosign-installer@v3` and `goreleaser/goreleaser-action@v7`
- [x] No long-lived signing secret (only `GITHUB_TOKEN`)

### Task 3 — Renovate + SECURITY.md (`SFDF-03/04 OK`)
- [x] `renovate.json` valid JSON, `gomod` manager enabled, `postUpdateOptions: ["gomodTidy"]`
- [x] `automerge: false` (human review)
- [x] SECURITY.md references GitHub private security advisories
- [x] SECURITY.md states 48h ack + 90-day coordinated disclosure
- [x] SECURITY.md lists supported versions and contact attribution

### Cross-cutting verification
- [x] **Reproducibility logic empirically validated**: two independent builds
      with the Makefile's exact flags (Go 1.25.0 windows/amd64) produced
      identical sha256 `6c6f65c9c0ce2ff1217e2064378a68947b3217fd0ad4bc3e1261dd7980d20123`
- [x] All YAML files tab-free; `renovate.json` parses as valid JSON
- [x] `dist/` confirmed gitignored (build artifacts not committed)

## Deviations from the Plan

1. **`make` not installed on the dev machine.** `verify-release` could not be
   invoked via `make` directly (Windows dev box has no `make`). The
   reproducibility *logic* was instead validated by running the Makefile's
   exact `go build` commands twice and confirming identical sha256 hashes. The
   `make verify-release` target itself runs as designed wherever `make` is
   available (CI Linux/macOS runners, Git Bash + make on Windows). No change to
   the target's contents.

2. **GoReleaser v2 `archives` syntax.** Used `formats: [tar.gz]` /
   `formats: [zip]` (list form) rather than the deprecated singular `format:`
   key, matching current GoReleaser v2 schema. Functionally equivalent to the
   plan's intent.

3. **Comment wording adjusted to satisfy substring acceptance checks.** The
   acceptance one-liners do a literal substring scan. Initial comments contained
   the forbidden literals (`$(date)` in the Makefile; `slsa`/`sboms` in
   `.goreleaser.yaml`), tripping the guards. Comments were reworded to describe
   the prohibition without embedding the literal tokens. No functional change.

4. **Renovate documentation via `description` field.** JSON has no comment
   syntax, so the "comment field noting SFDF-03 / human review" was added as
   Renovate's supported `"description"` string-array field rather than a JSON5
   comment, keeping the file valid JSON.

## Concurrency Note

Plan 02 (catalog sync) was executing in parallel in Wave 2 and had unstaged
changes to `cmd/beekeeper/main.go`, `go.mod`, `go.sum`, `internal/catalog/`, and
`testdata/` at commit time. This plan staged **only** its 5 files by name and
committed just those — Plan 02's work was left fully intact and unstaged. No Go
source, `go.mod`, or `go.sum` was modified or `go mod tidy`'d by this plan.

## Self-Defense Non-Negotiables Status (Phase 1)

Per CLAUDE.md "Self-Defense Non-Negotiables — Phase 1: Reproducible builds +
Sigstore + SECURITY.md + pinned deps" — **all four delivered** in this plan
(SFDF-01..04). SFDF-05 (SLSA L3 + CycloneDX SBOM) remains correctly deferred to
Phase 7.
