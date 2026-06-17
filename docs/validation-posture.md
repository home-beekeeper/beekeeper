# Validation Posture

**Phase 21 (VAL-07).** This document makes Beekeeper's "fully validated" claim
**auditable**: it states exactly what is verified, at which tier, and how — so an
external reader can check the coverage claim rather than take it on faith. It is
consistent with the honesty spine in
[THREAT-MODEL.md §8](THREAT-MODEL.md) and the
[harness-support-matrix.md](harness-support-matrix.md) tiers.

"Fully validated" means: **100% of what can be tested locally is tested and
gate-enforced + a CI matrix for everything platform-bound + a documented manual
register for the irreducible remainder, with zero silent gaps.**

---

## The validation tier model

| Tier | What | How it is verified | Enforcement |
|------|------|--------------------|-------------|
| **Tier A** | Locally testable on the dev box (pure logic, parsers, deny contracts, installer config, the policy/correlation engines) | `go test ./...` on Windows; the coverage gate accounts every production file | **100%, gate-enforced** (`internal/coveragegate`) |
| **Tier B** | Platform / kernel / build-tag bound (eBPF, eslogger, ETW, `-race`/CGO, Unix peer-cred, 3×GOOS cross-build) | Cross-platform CI matrix (ubuntu-22.04 + macos-latest + windows-latest). The eBPF generate + two-kernel (5.4/5.15) load is **decoupled to a manual workflow** pending a toolchain rebuild. | **CI matrix** (`.github/workflows/ci.yml`); eBPF/kernel = **manual** (`ebpf-kernel.yml`) |
| **Tier C** | Irreducible / manual / gated (a true live block on the 16 non-Claude-Code harnesses; the gated-22M-model LlamaFirewall e2e) | Documented manual procedures with sign-off | **Signed manual register** ([validation-register.md](validation-register.md)) |

Only **Claude Code** crosses from Tier C into automated coverage: its live block
is proven by `TestE2ELiveBinary/SPATH_hook_claude_code_exit2` (VAL-05), the
documented true-block reference.

### Why three tiers (not "100% coverage")

A flat coverage-percentage number would be dishonest for a security tool: a
kernel eBPF probe cannot run on a Windows dev box, and a live harness block
cannot be automated without that vendor's client installed. Splitting validation
into A/B/C states precisely which guarantee each behavior has, and the manual
register names exactly what remains hand-verified — no behavior sits in an
untested, unregistered fourth bucket.

---

## Tier A — the coverage gate (VAL-01 / VAL-08)

`internal/coveragegate` walks every production `.go` file under `internal/` and
`cmd/` and classifies each as **package-tested** (its directory contains at least
one `_test.go`) or **reason-coded allowlisted**. Any file that is neither is
**UNACCOUNTED** and fails `TestCoverageManifest` — and therefore `go test ./...`
and CI.

Linkage is package-level, not same-name-sibling: ~70 of ~184 production files
have no same-name `_test.go` yet are package-tested, so sibling linkage would
drown the real gaps in false positives.

### The no-test allowlist (`coverage-allowlist.txt`)

A file may be exempted from package-test linkage only with an explicit,
reason-coded entry of the form `path<TAB># reason: <code>`. The parser **fails
closed**: a bare path, an empty reason, or a reason code outside the closed
taxonomy below breaks the gate loudly (`TestAllowlistFailsClosed`). The coverage
bar therefore cannot be silently lowered — growing the allowlist requires a
recognized reason code, which is reviewable in the diff (VAL-08 self-defense).

**Closed reason-code taxonomy:**

| Reason code | Meaning |
|-------------|---------|
| `generated-bpf` | eBPF loader bindings (`bpf_*_bpfel.go`), committed as fail-closed stubs; the real bytecode is generated out-of-band (`ebpf-kernel.yml`), not committed |
| `platform-stub` | Fail-closed per-OS shim with no logic (`*_other.go` / `*_windows.go` / `*_unix.go`) |
| `type-only` | Pure type/const/build-metadata, no behavior to test |
| `exec-seam-stub` | One-line wrapper existing only as a test seam |
| `thin-delegator` | Delegates entirely to a tested function elsewhere |
| `gen-directive` | `go:generate` directive carrier |

As of Phase 21, `internal/version` (three ldflag-injected build-metadata strings)
is the only entry — every other production file is package-tested.

---

## Tier B — the CI matrix (VAL-03 / VAL-04)

`.github/workflows/ci.yml` gates the platform-bound behavior that cannot run on
the Windows dev box:

- **build** (native + a 3×GOOS cross-compile, build-only) and **vet**
- **test** with **`-race`** (CGO enabled — the race detector requires it)
- **eslogger** field-schema validation on macOS, **ETW** on Windows
- **Unix peer-cred** auth (the `internal/ipc` server/client/peer Tier-B tests run
  on the Linux/macOS legs)

The **fuzz suite** is a blocking release gate (`release-gate.needs`): the policy
engine, IPC proto parser, catalog parser, MCP message parser (gateway), and the
**Sentry rule evaluator** (`FuzzEvaluateEvent`). A discovered panic blocks
release.

### eBPF generate + two-kernel load: currently a manual workflow

The eBPF bytecode regeneration and the two-kernel (5.4 / 5.15) probe-load tests
are **split out of blocking CI** into `.github/workflows/ebpf-kernel.yml`, which
runs **manual-only** (`workflow_dispatch`). They were decoupled because the
GitHub-hosted runner's `bpftool` cannot regenerate `vmlinux.h` and the
`cilium/little-vm-helper` nested-VM setup needs a KVM-capable runner the default
leg lacks. `ci.yml` builds the Linux code against the committed `bpf_*_bpfel.go`
loader stubs (which fail closed at runtime when real bytecode is absent), so the
rest of CI stays green. This leg is **not yet a required check**; the tracked TODO
is to rebuild the toolchain on a Linux runner and re-add it to PR CI and the
branch ruleset.

> CI is statically authored and locally build/YAML-verified. The live GitHub run
> is confirmed at first push (the repo has not yet been pushed).

---

## Tier C — the manual register (VAL-06)

Everything that is irreducibly manual is enumerated in
[validation-register.md](validation-register.md): a live-block procedure for each
of the 16 non-Claude-Code harnesses (install the real client, drive a canary
credential read, confirm the per-harness deny contract) plus the gated-22M-model
LlamaFirewall e2e. Each row has a sign-off field; an unsigned row honestly means
that harness has been contract-shape unit-tested but **not** live-block-verified.

---

## Summary

| Requirement | Surface | Mechanism |
|-------------|---------|-----------|
| VAL-01 / VAL-08 | Tier A coverage | `internal/coveragegate` + fail-closed reason-coded allowlist |
| VAL-02 | Deny + installer conformance | golden deny contracts + 17-target installer sweep |
| VAL-03 | Platform/kernel/build | CI matrix (2 kernels + macOS + Windows) |
| VAL-04 | Parser robustness | 5 fuzz targets as a blocking release gate |
| VAL-05 | Live block reference | Claude Code `--hook` exit-2 canary e2e |
| VAL-06 | Irreducible manual | the signed validation register |
| VAL-07 | Auditability | this document + corrected README counts (17 / 16) |
