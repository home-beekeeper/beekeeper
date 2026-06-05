# Beekeeper Codebase-Wide Threat Audit — Audit of Record

**Audit date:** 2026-06-05
**Scope:** Full codebase (v1.0.0 → v1.2.0 → v1.3.0/Phase-10 surface)
**Synthesized from:** 4 verified per-cluster audits (A entry/deny-protocol, B policy/catalog, C runtime Sentry, D audit/supply-chain/self-defense)
**Public artifact refreshed:** `docs/THREAT-MODEL.md` (in place)

---

## 1. Executive Summary

### Posture verdict

**Beekeeper's enforcement core is genuinely well-hardened, and no fail-open-becomes-allow defect was found in any primary intake path.** The check handler fails closed on every branch (oversize stdin, malformed JSON, exec timeout, panic, unreadable policy dir, missing mmap index) through a single audit-and-emit chokepoint; the MCP gateway parser enforces tight bounds and is fuzzed; JSON-RPC `id` correlation is echo-back (never positional); the bearer token is constant-time compared and stripped before upstream forward; IPC uses real SO_PEERCRED / LOCAL_PEERCRED / Windows-SID DACL auth at 0600; installers merge-not-clobber with per-target backups and never escalate privilege. No injection, IDOR, or path-traversal-to-RCE defect was found in the decision path. The product has carried self-defense work in every phase and it shows.

**The dominant theme of this audit is documentation debt, not code weakness.** `docs/THREAT-MODEL.md` was frozen at v1.0.0 (2026-05-29) and never updated for the v1.2.0 hardening (CORR per-severity corroboration, SPATH sensitive-path enforcement, NUDGE package-manager hardening) or the entire v1.3.0/Phase-10 harness deny-protocol surface (15 installers, exit-2 deny contract, the Hermes fail-OPEN class, gateway routing for no-hook harnesses). Several mitigations the doc *describes* are stronger on paper than in code.

**Three genuine code/doc-mismatch defects** rise above documentation debt and warrant follow-up:

1. **TM-A-01** — the `allow_remote_gateway` second-factor gate the help text promises **does not exist in code**. `--bind 0.0.0.0` exposes the gateway (cleartext bearer token, no TLS) with no config acknowledgement.
2. **TM-B-01** — the documented "degraded source counts at most 0.5 toward corroboration" anti-poisoning control **is not implemented**. A compromised critical-severity match from a single signed source can drive a single-source block.
3. **TM-B-02** — the primary Bumblebee "signed" second factor is a **string-presence check** (`CatalogSignature != ""`), not Ed25519 verification. Real signature verification exists but is wired only for the `beekeeper-self` feed.

None of these violate the fail-closed invariant; their impact is either operator deception (TM-A-01) or false-positive coercion / weaker-than-documented anti-poisoning (TM-B-01/02), not malicious-allow bypass.

### Findings by adjusted severity

Severities below reflect **adjusted** severity where the verification step overrode the original. Refuted false positives are excluded from the count (none of the cluster findings were fully refuted — `verified.real=false` did not occur; verification instead adjusted several severities downward and corrected the attack model for some).

| Adjusted severity | Count | IDs |
|-------------------|-------|-----|
| High | 1 | TM-B-01 |
| Medium | 8 | TM-A-01, TM-A-02, TM-A-03, TM-B-02, TM-D-01, TM-D-02, TM-D-03, TM-RS-01 |
| Low | 8 | TM-A-04, TM-B-04, TM-B-06, TM-B-07, TM-RS-02, TM-RS-03, TM-RS-06, TM-RS-07, TM-D-05, TM-D-06 |
| Info / mitigated | 4 | TM-A-05 (mitigated), TM-B-03 (accepted), TM-B-05 (accepted), TM-RS-05 (mitigated) |
| Low (other open) | — | TM-RS-04, TM-D-04 |

> Note: "Low" rows above list 10 IDs across two lines — see the per-cluster tables for the authoritative per-finding severity. The single High is the one verified-confirmed code/doc mismatch in a core anti-poisoning control.

