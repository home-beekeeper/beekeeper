# Phase 18: Full Content Authoring — Pattern Map

**Mapped:** 2026-06-09
**Files analyzed:** 11 (8 MODIFY + 2 CREATE + 1 MODIFY-registration)
**Analogs found:** 11 / 11

---

## File Classification

| New/Modified File | Action | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|---|
| `web/content/docs/getting-started/index.mdx` | MODIFY/expand | docs-content | transform | `web/content/changelog/v1.3.0/index.mdx` | role-match |
| `web/content/docs/installation/index.mdx` | MODIFY/expand | docs-content | transform | `web/content/changelog/v1.3.0/index.mdx` | role-match |
| `web/content/docs/configuration/index.mdx` | MODIFY/expand | docs-content | transform | `web/content/changelog/v1.3.0/index.mdx` | role-match |
| `web/content/docs/security/index.mdx` | MODIFY/expand | docs-content | transform | `web/content/changelog/v1.3.0/index.mdx` | role-match |
| `web/content/docs/integration/index.mdx` | MODIFY/expand | docs-content | transform | `web/content/changelog/v1.3.0/index.mdx` | role-match |
| `web/content/docs/cli-reference/index.mdx` | MODIFY/expand | docs-content | transform | `web/content/changelog/v1.3.0/index.mdx` | role-match |
| `web/content/docs/troubleshooting/index.mdx` | MODIFY/expand | docs-content | transform | `web/content/changelog/v1.3.0/index.mdx` | role-match |
| `web/content/docs/audit-log/index.mdx` | MODIFY/expand | docs-content | transform | `web/content/changelog/v1.3.0/index.mdx` | role-match |
| `web/components/docs/unenforced-callout.tsx` | CREATE | component | request-response | `web/components/changelog/breaking-change-callout.tsx` | exact |
| `web/mdx-components.tsx` | MODIFY (registration) | config/glue | request-response | self (lines 1–21 — already read) | exact |
| `web/tests/accuracy_spec.py` | CREATE | test | batch | `web/tests/seo_spec.py` | exact |

---

## Pattern Assignments

### 8 Docs Content Files (MODIFY/expand) — Shared Analog

**Analog:** `web/content/changelog/v1.3.0/index.mdx`

All eight docs pages share the same MDX authoring pattern. The changelog v1.3.0 page is the
closest polished, shipped example of: (a) correct Fumadocs frontmatter shape, (b) MDX component
usage (`<BreakingChangeCallout>`), (c) the honest/caveat voice already approved by the
maintainer, and (d) fenced code blocks for copyable commands. The docs stubs currently exist
with 2–4 field frontmatter only — Phase 18 expands the body while preserving the frontmatter
keys already present, then adds `source_doc:`.

---

#### Frontmatter pattern — what already exists in every stub

Analog: each existing docs stub, e.g. `web/content/docs/getting-started/index.mdx` lines 1–4:

```mdx
---
title: Getting Started
description: Set up Beekeeper in minutes and protect your AI coding agent.
---
```

**Phase 18 expands to** (add `source_doc:` field; title + description stay):

```mdx
---
title: Getting Started
description: Zero to a working beekeeper check in under five minutes.
source_doc: README.md
---
```

For sections deriving from multiple Go-side docs:

```mdx
---
title: Security
description: Beekeeper's threat model, corroboration engine, and known gaps — co-located.
source_doc: "docs/THREAT-MODEL.md, docs/harness-support-matrix.md"
---
```

Rules:
- `source_doc:` value is a repo-root-relative path (or comma-separated list).
- Every section whose content derives from a Go-side doc MUST have `source_doc:`. This is
  the machine-checkable DOCS-09 gate (`accuracy_spec.py` AC-1).
- Do NOT remove or rename `title:` or `description:` — Fumadocs sidebar + SEO depend on them.

---

#### MDX component usage pattern

Analog: `web/content/changelog/v1.3.0/index.mdx` lines 16–59 (BreakingChangeCallout usage):

```mdx
<BreakingChangeCallout title="Breaking change: hook exit code 1 → 2">

**All users must upgrade and re-register their hooks to get actual enforcement.**

### Before (pre-v1.3.0)
...

</BreakingChangeCallout>
```

Phase 18 equivalent — `UnenforcedCallout` usage in MDX (once the component is created and
registered; see component section below):

```mdx
<UnenforcedCallout feature="release_age / minimumReleaseAge">
  Declaring `release_age` rules in policy files is **not enforced in v1.3.0**.
  The overlay evaluates only data in the tool call itself; release age requires
  a catalog/registry API lookup not available at check time. These rules are
  informational and for `policy test` dry-runs only.
</UnenforcedCallout>
```

