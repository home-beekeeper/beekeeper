// Command beekeeper is the real-time safety harness for autonomous coding
// agents. This file contains ONLY Cobra command wiring per the project
// architecture constraint — all business logic lives in internal/ packages.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/mzansi-agentive/beekeeper/internal/audit"
	"github.com/mzansi-agentive/beekeeper/internal/catalog"
	"github.com/mzansi-agentive/beekeeper/internal/check"
	"github.com/mzansi-agentive/beekeeper/internal/config"
	"github.com/mzansi-agentive/beekeeper/internal/editorinit"
	"github.com/mzansi-agentive/beekeeper/internal/notify"
	"github.com/mzansi-agentive/beekeeper/internal/platform"
	"github.com/mzansi-agentive/beekeeper/internal/quarantine"
	"github.com/mzansi-agentive/beekeeper/internal/scan"
	"github.com/mzansi-agentive/beekeeper/internal/version"
	"github.com/mzansi-agentive/beekeeper/internal/watch"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "beekeeper",
		Short:        "Real-time safety harness for autonomous coding agents",
		Long:         "Beekeeper intercepts agent tool calls before they execute and evaluates them against unified threat intelligence.",
		SilenceUsage: true,
	}

	root.AddCommand(
		newVersionCmd(),
		newInitCmd(),
		newCheckCmd(),
		newCatalogsCmd(),
		newAuditCmd(),
		newSelftestCmd(),
		newWatchCmd(),
		newScanCmd(),
		newQuarantineCmd(),
	)

	return root
}

// newVersionCmd prints the build metadata injected via ldflags. Fully
// implemented in Phase 1, Plan 01.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version, commit, and build date",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "version: %s\n", version.Version)
			fmt.Fprintf(out, "commit:  %s\n", version.Commit)
			fmt.Fprintf(out, "date:    %s\n", version.Date)
			return nil
		},
	}
}

// newInitCmd creates the Beekeeper state directory tree and, on first run,
// detects installed editors and gates configuration changes on explicit consent.
// EDXT-06: extends Phase 1 dir creation with quarantine/marketplace-cache dirs
// plus per-editor consent for auto-update disable and watch-dir registration.
func newInitCmd() *cobra.Command {
	var yes, noEditors bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create the Beekeeper state directory and configure editor protection",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			stateDir, err := platform.StateDir()
			if err != nil {
				return fmt.Errorf("resolve state directory: %w", err)
			}
			catalogDir, err := platform.CatalogDir()
			if err != nil {
				return fmt.Errorf("resolve catalog directory: %w", err)
			}
			auditDir, err := platform.AuditDir()
			if err != nil {
				return fmt.Errorf("resolve audit directory: %w", err)
			}
			configPath, err := platform.ConfigPath()
			if err != nil {
				return fmt.Errorf("resolve config path: %w", err)
			}

			// Phase 1: create state, catalogs, audit directories.
			for _, dir := range []string{stateDir, catalogDir, auditDir} {
				if err := os.MkdirAll(dir, 0700); err != nil {
					return fmt.Errorf("create directory %q: %w", dir, err)
				}
			}

			// Phase 3: create quarantine/extensions and marketplace-cache.
			for _, dir := range []string{
				filepath.Join(stateDir, "quarantine", "extensions"),
				filepath.Join(catalogDir, "marketplace-cache"),
			} {
				if err := os.MkdirAll(dir, 0700); err != nil {
					return fmt.Errorf("create directory %q: %w", dir, err)
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Initialized Beekeeper state directory at %s\n", stateDir)

			if noEditors {
				return nil
			}

			// EDXT-06: detect editors and gate configuration on consent.
			editors, err := editorinit.DetectEditors()
			if err != nil || len(editors) == 0 {
				return nil
			}

			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			out := cmd.OutOrStdout()
			in := cmd.InOrStdin()
			configDirty := false

			for _, editor := range editors {
				if editor.SettingsPath != "" {
					var consent bool
					if yes {
						consent = true
					} else {
						fmt.Fprintf(out, "Disable extension auto-update for %s? NOTE: comments in settings.json will be removed. [y/N] ", editor.Name)
						var resp string
						fmt.Fscanln(in, &resp)
						consent = resp == "y" || resp == "Y"
					}
					if consent {
						if err := editorinit.DisableExtensionAutoUpdate(editor.SettingsPath); err != nil {
							fmt.Fprintf(out, "  Warning: could not disable auto-update for %s: %v\n", editor.Name, err)
						} else {
							fmt.Fprintf(out, "  Disabled extension auto-update for %s\n", editor.Name)
						}
					}
				}

				if editor.ExtensionDir != "" {
					var consent bool
					if yes {
						consent = true
					} else {
						fmt.Fprintf(out, "Enable Beekeeper file-watcher for %s (%s)? [y/N] ", editor.Name, editor.ExtensionDir)
						var resp string
						fmt.Fscanln(in, &resp)
						consent = resp == "y" || resp == "Y"
					}
					if consent {
						cfg.AddWatchDirectory(editor.ExtensionDir)
						configDirty = true
						fmt.Fprintf(out, "  Registered watch directory: %s\n", editor.ExtensionDir)
					}
				}
			}

			if configDirty {
				if err := config.Save(configPath, cfg); err != nil {
					return fmt.Errorf("save config: %w", err)
				}
			}

			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Auto-consent to all editor configuration prompts")
	cmd.Flags().BoolVar(&noEditors, "no-editors", false, "Skip editor detection (scripted installs)")
	return cmd
}

// newCheckCmd is the hook handler entry point. It reads a tool call from stdin,
// evaluates it against the mmap catalog index under hard caps, writes an audit
// record, and exits 0 (allow) or non-zero (block) — failing CLOSED on any
// crash, timeout, oversized input, malformed JSON, or missing index.
func newCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Evaluate a tool call read from stdin (allow=0, block!=0)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			catalogDir, err := platform.CatalogDir()
			if err != nil {
				return fmt.Errorf("resolve catalog directory: %w", err)
			}
			auditDir, err := platform.AuditDir()
			if err != nil {
				return fmt.Errorf("resolve audit directory: %w", err)
			}
			configPath, err := platform.ConfigPath()
			if err != nil {
				return fmt.Errorf("resolve config path: %w", err)
			}

			indexPath := filepath.Join(catalogDir, "bumblebee.idx")
			auditPath := filepath.Join(auditDir, "beekeeper.ndjson")

			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			result := check.RunCheck(cmd.Context(), os.Stdin, cfg, indexPath, auditPath, catalogDir)
			os.Exit(result.ExitCode)
			return nil // unreachable; os.Exit above is the real return path
		},
	}
}