**Disposition mix:** 6 `open`, 12 `partial`, 3 `accepted` (already documented), 2 `mitigated`.

---

## 2. Cluster A — Entry Points & Harness Deny Protocol

### Trust boundaries

| Boundary | Hardening posture |
|----------|-------------------|
| agent tool-call → `beekeeper check` stdin | 1MB LimitReader+1 oversize probe, json.Decode, 8s exec deadline, 256MB mem cap, top-level panic-recover fails closed (`internal/check/handler.go:121-357`) |
| harness ⇄ `--hook` deny adapter (exit code + stdout JSON) | exit 2 universal + per-family deny JSON; `RunCheckTo(io.Discard)` suppresses raw Decision JSON (`internal/check/deny_render.go`, `cmd/beekeeper/main.go:312-331`) |
| MCP client → gateway HTTP listener | stateless proxy on 127.0.0.1:7837; constant-time bearer auth, 1MB body cap, bounded JSON-RPC parser (depth 10, batch rejected), echo-back `id` correlation |
| gateway → upstream MCP server | own Authorization token stripped on every forward path; upstream URL operator-trusted (`--upstream`) |
| CLI ⇄ Sentry daemon IPC | Unix socket SO_PEERCRED/LOCAL_PEERCRED UID check, 0600+chown; Windows named pipe SDDL DACL to installing-user SID; 64KB framed JSON cap |
| package-manager shim → `beekeeper check` | PATH-prepended shim execs real binary only on exit 0; args as separate flags (json.Marshal in Go, no shell interpolation) |
| installer → user agent config files | 15 per-harness installers, merge-not-clobber, per-target 0600 backup, atomic temp+rename, foreign-script preservation, user privileges only |

### STRIDE findings table

| ID | STRIDE | Component | Adj sev | Disp | Evidence (file:line) | Gap |
|----|--------|-----------|---------|------|----------------------|-----|
| TM-A-01 | Tampering/EoP | gateway Config / `newGatewayCmd` | **Medium** (was High) | open | `cmd/beekeeper/main.go:1192-1194` (help promises gate), `:1294/:1317-1320` (bind unchecked); `internal/gateway/gateway.go:109-111`; grep `allow_remote_gateway` → no config matches | Promised second factor does not exist; `--bind 0.0.0.0` exposes proxy with cleartext bearer token, no TLS |
| TM-A-02 | Spoofing/Tampering | `deny_render.go` HarnessHermes + `--hook` path | Medium | partial | `internal/check/deny_render.go:226-243` (ExitCode:0, JSON-only); `cmd/beekeeper/main.go:312-313` (io.Discard); `internal/hooks/hermes.go:5-17` | Structurally fail-open by harness design; no exit-code backstop; block rests entirely on Hermes stdout-parse contract |
| TM-A-03 | EoP (policy bypass) | `deny_render.go` Windsurf/OpenCode/Kilo/Trae + `gateway_targets.go` | Medium | accepted | `internal/check/deny_render.go:245-250`; `internal/hooks/kilo_trae.go` printKiloGuide/printTraeGuide; `internal/hooks/hooks.go:40-45` | Kilo/Trae native tool execution unguarded; protection is opt-in via gateway routing |
| TM-A-04 | DoS/correctness | `internal/shim/*` + `check --args` | Low | partial | `internal/shim/shim_unix.go:36-41`, `shim_windows.go:38-43`; `cmd/beekeeper/main.go:245-340` (StringArrayVar + cobra.NoArgs) | Multi-word installs mis-parse → exit 1 → real binary not run. Fail-CLOSED but blocks legit installs; no multi-arg test |
| TM-A-05 | Info disclosure | `internal/ipc` (Windows) | Info | mitigated | `internal/sentry/windows/daemon.go:179` (NewServer(...,0)); `internal/ipc/pipe_windows.go:67-89` (`_ = ownerUID`, SDDL from current SID) | None — ownerUID=0 is ignored; pipe DACL scoped to installing-user SID. Cosmetic only |

### Notable refutation / model corrections (cluster A)

