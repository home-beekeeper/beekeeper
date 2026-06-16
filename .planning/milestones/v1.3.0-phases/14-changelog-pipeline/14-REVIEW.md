---
phase: 14-changelog-pipeline
reviewed: 2026-06-08T00:00:00Z
depth: standard
files_reviewed: 21
files_reviewed_list:
  - web/source.config.ts
  - web/lib/changelog-source.ts
  - web/app/changelog/layout.tsx
  - web/app/changelog/[[...slug]]/page.tsx
  - web/components/changelog/breaking-change-callout.tsx
  - web/components/changelog/verify-commands.tsx
  - web/components/changelog/release-links.tsx
  - web/mdx-components.tsx
  - web/app/docs/layout.tsx
  - web/content/changelog/v1.0.0/index.mdx
  - web/content/changelog/v1.2.0/index.mdx
  - web/content/changelog/v1.3.0/index.mdx
  - web/content/changelog/meta.json
  - web/content/changelog/v1.0.0/meta.json
  - web/content/changelog/v1.2.0/meta.json
  - web/content/changelog/v1.3.0/meta.json
  - web/biome.json
  - web/app/api/search/route.ts
  - web/app/docs/[[...slug]]/page.tsx
  - web/app/providers.tsx
  - web/next.config.mjs
findings:
  critical: 2
  warning: 3
  info: 2
  total: 7
status: issues_found
---

# Phase 14: Code Review Report

**Reviewed:** 2026-06-08T00:00:00Z
**Depth:** standard
**Files Reviewed:** 21
**Status:** issues_found

## Summary

Phase 14 wires a second Fumadocs collection for the changelog route, ships three MDX
components (VerifyCommands, ReleaseLinks, BreakingChangeCallout), and registers them in
the global MDX map. The structural pipeline (source.config.ts, changelog-source.ts,
layout, catch-all page, meta.json files) is clean and mirrors the proven docs pipeline
correctly.

Two critical security-integrity defects are present in `verify-commands.tsx`: the SLSA
`--source-uri` uses the wrong casing for `beekeeper` (capital-B `Bantuson` vs. the
required lowercase `bantuson` that the SLSA provenance attests), and the SLSA
`--provenance-path` argument uses `${version}.intoto.jsonl` — a form found nowhere in
the canonical project documentation, which consistently uses the artifact-named form
(`beekeeper.intoto.jsonl` / `beekeeper-<target>.intoto.jsonl`). A user who copies the
Step 3 command verbatim will get a file-not-found or provenance-mismatch failure.

Three warnings round out the review: the `CopyButton` clipboard promise has no `.catch`
handler (silent failure on non-HTTPS / denied permission contexts), the docs catch-all
page renders `<MDX />` without passing the custom components map (breaking
`VerifyCommands`/`ReleaseLinks`/`BreakingChangeCallout` if they ever appear in docs
MDX), and the changelog MDX route renders a `DocsLayout` sidebar + nav that has no
index/landing page for `/changelog` itself, which causes a fumadocs 404 on the bare
route under static export.

---

## Critical Issues

### CR-01: SLSA `--source-uri` uses wrong casing — will break SLSA verification

**File:** `web/components/changelog/verify-commands.tsx:95`
**Issue:** The SLSA verify command uses `--source-uri github.com/home-beekeeper/beekeeper`
(capital-B `Bantuson`). The canonical form in `docs/THREAT-MODEL.md` (lines 156, 465),
`cmd/beekeeper/selfquarantine.go` (line 68), and the Go module path in `go.mod` is
`github.com/home-beekeeper/beekeeper` (lowercase). The SLSA provenance is produced by
`slsa-github-generator` and the source-uri it attests is derived from the repository's
Go module path — always lowercase. Passing `Bantuson` (capital B) to `--source-uri` will
cause `slsa-verifier` to reject the attestation with a source-mismatch error. A user
copying this command from the security product's own changelog will be unable to verify
the binary they just downloaded.

