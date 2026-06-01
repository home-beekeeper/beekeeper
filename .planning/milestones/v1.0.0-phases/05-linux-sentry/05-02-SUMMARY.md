---
plan: 05-02
status: complete
wave: 1
---
# 05-02 Summary: Sentry Correlation Engine

## Artifacts
- internal/sentry/types.go — SentryEvent, ProcessNode, SentryAlert, RuleState, RuleConfig, InventorySnapshot
- internal/sentry/baseline.go — BaselineState, IsBaselineActive, LoadBaseline, SaveBaseline
- internal/sentry/rules.go — EvaluateEvent with all 5 correlation rules (SENTRY-001 through SENTRY-005)
- internal/sentry/types_test.go, baseline_test.go, rules_test.go — all 19 tests pass on Windows
- internal/audit/types.go — extended with 11 sentry_alert fields

## Verification
- All 19 sentry tests green (go test ./internal/sentry/... -v -count=1)
- All audit tests green (go test ./internal/audit/... -v -count=1)
- go build ./... passes
- go vet ./internal/sentry/... ./internal/audit/... passes
- All 5 rules tested with trigger + non-trigger sequences

## Notes
- No build tags on any file — compiles on Windows, Linux, macOS
- SENTRY-003 fires on the first qualifying connection per PhoneHomeWindowMin window (count==1 post-append after expiry), matching the spec
- SENTRY-004 and SENTRY-005 both use the ExfilFusionWindowMin / FreshExtWindowMin cutoffs as half-open intervals (installTime >= cutoff) consistent with the spec
- expireWindow allocates a fresh zero-cap slice to avoid aliasing the original backing array
- isEditorDescendant walks up to 32 PPID hops, stopping at PID 0 or a self-referential cycle