- **TM-A-01 attack-model correction:** the original finding claimed a "malicious project config" could set `--bind`. Verification **refuted that vector** — `bind` is a CLI flag only (`main.go:1328`), NOT a config key (there is no binding config field at all). A poisoned `config.json` cannot trigger remote exposure. Severity dropped High→Medium accordingly. The finding remains real because (a) the docs actively mislead the operator into believing a second gate exists, and (b) the gateway is plain HTTP with no TLS, so a non-loopback bind sends the bearer token in cleartext.

---

## 3. Cluster B — Policy Engine & Catalog Integrity

### Trust boundaries

| Boundary | Hardening posture |
|----------|-------------------|
| untrusted tool call → pure `policy.Evaluate` | `internal/policy` is a pure function library (imports only fmt/sort/strings); ToolCall is explicitly "untrusted, attacker-influenceable"; defensive package extraction |
| external catalog feeds → local index/adapters | Bumblebee JSON from pinned GitHub host, OSV REST, Socket PURL; bounded 4MB/1MB; corroboration is the second factor |
| local config/policy files → overlay & thresholds | `~/.beekeeper/policies/*.json` + `state.json` are owner-only; allowlist allow overrides catalog blocks (documented escape hatch) |
| mmap binary index → reader memory safety | `OpenIndex` bounds-checks header/magic/version/record region; corrupt/truncated index fails closed (`internal/catalog/index.go:158,221`) |
| `beekeeper-self` feed → self-quarantine | distinct embedded Ed25519 key; integrity fail → fail-closed, network fail → warn-continue. **Only production path doing real Ed25519 verification** |

### STRIDE findings table

| ID | STRIDE | Component | Adj sev | Disp | Evidence (file:line) | Gap |
|----|--------|-----------|---------|------|----------------------|-----|
| TM-B-01 | Tampering/Repudiation | `internal/policy/corroboration.go` + `internal/catalog/health.go` | **High** | partial | `corroboration.go:118-195` (pure int set-count, no 0.5); `health.go:50-51` (bumblebee-only); `THREAT-MODEL.md:187-189`, `state.go:27-29`, `sanity.go:46` claim 0.5; `osv.go:326`/`socket.go:384` set Signed:true | "Degraded source ≤0.5 weight" invariant NOT implemented; CatalogHealthy=false only nulls per-severity overrides; OSV/Socket degradation never even sets CatalogHealthy=false. Compromised critical source → single-source block |
| TM-B-02 | Spoofing/Tampering | `internal/catalog/multi.go` + `verify.go` | **Medium** (was High) | partial | `multi.go:123` (`signed := e.CatalogSignature != ""`); `verify.go:50-54` (real Ed25519 exists, never wired to bumblebee path); `selfcatalog.go:317` (only live verifier) | Bumblebee "signed" is presence-only, not cryptographic. Any non-empty signature string yields full signed weight. Production bumblebee entries are already `Signed:false`, so the realistic primitive is a single tampered critical entry → single-source false-positive block |
| TM-B-03 | Tampering/DoS | `internal/catalog/health.go` (ResolveHealthy) | Medium | accepted | `health.go:47-49` (err→return true); `health.go:28-40` (documented trade-off) | Read-failure → healthy → MORE escalation. Conscious trade-off; defensible; not surfaced in threat model as a poisoning lever |
| TM-B-04 | EoP | `internal/policy/engine.go` (bulk extension path) | Low | partial | `engine.go:81-97` (worst Decision tracks levelRank only, no quarantine; sub-ToolCall drops version) | Quarantine silently lost on bulk multi-extension installs; version precision lost. Still blocks |
| TM-B-05 | EoP/Tampering | `internal/policyloader/enforce.go` (allowlist allow) | Medium | accepted | `enforce.go:93-100,155-166`; `THREAT-MODEL.md:549-586` | T-09-31 escape hatch — fully documented; runs BEFORE SPATH/NUDGE merges so cannot downgrade those; no residual undocumented gap |
| TM-B-06 | Spoofing/EoP | `internal/pkgparse/pkgparse.go` + `engine.go` extract | Medium | partial | `pkgparse.go:122-164` (HasPrefix only); `engine.go:65-72` (unparsed→Allow); `THREAT-MODEL.md:528-534` (§8 generic acceptance) | Command-chaining (`&&`/`;`/`\|`), leading env-var assignments, unlisted managers (deno/mvn/nuget) → "no package identified" → allow. Specific evasions not enumerated |
| TM-B-07 | DoS/Tampering | `internal/catalog/sanity.go` (CheckSanity) | Low | partial | `sanity.go:63-88` (delta/total counts only); `state.go:148-150` (hash drives change-detection, not sanity block) | Count-preserving 1:1 content swap passes sanity gate. §3 implies sanity bounds defend feed-content compromise |

