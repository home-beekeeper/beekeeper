# Sentry coverage analysis: 2026 attack patterns vs current rule set

**Date:** 2026-06-10
**Scope:** Beekeeper's runtime Sentry behavioral-correlation engine (`internal/sentry/`) measured against the documented 2026 supply-chain attack catalog.
**Method:** rules and event sources extracted verbatim from the shipped code, not from the PRD prose. Coverage is graded against what the code actually does. No credit-grading.

---

## 0. Grounding — what the code actually implements (read this first)

The external task brief describes a "v1.0 rule set" as `R1`–`R5`. The shipped engine uses IDs `SENTRY-001`–`SENTRY-005` (`internal/sentry/rules.go`). They map almost 1:1 to `R1`–`R5`, but there are **material discrepancies between the idealized brief and the real code**, and the matrix below is graded against the real code:

| Brief | Code | Material discrepancy (honest) |
|-------|------|-------------------------------|
| R1 ext-host credential cluster (≥2 sensitive reads/60s) | **SENTRY-001** `critical` | Watchlist is **11 fixed substrings** (`.ssh/ .aws/ .gnupg/ .config/Claude/ .config/op/ .config/gh/ .netrc .npmrc .pypirc .cargo/credentials .env`). Misses `~/.config/gcloud`, `~/.azure`, `~/.kube`, `~/.docker/config.json`. |
| R2 credential CLI burst (≥2 cred CLIs/60s) | **SENTRY-002** `critical` | Matches CLI **base name only** (`gh aws op vault npm gcloud az heroku fly vercel netlify supabase`), not subcommand (`gh auth`). A benign `gh`+`npm` pair trips it. |
| R3 phone-home to **non-allowlisted** domain within 10min of **extension activation** | **SENTRY-003** `high` | **No domain allowlist exists.** **Not tied to extension activation.** It fires on the **first outbound TCP connection** by any editor-descendant in a 10-min window — i.e. it is both noisier (fires on legit github/registry traffic) and blind to *which* domain. |
| R4 fresh-extension correlation (+ Bumblebee-tracked ext installed <30min) | **SENTRY-004** `high` | "Installed" = an extension **directory mod-time** within the look-back, from a 30s inventory rescan — not a real install event and not Bumblebee-cross-referenced. `QuarantineRec` always false. |
| R5 exfil-signature-fusion (read+conn+fresh-ext /5min) | **SENTRY-005** `critical` | As described, gated on the same mod-time inventory + 11-path watchlist. |

**Three facts dominate every verdict below:**

1. **Editor-descendant gate.** Every rule first calls `isEditorDescendant(pid, tree)` — the PID must be, or descend ≤32 PPID hops from, a process named `code / code-insiders / cursor / windsurf / codium` (`rules.go:38-44, 84-102`). **Anything not running inside one of those five editors is invisible to Sentry.** That excludes: agent CLIs in a standalone terminal (the common case for Claude Code / Codex / Gemini CLI), CI/CD runners, and system daemons. JetBrains, Zed, Emacs, Neovim, etc. are also unseen.

2. **Three event kinds only** (`types.go:18-25`): `EventProcessCreate` (exec), `EventFileAccess` (read/open on the 11-path watchlist), `EventNetworkConnect` (outbound TCP connect). **Not ingested:** DNS queries, process-memory reads (`/proc/<pid>/maps`, ptrace), file **writes** to persistence locations, registry/package-publish events, locale.

3. **Detection-only.** The Sentry daemon's sole action on a fire is `auditWriter.Write(rec)` (`linux/daemon.go:330`, mirrored darwin/windows). It never quarantines, kills, or pushes a live alert. (The separate **unprivileged editor-extension watcher** in `internal/watch/` *does* physically quarantine a catalog-known extension pre-execution and is test-proven — but that is the catalog/watch layer, not the Sentry correlation engine, and it acts on catalog identity, not on SENTRY-00x behavior.)

> **Stale-doc correction used in this analysis:** `docs/THREAT-MODEL.md` §8 still says SENTRY-004/005 "do not fire in production today" and "the Linux fanotify path does not even count drops." Both are now false in code (all three daemons build a live `InventoryStore` and pass `Snapshot(now)` into `EvaluateEvent`; fanotify counts drops via `atomic.AddUint64(&EventsDropped,1)`). This doc grades against the code; a PRD diff to fix THREAT-MODEL §8 is in Section 6.

