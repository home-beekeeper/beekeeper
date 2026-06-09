# 18-01 Summary — Wave-0 Accuracy Infrastructure

**Plan:** 18-01-PLAN.md (Wave 1) · **Status:** ✅ Complete · **Date:** 2026-06-09
**Requirements:** DOCS-09 (accuracy-gate machinery)

## What was built

| Artifact | Purpose |
|----------|---------|
| `web/tests/accuracy_spec.py` | Pure-Python (stdlib only) DOCS-09 gate over `content/docs/**/*.mdx`: AC-1 (`source_doc:` frontmatter present + every referenced repo-root path exists on disk), AC-2 (any mention of `release_age`/`minimumReleaseAge`/`lifecycle_script_allowlist` must carry an "unenforced"/"not enforced" label or `<UnenforcedCallout`), AC-3 (no phantom commands: `beekeeper hooks status` / `catalogs rebuild` / `check --input` / `beekeeper status`). |
| `web/components/docs/unenforced-callout.tsx` | `UnenforcedCallout` — amber dual-theme callout (`var(--amber)`/`var(--fg)`, NOT `--color-bk-*`), `Info` icon, header `Not enforced in v1.3.0 — {feature}`. Clone of `breaking-change-callout.tsx`. |
| `web/mdx-components.tsx` | Registered `UnenforcedCallout` (import + return-object entry after `BreakingChangeCallout`); the `as MDXComponents` cast + Phase-16 comment preserved. |

## Verification

- `python tests/accuracy_spec.py` → **exit 1 (RED)** — the correct Wave-0 state. It reported all 8 stubs missing `source_doc:` AND caught the troubleshooting stub's three phantom commands (`beekeeper hooks status`, `catalogs rebuild`, `beekeeper status`) — proving AC-3 works before content lands.
- `pnpm build` → **exit 0** with the new component compiled under the Next.js 16 static export.

## Acceptance criteria — all met

- accuracy_spec.py imports stdlib only; prints AC-1/AC-2/AC-3 headers; exits non-zero (RED) now; AC-1 checks referenced-path existence; AC-3 has all four phantom commands; references `UnenforcedCallout` for the AC-2 label.
- `export function UnenforcedCallout` present (1); no `color-bk` (0); `UnenforcedCallout` in mdx-components.tsx (2 — import + registration); `as MDXComponents` cast intact; `pnpm build` exit 0.

## Notes

- Banner string is `=== Phase 18 Full Content Authoring — AC-1..3 Accuracy Gate ===` (the `�` in the Windows console is cosmetic em-dash rendering; the file is UTF-8).
- Wave-0-first staging mirrors seo_spec.py/gfx_spec.py: the gate exists before content so each Wave-2 authoring task gets ≤3s per-commit feedback.

## Commits

- `71e3851` test(18-01): accuracy_spec.py DOCS-09 gate
- `f61bb07` feat(18-01): UnenforcedCallout amber dual-theme component + register
