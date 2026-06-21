package posture

// PURE — no imports. PMState is the caller-resolved local package-manager state
// produced by the detection readers (detect.go / scanners.go). It is a
// read-only snapshot for the Layer-2 `beekeeper posture` view: installed
// versions plus the derived hardening flags.
//
// History: relocated verbatim from the former package-manager nudge package (the
// steering decision that consumed PMState was removed in v1.1.0). Only the
// state shape survives.

// PMState is the caller-resolved local package-manager state.
// All detection I/O happens before any consumer reads it (in detect.go).
type PMState struct {
	NpmInstalled bool
	NpmVersion   string

	PnpmInstalled bool
	PnpmVersion   string
	// PnpmHardened is true when pnpm version meets the floor (>= 11.0.0).
	// Set by the detection reader.
	PnpmHardened bool

	BunInstalled bool
	BunVersion   string
	// BunScannerOK is true when @socketsecurity/bun-security-scanner is present
	// in bunfig.toml (either project root or ~/.bunfig.toml).
	BunScannerOK bool

	// NodeVersion is the active Node.js version string, e.g. "22.5.0".
	// Required for the pnpm 11 Node >= 22 compatibility check.
	NodeVersion string
}