// newCatalogsCmd groups catalog-management subcommands.
func newCatalogsCmd() *cobra.Command {
	catalogs := &cobra.Command{
		Use:   "catalogs",
		Short: "Manage cached threat-intel catalogs",
	}
	catalogs.AddCommand(&cobra.Command{
		Use:   "sync",
		Short: "Fetch and cache catalogs, then build the mmap index",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := platform.CatalogDir()
			if err != nil {
				return fmt.Errorf("resolve catalog directory: %w", err)
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("create catalog directory %q: %w", dir, err)
			}

			client := &http.Client{Timeout: 30 * time.Second}
			n, err := catalog.Sync(cmd.Context(), client, dir)
			if err != nil {
				return fmt.Errorf("catalog sync failed: %w", err)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Synced %d catalog entries\n", n)
			fmt.Fprintf(out, "Index: %s\n", filepath.Join(dir, "bumblebee.idx"))
			return nil
		},
	})

	// catalogs watch — foreground catalog watch daemon.
	catalogs.AddCommand(&cobra.Command{
		Use:   "watch",
		Short: "Poll catalog sources and trigger re-scans on delta (Ctrl+C to stop)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			catalogDir, err := platform.CatalogDir()
			if err != nil {
				return fmt.Errorf("resolve catalog directory: %w", err)
			}
			stateDir, err := platform.StateDir()
			if err != nil {
				return fmt.Errorf("resolve state directory: %w", err)
			}
			stateFile := filepath.Join(stateDir, "state.json")

			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			client := &http.Client{Timeout: 30 * time.Second}
			cfg := catalog.WatchConfig{
				PollInterval: time.Hour,
				CatalogDir:   catalogDir,
				StateFile:    stateFile,
				Client:       client,
				Sanity:       catalog.DefaultSanityConfig(),
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Starting catalog watch daemon (interval: %s, Ctrl+C to stop)...\n", cfg.PollInterval)

			return catalog.Watch(ctx, cfg, func(delta catalog.CatalogDelta, sanity catalog.SanityResult) {
				if sanity.Block {
					fmt.Fprintf(out, "catalog delta [HARD-BLOCK sanity breach]: source=%s prev=%d new=%d delta=%d — %s\n",
						delta.Source, delta.PrevCount, delta.NewCount, delta.DeltaCount, sanity.Reason)
				} else if sanity.Alert {
					fmt.Fprintf(out, "catalog delta [ALERT sanity breach]: source=%s prev=%d new=%d delta=%d — %s\n",
						delta.Source, delta.PrevCount, delta.NewCount, delta.DeltaCount, sanity.Reason)
				} else if delta.HasChanges() {
					fmt.Fprintf(out, "catalog delta: source=%s prev_count=%d new_count=%d delta=%d prev_hash=%s new_hash=%s\n",
						delta.Source, delta.PrevCount, delta.NewCount, delta.DeltaCount,
						delta.PrevHash, delta.NewHash)
				}
			})
		},
	})

	// catalogs verify — clear degraded mode for a named catalog source (CTLG-08).
	var verifySource string
	verifyCmd := &cobra.Command{
		Use:   "verify",
		Short: "Clear degraded mode for a catalog source after operator review",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if verifySource == "" {
				return fmt.Errorf("--source is required")
			}

			stateDir, err := platform.StateDir()
			if err != nil {
				return fmt.Errorf("resolve state directory: %w", err)
			}
			stateFile := filepath.Join(stateDir, "state.json")

			st, err := catalog.LoadState(stateFile)
			if err != nil {
				return fmt.Errorf("load state %q: %w", stateFile, err)
			}

			ss, ok := st.Sources[verifySource]
			if !ok {
				fmt.Fprintf(cmd.OutOrStdout(), "Source %q not found in state; nothing to clear.\n", verifySource)
				return nil
			}

			if !ss.Degraded {
				fmt.Fprintf(cmd.OutOrStdout(), "Source %q is not degraded; nothing to clear.\n", verifySource)
				return nil
			}

			ss.Degraded = false
			ss.DegradedReason = ""
			st.Sources[verifySource] = ss

			if err := catalog.SaveState(stateFile, st); err != nil {
				return fmt.Errorf("save state %q: %w", stateFile, err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Cleared degraded mode for source %q.\n", verifySource)
			return nil
		},
	}
	verifyCmd.Flags().StringVar(&verifySource, "source", "", "catalog source name to clear (e.g. bumblebee)")
	catalogs.AddCommand(verifyCmd)

	return catalogs
}

// newWatchCmd starts the foreground extension file-watcher daemon.
// It detects watch directories from config or DetectEditors, then runs
// watch.Watch until SIGINT/SIGTERM (Ctrl+C).
func newWatchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "watch",
		Short: "Watch extension directories for new installations (Ctrl+C to stop)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			catalogDir, err := platform.CatalogDir()
			if err != nil {
				return fmt.Errorf("resolve catalog directory: %w", err)
			}
			stateDir, err := platform.StateDir()
			if err != nil {
				return fmt.Errorf("resolve state directory: %w", err)
			}
			auditDir, err := platform.AuditDir()
			if err != nil {
				return fmt.Errorf("resolve audit directory: %w", err)
			}
			configPath, err := platform.ConfigPath()
			if err != nil {
				return fmt.Errorf("resolve config path: %w", err)
			}

			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			indexPath := filepath.Join(catalogDir, "bumblebee.idx")
			auditPath := filepath.Join(auditDir, "beekeeper.ndjson")
			quarantineDir := filepath.Join(stateDir, "quarantine")

			// Resolve watch directories: config > DetectEditors.
			var dirs []string
			if configDirs := cfg.WatchDirectories(); len(configDirs) > 0 {
				dirs = configDirs
			} else {
				editors, _ := editorinit.DetectEditors()
				for _, e := range editors {
					if e.ExtensionDir != "" {
						dirs = append(dirs, e.ExtensionDir)
					}
				}
			}

			handler := watch.NewHandler(
				indexPath,
				catalogDir,
				quarantineDir,
				auditPath,
				notify.Config{Enabled: true},
				cfg.SocketAPIToken(),
				&http.Client{Timeout: 4 * time.Second},
				func() time.Time { return time.Now().UTC() },
				dirs,
			)

			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Starting extension watch (Ctrl+C to stop)...\n")
			if len(dirs) == 0 {
				fmt.Fprintf(out, "Warning: no watch directories configured. Run 'beekeeper init' to detect editors.\n")
			}
			for _, d := range dirs {
				fmt.Fprintf(out, "Watching: %s\n", d)
			}

			return watch.Watch(ctx, dirs, watch.WatchConfig{}, handler)
		},
	}
}

