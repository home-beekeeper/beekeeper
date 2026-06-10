# Phase 20 — Runtime Hardening II (Tiers 1–3) — PLAN

**Milestone:** v1.3.0 (current). **Type:** Go runtime + Python sidecar + web/threat-model honesty text. **Executor:** inline on main (Go; web honesty edits need pnpm).
**Research:** `20-RESEARCH.md` (decisions D-T1-host/interval, D-T2-ipc/codeshield/alignment, D-T3-write/gate). Analysis: `analysis/sentry-coverage-2026-06.md`.

## Goal
Close the three runtime gaps from the 2026-06-10 audit: manual-only catalog sync, a non-functional (and silently fail-open) LlamaFirewall opt-in, and narrow/overstated Sentry coverage — and make the docs/threat-model match reality.

## Suggested plan-file split for execution
`20-01` Tier 1 · `20-02` Tier 2 · `20-03` Tier 3 rules (W1) · `20-04` Tier 3 file-write (W2) · `20-05` honesty+tests (W3) · `20-06` DNS stretch. (Tiers are independent → can run in any order / parallel; Tier 3 is the largest.)

---

## TIER 1 — Background catalog sync + TUI scheduler (CSYNC)

**T1.1 Config schema.** `internal/config/config.go`: add `CatalogSyncConfig{Enabled bool, Interval string}` (pointer field on `Config`), `DefaultCatalogSyncConfig()={true,"12h"}`, `ValidateCatalogSyncConfig` (reject non-duration / out of [5h,24h]), accessor `CatalogSyncInterval()` (parse+clamp, default on empty). Resolve nil→default + validate in `Load`. `layered.go`: `mergeCatalogSync` (mirror `mergeNudge` :358) in `merge`; **`mergeCatalogSyncUntrusted` refuses `enabled:false` from project/env** (mirror `mergeNudgeUntrusted` :519). **Accept:** unit tests — bad interval rejected; clamp works; project-layer can't disable.

**T1.2 State timestamps.** `internal/catalog/state.go`: add `LastSuccess/LastAttempt/LastError/ETag` (`omitempty`) to `SourceState`. **Accept:** existing `state.json` still loads (backward-compat test).

**T1.3 Conditional sync.** `internal/catalog/sync.go`: send `If-None-Match: <prevETag>` on the list call; on 304 skip fetches + bump `LastAttempt/LastSuccess`; on 200 store new ETag + proceed. Keep last-good on error (already true) + record `LastAttempt/LastError`. **Accept:** 304 path does no fetch/rebuild; error path leaves index intact.

**T1.4 User-level sync daemon.** New `beekeeper catalogs daemon install|uninstall|status` (unprivileged) registering an OS job that runs `beekeeper catalogs sync` on an **hourly heartbeat**; `catalogs sync` no-ops unless `time.Since(LastSuccess) ≥ CatalogSyncInterval()` (D-T1-interval). Per-OS, built from `protect_*.go` templates but unprivileged: Linux `~/.config/systemd/user/*.{service,timer}` + `systemctl --user`; macOS `~/Library/LaunchAgents/*.plist` `StartInterval` + `launchctl bootstrap gui/$(id -u)`; Windows `schtasks /create` current-user (no `/ru SYSTEM`, no `/rl HIGHEST`). **Accept:** install/uninstall idempotent per OS (CI-gated); `catalogs sync` honors the interval gate (injected-clock unit test).

**T1.5 TUI.** `tui/catalogs_panel.go`+`model.go`: wire `syncCatalogsMsg`→async `runSyncCmd` (mirror `scan_panel.go:81`)→`syncDoneMsg` toast (real `catalog.Sync`); add admin-gated 5h/10h/24h/off selector persisting via validate-before-write (mirror `policy_panel.go:369`); `pipColor` keys off `LastSuccess`, amber when `LastAttempt>LastSuccess`. **Accept:** `s` performs a real sync; selector persists; failing sync shows amber, not "fresh".

**T1.6** `hooks install` offers to register the sync daemon + does one first-run sync (D-T1 open-q, recommended). **Accept:** opt-in prompt; first index present after install.

---

## TIER 2 — LlamaFirewall opt-in actually works (LLMF)

