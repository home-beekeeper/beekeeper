package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/bantuson/beekeeper/internal/catalog"
	"github.com/bantuson/beekeeper/internal/platform"
)

// catalogSyncSourceName is the SourceState key for the Bumblebee catalog in
// state.json. It matches the `catalogs verify --source` default and the Watch
// daemon's source name, so the sync freshness fields live alongside the watch
// daemon's Hash/Count/Degraded fields for the same source.
const catalogSyncSourceName = "bumblebee"

// catalogSyncNow is the clock seam for the interval gate, overridable in tests.
var catalogSyncNow = func() time.Time { return time.Now() }

// catalogSyncDue reports whether a catalog sync is due. force always wins; a
// never-synced source (zero LastSuccess) is always due; otherwise the sync is
// due once at least interval has elapsed since the last success. This pure
// function is the injected-clock seam the interval-gate test drives directly
// (D-T1-interval).
func catalogSyncDue(lastSuccess time.Time, interval time.Duration, now time.Time, force bool) bool {
	if force {
		return true
	}
	if lastSuccess.IsZero() {
		return true // never synced — always due
	}
	return now.Sub(lastSuccess) >= interval
}

// runCatalogsSync performs an interval-gated, ETag-conditional catalog sync and
// records the freshness fields (LastAttempt/LastSuccess/LastError/ETag) in
// state.json. The OS scheduler fires this on a frequent (hourly) heartbeat; the
// interval gate makes it a no-op unless the configured cadence has elapsed, so
// the OS schedule never has to be rewritten when the config interval changes
// (D-T1-interval). force bypasses both the interval gate and a disabled config.
func runCatalogsSync(cmd *cobra.Command, force bool) error {
	dir, err := platform.CatalogDir()
	if err != nil {
		return fmt.Errorf("resolve catalog directory: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create catalog directory %q: %w", dir, err)
	}
	stateDir, err := platform.StateDir()
	if err != nil {
		return fmt.Errorf("resolve state directory: %w", err)
	}
	stateFile := filepath.Join(stateDir, "state.json")

	cfg, err := resolveConfig(cmd)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	st, err := catalog.LoadState(stateFile)
	if err != nil {
		return fmt.Errorf("load state %q: %w", stateFile, err)
	}
	ss := st.Sources[catalogSyncSourceName] // zero value if absent — preserves any watch-daemon fields

	out := cmd.OutOrStdout()
	interval := cfg.CatalogSyncInterval()
	now := catalogSyncNow()

	if !cfg.CatalogSyncEnabled() && !force {
		fmt.Fprintln(out, "Catalog sync is disabled (catalog_sync.enabled=false). Use --force to sync once.")
		return nil
	}
	if !catalogSyncDue(ss.LastSuccess, interval, now, force) {
		nextDue := ss.LastSuccess.Add(interval)
		fmt.Fprintf(out, "Catalog sync skipped: not due (last success %s ago, interval %s, next due %s). Use --force to override.\n",
			now.Sub(ss.LastSuccess).Round(time.Second), interval, nextDue.Format(time.RFC3339))
		return nil
	}

	client := &http.Client{Timeout: 30 * time.Second}
	res, syncErr := catalog.SyncConditional(cmd.Context(), client, dir, ss.ETag)

	ss.LastAttempt = now
	if syncErr != nil {
		// Record the failed attempt + error so the TUI shows amber, not "fresh".
		// The last-good index is preserved by SyncConditional (it errors before
		// any WriteFile/BuildIndex), so we only persist the freshness fields.
		ss.LastError = syncErr.Error()
		st.Sources[catalogSyncSourceName] = ss
		if saveErr := catalog.SaveState(stateFile, st); saveErr != nil {
			fmt.Fprintf(os.Stderr, "beekeeper: failed to record sync error in state: %v\n", saveErr)
		}
		return fmt.Errorf("catalog sync failed: %w", syncErr)
	}

	// Success (200 fetch+rebuild OR 304 not-modified).
	ss.LastSuccess = now
	ss.LastError = ""
	ss.ETag = res.ETag
	if !res.NotModified {
		ss.Count = res.Count
	}
	st.Sources[catalogSyncSourceName] = ss
	if err := catalog.SaveState(stateFile, st); err != nil {
		return fmt.Errorf("save state %q: %w", stateFile, err)
	}

	if res.NotModified {
		fmt.Fprintf(out, "Catalog unchanged (304); %d entries cached.\n", ss.Count)
	} else {
		fmt.Fprintf(out, "Synced %d catalog entries\n", res.Count)
	}
	fmt.Fprintf(out, "Index: %s\n", filepath.Join(dir, "bumblebee.idx"))

	// Phase 9 (CTLG-04): run the self-quarantine check AFTER every sync so a
	// newly-published compromise entry is acted on immediately.
	if sqErr := enforceSelfQuarantine(cmd); sqErr != nil {
		return sqErr
	}
	return nil
}