// newScanCmd scans installed extensions using the Bumblebee CLI (when present)
// and the Beekeeper-own per-extension catalog/release-age engine.
func newScanCmd() *cobra.Command {
	var deep bool
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan installed extensions against catalog and release-age policy",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			catalogDir, err := platform.CatalogDir()
			if err != nil {
				return fmt.Errorf("resolve catalog directory: %w", err)
			}
			auditDir, err := platform.AuditDir()
			if err != nil {
				return fmt.Errorf("resolve audit directory: %w", err)
			}
			configPath, err := platform.ConfigPath()
			if err != nil {
				return fmt.Errorf("resolve config path: %w", err)
			}

			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			var dirs []string
			if configDirs := cfg.WatchDirectories(); len(configDirs) > 0 {
				dirs = configDirs
			} else {
				editors, _ := editorinit.DetectEditors()
				for _, e := range editors {
					if e.ExtensionDir != "" {
						dirs = append(dirs, e.ExtensionDir)
					}
				}
			}

			scanCfg := scan.Config{
				Deep:          deep,
				ExtensionDirs: dirs,
				IndexPath:     filepath.Join(catalogDir, "bumblebee.idx"),
				CacheDir:      catalogDir,
				AuditPath:     filepath.Join(auditDir, "beekeeper.ndjson"),
				SocketToken:   cfg.SocketAPIToken(),
				HTTPClient:    &http.Client{Timeout: 4 * time.Second},
				Now:           func() time.Time { return time.Now().UTC() },
			}

			return scan.Scan(cmd.Context(), scanCfg, cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&deep, "deep", false, "Run a deep scan (passes --profile deep to Bumblebee)")
	return cmd
}

