---
phase: 02-policy-engine-multi-source-catalogs
plan: "03"
subsystem: policy
tags: [policy-engine, egress, exfil, baseline, pure-functions, tdd, shannon-entropy]
dependency_graph:
  requires: []
  provides:
    - internal/policy/egress.go — EvaluateEgress pure network-egress policy
    - internal/policy/exfil.go — EvaluateExfil pure entropy/base64 detection
    - internal/policy/baseline.go — EvaluateBaseline pure frequency-deviation + BaselineCounters type
  affects:
    - internal/baseline/store.go (future Wave 2) — will marshal/unmarshal BaselineCounters
    - internal/check/handler.go — can now call all three new policy functions
tech_stack:
  added: []
  patterns:
    - TDD RED/GREEN with failing tests committed before implementation
    - Caller-supplied nowUnix pattern (no time.Now() in pure layer)
    - Historical-only baseline statistics (current day excluded from mean/stddev)
    - Byte-frequency Shannon entropy over concatenated window
key_files:
  created:
    - internal/policy/egress.go
    - internal/policy/egress_test.go
    - internal/policy/exfil.go
    - internal/policy/exfil_test.go
    - internal/policy/baseline.go
    - internal/policy/baseline_test.go
  modified: []
decisions:
  - "Historical-only baseline: mean and stddev computed from days other than today, so today's spike does not self-dampen the threshold (prevents missed detections when history is uniform)"
  - "Uniform-history edge case (stddev==0): any count above mean triggers warn — a perfectly constant pattern with any deviation is anomalous"
  - "EvaluateEgress decision order: size limit first, then deny list, then allow list, then warn — size check before host check prevents oversized payloads to technically-allowed hosts"
  - "extractHost uses pure string ops (strings.HasPrefix, strings.Index) — no net/url import, preserving the forbidden-import contract"
  - "Base64Threshold check before entropy check in EvaluateExfil — base64 accumulation is a more direct indicator and should surface clearly in the reason"
metrics:
  duration: "~25 minutes"
  completed: "2026-05-26"
  tasks_completed: 3
  files_created: 6
---

# Phase 2 Plan 03: Network Egress + Exfil Detection + Baseline Engine Summary

Three pure policy functions added to `internal/policy`: network egress allowlist + size limits (PLCY-05), multi-turn exfiltration detection via Shannon entropy + base64 accumulation (PLCY-06), and the behavioral baseline frequency-deviation engine (PLCY-07). All files are pure — no forbidden imports, all tests pass including import purity checks.

## Function Signatures

### EvaluateEgress (egress.go)

```go
func EvaluateEgress(input EgressInput, cfg EgressConfig) Decision

type EgressInput struct {
    ToolName    string
    TargetURL   string // full URL; host extracted by pure string ops
    PayloadSize int64  // bytes
}

type EgressConfig struct {
    AllowHosts      []string
    DenyHosts       []string
    MaxPayloadBytes int64
    PerToolMaxBytes map[string]int64
}

func DefaultEgressConfig() EgressConfig
```

Decision order: (1) size limit check → block; (2) deny suffix match → block; (3) allow suffix match → allow; (4) unknown host → warn (non-blocking). Unknown hosts warn, never silently allow (T-02-03-01 mitigation).

Default config: 10 allow hosts (npm/PyPI/crates.io/rubygems/Go module registries + docs.anthropic.com); 6 deny hosts (pastebin/hastebin/ghostbin/webhook.site/requestbin/ngrok.io); 10MB default payload limit.

### EvaluateExfil (exfil.go)

```go
func EvaluateExfil(window ExfilWindow, cfg ExfilConfig) Decision

type ExfilWindow struct {
    Outputs     []string // last N tool outputs (pre-collected by caller)
    Base64Bytes int64    // accumulated base64 byte count across turns
}

type ExfilConfig struct {
    EntropyThreshold float64 // default 4.5 bits/byte
    Base64Threshold  int64   // default 1MB
}

func DefaultExfilConfig() ExfilConfig
func shannonEntropy(s string) float64 // unexported; package-internal
```

