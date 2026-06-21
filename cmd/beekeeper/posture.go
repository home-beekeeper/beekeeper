// posture.go — Cobra wiring for `beekeeper posture` (Layer-2 read-only view).
//
// Architecture constraint: this file is thin wiring only. The comparison model
// and rendering live in internal/posture (BuildComparison / Comparison.Render);
// the read-only detection lives in internal/posture (DetectState + scanners).
// This command does the impure resolution (config + DetectState + the pnpm
// weakness read) and hands the resolved PMState to the pure view.
//
// READ-ONLY guarantee (IPVIEW-02): `beekeeper posture` NEVER writes a
// package-manager config file. It only reads each detected manager's version and
// config to show it side-by-side with Beekeeper's enforced posture. The
// load-bearing self-defense test (posture_cmd_test.go) drives this command
// against fixture .npmrc / pnpm-workspace.yaml / bunfig.toml files and asserts
// they are byte-for-byte unchanged afterwards.
package main

import (
	"github.com/spf13/cobra"

	"github.com/home-beekeeper/beekeeper/internal/posture"
)

// postureDetectFn is the detection seam for the posture command. Tests swap it to
// inject a synthetic PMState without a real npm/pnpm/bun on PATH. Production code
// leaves it as posture.DetectStateFn (the real read-only detection).
var postureDetectFn = posture.DetectStateFn

// postureWeaknessFn is the pnpm-workspace weakness-note seam. Tests swap it to
// avoid touching the real working directory. Production code leaves it as the
// real read-only reader.
var postureWeaknessFn = posture.PnpmWeaknessNote

// newPostureCmd creates the `beekeeper posture` read-only view command.
func newPostureCmd() *cobra.Command {
	var full bool
	cmd := &cobra.Command{
		Use:   "posture",
		Short: "Show each package manager's install posture side-by-side with Beekeeper's enforced posture",
		Long: `Show install posture, read-only and machine-wide.

For each detected package manager (npm, pnpm, bun) this reads the manager's own
version and config (.npmrc, pnpm-workspace.yaml, bunfig.toml) and shows it
side-by-side with the posture Beekeeper enforces at the agent hook: release-age
24h, lifecycle scripts warned, and git/remote-URL dependencies flagged (all warn
by default). It names the gaps Beekeeper covers that your package manager does
not.

This command is strictly READ-ONLY. It reads package-manager config to display
it and never modifies any package-manager config file. It sets nothing and
enforces nothing on its own; it only describes what Beekeeper enforces at the
hook.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Read-only detection of each package manager's version + config.
			state := postureDetectFn(cmd.Context(), posture.DefaultConfig())

			// Read-only pnpm-workspace weakness note (empty when none / absent).
			weakness := postureWeaknessFn()

			// Build the pure comparison model and render it (plus the canonical
			// enforcement-boundary statement) to stdout. No file is written.
			comparison := posture.BuildComparison(state, posture.DefaultEnforced(), weakness)
			comparison.Render(cmd.OutOrStdout(), full)
			return nil
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "Print the full enforcement-boundary statement instead of the short form")

	// Plan 29-02: the actionable scoped-override surface (allow / enforce). The
	// parent `posture` command stays read-only; these subcommands are the ones that
	// record a graduated, audited override (IPOVR-01/02).
	addPostureOverrideCommands(cmd)
	return cmd
}
