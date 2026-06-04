---
phase: 07-sensitive-path-runtime-enforcement
reviewed: 2026-06-04T00:00:00Z
depth: standard
files_reviewed: 9
files_reviewed_list:
  - internal/check/handler.go
  - internal/check/handler_test.go
  - internal/check/integration_test.go
  - internal/check/paths.go
  - internal/check/paths_test.go
  - internal/policy/path.go
  - internal/policy/path_test.go
  - internal/policyloader/enforce.go
  - internal/policyloader/enforce_test.go
findings:
  critical: 2
  warning: 4
  info: 3
  total: 9
status: issues_found
---

# Phase 7: Code Review Report

**Reviewed:** 2026-06-04T00:00:00Z
**Depth:** standard
**Files Reviewed:** 9
**Status:** issues_found

## Summary

Phase 7 wires the pure `policy.EvaluatePath` / `DefaultSensitivePaths` engine into the
live `beekeeper check` pipeline via a new impure adapter (`internal/check/paths.go`),
extends the default block/allow patterns, fixes `extractTargetPath`, and adds RunCheck
integration tests. The purity contract is upheld (`internal/policy/path.go` imports only
`strings`; `TestPathImportsArePure` enforces this), the canonicalization order matches D-01
(env-var → tilde → Abs → EvalSymlinks-with-Abs-fallback → ToSlash), `mergeDecisions` is
correctly most-restrictive-wins and runs before `ApplyPolicyOverlay`, and the unresolved-var
path is fail-closed (raw `%VAR%` token preserved).

However, this is a credential-read blocker where any ALLOW that should be a BLOCK is a real
breach, and two such bypasses exist in the shipped code:

1. **`extractBashCredentialPaths` only scans the FIRST occurrence of each read verb** — a
   compound command like `cat a.txt && cat ~/.ssh/id_rsa` silently misses the credential
   read and returns ALLOW. The same verb-scan also blindly takes the first token after the
   verb, so a leading flag (`cat -n ~/.ssh/id_rsa`) defeats it.
2. **`ApplyPolicyOverlay`'s `package_allowlist` allow escape-hatch can silently downgrade a
   merged sensitive-path block to ALLOW** for a Bash call that carries both a package and a
   credential read — directly violating the phase invariant that overlays may escalate but
   never silently downgrade a path block.

Both are reachable from a single, realistic agent tool call and so are classified BLOCKER.
The remaining findings are robustness and consistency issues.

## Critical Issues

### CR-01: Bash credential extraction misses all-but-first verb occurrences and leading flags (verb-scan bypass)

**File:** `internal/check/paths.go:222-236` (`extractBashCredentialPaths`), `internal/check/paths.go:184-207` (`firstShellToken`)
**Issue:**
`extractBashCredentialPaths` uses `strings.Index(cmd, verb)` which finds only the FIRST
occurrence of each verb in the command string. For a compound/chained command the credential
read is silently dropped:

```
cat /tmp/banner.txt && cat ~/.ssh/id_rsa     # extracts "/tmp/banner.txt", MISSES ~/.ssh/id_rsa → ALLOW
echo start; tail ~/.aws/credentials          # "tail" found once, but if a prior benign "tail" exists it wins
```

The existing test `TestExtractBashCredentialPaths/"verb found mid-string after command chaining"`
only passes because there is a *single* `cat` in `ls && cat ~/.aws/credentials`. Add a second
benign `cat` before it and the credential read escapes detection.

Compounding this, `firstShellToken` returns the first whitespace/quote-delimited token verbatim
with no flag handling, so a leading option defeats extraction entirely:

```
cat -n ~/.ssh/id_rsa      # firstShellToken returns "-n"; ~/.ssh/id_rsa never evaluated → ALLOW
head -c 100 ~/.netrc      # returns "-c"; credential read missed
```

This is a fail-OPEN on the core credential-blocker surface. The phase's deferred list
explicitly covers only nested shells, base64, and here-strings — multiple-occurrence and
flag-prefixed reads are NOT deferred and are simple, common command forms.

**Fix:** Scan every occurrence of each verb (loop with a moving offset), and skip leading
`-`/`--` flag tokens to reach the first path-like argument:

```go
func extractBashCredentialPaths(cmd string) []string {
	var paths []string
	for _, verb := range bashReadVerbs {
		from := 0
		for {
			rel := strings.Index(cmd[from:], verb)
			if rel == -1 {
				break
			}
			idx := from + rel
			rest := strings.TrimSpace(cmd[idx+len(verb):])
			// Skip leading flag tokens to reach the first non-flag argument.
			for {
				tok := firstShellToken(rest)
				if tok == "" {
					break
				}
				if !strings.HasPrefix(tok, "-") {
					paths = append(paths, tok)
					break
				}
				rest = strings.TrimSpace(rest[strings.Index(rest, tok)+len(tok):])
			}
			from = idx + len(verb)
		}
	}
	return paths
}
```

