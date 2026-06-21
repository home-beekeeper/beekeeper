package posture

// view.go - PURE (imports only "fmt" and "sort"). This is the single source of
// truth for the Layer-2 `beekeeper posture` comparison model, shared by BOTH the
// CLI command (cmd/beekeeper) and the TUI panel (internal/tui). It takes a
// caller-resolved PMState (the read-only detection done in detect.go/scanners.go)
// plus the read pnpm hardening result and produces a structured, side-by-side
// comparison of what each detected package manager does by default versus what
// Beekeeper enforces at the hook, naming the gaps Beekeeper covers.
//
// Purity contract: NO I/O, NO goroutines, NO globals mutation, NO wall-clock
// access. All detection (running pnpm/bun/node --version, reading .npmrc /
// pnpm-workspace.yaml / bunfig.toml) happens in the IMPURE detect.go/scanners.go
// before this model is built. This function NEVER writes any file - it only
// formats already-read state. That read-only property is the Layer-2 self-defense
// guarantee (IPVIEW-02): the byte-for-byte unchanged-after-run test drives the
// whole CLI path through this model and asserts no package-manager config file is
// touched.
//
// Beekeeper's enforced posture is fixed default-action prose here (release-age
// 24h, lifecycle scripts warned, git/remote-URL deps flagged), all at WARN by
// default at the hook. This view DISPLAYS that default posture; it does not set,
// tighten, or mutate anything. The opt-up-to-block config is Phase 29.

import (
	"fmt"
	"io"
	"sort"
)

// Enforced is Beekeeper's default enforced posture as displayed by the view.
// These three rules are what `beekeeper check` evaluates at the agent hook for an
// install-class tool call, all at WARN by default (the opt-up-to-block config is
// Phase 29). The strings are display copy only; the real evaluators live in
// internal/policy (EvaluateReleaseAge, EvaluateLifecycle, EvaluateRemoteSource).
type Enforced struct {
	// ReleaseAge is the minimum-publish-age rule, e.g. "release-age 24h".
	ReleaseAge string
	// LifecycleScripts is the install-script rule, e.g. "scripts warned".
	LifecycleScripts string
	// RemoteSource is the git/remote-URL dependency rule, e.g. "git deps flagged".
	RemoteSource string
}

// DefaultEnforced returns the canonical default enforced posture shown by the
// view. It mirrors policy.DefaultReleaseAgeConfig (1440 minutes / 24h) and the
// lifecycle + remote-source rules wired into the hook at WARN default (Phase 27).
// It is plain display copy - no policy decision is made here.
func DefaultEnforced() Enforced {
	return Enforced{
		ReleaseAge:       "release-age 24h",
		LifecycleScripts: "scripts warned",
		RemoteSource:     "git deps flagged",
	}
}

// ManagerRow is one detected ecosystem/manager's side-by-side comparison: what
// the manager's own config does by default, what Beekeeper enforces, and the gaps
// Beekeeper covers that the manager does not. A row is produced only for a
// DETECTED manager; absent managers are omitted entirely.
type ManagerRow struct {
	// Manager is the package-manager name ("npm", "pnpm", "bun").
	Manager string
	// Version is the detected version string (may be empty if unknown).
	Version string
	// SelfPosture describes what the manager does on its own, e.g.
	// "Scripts run by default. Git deps allowed." or
	// "minimumReleaseAge honored.".
	SelfPosture []string
	// Gaps are the named gaps Beekeeper covers that this manager does not handle
	// by default. An empty slice means Beekeeper is aligned with the manager (no
	// gap), e.g. a hardened pnpm.
	Gaps []string
	// Aligned is true when the manager's own hardening already matches Beekeeper's
	// enforced posture, so there is no gap to cover.
	Aligned bool
}

// GapCount returns the number of gaps Beekeeper covers for this manager.
func (r ManagerRow) GapCount() int { return len(r.Gaps) }

// Comparison is the full posture-view model: the enforced posture plus one row
// per DETECTED package manager. It is the structured output both the CLI and the
// TUI render. The boundary statement is intentionally NOT embedded here - callers
// print posture.BoundaryShort / BoundaryStatement directly (single source of
// truth, IPBND-01).
type Comparison struct {
	// Enforced is the Beekeeper default posture displayed alongside each manager.
	Enforced Enforced
	// Managers is one row per detected manager, in stable display order
	// (npm, pnpm, bun). Undetected managers are omitted.
	Managers []ManagerRow
}

// TotalGaps returns the sum of gaps Beekeeper covers across all detected
// managers.
func (c Comparison) TotalGaps() int {
	total := 0
	for _, m := range c.Managers {
		total += m.GapCount()
	}
	return total
}

