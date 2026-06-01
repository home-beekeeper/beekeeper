---
phase: 5
slug: linux-sentry
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-27
---

# Phase 5 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` package + `go test` |
| **Config file** | None — `go test ./...` discovers automatically |
| **Quick run command** | `go test ./internal/sentry/... ./internal/ipc/... -count=1` |
| **Linux-tagged run** | `go test -tags linux -count=1 ./internal/sentry/... ./internal/ipc/...` (Linux or LVH CI) |
| **Full suite command** | `go test -race -tags linux -count=1 ./...` (CI-only; requires CGO + Linux kernel) |
| **LVH kernel matrix** | `cilium/little-vm-helper@v0.0.21` on `ubuntu-22.04` with image-version `5.4-main` and `5.15-main` |
| **Estimated runtime** | ~15s (quick, no Linux-gated tests), ~60–90s (full LVH, CI) |

---

## Sampling Rate

- **After every task commit:** `go test ./internal/sentry/... ./internal/ipc/... -count=1`
- **After every plan wave:** `go test -tags linux -count=1 ./internal/sentry/... ./internal/ipc/...` on Linux CI
- **Before `/gsd-verify-work`:** Full LVH matrix green for both 5.4-main and 5.15-main
- **Max feedback latency:** ~30 seconds (quick), ~5 min (LVH)

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Req | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-----|------------|-----------------|-----------|-------------------|-------------|--------|
| ipc-proto-encode-decode | 05-01 | 1 | SLNX-07 | — | Length-prefixed JSON round-trips correctly; truncated message returns error | unit | `go test ./internal/ipc/... -run "TestEncode\|TestDecode\|TestFraming" -v -count=1` | ❌ W0 | ⬜ pending |
| ipc-server-peercred | 05-01 | 1 | SLNX-07 | T-05-01-02 | SO_PEERCRED rejects wrong UID; accepts correct UID | unit | `go test -tags linux ./internal/ipc/... -run "TestPeerCred\|TestServerAccept" -v -count=1` | ❌ W0 | ⬜ pending |
| ipc-fuzz-gate | 05-01 | 1 | SLNX-07 | T-05-01-01 | FuzzIPCMessage does not panic on arbitrary input | fuzz (release gate) | `go test -tags linux -fuzz=FuzzIPCMessage -fuzztime=60s ./internal/ipc/...` | ❌ W0 | ⬜ pending |
| sentry-types-roundtrip | 05-02 | 1 | SLNX-06 | — | SentryEvent fields preserved through channel boundary | unit | `go test ./internal/sentry/... -run TestSentryEventRoundTrip -v -count=1` | ❌ W0 | ⬜ pending |
| correlation-rules-all5 | 05-02 | 1 | SLNX-08 | — | Crafted event sequence fires each of SENTRY-001 through SENTRY-005 | unit | `go test ./internal/sentry/... -run "TestSENTRY001\|TestSENTRY002\|TestSENTRY003\|TestSENTRY004\|TestSENTRY005" -v -count=1` | ❌ W0 | ⬜ pending |
| baseline-mode | 05-02 | 1 | SLNX-09 | T-05-02-01 | IsBaselineActive returns true for 7 days; false after; false when duration=0 | unit | `go test ./internal/sentry/... -run "TestBaselineMode\|TestIsBaselineActive\|TestLoadSaveBaseline" -v -count=1` | ❌ W0 | ⬜ pending |
| ebpf-c-sources | 05-03 | 2 | SLNX-02, SLNX-04 | T-05-03-03 | C sources contain correct program sections; go.mod has cilium/ebpf v0.21.0 | build | `go build ./... 2>&1 \| head -20` | ❌ W0 | ⬜ pending |
| probe-tier | 05-03 | 2 | SLNX-05 | — | ProbeTier returns one of Tier0/Tier1/Tier2 without panic; TierString non-empty | unit | `go test -tags linux ./internal/sentry/linux/... -run "TestProbeTier\|TestTierString" -v -count=1` | ❌ W0 | ⬜ pending |
| ebpf-binary-parse | 05-03 | 2 | SLNX-02, SLNX-04 | T-05-03-01 | ProcessEvent and NetworkEvent binary struct layout round-trips correctly | unit | `go test -tags linux ./internal/sentry/linux/... -run "TestParseProcessEvent\|TestParseNetworkEvent\|TestDropCounterIncrements" -v -count=1` | ❌ W0 | ⬜ pending |
| fanotify-init-fallback | 05-04 | 2 | SLNX-03 | T-05-04-02 | InitFanotify falls back from FAN_REPORT_FID to base flags on EINVAL; skips missing paths | unit | `go test -tags linux ./internal/sentry/linux/... -run "TestInitFanotify\|TestFanotifyMarkPaths" -v -count=1` | ❌ W0 | ⬜ pending |
| privilege-drop | 05-04 | 2 | SLNX-10 | T-05-04-04 | DropCapabilities [2]unix.CapUserData pattern compiles; ApplySeccomp uses FilterFlagTSync | build + grep | `go build -tags linux ./internal/sentry/linux/... 2>&1 \| head -10 && grep "\\[2\\]unix.CapUserData" internal/sentry/linux/privilege.go && grep "FilterFlagTSync" internal/sentry/linux/privilege.go` | ❌ W0 | ⬜ pending |
| daemon-startup-wiring | 05-05 | 3 | SLNX-01, SLNX-09, SLNX-10 | T-05-05-05 | alertToAuditRecord emits sentry_alert_baseline during baseline; unit file rendered with correct content | unit | `go test -tags linux ./internal/sentry/linux/... -run "TestAlertToAuditRecord\|TestCorrelationEngineLoopBaseline\|TestWriteUnitFile" -v -count=1` | ❌ W0 | ⬜ pending |
| cli-protect-sentry | 05-05 | 3 | SLNX-01, SLNX-07 | T-05-05-01 | protect/sentry commands compile; runtime.GOOS guard present; LVH jobs in ci.yml | build + grep | `go build ./... 2>&1 \| head -20 && grep "little-vm-helper" .github/workflows/ci.yml && grep "5.4-main" .github/workflows/ci.yml && grep "5.15-main" .github/workflows/ci.yml` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## LVH Integration Tests (CI-only — kernel matrix required)

