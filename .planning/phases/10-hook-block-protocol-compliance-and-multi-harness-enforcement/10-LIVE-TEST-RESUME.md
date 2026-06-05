# Phase 10 — Live-Test Resume (HPC-04)

Resume context for the **live** Claude Code hook-block test. The state below is from the
2026-06-05 session (about to be cleared). This is the manual, local-only test that PROVED
the bug and must be re-run to PROVE the fix once HPC-01/HPC-02 are implemented.

## Machine state at session end (2026-06-05)

- **`~/.claude/settings.json`**: CLEAN — restored byte-exact (3837 bytes; 5 PreToolUse GSD hooks, no beekeeper hook). The test hook was uninstalled + backups removed. Nothing to undo.
- **Catalog**: real 686-entry index at `%APPDATA%\beekeeper\catalogs\bumblebee.idx` (from `beekeeper catalogs sync`). Needed — without a catalog, `beekeeper check` fail-closed-blocks everything.
- **Binary on PATH**: `C:\Users\Bantu\go\bin\beekeeper.exe` — built WITH the timeout fixes but WITHOUT `--hook` (Phase 10 adds that). After implementing HPC-01, `go install ./cmd/beekeeper` to refresh it.
- **`~/.beekeeper` does NOT exist** on Windows — the state dir is `%APPDATA%\beekeeper` (Windows `platform.StateDir()`).

## The bug this re-proves (baseline, already confirmed)

`beekeeper check` exits `1` + emits `{"Allow":false,...}`. Claude Code treats exit 1 as a
non-blocking soft error → the tool RUNS. Confirmed via canary (ran) + audit log (a PreToolUse
`block` record immediately followed by a PostToolUse `tool_result` — the tool_result only
exists if the tool executed).

## Re-test procedure (after HPC-01 `--hook` + HPC-02 installer are implemented)

1. `go install ./cmd/beekeeper` (refresh the PATH binary with the `--hook` build).
2. Ensure catalog present: `beekeeper catalogs sync` (skip if `%APPDATA%\beekeeper\catalogs\bumblebee.idx` exists).
3. **Pre-flight (no lockout):** `printf '{"tool_name":"Read","tool_input":{"file_path":"./README.md"}}' | beekeeper check --hook claude-code; echo $?` → expect 0. And a credential read → expect **exit 2** now (was 1). Confirms the adapter before touching live settings.
4. Back up settings byte-exact: `cp ~/.claude/settings.json /tmp/bk-settings-orig.json`.
5. `beekeeper hooks install --target claude-code` (now merges + wires `beekeeper check --hook claude-code`). Verify GSD hooks preserved (PreToolUse count goes 5→6).
6. **RESTART Claude Code** — hooks load at session start; the hook must be present before the session begins. (Docs say mid-session reload works, but the canonical proof uses a fresh session.)
7. In the new session, attempt the canary credential read via the Bash tool — **NONEXISTENT path, never the real key**:
   `echo "RAN: $(cat ~/.ssh/id_rsa-beekeeper-canary-DOES-NOT-EXIST 2>&1)"`
   - **PASS (fix works):** the Bash tool call is DENIED by the PreToolUse hook (you never see "RAN:").
   - **FAIL (still broken):** you see `RAN: cat: ...: No such file`.
   ⚠️ NEVER `cat ~/.ssh/id_rsa` (the real key likely exists → would leak). Always use a canary suffix.
8. **Smoking-gun cross-check** in `%APPDATA%\beekeeper\audit\beekeeper.ndjson`: after the canary, a PreToolUse `policy_decision":"block"` with **NO** following PostToolUse `tool_result` for that call = the tool did NOT run (fix confirmed). (Before the fix: block WAS followed by tool_result.)
9. Repeat for a `Read` tool on a canary `~/.aws/credentials-canary` (different extraction path).
10. **Cleanup:** `beekeeper hooks uninstall --target claude-code`, then restore byte-exact: `cp /tmp/bk-settings-orig.json ~/.claude/settings.json`; verify with `cmp`. Remove `*.beekeeper-backup-*`.

## Honest scope of the live test

- Only **Claude Code** is installed on this machine → it is the ONLY harness whose live block can be verified here. The other 14 are verified against documented contracts only (HPC-03 contract-shape tests), never live. That's the ceiling; user-filed issues are the real-world feedback loop for the rest.
- `~/.claude/` is NOT in `policy.DefaultSensitivePaths`, so editing settings.json to install/uninstall is never self-blocked.

