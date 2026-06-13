# Phase 20 — Runtime Hardening II (Tiers 1–3) — RESEARCH

**Date:** 2026-06-10. Implementation research for the three runtime gaps. Every claim has a code file:line or an external source. Findings that **change** the plan are flagged ⚠.

## Decisions (the forks that needed research)

| # | Decision | Choice | Why |
|---|----------|--------|-----|
| D-T1-host | Where a background catalog sync runs for a hook-only user | **New unprivileged `beekeeper catalogs daemon` registered as a user-level OS job** (systemd `--user` timer / launchd LaunchAgent / Windows per-user `schtasks`) whose action is `beekeeper catalogs sync`. OS scheduler *is* the scheduler — no long-lived Go process. | Gateway needs `--upstream`, Sentry needs root → neither is "always on." User-level jobs need no elevation. Reuses `protect_*.go` templates (unprivileged variants). |
| D-T1-interval | OS-job period vs config interval (avoid drift) | **OS job = frequent heartbeat (hourly); `catalogs sync` no-ops unless `time.Since(LastSuccess) ≥ cfg.Interval`.** Config interval is authoritative; `off` is instant. | Avoids re-writing the OS schedule on every TUI change. |
| D-T2-ipc | Python AF_UNIX vs Windows named-pipe Go client | **Switch BOTH sides to loopback TCP `127.0.0.1` + per-launch bearer token.** | Only option that makes Windows work without a 2nd Python dep; `proto.go` framing is transport-agnostic (zero change); deletes `client_windows.go`; fixes the racy `os.Stat(sock)` readiness check (→ TCP dial-retry). Token restores the access control the `0600` socket gave. |
| D-T2-codeshield | De-stub CodeShield? | **YES.** Local Semgrep+regex, no model, no key. | Cheap, aligns with local-first. |
| D-T2-alignment | De-stub AlignmentCheck? | **NO — remove it** (delete dead handler block `handler.go:658-668`). | Requires `TOGETHER_API_KEY` → exfiltrates agent context to a 3rd-party cloud; violates the local-first/fail-closed threat model. |
| D-T3-write | New event kind vs write-bool | **New `EventFileWrite` kind** appended to `types.go` iota. | Clean `switch` dispatch to SENTRY-008; keeps the read-clustering SENTRY-001 path uncontaminated; minimal additive change. |
| D-T3-gate | Agent coverage | **Refactor editor-only guards to a single `isMonitoredDescendant` (editor OR agent)** rather than parallel rules. | VS Code/Cursor integrated terminals are already editor-descended → avoids double-firing; agents in bare terminals/CI/SSH get coverage. |

## Tier 1 — catalog sync (key facts)

- ⚠ **Rate limit is a non-issue.** `catalog.Sync` (`sync.go:41`) = 1 list call to `api.github.com` + N raw fetches to `raw.githubusercontent.com`. **Only the list call counts** against the 60/hr unauthenticated bucket; raw fetches are unmetered (different host). At a 5h floor that's ~5 metered calls/day. Add `ETag`/`If-None-Match` on the list for no-change skip (304 is only quota-free when authenticated, but quota isn't the concern — the win is skipping the 10 fetches + index rebuild on no change).
- ⚠ **`SourceState` has no timestamps** (`state.go:17-34`) — "last success vs last attempt" (and ETag) require **adding fields** (`LastSuccess/LastAttempt/LastError/ETag`, all `omitempty` → backward-compatible with existing `state.json`).
- **Last-good-on-failure already holds**: `Sync` writes the index only after all files parse (`sync.go:88-99`) and returns early on any error — the scheduler must only ever call `Sync` (never pre-delete).
- Config: `CatalogSyncConfig{Enabled bool, Interval string}` **pointer** in `config.go` (so merge can tell absent from disabled); `ValidateCatalogSyncConfig` rejects out-of-range, accessor clamps `[5h,24h]` defensively (mirror `drift.go:230` parse). Layered merge mirrors `mergeNudge` (`layered.go:358`).
- TUI: wire `syncCatalogsMsg` (`model.go:166`, currently a no-op toast) to an async `tea.Cmd` mirroring `ScanPanel.runScanCmd` (`scan_panel.go:81`) → `syncDoneMsg` toast; selector persists via validate-before-write like `PolicyPanel.persist` (`policy_panel.go:369`); `pipColor` should key off `LastSuccess`, amber when `LastAttempt > LastSuccess`.
- Self-defense: project/env layer setting `enabled:false` is **refused** (mirror `mergeNudgeUntrusted` `layered.go:519`) — disabling sync is a security-relaxing lever.
- Pitfall: systemd `--user`/LaunchAgents stop when logged out (lingering can need admin) — acceptable for v1 (agent only runs while user active); Windows `schtasks` works for a standard user as long as you DON'T pass `/ru SYSTEM` or `/rl HIGHEST`.

