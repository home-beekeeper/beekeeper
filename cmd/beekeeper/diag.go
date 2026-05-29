// diag.go — beekeeper diag command (CODE-06, Phase 9).
//
// Assembles and formats the DiagReport produced by check.CollectDiag into a
// human-readable sections layout. All business logic lives in internal/check.
// This file is thin Cobra wiring per the project architecture constraint.
package main

import (
	"fmt"
	"os"
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
	userPath, err := platform.ConfigPath()
	if err != nil {
		return config.Config{}, fmt.Errorf("resolve config path: %w", err)
	}

	// Full CODE-05 layer set: system → user → project → BEEKEEPER_* env → flags.
	// SystemPath and ProjectPath are skipped silently by LoadLayered when absent,
	// so the project layer overrides the user layer WITHOUT requiring env vars
	// (SC2). Environ MUST be os.Environ() in production or the BEEKEEPER_* layer
	// is silently dead.
	opts := config.LayerOpts{
		SystemPath:  systemConfigPath(),
		UserPath:    userPath,
		ProjectPath: discoverProjectConfig(userPath),
		Environ:     os.Environ(),
	}

	cfg, err := config.LoadLayered(opts)
	if err != nil {
		return config.Config{}, fmt.Errorf("load layered config: %w", err)
	}
	return cfg, nil
}

// systemConfigPath returns the system-wide config path per PRD §9
// (/etc/beekeeper/config.json). On platforms without /etc the file simply does
// not exist and LoadLayered skips the layer silently.
func systemConfigPath() string {
	return filepath.Join("/etc", "beekeeper", "config.json")
}

// discoverProjectConfig walks up from the current working directory (git-style)
// looking for a .beekeeper/config.json, returning the first match. Returns ""
// when none is found, which makes LoadLayered skip the project layer. This is
// what lets a project-level config override user config without env vars (SC2).
//
// The user-level config (userPath, typically ~/.beekeeper/config.json) is never
// returned as a project config, and the walk stops at the home directory so it
// does not climb into other users' homes or the filesystem root.
func discoverProjectConfig(userPath string) string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	home, _ := os.UserHomeDir()
	for {
		candidate := filepath.Join(dir, ".beekeeper", "config.json")
		if candidate != userPath {
			if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
				return candidate
			}
		}
		if home != "" && dir == home {
			return "" // do not search at or above the user's home directory
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "" // reached filesystem root without a match
		}
		dir = parent
	}
}