Rules:
- Components are used directly by name (no import needed in MDX — they are globally registered
  via `web/mdx-components.tsx`).
- Blank line after opening tag, blank line before closing tag — matches v1.3.0 changelog style.
- `###` headings inside a callout are fine (Fumadocs renders MDX children freely).

---

#### Fenced code block + copyable command pattern

Analog: `web/content/changelog/v1.3.0/index.mdx` lines 20–23, 45–50:

````mdx
```bash
beekeeper hooks install --target claude-code
```
````

````mdx
```bash
cosign verify \
  --certificate-identity=https://github.com/home-beekeeper/beekeeper/.github/workflows/release.yml@refs/tags/v<version> \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com \
  beekeeper
```
````

Rules:
- Always use `bash` language tag for shell commands.
- Multi-line commands use `\` continuation exactly as they appear in `docs/THREAT-MODEL.md`.
- JSON examples use `json` language tag; config file examples use `json`.

---

#### Heading hierarchy + section structure pattern

Analog: `web/content/changelog/v1.3.0/index.mdx` overall structure:

- `## Overview` — H2 for top-level sections
- `### Before / ### After / ### Migration steps` — H3 for sub-sections within a section
- No H1 in the body (Fumadocs renders `title:` frontmatter as the page H1)
- For the CLI reference: H2 per top-level command (`## beekeeper check`), H3 per
  subcommand (`### beekeeper catalogs sync`)

---

### `web/content/docs/meta.json` (MODIFY — only if sub-pages are added)

**Current file** (`web/content/docs/meta.json` lines 1–13):

```json
{
  "title": "Beekeeper",
  "pages": [
    "getting-started",
    "installation",
    "configuration",
    "integration",
    "security",
    "cli-reference",
    "audit-log",
    "troubleshooting"
  ]
}
```

**Pattern rule:** If a section grows sub-pages (e.g. `integration/claude-code/index.mdx`), a
`meta.json` must be added inside that section's directory. Example for a hypothetical
`integration/` sub-page split:

```json
{
  "title": "Integration",
  "pages": [
    "index",
    "claude-code",
    "mcp-gateway"
  ]
}
```

**Phase 18 recommendation:** Do NOT split any section into sub-pages (RESEARCH.md §5.3). The
top-level `meta.json` stays unchanged. Only touch it if the planner explicitly decides to split.

---

### `web/components/docs/unenforced-callout.tsx` (CREATE)

**Analog:** `web/components/changelog/breaking-change-callout.tsx` (lines 1–43 — full file)

**Why this analog:** Exact same role (reusable MDX callout component), exact same data flow
(props in → JSX out), same project, authored in Phase 14. The only differences are color
(amber/warning instead of red/danger), icon (e.g. `Info` or `AlertCircle` instead of
`TriangleAlert`), and props (`feature: string` instead of `title: string`).

**Full analog to copy shape from** (`web/components/changelog/breaking-change-callout.tsx`):

```typescript
import { TriangleAlert } from "lucide-react";
import type { ReactNode } from "react";

interface BreakingChangeCalloutProps {
  title: string;
  children: ReactNode;
}

/**
 * A prominently styled red callout for breaking changes.
 *
 * Uses raw theme tokens var(--red) so it renders correctly in BOTH light and
 * dark themes. Do NOT use --color-bk-red which is dark-only and would freeze
 * to dark colors in light mode (12-03-SUMMARY Deviation 3/4).
 */
export function BreakingChangeCallout({
  title,
  children,
}: BreakingChangeCalloutProps) {
  return (
    <div
      className="my-6 rounded-lg border-l-4 p-4"
      style={{
        borderLeftColor: "var(--red)",
        background: "color-mix(in srgb, var(--red) 10%, transparent)",
        borderTopColor: "color-mix(in srgb, var(--red) 30%, transparent)",
        borderRightColor: "color-mix(in srgb, var(--red) 30%, transparent)",
        borderBottomColor: "color-mix(in srgb, var(--red) 30%, transparent)",
      }}
    >
      <div
        className="mb-2 flex items-center gap-2 font-bold"
        style={{ color: "var(--red)" }}
      >
        <TriangleAlert size={18} aria-hidden="true" />
        <span>{title}</span>
      </div>
      <div className="text-sm leading-relaxed" style={{ color: "var(--fg)" }}>
        {children}
      </div>
    </div>
  );
}
```

**Adaptation rules for `UnenforcedCallout`:**

1. Change color token `var(--red)` → `var(--amber)` throughout (3 occurrences in style props).
   Same `color-mix(in srgb, var(--amber) 10%, transparent)` pattern for background.