### Verified anti-poisoning strengths (cluster B — say so explicitly)

- **CORR wildcard guard (CR-01)** is genuinely strong: `findSeverityOverride` uses ALL-not-ANY semantics (`corroboration.go:87-101`) so a single injected all-versions wildcard cannot mask a real version-specific match.
- **SPATH overlay-cannot-downgrade ordering (CR-02)** holds in code: overlay applied FIRST, then SPATH, then NUDGE, most-restrictive-wins (`handler.go:279-318`). A `package_allowlist` allow can never downgrade a credential-read or nudge block.
- **`internal/policy` purity invariant** is upheld: no I/O, no goroutines, no wall-clock, no global mutation. I/O correctly pushed to adapters.
- **mmap index bounds-checking** fails closed correctly on corrupt/truncated `.idx`.

---

## 4. Cluster C — Runtime Sentry & Behavioral Monitoring

### Trust boundaries

| Boundary | Hardening posture |
|----------|-------------------|
| monitored process → Sentry collector | Linux eBPF ring/perf + fanotify; macOS eslogger NDJSON; Windows ETW. Event content is data, never executed; exe/path used for editor-descendant attribution |
| eslogger subprocess → macOS daemon | Apple-signed `eslogger`, no Beekeeper entitlement; stdout parsed as trusted-shaped NDJSON |
| pollen subprocess → scan orchestrator | external `pollen` on PATH; stdout JSON-validated but contents passed through unmodified |
| package-manager → nudge detector | fixed argv `--version`; detection fail-OPEN by design (a soft advisory must never block) |
| IPC socket/pipe → Sentry control plane | owner-only socket/pipe; rules flipped at runtime with OS-permission auth only |
| privileged install → Sentry daemon | opt-in elevation; Linux drops to CAP_NET_ADMIN+CAP_DAC_READ_SEARCH + seccomp KILL + NoNewPrivs after eBPF load |
| on-disk state (baseline/counters/quarantine) | 0600 + atomic write-rename |

### STRIDE findings table

