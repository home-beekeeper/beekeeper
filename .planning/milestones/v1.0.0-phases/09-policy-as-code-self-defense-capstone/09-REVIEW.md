---
phase: 09-policy-as-code-self-defense-capstone
reviewed: 2026-05-29T00:00:00Z
depth: standard
files_reviewed: 17
files_reviewed_list:
  - cmd/beekeeper/policy.go
  - cmd/beekeeper/diag.go
  - cmd/beekeeper/selfquarantine.go
  - cmd/beekeeper/main.go
  - internal/policyloader/loader.go
  - internal/policyloader/validate.go
  - internal/policyloader/test.go
  - internal/config/layered.go
  - internal/config/config.go
  - internal/catalog/selfcatalog.go
  - internal/catalog/selfkey.go
  - internal/catalog/state.go
  - internal/check/diag.go
  - internal/check/diag_windows.go
  - internal/check/diag_other.go
  - internal/check/latency_persist.go
  - internal/check/handler.go
  - internal/llamafirewall/latency.go
  - docs/THREAT-MODEL.md
findings:
  critical: 3
  warning: 4
  info: 2
  total: 9
status: issues_found
---

# Phase 9: Code Review Report

**Reviewed:** 2026-05-29T00:00:00Z
**Depth:** standard
**Files Reviewed:** 17
**Status:** issues_found

## Summary

This review covers the Phase 9 policy-as-code and self-defense capstone:
`beekeeper-self` self-quarantine, the policy loader, the startup guard, and
the layered config merge. The fundamental architecture is sound — the
fail-closed vs. warn-continue distinction is correctly implemented, signature
verification is correctly wired, and the policy loader's `DisallowUnknownFields`
guard is genuine and covers nested structs. Three blockers were found, all in
wiring code rather than algorithmic logic.

---

## Critical Issues

### CR-01: `SelfCatalog.PubKey` user override is wired into config but never passed to `SelfCatalogOpts` — self-hosted feed feature is broken

**File:** `cmd/beekeeper/selfquarantine.go:90-103`

**Issue:** `enforceSelfQuarantine` reads `cfg.SelfCatalog.URL` from the merged
config and passes it to `SelfCatalogOpts.FeedURL`. However it never reads
`cfg.SelfCatalog.PubKey` and never sets `SelfCatalogOpts.pubKeyOverride`. The
`SelfCatalogOpts.pubKeyOverride` field stays `nil`, causing `CheckSelfCatalog`
to unconditionally use the compiled-in `SelfCatalogPublicKey` regardless of
what the user has configured.

The threat model (Section 6, "Governance Honesty Note") explicitly documents
the self-hosted feed override as a viable escape hatch for users who cannot
accept single-maintainer trust:

```json
{
  "self_catalog": {
    "url": "https://your-mirror.example.com/beekeeper-self.json",
    "pub_key": "<your-base64-ed25519-public-key>"
  }
}
```

With this bug, the `pub_key` override is silently ignored and verification
always uses the compiled-in key. A self-hosted feed signed with the user's own
key will produce an integrity failure (`SelfCatalogFailClosed`), blocking ALL
enforcement commands. The user-facing symptom is: "I configured my own feed and
pub_key per the docs, but beekeeper refuses to run with an integrity failure."
This is a functional security feature that is non-operational.

**Fix:**
```go
// cmd/beekeeper/selfquarantine.go
cfg, cfgErr := resolveConfig(cmd)
feedURL := ""
var pubKeyOverride ed25519.PublicKey
if cfgErr == nil {
    feedURL = cfg.SelfCatalog.URL
    if cfg.SelfCatalog.PubKey != "" {
        raw, err := base64.StdEncoding.DecodeString(cfg.SelfCatalog.PubKey)
        if err == nil && len(raw) == ed25519.PublicKeySize {
            pubKeyOverride = ed25519.PublicKey(raw)
        } else {
            return fmt.Errorf("enforce self-quarantine: self_catalog.pub_key is invalid: must be base64-encoded 32-byte Ed25519 key")
        }
    }
}

opts := catalog.SelfCatalogOpts{
    FeedURL:        feedURL,
    CacheDir:       catalogDir,
    StatePath:      filepath.Join(stateDir, "state.json"),
    Version:        version.Version,
    Client:         &http.Client{Timeout: 10 * time.Second},
    PubKeyOverride: pubKeyOverride, // nil = use compiled-in key
}
```