// newQuarantineCmd groups quarantine management subcommands.
func newQuarantineCmd() *cobra.Command {
	qCmd := &cobra.Command{
		Use:   "quarantine",
		Short: "Manage quarantined extensions",
	}

	// list
	qCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List quarantined extensions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			stateDir, err := platform.StateDir()
			if err != nil {
				return fmt.Errorf("resolve state directory: %w", err)
			}
			qDir := filepath.Join(stateDir, "quarantine")
			manifests, err := quarantine.List(qDir)
			if err != nil {
				return fmt.Errorf("list quarantined extensions: %w", err)
			}
			out := cmd.OutOrStdout()
			if len(manifests) == 0 {
				fmt.Fprintln(out, "no quarantined items")
				return nil
			}
			fmt.Fprintf(out, "%-40s %-24s %-12s %-22s %s\n", "ID", "publisher.name", "version", "quarantined_at", "reason")
			for _, m := range manifests {
				fmt.Fprintf(out, "%-40s %-24s %-12s %-22s %s\n",
					m.ID,
					m.Publisher+"."+m.Name,
					m.Version,
					m.QuarantinedAt.Format(time.RFC3339),
					m.Reason,
				)
			}
			return nil
		},
	})

	// restore
	qCmd.AddCommand(&cobra.Command{
		Use:   "restore <id>",
		Short: "Restore a quarantined extension to its original location",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			stateDir, err := platform.StateDir()
			if err != nil {
				return fmt.Errorf("resolve state directory: %w", err)
			}
			auditDir, err := platform.AuditDir()
			if err != nil {
				return fmt.Errorf("resolve audit directory: %w", err)
			}
			qDir := filepath.Join(stateDir, "quarantine")
			auditPath := filepath.Join(auditDir, "beekeeper.ndjson")

			if err := quarantine.Restore(qDir, args[0]); err != nil {
				return fmt.Errorf("restore %q: %w", args[0], err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Restored %q\n", args[0])

			// Emit audit record for restore.
			rec := audit.AuditRecord{
				RecordType:       "quarantine_restore",
				RecordID:         args[0],
				Timestamp:        time.Now().UTC().Format(time.RFC3339),
				ScannerName:      "beekeeper",
				Decision:         "allow",
				Reason:           "operator restore",
				RuleIDs:          []string{"EDXT-05"},
				CatalogMatches:   []audit.CatalogProvenance{},
				SourcesAgreed:    []string{},
				SourcesDissented: []string{},
				Endpoint:         "quarantine",
			}
			if w, werr := audit.NewWriter(auditPath); werr == nil {
				_ = w.Write(rec)
				w.Close()
			}
			return nil
		},
	})

	// purge
	var purgeYes bool
	purgeCmd := &cobra.Command{
		Use:   "purge",
		Short: "Remove ALL quarantined extensions (prompts for confirmation unless --yes)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			stateDir, err := platform.StateDir()
			if err != nil {
				return fmt.Errorf("resolve state directory: %w", err)
			}
			auditDir, err := platform.AuditDir()
			if err != nil {
				return fmt.Errorf("resolve audit directory: %w", err)
			}
			qDir := filepath.Join(stateDir, "quarantine")
			auditPath := filepath.Join(auditDir, "beekeeper.ndjson")

			if !purgeYes {
				fmt.Fprint(cmd.OutOrStdout(), "Remove ALL quarantined items? [y/N] ")
				var resp string
				fmt.Fscanln(cmd.InOrStdin(), &resp)
				if resp != "y" && resp != "Y" {
					fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
					return nil
				}
			}

			purged, err := quarantine.Purge(qDir)
			if err != nil {
				return fmt.Errorf("purge quarantine: %w", err)
			}

			if len(purged) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No items to purge.")
				return nil
			}

			if w, werr := audit.NewWriter(auditPath); werr == nil {
				for _, id := range purged {
					rec := audit.AuditRecord{
						RecordType:       "quarantine_purge",
						RecordID:         id,
						Timestamp:        time.Now().UTC().Format(time.RFC3339),
						ScannerName:      "beekeeper",
						Decision:         "allow",
						Reason:           "operator purge",
						RuleIDs:          []string{"EDXT-05"},
						CatalogMatches:   []audit.CatalogProvenance{},
						SourcesAgreed:    []string{},
						SourcesDissented: []string{},
						Endpoint:         "quarantine",
					}
					_ = w.Write(rec)
				}
				w.Close()
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Purged %d item(s).\n", len(purged))
			return nil
		},
	}
	purgeCmd.Flags().BoolVar(&purgeYes, "yes", false, "Skip confirmation prompt")
	qCmd.AddCommand(purgeCmd)

	return qCmd
}