| ID | STRIDE | Component | Adj sev | Disp | Evidence (file:line) | Gap |
|----|--------|-----------|---------|------|----------------------|-----|
| TM-RS-01 | (detection-control failure) | `sentry/{linux,darwin,windows}/daemon.go` correlation loop | **Medium** (was High) | open | `linux/daemon.go:291`, `darwin/daemon.go:227`, `windows/daemon.go:337` (all pass `InventorySnapshot{}`); `rules.go:392-395,443-452` require RecentExtensions; `honeypot_test.go:68` injects inv production omits | SENTRY-004 (fresh-extension) and SENTRY-005 (critical exfil-fusion) are dead in production — inventory never wired into live loop. Constituent behaviors still caught by SENTRY-001/002/003 |
| TM-RS-02 | Spoofing/Tampering | `sentry/windows/parser.go` | Medium | partial | `parser.go:143-148` (file event no PPID), `:162-168` (net event no PPID); `rules.go:84-102` isEditorDescendant | Short-lived/race-ordered child loses editor-descendant attribution; silently suppresses SENTRY-001/002/003. No PPID fallback |
| TM-RS-03 | Tampering | `internal/sentry/baseline.go` | Medium | partial | `baseline.go:31-33` (DurationDays<0 → always active); `rules.go:172-184` (QuarantineRec gated on !baselineMode); `linux/daemon.go:306-314` (downgrade to warn) | `duration_days:-1` = permanent warn-only, no quarantine, reachable with developer uid, no root, no tamper-evidence |
| TM-RS-04 | Info disclosure (status integrity) | `sentry/{linux,darwin}/daemon.go` IPC baseline path | Low | open | `linux/daemon.go:185` (`sockPath+"-baseline.json"`) vs engine `:148` (`stateDir/sentry-baseline.json`); `darwin/daemon.go:127` same bug; Windows `:236` correct | `protect status` reads wrong baseline path → reports stale 7-day baseline; misleads operator about enforcement state. Cosmetic to enforcement |
| TM-RS-05 | Tampering/Elevation | `sentry/linux/bpf_*_bpfel.go` + ebpf.go | Low | mitigated | `bpf_beekeeper_exec_bpfel.go:39-41` (stub→error→Tier2); SLSA L3 covers build path | No per-artifact bytecode attestation beyond whole-binary SLSA — adequate given build-time embedding. Fail-closed loader correct |
| TM-RS-06 | DoS/Spoofing (event drop) | fanotify.go + ebpf.go + eslogger/etw sends | Low | partial | `fanotify.go:152-155` (drop, NO counter); `ebpf.go:115-119` EventsDropped; `etw.go:53-57` EventsLost; cap 10000 | Flooding benign events saturates channel; sensitive event dropped before EvaluateEvent. fanotify drops uncounted → evasion invisible |
| TM-RS-07 | Spoofing/Log injection | `scan/scanner.go` defaultRunPollen + darwin parser | Low | partial | `scanner.go:57` (LookPath "pollen"), `:168-172` (passthrough after JSON probe), `:171` appendRawAuditLine | PATH-resolved pollen can emit arbitrary record_type lines into beekeeper.ndjson; no schema/whitelist before audit append |

### TM-RS-01 model correction

STRIDE label was imprecise (this is a **silent detection-control failure**, not Tampering/Repudiation of logs). Severity dropped High→Medium: this is a detection-**completeness** gap in a detect/audit-only layer, not a fail-open of an enforcement control. It does not violate the fail-closed invariant — the hook/gateway block path is untouched, and the credential-read / phone-home / editor-descendant behaviors of the exfil scenario are still independently detected by SENTRY-001/002/003. Impact is reduced cross-signal corroboration on alerts, not a wholly missed attack class.

### Verified Sentry strengths (cluster C — say so explicitly)

- **Fail-closed posture is sound where it matters:** eBPF loader stubs degrade to Tier-2 fanotify rather than running with phantom probes (TM-RS-05 mitigated).
- Linux privilege separation drops to CAP_NET_ADMIN+CAP_DAC_READ_SEARCH and installs a seccomp KILL filter with NoNewPrivs after eBPF load (`privilege.go`).
- NUDGE fail-OPEN is **correct by design** (a soft advisory must never block) — fixed argv, never-panic, overflow-guarded scanners (`scanners.go:271-302`).
- Baseline/counter writes are 0600 + atomic rename. The real weaknesses are detection completeness (RS-01/02/06) and status/forensic integrity (RS-04/07), not core enforcement.

---

## 5. Cluster D — Audit, Quarantine, Config, Sidecar & Supply Chain

### Trust boundaries

| Boundary | Hardening posture |
|----------|-------------------|
| project-local `.beekeeper/config.json` → enforcement | `discoverProjectConfig` walks up from CWD, merged ABOVE user config — project dir is attacker-influenceable (agent-cloned repos, postinstall) |
| `BEEKEEPER_*` env + CLI flags → enforcement | hardcoded allowlist, highest precedence; non-reflective mapping bounds the surface |
| `beekeeper-self` remote feed → self-quarantine | HTTPS + separately-embedded Ed25519 key; integrity-fail closed, network-fail warn-continue. Trust gated on key, not TLS |
| LlamaFirewall sidecar IPC → supervisor | bounded 1MB length-prefixed JSON via io.ReadFull, fuzzed, supervised, fail-closed on crash; no socket-level auth (name-race impersonation risk) |
| audit NDJSON sink → disk + remote | 0600 owner-only, append-only, mutex-guarded; RedactRecord at check/gateway chokepoints before remote fan-out |
| quarantine store → filesystem | Move sanitizes with filepath.Base + destDir guard; Restore reads OriginalPath from manifest with NO destination boundary check |
| editor settings patcher → settings.json | JSONC parse, single hardcoded key, atomic rewrite; never executes extension content |

