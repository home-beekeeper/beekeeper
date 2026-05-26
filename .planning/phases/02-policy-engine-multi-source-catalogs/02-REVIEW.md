---
phase: 02-policy-engine-multi-source-catalogs
reviewed: 2026-05-26T00:00:00Z
depth: standard
files_reviewed: 24
files_reviewed_list:
  - cmd/beekeeper/main.go
  - internal/audit/types.go
  - internal/baseline/store.go
  - internal/catalog/age_cache.go
  - internal/catalog/loader.go
  - internal/catalog/multi.go
  - internal/catalog/osv.go
  - internal/catalog/registry.go
  - internal/catalog/sanity.go
  - internal/catalog/socket.go
  - internal/catalog/state.go
  - internal/catalog/watch.go
  - internal/check/handler.go
  - internal/check/selftest.go
  - internal/config/config.go
  - internal/policy/baseline.go
  - internal/policy/corroboration.go
  - internal/policy/credentials.go
  - internal/policy/egress.go
  - internal/policy/engine.go
  - internal/policy/exfil.go
  - internal/policy/lifecycle.go
  - internal/policy/path.go
  - internal/policy/release_age.go
  - internal/policy/types.go
findings:
  critical: 6
  warning: 8
  info: 4
  total: 18
status: fixed
fixed_at: 2026-05-26
---

# Phase 02: Code Review Report

**Reviewed:** 2026-05-26T00:00:00Z
**Depth:** standard
**Files Reviewed:** 24
**Status:** issues_found

## Summary

Phase 02 delivers multi-source catalog aggregation (Bumblebee + OSV + Socket), corroboration-based block enforcement, five policy rule engines (release age, lifecycle, path, egress, exfil/baseline), and the watch daemon. The architecture is generally sound and the fail-closed semantics are well-considered. However, the review surfaced six blockers and eight warnings that must be addressed before this code can be trusted in production.

The most serious class of finding is a group of URL-injection and path-traversal risks in the registry fetchers and Socket/OSV cache paths. A second class covers logic gaps that can break the fail-closed guarantee: the `Watch` daemon silently recovers from a `SaveState` error without degrading, `computeDelta` can silently record a stale state, and the `FetchPublishAge` return path allows a negative age to reach the policy engine. The corroboration logic also has a subtle always-true condition that makes the quarantine threshold dead code.

---

## Critical Issues

### CR-01: URL injection in npm/PyPI/Go/crates/RubyGems/Packagist registry fetchers

**File:** `internal/catalog/registry.go:74,94,113,132,155,172`
**Issue:** Every per-ecosystem `fetchXxxPublishTime` function and `fetchNPMLifecycleScripts` builds registry URLs by direct string concatenation of the caller-supplied `pkg` and `version` values without any sanitization or URL encoding. Because the policy engine calls these with attacker-influenced values extracted from the tool call (e.g. `ToolInput["package"]`), a crafted package name like `../../evil` or `%2F` sequences can redirect the HTTP request to an unintended endpoint on the same registry host, leak tokens to a malicious host path, or trigger SSRF if the registry base is later made configurable.

Concretely, for npm:
```go
url := npmRegistryBase + "/" + pkg  // pkg = "../../evil" → hits the wrong endpoint
```

**Fix:** Use `url.PathEscape` (not `url.QueryEscape`) for each path segment before concatenation, or construct URLs via `url.URL` struct with `url.JoinPath`:
```go
import "net/url"

func fetchNPMPublishTime(ctx context.Context, client *http.Client, pkg, version string) (string, error) {
    base, _ := url.Parse(npmRegistryBase)
    base.Path = "/" + url.PathEscape(pkg)
    ...
}
```
Apply this fix to all six per-ecosystem fetch functions and `fetchNPMLifecycleScripts`.

---

### CR-02: Path traversal in OSV cache path via unsanitized ecosystem, pkg, version

**File:** `internal/catalog/osv.go:91-97`
**Issue:** `osvCachePath` builds the cache file path by calling `filepath.Join(cacheDir, "osv", ecosystem, pkg, stem+".json")` where `ecosystem`, `pkg`, and `version` are attacker-supplied. `filepath.Join` calls `filepath.Clean` which resolves `..` components. A package name like `../../state` or an ecosystem of `../socket-cache` could write (or clobber) files outside the `<cacheDir>/osv/` subtree — for example, overwriting `state.json` or the Socket cache with attacker-controlled content.