// newAuditCmd groups audit-log subcommands.
func newAuditCmd() *cobra.Command {
	audit := &cobra.Command{
		Use:   "audit",
		Short: "Inspect the Beekeeper audit log",
	}
	audit.AddCommand(&cobra.Command{
		Use:   "tail",
		Short: "Stream the live audit log to the terminal",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			auditDir, err := platform.AuditDir()
			if err != nil {
				return fmt.Errorf("resolve audit directory: %w", err)
			}
			logPath := filepath.Join(auditDir, "beekeeper.ndjson")
			return tailAuditLog(cmd.Context(), cmd.OutOrStdout(), logPath)
		},
	})
	return audit
}

// tailAuditLog streams the NDJSON audit log to out: it prints all existing
// content, then follows the file by polling for newly-appended bytes every
// ~500ms until the context is cancelled (Ctrl+C). Stdlib only — no external
// dependencies. The log is opened read-only; tailing never mutates it.
func tailAuditLog(ctx context.Context, out io.Writer, path string) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("audit log %q does not exist yet (run a beekeeper check first)", path)
		}
		return fmt.Errorf("open audit log %q: %w", path, err)
	}
	defer f.Close()

	offset, err := io.Copy(out, f)
	if err != nil {
		return fmt.Errorf("read audit log: %w", err)
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	buf := make([]byte, 32*1024)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			for {
				n, readErr := f.ReadAt(buf, offset)
				if n > 0 {
					if _, werr := out.Write(buf[:n]); werr != nil {
						return fmt.Errorf("write audit log to output: %w", werr)
					}
					offset += int64(n)
				}
				if readErr == io.EOF || n == 0 {
					break
				}
				if readErr != nil {
					return fmt.Errorf("follow audit log: %w", readErr)
				}
			}
		}
	}
}

// newSelftestCmd runs the embedded adversarial corpus and exits non-zero if any
// fixture produces an unexpected decision.
func newSelftestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "selftest",
		Short: "Run embedded adversarial fixtures as a sanity check",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			passed, failed, err := check.RunSelftest()
			fmt.Fprintf(cmd.OutOrStdout(), "PASS: %d, FAIL: %d\n", passed, failed)
			if err != nil {
				return fmt.Errorf("selftest error: %w", err)
			}
			if failed > 0 {
				os.Exit(1)
			}
			return nil
		},
	}
}