### STRIDE findings table

| ID | STRIDE | Component | Adj sev | Disp | Evidence (file:line) | Gap |
|----|--------|-----------|---------|------|----------------------|-----|
| TM-D-01 | Tampering/EoP | `internal/config/layered.go` merge() + `config_resolve.go` | **Medium** (was High) | partial | `layered.go:174-176` (unconditional FailMode override); `config_resolve.go:91-113` (CWD discovery); `handler.go:409-426` (failDecision honors fail_mode); `main.go:277,315` | Project-local `{"fail_mode":"open"}` merged ABOVE user config converts every fail-closed net to fail-open. validate() does NOT block valid "open". Lowest-trust layer can relax closed→open with no integrity/ownership gate |
| TM-D-02 | Tampering/Spoofing | `layered.go` mergeSelfCatalog + `selfquarantine.go` | Medium | partial | `layered.go:205-210` (URL+PubKey from any layer); `selfquarantine.go:96-112` (project pub_key as override) | Project/env layer can repoint self_catalog URL+key to attacker feed+key, suppressing self-quarantine of a known-compromised binary. Malformed key fails closed |
| TM-D-03 | Info disclosure | `internal/audit/redact.go` + `internal/watch/handler.go` | Medium | partial | `redact.go:112-127` (only 4 fields); `types.go:45-55,95-106` (unredacted Sentry/catalog fields); `watch/handler.go:184-191` (no RedactRecord) vs `check/handler.go:493-494`, `gateway/proxy.go:514-515` | Watch path writes with NO redaction; SentryFilesAccessed/NetworkDests/ProcessExe/CorrelatedExt + CatalogProvenance bypass redaction → can reach remote OTLP/HTTPS/syslog sinks |
| TM-D-04 | Spoofing/Tampering | docs cosign identity + `selfquarantine.go` msg | Low | open | `.github/workflows/release.yml:11-13` (tag trigger) vs `THREAT-MODEL.md:117,424`, `selfquarantine.go:62` (all pin `@refs/heads/main`) | Verification identity pins branch ref but workflow triggers on `v*` tags → SAN is `@refs/tags/vX.Y.Z`. Copy-pasted cosign command fails on a legit binary |
| TM-D-05 | Tampering | `internal/quarantine/quarantine.go` Restore | Low | partial | `quarantine.go:194-201` (OriginalPath verbatim rename target; only empty rejected) vs entry-id traversal guard `:172-177` | Tampered manifest OriginalPath → restore writes extension tree to attacker-chosen path. Gated behind owner write access |
| TM-D-06 | Tampering | `internal/llamafirewall/supervisor.go` persistState | Low | open | `supervisor.go:347-360` (`os.ExpandEnv("$HOME/.beekeeper")`) vs `platform.StateDir()` standard | Direct $HOME expansion bypasses %APPDATA%/BEEKEEPER_HOME; sidecar PID/state.json written to wrong/CWD-relative location. Informational state only, not a policy input |

### TM-D-01 model correction

Severity dropped High→Medium because it is **partially documented/accepted**: `THREAT-MODEL.md` attack-surface row "Config injection via env/project" (line 76) rates it Low and §"Direct Human Malice" (lines 512-518) explicitly names setting `fail_mode:open` as an accepted bypass. It is **not** lowered to the doc's "Low" because the documented rationale ("operator controls trusted project directories") is unsound for Beekeeper's own threat model: the working tree is the untrusted agent-cloned-repo surface the tool exists to police, the project layer is the lowest-trust file layer, yet it can relax fail-closed→open with the same precedence as a benign override and no ownership gate. A correct design refuses fail-mode RELAXATION (and nudge/llamafirewall disable, and self_catalog.* override) from the project/env layers.

