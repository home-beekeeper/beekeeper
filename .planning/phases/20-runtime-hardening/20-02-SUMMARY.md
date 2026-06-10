---
phase: 20-runtime-hardening
plan: 02
subsystem: llamafirewall
tags: [llamafirewall, sidecar, ipc, tcp, token, codeshield, promptguard, venv, gated-model, e2e, fail-closed]

requires:
  - phase: 20-runtime-hardening
    provides: "20-01 config plumbing (shared config.go/layered.go/main.go); embedded sidecar + InstallSidecar (Task 1)"
provides:
  - Single loopback-TCP + per-launch bearer-token IPC (unix + named-pipe fork deleted)
  - TCP dial-retry readiness; HF_HOME + BEEKEEPER_LLMF_PORT/TOKEN injected via cmd.Env
  - platform.StateDir()/ConfigPath() for llamafirewall enable/disable/status (Windows StateDir bug fixed)
  - audit-record reads port+token from state.json and dials the running sidecar
  - `beekeeper llamafirewall install` (venv + CPU torch index deps + gated 22M model pre-pull)
  - supervisor venv-interpreter auto-detect
  - //go:build e2e real-sidecar test + TestSupervisorRestartOnCrash
  - honest home/docs claims (AlignmentCheck removed; opt-in/experimental/gated/local-no-cloud)
affects: [20-05]

tech-stack:
  added: []
  patterns:
    - "One loopback-TCP transport on every OS replaces the unix-socket/named-pipe build-tag fork; access control restored by a per-launch crypto/rand bearer token the sidecar checks per request (D-T2-ipc, T-20-06)"
    - "Readiness = TCP dial-retry (a successful dial is the only true readiness signal), replacing the meaningless os.Stat(socketFile) probe"
    - "Sidecar endpoint (port+token) persisted to state.json so one-shot commands reach the long-lived gateway sidecar"

key-files:
  created:
    - internal/llamafirewall/client_token_test.go
    - internal/llamafirewall/e2e_test.go
  modified:
    - internal/llamafirewall/client.go
    - internal/llamafirewall/supervisor.go
    - internal/llamafirewall/supervisor_test.go
    - internal/llamafirewall/supervisor_failclosed_test.go
    - internal/llamafirewall/sidecar_assets.go
    - cmd/beekeeper/main.go
    - web/components/home/feature-cards.tsx
    - web/components/home/how-it-works.tsx
    - web/content/docs/security.mdx
    - web/content/docs/cli-reference.mdx
  deleted:
    - internal/llamafirewall/client_windows.go

key-decisions:
  - "IPC collapsed to one net.DialTimeout(\"tcp\", 127.0.0.1:port) transport; client_windows.go (named pipe) deleted. The per-launch bearer token (crypto/rand, 256-bit hex) restores the access control the old 0600 unix socket gave; the sidecar rejects a mismatch (proven by cross-platform client_token_test.go, which also gives Windows the coverage the linux-only unix tests never did)."
  - "Supervisor picks a free 127.0.0.1 port + token at Start, reused across relaunches; injects BEEKEEPER_LLMF_PORT/TOKEN + a pinned HF_HOME=<stateDir>/llamafirewall/hf via cmd.Env (Start and relaunch); persistState records port+token. NewSupervisor dropped the sockPath arg (2-arg)."
  - "main.go enable/disable/status now resolve via platform.ConfigPath()/StateDir() (helpers llamafirewallConfigPath/StatePath) instead of os.ExpandEnv(\"$HOME\")/.beekeeper — the Windows StateDir bug. audit-record reads port+token from state.json (readLlamafirewallEndpoint) and dials the running sidecar, fail-closed when the endpoint is unknown."
  - "install logic lives inline in the cmd handler (the AC greps main.go for venv/HF_HOME/requirements.txt/CPU-index/22M model); venv/model path helpers (VenvDir/VenvPython/HFHome/DefaultPromptGuardModel) live in the package and are reused by the supervisor's venv auto-detect."
  - "requirements.txt pins the REAL Meta `llamafirewall==1.0.3` (verified in Task 1), not a `llama-firewall` typosquat (RESEARCH open-q / T-20-SC)."