The capital-B `Bantuson` constraint applies only to cosign's `--certificate-identity-regexp`
(the GitHub OIDC certificate identity, which GitHub binds to the canonical account casing).
It does NOT apply to `slsa-verifier --source-uri`, which must match the Go module source
URI.

**Fix:**
```tsx
// Line 95 — change Bantuson → bantuson for the slsa-verifier source-uri
const slsaCmd = `slsa-verifier verify-artifact \\
  --provenance-path <artifact>.intoto.jsonl \\
  --source-uri github.com/home-beekeeper/beekeeper \\
  <artifact>`;
```

---

### CR-02: SLSA `--provenance-path` uses `${version}.intoto.jsonl` — wrong filename convention; file will not exist

**File:** `web/components/changelog/verify-commands.tsx:94`
**Issue:** The rendered command is:
```
slsa-verifier verify-artifact \
  --provenance-path v1.0.0.intoto.jsonl \
  --source-uri ...
  <artifact>
```
No project documentation uses a version-named provenance file. The canonical form in
`docs/THREAT-MODEL.md` (lines 155, 464) and `cmd/beekeeper/selfquarantine.go` (line 67)
is `beekeeper.intoto.jsonl`. The research stack (`STACK.md` line 698) uses the
per-artifact form `beekeeper-linux-amd64.intoto.jsonl`. The GoReleaser SLSA generator
names the provenance file after the artifact, not the tag. A user following this command
will run `slsa-verifier` against a file that does not exist in the downloaded release
assets, getting a `no such file or directory` error.

The `<artifact>` placeholder at the end of the command is also unpopulated and is a
separate usability concern, but the provenance path is the operationally wrong element.

**Fix:**
```tsx
// Replace the version-named pattern with the artifact-named placeholder form
// that matches docs/THREAT-MODEL.md and selfquarantine.go
const slsaCmd = `slsa-verifier verify-artifact \\
  --provenance-path beekeeper.intoto.jsonl \\
  --source-uri github.com/home-beekeeper/beekeeper \\
  <artifact>`;
```
If per-platform filenames are preferred, use `beekeeper-<os>-<arch>.intoto.jsonl` and
add a note that the user must substitute the artifact name matching their platform.

---

## Warnings

### WR-01: `CopyButton` clipboard promise has no `.catch` — silent failure on permission denial or non-HTTPS

**File:** `web/components/changelog/verify-commands.tsx:14`
**Issue:** `navigator.clipboard.writeText(text).then(...)` has no `.catch` handler.
The Clipboard API throws `NotAllowedError` when the document is not focused or clipboard
permission is denied (common in iframes, some mobile browsers, and when the page is loaded
over HTTP). On failure the button state never updates to "Copied" and no error is reported,
but — more importantly — the button becomes permanently non-functional with no user
feedback. For a security product where users are expected to copy verification commands
exactly, silent copy failure is a usability and integrity risk.

**Fix:**
```tsx
function handleCopy() {
  navigator.clipboard.writeText(text).then(() => {
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }).catch(() => {
    // Fallback: select the pre element text so the user can Ctrl+C manually
    // Or at minimum surface a visual indicator that copy failed
    setCopied(false);
  });
}
```

---

### WR-02: Docs catch-all page renders `<MDX />` without the custom components map — changelog components would silently disappear if used in docs MDX

**File:** `web/app/docs/[[...slug]]/page.tsx:41`
**Issue:** The docs page renders `<MDX />` with no `components` prop:
```tsx
<MDX />
```
The changelog page correctly passes `useMDXComponents({})`:
```tsx
const components = useMDXComponents({});
<MDX components={components} />
```
This inconsistency means that if any docs MDX file ever uses `<VerifyCommands>`,
`<ReleaseLinks>`, or `<BreakingChangeCallout>` (all of which are now in the global
`mdx-components.tsx` map), those components will not be injected into the render and
will silently produce empty/broken output. The docs pipeline relies on fumadocs-ui's
`defaultMdxComponents` being passed through, so the omission of the components prop
also means any fumadocs default MDX components that require explicit injection may
also be absent in the docs route.

