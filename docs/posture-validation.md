# Install-Posture Validation Coverage (v1.1.0 "Install Posture")

This is the criterion-4 coverage report for the install-posture feature (PRD
Layer 1: IPST / IPVIEW / IPOVR / IPBND, plus SENTRY-009 and REL-01). It maps
every rule, every adapter decision branch, every override path, the view
read-only guarantee, and the Sentry observe path to the test that covers it
(file:func), and it NAMES the deliberate gaps honestly.

The covering tests are all deterministic and offline (no network / no live
registry fetch): the pure evaluators take pre-resolved inputs, the adapter uses
the `posturePublishAgeFn` / `postureLifecycleFn` seams, and the git-remote rule
is parsed from the command string.

---

## Layer 1: pure rule evaluators (`internal/policy`)

Each rule is a pure function (no I/O, no time, no goroutines), enforced by an
imports-purity contract test.

| Rule | Branch | Covering test (file:func) |
|------|--------|---------------------------|
| release-age | younger than threshold -> block | `release_age_test.go:TestReleaseAgeYoungPackageBlocked` |
| release-age | older than threshold -> allow | `release_age_test.go:TestReleaseAgeOldPackageAllowed` |
| release-age | exactly at the 24h/1440m boundary -> allow (not younger-than) | `release_age_test.go:TestReleaseAgeExactly24hBoundary` |
| release-age | missing timestamp -> fail-closed block | `release_age_test.go:TestReleaseAgeTimestampMissingBlocks` |
| release-age | allowlist exempt | `release_age_test.go:TestReleaseAgeAllowlistExempt` |
| release-age | allowlist precedes missing-timestamp | `release_age_test.go:TestReleaseAgeAllowlistBeforeMissing` |
| release-age | per-ecosystem threshold override | `release_age_test.go:TestReleaseAgePerEcosystemOverride` |
| release-age | pure-imports contract | `release_age_test.go:TestReleaseAgeImportsArePure` |
| lifecycle | each script type fires + is named (preinstall/install/postinstall/prepare/pre+postuninstall) | `lifecycle_test.go:TestLifecycleEachScriptTypeFires` |
| lifecycle | script present, not allowlisted -> block | `lifecycle_test.go:TestLifecycleScriptPresentNotAllowlisted` |
| lifecycle | script present, allowlisted -> allow | `lifecycle_test.go:TestLifecycleScriptPresentAllowlisted` |
| lifecycle | no / nil scripts -> allow | `lifecycle_test.go:TestLifecycleNoScriptsAllowed`, `:TestLifecycleNilScriptsAllowed` |
| lifecycle | registry check failed -> fail-closed block | `lifecycle_test.go:TestLifecycleRegistryCheckFailedBlocks` |
| lifecycle | multiple scripts named in reason | `lifecycle_test.go:TestLifecycleMultipleScriptsReason` |
| lifecycle | pure-imports contract | `lifecycle_test.go:TestLifecycleImportsArePure` |
| git-remote | registry install -> allow (nothing to flag) | `internal/pkgparse/remote_source_test.go` (classification) + adapter tests below |
| git-remote | git/github/url/tarball/file source -> warn | `posture_adapter_test.go:TestPostureRemoteSourceWarns` |
| git-remote | allowlist exempt | covered via `PostureRuleExcludes` wiring (see overrides) |

## Layer 1: impure adapter decision branches (`internal/check/posture_adapter.go`)

The adapter applies the configured per-rule action and the fail-soft-on-unknown
divergence. The default action is WARN; a rule can be opted UP to block.

| Adapter branch | Covering test (file:func) |
|----------------|---------------------------|
| fresh package (definite) -> warn by default | `posture_adapter_test.go:TestPostureFreshPackageWarns` |
| old clean package -> allow | `posture_adapter_test.go:TestPostureOldCleanPackageAllows`, `:TestPostureCleanRegistryInstallAllows` |
| lifecycle scripts present -> warn | `posture_adapter_test.go:TestPostureLifecycleScriptsWarn` |
| remote source -> warn | `posture_adapter_test.go:TestPostureRemoteSourceWarns` |
| missing timestamp -> warn-unknown (NOT block) | `posture_adapter_test.go:TestPostureMissingTimestampWarnsNotBlock` |
| registry error -> warn-unknown | `posture_adapter_test.go:TestPostureRegistryErrorWarnsNotBlock` |
| fetch TIMEOUT -> warn-unknown, even under block mode | `posture_adapter_test.go:TestPostureFetchTimeout` |
| unsupported-ecosystem lifecycle -> warn-unknown | `posture_adapter_test.go:TestPostureLifecycleUnsupportedWarnsNotBlock` |
| block mode blocks a definite violation | `posture_adapter_test.go:TestPostureBlockModeBlocksFreshPackage` |
| block mode: unknown stays fail-soft warn | `posture_adapter_test.go:TestPostureBlockModeMissingTimestampStillWarns`, `:TestPostureBlockModeRegistryErrorStillWarns` |
| block mode never blocks a clean install | `posture_adapter_test.go:TestPostureBlockModeOldCleanPackageAllows` |
| block mode affects only the opted rule | `posture_adapter_test.go:TestPostureBlockModeOnlyAffectsOptedRule` |
| posturizeWithAction allow/warn/block mapping | `posture_adapter_test.go:TestPosturize*` (5 tests) |
| non-Bash / non-install commands skip posture | `posture_adapter_test.go:TestPostureSkipsNonBash`, `:TestPostureSkipsNonInstall` |

