---
phase: 01-fork-setup-discipline
plan: 05
status: complete
completed: 2026-06-02
requirements: [FORK-03, SDEF-02]
---

# Plan 01-05 Summary — First Pollen Release (capstone)

**Objective:** Create the `github.com/Bantuson/pollen` GitHub repo, push the code, confirm the full CI matrix is green via a human checkpoint, then tag `v0.1.1-pollen.1` to trigger the cosign-signed, SLSA-attested, SBOM-bearing release pipeline — and verify the published artifacts.

## Outcome: ✅ Complete

- **Repo:** https://github.com/Bantuson/pollen (public)
- **Release:** https://github.com/Bantuson/pollen/releases/tag/v0.1.1-pollen.1
- **Driven inline** by the orchestrator (cross-repo, outward-facing, CI-gated, multi-turn), not a subagent.

## Task 1 — repo + push + CI-GREEN human checkpoint

- `gh repo create Bantuson/pollen --public`; pushed `main` (origin).
- **Email-privacy fix:** the fork commits were authored with a private gmail (blocked by GH007). Rewrote the fork commits to the GitHub noreply (`152793144+Bantuson@users.noreply.github.com`), preserving upstream's `adel.karimishiraz@perplexity.ai` base-commit authorship. Set local repo identity to noreply.
- Removed the inherited upstream `v0.1.1` tag locally (footgun: `release.yml` triggers on `v*`).
- **First CI run RED** (expected — first-ever cross-platform run). Triaged 4 CI-only failures the Windows dev box structurally cannot catch (`TestDifferential` skips on Windows; release pipeline can't run locally):
  1. `TestDifferential` (ubuntu/macos) — only differing field after normalization was **`scanner_version`** (pollen's Go VCS pseudo-version vs upstream's tagged `v0.1.1`). Fixed: strip `scanner_version` in `normalize()` alongside `scanner_name`. Confirmed byte-identical via local reproduction (built upstream @ pinned SHA, diffed).
  2. go.mod tidy (windows) — `git diff go.sum` failed (zero-dep module, no `go.sum`). Fixed: `shell: bash` + conditional `go.sum` check.
  3. goreleaser `checksums:` → `checksum:` (v2 schema). Validated with `goreleaser check`.
  4. goreleaser `main: ./cmd/pollen` (build defaulted to repo root with no main). Validated with `goreleaser build --snapshot` (all 6 targets).
- **Second CI run GREEN** — `test` on ubuntu/macos/windows + `differential` on Linux+macOS + `govulncheck`, all ✓. Only skip is the intentional Windows differential skip.
- **Human confirmed CI-GREEN** before tagging (the blocking gate; user chose "publish public").

## Task 2 — tag + signed release

- Annotated tag `v0.1.1-pollen.1` on the green HEAD (`005fe06`), pushed → fired `release.yml`.
- First release run RED (goreleaser config bugs above) → fixed → **release GREEN**: `goreleaser` (build + archive + checksum + cosign sign + syft SBOM) + `provenance` (SLSA L3) jobs all ✓.
- Release assets: `checksums.txt`, `checksums.txt.sigstore.json`, `multiple.intoto.jsonl` (SLSA), 6 archives (linux/darwin/windows × amd64/arm64), 6 `*.cdx.json` CycloneDX SBOMs.

## Task 3 — verification (FORK-03 + SDEF-02 final proof)

- **cosign verify-blob → `Verified OK`** against `^https://github.com/Bantuson/pollen/`.
- **Assumption A1 RESOLVED:** the real Sigstore cert subject is `https://github.com/Bantuson/pollen/.github/workflows/release.yml@refs/tags/v0.1.1-pollen.1`. GitHub OIDC uses the **canonical account casing `Bantuson`** (capital B), NOT the lowercase Go module path. The lowercase regexp failed; the corrected one passes. Folded into `docs/THREAT-MODEL.md` + `CHANGES.md`.
- **SLSA L3 provenance** (`multiple.intoto.jsonl`) attached.
- **CycloneDX SBOM** published per archive (CycloneDX 1.6).

## Deviations

- **SDEF-02 "SBOM records the source upstream commit" — accepted via UPSTREAM.md.** syft's default SBOM records artifact composition + per-archive sha256, not the fork-source upstream commit. The pinned upstream SHA `c24089804ee66ece4bec6f14638cb98985389cdb` is the canonical pin-of-record in `UPSTREAM.md` (§6.1) + `CHANGES.md` + `THREAT-MODEL.md`. User chose Accept (no testing impact either way; fork-source provenance architecturally belongs in UPSTREAM.md, not a composition SBOM). A strict variant (custom CycloneDX property + re-release) was offered and declined.
- The published `v0.1.1-pollen.1` tag (`005fe06`) carries the pre-A1-correction `THREAT-MODEL.md` (lowercase regexp); the correction is on `main` (`37c71e5`) for future consumers and the next release. Binaries/signature/SBOM are unchanged and verified.
- **Latent in beekeeper:** the `checksums:`→`checksum:` and missing `main:` goreleaser bugs also exist in beekeeper's `.goreleaser.yaml` (its release has never run). Flagged for the beekeeper activation thread.

## Pollen repo final state

`github.com/Bantuson/pollen` @ `main` (`37c71e5`): forked at pinned SHA with upstream history + `upstream` remote preserved; module `github.com/bantuson/pollen`; `cmd/pollen`; Apache-2.0 attribution; differential + selftest guards; reproducible + cosign-keyless + syft-SBOM + SLSA-L3 release pipeline; signed `v0.1.1-pollen.1` published and verified. **No Windows scanner code yet** (Phase 2).

**Next:** `/gsd-verify-work 1` (UAT), then Phase 2 (Windows Root Resolver).