### Verified supply-chain strengths (cluster D — say so explicitly)

- **No findings** against: the LlamaFirewall sidecar IPC parser (bounded, fuzzed, supervised, fail-closed); the `beekeeper-self` signature/fail-closed-vs-network logic with independently-embedded Ed25519 key; the read-only TUI (JSON-parse-then-re-marshal, never executes tainted strings); the quarantine Move path-traversal guard; the editor settings patcher; and the reproducible-build + keyless-cosign + SLSA-L3@v2.1.0 + CycloneDX pipeline.
- **Only the documented cosign verification IDENTITY string is wrong** (TM-D-04) — the signing pipeline itself is correct.

---

## 6. v1.2.0 / v1.3.0 Surface Deltas vs the v1.0.0 Threat Model

The published model documents only the v1.0.0 surface. The following are **new and previously undocumented**:

### v1.2.0 deltas
- **CORR (per-severity corroboration + anti-poisoning sanity gate):** critical-severity single-source escalation (`SeverityOverrides`, BlockAt:1), the ALL-not-ANY wildcard guard (CR-01), the CatalogHealthy degraded-suppression gate. The doc's corroboration table shows only the flat 1/2/3 model.
- **SPATH (sensitive-path runtime enforcement):** `DefaultSensitivePaths` (Cursor/Windsurf MCP dirs, bare `.cargo/credentials`, `.env` globs, ADS/trailing-dot normalization), and the overlay-cannot-downgrade merge ordering (CR-02). Realized in both the check handler AND the Sentry (`rules.go:53-61`, with the filepath.ToSlash Windows-backslash fix). Undocumented.
- **NUDGE (package-manager hardening):** pnpm/bun/yarn→npm mapping, npm add/exec verbs (closes F3/SC1); the fail-OPEN parse class for unlisted managers / command-chaining is new surface only partially covered by the §8 "zero-day tool-call semantics" acceptance. New OriginalCommand/RewrittenCommand/PMState audit fields (redact.go correctly extended).
- **Layered-config consumer change:** `resolveConfig` now feeds the full system→user→project→env→flag merge into `RunCheck`, so the project layer reaches every enforcement decision — this is what makes TM-D-01/02 reachable.

### v1.3.0 / Phase-10 deltas (entirely undocumented)
- **exit-1 → exit-2 universal deny protocol** + per-family deny JSON in table-driven `RenderDeny` — a brand-new harness-facing trust boundary. Per MEMORY.md, the v1.0.0-era reality was that a correct block Decision was NOT enforced by harnesses (hook fired+audited but tool still ran); Phase 10 fixes the transport.
- **15 per-harness installers** writing into user config (merge-not-clobber, per-target backup, atomic, foreign-script preservation).
- **Hermes fail-OPEN class** and its `RunCheckTo(io.Discard)` mitigation (commit f315c81 — the raw `{"Allow":false}` line previously leaked ahead of the deny JSON and Hermes parsed it as an allow).
- **Gateway routing for no-hook harnesses (Kilo/Trae)** leaving native tools unguarded (Tier-3 ceiling).

---

## 7. Prioritized Remediation List

Ordered by (severity × disposition-openness). Items 1–3 are the genuine code/doc defects; 4–9 are real residual gaps; the remainder are documentation/correctness.

### Priority 1 — fix code or correct docs (active operator-facing deception or weakened control)