2. Change icon: `TriangleAlert` → `Info` (also from lucide-react; import name changes).
3. Change interface name: `BreakingChangeCalloutProps` → `UnenforcedCalloutProps`.
4. Change prop: `title: string` → `feature: string` (displayed as the header label).
5. Change export name: `BreakingChangeCallout` → `UnenforcedCallout`.
6. Update the JSDoc comment to describe the amber/warning purpose and cite the same dual-theme
   raw-token rule (do NOT use `--color-bk-*`).
7. In the header `<span>`: render the `feature` prop with a prefix label, e.g.:
   `Not enforced in v1.3.0 — {feature}`.
8. The `var(--fg)` token for body text is UNCHANGED — same as the analog.

**CRITICAL styling rule** (from Phase 12/14 memory): All inline style tokens MUST use raw
theme-switched tokens (`var(--amber)`, `var(--fg)`, `var(--red)`, etc.), NEVER
`var(--color-bk-*)` dark-only tokens. The `--color-bk-*` family is dark-mode-only and
silently no-ops in light theme.

**File to create:** `web/components/docs/unenforced-callout.tsx`
(Create the `web/components/docs/` directory if it doesn't exist — there is no existing file
there; `web/components/changelog/` is the existing parallel.)

---

### `web/mdx-components.tsx` (MODIFY — add UnenforcedCallout registration)

**Analog:** `web/mdx-components.tsx` lines 1–21 (full file, already read above):

```typescript
import defaultMdxComponents from "fumadocs-ui/mdx";
import type { MDXComponents } from "mdx/types";
import { BreakingChangeCallout } from "@/components/changelog/breaking-change-callout";
import { ReleaseLinks } from "@/components/changelog/release-links";
import { VerifyCommands } from "@/components/changelog/verify-commands";

export function useMDXComponents(components: MDXComponents): MDXComponents {
  // Cast required since Phase 16: ...
  return {
    ...defaultMdxComponents,
    VerifyCommands,
    ReleaseLinks,
    BreakingChangeCallout,
    ...components,
  } as MDXComponents;
}
```

**Pattern:** Add one import line and one entry in the return object. The planner should add:

```typescript
// New import (line 6, after existing imports):
import { UnenforcedCallout } from "@/components/docs/unenforced-callout";

// New entry in return object (after BreakingChangeCallout):
UnenforcedCallout,
```

Do NOT disturb the existing cast comment or the `as MDXComponents` cast — it is load-bearing
(Phase 16 R3F type pollution fix).

---

### `web/tests/accuracy_spec.py` (CREATE)

**Analog:** `web/tests/seo_spec.py` (lines 1–164 — full file, already read above)

**Why this analog:** Exact same role (pure-Python stdlib file-walk test harness), exact same
data flow (batch assertion), same project, authored in Phase 17. The `accuracy_spec.py` is
structurally identical but walks `web/content/docs/**/*.mdx` (source files) rather than
`web/out/**/*.html` (built output).

**Full pattern to copy from `seo_spec.py`:**

Structure (copy exactly):
1. Module docstring — describes the phase, requirements covered, invocation, prerequisites.
2. `import os, pathlib, re, sys` — stdlib only, no pip.
3. `SCRIPT_DIR`, `WEB_DIR`, `CONTENT_DIR` path setup using `os.path.abspath(__file__)`.
4. Guard: check that `CONTENT_DIR` exists (if not, `print + sys.exit(1)`).
5. `failures = []` list.
6. `def fail(msg: str) -> None` — appends to failures, prints `"  FAIL: {msg}"`.
7. `def ok(msg: str) -> None` — prints `"  PASS: {msg}"`.
8. One function per assertion group (`ac1_source_doc_frontmatter()`, `ac2_unenforced_labels()`,
   `ac3_no_phantom_commands()`), each printing `"\n[AC-N] ..."` as a section header.
9. `def run_tests()` — calls each AC function in order.
10. `if __name__ == "__main__":` block — prints banner, calls `run_tests()`, prints final
    RESULT line, calls `sys.exit(1 if failures else 0)`.

**Key adaptation differences from seo_spec.py:**

- `CONTENT_DIR = pathlib.Path(WEB_DIR) / "content" / "docs"` (not `OUT_DIR / "out"`).
- The guard checks `CONTENT_DIR.is_dir()` exists.
- File iterator: `CONTENT_DIR.rglob("*.mdx")` — walks all `.mdx` files recursively.
- No Playwright, no `BASE_URL` constant needed.

**AC-1 function shape** (mirrors `sc1_html_metadata`):

