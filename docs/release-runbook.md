# Beekeeper M2 "Pollen" Release Runbook

**Document:** D-5 maintainer hand-off procedure (Phase 5, Plan 04)
**Scope:** Push both repos (`home-beekeeper/pollen`, `home-beekeeper/beekeeper`), cut four signed
Pollen tags in order, cosign-verify each release, push beekeeper main.
**Executor:** Maintainer (auth-gated, outward-facing steps — NOT the autonomous executor)
**Reference:** `05-04-SUMMARY.md`, `05-RESEARCH.md` RQ-6, `02-04-SUMMARY.md`, `03-03-SUMMARY.md`

---

## Standard release checklist (every release)

> The detailed sections below are the one-time M2 "Pollen" tag sequence (parked).
> This short checklist applies to **every** beekeeper release (v1.1.2, v1.2.0, ...).

A release is cut by pushing a signed semver tag, which triggers
`.github/workflows/release.yml`: GoReleaser builds the multi-platform binaries,
cosign keylessly signs `checksums.txt` (bundle `checksums.txt.sigstore.json`), and
SLSA L3 provenance plus CycloneDX SBOMs are attached. After the release publishes:

- [ ] **Bump the pinned `go install` version in the docs.** Beekeeper pins its own
  install command to an exact tag (no `@latest` — it is the supply-chain practice the
  product enforces, so the docs must not drift to a stale or mutable reference). Update
  `go install github.com/home-beekeeper/beekeeper/cmd/beekeeper@vX.Y.Z` to the new tag in
  **both repos** (run `grep -rn 'cmd/beekeeper@v' .` in each to find every occurrence):
  - `beekeeper`: `README.md`
  - `beekeeper-web`: `web/content/docs/{getting-started,installation,troubleshooting}.mdx`,
    `web/content/blog/{introducing-beekeeper,install-posture-and-the-corpus}.mdx`, and
    `web/components/home/{install-chip,quickstart-card,how-it-works}.tsx`
- [ ] **Bump the version badges + changelog** in beekeeper-web so the site, the pinned
  install command, and the published release all agree on the version.
- [ ] **Confirm the release is immutable.** GitHub immutable releases / tag protection
  should be enabled on `home-beekeeper/beekeeper` so a published tag cannot be
  overwritten. (The installer's cosign verification is the runtime tamper defense;
  tag immutability is the upstream one — the two are complementary.)
- [ ] **Spot-check the installers** still resolve the new tag and pass verification:
  `BEEKEEPER_VERSION=vX.Y.Z` is honored, the cosign step prints `Verified OK`, and an
  unpinned install older than the cool-down window prints the advisory (warn, not block).

---

## Preconditions

Verify ALL of the following before starting:

- [ ] `../pollen` local work is committed:
  - pollen.2 release-prep commit `c94b271` present (`git -C ../pollen log --oneline`)
  - pollen.3 release-prep commit `19695e3` present
  - pollen.4 work and release-prep commit `b906404` (or `a9db7b3` — see Step 4 note) present
  - pollen.5 VERSION / CHANGES.md / UPSTREAM.md commits present (Phase 5 Plan 01 done)
- [ ] beekeeper BKINT-02 CI edit committed (`internal/scan/pollen_version.go` + `.github/workflows/ci.yml` — Phase 5 Plan 04 Task 1 done)
- [ ] `gh auth status` shows authenticated with push access to the `home-beekeeper` org
- [ ] `cosign version` returns v3.x
- [ ] No uncommitted changes in either repo (`git status` is clean)

---

## Sequencing Rationale

**Pollen must be pushed and tagged BEFORE beekeeper is pushed** (Pitfall 3 from
`05-RESEARCH.md`). The beekeeper CI step
`go install github.com/home-beekeeper/pollen/cmd/pollen@v0.1.1-pollen.4` resolves from the
Go module proxy, which requires the module to be publicly available with that tag on
GitHub. If beekeeper CI runs before the pollen tag exists, the `go install` step fails
with "no such module".

