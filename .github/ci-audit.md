# Beekeeper CI/CD Security Audit (Phase 1, read-only)

Date: 2026-06-16
Scope: every file under `.github/`, plus repository and organization Actions
settings reachable with the available token.
Author: automated audit, pending maintainer review.
Status: AUDIT ONLY. No workflow or configuration file was modified in this phase.

## Governing principle

Beekeeper exists to catch poisoned workflows, OIDC token theft, malicious
actions, and secret exfiltration in agent and developer pipelines. Those same
attack classes apply to Beekeeper's own CI. This audit grades the project's
pipeline against the control set it would expect any hardened repository to
meet, and it does not credit-grade: where a control is missing it is marked
ABSENT, and where a control is already correct it is marked PRESENT and left
alone in Phase 2.

## Files reviewed

| Path | Type | Reviewed |
|------|------|----------|
| `.github/workflows/ci.yml` | Workflow (CI) | yes |
| `.github/workflows/release.yml` | Workflow (release) | yes |
| `.github/renovate.json` | Dependency automation (Renovate) | yes |

Not present: `.github/dependabot.yml`, `.github/CODEOWNERS`, any composite
action (`action.yml` / `action.yaml`), any Claude Code review workflow, any
`pull_request_target` workflow.

## Repository and platform settings (observed via API)

These facts ground several findings below. They were read, not changed.

| Setting | Observed value | Relevance |
|---------|----------------|-----------|
| Repository visibility | public, forking allowed, not archived | Fork PR attack surface is real, not hypothetical |
| Default branch | `main` | Target of the protection gap below |
| Default `GITHUB_TOKEN` permissions | `read` (PR approval disabled) | Positive: backstops the missing in-workflow permissions block, but is an org/repo setting, not an in-workflow guarantee |
| Allowed actions policy | `all`, `sha_pinning_required: false` | Any action from anywhere may run; platform-level SHA pinning is not enforced |
| Fork PR workflow approval | `all_external_contributors` | Positive: fork workflows require maintainer approval before they run |
| Repo-level Actions secrets | none configured | No long-lived repo secrets to leak |
| Repo rulesets | none (`[]`) | See protection gap below |
| Classic branch protection on `main` | none ("Branch not protected") | `main` is currently unprotected |
| Org-level rulesets | not readable (token lacks `admin:org`) | Could not confirm or rule out an org ruleset covering `main` |
| Org-level Actions secrets | not readable (token lacks `admin:org`) | Could not enumerate; no workflow references an org secret regardless |

Honest caveat on the task premise: the directive states that "main is protected
by a ruleset requiring pull requests." At the repository level that is not the
case today. There are no rulesets and no classic branch protection on `main`.
An org-level ruleset cannot be ruled out because the token lacks `admin:org`
scope, but nothing at the repo level enforces pull requests or status checks.
All Phase 2 work will still be delivered as reviewable pull requests, and
control 12 below (populate required status checks) becomes more urgent, not
less, because of this gap.

## Per-workflow analysis

### `.github/workflows/ci.yml`

What it does: cross-platform build, test, vet, tidy check, eBPF bytecode
generation on Linux, three-OS cross-compile gate, and a fan of fuzz and
Sentry kernel jobs that converge on a `release-gate` aggregator.

Triggers: `pull_request` (all branches, no branch filter) and `push` to
`main`. No `pull_request_target`. No `workflow_dispatch`, `schedule`, or
`release`.

Jobs: `test` (matrix over `ubuntu-latest`, `macos-latest`, `windows-latest`),
`fuzz`, `fuzz-ipc`, `fuzz-llamafirewall`, `fuzz-sentry`,
`test-sentry-kernel-5-4`, `test-sentry-kernel-5-15`, `test-eslogger-fields`,
`release-gate`.

Third-party and first-party actions:

| Action | Ref in use | Pin type | Party |
|--------|-----------|----------|-------|
| `actions/checkout` | `@v4` | tag | first-party |
| `actions/setup-go` | `@v5` | tag | first-party |
| `cilium/little-vm-helper` | `@v0.0.21` | tag | third-party |

`GITHUB_TOKEN` permissions: no `permissions` block at workflow or job level.
The workflow therefore runs at the repository default, which is currently
read-only. This is a defense-in-depth gap, not an active exposure today,
because the default backstops it. The directive nonetheless requires an
explicit top-level block so the workflow does not depend on an org or repo
setting that can change.

