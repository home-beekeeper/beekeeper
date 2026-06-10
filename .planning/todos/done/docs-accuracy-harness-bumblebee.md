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

---

## ✅ RESOLVED 2026-06-10

Both items done after **verifying the code first** (per the nudge.md-deviation lesson — the todo's claims were checked, not trusted):

**1. Harness tier table (15 → 17).** Confirmed `internal/hooks/hooks.go` `allTargets` has **17** entries and that `continue` + `openclaw` are both gateway targets (`gatewayTargets` map + the `printGatewayGuide` switch, hooks.go:142). Fixed `web/content/docs/integration.mdx` (count 15→17; Tier-3 row now Kilo/Trae/Continue/OpenClaw; added their install commands + an honest gateway-only caveat with the real config paths `~/.continue/config.yaml` + `~/.openclaw/config.json`) AND found a second bug: line 47 had `--target continue` mislabeled "Gemini CLI family" while `--target gemini` was *missing* from the Tier-1 list — fixed (continue moved to Tier-3, gemini restored to Tier-1). Updated `docs/harness-support-matrix.md` to the same 17 (added rows 16/17, Tier-3 bullets, honesty-note 4 generalized, 14→16 non-Claude counts, source citations). Swept + fixed the stale "14/15" counts in getting-started.mdx, security.mdx, and THREAT-MODEL.md:939. (Left the v1.3.0 changelog "Known Deferred at Close" list alone — it is a separate stale draft that lists completed Phases 16/18 as deferred and is reconciled at milestone close.)

**2. Bumblebee / pollen role.** Verified `internal/catalog/sync.go` `bumblebeeContentsURL = https://api.github.com/repos/perplexityai/bumblebee/contents/threat_intel` (GitHub Contents API, cross-platform, no platform guard — a catalog *source*, not a binary Beekeeper runs) and `internal/scan/scanner.go` exec's `pollen` via `exec.LookPath` (cross-platform). Added a "Where the threat intel comes from" note to `getting-started.mdx` §3 explaining Bumblebee (catalog source over HTTP) vs pollen (the exec'd scanner, a `bantuson` fork adding Windows), and stating Windows is a first-class target. **Did NOT** add any Windows-exclusion warning (the premise "Windows users can't use it" is false).

**Verification:** `pnpm build` exit 0; `accuracy_spec` (source_doc paths exist + no phantom commands) + `seo_spec` + `gfx_spec` PASS; `home_spec` PASS on retry (the dual-theme body-bg check flaked once; home page untouched in this task). No Go changed.