Correct order: Steps 1–5 (pollen push + tags + cosign verify) → Step 6 (beekeeper push).

---

## Step 1 — Create the beekeeper GitHub repository (if it does not exist)

**Auth gate:** requires `gh` authenticated with repo-create/push access to the `home-beekeeper` org.

```bash
# Option A: one-liner (creates the repo, adds origin remote, and pushes main)
gh repo create home-beekeeper/beekeeper --public --source=. --push

# Option B: if the GitHub repo was already created via the web UI
git remote add origin https://github.com/home-beekeeper/beekeeper.git
# (do NOT push yet — beekeeper push is Step 6, after pollen tags are live)
```

Note: `home-beekeeper/beekeeper` (lowercase) matches the `github.com/home-beekeeper/beekeeper`
module path in `go.mod`. GitHub normalises display casing; the URL is always lowercase.

---

## Step 2 — Push pollen main and wait for 3-OS CI green

**Auth gate:** requires push access to `github.com/home-beekeeper/pollen` (capital B).

```bash
# Push all unpushed pollen commits (currently 14+ ahead of origin/main)
git -C C:/Users/Bantu/mzansi-agentive/pollen push origin main

# Wait for the 3-OS (ubuntu-latest, macos-latest, windows-latest) CI matrix to pass
gh -R home-beekeeper/pollen run watch
```

**Expected outcome:** All CI jobs green, including `TestParityAllEcosystems` and
`TestWindowsBaselineRoots` on `windows-latest` (zero skips).

---

## Step 3 — Cut and push tag v0.1.1-pollen.2

**Confirm commit hash first:**

```bash
git -C C:/Users/Bantu/mzansi-agentive/pollen log --oneline | grep c94b271
# Expected: c94b271 release(02-04): prepare v0.1.1-pollen.2 — Windows root resolver ...
```

**Tag and push:**

```bash
git -C C:/Users/Bantu/mzansi-agentive/pollen tag -a v0.1.1-pollen.2 c94b271 \
  -m "Pollen v0.1.1-pollen.2 — Windows root resolver (WRES-01, WRES-02, PTEST-01)"

git -C C:/Users/Bantu/mzansi-agentive/pollen push origin v0.1.1-pollen.2
```

**Wait for the release job to complete (cosign + SBOM + SLSA L3):**

```bash
gh -R home-beekeeper/pollen run watch
```

**Cosign verify (Step 7 covers all four releases — you may batch the verify at the end,
or verify each one inline. Inline is recommended for early failure detection):**

See Step 7 for the exact cosign verify command. Download
`checksums.txt` and `checksums.txt.sigstore.json` from the
`v0.1.1-pollen.2` GitHub Release assets page first.

---

## Step 4 — Cut and push tag v0.1.1-pollen.3

**Confirm commit hash first:**

```bash
git -C C:/Users/Bantu/mzansi-agentive/pollen log --oneline | grep 19695e3
# Expected: 19695e3 release(03-03): prepare v0.1.1-pollen.3 — Windows path representation ...
```

**Tag and push:**

```bash
git -C C:/Users/Bantu/mzansi-agentive/pollen tag -a v0.1.1-pollen.3 19695e3 \
  -m "Pollen v0.1.1-pollen.3 — Windows path representation (WPATH-01, WPATH-02)"

git -C C:/Users/Bantu/mzansi-agentive/pollen push origin v0.1.1-pollen.3
```

**Wait for release job:**

```bash
gh -R home-beekeeper/pollen run watch
```

---

## Step 5 — Cut and push tag v0.1.1-pollen.4

### Decision point: which commit to tag as pollen.4?

Two candidate commits exist in the pollen repo:

| Hash | Description | Includes WEXT fix? |
|------|-------------|-------------------|
| `a9db7b3` | release-prep "Windows extension & MCP coverage" (WEXT-01/02/03) — the Phase 4 release-prep commit | No — the `.vscode-oss` labelling fix lands after this |
| `b906404` | "label .vscode-oss extensions as vscodium" — the WR-01 production fix committed after `a9db7b3` | Yes |

