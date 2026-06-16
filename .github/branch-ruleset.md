# Branch ruleset for `main` (control 12)

`branch-ruleset.json` in this directory is a GitHub repository ruleset, ready to
import. It closes two audit findings:

- Finding 2: `main` currently has no ruleset and no branch protection, so direct
  pushes are possible and no status check gates a merge.
- Control 12: the required status checks rule was deferred when the repo had no
  checks to require. The CI jobs now exist, so it can be populated.

## What it enforces

- Pull requests are required to change `main` (no direct pushes).
- The branch cannot be deleted or force-pushed (`deletion`, `non_fast_forward`).
- A merge is blocked unless these status checks pass (`strict` means the branch
  must also be up to date with `main` first):
  - `test (ubuntu-latest)`
  - `test (macos-latest)`
  - `test (windows-latest)`
  - `zizmor`

These four are the core build, cross-platform test, and workflow-lint gates.
`build` and `vet` are steps inside the `test` job, so the three `test (os)`
checks already cover them.

## How to import

```sh
gh api --method POST \
  -H "Accept: application/vnd.github+json" \
  repos/home-beekeeper/beekeeper/rulesets \
  --input .github/branch-ruleset.json
```

Then confirm:

```sh
gh api repos/home-beekeeper/beekeeper/rulesets
```

## Solo-maintainer choices (and how to tighten later)

- `required_approving_review_count` is `0` so the maintainer can self-merge once
  checks pass. A sole owner cannot approve their own PR, so requiring an approval
  would deadlock solo work. Raise it to `1` once there is a second maintainer.
- `require_code_owner_review` is `false` for the same reason. Set it to `true`
  (with `CODEOWNERS` already in place) once there is more than one owner.
- `bypass_actors` is empty, so the rules apply to everyone, including admins. The
  maintainer is not locked out: as an admin they can still self-merge a passing
  PR, and can edit or pause the ruleset in an emergency. To allow emergency
  direct pushes, add the Repository admin role as a bypass actor in the ruleset
  UI.

## Optional additional required checks

The following CI jobs also run on every PR and can be added to
`required_status_checks` if you want them blocking. They are left out of the
core set because some are slower or environment-sensitive (nested-VM kernel
tests, macOS `eslogger` with sudo):

`fuzz`, `fuzz-ipc`, `fuzz-llamafirewall`, `fuzz-sentry`,
`test-sentry-kernel-5-4`, `test-sentry-kernel-5-15`, `test-eslogger-fields`,
`release-gate`.

## Note

This file is delivered for import; it is not consumed by CI. Importing it is a
repository-settings change, so it is left for the maintainer to apply rather
than done automatically.
