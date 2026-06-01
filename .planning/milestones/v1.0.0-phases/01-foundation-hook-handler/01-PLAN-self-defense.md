---
phase: 01-foundation-hook-handler
plan: 03
type: execute
wave: 2
depends_on: [01]
files_modified:
  - Makefile
  - .goreleaser.yaml
  - .github/workflows/release.yml
  - .github/renovate.json
  - SECURITY.md
autonomous: true
requirements: [SFDF-01, SFDF-02, SFDF-03, SFDF-04]
must_haves:
  truths:
    - "make verify-release VERSION=X.Y.Z reproducibly builds and compares artifact hashes"
    - "The release pipeline builds multi-platform binaries with reproducible flags and Sigstore-signs them via cosign v3 keyless OIDC"
    - "Renovate is configured to update go.mod/actions with gomodTidy post-update so go.sum never goes stale"
    - "SECURITY.md documents a responsible disclosure process with a 48h acknowledgment SLA and 90-day coordinated disclosure default"
  artifacts:
    - path: "Makefile"
      provides: "build, test, verify-release targets with reproducible flags"
      contains: "verify-release"
    - path: ".goreleaser.yaml"
      provides: "Reproducible multi-platform build + cosign v3 signing config"
      contains: "mod_timestamp"
    - path: ".github/workflows/release.yml"
      provides: "Tag-triggered GoReleaser + cosign OIDC release workflow"
      contains: "id-token: write"
    - path: ".github/renovate.json"
      provides: "Renovate config pinning go.mod and actions with gomodTidy"
      contains: "gomodTidy"
    - path: "SECURITY.md"
      provides: "Responsible disclosure policy"
      contains: "48"
  key_links:
    - from: ".goreleaser.yaml"
      to: "cosign"
      via: "signs section sign-blob --bundle"
      pattern: "cosign"
    - from: ".github/workflows/release.yml"
      to: ".goreleaser.yaml"
      via: "goreleaser-action release --clean"
      pattern: "goreleaser"
    - from: "Makefile"
      to: ".goreleaser.yaml"
      via: "verify-release invokes reproducible build comparison"
      pattern: "verify-release"
---

<objective>
Land the four Phase 1 self-defense foundations that CLAUDE.md forbids deferring: reproducible builds (SFDF-01), Sigstore/cosign v3 signing (SFDF-02), pinned dependencies with Renovate (SFDF-03), and a published SECURITY.md disclosure policy (SFDF-04). For a security tool, the supply chain IS the product — these ship from v0.1.0.

Purpose: A safety harness that cannot prove its own build integrity is not trustworthy. This plan makes every release binary reproducibly buildable, keylessly signed, dependency-pinned, and accompanied by a responsible disclosure process.
Output: `Makefile`, `.goreleaser.yaml`, `.github/workflows/release.yml`, `.github/renovate.json`, and `SECURITY.md`. This plan touches only build/release/policy files — no Go source — so it runs in parallel with the catalog plan (plan 02) in Wave 2.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/phases/01-foundation-hook-handler/01-CONTEXT.md
@.planning/phases/01-foundation-hook-handler/01-RESEARCH.md
@CLAUDE.md
@.planning/phases/01-foundation-hook-handler/01-01-SUMMARY.md
</context>

<tasks>