patterns-established:
  - "Cross-platform token-checking mock sidecar (Go) to prove client sends + server rejects the bearer token without spawning Python"
  - "Double-gated e2e: //go:build e2e AND BEEKEEPER_LLMF_E2E=1 (t.Skip otherwise) for a CI-Linux-only real-sidecar gate that needs the human-accepted gated model"

requirements-completed: [LLMF-01, LLMF-02, LLMF-03, LLMF-04, LLMF-05, LLMF-06]

duration: ~95 min
completed: 2026-06-10
human_gate_pending: true
---

# Phase 20 Plan 02: LlamaFirewall — Real, Fail-Closed, Cloud-Free, Cross-OS (LLMF) Summary

**The opt-in prompt-injection sidecar is now real and safe: Task 1 fixed the silent fail-open (scanners built once, correct `UserMessage`, real CodeShield, fail-closed on error) and removed the AlignmentCheck cloud-exfil path; Tasks 2–4 collapse IPC to one loopback-TCP+token transport on every OS, fix the Windows StateDir bug, add the `llamafirewall install` venv/gated-model bootstrap, add the gated real-sidecar e2e + restart-on-crash tests, and make the home/docs claims honest. Only the human HF-license live-bootstrap verification (Task 3 checkpoint) remains.**

## Performance

