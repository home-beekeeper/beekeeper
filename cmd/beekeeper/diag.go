// diag.go — beekeeper diag command (CODE-06, Phase 9).
//
// Assembles and formats the DiagReport produced by check.CollectDiag into a
// human-readable sections layout. All business logic lives in internal/check.
// This file is thin Cobra wiring per the project architecture constraint.
package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mzansi-agentive/beekeeper/internal/check"
	"github.com/mzansi-agentive/beekeeper/internal/config"
	"github.com/mzansi-agentive/beekeeper/internal/platform"
)

// newDiagCmd implements `beekeeper diag`.
// Standalone command (no subcommands), analogous to newVersionCmd / newSelftestCmd.
func newDiagCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diag",
		Short: "Show system health: hook latency, sidecar latency, catalog freshness, ETW loss",
		Long: `Assemble and print the Beekeeper system health report.

Output sections:
  Hook Handler        — p95 and p99 latency over the last 100 persisted samples
  LlamaFirewall       — p95 sidecar inference latency
  Catalog Sources     — last-sync hash, entry count, degraded flag per source
  ETW Event Loss      — events dropped by the Windows Sentry consumer (0 on non-Windows)

Data sources:
  ~/.beekeeper/hook-latency.json  (ring file written by each beekeeper check)
  ~/.beekeeper/state.json          (catalog freshness snapshot)
  internal/llamafirewall global tracker  (in-process p95)
  internal/sentry/windows.EventsLost    (Windows only)`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			stateDir, err := platform.StateDir()
			if err != nil {
				return fmt.Errorf("diag: resolve state directory: %w", err)
			}
			configPath, err := platform.ConfigPath()
			if err != nil {
				return fmt.Errorf("diag: resolve config path: %w", err)
			}

			_, _ = config.Load(configPath) // load config via layered resolver (CODE-05)

			stateFile := filepath.Join(stateDir, "state.json")
			hookLatencyRingPath := filepath.Join(stateDir, "hook-latency.json")

			report := check.CollectDiag(stateFile, hookLatencyRingPath)

			out := cmd.OutOrStdout()

			// Section 1: Hook Handler latency.
			fmt.Fprintln(out, "Hook Handler")
			fmt.Fprintf(out, "  p95 latency:  %dms  (target <100ms)\n", report.HookLatencyP95MS)
			fmt.Fprintf(out, "  p99 latency:  %dms\n", report.HookLatencyP99MS)
			fmt.Fprintln(out)

			// Section 2: LlamaFirewall sidecar latency.
			fmt.Fprintln(out, "LlamaFirewall Sidecar")
			fmt.Fprintf(out, "  p95 latency:  %dms  (target <100ms)\n", report.SidecarLatencyP95MS)
			fmt.Fprintln(out)

			// Section 3: Catalog freshness per source.
			fmt.Fprintln(out, "Catalog Sources")
			if len(report.CatalogSources) == 0 {
				fmt.Fprintln(out, "  (no catalog state — run 'beekeeper catalogs sync' first)")
			} else {
				for _, src := range report.CatalogSources {
					hash := src.Hash
					if hash == "" {
						hash = "(none)"
					}
					fmt.Fprintf(out, "  %-16s  last sync: %-20s  entries: %-6d  degraded: %v\n",
						src.Name, hash, src.Count, src.Degraded)
				}
			}
			fmt.Fprintln(out)

			// Section 4: ETW event loss (Windows only; always 0 on other platforms).
			fmt.Fprintln(out, "ETW Event Loss (Windows only)")
			fmt.Fprintf(out, "  events lost:  %d\n", report.ETWEventsLost)

			return nil
		},
	}
}

// resolveConfig builds LayerOpts for LoadLayered from platform paths plus the
// caller's environment. It is used by new Phase 9 commands that require CODE-05
// layered config resolution. Existing per-subcommand config.Load calls in earlier
// commands are NOT modified here — they remain for backwards compatibility.
func resolveConfig(cmd *cobra.Command) (config.Config, error) {
	configPath, err := platform.ConfigPath()
	if err != nil {
		return config.Config{}, fmt.Errorf("resolve config path: %w", err)
	}

	opts := config.LayerOpts{
		UserPath: configPath,
	}

	cfg, err := config.LoadLayered(opts)
	if err != nil {
		return config.Config{}, fmt.Errorf("load layered config: %w", err)
	}
	return cfg, nil
}
