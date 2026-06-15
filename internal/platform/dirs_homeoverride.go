//go:build beekeeperhomeoverride

package platform

// buildTagHomeOverride is true when the binary is compiled with the
// `beekeeperhomeoverride` build tag. The live-binary E2E harness builds the
// beekeeper binary with this tag so the shipped binary honors BEEKEEPER_HOME for
// hermetic test isolation (BTEST-03). Normal release builds never set this tag.
//
// See platform.StateDir / homeOverrideAllowed for the security rationale
// (remediation 260615, finding #1).
const buildTagHomeOverride = true