```python
def ac1_source_doc_frontmatter():
    print("\n[AC-1] Every docs MDX file has source_doc: frontmatter")
    for path in CONTENT_DIR.rglob("*.mdx"):
        content = path.read_text(encoding="utf-8")
        rel = path.relative_to(CONTENT_DIR)
        if not re.search(r"^source_doc:", content, re.MULTILINE):
            fail(f"MISSING source_doc: frontmatter in {rel}")
        else:
            ok(f"source_doc: present in {rel}")
```

**AC-2 function shape** (mirrors `sc3_sitemap_robots` pattern — check + assert):

```python
UNENFORCED_FEATURES = ["release_age", "minimumReleaseAge", "lifecycle_script_allowlist"]

def ac2_unenforced_labels():
    print("\n[AC-2] Files mentioning unenforced features carry an unenforced label")
    for path in CONTENT_DIR.rglob("*.mdx"):
        content = path.read_text(encoding="utf-8")
        rel = path.relative_to(CONTENT_DIR)
        for feature in UNENFORCED_FEATURES:
            if feature in content:
                if not re.search(
                    r"(unenforced|not enforced|<UnenforcedCallout)",
                    content,
                    re.IGNORECASE,
                ):
                    fail(f"{rel} mentions '{feature}' but has no unenforced label")
                else:
                    ok(f"{rel} mentions '{feature}' and has unenforced label")
```

**AC-3 function shape**:

```python
PHANTOM_COMMANDS = [
    "beekeeper hooks status",
    "beekeeper catalogs rebuild",
    "beekeeper check --input",
    "beekeeper status",
]

def ac3_no_phantom_commands():
    print("\n[AC-3] No references to non-existent subcommands")
    for path in CONTENT_DIR.rglob("*.mdx"):
        content = path.read_text(encoding="utf-8")
        rel = path.relative_to(CONTENT_DIR)
        for phantom in PHANTOM_COMMANDS:
            if phantom in content:
                fail(f"{rel} references phantom command: '{phantom}'")
            else:
                ok(f"{rel} clean of '{phantom}'")
```

**Banner string** (for `__main__` block):

```python
print("=== Phase 18 Full Content Authoring — AC-1..3 Accuracy Gate ===")
```

---

## Shared Patterns

### Dual-Theme Raw Token Styling Discipline
**Source:** `web/components/changelog/breaking-change-callout.tsx` (lines 9–15, comment block)
**Apply to:** `web/components/docs/unenforced-callout.tsx`

```typescript
/**
 * Uses raw theme tokens var(--amber) so it renders correctly in BOTH light and
 * dark themes. Do NOT use --color-bk-amber which is dark-only and would freeze
 * to dark colors in light mode (12-03-SUMMARY Deviation 3/4).
 */
```

- `var(--red)`, `var(--amber)`, `var(--fg)`, `var(--bg)` — these switch with the theme.
- `var(--color-bk-*)` — dark-only, silently no-ops in light mode. Never use in components.

### MDX Component Global Registration
**Source:** `web/mdx-components.tsx` lines 1–21
**Apply to:** Any new MDX component added in Phase 18

Pattern: import at top of file, add bare name to the return object of `useMDXComponents`.
Components are available in ALL MDX files (docs + changelog) without per-file imports.

### Pure-Python Stdlib Test Harness
**Source:** `web/tests/seo_spec.py` lines 1–164
**Apply to:** `web/tests/accuracy_spec.py`

Pattern: `fail()`/`ok()` accumulator, section-header print per group, final `sys.exit(1 if failures else 0)`.
Run via `cd web && python tests/accuracy_spec.py`.
No pip dependencies — stdlib `os`, `pathlib`, `re`, `sys` only.

### Fumadocs Frontmatter Passthrough
**Source:** RESEARCH.md §5.1 (verified from `web/source.config.ts` + `web/lib/source.ts`)
**Apply to:** All 8 docs MDX files

`source_doc:` is an arbitrary frontmatter field. Fumadocs `defineDocs()` does NOT validate or
strip unknown frontmatter fields — they pass through to the page component silently. No schema
changes to `source.config.ts` or `lib/source.ts` are needed. The field is consumed only by
`accuracy_spec.py` via Python file-walk.

---

## No Analog Found

None. All 11 files have close analogs in the codebase.

---

## Metadata

**Analog search scope:** `web/content/`, `web/components/`, `web/tests/`, `web/mdx-components.tsx`
**Files scanned:** 12 (8 stubs + v1.3.0 changelog MDX + breaking-change-callout.tsx + seo_spec.py + mdx-components.tsx)
**Pattern extraction date:** 2026-06-09