**T2.1 Move + embed assets.** Move `sidecar/` → `internal/llamafirewall/assets/`; `sidecar_assets.go` `//go:embed assets/llamafirewall_sidecar.py assets/requirements.txt`; `InstallSidecar(stateDir)` writes them (0600) with a sha/version stamp, rewrites on mismatch. **Accept:** install writes the script; upgrade re-stamps; unit test on hash-skip.

**T2.2 Fix the sidecar API (the silent no-op).** In the embedded script: build `LlamaFirewall(scanners={Role.USER:[ScannerType.PROMPT_GUARD]})` **once at startup**; `lf.scan(UserMessage(content=...))`; map `decision!=ALLOW`→injection; stop swallowing exceptions into `clean` (return an `error` field the Go side treats fail-closed). **Accept:** a known injection string → injection (proven in T2.6 e2e).

**T2.3 IPC → loopback TCP + token (D-T2-ipc).** Python: `AF_INET` bind `127.0.0.1`, drop unlink/chmod. Go: collapse `client.go`/`client_windows.go` → one TCP `client.go` (delete the pipe fork; `proto.go` unchanged); supervisor readiness = TCP dial-retry (replace `os.Stat(sock)` `supervisor.go:128`); Go picks a free port + a per-launch token, passes both via `cmd.Env` (`BEEKEEPER_LLMF_PORT/TOKEN`); Python rejects token mismatch. **Accept:** sidecar reachable on all 3 GOOS builds; non-token request rejected.

**T2.4 De-stub CodeShield; remove AlignmentCheck (D-T2).** Wire `scan_code` to `ScannerType.CODE_SHIELD` (`AssistantMessage`); delete `scan_alignment` + the dead alignment block `handler.go:658-668` + the `AlignmentCheck` config field. **Accept:** an obvious-injection code snippet → unsafe; no decision path a stub can't reach (the CodeShield block at `handler.go:643` now reachable).

**T2.5 Models/deps/cache + fix StateDir bug.** `llamafirewall install` = venv under stateDir + `pip install -r requirements.txt` (pin + CPU torch index) + `llamafirewall configure` model pre-pull, with `HF_HOME=<stateDir>/llamafirewall/hf` injected by supervisor env; default the **22M** gated model; surface the Llama-license + `huggingface-cli login` step. Fix `enable/disable/status` to use `platform.StateDir()` not `$HOME` (`main.go:1658,1677,1697`). **Accept:** install bootstraps a working venv; status finds state on Windows.

**T2.6 Real-sidecar e2e + honesty.** `//go:build e2e` test (gated `BEEKEEPER_LLMF_E2E=1`, Linux nightly/dispatch, `HF_TOKEN` secret, cached model, CPU torch): start real sidecar → benign-allow / injection / CodeShield-unsafe / crash-fail-closed; add the missing `TestSupervisorRestartOnCrash` (use the existing unused `startCrashingSidecar`). Docs/home: state Python+gated-model+no-key, non-blocking injection scan, opt-in steps; soften `feature-cards.tsx` LlamaFirewall claim; mark experimental until this lands. **Accept:** e2e green in the gated job; docs accurate.

---

## TIER 3 — Sentry v1.x closure + honesty (SENT)

### W1 — cheap rule wins (no new event source)
**T3.1 Watchlist expansion.** `rules.go:12` `defaultSensitivePaths` + `linux/daemon.go:25` `daemonSensitivePaths`: add `.config/gcloud`, `.azure`, `.kube/config`, `.docker/config.json`, `.claude/`. **Accept:** `.aws`+`.config/gcloud` reads → SENTRY-001.
**T3.2 SENTRY-006 + `isMonitoredDescendant`.** Add `agentExes`+`isAgentDescendant`; refactor SENTRY-001/002/003/005 guards (`rules.go:247,298,345,424`) to `isMonitoredDescendant` (editor OR agent); SENTRY-006 = agent-descendant credential cluster. **Accept:** agent-in-bare-terminal 2 cred reads → fires; integrated-terminal doesn't double-fire.
**T3.3 SENTRY-007 + `isExternalDest`.** Precomputed private-CIDR set; gate SENTRY-003 + 007 on external dest; SENTRY-007 = monitored-descendant + (recent cred read OR persistence write) + external outbound /5min → critical (warn-first in baseline). **Accept:** cred read→external outbound (no fresh ext)→fires; loopback/RFC1918→no fire (+ `::ffff:10.0.0.1` test).
**T3.4 Purity test.** Add `TestRulesImportsArePure` (forbid os/net-http/io/sync/context; allow net/time).