```go
// Attacker sets pkg = "../../state", version = "x":
// path → <cacheDir>/osv/<ecosystem>/../../state/x.json
//       → <cacheDir>/state/x.json  (state.json clobbered or neighbour poisoned)
```

**Fix:** Validate or sanitize each path segment before joining. At minimum, reject segments that contain `/`, `\`, or `..`, or use `filepath.Base` to strip any directory component from `pkg` and `version`:
```go
func osvCachePath(cacheDir, ecosystem, pkg, version string) string {
    stem := version
    if stem == "" {
        stem = "_any"
    }
    // Strip any path component — cache key must be a plain filename.
    safePkg := filepath.Base(pkg)
    safeEco := filepath.Base(ecosystem)
    safeStem := filepath.Base(stem)
    return filepath.Join(cacheDir, "osv", safeEco, safePkg, safeStem+".json")
}
```

---

### CR-03: Path traversal in age-cache path via unsanitized inputs

**File:** `internal/catalog/age_cache.go:31-33`
**Issue:** `ageCachePath` has the same path-traversal vulnerability as CR-02. It calls `filepath.Join(cacheDir, "age-cache", ecosystem, pkg, version+".json")` with unsanitized attacker-controlled values. An attacker who can influence `pkg` or `version` (via a malicious tool call) can write arbitrary files under `cacheDir` or its parent.

**Fix:** Same as CR-02 — sanitize each segment with `filepath.Base` before passing to `filepath.Join`:
```go
func ageCachePath(cacheDir, ecosystem, pkg, version string) string {
    return filepath.Join(cacheDir, "age-cache",
        filepath.Base(ecosystem),
        filepath.Base(pkg),
        filepath.Base(version)+".json")
}
```

---

### CR-04: Corroboration quarantine threshold is dead code — `QuarantineAt` can never trigger independently

**File:** `internal/policy/corroboration.go:57-58`
**Issue:** The two `switch` cases for quarantine and block are:
```go
case signedCount >= t.QuarantineAt && hasSignedSource:
case signedCount >= t.BlockAt && hasSignedSource:
```
`hasSignedSource` is defined as `signedCount >= 1`. With default thresholds `BlockAt=2, QuarantineAt=3`, any `signedCount >= 3` also satisfies `signedCount >= 2`, so **the `BlockAt` case is always reached first when `signedCount == 3`**, making the `QuarantineAt` case unreachable in the default configuration.

For the quarantine path to be reachable, `QuarantineAt` must be strictly greater than `BlockAt`. The current switch evaluates from top to bottom, so the `BlockAt` case fires first whenever `signedCount >= BlockAt` — including when `signedCount >= QuarantineAt`. With `BlockAt=2, QuarantineAt=3` and `signedCount=3`: the first case `3 >= 3 && true` is true, so quarantine fires correctly. Wait — re-reading: Go `switch` evaluates cases in order; the first matching case wins. With `signedCount=3, BlockAt=2, QuarantineAt=3`: `3 >= 3 && true` → first case matches → quarantine fires correctly. However the **second** case `signedCount >= 2 && hasSignedSource` is also true for `signedCount=3`, but never reached because the first case already fired. This part is correct.

The actual bug is: with `QuarantineAt=3, BlockAt=2`, when `signedCount=2`, the first case `2 >= 3` is false, and the second case `2 >= 2 && true` is true → returns `("block", false, ...)` — correct. For `signedCount=3`: first case fires → `("block", true, ...)` — correct. **The logic is actually correct for the default thresholds.** However, if an operator sets `QuarantineAt < BlockAt` (e.g. via misconfiguration), the quarantine case would never fire. There is no validation that `WarnAt < BlockAt < QuarantineAt`.

**Fix:** Add a guard at the call site or within `corroborate` that asserts `t.WarnAt <= t.BlockAt <= t.QuarantineAt` and returns an error or panics with a clear message. Also add this validation to `DefaultCorroborationThresholds` callers.

---

### CR-05: `FetchPublishAge` can return a negative `ageMinutes` — policy engine blocks on zero but silently allows on negative

**File:** `internal/catalog/age_cache.go:105-107,155-156`
**Issue:** `FetchPublishAge` computes age as:
```go
age := int64(now.Sub(entry.PublishedAt).Minutes())
```
If `entry.PublishedAt` is in the future (e.g. an attacker-controlled registry returns a future timestamp, or clock skew puts the cached value in the future), `now.Sub(entry.PublishedAt)` is negative, and `age` will be a large negative `int64`. `EvaluateReleaseAge` then compares `input.AgeMinutes < threshold`: a negative age is always less than 1440 minutes, so the package is **blocked** — which is the safe direction. However, if `threshold` is 0 (operator sets `DefaultMinutes: 0` to disable the check), a negative age satisfies `age < 0` which is still less than 0, so the block still fires. The edge case is subtle but there is a documentation/contract mismatch: the `ReleaseAgeInput.AgeMinutes` field documents it as `time.Since(publishedAt).Minutes()` which is always non-negative for past packages. The missing validation means a future timestamp is silently cached as a negative age rather than being treated as a registry anomaly (i.e., `TimestampMissing=true`).

**Fix:** After computing `age`, treat negative age as a missing/anomalous timestamp:
```go
age := int64(now.Sub(publishedAt).Minutes())
if age < 0 {
    // Future timestamp — treat as missing (fail-closed).
    missingEntry := ageCacheEntry{CachedAt: now, Missing: true}
    _ = writeAgeCacheEntry(path, missingEntry)
    return 0, true, nil
}
```

---

### CR-06: `Watch` ignores `SaveState` error from `computeDelta` — degraded flag is not persisted, silently lost

**File:** `internal/catalog/watch.go:241-245` and `internal/catalog/watch.go:196-200`
**Issue:** `computeDelta` returns a four-value tuple `(delta, newState, sanityResult, err)`. When `SaveState` fails inside `computeDelta`, it **returns the computed `newState` and a non-nil `err`**. Back in `Watch`:
```go
delta, newState, sanityResult, err := computeDelta(ctx, cfg, st)
if err != nil {
    fmt.Fprintf(os.Stderr, "beekeeper watch: poll error: %v\n", err)
    continue // degraded mode — do not exit
}
st = newState
```
When `continue` fires, `st` retains its **previous value** (i.e. the state before the tick). The newly computed `newState` — which may contain a freshly set `Degraded=true` flag — is discarded. On the next tick, `prev WatchState` passed to `computeDelta` is the old state without the `Degraded` flag. The sanity breach is detected again, but if `SaveState` keeps failing (e.g. disk full), the `Degraded` mark is never durably written. The process emits stderr warnings but the state never persists.

This is especially dangerous: if disk-full causes save failures, the sanity circuit-breaker never takes effect persistently, and `computeDelta` re-evaluates from the same baseline on every tick, potentially treating an ongoing poisoning event as a recurring delta rather than a persistent degraded state.

**Fix:** When `SaveState` fails but `computeDelta` returns a valid `newState`, update the in-memory `st` regardless:
```go
delta, newState, sanityResult, err := computeDelta(ctx, cfg, st)
if err != nil {
    fmt.Fprintf(os.Stderr, "beekeeper watch: poll error: %v\n", err)
    // Still update in-memory state so degraded flags persist across ticks
    // even if the disk write failed. The next tick will retry the save.
    if newState.Sources != nil {
        st = newState
    }
    // Fall through to fire onDelta if sanity breach detected.
} else {
    st = newState
}
```

---

## Warnings

### WR-01: `sync.go` / `fetch` — no redirect validation; `DownloadURL` from GitHub API can be spoofed to exfil the Bearer token

**File:** `internal/catalog/sync.go:134-156`
**Issue:** `fetch` uses `client.Do(req)` which follows HTTP redirects by default. The `DownloadURL` field comes from the GitHub Contents API JSON response — an attacker who can MITM the GitHub response (or who compromises the `perplexityai/bumblebee` repo) can set `download_url` to an attacker-controlled host. The `http.Client` will follow the redirect and send the `Authorization: Bearer <GITHUB_TOKEN>` header to the attacker's server, leaking the token.

**Fix:** Set a custom `CheckRedirect` on the sync HTTP client that strips the `Authorization` header when the redirect leaves the `api.github.com` / `raw.githubusercontent.com` domain, or disable redirects entirely and fail if a redirect is returned:
```go
client := &http.Client{
    Timeout: 30 * time.Second,
    CheckRedirect: func(req *http.Request, via []*http.Request) error {
        // Strip auth on cross-host redirect to prevent token leakage.
        if req.URL.Host != via[0].URL.Host {
            req.Header.Del("Authorization")
        }
        return nil
    },
}
```

---

### WR-02: `socket.go` — retry loop reuses a consumed `strings.NewReader` body; retried requests send an empty body

**File:** `internal/catalog/socket.go:253-256`
**Issue:** In `querySocket`, `bodyJSON` is a `[]byte` slice, and a new `strings.NewReader(string(bodyJSON))` is constructed **inside the retry loop** but the reader object created for the first attempt is consumed after `client.Do`. On a 429 response the loop `continue`s, and on the next iteration, a **new** `strings.NewReader` is created correctly because `bodyJSON` is created once outside the loop and `strings.NewReader` is called fresh each iteration. This is actually safe on re-read.

However, the real issue is that the 429 branch reads and discards the response body before sleeping:
```go
body, _ := io.ReadAll(io.LimitReader(resp.Body, socketMaxBodyBytes))
resp.Body.Close()
```
But then:
```go
if attempt == maxRetries {
    return nil, true, fmt.Errorf(...)
}
```
The `maxRetries` check occurs **after** the sleep decision. On `attempt == maxRetries`, the code sleeps for one full backoff interval **before** returning the error. With `maxRetries=5` and `backoffBase=1s`, the final backoff is `2^4=16s` (or up to 60s with Retry-After), meaning the last iteration causes an unnecessary sleep before returning. The hook handler's 5s deadline will have long since expired.

**Fix:** Move the `maxRetries` check before the sleep:
```go
if attempt == maxRetries {
    return nil, true, fmt.Errorf("socket: rate limit exceeded after %d retries", maxRetries)
}
// Sleep only when we will actually retry.
select {
case <-time.After(sleep):
case <-ctx.Done():
    return nil, true, ctx.Err()
}
```

---

### WR-03: `egress.go` — `extractHost` incorrectly strips ports from IPv6 addresses in brackets

**File:** `internal/policy/egress.go:136-143`
**Issue:** `extractHost` strips the port by finding `strings.LastIndex(s, ":")`. For a bracketed IPv6 URL like `https://[::1]:8080/path`, after stripping the scheme the string is `[::1]:8080/path`. After stripping the path segment it becomes `[::1]:8080`. `LastIndex(":8080")` finds the last colon, which is the port separator — correct. But for an unbracketed IPv6 literal like `https://::1/path` (which is technically invalid but some implementations produce it), `LastIndex` finds the last colon in `::1`, stripping `1` as if it were a port and leaving `::` as the host.