// BuildComparison is the PURE comparison builder. Given a caller-resolved PMState
// (the read-only detection result) and the read pnpm hardening result, it returns
// the structured side-by-side comparison for every DETECTED manager. It performs
// no I/O and never writes any file.
//
// Detection inputs:
//   - state.NpmInstalled / NpmVersion: npm has no install hardening of its own
//     (scripts run by default, release-age not enforced, git deps allowed), so
//     Beekeeper covers all three gaps.
//   - state.PnpmInstalled / PnpmVersion / PnpmHardened: a pnpm that meets the
//     version floor and has not had hardening removed honors minimumReleaseAge
//     and blocks exotic subdeps, so Beekeeper is aligned (no gap). A pnpm whose
//     hardening was removed (or below floor) is treated like npm for the gaps it
//     no longer covers.
//   - state.BunInstalled / BunVersion / BunScannerOK: bun runs lifecycle scripts
//     by default and does not enforce release-age; with the Socket scanner
//     configured it gains supply-chain scanning but still leaves release-age and
//     git-dep flagging to Beekeeper.
//
// pnpmWeakness, when non-empty, is a human-readable note that the detected pnpm
// config has an explicit hardening WEAKNESS (e.g. minimumReleaseAge set below the
// 1440-minute baseline). It is surfaced as a self-posture line so the view is
// honest about a downgraded pnpm without claiming Beekeeper rewrites it.
func BuildComparison(state PMState, enforced Enforced, pnpmWeakness string) Comparison {
	c := Comparison{Enforced: enforced}

	if state.NpmInstalled {
		c.Managers = append(c.Managers, buildNpmRow(state.NpmVersion))
	}
	if state.PnpmInstalled {
		c.Managers = append(c.Managers, buildPnpmRow(state.PnpmVersion, state.PnpmHardened, pnpmWeakness))
	}
	if state.BunInstalled {
		c.Managers = append(c.Managers, buildBunRow(state.BunVersion, state.BunScannerOK))
	}

	// Stable display order regardless of detection order. The slice is already
	// appended in npm/pnpm/bun order above, but sort defensively so a future
	// caller that appends in a different order still renders deterministically.
	sort.SliceStable(c.Managers, func(i, j int) bool {
		return managerRank(c.Managers[i].Manager) < managerRank(c.Managers[j].Manager)
	})

	return c
}

// managerRank gives the stable display order: npm, pnpm, bun, then anything else
// alphabetically after.
func managerRank(name string) int {
	switch name {
	case "npm":
		return 0
	case "pnpm":
		return 1
	case "bun":
		return 2
	default:
		return 3
	}
}

// buildNpmRow describes npm. npm has no install-time hardening of its own:
// lifecycle scripts run by default, release-age is not enforced, and git/remote
// dependency URLs are allowed. Beekeeper covers all three gaps at WARN.
func buildNpmRow(version string) ManagerRow {
	return ManagerRow{
		Manager: "npm",
		Version: version,
		SelfPosture: []string{
			"Scripts run by default.",
			"Release-age not enforced.",
			"Git deps allowed.",
		},
		Gaps: []string{
			"scripts warned",
			"release-age 24h",
			"git deps flagged",
		},
		Aligned: false,
	}
}

// buildPnpmRow describes pnpm. A pnpm at or above the version floor whose
// workspace hardening is intact honors minimumReleaseAge and blocks exotic
// subdeps, so it is ALIGNED with Beekeeper (no gap). A pnpm that is below the
// floor or has had its hardening removed is treated like npm for the gaps it no
// longer covers.
//
// weakness (non-empty) means an explicit hardening downgrade was detected in
// pnpm-workspace.yaml (e.g. minimumReleaseAge below the 1440 baseline). pnpm still
// counts as hardened in that case (it is the user's explicit choice), but the
// weakness is surfaced honestly and Beekeeper's release-age floor is shown as the
// gap it covers.
func buildPnpmRow(version string, hardened bool, weakness string) ManagerRow {
	if hardened && weakness == "" {
		return ManagerRow{
			Manager: "pnpm",
			Version: version,
			SelfPosture: []string{
				"minimumReleaseAge honored.",
				"Exotic subdeps blocked.",
			},
			Gaps:    nil,
			Aligned: true,
		}
	}

	row := ManagerRow{
		Manager: "pnpm",
		Version: version,
	}
	if hardened && weakness != "" {
		// Hardening present but explicitly downgraded.
		row.SelfPosture = []string{
			"minimumReleaseAge honored.",
			weakness,
		}
		row.Gaps = []string{"release-age 24h"}
		row.Aligned = false
		return row
	}

	// Below the version floor (or hardening unknown) - treat like an unhardened
	// manager: Beekeeper covers all three gaps.
	row.SelfPosture = []string{
		"Hardening defaults not met.",
		"Release-age not enforced.",
		"Git deps allowed.",
	}
	row.Gaps = []string{
		"scripts warned",
		"release-age 24h",
		"git deps flagged",
	}
	row.Aligned = false
	return row
}