**Fix:**
```tsx
// web/app/docs/[[...slug]]/page.tsx
import { useMDXComponents } from "@/mdx-components";

// inside Page():
const MDX = page.data.body;
const components = useMDXComponents({});

return (
  // ...
  <MDX components={components} />
);
```

---

### WR-03: No index page for `/changelog` — bare route will 404 under static export

**File:** `web/app/changelog/layout.tsx` (layout), `web/content/changelog/meta.json:3`
**Issue:** The `meta.json` orders pages as `["v1.3.0", "v1.2.0", "v1.0.0"]`. None of
these resolve to `/changelog` (the bare route). Under static export with `trailingSlash:
true`, a request to `/changelog/` maps to `out/changelog/index.html`. The fumadocs
`DocsLayout` sidebar will render a tree whose root is `/changelog/`, but
`generateStaticParams()` only yields slugs for the three version pages. The bare
`/changelog/` route has no `page.tsx` handler outside of the `[[...slug]]` catch-all,
and `source.getPage([])` or `source.getPage(undefined)` will return `undefined` →
`notFound()`. This means clicking "Changelog" in the docs nav (`/docs/layout.tsx` links
to `/changelog`) lands on a 404.

Fumadocs' standard fix is to add an `index.mdx` at `content/changelog/` (the root
index) or redirect `/changelog` → `/changelog/v1.3.0` (the latest).

**Fix — add a root index page:**
```
content/changelog/index.mdx
---
title: "Changelog"
description: "Release notes for every Beekeeper version."
---
See the version-specific entries in the sidebar, or jump to:
- [v1.3.0](/changelog/v1.3.0) — latest
- [v1.2.0](/changelog/v1.2.0)
- [v1.0.0](/changelog/v1.0.0)
```
And add `"index"` as the first entry in `content/changelog/meta.json`:
```json
{ "title": "Changelog", "pages": ["index", "v1.3.0", "v1.2.0", "v1.0.0"] }
```

---

## Info

### IN-01: `"use client"` boundary prevents `VerifyCommands` from being server-rendered — expected and correct, but clipboard commands could be static

**File:** `web/components/changelog/verify-commands.tsx:1`
**Issue:** The entire `VerifyCommands` component is a client component because of the
`CopyButton` useState. The three verification commands are purely static strings derived
from the version prop — they do not require client-side state. This means the verification
commands are not statically rendered in the HTML, which affects SEO and initial-paint
correctness for a security-critical page element. This is not a bug in the current
implementation (fumadocs handles the hydration), but it is a structural note: if the
clipboard interactivity were split into a small `CopyButton` client subcomponent with the
static `CommandBlock` shell server-rendered, the command text would be in the static
export HTML and thus verifiable by tooling.

**Fix (optional, not urgent):** Keep `CopyButton` as `"use client"` only; make
`CommandBlock` and `VerifyCommands` server components. This is a refactor, not a bug.

---

### IN-02: `api/search/route.ts` indexes only the `docs` source — changelog content not searchable

**File:** `web/app/api/search/route.ts:3`
**Issue:** The search route is built from `@/lib/source` (docs only). The changelog
content is in `@/lib/changelog-source` and is not included in the Orama index. Users
searching for "v1.3.0", "corroboration", or "hook exit code 2" will get no results
even though those terms appear in the changelog MDX. The plan notes "changelog reuses no
Orama index in this phase" (14-01-PLAN.md:147), so this is a known, intentional deferral
— not a defect introduced by this phase. Flagged here for tracking.

**Fix (future phase):** Pass both sources to `createFromSource`:
```ts
export const { staticGET: GET } = createFromSource(source, changelogSource);
```
or merge the two page trees before indexing.

---

_Reviewed: 2026-06-08T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