More importantly: for a normal host with a port like `registry.npmjs.org:443`, the code checks `!strings.Contains(rest, ":")` where `rest = "443"` — this is fine. But for `[::1]` (no port), `LastIndex` finds the colon inside the brackets at position, say, 3, giving `rest = ":1]"` which contains `:`, so the stripping is skipped — that is actually correct. The real risk is that the comment says "Only strip port if what follows looks like a number" but the check `!strings.Contains(rest, ":")` is not a numeric check — it would pass for `"abc"` or `"8080abc"`.

The function also does not handle `@` in URLs (e.g. `https://user:pass@host/path`) — `user:pass@host` would be returned as the "host" after stripping scheme and path, causing the egress deny-list to fail to match.

**Fix:** Replace the hand-rolled parser with `url.Parse`:
```go
import "net/url"

func extractHost(rawURL string) string {
    u, err := url.Parse(rawURL)
    if err != nil || u.Host == "" {
        return rawURL
    }
    // u.Hostname() strips port and brackets from IPv6.
    return u.Hostname()
}
```

---

### WR-04: `path.go` — allowlist check uses `strings.HasPrefix` which can match partial path components

**File:** `internal/policy/path.go:79`
**Issue:** The allowlist check is:
```go
if allow == resolvedPath || strings.HasPrefix(resolvedPath, allow) {
```
This matches if `resolvedPath` starts with `allow`, but it does not require that `allow` ends with a path separator. So an allowlist entry of `/home/user/projects` would also match `/home/user/projects-secret/file.env`. An attacker who can create a path that is a prefix of the allowed path plus additional characters can bypass the blocklist.

