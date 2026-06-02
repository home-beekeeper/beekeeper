# Plan 02-04 Summary — Release v0.1.1-pollen.2 (PREPARED, tag/sign DEFERRED)

**Status:** Task 1 complete (local prep). Tasks 2–3 (push, tag, sign, verify) **deferred to milestone (M2) close** by maintainer decision (this session).
**Plan:** 02-04 (wave 3, autonomous: false — release checkpoint)
**Requirements:** none directly (this plan is the Success-Criterion-4 release gate)

## What was done (Task 1 — local, reversible)

- `../pollen/VERSION` bumped `0.1.1-pollen.1` → `0.1.1-pollen.2`.
- `../pollen/CHANGES.md` gained a `## v0.1.1-pollen.2 (2026-06-02) — Windows root resolver` section recording the **actual** files (`cmd/pollen/roots_windows.go`, `roots_notwindows.go`, `roots_windows_test.go`, `parity_test.go`, `testdata/parity-fixture/`, the `roots.go` `case "windows":` + `isBroadHomeRoot` changes, the 6 flipped `main_test.go` skips) — NOT the PRD-draft `internal/resolver/resolver_windows.go` name. The section is explicitly marked **"prepared, not yet tagged."**
- Committed locally in pollen: `c94b271` `release(02-04): prepare v0.1.1-pollen.2 — Windows root resolver (tag/sign deferred to M2 close)`.
- Local sanity: `go build ./...`, `go vet ./cmd/pollen/`, `go test ./cmd/pollen/` all green on the Windows dev host.

## What is DEFERRED (Tasks 2–3 — outward-facing, milestone-close)

The maintainer chose to batch the signed release to end-of-M2. The following are **NOT done** and remain a tracked obligation:

- **Push** the 4 local commits (`2c202ef`, `eba8e4c`, `833d29d`, `c94b271`) to `origin` (`github.com/Bantuson/pollen`). `main` is currently **4 ahead of origin/main**, unpushed.
- **Confirm** the 3-OS CI matrix green on the pushed commit — specifically `TestWindowsBaselineRoots` + `TestParityAllEcosystems` passing on `windows-latest` (no skip), and `TestDifferential` green on ubuntu+macos.
- **Tag + sign** `v0.1.1-pollen.2` (triggers `release.yml`: keyless cosign OIDC + CycloneDX SBOM).
- **Verify** the Sigstore signature + SBOM.

### Exact commands to cut the release at M2 close

```bash
# from C:\Users\Bantu\mzansi-agentive\pollen
git push origin main                       # pushes the 4 Phase-2 commits
gh -R Bantuson/pollen run watch            # wait: 3-OS test + differential jobs all green
git tag -a v0.1.1-pollen.2 -m "Pollen v0.1.1-pollen.2 — Windows root resolver (WRES-01, WRES-02, PTEST-01)"
git push origin v0.1.1-pollen.2            # triggers release.yml (cosign + SBOM)
gh -R Bantuson/pollen run watch            # wait for the release job
# verify (download checksums.txt + .sigstore.json from the release first):
cosign verify-blob --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github\.com/Bantuson/pollen/' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' checksums.txt
# expect: Verified OK ; confirm CycloneDX SBOM attached to the v0.1.1-pollen.2 release
```

## Cross-repo

All edits + the future tag are in the sibling repo `../pollen` (`C:\Users\Bantu\mzansi-agentive\pollen`), committed via explicit `git -C ../pollen`. No `../pollen` paths were staged into beekeeper.

## Phase-2 Success Criteria status

- SC1 (8 ecosystems discovered on Windows): ✅ (02-01, tested in 02-02)
- SC2 (parity test green on 3 OSes, endpoint.os differs): ✅ (02-03, `TestParityAllEcosystems` PASS)
- SC3 (differential still green on Linux+macOS): ✅ (no drift — Windows code build-tag-isolated; `normalize_diff.go` untouched)
- SC4 (v0.1.1-pollen.2 tagged + signed; Windows CI no longer skips root-resolver tests): **PARTIAL** — skips flipped ✅; signed tag **deferred to M2 close** ⏸

## Self-Check: PASSED (for the autonomous scope; release intentionally deferred)