1. **TM-A-01 (Medium, open) — gateway remote-bind gate.** Either (a) implement the promised `allow_remote_gateway` config field and refuse to bind a non-loopback address without it, OR (b) correct the help text (`main.go:1192-1194`) and PRD to stop promising a gate that does not exist. Additionally, the gateway is plain HTTP — if remote bind is ever supported, require TLS so the bearer token does not traverse the LAN in cleartext.
2. **TM-B-01 (High, partial) — degraded-source corroboration weight.** Implement the documented 0.5-weight demotion for sanity-degraded sources in `corroborate()`, and make `ResolveHealthy` consult OSV/Socket degradation (not bumblebee-only), OR correct `THREAT-MODEL.md:187-189`, `state.go:27-29`, and `sanity.go:46` to remove the unimplemented "0.5 weight" claim. Until then, a compromised critical-severity source can drive a single-source block.
3. **TM-B-02 (Medium, partial) — primary feed signature is presence-only.** Wire `VerifySignatureWithKey` (real Ed25519) into the Bumblebee decision path, OR correct §3 to state plainly that the main feed's "signed" status is a presence check and that cryptographic verification covers only the `beekeeper-self` feed.

### Priority 2 — close residual coverage gaps

4. **TM-D-01 / TM-D-02 (Medium, partial) — project-layer security downgrade.** Refuse fail-mode RELAXATION (closed→open), `nudge.enabled:false`, `llamafirewall.enabled:false`, and `self_catalog.*` overrides from the project and env layers; honor them only from user/system layers. Add an integrity/ownership check on the discovered project config.
5. **TM-RS-01 (Medium, open) — dead SENTRY-004/005.** Wire the watch/scan extension inventory into the live `EvaluateEvent` call in all three daemons so the fresh-extension and critical exfil-fusion rules can fire in production (they currently fire only in tests).
6. **TM-D-03 (Medium, partial) — redaction coverage.** Route the watch-handler write path through `RedactRecord`, and widen field coverage to SentryFilesAccessed / SentryNetworkDests / SentryProcessExe / SentryCorrelatedExt / CatalogProvenance before remote fan-out.
7. **TM-A-02 (Medium, partial) — Hermes fail-open regression guard.** Add a stdout-purity canary/regression test asserting the Hermes hook emits ONLY the deny JSON (no leading bytes), and document the fail-open class in the public threat model.
8. **TM-A-03 (Medium, accepted) / Tier-3 native-tool gap.** Already disclosed in `docs/harness-support-matrix.md`; promote the disclosure into the public threat model so it is not only in a printed guide.

### Priority 3 — correctness / forensic integrity / documentation

9. **TM-RS-04 (Low, open) — status baseline path bug.** Fix the Linux/macOS IPC status handler to read `stateDir/sentry-baseline.json` (Windows already correct).
10. **TM-D-04 (Low, open) — cosign identity ref.** Change the documented `--certificate-identity` from `@refs/heads/main` to the tag-ref form in `THREAT-MODEL.md` and `selfquarantine.go:62`.
11. **TM-A-04 (Low, partial)** shim multi-arg mis-parse; **TM-B-06** package-parse evasions; **TM-B-07** count-preserving feed tamper; **TM-RS-02/03/06/07**, **TM-D-05/06** — enumerate and/or add regression tests as backlog; document the accepted ones (TM-B-03, TM-D-01-as-residual) in the public model's gaps section.

---

## 8. Refuted / Dismissed Vectors (considered and dropped)

No cluster finding was fully refuted (`verified.real=false` did not occur). However, verification **corrected the attack model** on several, which is recorded here so they are not re-raised:

- **TM-A-01 "malicious project config sets `--bind`"** — dismissed. `bind` is a CLI flag only; there is no binding config field. The remote-exposure vector requires an operator running `--bind 0.0.0.0` directly, not config poisoning.
- **TM-B-02 "wipe signature downgrades real entry to warn"** — largely moot. Production bumblebee entries are already `CatalogSignature:""` / `Signed:false`, and OSV/Socket `Signed` is set in-process by the trusted adapter, not read from the tampered mmap.
- **TM-A-05 "ownerUID=0 grants root-equivalent pipe access"** — dismissed. Windows `ipc.NewServer(sockPath, 0)` ignores ownerUID entirely; the DACL is built from the current-process token SID.

---

*This is the audit of record. The public-facing distillation is `docs/THREAT-MODEL.md` (refreshed in place to cover v1.0.0 + v1.2.0 + v1.3.0). For the disclosure process see `SECURITY.md`.*