// buildBunRow describes bun. bun runs lifecycle scripts by default and does not
// enforce release-age. With the Socket security scanner configured (bunfig.toml)
// it gains supply-chain scanning, but Beekeeper still covers release-age and
// git-dep flagging. Without the scanner, Beekeeper additionally covers script
// warning.
func buildBunRow(version string, scannerOK bool) ManagerRow {
	if scannerOK {
		return ManagerRow{
			Manager: "bun",
			Version: version,
			SelfPosture: []string{
				"Socket security scanner configured.",
				"Release-age not enforced.",
			},
			Gaps: []string{
				"release-age 24h",
				"git deps flagged",
			},
			Aligned: false,
		}
	}
	return ManagerRow{
		Manager: "bun",
		Version: version,
		SelfPosture: []string{
			"Scripts run by default.",
			"No security scanner configured.",
			"Release-age not enforced.",
		},
		Gaps: []string{
			"scripts warned",
			"release-age 24h",
			"git deps flagged",
		},
		Aligned: false,
	}
}

// Summary returns a one-line human summary for a manager row, e.g.
//
//	"npm v11 detected. Scripts run by default. Beekeeper enforces: scripts
//	 warned, release-age 24h, git deps flagged. Covering 3 gaps your npm version
//	 does not."
//
// or for an aligned manager:
//
//	"pnpm v11 detected. minimumReleaseAge honored. Beekeeper enforces: aligned,
//	 no gap."
//
// This is PURE string formatting shared by the CLI and TUI so the legible PRD
// rows have a single source of truth.
func (r ManagerRow) Summary() string {
	ver := r.Version
	if ver == "" {
		ver = "detected."
	} else {
		ver = "v" + ver + " detected."
	}

	self := joinSentences(r.SelfPosture)

	if r.Aligned || len(r.Gaps) == 0 {
		return fmt.Sprintf("%s %s %s Beekeeper enforces: aligned, no gap.", r.Manager, ver, self)
	}

	gaps := joinList(r.Gaps)
	count := len(r.Gaps)
	noun := "gaps"
	if count == 1 {
		noun = "gap"
	}
	return fmt.Sprintf(
		"%s %s %s Beekeeper enforces: %s. Covering %d %s your %s version does not.",
		r.Manager, ver, self, gaps, count, noun, r.Manager,
	)
}

// Render writes the comparison as plain text (no color, no box-drawing) to w,
// followed by the canonical enforcement-boundary statement. It is PURE I/O over
// an io.Writer the caller supplies - it reads no files and writes no
// package-manager config. The boundary text is BoundaryShort (single source of
// truth, IPBND-01); pass full=true to print the longer BoundaryStatement.
//
// Rendering layout (ASCII only):
//
//	Install posture (read-only)
//
//	Beekeeper enforces at the hook: release-age 24h, scripts warned, git deps flagged (warn by default).
//
//	npm v11 detected. Scripts run by default. ... Covering 3 gaps your npm version does not.
//	pnpm v11 detected. minimumReleaseAge honored. ... Beekeeper enforces: aligned, no gap.
//
//	<boundary statement>
//
// When no managers are detected it still prints the enforced posture and the
// boundary so the honesty statement is always visible.
func (c Comparison) Render(w io.Writer, full bool) {
	fmt.Fprintln(w, "Install posture (read-only)")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Beekeeper enforces at the hook: %s, %s, %s (warn by default).\n",
		c.Enforced.ReleaseAge, c.Enforced.LifecycleScripts, c.Enforced.RemoteSource)
	fmt.Fprintln(w)

	if len(c.Managers) == 0 {
		fmt.Fprintln(w, "No package managers detected on this machine.")
	} else {
		for _, m := range c.Managers {
			fmt.Fprintln(w, m.Summary())
		}
		fmt.Fprintln(w)
		total := c.TotalGaps()
		noun := "gaps"
		if total == 1 {
			noun = "gap"
		}
		fmt.Fprintf(w, "Beekeeper covers %d %s across %d detected manager(s).\n",
			total, noun, len(c.Managers))
	}

	fmt.Fprintln(w)
	if full {
		fmt.Fprintln(w, BoundaryStatement)
	} else {
		fmt.Fprintln(w, BoundaryShort)
	}
}

// joinSentences joins already-terminated sentence fragments with single spaces.
func joinSentences(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " "
		}
		out += p
	}
	return out
}

// joinList joins gap labels with commas, e.g. "scripts warned, release-age 24h".
func joinList(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
