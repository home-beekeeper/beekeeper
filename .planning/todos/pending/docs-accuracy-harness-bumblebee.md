# Docs accuracy gaps — harness table + Bumblebee/pollen (found 2026-06-10)

Quick web-docs fixes. Separate from Phase 20 (runtime). Tend to during v1.3.0 close.
(Sentry/LlamaFirewall honesty edits + the catalog-sync/LlamaFirewall runtime work moved INTO Phase 20 — `phases/20-runtime-hardening/`.)

## 1. Harness tier table is stale ("15" is wrong; two targets missing/mis-tiered)
Code has **17** `--target` values (`internal/hooks/hooks.go:58-65`), docs claim **15**.
- **openclaw** — supported in code (Tier-3 gateway, `hooks.go:35`, `printOpenClawGuide`, tested `hooks_test.go:505`) but **absent** from the tier table.
- **continue** — also missing AND mis-tiered: docs put it under Tier-1 "Gemini family" (`integration.mdx:47`) but code routes it as a Tier-3 gateway target (`hooks.go:41,142`).
- Fix `web/content/docs/integration.mdx` tier table + `docs/harness-support-matrix.md` (both list the same wrong 15); correct the count and add openclaw + continue as Tier-3.

## 2. Bumblebee / pollen role undocumented
- Bumblebee = the Perplexity **threat-intel catalog source**, fetched over the GitHub API (`internal/catalog/sync.go:18`), **cross-platform** (no platform guard). NOT a binary Beekeeper runs.
- **pollen** = the exec'd scanner (a `bantuson` fork of Bumblebee adding Windows), run on all 3 OSes (`internal/scan/scanner.go:57`).
- The bumblebee *binary* being macOS/Linux-only is **moot** — Windows users are fully supported via pollen + the HTTP catalog. Premise "Windows users can't use it" is wrong.
- Gap: neither role is explained in user docs (Bumblebee appears twice incidentally; pollen never named). Add a short "where threat intel comes from" doc note; do NOT add a false Windows-exclusion warning.