Untrusted input in `run:` blocks: none found. The only shell variables in
`run:` steps are internal (`$FIXTURE`, `$ESLPID`, `$i`, loop counters). No
`github.event.*`, `github.head_ref`, `github.ref_name`, `github.actor`, or
label or title field is interpolated into any script. No template-injection
vector present.

`pull_request_target`: not used. Fork CI runs under `pull_request`, which has
no secrets in scope. Correct.

Secrets: none referenced. No fork-exposure path.

OIDC: not applicable. CI authenticates to no cloud, registry, or signing
service.

Egress monitoring: none. No Harden-Runner or equivalent on any job.

Notable but non-blocking: the `test` job runs
`go install github.com/home-beekeeper/pollen/cmd/pollen@v0.2.0`. This installs
a binary from a version tag rather than a checksum-pinned reference. It is a
first-party module under the same org, and `@v0.2.0` is a semantic-version tag
backed by the Go checksum database, so the residual risk is low. Worth a note,
not a blocker.

### `.github/workflows/release.yml`

What it does: on a `v*` tag push, GoReleaser builds reproducible multi-platform
binaries, cosign v3 keyless-signs the checksums, syft emits CycloneDX SBOMs,
and the SLSA generator produces a Level 3 provenance attestation.

Triggers: `push` to tags matching `v*` only. No PR trigger, no
`pull_request_target`. Tag pushes cannot be performed by forks, so this
workflow is not reachable by untrusted contributors.

Jobs: `goreleaser`, `provenance`.

Actions:

| Action | Ref in use | Pin type | Party | Note |
|--------|-----------|----------|-------|------|
| `actions/checkout` | `@v4` | tag | first-party | |
| `actions/setup-go` | `@v5` | tag | first-party | |
| `sigstore/cosign-installer` | `@v3` | floating major tag | third-party | runs in the OIDC-privileged job |
| `anchore/sbom-action/download-syft` | `@v0` | floating major tag | third-party | runs in the OIDC-privileged job |
| `goreleaser/goreleaser-action` | `@v7` | tag | third-party | runs in the OIDC-privileged job |
| `slsa-framework/slsa-github-generator/.github/workflows/generator_generic_slsa3.yml` | `@v2.1.0` | full semver | third-party (reusable workflow) | correct as-is, see exception below |

`GITHUB_TOKEN` permissions: top-level `permissions: contents: read`. Per-job
elevation is correctly scoped:
- `goreleaser`: `contents: write` (upload artifacts) and `id-token: write`
  (cosign keyless OIDC).
- `provenance`: `actions: read`, `id-token: write`, `contents: write`.

This is the correct least-privilege pattern and satisfies control 2 for this
workflow. It will be left unchanged in Phase 2.

Untrusted input in `run:` blocks: none. The hash step reads GoReleaser output
through an `env:` variable (`ARTIFACTS`) rather than inline interpolation,
which is the recommended pattern. The tag name is not interpolated into any
script.

Secrets: only the built-in `secrets.GITHUB_TOKEN`, which is ephemeral and
request-scoped. No long-lived signing, cloud, or registry credential is used.
Signing is keyless via Sigstore OIDC. This satisfies control 5.

Egress monitoring: none. No Harden-Runner on either job. This is the
highest-value place to add egress monitoring, because the `goreleaser` job
holds `contents: write` and `id-token: write` at the same time.

