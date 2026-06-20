package coveragegate

// Statement-coverage floor (the complement of the linkage gate in
// coveragegate.go). The linkage gate proves every production file is *accounted*
// (package-tested or allowlisted); the floor proves the package-tested ones stay
// at a minimum statement coverage, so a silent regression (delete the tests,
// keep the file) cannot slip under a green linkage gate.
//
// The floor is checked by TestCoverageFloor against a coverprofile produced by a
// prior `go test ./... -coverprofile=...` run (see the Makefile `coverage`
// target and the CI coverage-floor job). It is OFF in the normal unit run (the
// test skips when BEEKEEPER_COVERPROFILE is unset) so it never double-runs the
// suite from inside a test.
//
// Floors are deliberately set a few points BELOW the host-measured level to
// absorb cross-OS variance (some packages compile different files per GOOS), so
// the gate guards against real regressions without flaking across the CI matrix.

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

// ModulePath is the module import-path prefix stripped from coverprofile file
// paths to recover the repo-relative package key (e.g. "internal/audit").
const ModulePath = "github.com/home-beekeeper/beekeeper"

// defaultFloor is the minimum statement coverage (percent) required of any
// package-tested package without an explicit override or exemption. Every
// non-exempt package currently clears it with margin.
const defaultFloor = 80.0

// packageFloors are per-package minimums above the default. Values sit a few
// points below the host-measured coverage so cross-OS variance cannot flake the
// gate. Raising these as coverage improves is encouraged; lowering one is a
// deliberate, reviewable loosening.
var packageFloors = map[string]float64{
	// Locked-in gains from the coverage push (measured 90-97% on host).
	"internal/audit":      85,
	"internal/baseline":   92,
	"internal/catalog":    85,
	"internal/editorinit": 88,
	"internal/gateway":    85,
	"internal/hooks":      85,
	"internal/tui":        88,
	"internal/watch":      85,
	// Already-high, OS-independent pure-logic packages.
	"internal/corpus":   88,
	"internal/nudge":    90,
	"internal/pkgparse": 90,
	"internal/policy":   88,
	// OS-variant packages: these compile DIFFERENT files per GOOS (ipc uses a
	// unix-socket transport under //go:build linux||darwin vs a Windows named
	// pipe; quarantine has per-OS move/permission paths), so the unix-only code
	// is compiled and partly exercised only on non-Windows. This gate runs ONLY
	// on the canonical Linux CI runner (ci.yml coverage-floor: runs-on
	// ubuntu-latest), where that code counts against the total — so Linux
	// coverage is several points BELOW a Windows host (ipc 79.8% vs 87.7%,
	// quarantine 70.3% vs 82.5%). Floors are calibrated a few points under the
	// LINUX runner number (the gate's real environment), not the higher Windows
	// host. Raising them needs Linux-runnable tests for the per-OS branches.
	"internal/ipc":        75,
	"internal/quarantine": 68,
}

// exemptPackages are OS-bound or e2e-gated packages whose real coverage is
// validated elsewhere (the cross-platform CI matrix or the `-tags e2e` jobs),
// not the default host unit run, so a host-measured statement floor would be
// meaningless or flaky. Each carries a reason code (mirrors coverage-allowlist).
var exemptPackages = map[string]string{
	"internal/sentry/windows": "etw-os: ETW needs a live session/admin; validated in the windows CI + integration path",
	"internal/sentry/darwin":  "eslogger-os: validated by the macos eslogger CI job",
	"internal/sentry/linux":   "ebpf-os: validated on linux CI + ebpf-kernel.yml",
	"internal/llamafirewall":  "e2e-sidecar: real sidecar + gated model run under -tags e2e in CI",
	"cmd/beekeeper":           "e2e+daemon: thin cobra wiring + per-OS daemon code; exercised by -tags e2e",
	"internal/platform":       "os-variant: per-OS perms/dirs; each branch only compiles on its GOOS",
	"internal/notify":         "os-desktop: platform desktop-notifier shells",
	"internal/version":        "type-only: no tests by design (see coverage-allowlist.txt)",
}

// FloorViolation is one package below its required statement-coverage floor.
type FloorViolation struct {
	Package string
	Percent float64
	Floor   float64
}

// PackageCoverage parses a Go coverprofile and returns repo-relative package
// import paths mapped to their statement-coverage percent. The module prefix is
// stripped, so keys look like "internal/audit". A package with zero counted
// statements is reported as 0.
func PackageCoverage(r io.Reader) (map[string]float64, error) {
	type acc struct{ covered, total int }
	byPkg := map[string]*acc{}

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	first := true
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if first {
			first = false
			if strings.HasPrefix(line, "mode:") {
				continue
			}
		}
		// Format: <file>:<s.col>,<e.col> <numStmts> <count>
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, fmt.Errorf("coveragegate: malformed coverprofile line: %q", line)
		}
		pos := fields[0]
		colon := strings.LastIndex(pos, ":")
		if colon < 0 {
			return nil, fmt.Errorf("coveragegate: malformed coverprofile position: %q", pos)
		}
		file := pos[:colon]
		numStmts, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, fmt.Errorf("coveragegate: bad statement count in %q: %w", line, err)
		}
		count, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("coveragegate: bad hit count in %q: %w", line, err)
		}
		pkg := strings.TrimPrefix(path.Dir(file), ModulePath+"/")
		a := byPkg[pkg]
		if a == nil {
			a = &acc{}
			byPkg[pkg] = a
		}
		a.total += numStmts
		if count > 0 {
			a.covered += numStmts
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("coveragegate: reading coverprofile: %w", err)
	}

	out := make(map[string]float64, len(byPkg))
	for pkg, a := range byPkg {
		if a.total == 0 {
			out[pkg] = 0
			continue
		}
		out[pkg] = 100 * float64(a.covered) / float64(a.total)
	}
	return out, nil
}

// FloorFor returns the required floor for a repo-relative package key and whether
// the package is exempt (in which case the float is irrelevant).
func FloorFor(pkg string) (floor float64, exempt bool) {
	if _, ok := exemptPackages[pkg]; ok {
		return 0, true
	}
	if f, ok := packageFloors[pkg]; ok {
		return f, false
	}
	return defaultFloor, false
}

// FloorViolations returns the packages in cov that fall below their floor,
// sorted by package. Exempt packages are skipped.
func FloorViolations(cov map[string]float64) []FloorViolation {
	var v []FloorViolation
	for pkg, pct := range cov {
		floor, exempt := FloorFor(pkg)
		if exempt {
			continue
		}
		// Round to one decimal so a 84.96 vs 85.0 float artifact does not trip.
		if rounded := float64(int(pct*10+0.5)) / 10; rounded < floor {
			v = append(v, FloorViolation{Package: pkg, Percent: rounded, Floor: floor})
		}
	}
	sort.Slice(v, func(i, j int) bool { return v[i].Package < v[j].Package })
	return v
}

// CheckFloorProfile parses the coverprofile at path and returns any floor
// violations. It is the entry point used by TestCoverageFloor and any external
// caller (e.g. a CI script).
func CheckFloorProfile(profilePath string) ([]FloorViolation, error) {
	f, err := os.Open(profilePath)
	if err != nil {
		return nil, fmt.Errorf("coveragegate: open coverprofile: %w", err)
	}
	defer func() { _ = f.Close() }()
	cov, err := PackageCoverage(f)
	if err != nil {
		return nil, err
	}
	return FloorViolations(cov), nil
}