(At minimum, also evaluate ALL tokens after the verb, not just the first, since
`cat a b ~/.ssh/id_rsa` reads three files.)

### CR-02: package_allowlist allow escape-hatch silently downgrades a merged sensitive-path block

**File:** `internal/check/handler.go:265-290` (path merge then overlay), `internal/policyloader/enforce.go:153-165` (overlay step 2)
**Issue:**
`runCheck` merges the sensitive-path decision into `decision` (lines 265-273) and then calls
`ApplyPolicyOverlay(policyFiles, toolCall, decision)` (line 289). `ApplyPolicyOverlay` step 2
(`enforce.go:153-165`) downgrades `base.Level == "block"` to ALLOW whenever an `allow`
`package_allowlist` rule matches the tool call's package. For a Bash tool call the package and
the credential read coexist, so a user with a benign allowlist entry gets a credential block
silently dropped:

```
toolCall: {"tool_name":"Bash","tool_input":{"command":"npm install react && cat ~/.ssh/id_rsa"}}
policy file: package_allowlist allow react (npm)

→ extractPathTargets finds "cat ~/.ssh/id_rsa"  → mergeDecisions yields decision.Level="block"
→ extractEcoPackage finds (npm, react)          → overlay step 2 matches allow rule
→ ApplyPolicyOverlay returns Allow:true / "allow"  ❌  credential block silently downgraded
```

The phase context states the path block "must run before ApplyPolicyOverlay so overlays can
escalate but not silently downgrade a path block." The current ordering satisfies escalation
(block rules still win via step 1) but the step-2 allow hatch (T-09-31, intended only for
*package* trust overrides) now reaches *path* blocks because they were folded into the same
`base` Decision. The phase's deferred list defers the `sensitive_path action:"allow"` branch
but does NOT authorize package allowlists to override path blocks.

**Fix:** Keep the sensitive-path block separate from the overlay's downgradeable base. Apply
`ApplyPolicyOverlay` to the engine decision first, then merge the path decision last so a path
block cannot be downgraded:

```go
// Engine decision → overlay (package allowlist may downgrade engine block) → path block last.
if len(policyFiles) > 0 {
	decision = policyloader.ApplyPolicyOverlay(policyFiles, toolCall, decision)
}
spathCfg := policy.DefaultSensitivePaths()
for _, rawPath := range extractPathTargets(toolCall) {
	resolved := canonicalizePath(rawPath)
	if resolved == "" {
		continue
	}
	decision = mergeDecisions(decision, policy.EvaluatePath(resolved, spathCfg))
}
```

Alternatively, gate the overlay step-2 downgrade so it never applies when the base block
carries the `sensitive-path-policy` rule ID. Either way, add a regression test:
`Bash{command:"npm install react && cat ~/.ssh/id_rsa"}` + allow-react policy MUST still block.
(`integration_test.go:runCheckWithIndex` does not exercise the overlay at all, so this gap is
currently untested.)

## Warnings

### WR-01: expandWinEnvVars re-parses substituted values, allowing unintended re-expansion

**File:** `internal/check/paths.go:51-100`
**Issue:**
After substituting `%VAR%` with its value (line 87), the loop restarts `strings.Index(result, "%")`
from the beginning of the mutated string. If a resolved env-var value contains a `%`, or if
substitution juxtaposes a value ending in non-`%` text with a following literal `%token%`, the
walker can interpret spans of the *substituted* content as a new variable reference and expand
again. Example: raw `%A%%B%` where `A`→`x%y` produces `x%y<B-value>` and the stray `%` from A's
value can pair with B's delimiters in a surprising way. Because the attacker controls `raw`
fully, this is primarily a correctness/robustness defect rather than a privilege escalation, but
it makes the expansion non-deterministic and hard to reason about for a security-critical path.

**Fix:** Build the output in a single left-to-right pass that never re-scans already-emitted
output (track a write cursor and append resolved values to a `strings.Builder`), so substituted
values are treated as opaque literals and never re-expanded.

### WR-02: NUL-sentinel placeholder is attacker-injectable and fragile

**File:** `internal/check/paths.go:80-99`
**Issue:**
Unresolved vars are encoded with an in-band `"\x00UNEXPANDED\x00" + varName + "\x00"` sentinel,
then restored in a second loop. A `raw` input containing a literal `\x00UNEXPANDED\x00FOO\x00`
sequence (NUL bytes are legal in a JSON string) is rewritten to `%FOO%` by the restore loop even
though no expansion occurred. While NUL in a path is unusual and the impact here is low (the
forged `%FOO%` is not re-expanded), encoding control data in-band within untrusted data is a
fragile pattern that invites future bypasses.