---

## 1. Coverage matrix

Grades: **FULL** = fires reliably on the core behavior; **PARTIAL** = catches one half (read *or* exfil) or only under a narrow precondition; **NONE** = no SENTRY rule fires.

| # | Attack pattern | Grade | Honest reason (against real code) |
|---|----------------|-------|-----------------------------------|
| A1 | **Nx Console** (editor-extension trojan; reads ~/.aws,~/.ssh,~/.config/op,~/.config/Claude,~/.npmrc; HTTPS+DNS+GitHub exfil; Bun LOLBin) | **FULL** | This *is* the rule set's design target. ≥2 watchlist reads from an extension-host descendant → SENTRY-001 (critical); first outbound → SENTRY-003; read+conn+recent-ext → SENTRY-005 (critical). Caveat: the DNS-tunneling exfil leg specifically is **not** seen (no DNS source); the HTTPS/GitHub legs are caught generically by SENTRY-003/005. |
| A2 | **VS Code task.json folderOpen persistence** | **PARTIAL** | The persistence *mechanism* — a write to `.vscode/tasks.json` — is invisible (no file-write watch). If the payload that runs on folder-open is editor-descended and reads ≥2 watchlist paths / makes an outbound, SENTRY-001/003/005 fire on the *behavior*. The install/persistence is missed; the runtime behavior is partially caught. |
| B1 | **TanStack Mini Shai-Hulud** (OIDC token scraped from CI runner memory; npm trusted-publisher federation; self-propagating) | **NONE** | CI runner is not editor-descended → gate fails. No memory-read event. Exfil/publish is to npm via OIDC federation (registry-side). Nothing fires. |
| B2 | **Miasma / RedHat Insights** (npm preinstall hook; harvests AWS/GCP/Azure/K8s; AWS-service C2) | **PARTIAL→NONE** | If `npm install` runs **inside a VS Code integrated terminal**, the preinstall hook is editor-descended and a `.aws` read + outbound *could* trip 001/003/005 — but only `.aws` is on the watchlist (GCP/Azure/K8s are not), so the ≥2 threshold often isn't met, and AWS-service C2 is to legitimate endpoints SENTRY-003 can't distinguish. Run in any non-editor terminal (the norm) → **NONE**. The real defense here is the **catalog corroboration / nudge layer pre-install**, not Sentry. |
| B3 | **Shai-Hulud 2.0** (`/proc/<pid>/maps` memory scrape; GitHub-repo dead-drop exfil; malicious workflow inject) | **NONE** | No memory-read event; CI not editor-descended; exfil to api.github.com is host-indistinguishable from `git push`. |
| C1 | **intercom-client@7.0.4** (preinstall hook; Bun LOLBin; cloud+secrets harvest; `.claude/` poisoning) | **PARTIAL→NONE** | Same as B2: incidental catch only if editor-terminal-descended and ≥2 watchlist paths hit; `.claude/` write-poisoning is invisible. Catalog layer is the actual defense for this known package. |
| C2 | **node-ipc credential stealer** (postinstall; 7 malicious versions) | **PARTIAL→NONE** | Same install-time-hook reasoning. Known versions → caught by catalog corroboration, not Sentry. |
| D1 | **Claude Code SessionStart hook injection** (writes `~/.claude/settings.json`) | **NONE** | A file **write** to `~/.claude/settings.json`; Sentry watches **reads** on a different path list. (Beekeeper's *check-layer self-protection* blocks an agent from writing its own hook entry — a real defense, but not Sentry.) |
| D2 | **`.claude/` directory poisoning** | **NONE** | File write to project `.claude/`; not a watched event. |
| E1 | **gh-token-monitor** (LaunchAgent / systemd-user persistence daemon; GitHub poll; `rm -rf` on revoke) | **NONE** | Persistence install = a write to `~/Library/LaunchAgents` / systemd-user unit; not watched. No persistence-daemon rule. The destructive `rm -rf` is also unseen (no such rule). |
| F1 | **DNS TXT exfiltration** (shopsprint/decimal Go typosquat) | **NONE** | No DNS event source; SENTRY-003 watches TCP connect, not UDP/53 queries. Also not editor-descended (Go module install). |
| F2 | **GitHub API dead-drops** (public repos as drops via stolen token) | **NONE (residual)** | Host-layer indistinguishable from legitimate `git push` to api.github.com. SENTRY-003 has no allowlist but also no way to tell drop from push. Architectural-only mitigation. |
| F3 | **Session messenger** (blockchain network exfil) | **PARTIAL** | If editor-descended and correlated with a cred read, the *outbound* is visible to SENTRY-003/005; the channel's identity/content is not. Catches "an exfil-shaped connection happened," not "this is the messenger." |
| F4 | **AWS legitimate-service C2** (SQS/S3/Lambda) | **NONE (residual)** | Outbound to legitimate AWS endpoints; not distinguishable from real AWS use at the host layer. |
| G1 | **Shai-Hulud read-self worm propagation** (republish to writable npm pkgs; no C2) | **NONE** | Not editor-descended; propagation is an npm publish to registry.npmjs.org (registry-side, legit endpoint). |
| G2 | **Russian-locale-exit fingerprint** | **NONE** | Locale is not an ingested signal. Listed for completeness; unreliable as a primary signal even if added. |
| H1 | **TrapDoor Crypto Stealer** (npm/PyPI/Cargo; 28 pkgs) | **NONE** | Package-registry campaign; install-time, not editor-extension. Defense is **catalog corroboration** (cross-ecosystem), not Sentry. |
| H2 | **GemStuffer** (123 RubyGems) | **NONE** | As H1 — catalog layer, not Sentry. |
| H3 | **Laravel Lang** (4 Composer pkgs; maintainer-account compromise) | **NONE** | As H1 — catalog layer, not Sentry. |

**Tally (Sentry only):** FULL 1 · PARTIAL 4 · NONE 14. The single FULL is the editor-extension-trojan family the rules were built around. This is expected and honest: **v1 Sentry is an editor-extension exfiltration detector, not a general runtime IDS.** Several NONE rows are nonetheless covered by *other* Beekeeper layers (catalog corroboration for known packages; check-layer self-protection for `settings.json`) — those are noted but not credited to Sentry.

---

## 2. Gap analysis (grouped by root cause)

**G-A. Process-tree gate too narrow (editor-descendant only).**
Excludes the most common real execution contexts. Patterns: B1, B3, G1, H1–H3, and the "→NONE" half of B2/C1/C2 (agent or install run outside a recognized editor). Root fix: a second ancestry mode that also recognizes **agent CLIs** and (optionally) **CI runner** roots, not just the five editors.

**G-B. No file-WRITE watch on persistence locations.**
Patterns: A2 (`.vscode/tasks.json`), D1 (`~/.claude/settings.json`), D2 (`.claude/`), E1 (LaunchAgents / systemd-user). The Sentry file source is read/open-only on a credential list. Persistence is the whole attack here and it is invisible.

**G-C. No DNS query event source.** Pattern: F1. TCP-connect ingestion cannot see DNS-TXT tunneling.

**G-D. No process-memory read event source.** Patterns: B1, B3. OIDC/secret scraping from `/proc/<pid>/maps` (or `process_vm_readv`/ptrace) is the 2026 CI-attack signature and is wholly unseen.

**G-E. Credential watchlist too small.** Patterns: B2, C1 (cloud harvesters). Missing `~/.config/gcloud`, `~/.azure`, `~/.kube`, `~/.docker/config.json`, `~/.config/Claude` is present but `~/.claude/` (the newer location) is not.

**G-F. Allowlisted / legitimate-endpoint exfil is host-invisible.** Patterns: F2 (GitHub API), F4 (AWS-service C2), G1 (npm registry). Not closeable with host rules — see Section 5.

**G-G. Detection-only; no enforcement from Sentry rules.** Even on a `critical` SENTRY-005, the daemon only writes an audit record (`QuarantineRec` is a flag nothing consumes). Containment exists only in the separate extension watcher for catalog-known extensions.

**G-H. Not-a-Sentry-problem (covered elsewhere).** Install-time package campaigns (B2/C1/C2/H1–H3 known versions) are the **catalog corroboration** layer's job; `settings.json` self-injection (D1) is the **check-layer self-protection** layer's job. Listed so the threat model doesn't double-count them as Sentry gaps.

---

## 3. New rule proposals (closeable on the EXISTING three event streams)

These need no new event source — only the process/file/network streams already ingested (plus, where noted, a watchlist expansion).

### SENTRY-006 — Agent-descendant credential cluster — severity `critical`
- **Trigger:** identical to SENTRY-001 (≥2 sensitive-path reads within 60s by one PID) **but gated on `isAgentDescendant`** — PID descends from a known agent CLI process (`claude`, `codex`, `cursor-agent`, `gemini`, `copilot`, `qwen`, `aider`, `opencode`, `hermes`) instead of an editor. Reuse the existing `CredAccessByPID` window machinery.
- **Severity:** critical. **FP bound:** an agent legitimately reading ≥2 distinct credential paths in 60s is rare; same threshold/window proven for SENTRY-001. Add an allowlist for the agent's own config dir to suppress self-reads.
- **Newly covers:** the "→NONE" halves of B2/C1/C2 when the install runs under an agent in a plain terminal (the common case); broadens A1 to agent-launched extension payloads.

### SENTRY-007 — Generalized exfil fusion (no fresh-extension precondition) — severity `critical`
- **Trigger:** editor- *or* agent-descendant PID performs ≥1 sensitive-path read AND ≥1 outbound TCP connect to a **non-loopback, non-RFC1918** destination within 5 min — **without** requiring a freshly-installed extension (SENTRY-005 requires the extension; this generalizes it for dormant trojans and install-time harvesters).
- **Severity:** critical. **FP bound:** higher than SENTRY-005 (drops the extension corroborant), so ship it **warn-first in baseline mode** and require the external-destination filter to cut legit localhost/LAN traffic. Suppress when the only outbound is to a configured update/telemetry allowlist.
- **Newly covers:** F3 (messenger exfil after cred read), the exfil half of B2/C1, dormant-trojan exfil that A1/A2's extension precondition would miss.

### SENTRY-008 — Persistence-location write — severity `high`
- **Trigger:** editor- or agent-descendant PID **writes/creates** a file under a persistence watchlist: `~/Library/LaunchAgents/`, `~/.config/systemd/user/`, `**/.vscode/tasks.json`, `**/.vscode/settings.json`, `~/.claude/settings.json`, `**/.claude/**`.
- **Severity:** high (warn). **FP bound:** developers and legit tools do write these; warn-only + dedupe per path per session. Pair with the existing check-layer block on `settings.json` self-edits to escalate when the writer is an agent.
- **Newly covers:** A2, D1, D2, E1 — **but only once file-WRITE ingestion lands on those paths** (Section 4, G-B). On macOS/Windows the write event type already exists (eslogger `create`, ETW Kernel-File 12/14/15); on Linux it needs a fanotify mark change. So SENTRY-008's *rule* is free; its *fuel* is a partial ingestion task.

### Watchlist expansion (config change, not a rule)
Add to `defaultSensitivePaths` (`rules.go:12-16`): `~/.config/gcloud`, `~/.azure`, `~/.kube/config`, `~/.docker/config.json`, `~/.claude/` (newer Claude dir). Immediately strengthens SENTRY-001/005/006/007 against B2/C1 cloud harvesters. Zero new event source.

---

## 4. Event-ingestion gaps (need a NEW event source) — v1.x roadmap fragment

| Missing source | Linux mechanism | macOS mechanism | Windows mechanism | Closes | Milestone |
|----------------|-----------------|-----------------|-------------------|--------|-----------|
| **File-WRITE on persistence paths** | fanotify: add `FAN_CREATE\|FAN_MODIFY` marks on persistence dirs (extends existing `FAN_ACCESS\|FAN_OPEN_PERM` setup, `fanotify.go:47`) | eslogger: extend the existing `create`/`open` subscription with `ES_EVENT_TYPE_NOTIFY_WRITE` on the persistence dirs | ETW Kernel-File write IDs 12/14/15 already ingested — add persistence paths to the watchlist | A2, D1, D2, E1 (fuels SENTRY-008) | **v1.x** (mostly watchlist + small Linux mark change; lowest effort, highest coverage gain) |
| **DNS query events** | eBPF: kprobe on `udp_sendmsg`/`udp_recvmsg` to :53, or a uprobe on `getaddrinfo`, or an XDP/socket filter parsing DNS | No native eslogger DNS event — requires a NetworkExtension DNSProxy provider (heavy) → **v2** | ETW provider **Microsoft-Windows-DNS-Client** `{1C95126E-7EEA-49A9-A3FE-A378B03DDB4D}` | F1 (DNS-TXT exfil) | **v1.x** Linux + Windows; macOS **v2** |
| **Process-memory read / ptrace** | eBPF LSM hook (`ptrace_access_check`) or kprobe on `process_vm_readv` + watch `open()` of `/proc/<pid>/maps` | EndpointSecurity `ES_EVENT_TYPE_NOTIFY_PROC_CHECK` / `task_for_pid` — **requires the ES entitlement** (already v2 per PRD) | Kernel callback / ETW for `OpenProcess(VM_READ)` is hard and noisy | B1, B3 (OIDC/secret memory scrape) | **v1.x** Linux (eBPF/LSM); macOS **v2** (entitlement); Windows **v2/v3** |

Priority order by coverage-per-effort: **file-write persistence (v1.x, do first) → DNS Linux/Windows (v1.x) → memory-read Linux (v1.x) → macOS/Windows parity (v2).**

---

## 5. Acknowledged residual gaps (NOT host-detectable at all)

These cannot be closed by any host-layer rule or event source. The only honest answer is architectural mitigation, and that is out of Beekeeper's host-agent scope:

- **GitHub API dead-drops (F2)** — exfil to `api.github.com` is byte-indistinguishable from a legitimate `git push`/API call. Mitigation: scoped/short-lived tokens, token-to-workflow binding, network egress allowlists. **Not a Sentry rule.**
- **AWS-/cloud-service C2 (F4, Miasma)** — C2 over SQS/S3/Lambda to legitimate AWS endpoints. Mitigation: AWS Data Perimeter / egress allowlists / VPC endpoints. **Not host-detectable.**
- **npm-registry worm propagation & trusted-publisher OIDC abuse (G1, B1)** — propagation/publish to `registry.npmjs.org` via legitimate federation. Mitigation: registry-side provenance, mandatory 2FA, trusted-publisher hardening. **Registry/CI problem, not host.**
- **Sophisticated prompt injection that produces individually-benign tool calls** — already acknowledged in THREAT-MODEL §8; Sentry's behavioral correlation raises the bar but cannot guarantee detection.

---

## 6. PRD + home-page edits (before/after diffs)

> The brief references "PRD Section 5.5", "Section 11.2", and `beekeeper-home.html`. **None of those exist.** The real targets are `.planning/PROJECT.md` (`#### Sentry Daemon`, `### Out of Scope`), `docs/THREAT-MODEL.md` §8, and `web/components/home/*`. Diffs below target the real files.

### 6.1 `.planning/PROJECT.md` — `#### Sentry Daemon (protected-mode, opt-in)` (line ~115)

```diff
- Default rule set: extension-host credential cluster, credential CLI burst, phone-home, fresh-extension behavior correlation, exfil signature fusion
+ Default rule set (SENTRY-001..005), each gated on editor-descendant ancestry
+ (code/cursor/windsurf/codium): SENTRY-001 credential-file cluster, SENTRY-002
+ credential-CLI burst, SENTRY-003 first-outbound phone-home (no domain allowlist
+ in v1), SENTRY-004 fresh-extension correlation, SENTRY-005 exfil-signature fusion.
+ Sentry is DETECTION-ONLY (writes audit records; it does not quarantine or kill —
+ extension quarantine lives in the unprivileged watch/scan layer). Scope is the
+ editor-extension-trojan family; agent CLIs in standalone terminals and CI runners
+ are NOT seen by Sentry in v1 (see Out of Scope).
```

### 6.2 `.planning/PROJECT.md` — `### Out of Scope` (line ~152) — add:

```diff
+ - Sentry coverage of non-editor execution contexts (standalone-terminal agent
+   CLIs, CI/CD runners, system daemons) — v1 Sentry is editor-descendant-gated;
+   broader ancestry is SENTRY-006 (v1.x).
+ - Persistence-write detection (.vscode/tasks.json, ~/.claude/settings.json,
+   LaunchAgents, systemd-user) — needs file-write ingestion (v1.x).
+ - DNS-tunneling and process-memory-scrape detection — new event sources (v1.x/v2).
+ - Exfil over legitimate/allowlisted endpoints (GitHub API, AWS services, npm
+   registry) — host-undetectable; architectural mitigation only.
```

### 6.3 `docs/THREAT-MODEL.md` §8 — `#### Detection-Completeness Gaps in the Behavioral Sentry` (line ~658)

```diff
- Two cross-signal correlation rules (fresh-extension correlation and the critical
- read-creds + fresh-extension + phone-home exfiltration-fusion rule) require an
- extension-inventory snapshot that the shipped daemons do not yet build, so those
- two rules do not fire in production today; ... the Linux fanotify path does not
- even count drops ...
+ SENTRY-004/005 now fire in production: all three daemons build a live extension
+ InventoryStore (mod-time-based) and pass Snapshot(now) into EvaluateEvent, and the
+ Linux fanotify path counts drops (EventsDropped). The remaining honest gaps are:
+ (1) Editor-descendant scope — every rule requires ancestry from code/cursor/
+ windsurf/codium, so agent CLIs in a standalone terminal, CI runners, and system
+ daemons are not monitored. (2) SENTRY-003 has no domain allowlist and is not tied
+ to extension activation — it fires on the first outbound by any editor-descendant,
+ so it is noisy and cannot identify the destination. (3) No file-write, DNS, or
+ process-memory event sources, so persistence injection (.vscode/tasks.json,
+ ~/.claude/settings.json, LaunchAgents), DNS-TXT tunneling, and /proc/<pid>/maps
+ secret-scraping are undetected. (4) Sentry is detection-only; no rule triggers
+ automated containment. (5) On Windows, file/network events carry no parent-PID,
+ weakening the editor-descendant attribution the rules depend on.
```

### 6.4 Home-page overstatement flags (`web/components/home/*`)

| File:line | Current (overstates) | Proposed (honest) |
|-----------|----------------------|-------------------|
| `how-it-works.tsx:177-178` | "...correlates process, file, and network behavior into the exfiltration signature, **catching novel campaigns the catalog has never seen.**" | "...correlates process, file, and network behavior of **editor-extension hosts** into the exfiltration signature, flagging extension-trojan exfil patterns **before the catalog has flagged them**." |
| `feature-cards.tsx:52-54` | "Correlates process events, credential file access, and outbound network connections into the exfiltration signature. **Fires regardless of catalog knowledge.**" | "...into the exfiltration signature for **editor-extension activity**. Catalog-independent, **detection-only** (alerts/audits; does not auto-contain)." |
| `hero.tsx:50-53` | "A hijacked agent **cannot act on your machine without Beekeeper's permission.**" | Scope to the enforcement layer: "Every tool call passes the fail-closed **hook/gateway** before it runs." (The blanket claim is true for the hook layer, not for Sentry detection — keep it about the layer that actually blocks.) |
| `honesty-callout.tsx` (omission) | Lists Hermes/Tier-3/release_age/gateway gaps but **no Sentry gap.** | Add one gap: "Sentry behavioral detection is editor-extension-scoped and detection-only — it does not watch standalone-terminal agents, CI runners, persistence writes, DNS, or process memory." |

**Worst offenders (untracked prototypes, not the live site):** `beekeeper-docs.html:1016` ("Best at catching: zero-day campaigns, novel malware, anything pre-disclosure") and `:867-868` ("Fires regardless of what catalogs know"). These use stale `R5` naming and the un-caveated "zero-day / anything pre-disclosure" framing. They are `git status ??` (untracked) — recommend deleting them or aligning them with the language above before they ever ship.

---

## 7. Bottom line

v1 Sentry does exactly one thing well — detect the **Nx-Console-class editor-extension exfiltration pattern** — and the home page currently implies far more ("novel campaigns the catalog has never seen", "regardless of catalog knowledge"). The honest framing is: Sentry is a **narrow, editor-scoped, detection-only behavioral correlator**, and the catalog-corroboration + fail-closed hook are the layers that actually *block*. The highest-leverage v1.x work, in order: **(1) file-write persistence ingestion + SENTRY-008**, **(2) agent-descendant gate + SENTRY-006**, **(3) watchlist expansion**, **(4) SENTRY-007 generalized fusion**, **(5) DNS (Linux/Windows)**, **(6) memory-read (Linux)**. Everything in Section 5 stays a residual gap and should be stated as such.