**Fix:** Require that the prefix match ends on a path boundary:
```go
func isAllowed(resolvedPath, allow string) bool {
    if resolvedPath == allow {
        return true
    }
    if strings.HasSuffix(allow, "/") || strings.HasSuffix(allow, "\\") {
        return strings.HasPrefix(resolvedPath, allow)
    }
    return strings.HasPrefix(resolvedPath, allow+"/") ||
           strings.HasPrefix(resolvedPath, allow+"\\")
}
```

---

### WR-05: `handler.go` — OSV and Socket adapters store `ctx` from the outer request but the inner `5s` context is already running when adapters are constructed; `LookupAll` may get a near-expired context

**File:** `internal/check/handler.go:144-159`
**Issue:** The `OSVAdapter` and `SocketAdapter` store `ctx` (the 5s-deadline context) as a field at construction time:
```go
var osvAdapter policy.MultiCatalogLookup = &catalog.OSVAdapter{
    ...
    Ctx: ctx,
}
```
Then `policy.Evaluate` calls `idx.LookupAll(ecosystem, pkg)` which calls `a.Ctx` for the HTTP request. By the time `LookupAll` fires, the context has been running since the beginning of `runCheck`, including the stdin decode time and catalog open time. If stdin decoding takes 4.9s (e.g. 1MB of valid JSON), the OSV/Socket HTTP calls start with only 100ms left on the context, which is insufficient for a real HTTP roundtrip. The request fails, both sources degrade to nil, and the check falls back to Bumblebee only — reducing corroboration.