<task type="auto">
  <name>Task 1: Makefile with reproducible build + verify-release target</name>
  <read_first>
    - .planning/phases/01-foundation-hook-handler/01-RESEARCH.md (Pitfall 3 reproducible build breakage, GoReleaser config example, SFDF-01 row)
    - CLAUDE.md (Build constraints: -trimpath -buildvcs=false -mod=readonly; make verify-release must work)
    - go.mod (module path and version vars location: internal/version)
  </read_first>
  <files>Makefile</files>
  <action>
    Create a Makefile with these targets. `build`: runs `go build -trimpath -buildvcs=false -mod=readonly -ldflags "-s -w -X github.com/mzansi-agentive/beekeeper/internal/version.Version=$(VERSION) -X github.com/mzansi-agentive/beekeeper/internal/version.Commit=$(shell git rev-parse HEAD) -X github.com/mzansi-agentive/beekeeper/internal/version.Date=$(shell git show -s --format=%cI HEAD)" -o dist/beekeeper ./cmd/beekeeper` — use the COMMIT DATE (`git show -s --format=%cI`), never `$(date)` (Pitfall 3). `test`: `go test -race -count=1 ./...`. `vet`: `go vet ./...`. `verify-release`: requires a VERSION argument (error if empty), performs two independent clean builds of the same source into two output paths, computes sha256 of each, and fails (non-zero exit) if the two hashes differ — this proves byte-for-byte reproducibility locally before relying on GoReleaser. Use `sha256sum` where available with a portable fallback note in a comment; the target must work from the dev machine. Mark `.PHONY` for all targets. Add a comment header citing SFDF-01.

    Pin the toolchain expectation in a comment: the same Go toolchain version (from go.mod `toolchain` directive) must be used for both builds — reproducibility is only guaranteed with a fixed toolchain (RESEARCH Open Question 2).
  </action>
  <verify>
    <automated>node -e "const m=require('fs').readFileSync('Makefile','utf8'); ['verify-release','-trimpath','-buildvcs=false','-mod=readonly','internal/version.Version'].forEach(s=>{if(!m.includes(s)){console.error('MISSING: '+s);process.exit(1)}}); if(m.includes('$(date)')||m.includes('`date`')){console.error('FORBIDDEN wall-clock date in ldflags');process.exit(1)} console.log('Makefile OK')"</automated>
  </verify>
  <acceptance_criteria>
    - `Makefile` defines a `verify-release` target that errors when VERSION is empty
    - The build ldflags use `internal/version.Version`, `.Commit`, and `.Date` and the Date comes from `git show -s --format=%cI` (commit date), NOT `$(date)` or backtick-date
    - Build flags include `-trimpath`, `-buildvcs=false`, and `-mod=readonly`
    - `verify-release` performs two builds and compares sha256 hashes, exiting non-zero on mismatch
    - All targets are declared `.PHONY`
  </acceptance_criteria>
  <done>`make verify-release VERSION=X.Y.Z` reproducibly builds twice and asserts identical hashes; `make build` produces a deterministic binary.</done>
</task>