// newCatalogsDaemonCmd builds the `catalogs daemon install|uninstall|status`
// subcommand tree. The daemon is strictly UNPRIVILEGED — it registers a
// user-level OS job (systemd --user timer / launchd LaunchAgent / current-user
// schtasks) that runs `beekeeper catalogs sync` on an hourly heartbeat; the
// interval gate inside `catalogs sync` enforces the configured cadence
// (D-T1-host / D-T1-interval). No elevation is requested on any OS.
func newCatalogsDaemonCmd() *cobra.Command {
	daemon := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the unprivileged background catalog-sync scheduler (user-level OS job)",
		Long: `Register an unprivileged user-level OS job that runs ` + "`beekeeper catalogs sync`" + ` on
an hourly heartbeat so threat intel stays fresh without manual syncs. The job
runs as the current user with NO elevation (systemd --user / LaunchAgent /
current-user schtasks). The configured catalog_sync.interval gates each run, so
the OS schedule never needs rewriting when you change the interval.`,
	}

	daemon.AddCommand(&cobra.Command{
		Use:   "install",
		Short: "Register the unprivileged hourly catalog-sync job for the current user",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			self, err := os.Executable()
			if err != nil {
				return fmt.Errorf("resolve executable: %w", err)
			}
			return installCatalogDaemon(cmd.OutOrStdout(), self)
		},
	})
	daemon.AddCommand(&cobra.Command{
		Use:   "uninstall",
		Short: "Remove the catalog-sync job for the current user",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return uninstallCatalogDaemon(cmd.OutOrStdout())
		},
	})
	daemon.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Report whether the catalog-sync job is registered",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			installed, detail, err := catalogDaemonStatus()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if installed {
				fmt.Fprintf(out, "Catalog sync daemon: installed — %s\n", detail)
			} else {
				fmt.Fprintf(out, "Catalog sync daemon: not installed (%s)\n", detail)
			}
			return nil
		},
	})
	return daemon
}

// offerCatalogSyncDaemon is the CSYNC-06 first-run hook invoked after a real
// `hooks install`. It performs one best-effort first-run catalog sync (so a
// fresh threat-intel index exists immediately) and prints how to register the
// unprivileged background sync daemon. Daemon registration is opt-in (the user
// runs `catalogs daemon install`); the first-run sync is best-effort and never
// fails the install.
func offerCatalogSyncDaemon(cmd *cobra.Command) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "Running first catalog sync to populate the threat-intel index...")
	if err := runCatalogsSync(cmd, true); err != nil {
		fmt.Fprintf(out, "  First-run catalog sync skipped (%v) — run `beekeeper catalogs sync` later.\n", err)
	}
	fmt.Fprintln(out, "To keep catalogs fresh automatically (unprivileged, no elevation), run:")
	fmt.Fprintln(out, "  beekeeper catalogs daemon install")
}