`shannonEntropy` uses `[256]int` byte frequency table and `math.Log2`. Handles empty string (returns 0). Verified: `shannonEntropy("aaaa") == 0`, `shannonEntropy("abcd") == 2.0`. Base64 accumulation checked before entropy — both are warn-level (Allow stays true), implementing T-02-03-02 mitigation.

### EvaluateBaseline (baseline.go)

```go
func EvaluateBaseline(key string, nowUnix int64, counters BaselineCounters, cfg BaselineConfig) Decision

type BaselineCounters struct {
    Counts     map[string][]int64 `json:"counts"`      // keyed by "tool::target"
    WindowDays int                `json:"window_days"` // default 7 if zero
}

type BaselineConfig struct {
    DeviationSigma float64 // default 3.0 — sigma multiplier for detection threshold
}

func DefaultBaselineConfig() BaselineConfig
```

`BaselineCounters` is the exported on-disk contract for the persistence layer (future `internal/baseline/store.go`). It round-trips cleanly through `encoding/json` with correct field names.

**Statistical approach:** `nowUnix` is caller-supplied (no `time.Now()` — T-02-03-03 mitigation). Mean and stddev are computed from **historical days only** (days where `day != nowUnix/86400`). Today's count is then compared to `mean + sigma*stddev`. When history is perfectly uniform (stddev==0), any count above mean is flagged — preventing missed detections from self-dampening.

**Allow conditions:** empty counters; all timestamps outside window; fewer than 2 historical days (insufficient data for stddev); current count within threshold.

## BaselineCounters JSON Contract

```json
{
  "counts": {
    "bash::npm install": [1748217600, 1748131200, 1748044800],
    "webfetch::github.com": [1748131200]
  },
  "window_days": 7
}
```

This is the schema the `internal/baseline` persistence layer (Wave 2) will read and write. The `encoding/json` round-trip test verifies field names and value preservation.

## TDD Gate Compliance

All three tasks followed the mandatory RED/GREEN cycle:

| Task | RED commit | GREEN commit |
|------|-----------|-------------|
| EvaluateEgress | `1b4114f` | `e124f67` |
| EvaluateExfil | `62dd658` | `1b2f8c9` |
| EvaluateBaseline | `418c22c` | `05f6348` |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Baseline spike detection self-dampening when history is uniform**
- **Found during:** Task 3 GREEN phase (test `TestBaselineFrequencySpikeWarns` failed)
- **Issue:** Initial implementation computed mean and stddev including today's current count. With 6 historical days at 1/day and today at 20, the combined mean was 3.71 and combined stddev was 6.65, giving threshold 23.66 — higher than the actual spike of 20. The spike was not detected.
- **Fix:** Split days into historical (all days except today) and current (today). Compute mean+stddev from historical only, then compare today's count against that baseline. When historical stddev==0 (perfectly uniform), flag any count above mean.
- **Files modified:** `internal/policy/baseline.go`
- **Commit:** `05f6348` (same GREEN commit; fix applied before commit)

## Known Stubs

None — all three functions are fully implemented with real logic and data.

## Threat Flags

No new threat surface beyond what the plan's threat model already covers. All three functions are pure (no network endpoints, no file I/O, no auth paths, no schema at trust boundaries).

## Self-Check

Verified:
- `internal/policy/egress.go` — exists ✓
- `internal/policy/egress_test.go` — exists ✓
- `internal/policy/exfil.go` — exists ✓
- `internal/policy/exfil_test.go` — exists ✓
- `internal/policy/baseline.go` — exists ✓
- `internal/policy/baseline_test.go` — exists ✓

Commits verified:
- `1b4114f` — test RED egress ✓
- `e124f67` — feat GREEN egress ✓
- `62dd658` — test RED exfil ✓
- `1b2f8c9` — feat GREEN exfil ✓
- `418c22c` — test RED baseline ✓
- `05f6348` — feat GREEN baseline ✓

`go test ./internal/policy/... -count=1` — PASS (27 tests across 3 new files + existing engine tests) ✓
`go vet ./internal/policy/...` — clean ✓
All 3 purity tests (`TestEgressImportsArePure`, `TestExfilImportsArePure`, `TestBaselineImportsArePure`) — PASS ✓

## Self-Check: PASSED
