// Package scan — subprocess-boundary pin for the Pollen binary (BKINT-02).
//
// PinnedPollenVersion records the exact Pollen binary version that beekeeper CI
// installs via `go install github.com/bantuson/pollen/cmd/pollen@<version>` in
// .github/workflows/ci.yml.
//
// Design rationale:
//   - beekeeper consumes Pollen exclusively as a subprocess binary (exec.LookPath +
//     exec.CommandContext in scanner.go). There is NO Go-module import of Pollen
//     (github.com/bantuson/pollen) — adding one would violate the subprocess
//     isolation boundary established by BKINT-01 and allow Pollen code to execute
//     inside beekeeper's address space.
//   - The pin is therefore expressed as a CI `go install @version` step, NOT a
//     go.mod require directive.
//   - This const makes the pinned version machine-auditable from beekeeper source.
//     When the pinned version is bumped, update BOTH this const AND the CI step —
//     they must stay in sync (the CI step references this file as the source of truth).
//
// To bump the pin:
//  1. Update PinnedPollenVersion below.
//  2. Update the "Install Pollen (BKINT-02 — pinned binary for inventory tests)"
//     step in .github/workflows/ci.yml to the same version.
//  3. Open a beekeeper PR with both changes (no go.mod change needed or permitted).
package scan

// PinnedPollenVersion is the Pollen binary version pinned for beekeeper CI
// inventory tests (BKINT-02). CI installs this version via:
//
//	go install github.com/bantuson/pollen/cmd/pollen@v0.1.1-pollen.4
//
// This is a subprocess-boundary pin — NOT a Go-module dependency.
// See .github/workflows/ci.yml "Install Pollen (BKINT-02)" step.
const PinnedPollenVersion = "v0.1.1-pollen.4"