**Recommendation: tag pollen.4 at `b906404`** so that the VSCodium extension labelling
fix (WR-01) ships in the pollen.4 release. `a9db7b3` predates this fix and would ship
a release that still misclassifies VSCodium extensions.

**Default (recommended):**

```bash
# Confirm b906404 is present
git -C C:/Users/Bantu/mzansi-agentive/pollen log --oneline | grep b906404
# Expected: b906404 fix(04-...): label .vscode-oss extensions as vscodium ...

git -C C:/Users/Bantu/mzansi-agentive/pollen tag -a v0.1.1-pollen.4 b906404 \
  -m "Pollen v0.1.1-pollen.4 — Windows extension & MCP coverage (WEXT-01, WEXT-02, WEXT-03, WR-01)"

git -C C:/Users/Bantu/mzansi-agentive/pollen push origin v0.1.1-pollen.4
```

**Alternative (only if b906404 introduces regressions):**

```bash
# Fallback: tag at a9db7b3 (omits WR-01 VSCodium fix)
git -C C:/Users/Bantu/mzansi-agentive/pollen tag -a v0.1.1-pollen.4 a9db7b3 \
  -m "Pollen v0.1.1-pollen.4 — Windows extension & MCP coverage (WEXT-01, WEXT-02, WEXT-03)"
git -C C:/Users/Bantu/mzansi-agentive/pollen push origin v0.1.1-pollen.4
```

**Wait for release job:**

```bash
gh -R home-beekeeper/pollen run watch
```

---

## Step 5b — Cut and push tag v0.1.1-pollen.5

Tag pollen.5 at the pollen HEAD **after** Phase 5 Plan 01 commits land
(VERSION bump to 0.1.1-pollen.5, CHANGES.md pollen.5 section, UPSTREAM.md delta).
**Do NOT tag at `a9db7b3`** — that is the Phase 4 HEAD before Phase 5 local work
(Pitfall 6, `05-RESEARCH.md`).

```bash
# Confirm Phase 5 pollen work is the latest commit
git -C C:/Users/Bantu/mzansi-agentive/pollen log --oneline -3
# The top commit should be the pollen.5 release-prep (VERSION/CHANGES/UPSTREAM.md)

git -C C:/Users/Bantu/mzansi-agentive/pollen tag -a v0.1.1-pollen.5 HEAD \
  -m "Pollen v0.1.1-pollen.5 — Milestone close (SYNC-01, BKINT-02, PTEST-05, SDEF-01)"

git -C C:/Users/Bantu/mzansi-agentive/pollen push origin v0.1.1-pollen.5
```

**Wait for release job:**

```bash
gh -R home-beekeeper/pollen run watch
```

---

## Step 6 — Cosign verify all four releases

For **each** of the four releases (pollen.2, pollen.3, pollen.4, pollen.5):

**a) Download the release assets from GitHub:**

```bash
# Replace v0.1.1-pollen.N with the tag being verified
gh -R home-beekeeper/pollen release download v0.1.1-pollen.N \
  --pattern "checksums.txt" \
  --pattern "checksums.txt.sigstore.json" \
  --dir ./verify-pollen-N
```

**b) Verify the cosign bundle:**

```bash
cosign verify-blob \
  --bundle ./verify-pollen-N/checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github\.com/home-beekeeper/pollen/' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  ./verify-pollen-N/checksums.txt
```

**Expected output:** `Verified OK`

**Identity note (org migration):**
The `--certificate-identity-regexp` must match the release workflow's GitHub Actions
OIDC identity under the `home-beekeeper` org. Org and repo slugs are lowercase, so use
`home-beekeeper/pollen` exactly as shown. The prior capital-B `Bantuson` requirement
(Pitfall 4) applied only while the repo lived under a personal account and no longer
applies. A mismatch causes: `Error: none of the expected identities matched what was in the certificate`.

**c) Confirm SLSA L3 provenance and CycloneDX SBOM are attached to the release:**