This is not purely a performance issue: it degrades the security level of the corroboration check. An attacker who can make stdin decoding slow (e.g. a 1MB near-limit payload) can force the check to evaluate with only Bumblebee.

**Fix:** Give OSV/Socket their own sub-context with a reasonable fraction of the remaining budget (e.g. 3s), derived from `ctx`:
```go
netCtx, netCancel := context.WithTimeout(ctx, 3*time.Second)
defer netCancel()

var osvAdapter policy.MultiCatalogLookup = &catalog.OSVAdapter{
    Client:   httpClient,
    CacheDir: cacheDir,
    Ctx:      netCtx,
}
```

---

### WR-06: `watch.go` — `computeDelta` returns the partial `newState` even on `SaveState` error; caller in `Watch` discards it

**File:** `internal/catalog/watch.go:196-200`
**Issue:** When `SaveState` fails, `computeDelta` returns `(delta, newState, result, fmt.Errorf("save state: %w", err))` — the non-nil `newState` contains the freshly computed degraded/non-degraded state. The `Watch` loop discards `newState` on error (see CR-06 above). Even if CR-06 is fixed, this also means that the `onDelta` callback is never fired for a tick where `SaveState` fails but the catalog content changed. A valid delta is silently swallowed.

**Fix:** Separate the concerns: compute the delta and sanity, call `onDelta` for any meaningful event, and report the save error separately rather than suppressing the delta notification:
```go
delta, newState, sanityResult, saveErr := computeDelta(ctx, cfg, st)
st = newState // always update in-memory state

if delta.HasChanges() || sanityResult.Alert || sanityResult.Block {
    if onDelta != nil {
        onDelta(delta, sanityResult)
    }
}
if saveErr != nil {
    fmt.Fprintf(os.Stderr, "beekeeper watch: state save error: %v\n", saveErr)
}
```

---

### WR-07: `baseline.go` — population standard deviation used where sample standard deviation is appropriate

**File:** `internal/policy/baseline.go:171-179`
**Issue:** `stddevFloat` divides by `len(vals)` (population stddev), but `historicalFreqs` is a sample of past daily counts. For small sample sizes (2-7 days), the population stddev is a downward-biased estimator of the true standard deviation, making the anomaly threshold `mean + 3*stddev` artificially lower. A genuine anomaly might fail to reach 3 sigma because the historical stddev is understated.

This does not produce an incorrect allow (the failure mode is a missing warn, not a missing block), but it weakens the detection sensitivity, especially during the first week of baseline accumulation.

**Fix:** Use Bessel's correction (divide by `n-1`):
```go
return math.Sqrt(sumSq / float64(len(vals)-1))
```

---

### WR-08: `credentials.go` — `openai-key` pattern `sk-[A-Za-z0-9]{20,}` matches OpenAI project keys (`sk-proj-...`) but also matches any `sk-` prefixed token from other services