<task type="auto">
  <name>Task 2: GoReleaser config + cosign v3 release workflow</name>
  <read_first>
    - .planning/phases/01-foundation-hook-handler/01-RESEARCH.md (GoReleaser Release Configuration example, GitHub Actions Release Workflow example, State of the Art cosign v3 row)
    - CLAUDE.md (Release signing: Sigstore/cosign v3 via GitHub Actions OIDC, no long-lived keys; SLSA is full semver but Phase 7)
    - Makefile (ldflags pattern from Task 1 — keep GoReleaser ldflags consistent)
  </read_first>
  <files>.goreleaser.yaml, .github/workflows/release.yml</files>
  <action>
    Create .goreleaser.yaml with `version: 2`. A `builds` section: `env: [CGO_ENABLED=0]`, `goos: [linux, darwin, windows]`, `goarch: [amd64, arm64]`, `mod_timestamp: "{{ .CommitTimestamp }}"` (reproducibility — Pitfall 3), `flags: [-trimpath]`, `ldflags: [-s -w, -X github.com/mzansi-agentive/beekeeper/internal/version.Version={{.Version}}, -X github.com/mzansi-agentive/beekeeper/internal/version.Commit={{.Commit}}, -X github.com/mzansi-agentive/beekeeper/internal/version.Date={{.CommitDate}}]`, `binary: beekeeper`. A `checksums` section with `name_template: "checksums.txt"`. A `signs` section using cosign v3 keyless: `cmd: cosign`, `signature: "${artifact}.sigstore.json"`, `args: [sign-blob, "--bundle=${signature}", "${artifact}", "--yes"]`, `artifacts: checksum` (signing checksums.txt covers all artifacts). An `archives` section: tar.gz with a windows zip override. Do NOT add an `sboms` section or `slsa` provenance — those are SFDF-05, Phase 7 (out of scope per CONTEXT deferred list). `release: { draft: false }`.

    Create .github/workflows/release.yml triggered on `push: tags: ['v*']`. Set `permissions: { contents: write, id-token: write }` (id-token required for cosign OIDC keyless). A `goreleaser` job on `ubuntu-latest`: checkout@v4 with `fetch-depth: 0`, setup-go@v5 with `go-version-file: go.mod`, `sigstore/cosign-installer@v3`, then `goreleaser/goreleaser-action@v7` with `distribution: goreleaser`, `version: '~> v2'`, `args: release --clean`, env `GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}`. No long-lived signing secrets anywhere (CLAUDE.md).
  </action>
  <verify>
    <automated>node -e "const fs=require('fs');const g=fs.readFileSync('.goreleaser.yaml','utf8');const r=fs.readFileSync('.github/workflows/release.yml','utf8');[['goreleaser','mod_timestamp',g],['goreleaser','cosign',g],['goreleaser','sign-blob',g],['release','id-token: write',r],['release','cosign-installer@v3',r],['release','goreleaser-action@v7',r]].forEach(([f,s,c])=>{if(!c.includes(s)){console.error('MISSING in '+f+': '+s);process.exit(1)}}); if(g.includes('slsa')||g.includes('sboms')){console.error('SLSA/SBOM is Phase 7, out of scope');process.exit(1)} console.log('release config OK')"</automated>
  </verify>
  <acceptance_criteria>
    - `.goreleaser.yaml` has `version: 2`, `mod_timestamp: "{{ .CommitTimestamp }}"`, and `flags: [-trimpath]`
    - The `signs` section uses `cosign` with `sign-blob` and `--bundle=${signature}` and `signature: "${artifact}.sigstore.json"` (cosign v3 bundle format)
    - GoReleaser ldflags use `{{.CommitDate}}` for Date (not build wall-clock)
    - `.goreleaser.yaml` does NOT contain `sboms` or `slsa` (deferred to Phase 7)
    - `.github/workflows/release.yml` triggers on `v*` tags, sets `id-token: write`, and uses `sigstore/cosign-installer@v3` and `goreleaser/goreleaser-action@v7`
    - No long-lived signing key secret is referenced (only `GITHUB_TOKEN`)
  </acceptance_criteria>
  <done>Tagging `vX.Y.Z` triggers a reproducible multi-platform GoReleaser build with cosign v3 keyless Sigstore signing and no long-lived keys.</done>
</task>