### W2 — file-write persistence
**T3.5 Event model.** `types.go:25` append `EventFileWrite`; `rules.go` add `persistenceWritePaths`+`isPersistencePath`; `evalSENTRY008` dispatched from new `case EventFileWrite`. `daemonPersistenceDirs` mirror. **Accept:** synthetic write to `~/.claude/settings.json` by monitored descendant → SENTRY-008; benign path → no fire.
**T3.6 Linux ingestion.** New 2nd fanotify group `FAN_CLASS_NOTIF|FAN_REPORT_DFID_NAME` (kernel ≥5.9 gate), mark persistence parent dirs `FAN_CREATE|FAN_MOVED_TO|FAN_ONDIR`; new read loop (open_by_handle_at + name) → `EventFileWrite`. Degrade if <5.9. **Accept:** builds; OS-gated CI capture.
**T3.7 macOS ingestion.** Subscribe `write`,`rename`; parser cases → `EventFileWrite`; **fix `esloggerCreateEvent` to read `new_path.dir`+`filename` union** (`darwin/parser.go:55`). **Accept:** new-file create parsed (fixture); write/rename emit `EventFileWrite`.
**T3.8 Windows ingestion.** Split Kernel-File branch (`windows/parser.go:136`): 15→read/`EventFileAccess`; **16/30/27(/19)→`EventFileWrite`**; fix the wrong 12/14/15 comment. **Accept:** write/createnew/rename → `EventFileWrite` (verify field keys vs live golang-etw).

### W3 — honesty + tests
**T3.9 Honesty edits** (apply `analysis/.../§6` diffs): `PROJECT.md` `#### Sentry Daemon`/`### Out of Scope`; `THREAT-MODEL.md §8` (fix stale 004/005-don't-fire + fanotify-no-drops; add editor/agent-scope + detection-only + no-DNS/memory gaps); home `how-it-works.tsx:177`, `feature-cards.tsx:52`, `honesty-callout.tsx` (add Sentry gap). **Gate:** `pnpm build` + accuracy/seo/home/gfx specs green.
**T3.10 Tests.** Synthetic `rules_test.go` cases for 006/007/008 + watchlist. `go test ./internal/sentry/...` + `go vet`.

### W4 — STRETCH (drop if scope fills)
**T3.11 DNS.** Linux kprobe `udp_sendmsg`/`tcp_sendmsg` dport 53 + QNAME parse → `EventDNSQuery` (new bpf2go target); Windows ETW DNS-Client `{1C95126E-...}` ID 3006 `QueryName`. macOS out.

---

## Phase-level verification gates (complete when ALL true)
1. `go build ./...` (all 3 GOOS) + `go test ./...` + `go vet` green; `TestRulesImportsArePure` green.
2. Tier 1: injected-clock test proves interval-gated sync; project-layer can't disable; TUI `s`/selector work.
3. Tier 2: gated `//go:build e2e` proves benign-allow / injection / CodeShield-unsafe / crash-fail-closed; no stub-only decision path; StateDir bug fixed.
4. Tier 3: SENTRY-006/007/008 fire on target + not on baselines; `EventFileWrite` ingestion builds on all 3 GOOS; watchlist trips SENTRY-001 on 2-cloud-cred.
5. Honesty: THREAT-MODEL §8 + home no longer overstate Sentry/LlamaFirewall; `pnpm build` + web specs green.

## Out of scope (residual / v2 — state, don't build)
Process-memory-read detection (mac/win entitlement/kernel; Linux eBPF later); macOS DNS (NetworkExtension); Windows missing-PPID (residual); exfil over legitimate endpoints (GitHub API dead-drops / AWS-service C2 / npm-registry worm) — architectural only. Separate web-docs accuracy (openclaw/continue tier table, Bumblebee/pollen role) stays in `.planning/todos/pending/docs-accuracy-harness-bumblebee.md`.

## Risks
- Tier 2 is cross-language + CI-heavy + depends on a gated HF model (needs an org HF token in CI) — keep the e2e job non-blocking for PRs, blocking for a release tag.
- Tier 3 file-write ingestion is per-OS and hardest (separate fanotify group ≥5.9; ETW field-key verification; macOS union fix) — land W1 + honesty first (cheap, high-trust), W2 second.
- Windows is the dev machine; non-native ingestion (Linux/mac) is build-check-only locally → CI-validated (existing constraint).
