//go:build !beekeeperhomeoverride

package platform

// buildTagHomeOverride is false in normal production builds: BEEKEEPER_HOME is
// ignored at runtime unless the process is a `go test` binary (see
// platform.homeOverrideAllowed). This is the default for every released binary.
//
// See platform.StateDir for the security rationale (remediation 260615, #1).
const buildTagHomeOverride = false