`SelfCatalogOpts.pubKeyOverride` must be exported (renamed `PubKeyOverride`)
or a `WithPubKey` constructor option must be used to allow the caller to set it
without exposing the test-injection seam directly.

---

### CR-02: `resolveConfig` never passes `os.Environ()` — the entire BEEKEEPER_* env-var config layer is silently dead in production

**File:** `cmd/beekeeper/diag.go:104-106`

**Issue:** `resolveConfig` builds `config.LayerOpts` with only `UserPath` set:

```go
opts := config.LayerOpts{
    UserPath: configPath,
}
```

`LayerOpts.Environ` is left `nil`. In `LoadLayered`, Layer 4 calls
`applyEnvVars(cfg, opts.Environ)` — with a nil slice this iterates over
nothing and applies no env vars. `BEEKEEPER_FAIL_MODE`,
`BEEKEEPER_SOCKET_API_TOKEN`, `BEEKEEPER_LLAMAFIREWALL_ENABLED`,
`BEEKEEPER_AUDIT_SINKS`, and `BEEKEEPER_SELF_CATALOG_URL` are all
documented and tested (there is a `TestLoadLayered_EnvSelfCatalogURL` test),
but none of them take effect in production because `resolveConfig` — the only
production call site for `LoadLayered` — never passes the environment.

This affects `enforceSelfQuarantine` (which calls `resolveConfig`): a user who
sets `BEEKEEPER_SELF_CATALOG_URL` in their environment expecting it to override
the feed URL will find that the env var is ignored. The system config layer
(`SystemPath`) and project config layer (`ProjectPath`) are also never applied
— `resolveConfig` loads only the user config.

The BEEKEEPER_* env-var layer was explicitly required by the architecture
(T-09-05, CODE-05). Its silent non-application means the documented behavior
does not match the implementation.

**Fix:**
```go
func resolveConfig(cmd *cobra.Command) (config.Config, error) {
    configPath, err := platform.ConfigPath()
    if err != nil {
        return config.Config{}, fmt.Errorf("resolve config path: %w", err)
    }

    opts := config.LayerOpts{
        UserPath: configPath,
        Environ:  os.Environ(), // apply BEEKEEPER_* env vars (T-09-05)
    }

    cfg, err := config.LoadLayered(opts)
    if err != nil {
        return config.Config{}, fmt.Errorf("load layered config: %w", err)
    }
    return cfg, nil
}
```

---

### CR-03: Unguarded type assertions in `llamafirewall status` will panic on malformed state.json

**File:** `cmd/beekeeper/main.go:1383-1384`

**Issue:** The `llamafirewall status` command performs two bare type assertions
on data read from disk without the comma-ok form:

```go
pid := int(lfState["pid"].(float64))
startedAt := lfState["started_at"].(string)
```

If `lfState["pid"]` is absent, null, or an integer (Go's `encoding/json` only
uses `float64` for numbers when unmarshaling into `any`, but a future schema
change could break this assumption), the assertion panics. Same for
`lfState["started_at"]` being absent or null. Cobra does not recover panics in
`RunE` handlers — the process crashes with a stack trace rather than printing
"Not running."

This is a diagnostic command, but panicking diagnostic commands that read
state files are especially fragile: any upgrade that changes the `state.json`
schema, or any truncated state.json from an interrupted write, causes an
unrecoverable panic instead of a graceful degraded message.