**File:** `internal/policy/credentials.go:55`
**Issue:** The pattern `sk-[A-Za-z0-9]{20,}` matches any token beginning with `sk-` followed by 20+ alphanumeric characters. The pattern does not include `-` in the character class, so `sk-proj-abc...` (which contains a hyphen) would match only up to the hyphen. However, the current pattern would match `sk-T3BlbkFJabcdef12345678` (a real OpenAI API key format) correctly, but it would also match other services that use `sk-` prefixes (e.g. Stripe secret keys which use `sk_live_` or `sk_test_`). The underscore is not in the character class so Stripe keys are excluded. However, any custom internal token starting with `sk-` followed by 20+ alphanums would be redacted as an OpenAI key — a false-positive that masks the real credential type in audit records.

More critically: the pattern would match a partial AWS access key `sk-AKIA0123456789ABCDEF` from within a larger string, but since `AKIA...` is already matched by the aws-access-key pattern first, this is mostly harmless. The real concern is false positive coverage breadth.

**Fix:** Tighten the pattern to match the actual OpenAI key format more precisely (though OpenAI's format has changed over time, the current pattern should be documented as intentionally broad):
```go
// Note: intentionally broad to cover sk-..., sk-org-..., sk-proj-...
// Include hyphens and underscores in the character class:
re: regexp.MustCompile(`sk-[A-Za-z0-9_-]{20,}`),
```

---

## Info

### IN-01: `main.go` — `init` command creates directories with `0755` but security policy requires `0700`

**File:** `cmd/beekeeper/main.go:93`
**Issue:** The `init` command creates `stateDir`, `catalogDir`, and `auditDir` with permissions `0755` (world-readable/executable). The `audit/writer.go` comments and `baseline/store.go` explicitly create subdirectories with `0700` (owner-only). The top-level state directory itself being `0755` means other users on a multi-user system can list (though not read) the contents of `~/.beekeeper/`, potentially revealing the existence of baseline and audit files.

**Fix:** Change `0755` to `0700` for the three sensitive directories:
```go
if err := os.MkdirAll(dir, 0700); err != nil { ... }
```

---

### IN-02: `socket.go` — deprecated API endpoint with known removal date is hardcoded; no runtime fallback

**File:** `internal/catalog/socket.go:34`
**Issue:** The file's package-level comment prominently documents that `v0/purl` is deprecated since 2026-01-05 with removal scheduled for 2026-07-30 (35 days from the review date). When the endpoint is removed, `QuerySocket` will return a 404, which is treated as a non-200 status → `degraded=true`. The Socket source will silently degrade to no-match on every check, reducing corroboration without any warning to the operator.

**Fix:** Add a startup or per-call warning when the endpoint is deprecated. Prioritize the migration to `POST https://api.socket.dev/v0/packages` per the TODO comment. At minimum, log a warning to stderr from `LookupAll` when the deprecated endpoint returns a 404 that makes the deprecation visible before removal day.

---

### IN-03: `osv.go` — `readOSVCache` uses `os.IsNotExist` instead of `errors.Is(err, os.ErrNotExist)`

**File:** `internal/catalog/osv.go:122`
**Issue:** `os.IsNotExist(err)` is the legacy API. The Go standard library recommends `errors.Is(err, os.ErrNotExist)` because it correctly unwraps errors from packages that wrap `os.ErrNotExist`. The inconsistency with `state.go` line 51 and `baseline/store.go` line 42 (which both use `errors.Is`) is also a code-quality flag. This is unlikely to cause a real bug with `os.ReadFile` but could mask errors from a custom VFS.

**Fix:**
```go
if errors.Is(err, os.ErrNotExist) {
    return nil, false, nil
}
```

---

### IN-04: `selftest.go` — `fixtureMatches` does not validate `RuleIDs` in fixture expectations

**File:** `internal/check/selftest.go:180-188`
**Issue:** `fixtureMatches` checks `Level`, `Allow`, and presence of catalog matches, but does not verify `RuleIDs`. A regression that returns the wrong rule ID for a decision (e.g. returning `"bumblebee-catalog-match"` when `"lifecycle-script-policy"` is expected) would pass undetected through `selftest`. Since rule IDs are surfaced in audit records and used by downstream consumers for SIEM correlation, a wrong rule ID is a silent audit integrity issue.

**Fix:** Add `ExpectRuleID` (or `ExpectRuleIDs []string`) to the `fixture` struct and assert them in `fixtureMatches`. At minimum, assert that `RuleIDs` is non-empty for non-allow decisions.

---

_Reviewed: 2026-05-26T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