## CI coverage reality (answers "does CI test hooks?")

- `ci.yml` runs `go test -v -race ./...` → this covers `internal/hooks/...` (the **installer** config-writing tests, incl. `TestInstallClaudeCodePreservesExistingHooks`) and `internal/check/...` (the `beekeeper check` **decision + internal exit code**). So CI tests *what config the installer writes* and *what Decision/exit code the binary returns*.
- CI does **NOT** run any real agent harness (no Claude Code/Cursor/Codex in CI) → it **cannot** verify whether a harness actually HONORS the hook to block. That is **local + manual, Claude-Code-only**.
- The `-tags e2e` release gate (`TestE2ELiveBinary`) is **not wired into ci.yml** (no `-tags e2e` step) — it runs locally only. And even it asserts beekeeper's exit code `1`, not the harness deny contract.
- **Plus:** the repo has never been pushed (local-only), so CI has **never actually run**.
- **This is exactly why the exit-1 bug shipped:** every test (CI or e2e) asserted Beekeeper's *internal* contract (exit 1 = block); none asserted the *harness* deny contract (exit 2 / deny JSON), and no harness runs anywhere in the pipeline. **HPC-03/HPC-06 close this**: add CI-runnable tests asserting the binary EMITS exit 2 + the correct per-harness deny JSON (these need no harness), making the harness contract a release gate. The true live "harness honors it" check stays manual + Claude-Code-only (this file).

---

## POST-FIX RESUME (2026-06-05, after Phase 10 execution)

**Status:** Phase 10 code is DONE + committed on `main`. 10-01/03/04/05/06 complete + verified;
full `go test ./...` green. A deviation bug was found during the live pre-flight and FIXED
(commit `f315c81`): `--hook` mode was leaking the raw `{"Allow":false,...}` Decision JSON
*before* the harness deny form — which would **silently allow on Hermes** (fail-open, ignores
exit codes, parses first JSON object). Now `beekeeper check --hook <h>` emits ONLY the harness
deny form. CLI pre-flight (post-fix) PROVEN: `--hook claude-code` block → exit 2 + only
`hookSpecificOutput` deny; `--hook hermes` block → exit 0 + only `{"action":"block",...}`;
default path unchanged (raw JSON + exit 1); allow → exit 0 nothing.

**Machine state RIGHT NOW (hook IS installed — do the live proof):**
- `~/.claude/settings.json`: beekeeper hook INSTALLED (merge-safe: 5 GSD PreToolUse hooks
  preserved + `beekeeper check --hook claude-code`; PostToolUse `beekeeper audit-record`).
  Now 5480 bytes (was 3837).
- Byte-exact backup of the pre-install settings: `C:\Users\Bantu\bk-settings-orig-phase10.json` (3837 bytes).
- PATH binary `C:\Users\Bantu\go\bin\beekeeper.exe` rebuilt WITH `--hook` + the fix (`go install` done).
- Catalog present at `%APPDATA%\beekeeper\catalogs\bumblebee.idx`.

**Next actions (in a FRESH Claude Code session after restart):**
1. Canary credential read via the Bash tool — NONEXISTENT path:
   `echo "RAN: $(cat ~/.ssh/id_rsa-beekeeper-canary-DOES-NOT-EXIST 2>&1)"`
   PASS = DENIED (no `RAN:` line); FAIL = you see `RAN: cat: ...: No such file`.
2. (Optional) repeat for `~/.aws/credentials-beekeeper-canary`.
3. Smoking-gun: `%APPDATA%\beekeeper\audit\beekeeper.ndjson` — PreToolUse block with NO
   following PostToolUse `tool_result` for that call = tool did NOT run.
4. Cleanup: EITHER keep it installed (real protection) OR restore byte-exact:
   `beekeeper hooks uninstall --target claude-code` →
   `cp C:\Users\Bantu\bk-settings-orig-phase10.json C:\Users\Bantu\.claude\settings.json` →
   verify `cmp`/`fc` clean → remove `*.beekeeper-backup-*`.
5. Finalize: write `10-02-SUMMARY.md` (record pass/fail + the audit smoking-gun), then
   `/gsd-verify-work 10` to verify + complete the phase. 10-02 is the ONLY plan still open.