**Fix:**
```go
pidVal, hasPID := lfState["pid"]
startedAtVal, hasStarted := lfState["started_at"]
if !hasPID || !hasStarted {
    fmt.Fprintln(cmd.OutOrStdout(), "LlamaFirewall Sidecar — Not running (incomplete state)")
    return nil
}
pidFloat, ok := pidVal.(float64)
if !ok {
    fmt.Fprintln(cmd.OutOrStdout(), "LlamaFirewall Sidecar — Not running (invalid state)")
    return nil
}
startedAt, ok := startedAtVal.(string)
if !ok {
    fmt.Fprintln(cmd.OutOrStdout(), "LlamaFirewall Sidecar — Not running (invalid state)")
    return nil
}
pid := int(pidFloat)
```

---

## Warnings

### WR-01: `beekeeper diag` loads config and discards the result — dead code with wrong comment

**File:** `cmd/beekeeper/diag.go:49`

**Issue:** Line 49 loads the config and discards both return values:

```go
_, _ = config.Load(configPath) // load config via layered resolver (CODE-05)
```

This is pure dead code. The config result is never used by `CollectDiag` or
anything else in the function body. The comment is also incorrect: `config.Load`
is not the layered resolver (that would be `config.LoadLayered`). This looks
like a stub that was never completed.

**Fix:** Remove the dead load line. If the intent was to apply config to diag
(e.g., for display purposes), wire it properly:
```go
// Remove this line entirely, or if config fields are needed for diag output:
cfg, err := resolveConfig(cmd)
if err != nil {
    // diag is a diagnostic command — continue with defaults rather than failing
    cfg = config.Config{}
}
_ = cfg // use cfg where needed
```

---

### WR-02: `LatencyTracker.head` is never reset — the comment documents wrong invariant and overflow is theoretically possible

**File:** `internal/llamafirewall/latency.go:17`

**Issue:** The field comment says `// next write position (0..99)` but `head`
is never reset to 0. It monotonically increases. After 100 samples `head` is
100, then 101, etc. The ring write uses `t.p95buf[t.head%100]` which is
functionally correct. However:

1. The invariant comment is wrong, which is a maintenance hazard: future
   readers or contributors may trust the documented range and introduce logic
   that breaks.
2. On a long-running process (e.g., inside the MCP gateway or Sentry daemon)
   that makes trillions of calls, `head` as a plain `int` would theoretically
   overflow. On 64-bit platforms `math.MaxInt64 / 1 = 9.2e18` calls — not a
   practical risk, but the comment misled the implementation.

**Fix:**
```go
// Option A: Reset head after reaching 100 to match the comment invariant:
t.p95buf[t.head] = ms
t.head = (t.head + 1) % 100
if t.head == 0 {
    t.filled = true
}

// Option B: Keep the current modulo logic but fix the comment:
head   int  // monotonically increasing write counter; ring index is head%100
```

Option B is the minimal fix if the modulo approach is intentional.

---

### WR-03: P95/P99 percentile index formula overstates the percentile for small sample counts

**File:** `internal/llamafirewall/latency.go:49` and `72`

**Issue:** Both `P95()` and `P99()` compute the percentile index as:

```go
idx := int(float64(n) * 0.95)  // or 0.99
```

For n=100: `idx = 95`. `buf[95]` is the 96th sorted value, which is the 96th
percentile (0-indexed), not the 95th. The standard nearest-rank formula for
the k-th percentile is `ceil(k/100 * n) - 1` (0-indexed). This formula
consistently returns a value one rank higher than the true P95/P99 for any n
where `n * 0.95` is a whole number.

For n=20: `idx = 19 = n-1`, which is the maximum value (P100), not P95.

Since latency metrics are used to determine whether the system meets the
`<100ms` target, overstating P95/P99 will generate false alarms during normal
operation (the reported value is higher than the true percentile) but
understates safety issues in adversarial conditions. This is a
quality/reliability defect rather than a security defect.

