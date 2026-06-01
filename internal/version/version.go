// Package version holds build-time metadata for the beekeeper binary.
//
// The package-level variables are overwritten at release time via
// -ldflags -X, for example:
//
//	-X github.com/bantuson/beekeeper/internal/version.Version=v0.1.0
//	-X github.com/bantuson/beekeeper/internal/version.Commit=<sha>
//	-X github.com/bantuson/beekeeper/internal/version.Date=<commit-date>
//
// The defaults below are what a plain `go build`/`go run` produces when no
// ldflags are supplied.
package version

// Build metadata. Populated via -ldflags -X at release time; the values here
// are the development defaults.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)