## Layer 1: live `RunCheck` path (`internal/check/posture_integration_test.go`)

These drive the REAL `RunCheck` (test the PATH, not the component): they assert
the resulting decision, the exit code, the reason, AND the audit record.

| Live-path scenario | Covering test (file:func) |
|--------------------|---------------------------|
| fresh package warns (exit 0, audit "warn") | `posture_integration_test.go:TestRunCheckPostureFreshPackageWarns` |
| missing timestamp warns not block (fail-soft) | `posture_integration_test.go:TestRunCheckPostureMissingTimestampWarnsNotBlock` |
| posture warn cannot downgrade a catalog block | `posture_integration_test.go:TestRunCheckPostureCannotDowngradeCatalogBlock` |
| release-age block mode blocks (exit 1, audit "block") | `posture_integration_test.go:TestRunCheckPostureBlockModeBlocksFreshPackage` |
| lifecycle block mode blocks (exit 1, lifecycle reason, audit "block") | `posture_integration_test.go:TestRunCheckPostureBlockModeBlocksLifecycle` |
| git-remote block mode blocks (exit 1, remote reason, audit "block") | `posture_integration_test.go:TestRunCheckPostureBlockModeBlocksRemoteSource` |
| git-remote default warns not block (attribution sibling) | `posture_integration_test.go:TestRunCheckPostureRemoteSourceDefaultWarnsNotBlock` |
| block mode: unknown stays warn on live path | `posture_integration_test.go:TestRunCheckPostureBlockModeMissingTimestampStillWarns` |
| block mode cannot downgrade a catalog block | `posture_integration_test.go:TestRunCheckPostureBlockModeCannotDowngradeCatalogBlock` |
| shim shape enforces posture (latent-gap close) | `posture_integration_test.go:TestRunCheckShimShapeEnforcesPosture` |

## Overrides (IPOVR-01/02/03)

| Override path | Covering test (file:func) |
|---------------|---------------------------|
| allow --once writes distinct allow_once record + on-disk token | `cmd/beekeeper/posture_override_test.go:TestPostureAllowOnceWritesDistinctRecord` |
| allow --always writes allow_always record + posture-scoped config (not package_allowlist) | `cmd/beekeeper/posture_override_test.go:TestPostureAllowAlwaysWritesDistinctRecordAndConfig` |
| allow --always ecosystem-scoped (npm exempt, pypi still warns) | `cmd/beekeeper/posture_override_test.go:TestPostureAllowAlwaysEcosystemScoped` |
| allow --always survives an on-disk config reload | `cmd/beekeeper/posture_override_test.go:TestPostureAllowAlwaysConfigPersistence` |
| allow --always requires --reason | `cmd/beekeeper/posture_override_test.go:TestPostureAllowAlwaysRequiresReason` |
| allow rejects both/neither mode | `cmd/beekeeper/posture_override_test.go:TestPostureAllowRejectsBothModes` |
| enforce --block writes enforce_block record + sets block action | `cmd/beekeeper/posture_override_test.go:TestPostureEnforceBlockWritesDistinctRecordAndConfig` |
| enforce --warn writes enforce_warn record + lowers action back to warn | `cmd/beekeeper/posture_override_test.go:TestPostureEnforceWarnWritesRecord` |
| enforce rejects an unknown rule | `cmd/beekeeper/posture_override_test.go:TestPostureEnforceRejectsBadRule` |
| allow-once consumed then warns (live `RunCheck`) | `posture_integration_test.go:TestRunCheckPostureAllowOnceConsumedThenWarns` |
| allow-always exempts a fresh package (live `RunCheck`) | `posture_integration_test.go:TestRunCheckPostureAllowAlwaysAllowsFreshPackage` |
| allow-always does NOT bypass a catalog malware block (T-09-31, load-bearing) | `posture_integration_test.go:TestRunCheckPostureAllowAlwaysDoesNotBypassCatalogBlock` |
| per-rule action config: default warn / opt-up block / nil-safe / fail-closed bad action | `internal/config/posture_test.go:TestDefaultPostureConfigAllWarn`, `:TestPostureRuleActionNilSafe`, `:TestPostureRuleActionReturnsConfigured`, `:TestValidatePostureConfigAccepts`, `:TestValidatePostureConfigRejectsBogus` |
| layered untrusted tighten-only: warn->block applied, block->warn refused | `internal/config/layered_posture_test.go:TestMergePostureUntrustedRefusesLoosen`, `:TestLoadLayeredPostureProjectCanTightenNotLoosen` |
| layered untrusted cannot inject an Allow exemption | `internal/config/layered_posture_test.go:TestMergePostureUntrustedDropsAllow`, `:TestMergePostureUntrustedDropsAllowNilDst`, `:TestLoadLayeredPostureProjectCannotAddAllow` |
| PostureRuleExcludes rule/ecosystem scoping | `internal/config/layered_posture_test.go:TestPostureRuleExcludesScoping` |

