# Security Review — Detection-Only First-Responder + Reversible Quarantine

**Feature branch:** `feat/first-responder-quarantine` (diff base: `main`, 6 Go commits + web docs)
**Reviewer:** security-engineer (adversarial)
**Date:** 2026-06-12
**Scope:** `internal/quarantine`, `internal/watch/{crossref,firstresponder}.go`, `internal/sentry/{targets,rules,types}.go` + `linux/daemon.go`, `internal/config/config.go`, `internal/tui/incidents.go`, `cmd/beekeeper/main.go` wiring.

---

## Executive Summary

**Overall risk posture: SHIP WITH FIXES (no Critical, no exploitable High that crosses a NEW trust boundary).**

The feature is conservatively designed and the dangerous primitives are gated behind the StateDir self-protection boundary that already exists. The auto-quarantine mover defaults to **off + dry-run + threshold 2**, dry-run is genuinely side-effect-free, and the Sentry target-list is strictly detection-only (it can only *lower* correlation thresholds, never loosen them, never add a kill/isolate/network action). Fail-closed discipline on the move path is correct: a `MoveTyped` error leaves the artifact in place and still audits.

The findings that matter are **defense-in-depth gaps and one genuine bug**, not a remote-exploit chain:

- **One real bug (High, not remotely exploitable):** the first-responder and cross-reference audit-write paths **bypass the `audit.RedactRecord` chokepoint** that every other audit producer in the codebase applies. Attacker-influenced strings (package `Reason`, `CatalogMatches[].Package/EntryID`) are written to the security log unredacted. This is a regression against the documented TM-D-03 redaction invariant.
- **Two Medium hardening gaps:** (a) the quarantine **move/restore follow symlinks/junctions** with no `Lstat`/`EvalSymlinks` check — a TOCTOU/redirection primitive that is only reachable by an attacker who already controls the install tree (so it widens an existing capability rather than creating a new one); (b) the **Restore `..` guard does not normalize Windows drive-relative (`C:foo`) or extended-length (`\\?\`) path forms** before checking — defense-in-depth only, because writing the manifest requires already breaching the protected StateDir.

The headline path-traversal and arbitrary-write concerns were probed hard and are **adequately defended for the realistic threat model**: writing a malicious manifest or `sentry-targets.json` requires defeating the existing whole-StateDir self-protection block, which is out of scope for the agent.

### Counts by severity

| Severity | Count |
|----------|-------|
| Critical | 0 |
| High | 1 |
| Medium | 3 |
| Low | 3 |
| Informational | 4 |

---

## Findings Table

| ID | Severity | CWE | Title | File:line | Exploitable? |
|----|----------|-----|-------|-----------|--------------|
| F-1 | High | CWE-532 / CWE-117 | First-responder + cross-ref audit writes bypass `RedactRecord` | crossref.go:232; firstresponder.go:198 | Yes (info leak into security log) |
| F-2 | Medium | CWE-59 / CWE-367 | Quarantine Move/Restore follow symlinks/junctions (no Lstat/EvalSymlinks) | quarantine.go:145, 336 | Conditional (needs install-tree control) |
| F-3 | Medium | CWE-22 / CWE-59 | Restore `..` guard misses Windows drive-relative / `\\?\` / ADS forms | quarantine.go:316-334 | Conditional (needs StateDir breach) |
| F-4 | Medium | CWE-improper-input | Sentry target tightening fires regardless of `auto_quarantine.enabled`; a single warn-tier hit tightens detection on a legit package | crossref.go:204, firstresponder.go:115-118 | Yes (low-impact detection-only) |
| F-5 | Low | CWE-755 | Malformed `sentry-targets.json` at daemon startup silently disables tightening (detection fail-open) | linux/daemon.go (LoadTargets `_`), targets.go:105 | No (defense degradation) |
| F-6 | Low | CWE-367 | TOCTOU between `os.Stat(candidate)` and manifest read/rename in Restore | quarantine.go:271, 293, 336 | No (within StateDir) |
| F-7 | Low | CWE-400 | Cross-reference is unbounded over inventory size + opens a fresh `MultiIndex` and a fresh audit `Writer` per hit | crossref.go:186-236 | No (self-DoS only) |
| F-8 | Info | CWE-improper-trust | `MoveTyped` does not validate the SOURCE `artifactPath`; the move source is fully attacker-influenced via pollen `project_path` | quarantine.go:115-147 | Bounded by corroboration gate |
| F-9 | Info | — | `policy` purity preserved; no new I/O in the pure engine | rules.go, types.go | Verified safe |
| F-10 | Info | — | No command/argument injection in pollen exec | crossref.go:83-108 | Verified safe |
| F-11 | Info | — | No NDJSON newline injection (json.Marshal escapes control chars) | writer.go:88 | Verified safe |

---

## Per-Finding Detail

### F-1 — High — Audit redaction bypass in first-responder + cross-reference (CWE-532, CWE-117)

**Location:** `internal/watch/crossref.go:229-236` and `internal/watch/firstresponder.go:198-203`.

**Description.** Every other audit producer in the codebase routes the record through `audit.RedactRecord(rec, audit.DefaultRedactPatterns())` *before* calling `Writer.Write` — confirmed in `internal/check/handler.go:513,676` and `internal/watch/handler.go:185,233`. The new code paths do **not**:

```go
// crossref.go:230-235
auditRec := audit.FromDecision(tc, decision, ...)
auditRec.RecordType = "finding"
if w, err := audit.NewWriter(cfg.AuditPath); err == nil {
    _ = w.Write(auditRec)   // <-- no RedactRecord()
    w.Close()
}
```

`audit.Writer.Write` (writer.go:84-120) does a bare `json.Marshal(rec)` with no redaction — it is not the chokepoint; the *caller* is expected to redact. The first-responder's `writeFirstResponderAudit` (firstresponder.go:178-204) has the same omission.

**Attack scenario.** A package's catalog match carries attacker-influenced `decision.Reason` and `decision.CatalogMatches[].Package / .EntryID`. `RedactRecord` exists specifically because these "attacker-influenced data paths" (its own comment, redact.go:147-149) could carry credential-adjacent strings (e.g. a package named or described to embed a `Bearer ...`/JWT/api-key-prefixed token, or a `project_path` that traverses a directory whose name leaks a secret). Those land verbatim in `~/.beekeeper/audit/beekeeper.ndjson`, which is then fanned out to remote sinks (syslog/OTLP/HTTPS) when configured. The security audit log is a forensic artifact that is explicitly redacted everywhere else; this path defeats that guarantee.

**Impact.** Sensitive-string leakage into the on-disk + remote-sink audit log; inconsistency with the documented TM-D-03 invariant. Not a code-execution or block-bypass issue, hence High not Critical.

**Remediation.** Wrap both writes exactly as the existing producers do:

```go
patterns := audit.DefaultRedactPatterns()
rec = audit.RedactRecord(rec, patterns)
_ = w.Write(rec)
```

Apply at crossref.go:233 (the `finding` record) and inside `writeFirstResponderAudit` at firstresponder.go:198. Consider additionally moving redaction *into* `Writer.Write` as a belt-and-suspenders chokepoint so future producers cannot regress — but at minimum match the existing call-site discipline now.

**Verification.** Add a test that emits a hit whose `Reason`/`CatalogMatches.Package` contains a `Bearer xxx`/JWT token and assert the on-disk NDJSON contains `[REDACTED]`, mirroring `internal/audit/redact_test.go`.

---

### F-2 — Medium — Quarantine Move/Restore follow symlinks/junctions (CWE-59, CWE-367)

**Location:** `internal/quarantine/quarantine.go:145` (`os.Rename(artifactPath, destDir)`) and `:336` (`os.Rename(entryDir, m.OriginalPath)`).

**Description.** Neither Move nor Restore performs an `os.Lstat` / `filepath.EvalSymlinks` on the source or destination before the rename. `os.Rename` on a path that *is* a symlink renames the link itself, but a path *containing* a symlinked/junctioned intermediate component is resolved by the OS at rename time. The project has previously flagged Windows junction-point quirks (CLAUDE.md Phase-2 research note, fsnotify junctions).

**Attack scenario.** The move source is `hit.InstalledPath`, derived from pollen's `project_path` (crossref.go:211-213). An attacker who controls the project tree (the realistic case for a malicious dependency) can make `node_modules/<flagged-pkg>` — or a parent component of the reported path — a symlink/junction pointing at a directory the *user* can write but the package install did not own (e.g. a sibling project, a shared cache). When auto-quarantine fires, `os.Rename` then moves that *redirected* target into quarantine, or on Restore writes the quarantined dir *through* a junction to an operator-unexpected location. There is a TOCTOU window between the corroboration decision and the rename in which the link can be swapped (CWE-367).

**Why Medium, not High.** Reaching this requires the attacker to already control the install tree on disk *and* to get the package corroborated by ≥2 signed sources (the auto-quarantine gate, off by default). It widens an existing "I can write your project dir" capability into a "I can also relocate a sibling dir" capability; it does not cross a fresh trust boundary on its own. But it is a genuine confused-deputy primitive in a privileged-ish daemon move and deserves hardening.

**Remediation.** Before each rename, `Lstat` the source and reject if it is a symlink/junction (`fi.Mode()&os.ModeSymlink != 0`, and on Windows additionally detect reparse points). For Restore, additionally `EvalSymlinks` the *parent* of `m.OriginalPath` and re-assert it is outside `quarantineDir` after resolution. Document that cross-link moves are refused (mirrors the existing cross-device refusal note at quarantine.go:108-112).

**Verification.** Test: create `src` as a symlink to a sentinel dir; assert Move refuses or moves the link (not the target). On Windows, repeat with `mklink /J` junction.

---

### F-3 — Medium — Restore `..` guard misses Windows drive-relative / extended-length / ADS path forms (CWE-22, CWE-59)

**Location:** `internal/quarantine/quarantine.go:316-334`.

**Description.** The Restore guard against a tampered `OriginalPath` does two checks: (1) reject if `filepath.Clean(OriginalPath)` is inside `quarantineDir`; (2) split the *raw* path on `/` and `\` and reject any `..` segment. This correctly catches `../../etc/cron.d` and `/q/../../etc/passwd` (covered by tests). But on Windows it does not normalize several path forms before the `..` split / prefix check:

- **Drive-relative:** `C:foo` (no separator) resolves against the *current directory on drive C:*, not the drive root — a classic Windows traversal that contains no `..` and is not absolute in the way `filepath.IsAbs` of a cleaned path expects.
- **Extended-length / UNC:** `\\?\C:\Windows\...` and `\\?\UNC\host\share\...` bypass normalization (`filepath.Clean` does not strip `\\?\`), so the `strings.HasPrefix(cleanOriginal, cleanQuarantine+sep)` quarantine-containment check can be evaded while still pointing at an arbitrary absolute location.
- **ADS / trailing dot/space:** `C:\Windows\System32\foo.exe:stream`, trailing `.`/space, and 8.3 short names are not canonicalized; an `OriginalPath` crafted in these forms can name a location that the operator did not intend, and the equality/prefix comparisons are byte-wise.

**Attack scenario.** Requires writing the on-disk `beekeeper-manifest.json` (the manifest is fully trusted on read, quarantine.go:291-303). That file lives under `~/.beekeeper/quarantine/...`, which is inside the StateDir `ReadWritePrefixes` self-protection block (selfprotect.go:19-20 explicitly lists `quarantine/`). So the agent cannot write it through the hook. The exploit therefore needs an out-of-band StateDir breach. Given that, a tampered `OriginalPath` of `\\?\C:\Windows\System32\...` would let Restore move a quarantined (attacker-planted) directory to an arbitrary absolute location = arbitrary-write/overwrite primitive that the `..`-only guard would not stop.

**Why Medium.** The precondition (write access to a self-protected file) is high, so this is defense-in-depth, not a live exploit. But the guard *advertises* itself (TM-D-05) as the backstop for a tampered manifest, so it should actually be Windows-complete.

**Remediation.** Canonicalize before validating: reject `OriginalPath` unless `filepath.IsAbs(m.OriginalPath)` AND `filepath.VolumeName(m.OriginalPath) != ""` (rejects `C:foo` drive-relative), strip/normalize a leading `\\?\`/`\\?\UNC\` before the containment and `..` checks, reject any path containing `:` after the volume name (ADS), and reject trailing dot/space components. Equivalently, resolve with `filepath.Abs` + `EvalSymlinks` and re-run the containment assertion on the resolved result. Mirror the `canonicalizePath` discipline already used in `internal/check`.

**Verification.** Table test adding `C:foo`, `\\?\C:\Windows\x`, `C:\x\y.txt:ads`, `C:\x ` (trailing space) cases to `TestQuarantineRestoreTamperedOriginalPath`, asserting each is rejected.

---

### F-4 — Medium — Sentry target tightening is not gated by `auto_quarantine.enabled` (CWE-improper-input)

**Location:** `internal/watch/crossref.go:204` (a hit is emitted for any catalog match ≥ warn tier) → `internal/watch/firstresponder.go:112-118` (`targets.AddTarget` runs **before** the `if !cfg.Enabled || count < threshold` gate) → persisted to `sentry-targets.json` → `applyTargetTightening` lowers `CredAccessThreshold`/`CredCLIThreshold` to 1 (rules.go:340-352).

**Description.** Two coupled facts: (a) `cmd/beekeeper/main.go` *always* sets `SentryTargetsPath`, independent of the auto-quarantine knob; (b) `RunFirstResponder` records *every* hit into the target list "regardless of threshold" (its own comment), and even a single warn-tier (1-source) hit produces a ScanHit. Net effect: with auto-quarantine fully **disabled** (the default), a *single* catalog source flagging a package still installs a Sentry tightening on that package's process subtree.

**Attack scenario / abuse.** An adversary controlling one catalog source (the exact single-source threat the corroboration model is meant to neutralize, THREAT-MODEL §"Corroboration Semantics") can flag a legitimate, widely-installed package (say `node`-driving npm package). Every process descended from `node` then trips credential rules at threshold 1 instead of 2 → alert fatigue / false-positive flood on the victim, usable as a noise/cover or a denial-of-attention tactic. It is **detection-only** (no block, no move), so impact is bounded to alert quality, not availability or integrity — hence Medium.

**Remediation.** Gate target-list recording on the same corroboration discipline the move path uses: only `AddTarget` when `hit.CorroborationCount >= threshold` (≥2), so a single-source flag cannot tighten Sentry. Alternatively, only maintain the target list when `cfg.Enabled` is true. Document the chosen posture in THREAT-MODEL.md so the "one compromised factor is not sufficient" claim stays accurate for the *detection-tightening* side-effect, not just the block decision.

**Verification.** Test: a `CorroborationCount:1` hit with `Enabled:false` must produce an empty/absent `sentry-targets.json`.

---

### F-5 — Low — Malformed targets file silently disables tightening at startup (CWE-755)

**Location:** `internal/sentry/linux/daemon.go` (`targets, _ := sentry.LoadTargets(targetsPath)`) and `internal/sentry/targets.go:105-118`.

**Description.** `LoadTargets` returns `(nil, err)` on a parse error (fail-closed *return*, but the daemon discards the error). With `targets == nil`, `applyTargetTightening` short-circuits (`cfg.Targets == nil → return cfg`), so a corrupt `sentry-targets.json` means **no tightening** — the detection enhancement silently turns itself off. The 60s reload (`if tl, err := ...; err == nil { targets = tl }`) correctly *keeps the old good list* on a mid-run corruption, which is the right call; only the startup path degrades.

**Impact.** Defense degradation, not a block-to-allow. Baseline (threshold-2) correlation still runs. Low.

**Remediation.** Log the startup `LoadTargets` error to stderr (and ideally an audit `targets_load_error` record) so an operator notices a corrupt file rather than silently losing tightening. Do not fail the whole daemon (that would let an attacker DoS Sentry by corrupting one file) — log-and-continue is correct, just make it visible.

---

### F-6 — Low — TOCTOU in Restore between Stat, manifest read, and rename (CWE-367)

**Location:** `internal/quarantine/quarantine.go:271` (`os.Stat(candidate)`), `:293` (`ReadFile(manifestPath)`), `:336` (`os.Rename`).

**Description.** Restore stats the entry dir, then reads the manifest, then renames — three separate filesystem operations on a path that, between steps, could be swapped if an attacker can write the quarantine dir. Same StateDir-breach precondition as F-3, so Low. Worth noting because it compounds F-2/F-3: the validated `OriginalPath` is read at one instant and acted on at another.

**Remediation.** Minor; if F-2/F-3 hardening adds an `EvalSymlinks`-after-resolve assertion immediately before the rename, the window narrows. Not independently actionable without the StateDir-breach precondition.

---

### F-7 — Low — Unbounded cross-reference work + per-hit allocations (CWE-400)

**Location:** `internal/watch/crossref.go:131-237`.

**Description.** `CrossReference` accumulates **all** `package` records into `pkgRecords` (no cap) and then, per hit, constructs a fresh `catalog.NewMultiIndex` (crossref.go:187) and — on every audited finding — opens and closes a brand-new `audit.NewWriter` (crossref.go:232-234). On a machine with a very large inventory (monorepo with tens of thousands of installed packages) and a catalog delta that matches many, this is O(hits) file-open/close churn plus unbounded slice growth, all on the catalog-watch goroutine. It is self-inflicted (your own inventory) so not an external DoS, but it can stall the watch loop.

**Remediation.** Bound `pkgRecords` (or stream-evaluate without materializing the whole slice), hoist the `MultiIndex` construction out of the loop (it is rebuilt identically each iteration), and open one audit `Writer` for the whole pass instead of one per finding. Cap total findings emitted per delta.

---

### F-8 — Informational — Move SOURCE path is unvalidated and fully attacker-influenced

**Location:** `internal/quarantine/quarantine.go:115-147`; source = `hit.InstalledPath` = pollen `project_path` (crossref.go:211).

**Note.** The traversal guard at quarantine.go:126-131 validates only the *destination* is under the type subdir. The *source* `artifactPath` is whatever pollen reported, with no allow-list (e.g. "must be under a known package root"). This is inherent to "move the thing from where it is installed," but it means the auto-quarantine mover will `os.Rename` **any absolute path** a corroborated catalog entry can get associated with a `project_path`. The corroboration gate (≥2 signed sources) is the real control here, and it is sound; combined with F-2 (symlinks) this is the avenue most worth watching. Recommend (defense-in-depth) asserting `artifactPath` is absolute and resides under a recognized ecosystem install root before moving, and refusing to move system-critical roots (`/`, `C:\Windows`, the StateDir itself).

---

### F-9 / F-10 / F-11 — Informational — Verified-safe items (see next section).

---

## Verified Safe / Checked

The following were probed adversarially and found adequately defended — each confirmed by reading the implementation, not the tests or comments:

1. **Destination traversal guard (Move).** quarantine.go:118-131 sanitizes `Publisher/Name/Version` with `filepath.Base` *and* re-asserts `cleanDest` has prefix `cleanTypeDir + sep`. The `../../etc` / `../passwd` publisher/name case is neutralized (each `filepath.Base` collapses to `etc`/`passwd`, and the prefix re-check is a second layer). Confirmed.

2. **Restore entry-id traversal.** quarantine.go:257 `filepath.Base(id)` strips directory components; the per-subdir prefix re-check at :266-270 rejects anything that escapes. `../../escape` is rejected (test + guard read). Confirmed.

3. **Restore-into-quarantine cycle.** quarantine.go:320-323 rejects any `OriginalPath` whose clean form equals or is under `quarantineDir`. Prevents the restore-to-quarantine loop. Confirmed for POSIX/normal paths (Windows edge forms = F-3).

4. **Relative `..` in manifest OriginalPath.** quarantine.go:328-334 splits the **raw** (pre-clean) path on both separators and rejects any `..` segment — correctly avoids the `Clean("a/../b")→"b"` blind spot. Confirmed.

5. **`policy` purity preserved (F-9).** The Sentry changes add a `Targets *TargetList` field to `RuleConfig` (types.go) and an `applyTargetTightening` pure function (rules.go:340-352) that only reads `event.PID`/`tree` and lowers two integer thresholds. No I/O, no goroutines, no side effects added to the engine hot path. `TargetList.MatchesPID`/`AddTarget` are pure; the only I/O (`LoadTargets`/`SaveTargets`) is outside `EvaluateEvent`, called by the daemon loop and first-responder. Matches the CLAUDE.md "internal/policy must stay pure / one impl three consumers" constraint. Confirmed.

6. **Tightening cannot LOOSEN or add destructive actions.** `applyTargetTightening` only ever sets thresholds *to 1* (lower = more sensitive); there is no code path that raises a threshold or adds a kill/isolate/network-cut. A nil `Targets` is a no-op identical to prior behavior. `MatchesPID` is monotonic (returns true → tighten). An attacker cannot invert it into a detection-*loosening* primitive. Confirmed by reading rules.go:340-352 + types.go:140-148.

7. **Command/argument injection in pollen exec (F-10).** crossref.go:83-108 builds `exec.CommandContext(ctx, bin, "scan"[, "--profile","deep"])` with a **fixed** arg slice — no package name, path, or version flows into argv, and there is no shell. `resolveCrossRefScanner` only ever returns a `LookPath` of the literals `"bumblebee"`/`"pollen"`. No injection surface. Confirmed.

8. **NDJSON newline / log injection (F-11).** `audit.Writer.Write` uses `json.Marshal(rec)` then appends a single `\n` (writer.go:88-93). `json.Marshal` escapes all control characters including `\n`/`\r` inside string values, so an attacker-controlled package name like `"evil\n{\"fake\":true}"` cannot forge a second NDJSON record. (The redaction gap F-1 is a *content* leak, not a *structural* injection.) Confirmed.

9. **Dry-run performs zero filesystem mutation.** firstresponder.go:125-129: when `DryRun`, the only action is `writeFirstResponderAudit` (append to the audit log) and `continue` — `MoveTyped` is never reached. The audit append is the intended observation. `TestFirstResponderDryRun` asserts `pkgDir` survives. Confirmed.

10. **Move fail-closed leaves artifact in place + audits.** firstresponder.go:153-159: a `MoveTyped` error logs, writes a `quarantine_error` audit record, and `continue`s — it never deletes or half-moves. `MoveTyped` itself does the rename *before* writing the manifest, so a manifest-write failure (after a successful rename) returns an error with the artifact already in quarantine but no manifest — that entry is then *skipped* by `List`/`Restore` (no manifest = invisible). That is a recoverable-only-by-hand state but not a fail-open; noted as a minor robustness gap, not a finding (the rename succeeded = artifact is contained, which is the safe direction).

11. **`auto_quarantine` defaults are safe.** config.go:391-397 `DefaultAutoQuarantineConfig` = `{Enabled:false, DryRun:true, Threshold:2}`; a nil block → `AutoQuarantineEnabled()` false, `AutoQuarantineDryRun()` true (config.go:442-456). Threshold clamp `[1,3]` with zero→2 (not zero→floor-1) is correct (parseClampAutoQuarantineThreshold, config.go:427-438) and validated fail-closed at Load (config.go:640-644). Two explicit changes (`enabled:true` + `dry_run:false`) are required for any real move. Confirmed.

12. **File permissions.** `SaveTargets` writes `sentry-targets.json` with `0600` and `MkdirAll(...,0700)` (targets.go:124-138); the quarantine manifest is written `0600` then re-enforced owner-only via `platform.SetOwnerOnly` (quarantine.go:155-162), which on Windows applies a real owner-only DACL via `hectane/go-acl` (perms_windows.go), not just the read-only attribute. Both files sit under the StateDir, which is the `ReadWritePrefixes` self-protection block — the agent cannot read or write them through the hook. Confirmed.

13. **Malformed targets file does not panic the daemon (DoS).** `LoadTargets` uses `json.Unmarshal` (returns error, never panics); `MatchesPID`/`AddTarget` are nil-safe (targets.go:40-42, 65-68). A garbage file degrades tightening (F-5) but cannot crash the engine loop. Confirmed.

---

## Deviations From / Additions To the Documented Threat Model

- **New trust input: pollen `project_path` → filesystem move source.** The feature promotes a pollen-reported path into the source of an `os.Rename` performed by the (auto-quarantine) daemon. THREAT-MODEL.md has no entry for this. Recommend adding a TM row documenting that the corroboration gate (≥2 signed sources) is the control, plus the F-2/F-8 symlink-and-allow-list hardening.
- **New trust input: `sentry-targets.json` → live correlation-threshold tightening.** Documented as detection-only in the new security.mdx, which is accurate. But F-4 (single-source tightening with auto-quarantine off) is a deviation from the spirit of the "one compromised factor is not sufficient" corroboration claim — the *detection-tightening* side-effect is reachable at 1 source. Reconcile the code (gate at ≥threshold) or the doc.
- **TM-D-03 redaction invariant regression (F-1).** The threat model asserts attacker-influenced strings are redacted before audit write; the two new write sites violate it. Fix the code, not the doc.

---

## Recommended Disposition

**SHIP AFTER F-1 (redaction) is fixed.** F-2/F-3/F-4 are strong hardening recommendations that can land in the same PR or a fast follow; none is a live remote exploit. The dangerous primitives are correctly fenced behind the existing StateDir self-protection and the off-by-default + dry-run + corroboration-2 posture, and the detection-only contract holds.

---

## Remediation (2026-06-12)

All actionable findings remediated on branch `feat/first-responder-quarantine`. Each fix is one atomic commit (behavior-first: test written/extended, then code). Full gate green: `go build ./...`, `go vet ./...`, `go test ./... -count=1`, and cross-OS `GOOS=linux/darwin go vet ./...` all exit 0.

| ID | Status | Commit | Proving test(s) |
|----|--------|--------|-----------------|
| F-1 | FIXED | `cada681` | `TestCrossReferenceFindingRedacted`, `TestFirstResponderAuditRedacted` (internal/watch) |
| F-2 | FIXED | `8125814` | `TestMoveTypedRefusesSymlinkSource`, `TestMoveTypedRefusesJunctionSource`, `TestRestoreRefusesSymlinkEntry` |
| F-3 | FIXED | `cff1535` | `TestQuarantineRestoreWindowsTamperedOriginalPath` (drive-relative / `\\?\` / ADS / trailing-dot / trailing-space), regression `TestQuarantineRestoreTamperedOriginalPath` |
| F-4 | FIXED | `3e0c70f` | `TestFirstResponderSentryTargetsCorroborationGate` (single-source → 0 targets; corroborated → 1 target); regression `TestFirstResponderSentryTargetsWritten` |
| F-5 | FIXED | `2efdedf` | `TestLoadTargetsCorruptFileReturnsError` (internal/sentry) + `GOOS=linux go vet` of the daemon log/audit path |
| F-6 | MITIGATED (by F-2/F-3) | — | window narrowed by the F-2 `EvalSymlinks` destination-parent re-assertion immediately before the Restore rename; precondition remains a StateDir breach (out of scope) |
| F-7 | FIXED | `06238d5` | regression `TestCrossReferenceHit` / `TestCrossReferenceNoHit` / `TestCrossReferenceUnresolvedPath` / `TestCrossReferenceReadOnly` (behavior unchanged; index hoisted, single deferred-Close writer, audited-findings cap of 1000 with no-silent-cap log) |
| F-8 | FIXED | `f7e5bee` | `TestMoveTypedRefusesSystemCriticalSource` (`/`, `C:\`, `C:\Windows`, `C:\Program Files`, quarantineDir), `TestMoveTypedRefusesRelativeSource` |

Chronological commit ordering: F-1 `cada681` → F-4 `3e0c70f` → F-7 `06238d5` → F-5 `2efdedf` → F-2 `8125814` → F-3 `cff1535` → F-8 `f7e5bee`.

### F-6 residual / overall residual

- **F-6** is mitigated by F-2 (the Restore path now refuses a reparse-point entry source and `EvalSymlinks`-re-asserts the resolved destination parent is outside `quarantineDir` immediately before the rename) and by F-3 (Windows-complete `OriginalPath` canonicalization). The remaining TOCTOU window between `os.Stat`, manifest read, and rename is only reachable by an attacker who can already write the self-protected quarantine dir (a StateDir breach), which is out of scope per the existing self-protection boundary.
- **F-8 residual:** the move source remains attacker-influenced (it is inherently "move the artifact from where pollen reported it"), but it is now (a) corroboration-gated (≥2 signed sources, the real control), (b) symlink/junction-refused (F-2), and (c) system-root-refused + absolute-path-required (F-8). A normal package directory is unaffected. No allow-list of "recognized ecosystem install roots" was added — that would risk false-refusals across the many real install layouts and the corroboration + system-root + symlink controls already bound the primitive.

### Notes / deviations from the spec

- **F-3 `\\?\` handling:** the spec said "strip the `\\?\` prefix and re-run the checks." Stripping alone would NOT reject `\\?\C:\Windows\x` (the stripped `C:\Windows\x` is absolute, has no `..`, no ADS colon, no trailing dot/space, and is not inside `quarantineDir`), yet the spec's own test table requires that form rejected. Resolved by **rejecting any extended-length-prefixed `OriginalPath` outright** (a restored artifact's path has no legitimate reason to carry `\\?\`), which both satisfies the test and is strictly safer than silent normalization. The containment + `..` checks still run on the (prefix-free) validation path for all other forms.
- **F-3 "dotdot in absolute path" regression case:** `filepath.Join(q, "..", "..", ...)` collapses the `..` before it reaches the guard, so on Windows that case is now caught at the rename stage (non-existent target) rather than the `..` segment check. The test only asserts a non-nil error and the entry staying intact, both of which hold; the legitimate round-trip restore is unaffected.
- **Windows-only path-form checks are `runtime.GOOS == "windows"`-guarded** (F-3) and the reparse-point helper is split into `reparse_windows.go` (symlink + `FILE_ATTRIBUTE_REPARSE_POINT`) / `reparse_other.go` (POSIX `ModeSymlink` no-op sibling), so POSIX behavior and the existing passing tests are untouched. The F-2 symlink and F-3 Windows-form tests ran (not skipped) on the Windows dev host; the `mklink /J` junction test also ran.