**Fix:**
```go
// P95: nearest-rank formula, 0-indexed
idx := int(math.Ceil(float64(n)*0.95)) - 1
if idx < 0 {
    idx = 0
}
if idx >= n {
    idx = n - 1
}
```

Requires adding `"math"` to the import list.

---

### WR-04: `resolveConfig` does not apply the system layer or project layer — documented multi-layer behavior is incomplete

**File:** `cmd/beekeeper/diag.go:104-106`

**Issue:** Beyond the env-var omission in CR-02, `resolveConfig` also never
sets `LayerOpts.SystemPath` or `LayerOpts.ProjectPath`. The architecture
specifies a five-layer merge (CODE-05): system → user → project → env → flags.
In practice, `resolveConfig` only applies one layer (user config file). System
administrators who deploy Beekeeper and configure `/etc/beekeeper/config.json`
to enforce a fail-closed mode will find their configuration silently ignored.

This is a warning (not critical) because the user config file is the most
important layer and its behavior is correct. The missing layers reduce security
posture for enterprise/multi-user deployments.

**Fix:**
```go
func resolveConfig(cmd *cobra.Command) (config.Config, error) {
    configPath, err := platform.ConfigPath()
    if err != nil {
        return config.Config{}, fmt.Errorf("resolve config path: %w", err)
    }

    opts := config.LayerOpts{
        SystemPath:  "/etc/beekeeper/config.json",
        UserPath:    configPath,
        ProjectPath: filepath.Join(".", ".beekeeper", "config.json"), // CWD project layer
        Environ:     os.Environ(),
    }

    cfg, err := config.LoadLayered(opts)
    if err != nil {
        return config.Config{}, fmt.Errorf("load layered config: %w", err)
    }
    return cfg, nil
}
```

Note: `SystemPath` should use a platform-specific constant (e.g. empty on
Windows where `/etc` doesn't exist). The project path should be resolved from
the working directory or a flag.

---

## Info

### IN-01: `llamafirewall enable/disable/status` use `os.ExpandEnv("$HOME")` instead of the `platform` package

**File:** `cmd/beekeeper/main.go:1328`, `1347`, `1367`, `1390`

**Issue:** Four places in the `llamafirewall` subcommand group hard-code the
config path as `filepath.Join(os.ExpandEnv("$HOME"), ".beekeeper", "config.json")`
instead of calling `platform.ConfigPath()`. All other subcommands use the
`platform` package. On Windows, `$HOME` may be unset or differ from the
expected user home directory (`%USERPROFILE%`). `platform.ConfigPath()` applies
platform-appropriate resolution. This is an inconsistency that could cause
incorrect paths on non-Unix platforms.

**Fix:** Replace all four instances with `platform.ConfigPath()` and handle
the error, consistent with other subcommands:
```go
cfgPath, err := platform.ConfigPath()
if err != nil {
    return fmt.Errorf("resolve config path: %w", err)
}
```

---

### IN-02: `RunAuditRecordWithLLMF` uses `context.Background()` instead of the command's context

**File:** `internal/check/handler.go:447`

**Issue:** `RunAuditRecordWithLLMF` creates a `context.Background()` for its
LlamaFirewall scan, ignoring any deadline or cancellation from the invoking
context. In practice `audit-record` does not have a timeout context passed
from Cobra, so impact is limited. However it is inconsistent with `runCheck`
which correctly uses a timeout-bounded context for LLMF calls. If a timeout
is added to the `audit-record` command in a future phase, this call would
silently ignore it.

**Fix:** Pass the command's context through to `RunAuditRecordWithLLMF` and
use it:
```go
// Signature change:
func RunAuditRecordWithLLMF(ctx context.Context, stdin io.Reader, ...) int

// Internal usage (replace context.Background()):
ctx := ctx // already passed in; no change needed
```

---

_Reviewed: 2026-05-29T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
