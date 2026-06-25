# Release documentation checklist

A pushed tag is not a finished release. A release is done when every place that
shows a version, every install command, the changelog, and the feature docs all
match it. Skipping this is how the live site sat on a stale `v1.1.1` badge across
two releases. **Run this after every release (patch, minor, or major).**

The `/release-docs-sync` skill drives this checklist. CLAUDE.md makes it mandatory.

## 0. Inputs

- `NEW` = the version just cut, for example `v1.2.0`.
- Two working copies side by side: this repo, and beekeeper-web (remote
  `Bantuson/beekeeper-web`). Both have protected `main`: branch, PR, CI, merge.

## 1. Bump every displayed version

The web badge has a single source of truth: `web/lib/site-version.ts`
(`SITE_VERSION`). Everything else is checked against it by the hard gate in step 4.

| Surface | File(s) | Change |
|---|---|---|
| Web header + footer badge | `web/lib/site-version.ts` | `SITE_VERSION = "NEW"` (header and footer import it) |
| Web install pins | `web/content/docs/{getting-started,installation,troubleshooting}.mdx`, `web/content/blog/*.mdx`, `web/components/home/{install-chip,quickstart-card,how-it-works}.tsx` | `cmd/beekeeper@v...` to `cmd/beekeeper@NEW` (find: `grep -rln "cmd/beekeeper@v" web/content web/components`) |
| Web verify examples | `web/content/docs/installation.mdx` | archive name, `gh release download`, cosign `@refs/tags/`, slsa source-tag, `verify-release VERSION=` |
| Repo README pin | `README.md` | `cmd/beekeeper@v...` to `cmd/beekeeper@NEW` |
| Repo docs | `docs/*.md` | only CONCRETE pins, never the `vX.Y.Z` placeholders in `release-runbook.md` / `THREAT-MODEL.md` / `SECURITY.md` |

Never touch historical references (an old changelog, "removed in v1.1.0" prose).

## 2. Write the changelog entry (beekeeper-web)

1. Create `web/content/changelog/NEW/index.mdx` modeled on the previous entry:
   frontmatter `title` + `description`, `## Overview`, `## Highlights` (numbered
   subsections), `## Download and verify` with `<ReleaseLinks version="NEW" />` and
   `<VerifyCommands version="NEW" />`, and `## Notes`.
2. Add `"NEW"` as the FIRST element of `web/content/changelog/meta.json` `pages`.
3. Add a bullet at the top of the list in `web/content/changelog/index.mdx`.

## 3. Copy standard (tech-marketing guidelines)

Read and apply before writing prose:
`../beekeeper-product-launch/tech-marketing-framework/.claude/rules/content-guidelines.md`
(voice: `.../docs/inputs/brand_guidelines.md`; positioning: `.../messaging_positioning.md`).

Non-negotiables, plus the stricter web rules:

- Bold, confident, specific. No hedging. State facts.
- Show, do not tell: every claim gets a command, default, threshold, or example.
- Answer first, then elaborate. Headers name the subject.
- Honest about the enforcement boundary. No overclaiming, no fear-mongering.
- Banned words (delve, robust, seamless, leverage as a verb, transformative, ...)
  and banned structures ("this isn't just X, it's Y", rhetorical-question endings).
- Oxford comma, 5th-grade reading level, do not capitalize feature names.
- **Web copy: ZERO em-dashes** (stricter than the framework's max of 3). No colored
  left-edge accent stripes. The accuracy gate must stay green.

## 4. Make the hard gate pass (beekeeper-web)

`web/tests/unit/version-consistency.test.ts` fails the build if the newest
changelog page, a changelog entry for `SITE_VERSION`, or any `go install` pin
drifts from `SITE_VERSION`.

```sh
cd web && pnpm test    # unit + accuracy + version-consistency
cd web && pnpm build   # validates all MDX/TSX, or rely on PR CI
```

Red gate means a missed pin or changelog. Fix the docs; never weaken the test.

## 5. Repo-side feature docs

Confirm the shipped features are documented in this repo's `docs/`. For catalog-sync
visibility that is `docs/catalog-sync.md`. Add or update as needed.

## 6. Ship

One PR per repo, CI green, squash-merge, sync local main. Then spot-check the live
site: header badge shows `NEW`, changelog lists it.

## Surfaces that get missed

- The FOOTER badge, not just the header.
- Blog-post install pins (the gate catches these).
- `installation.mdx` verify examples (cosign identity tag, archive name).
- The `meta.json` `pages` array (controls the changelog "latest").
