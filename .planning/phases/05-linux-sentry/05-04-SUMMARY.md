---
plan: 05-04
status: complete
wave: 2
---
# 05-04 Summary: fanotify + Privilege Separation

## Artifacts
- internal/sentry/linux/probe.go — DegradationTier type (Tier0/Tier1/Tier2), ProbeTier(), TierString(), probeTier() with two-struct Capget
- internal/sentry/linux/fanotify.go — InitFanotify, FanotifyMarkPaths, StartFanotifyReader (always writes FAN_ALLOW, closes fd immediately)
- internal/sentry/linux/fanotify_test.go — smoke tests (TestInitFanotifyFallback, TestFanotifyMarkPathsSkipsMissing)
- internal/sentry/linux/privilege.go — DropCapabilities (two-struct Capget), ApplySeccomp (elastic/go-seccomp-bpf + FilterFlagTSync), keepCaps, sdNotifyReady

## Verification
- go build ./... passes (linux-tagged files excluded from Windows build)
- go vet ./internal/sentry/... passes
- [2]unix.CapUserData confirmed in privilege.go (line 25)
- FilterFlagTSync confirmed in privilege.go (line 76)
- unix.O_RDWR confirmed in fanotify.go (lines 26, 32, 35)
- unix.Close(int(meta.Fd)) confirmed in fanotify.go (line 132)
- FAN_ALLOW confirmed in fanotify.go (line 140)

## Notes
- probe.go was also created in this plan (not 05-03 as originally scoped) since the linux/ subdirectory did not exist yet; DegradationTier is defined here as planned
- elastic/go-seccomp-bpf is v1.6.0 in go.mod (not v1.0.2 from plan spec); API is compatible — Filter, FilterFlagTSync, ActionAllow, ActionKillProcess, LoadFilter all exist in v1.6.0
- The package import alias `seccomp "github.com/elastic/go-seccomp-bpf"` is used in privilege.go to match the plan's `seccomp.Filter{}` usage pattern