These tests require a real Linux kernel and cannot run on Windows dev machine. They run in the LVH CI matrix jobs `test-sentry-kernel-5-4` and `test-sentry-kernel-5-15`.

| Req ID | Behavior | Kernel | Automated Command |
|--------|----------|--------|-------------------|
| SLNX-02 | eBPF exec tracepoint fires on process exec; pid/ppid/exe correct | 5.4 + 5.15 | `go test -v -tags linux -count=1 ./internal/sentry/linux/... -run TestExecTracer` |
| SLNX-03 | fanotify fires on sensitive path read; FAN_ALLOW sent | 5.4 + 5.15 | `go test -v -tags linux -count=1 ./internal/sentry/linux/... -run TestFanotifyIntegration` |
| SLNX-04 | eBPF tcp_connect kprobe fires; dst addr/port correct | 5.4 + 5.15 | `go test -v -tags linux -count=1 ./internal/sentry/linux/... -run TestNetTracer` |
| SLNX-05 | Tier2 on 5.4-main (no ring buffer); Tier0 on 5.15-main | 5.4 → Tier2; 5.15 → Tier0 | `go test -v -tags linux -count=1 ./internal/sentry/linux/... -run TestDegradationTier` |
| SLNX-10 | DropCapabilities leaves only expected caps post-eBPF-load | 5.4 + 5.15 | `go test -v -tags linux -count=1 ./internal/sentry/linux/... -run TestCapabilityDrop` |

---

## Wave 0 Requirements

Files that must be created before or during Wave 1 execution (tests can be stubs initially):

- [ ] `internal/ipc/proto_test.go` — TestEncodeDecodeRoundTrip, TestFramingTruncated, TestFramingOversize
- [ ] `internal/ipc/proto_fuzz_test.go` — FuzzIPCMessage (RELEASE GATE for v0.6.0)
- [ ] `internal/ipc/server_test.go` — TestPeerCredRejection, TestServerAcceptCorrectUID
- [ ] `internal/sentry/types_test.go` — TestSentryEventRoundTrip
- [ ] `internal/sentry/rules_test.go` — TestSENTRY001 through TestSENTRY005, TestCorrelationWindowExpiry
- [ ] `internal/sentry/baseline_test.go` — TestIsBaselineActive, TestBaselineActiveFor7Days, TestBaselineExpiredAfter7Days, TestBaselineImmediateWhenDuration0, TestLoadSaveBaseline
- [ ] `internal/sentry/linux/probe_test.go` — TestProbeTierReturnsDegradationTier, TestTierString
- [ ] `internal/sentry/linux/ebpf_test.go` — TestParseProcessEvent, TestParseNetworkEvent, TestDropCounterIncrements
- [ ] `internal/sentry/linux/fanotify_test.go` — TestInitFanotifyFallback, TestFanotifyMarkPathsSkipsMissing
- [ ] `internal/sentry/linux/systemd_test.go` — TestWriteUnitFile (unit file rendered content), TestIsSystemdRunningReturnsValue
- [ ] `internal/sentry/linux/daemon_test.go` — TestAlertToAuditRecord, TestCorrelationEngineLoopBaseline

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `beekeeper protect install` writes unit file and enables daemon | SLNX-01 | Requires root + systemd host | On Ubuntu 22.04: `sudo beekeeper protect install`; verify `systemctl status beekeeper-sentry` shows active |
| Sentry daemon detects Nx Console-class credential cluster | SLNX-08 | Requires root + eBPF privileges + editor process | Start daemon; spawn a cursor subprocess that reads `~/.ssh/id_rsa` and `~/.aws/credentials`; verify sentry_alert NDJSON record emitted within 60s |
| `beekeeper protect status` shows correct tier on Tier 1 kernel | SLNX-05 | Requires kernel 5.4–5.14 host | On LVH 5.4-main: start daemon; run `beekeeper protect status`; verify "Tier: Degraded (Tier 1)" in output |
| IPC SO_PEERCRED rejects different-user CLI connection | SLNX-07 | Requires multi-user Linux environment | Run daemon as user A; run `beekeeper protect status` as user B; verify connection refused with UID mismatch |
| fanotify FAN_ALLOW response unblocks accessing process | SLNX-03 | Requires root + real process | Mark `~/.ssh/id_rsa`; `cat ~/.ssh/id_rsa`; verify no hang (FAN_ALLOW sent) and sentry event emitted |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s (quick), < 5 min (LVH)
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