<task type="auto">
  <name>Task 3: Renovate dependency-pinning config + SECURITY.md</name>
  <read_first>
    - .planning/phases/01-foundation-hook-handler/01-RESEARCH.md (Pitfall 4 go mod verify after Renovate, SFDF-03 and SFDF-04 rows, Security Domain)
    - CLAUDE.md (Self-Defense Non-Negotiables: pinned deps + SECURITY.md in Phase 1)
  </read_first>
  <files>.github/renovate.json, SECURITY.md</files>
  <action>
    Create .github/renovate.json: extend `config:recommended`, enable the `gomod` and `github-actions` managers, set `"postUpdateOptions": ["gomodTidy"]` (CRITICAL — Pitfall 4: without this, Renovate PRs leave go.sum stale and fail `go mod verify`), require human review (no automerge: `"automerge": false`), and group dependency updates with a reasonable schedule. Add a comment field noting this satisfies SFDF-03 and that all updates require human review.

    Create SECURITY.md documenting the responsible disclosure process per SFDF-04: a "Reporting a Vulnerability" section instructing reporters to use GitHub private security advisories (the intake mechanism) rather than public issues; an explicit 48-hour acknowledgment SLA; a 90-day coordinated disclosure default window; a statement of supported versions (v0.1.0+); and contact attribution to Mzansi Agentive Pty Ltd / Mfanafuthi Mhlanga. Keep it concise and standard-OSS-CVD shaped.
  </action>
  <verify>
    <automated>node -e "const fs=require('fs');const j=JSON.parse(fs.readFileSync('.github/renovate.json','utf8'));const s=fs.readFileSync('SECURITY.md','utf8');if(!JSON.stringify(j).includes('gomodTidy')){console.error('renovate: missing gomodTidy');process.exit(1)}if(!s.includes('48')||!s.includes('90')){console.error('SECURITY.md missing 48h/90-day SLA');process.exit(1)}if(!/security advisor/i.test(s)){console.error('SECURITY.md missing advisory intake');process.exit(1)}console.log('SFDF-03/04 OK')"</automated>
  </verify>
  <acceptance_criteria>
    - `.github/renovate.json` is valid JSON, enables the `gomod` manager, and sets `"postUpdateOptions": ["gomodTidy"]`
    - Renovate config does not enable automerge (human review required for SFDF-03)
    - `SECURITY.md` contains a "Reporting a Vulnerability" section referencing GitHub private security advisories
    - `SECURITY.md` states a 48-hour acknowledgment SLA and a 90-day coordinated disclosure default
    - `SECURITY.md` lists supported versions and contact attribution
  </acceptance_criteria>
  <done>Renovate keeps dependencies pinned with gomodTidy and human review; SECURITY.md publishes the responsible disclosure process.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| CI/CD → released artifact | The release pipeline is the integrity anchor for every distributed binary |
| dependency feed → go.mod | Automated dependency updates can introduce malicious or unpinned code |
| public → maintainer | Vulnerability reports must reach the maintainer through a private, defined channel |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-03-01 | Tampering | Release binary substitution / supply-chain compromise | mitigate | cosign v3 keyless Sigstore signing of checksums.txt via OIDC (SFDF-02) + reproducible builds (`make verify-release`, SFDF-01) let anyone independently rebuild and verify |
| T-03-02 | Repudiation | No provenance for who/what produced a release | mitigate | GitHub Actions OIDC identity bound into the Sigstore certificate; full SLSA L3 provenance deferred to Phase 7 (SFDF-05) and explicitly out of scope here |
| T-03-03 | Tampering | Malicious dependency update | mitigate | Renovate with `automerge: false` (human review) + CI `go mod verify` + `gomodTidy` keeps go.sum consistent (SFDF-03) |
| T-03-04 | Elevation of Privilege | Long-lived signing key theft | mitigate | Keyless OIDC signing — no long-lived keys exist to steal (CLAUDE.md locked decision) |
| T-03-05 | Information Disclosure | Uncoordinated public vulnerability disclosure | mitigate | SECURITY.md mandates private advisory intake with 48h ack / 90-day coordinated disclosure (SFDF-04) |
| T-03-06 | Tampering | Non-reproducible build masking injected code | mitigate | `-trimpath -buildvcs=false -mod=readonly`, `mod_timestamp: CommitTimestamp`, commit-date-only ldflags; `verify-release` fails on hash mismatch (SFDF-01) |
</threat_model>

<verification>
- Makefile, .goreleaser.yaml, release.yml, renovate.json, and SECURITY.md all exist with the asserted content
- `make verify-release VERSION=0.0.1-test` (manual/CI smoke) builds twice and reports matching hashes
- No wall-clock timestamps in any ldflags; no long-lived signing secrets in any workflow
- `.goreleaser.yaml` excludes SLSA/SBOM (correctly deferred to Phase 7)
</verification>

<success_criteria>
- Reproducible builds with a working `make verify-release` (SFDF-01)
- cosign v3 keyless Sigstore signing in the release pipeline (SFDF-02)
- Renovate pinning with gomodTidy + human review, complementing CI `go mod verify` (SFDF-03)
- Published SECURITY.md with 48h/90-day disclosure policy (SFDF-04)
</success_criteria>

<output>
After completion, create `.planning/phases/01-foundation-hook-handler/01-03-SUMMARY.md`
</output>