> Note: the CLI surface deliberately does NOT re-test untrusted tighten-only.
> `internal/config/layered_posture_test.go` is the authoritative layer for the
> block->warn refusal and the injected-Allow refusal (both at the merge and the
> end-to-end `LoadLayered` level), so duplicating it at the CLI level would add no
> coverage. The override tests assert only the CLI-specific behavior (the distinct
> audit record + the persisted config shape).

## View read-only guarantee (IPVIEW-02)

| Guarantee | Covering test (file:func) |
|-----------|---------------------------|
| `beekeeper posture` view never mutates .npmrc / pnpm-workspace.yaml / bunfig.toml (sha256 byte-for-byte unchanged) | `cmd/beekeeper/posture_cmd_test.go:TestPostureCmd_ReadOnlyGuarantee` |
| npm + pnpm view output | `cmd/beekeeper/posture_cmd_test.go:TestPostureCmd_Output_NpmAndPnpm` |
| full boundary statement rendered | `cmd/beekeeper/posture_cmd_test.go:TestPostureCmd_FullBoundary` |

## SENTRY-009: human-install observe (not block)

| Property | Covering test (file:func) |
|----------|---------------------------|
| an install with monitored ancestry is OBSERVED (record_type sentry_install_observed, decision "observe") | `internal/sentry/install_observe_test.go:TestSENTRY009ObservesInstall` |
| observation NEVER recommends quarantine (QuarantineRec=false) | `internal/sentry/install_observe_test.go:TestSENTRY009ObservesInstall` |
| a non-install process is ignored | `internal/sentry/install_observe_test.go:TestSENTRY009IgnoresNonInstall` |
| only monitored (editor/agent-descended) ancestry is observed | `internal/sentry/install_observe_test.go:TestSENTRY009RequiresMonitoredAncestry` |

## Live-binary E2E (`internal/check/e2e_test.go`, `//go:build e2e`)

Run via the CI `e2e` job: `go test -tags e2e -run TestE2ELiveBinary ./internal/check/...`.

| E2E sub-case | Asserts |
|--------------|---------|
| `TestE2ELiveBinary/posture_git_remote_default_warn` | a git install at the hook under default config: exit 0, audit decision "warn", git/remote reason |
| `TestE2ELiveBinary/posture_git_remote_block_mode` | the SAME install with a hermetic config.json opting git-remote to block: exit 1, audit decision "block" |
| `TestE2ELiveBinary/posture_sentry009_human_install_observe_only` | SKIPS with a structured reason (daemon taps are CI/Linux-only; see the gap below) |

---

## Deliberate gaps (named honestly)

1. **SENTRY-009 per-OS daemon install taps are CI/Linux-only.** The live daemon
   install observation rides the OS process taps (eBPF on Linux, ETW on Windows,
   eslogger on macOS), which are not exercisable from a single cross-platform
   live-binary E2E and are flaky to run per-OS. The observe-only, never-block
   contract is covered deterministically at the unit level
   (`internal/sentry/install_observe_test.go`); the live daemon tap itself is a
   CI/Linux-only surface and the E2E sub-case skips with a structured reason
   rather than asserting it. The hook (the `beekeeper check` binary) only ever
   sees agent tool calls, so a human-run install never reaches it and there is
   nothing for the hook to (not) block: Sentry observes, it never prevents
   (IPBND-01).
2. **DNS / network correlation for installs is out of scope** for the
   install-posture milestone. Posture evaluates the install command and registry
   metadata; it does not correlate post-install network/DNS activity. That
   correlation belongs to the Sentry runtime-rule layer, not Layer 1.
3. **The package-manager shim as a machine-wide enforcement surface is roadmap.**
   The shim that would extend pre-exec posture enforcement to EVERY install
   (including non-hooked, human-run installs) is experimental, not a headline
   guarantee. The live `RunCheck` already enforces posture for the shim tool-call
   SHAPE (`TestRunCheckShimShapeEnforcesPosture`), but installing the shim as a
   machine-wide interceptor is future work. Today, posture is enforced pre-exec at
   the agent hook for hooked (Tier-1) harnesses only, inheriting each harness
   tier's caveats (see `harness-support-matrix.md` and `validation-register.md`).

## REL-01 (version path)

The binary version is ldflags-injected at tag/build time; there is nothing to
hardcode-bump in Go. The web v1.1.0 changelog entry already exists (authored in
30-02, web repo). No Go doc touched here claims a roadmap item as shipped: the
shim machine-wide surface, DNS correlation, and per-OS daemon install blocking
are all named above as gaps/roadmap, consistent with `posture.BoundaryStatement`
and the adapter file header.