**Fix:** Avoid in-band sentinels entirely. Use the single-pass `strings.Builder` rewrite from
WR-01: when a var is unresolved, append the original `%varName%` directly to the builder and
advance the cursor — no placeholder/restore round-trip needed.

### WR-03: Overlay package_allowlist downgrade is untested in the live RunCheck path

**File:** `internal/check/integration_test.go:36-87`, `internal/check/handler_test.go:519-565`
**Issue:**
`runCheckWithIndex` (the hermetic integration harness) does NOT load or apply `ApplyPolicyOverlay`
— it only calls `policy.Evaluate` + the path merge. The only live-overlay test
(`TestPolicyOverlayBlocksViaDir`) exercises a *block* rule on a package-only call. No test covers
the interaction of an `allow` package_allowlist rule with a sensitive-path block in a Bash
command (the CR-02 scenario), so the regression is invisible to CI. This is a test-coverage gap
on the highest-risk interaction in the phase.

**Fix:** Add a `RunCheck` test with a real policies dir containing a `package_allowlist` allow
rule and a Bash command that both installs the allowlisted package and reads a credential file;
assert the result is still `block`.

### WR-04: extractTargetPath returns an empty "path" value where extractPathTargets guards against it

**File:** `internal/policyloader/enforce.go:282-295`
**Issue:**
`extractTargetPath` guards the `file_path` branch with `ok && p != ""` (line 287) but the legacy
`path` branch only checks `ok` (line 291), so an explicit `{"path":""}` returns `""` from the
`path` branch instead of falling through. By contrast, the sibling `extractPathTargets`
(`paths.go:261`) correctly guards both with `p != ""`. The behavioral impact is nil today (empty
path matches nothing), but the asymmetry between two functions that are documented as mirroring
each other is a latent inconsistency that will bite if either gains a non-empty-default code path.

**Fix:** Add the `&& p != ""` guard to the `path` branch for parity with both `file_path` here
and `extractPathTargets`:

```go
if p, ok := tc.ToolInput["path"].(string); ok && p != "" {
	return p
}
```

## Info

### IN-01: EvalSymlinks can resolve away a sensitive fragment when an ancestor dir is a symlink

**File:** `internal/check/paths.go:144-150`
**Issue:**
`canonicalizePath` runs `filepath.EvalSymlinks` after `Abs`. For an existing path whose sensitive
ancestor segment is itself a symlink (e.g. `~/.ssh` symlinked to `~/keystore-public`), EvalSymlinks
rewrites the resolved path so the `/.ssh/` fragment disappears, and `EvaluatePath` no longer
matches. This is an uncommon, user-self-inflicted configuration and is largely out of the realistic
threat model, but it is worth a documented note that symlink resolution can both add and remove
sensitive fragments.

**Fix:** Consider evaluating `EvaluatePath` against BOTH the pre-EvalSymlinks `Abs` result and the
post-EvalSymlinks result, blocking if either matches (most-restrictive). Document the trade-off if
left as-is.

### IN-02: Windows alternate-data-stream / trailing-dot basename forms can evade exact basename match

**File:** `internal/policy/path.go:128-148`
**Issue:**
Basename block patterns (`.env`, `.netrc`, `.npmrc`, `.pypirc`) use exact last-segment equality.
On Windows, `\.env::$DATA` (NTFS default-stream syntax) or a trailing dot/space (`.env.` / `.env `)
opens the same underlying file but the last segment is not exactly `.env`, so the block does not
fire. The fragment patterns (`/.ssh/` etc.) are unaffected. This is a Windows-specific
canonicalization edge that the phase's deferred list does not mention.

**Fix:** Normalize the basename before exact comparison (strip a trailing `::...$DATA` stream
suffix and trailing dots/spaces) in `lastSegment`/`matchesBlockPattern`, or document as a known
deferred Windows limitation.

### IN-03: "more " and "less " pager verbs match inside larger words/paths

**File:** `internal/check/paths.go:164-173` (`bashReadVerbs`), `internal/check/paths.go:225` (`strings.Index`)
**Issue:**
The trailing space in each verb (`"more "`, `"type "`) prevents matching bare prose words, but
`strings.Index` still matches the verb anywhere in the string, including immediately after a path
component or as a substring of a longer command token (e.g. `nomore foo` does not match, but
`echo done; more readme` matches `more `, extracting `readme` as a false positive). False positives
here fail SAFE (extra evaluation, possible spurious block of a benign read), so this is
informational, but the verb matching is not word-boundary-aware.

**Fix:** Require the verb to start at a command boundary (string start, or preceded by a shell
separator `; & | \n`) before treating it as a read verb, to reduce false positives.

---

_Reviewed: 2026-06-04T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