SLSA pin exception (important for Phase 2): the SLSA reusable workflow is
pinned to the full semver tag `@v2.1.0`, not a commit SHA. This is correct and
must not be changed. The slsa-github-generator trusted-builder verification
model requires a semantic-version tag reference; pinning it to a commit SHA
breaks provenance verification. `CLAUDE.md` records the same constraint ("full
semver, NOT @v2"). Control 1 (SHA-pin third-party actions) explicitly excludes
this reference.

### `.github/renovate.json`

The repository uses Renovate for dependency automation. It watches the `gomod`
and `github-actions` ecosystems, runs on a weekly schedule, groups updates,
and sets `automerge: false` so every update requires human review. That review
posture is correct and should be kept.

Two gaps relative to the control set:

1. Digest pinning is not enabled. Renovate currently tracks tags. Once Phase 2
   pins actions to commit SHAs, Renovate will not keep those SHA pins fresh
   unless digest pinning is turned on (the `helpers:pinGitHubActionDigests`
   preset or `pinDigests: true`). This is the Renovate equivalent of the
   Dependabot `enable-beta-ecosystems: true` requirement in control 8. Without
   it, SHA-pinned actions silently stop receiving security updates, which is
   worse than not pinning.

2. No update cooldown. Renovate supports `minimumReleaseAge`, which maps
   directly to control 9's 7-day cooldown. It is not configured, so a freshly
   published (and possibly malicious) action or module version could be
   proposed immediately.

Tooling-choice note for the maintainer: control 8 names Dependabot, but this
repo already runs Renovate, which performs the same role and has native digest
pinning and cooldown support. Running both Dependabot and Renovate at once
produces duplicate, conflicting PRs. The recommendation is to keep Renovate and
configure it to satisfy the intent of controls 8 and 9, rather than add a
`dependabot.yml`. This is flagged as a decision for maintainer sign-off in
Phase 2, not assumed.

## Coverage matrix

| # | Control | State | Evidence |
|---|---------|-------|----------|
| 1 | SHA-pin every third-party action | ABSENT | All actions reference tags. Third-party `cilium/little-vm-helper@v0.0.21`, `sigstore/cosign-installer@v3`, `anchore/sbom-action/download-syft@v0`, `goreleaser/goreleaser-action@v7` are unpinned. The SLSA `@v2.1.0` semver reference is a correct, required exception, not a finding. |
| 2 | Default token read-only, elevate per job | PARTIAL | `release.yml` is correct (top-level `contents: read`, scoped per-job elevation). `ci.yml` has no `permissions` block; it relies on the repo default being read-only. |
| 3 | Never run fork PR code with secrets | PRESENT | CI uses `pull_request` with no secrets. No `pull_request_target` anywhere. Release is tag-triggered. Fork-PR approval policy is `all_external_contributors`. Leave alone. |
| 4 | Pass untrusted input through env vars | PRESENT | No untrusted `github.event.*` or ref field is interpolated into any `run:` block. The one passthrough (`ARTIFACTS`) already uses `env:`. No violation to fix. |
| 5 | OIDC for all credentialed steps | PRESENT | cosign keyless signing and SLSA provenance both use `id-token: write`. Only secret in use is the ephemeral `GITHUB_TOKEN`. No long-lived credential exists. Leave alone. |
| 6 | Harden-Runner on every job | ABSENT | No egress monitoring on any job in either workflow. |
| 7 | Workflow linter (zizmor) | ABSENT | No linter present. Hardening is not self-enforcing; a future unpinned action would not fail CI. |
| 8 | Dependency update tool watches actions and gomod | PARTIAL | Renovate watches both ecosystems with `automerge: false` (correct), but digest pinning is off, so SHA pins added in Phase 2 will not receive updates. |
| 9 | Action update cooldown (7 days) | ABSENT | No `minimumReleaseAge` in `renovate.json`. |
| 10 | CODEOWNERS | ABSENT | No `.github/CODEOWNERS`. Security-critical paths do not force maintainer review. |
| 11 | Hardened Claude PR reviewer | ABSENT (not present) | No Claude review workflow exists. Controls apply only if one is added. |
| 12 | Ruleset required status checks | ABSENT | No repo ruleset and no classic branch protection on `main`. PRs are not required and no status check gates merges. Worse than "deferred": `main` is currently unprotected at the repo level. |

## Findings, highest risk first

### High

1. Unpinned third-party actions in the OIDC-privileged release job (control 1).
   `sigstore/cosign-installer@v3`, `anchore/sbom-action/download-syft@v0`, and
   `goreleaser/goreleaser-action@v7` run inside the `goreleaser` job, which
   holds `contents: write` and `id-token: write` at the same time. A compromised
   tag on any of these (especially the floating `@v0` and `@v3` major tags)
   would execute attacker code with the ability to upload release artifacts and
   to request a Sigstore OIDC token. This is the sharpest supply-chain edge in
   the repository. The unpinned `cilium/little-vm-helper@v0.0.21` in CI is the
   same class of risk at lower privilege (no secrets, read-only context).

2. `main` is unprotected and gates nothing (control 12). There is no repository
   ruleset and no classic branch protection. Direct pushes to `main` are
   possible and no CI status check is required before merge. This both
   contradicts the stated premise and removes the safety net that the rest of
   this hardening assumes.

### Medium

3. `ci.yml` has no explicit `permissions` block (control 2). Real exposure today
   is low because the repo default is read-only, but the workflow is one
   org-or-repo-setting change away from running with a broad token. Defense in
   depth requires the block in the workflow itself.

4. No egress monitoring on any job (control 6), most importantly on the
   release `goreleaser` job. There is no recorded baseline of where CI runners
   talk to, so exfiltration during a build would be invisible.

5. Hardening is not self-enforcing (control 7). With no workflow linter, a
   future PR that reintroduces an unpinned action, a dangerous trigger, or an
   over-broad token passes CI. The pins added in Phase 2 would then erode over
   time.

6. SHA pins will not receive updates (controls 8 and 9). After Phase 2 pins
   actions to SHAs, Renovate without digest pinning will leave those pins frozen
   and unpatched, and without a cooldown it could also propose brand-new
   versions the moment they publish.

### Low

7. No CODEOWNERS (control 10). Changes to the policy engine, Sentry rules,
   catalog ingestion, audit log, release pipeline, and `.github/` itself do not
   automatically require maintainer review. Impact is limited while the project
   is effectively single-maintainer, but it grows as a public repo attracts
   contributors.

8. `go install ...pollen@v0.2.0` uses a version tag rather than a pinned,
   checksum-verified reference. Low risk (first-party module, semver tag backed
   by the Go checksum database). Noted for completeness.

### Already correct, leave unchanged

- Fork CI runs under `pull_request` with no secrets, and no `pull_request_target`
  exists (control 3).
- No untrusted input reaches any `run:` block (control 4).
- All credentialed steps use OIDC; no long-lived credential exists (control 5).
- `release.yml` permissions are correctly least-privilege per job (control 2 for
  that workflow).
- The SLSA generator is correctly pinned to the `@v2.1.0` semver tag and must
  stay that way.

## Proposed remediation order (Phase 2, each a separate PR)

The order front-loads the highest-value, lowest-risk-of-breakage changes and
sequences the self-enforcement (zizmor) after the pins it will check exist.

1. PR: SHA-pin all third-party actions (control 1). Pin `ci.yml` and
   `release.yml` actions to the commit SHA of the version currently in use,
   with a trailing `# vX.Y.Z` comment. Preserve the SLSA `@v2.1.0` semver
   reference unchanged. No major-version upgrades. Closes finding 1 and 8-class
   risk for actions.
2. PR: Add explicit `permissions: contents: read` at the top of `ci.yml`
   (control 2). Do not touch `release.yml`, which is already correct. Closes
   finding 3.
3. PR: Add Harden-Runner as the first step of every job in both workflows in
   `egress-policy: audit` mode (control 6). PR note will ask the maintainer to
   review the learned baseline and move to `block` with an allowlist later.
   Closes finding 4 (audit stage).
4. PR: Add zizmor as a CI job with a chosen failing severity, plus its config
   (control 7). Sequenced after pinning so it passes on the hardened state and
   then enforces it. Closes finding 5.
5. PR: Configure Renovate for digest pinning and a 7-day `minimumReleaseAge`
   (controls 8 and 9), after maintainer confirms keep-Renovate over
   add-Dependabot. Closes finding 6.
6. PR: Add `.github/CODEOWNERS` routing all paths and the security-critical
   paths explicitly to the maintainer (control 10). Closes finding 7.
7. PR (optional, maintainer decision): Add a hardened Claude PR-review workflow
   (control 11) on `pull_request`, `pull-requests: write` plus
   `contents: read`, restricted `--allowedTools`, never auto-merge.
8. Deliverable, not a workflow PR: provide updated branch ruleset JSON with
   `required_status_checks` populated once the `zizmor` job name is stable
   (control 12), for the maintainer to import. This is also where the
   unprotected-`main` finding (finding 2) is closed, by the maintainer enabling
   the ruleset.

Candidate required status check names for control 12 (current CI job names that
run on `pull_request`): `test (ubuntu-latest)`, `test (macos-latest)`,
`test (windows-latest)`, `fuzz`, `fuzz-ipc`, `fuzz-llamafirewall`,
`fuzz-sentry`, `test-sentry-kernel-5-4`, `test-sentry-kernel-5-15`,
`test-eslogger-fields`, `release-gate`, plus `zizmor` once PR 4 lands. The exact
required subset is a maintainer choice and will be proposed with the ruleset
JSON in Phase 2.

## Phase 1 stop

This completes the read-only audit. No file under `.github/` was modified.
Awaiting maintainer review of this report before beginning Phase 2. On approval,
Phase 2 proceeds as the sequence of small, separately reviewable pull requests
above, and any control that cannot be applied will be reported with the reason.