- **Duration:** ~95 min (Tasks 2–4 this session; Task 1 in a prior session, commit `d306f19`)
- **Tasks:** 4 (Task 3's human verification is pending)
- **Files:** 10 modified + 2 created + 1 deleted

## Accomplishments

**Task 1 (prior session, `d306f19`):** moved sidecar → `internal/llamafirewall/assets/` + `//go:embed` + `InstallSidecar`; FIXED the silent fail-open (scanners built ONCE for PROMPT_GUARD/USER + CODE_SHIELD/ASSISTANT, `lf.scan(UserMessage(content=...))`, real CodeShield, ANY exception → `error` sentinel that the Go layer blocks on); REMOVED AlignmentCheck (Together AI cloud) everywhere; `requirements.txt` pins real `llamafirewall==1.0.3`.

**Task 2 (`b116c5b`):** collapsed `client.go` + `client_windows.go` → one `net.DialTimeout("tcp", ...)` `Dial(addr, token, timeout)` (every Scan stamps the token); deleted the named-pipe client. Supervisor picks a free loopback port + crypto/rand bearer token at Start (reused on relaunch), injects `BEEKEEPER_LLMF_PORT/TOKEN` + pinned `HF_HOME` via `cmd.Env`, replaces `os.Stat(socket)` with a TCP dial-retry readiness loop, persists port+token to state.json; `NewSupervisor` is 2-arg. `main.go` enable/disable/status use platform paths (Windows StateDir bug) and audit-record dials the persisted endpoint. Reworked the linux unix-socket tests to TCP + new cross-platform `client_token_test.go` (wrong-token rejected).

**Task 4 (`923c4de`):** `e2e_test.go` (`//go:build e2e`, gated `BEEKEEPER_LLMF_E2E=1`) asserting benign→clean / injection→injection / unsafe-code→unsafe / crash→fail-closed against the real sidecar (CI-Linux only); `TestSupervisorRestartOnCrash` (default suite, uses `startCrashingSidecar`) proving crash-during-scan surfaces an error and the relaunch budget trips degraded mode.

**Task 3 code automation (`f36546d`):** `beekeeper llamafirewall install` (CPU-only venv under StateDir + pinned deps via the CPU torch index + gated 22M model pre-pull into HF_HOME, surfacing the Llama-license + huggingface-cli login requirement); supervisor venv-interpreter auto-detect; home + docs claims made honest (AlignmentCheck removed, opt-in/experimental/gated/local-no-cloud, install command documented).

## Task Commits

1. **Task 1: embed+install; fix silent fail-open; real CodeShield; remove AlignmentCheck** - `d306f19` (feat, prior session)
2. **Task 2: loopback-TCP + per-launch token IPC; TCP readiness; HF_HOME + StateDir fix** - `b116c5b` (feat)
3. **Task 4: gated real-sidecar e2e + restart-on-crash test** - `923c4de` (test)
4. **Task 3 (code): llamafirewall install venv/CPU deps/gated model + honest web claims** - `f36546d` (feat)

## Deviations from Plan

### Auto-fixed / scoped decisions

**1. install logic inline in the cmd handler (not extracted to internal/)**
- The Task-3 acceptance criteria grep `cmd/beekeeper/main.go` for `venv|HF_HOME|requirements.txt`, the CPU torch index, and the 22M model default, so the orchestration lives inline in the install handler (consistent with the existing non-trivial status/audit-record handlers). Reusable path helpers (`VenvDir/VenvPython/HFHome/DefaultPromptGuardModel`) live in the package so the supervisor's venv auto-detect shares them.

**2. security.mdx had no prior LlamaFirewall section**
- The plan's read_first assumed security.mdx already stated the LlamaFirewall posture; it did not. Added a new "LlamaFirewall prompt-injection scan (opt-in, experimental)" subsection (covers both Task 3 softening and Task 4 e2e/gated posture accuracy). Also fixed the AlignmentCheck claim in BOTH home components (feature-cards.tsx and how-it-works.tsx) and added the `install` command to cli-reference.mdx.

**3. TestSupervisorRestartOnCrash + e2e are CI-Linux validated, not local**
- `supervisor_test.go` is `//go:build linux`; on the Windows dev machine its run is confirmed by `GOOS=linux go vet` (type-check) + the CI Linux job, mirroring the v1.2.0 `-race` CI-only precedent. The e2e file compiles under `-tags e2e` on Windows and `t.Skip`s without `BEEKEEPER_LLMF_E2E=1`.

## Issues Encountered / Carried Forward

- **BLOCKING HUMAN GATE (Task 3 verification):** accepting the `meta-llama/Llama-Prompt-Guard-2-22M` license on Hugging Face + `huggingface-cli login` is a human-only web action. The live `beekeeper llamafirewall install` bootstrap, StateDir status check, and served home/docs review remain for the maintainer. The real-sidecar e2e cases also depend on that accepted model (CI-Linux job). All CODE automation around the gate is complete.

## Verification

- `go build ./...` + `GOOS=windows/darwin/linux go build ./...` all exit 0.
- `go test ./internal/llamafirewall/ ./internal/config/ ./internal/check/` green; `go vet` native + `GOOS=linux` green.
- `go build -tags e2e ./internal/llamafirewall/` exit 0; e2e compiles + skips on Windows.
- AC source greps: `DialPipe|winio`=0, `net.DialTimeout("tcp"` in client.go, `BEEKEEPER_LLMF_TOKEN`+`HF_HOME` in supervisor.go, `os.ExpandEnv("$HOME")`=0 in main.go, `AF_UNIX`(non-comment)=0 / `AF_INET`>=1 in the sidecar; install handler greps for venv/HF_HOME/requirements.txt/CPU-index/22M model.
- web `pnpm build` exit 0; `accuracy_spec` (no phantom commands) + `seo_spec` + `home_spec` + `gfx_spec` all PASS.

## Next Phase Readiness

- 20-05 (Tier-3 W3 honesty + synthetic tests) is unblocked (20-03 + 20-04 done) and independent of 20-02. NOTE: `feature-cards.tsx` is touched by both this plan (LlamaFirewall claim) and 20-05 (Sentry claim) — already composed (Sentry card left for 20-05).
- 20-02 closes once the maintainer completes the HF-license live bootstrap verification.

---
*Phase: 20-runtime-hardening*
*Completed (code): 2026-06-10 — human verification pending*
