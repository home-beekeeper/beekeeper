package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/home-beekeeper/beekeeper/internal/catalog"
	"github.com/home-beekeeper/beekeeper/internal/corpus"
	"github.com/home-beekeeper/beekeeper/internal/platform"
	"github.com/home-beekeeper/beekeeper/internal/policy"
	"github.com/home-beekeeper/beekeeper/internal/watch"
)

// catalogSyncSourceName is the SourceState key for the Bumblebee catalog in
// state.json. It matches the `catalogs verify --source` default and the Watch
// daemon's source name, so the sync freshness fields live alongside the watch
// daemon's Hash/Count/Degraded fields for the same source.
const catalogSyncSourceName = "bumblebee"

// firstResponderFn is the package-level injectable seam for runCatalogsSync.
// Mirrors scanOnDeltaFn: production code leaves it as watch.RunFirstResponder;
// cmd tests replace it with a no-op or a closure to isolate the FRB wiring.
var firstResponderFn = func(ctx context.Context, cfg watch.FirstResponderConfig) (watch.FirstResponderResult, error) {
	return watch.RunFirstResponder(ctx, cfg)
}

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
	// SEC (remediation 260615, #2/#3): the catalog directory holds the mmap index
	// that drives block decisions; create it owner-only so a non-owner local
	// process cannot list or replace its contents.
	if err := os.MkdirAll(dir, 0o700); err != nil {
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
	// errOut is the command's error stream (teed to sync.log under --background),
	// used in place of os.Stderr so a scheduled run's diagnostics are captured.
	errOut := cmd.ErrOrStderr()
	interval := cfg.CatalogSyncInterval()
	now := catalogSyncNow()

	// frResult is populated by the first-responder pass below (stays zero when
	// corpus is disabled); its counts feed the persisted SyncSummary so
	// `beekeeper catalogs status` reports what this run actually did.
	var frResult watch.FirstResponderResult

	// buildSummary constructs the SyncSummary for the current run. NextDue is
	// derived from the persisted LastSuccess + interval (zero when never synced).
	buildSummary := func(result, lastErr string, entries int) *catalog.SyncSummary {
		var nextDue time.Time
		if cur := st.Sources[catalogSyncSourceName]; !cur.LastSuccess.IsZero() {
			nextDue = cur.LastSuccess.Add(interval)
		}
		return &catalog.SyncSummary{
			At:              now,
			Result:          result,
			Entries:         entries,
			ScanHits:        frResult.ScanHits,
			Quarantined:     frResult.Quarantined,
			Pending:         frResult.Pending,
			WouldQuarantine: frResult.WouldQuarantine,
			LastError:       lastErr,
			NextDue:         nextDue,
		}
	}

	// Phase 23 (OQ-3 / ADJ-01): bounded adjudication batch pass.
	// Run BEFORE the HTTP catalog fetch (so a fetch failure never skips adjudication
	// of already-stored incidents — per 23-RESEARCH §Adjudicator Lifecycle).
	// Only when cfg.Corpus.Enabled; a batch-pass error MUST NOT fail runCatalogsSync.
	if cfg.Corpus.Enabled {
		corpusPath, cpErr := corpus.ResolveCorpusPath(cfg.Corpus, stateDir)
		if cpErr != nil {
			fmt.Fprintf(errOut, "beekeeper: corpus: resolve corpus path for adjudication: %v\n", cpErr)
		} else {
			// Default corroboration thresholds (PLCY-01). Thresholds from policy files
			// are not loaded here to keep the sync fast; catalog_confirmation uses the
			// global defaults (WarnAt 1 / BlockAt 2 / QuarantineAt 3).
			thresholds := policy.CorroborationThresholds{
				WarnAt:         1,
				BlockAt:        2,
				QuarantineAt:   3,
				CatalogHealthy: true,
			}

			// Open the mmap index best-effort for catalog_confirmation re-queries.
			// *catalog.Index does not directly implement policy.MultiCatalogLookup;
			// wrap it in a single-source MultiIndex (nil OSV/Socket for this batch pass).
			// A nil lookup → RunAdjudicationBatch skips catalog_confirmation queries safely.
			var idx policy.MultiCatalogLookup
			idxPath := filepath.Join(dir, "bumblebee.idx")
			if bbIdx, idxErr := catalog.OpenIndex(idxPath); idxErr == nil {
				// Wrap in MultiIndex: bumblebee only (no OSV/Socket for the batch pass).
				idx = catalog.NewMultiIndex(bbIdx, nil, nil)
				defer bbIdx.Close()
			}
			// nil idx → RunAdjudicationBatch skips catalog_confirmation queries safely.

			// 5-second deadline (T-23-12 / OQ-3): the batch pass is bounded so it
			// never stalls the hourly sync. Cancelled on deadline; writes whatever completed.
			batchCtx, batchCancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer batchCancel()

			// IN-02: reuse the outer stateFile computed above (line ~61); no
			// redundant recompute.
			cleanDays := cfg.CorpusDownstreamCleanDays()
			if batchErr := corpus.RunAdjudicationBatch(batchCtx, corpusPath, stateFile, idx, thresholds, cleanDays); batchErr != nil {
				// Non-fatal: log to stderr and continue. The sync must proceed.
				fmt.Fprintf(errOut, "beekeeper: corpus adjudication batch: %v\n", batchErr)
			}

			// Phase 24 (FRB-01/04): first-responder corpus pass.
			// Runs AFTER the adjudication batch so confirmed-malicious records are
			// already written before the first-responder reads them.
			// Resolve the audit directory for the audit log path.
			auditDir, adErr := platform.AuditDir()
			if adErr != nil {
				fmt.Fprintf(errOut, "beekeeper: corpus first-responder: resolve audit dir: %v\n", adErr)
				// Non-fatal: continue without the first-responder pass.
			} else {
				frRes, frErr := firstResponderFn(cmd.Context(), watch.FirstResponderConfig{
					CorpusPath:            corpusPath,
					CorpusEnabled:         cfg.Corpus.Enabled,
					CorpusSentryThreshold: 2,
					SentryTargetsPath:     filepath.Join(stateDir, "sentry-targets.json"),
					QuarantineDir:         filepath.Join(stateDir, "quarantine"),
					AuditPath:             filepath.Join(auditDir, "beekeeper.ndjson"),
					IndexPath:             filepath.Join(dir, "bumblebee.idx"),
					CacheDir:              dir,
					Enabled:               cfg.AutoQuarantineEnabled(),
					// SEC (remediation 260615, #3): thread the real dry-run and
					// threshold config. Previously DryRun was the zero value (false)
					// and Threshold was hardcoded 2, so the scheduled sync daemon
					// performed LIVE quarantine moves even when the operator set the
					// safe-default dry_run:true, and ignored a tightened threshold.
					DryRun:    cfg.AutoQuarantineDryRun(),
					Threshold: cfg.AutoQuarantineThreshold(),
				})
				frResult = frRes
				if frErr != nil {
					// Non-fatal: log to stderr and continue. The sync must proceed.
					fmt.Fprintf(errOut, "beekeeper: corpus first-responder: %v\n", frErr)
				}
			}

			// Phase 24 (FRB-05): local catalog overlay — one entry per malicious
			// corpus record. Runs after the first-responder pass so the overlay
			// reflects the same confirmed-malicious set that just armed the card.
			malicious, rdErr := corpus.ReadMaliciousRecords(corpusPath)
			if rdErr != nil {
				// Non-fatal: log to stderr and continue. The sync must proceed.
				fmt.Fprintf(errOut, "beekeeper: corpus: read malicious records for overlay: %v\n", rdErr)
			} else {
				for _, rec := range malicious {
					if rec.PushEnvelope == nil || rec.PushEnvelope.Signature.PackageOrExtensionID == "" {
						continue
					}
					if ovErr := catalog.AddLocalOverlayEntry(dir, buildOverlayEntry(rec)); ovErr != nil {
						// Non-fatal per record: log to stderr and continue.
						fmt.Fprintf(errOut, "beekeeper: catalog overlay: add entry: %v\n", ovErr)
					}
				}
			}
		}
	}

	if !cfg.CatalogSyncEnabled() && !force {
		fmt.Fprintln(out, "Catalog sync is disabled (catalog_sync.enabled=false). Use --force to sync once.")
		st.LastSync = buildSummary("disabled", "", ss.Count)
		if saveErr := catalog.SaveState(stateFile, st); saveErr != nil {
			fmt.Fprintf(errOut, "beekeeper: failed to record sync summary: %v\n", saveErr)
		}
		return nil
	}
	if !catalogSyncDue(ss.LastSuccess, interval, now, force) {
		nextDue := ss.LastSuccess.Add(interval)
		fmt.Fprintf(out, "Catalog sync skipped: not due (last success %s ago, interval %s, next due %s). Use --force to override.\n",
			now.Sub(ss.LastSuccess).Round(time.Second), interval, nextDue.Format(time.RFC3339))
		st.LastSync = buildSummary("skipped", "", ss.Count)
		if saveErr := catalog.SaveState(stateFile, st); saveErr != nil {
			fmt.Fprintf(errOut, "beekeeper: failed to record sync summary: %v\n", saveErr)
		}
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
		st.LastSync = buildSummary("error", syncErr.Error(), ss.Count)
		if saveErr := catalog.SaveState(stateFile, st); saveErr != nil {
			fmt.Fprintf(errOut, "beekeeper: failed to record sync error in state: %v\n", saveErr)
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
	syncResult := "synced"
	if res.NotModified {
		syncResult = "unchanged"
	}
	st.LastSync = buildSummary(syncResult, "", ss.Count)
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

// buildOverlayEntry constructs a catalog.Entry from a confirmed-malicious
// CorpusRecord for FRB-05 (local catalog overlay).
//
// Key properties:
//   - CatalogSignature is always "" (empty): unsigned overlay entries are
//     warn-only (source_count:1) per CTLG-07 corroboration semantics (Pitfall 3).
//   - CatalogSource is "local-overlay" so the policy engine counts it as a
//     distinct source in corroborate() deduplication.
//   - Ecosystem+package are parsed from PushEnvelope.Signature.PackageOrExtensionID
//     using the same rune-by-rune first-colon split as parsePackageID in firstresponder.go.
func buildOverlayEntry(rec corpus.CorpusRecord) catalog.Entry {
	id := rec.PushEnvelope.Signature.PackageOrExtensionID
	// Split on first ':' to separate ecosystem from package name.
	// Handles scoped npm names: "npm:@org/pkg" → ("npm", "@org/pkg").
	ecosystem, pkg := "", id
	for i, c := range id {
		if c == ':' {
			ecosystem, pkg = id[:i], id[i+1:]
			break
		}
	}

	version := ""
	if rec.PushEnvelope != nil {
		version = rec.PushEnvelope.Signature.Version
	}

	var versions []string
	if version != "" {
		versions = []string{version}
	}

	return catalog.Entry{
		ID:               "local-overlay-" + rec.AuditRecord.ClusterID,
		Name:             pkg,
		Ecosystem:        ecosystem,
		Package:          pkg,
		Versions:         versions,
		Severity:         "critical",
		SourceURL:        "",
		CatalogSignature: "", // MUST be empty — unsigned → warn-only per Pitfall 3
		CatalogSource:    "local-overlay",
	}
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