```bash
gh -R home-beekeeper/pollen release view v0.1.1-pollen.N --json assets \
  | jq -r '.assets[].name' | grep -E '(slsa|cdx|sigstore)'
# Expected: files matching *.intoto.jsonl (SLSA), *.cdx.json (SBOM), checksums.txt.sigstore.json
```

---

## Step 7 — Push beekeeper main

**Only after all four pollen tags are live and cosign-verified (Steps 3–6).**

```bash
# From the beekeeper repo root
git push origin main

# Wait for beekeeper CI (3-OS matrix: ubuntu-latest, macos-latest, windows-latest)
gh run watch
```

**Expected CI behaviour:**
- "Install Pollen (BKINT-02 — pinned binary for inventory tests)" step succeeds on all 3 OSes
  (`go install github.com/home-beekeeper/pollen/cmd/pollen@v0.1.1-pollen.4` resolves now that
  the pollen tag is live)
- `go test -v -race ./...` passes with zero `t.Skip` in `internal/scan/`
- All `TestScanWithBumblebee`, `TestScanWindowsShapedRecord`, `TestScanPollenUnavailable`,
  `TestPollenCompatibility` pass on `windows-latest` (zero-skip baseline, BKINT-02 complete)

---

## Post-Release Verification Checklist

After all steps complete:

- [ ] `gh -R home-beekeeper/pollen release list` shows four releases: `v0.1.1-pollen.2`, `.3`, `.4`, `.5`
- [ ] Each release has: cosign `.sigstore.json` bundle, CycloneDX `.cdx.json` SBOM(s), SLSA `.intoto.jsonl` provenance
- [ ] `cosign verify-blob` returned `Verified OK` for all four releases (Step 6)
- [ ] Beekeeper CI green on `ubuntu-latest`, `macos-latest`, `windows-latest`
- [ ] Zero `t.Skip` in `internal/scan/` on `windows-latest` (BKINT-02 complete, D-7)
- [ ] `gh -R home-beekeeper/beekeeper run list` shows CI green for the beekeeper push

---

## Tag-to-Commit Reference Table

| Tag | Commit | Phase | Description |
|-----|--------|-------|-------------|
| `v0.1.1-pollen.2` | `c94b271` | Phase 02 | Windows root resolver (WRES-01, WRES-02, PTEST-01) |
| `v0.1.1-pollen.3` | `19695e3` | Phase 03 | Windows path representation (WPATH-01, WPATH-02) |
| `v0.1.1-pollen.4` | `b906404` (recommended) / `a9db7b3` (fallback) | Phase 04 | Windows extension & MCP coverage (WEXT-01/02/03, WR-01) |
| `v0.1.1-pollen.5` | HEAD after Phase 5 Plan 01 | Phase 05 | Milestone close (SYNC-01, BKINT-02, PTEST-05, SDEF-01) |

---

## Troubleshooting

**`cosign verify-blob` returns "none of the expected identities matched"**
Check: does `--certificate-identity-regexp` exactly match the release workflow
identity under the `home-beekeeper` org (lowercase, `home-beekeeper/pollen`)?
See the Step 6 identity note.

**`go install github.com/home-beekeeper/pollen/cmd/pollen@v0.1.1-pollen.4` fails in beekeeper CI**
The pollen tag is not yet live on GitHub. Ensure Steps 3–5 (tag push + release job) completed
before running Step 7 (beekeeper push). The `go install` step resolves from the Go module proxy,
which caches the module only after it is publicly tagged (Pitfall 3).

**GoReleaser fails on old-commit tag**
GoReleaser builds from the tag's target commit (not HEAD). Tagging an old commit is safe —
GoReleaser reads the tag reference directly. If the release job fails, check the GoReleaser
logs; the most common cause is a missing `GITHUB_TOKEN` permission (needs `contents: write`).

**`gh run watch` shows a failed CI run after pollen push (Step 2)**
Check the `windows-latest` runner logs. Ensure `TestWindowsBaselineRoots` and
`TestParityAllEcosystems` pass (these were the Phase 2/3 zero-skip targets). If they fail,
do NOT proceed to cutting tags until the failure is diagnosed and fixed.