## Tier 2 — LlamaFirewall (key facts)

- ⚠ **Prompt scanning is a silent fail-open no-op TODAY, even on Linux.** `sidecar:71` calls `UserMessage(role="user", content=...)` — `role=` is not a constructor arg (it's the scanners-dict key); this `TypeError`s, gets swallowed by the `except` (`sidecar:86-94`), and returns `result="clean"`. Also `LlamaFirewall()` is built with no `scanners=` and **re-constructed per request** (reloads the model every scan). Real API: `LlamaFirewall(scanners={Role.USER:[ScannerType.PROMPT_GUARD]})` once at startup; `lf.scan(UserMessage(content=...))`; `result.decision != ScanDecision.ALLOW` → injection.
- ⚠ **PromptGuard 2 is a GATED HF model** (`meta-llama/Llama-Prompt-Guard-2-86M`/`-22M`) — needs Llama-license acceptance + an HF token (`huggingface-cli login`). Install must surface this or first scan fails-closed with an opaque 401. Default to **22M** for CPU/Windows. No API key for inference (local).
- **CodeShield** = local Semgrep+regex, no model/key → de-stub. **AlignmentCheck** = Llama-4-Maverick via Together AI cloud + `TOGETHER_API_KEY` → remove.
- Installer: `//go:embed` requires assets under the embedding package → **move `sidecar/` into `internal/llamafirewall/assets/`** (can't embed `../../sidecar`). `InstallSidecar(stateDir)` writes script+requirements with a version/sha stamp, re-writes on upgrade; self-heal at gateway start (idempotent). Cache pinned via supervisor env `HF_HOME=<stateDir>/llamafirewall/hf` (`cmd.Env` already forwards env, `supervisor.go:115/186`). Deps in a venv under stateDir; `PythonPath`→venv interpreter; pin `requirements.txt` + CPU torch index (default torch pulls multi-GB CUDA wheels).
- ⚠ **Existing bug:** `enable/disable/status` use `$HOME/.beekeeper` not `platform.StateDir()` (`main.go:1658,1677,1697`) → wrong dir on Windows; fix.
- e2e: `//go:build e2e`, gated `BEEKEEPER_LLMF_E2E=1`, Linux-only nightly/dispatch, `HF_TOKEN` secret, cached 22M model, CPU torch; start the real sidecar → benign-allow / injection / CodeShield-unsafe / crash-fail-closed. (No real-sidecar test exists today; `Supervisor.Start/relaunch/watchProcess` untested.)
- Package: `pip install llamafirewall` (PyPI v1.0.3, py≥3.10) — ⚠ verify `requirements.txt`'s hyphenated `llama-firewall` resolves to the real Meta dist, not a squat.

## Tier 3 — Sentry (key facts)

- **Purity:** `rules.go` MAY import `net`+`time` (already does) but not `os/net-http/io/sync/context`. Add `TestRulesImportsArePure` with that narrower forbidden set.
- ⚠ **Linux fanotify writes need a SEPARATE group.** Create/modify/move dir events are 5.1+, but to get the path you need `FAN_REPORT_DFID_NAME` (5.9+), which is **incompatible with the existing `FAN_CLASS_CONTENT` permission group (`EINVAL`)**. → open a 2nd group `FAN_CLASS_NOTIF|FAN_REPORT_DFID_NAME`, mark **parent dirs** with `FAN_CREATE|FAN_MOVED_TO|FAN_ONDIR` (prefer MOVED_TO/CREATE; MODIFY is chatty; editors write-temp-then-rename). New read loop (no fd; `open_by_handle_at` on dir handle + name). Gate kernel ≥5.9 → degrade if older.
- ⚠ **Windows ETW Kernel-File IDs are mislabeled in our code.** `parser.go:98,136` treats 12/14/15 as "create/name" — actually **12=Create(open handle), 14=Close, 15=Read**. For writes use **16=Write, 30=CreateNewFile, 27=RenamePath, 19=Rename**. Split the branch: 15→read/`EventFileAccess`, 16/30/27→`EventFileWrite`.
- ⚠ **macOS create-parse bug:** `esloggerCreateEvent` (`darwin/parser.go:55-61`) only reads `destination.existing_file.path`; a **new** file's path is in `destination.new_path.dir.path`+`.filename` → current parser drops new-file creates. Fix the union; add `write`/`rename` subscriptions + cases.
- **SENTRY-008** (new): `isMonitoredDescendant` + `EventFileWrite` to a `persistenceWritePaths` entry → `high`/warn, stateless. Paths: `.config/systemd/user/`, `Library/LaunchAgents/`+`LaunchDaemons/`, `.vscode/tasks.json`+`settings.json`, `.claude/settings.json`+`.claude/`, `.cursor/`. `daemonPersistenceDirs` mirror for fanotify marks.
- **SENTRY-006**: `agentExes` (claude/codex/cursor-agent/gemini/copilot/qwen/aider/opencode/hermes/goose/amp — cross-check basenames vs `internal/hooks/*`) + `isAgentDescendant`; refactor SENTRY-001/002/003/005 guards (`rules.go:247,298,345,424`) to `isMonitoredDescendant`.
- **SENTRY-007**: `isExternalDest(net.IP)` excluding loopback/RFC1918/link-local/ULA/CGNAT (precompute `[]*net.IPNet` at init). Gate SENTRY-003 + 007 on external dest; 007 = monitored-descendant + (recent cred read OR recent persistence write) + external outbound /5min → critical. Watchlist expansion (gcloud/azure/kube/docker/.claude) in `defaultSensitivePaths` (`rules.go:12`) + `daemonSensitivePaths` (`linux/daemon.go:25`).
- **DNS (stretch):** Linux kprobe `udp_sendmsg`/`tcp_sendmsg` filtered dport 53 + in-kernel QNAME parse → new `EventDNSQuery` (uprobe `getaddrinfo` rejected — Go bypasses libc + 10–20× overhead). Windows ETW `Microsoft-Windows-DNS-Client` `{1C95126E-7EEA-49A9-A3FE-A378B03DDB4D}` ID **3006** `QueryName`. macOS DNS out (NetworkExtension).

## Open questions (resolve at execute or in discuss)
- T1: should `hooks install` *offer* to register the user-level sync job + do one first-run sync? (recommended)
- T2: confirm CodeShield sync vs async (`lf.scan` vs `scan_async`) in v1.0.3; confirm 22M-vs-86M default; verify the `llama-firewall`→`llamafirewall` dist name.
- T3: validate exact ETW field keys for write/rename templates against live `tekert/golang-etw` output (CLAUDE.md Phase-7 ETW-field research flag); confirm `tekert/golang-etw` can enable a manifest provider (DNS-Client) on the existing session.

## Sources
GitHub rate limits / conditional requests (docs.github.com REST rate-limits + best-practices); systemd user timers (Arch Wiki); launchd LaunchAgents + `launchctl bootstrap` (Apple docs); `schtasks` (MS Learn); llamafirewall PyPI 1.0.3 + PurpleLlama docs (how-to-use, prompt-guard tutorial); HF gated models Llama-Prompt-Guard-2-86M/22M; fanotify(7)/fanotify_init(2) man-pages; Microsoft-Windows-Kernel-File manifest (repnz/winevt-kb); Apple es_event_create_t/es_event_rename_t + eslogger(1); Microsoft-Windows-DNS-Client manifest; Brendan Gregg (uprobe overhead). Full URLs in the agent transcripts.
