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
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/mzansi-agentive/beekeeper/internal/catalog"
	"github.com/mzansi-agentive/beekeeper/internal/platform"
	"github.com/mzansi-agentive/beekeeper/internal/version"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "beekeeper",
		Short:         "Real-time safety harness for autonomous coding agents",
		Long:          "Beekeeper intercepts agent tool calls before they execute and evaluates them against unified threat intelligence.",
		SilenceUsage: true,
	}

	root.AddCommand(
		newVersionCmd(),
		newInitCmd(),
		newCheckCmd(),
		newCatalogsCmd(),
		newAuditCmd(),
		newSelftestCmd(),
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

// newInitCmd creates the Beekeeper state directory tree. This is the Phase 1
// stub: it creates state, catalogs/, and audit/ directories only — no editor
// detection or full onboarding (that is EDXT-06, Phase 3).
func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create the Beekeeper state directory",
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

			for _, dir := range []string{stateDir, catalogDir, auditDir} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("create directory %q: %w", dir, err)
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Initialized Beekeeper state directory at %s\n", stateDir)
			return nil
		},
	}
}

// newCheckCmd is the hook handler entry point. Implemented by Plan 05.
func newCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Evaluate a tool call read from stdin (allow=0, block!=0)",
		RunE: func(*cobra.Command, []string) error {
			return fmt.Errorf("not yet implemented")
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
	return catalogs
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

	// Stream existing content, then continue from the current end offset.
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

// newSelftestCmd runs embedded adversarial fixtures. Implemented by a later plan.
func newSelftestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "selftest",
		Short: "Run embedded adversarial fixtures as a sanity check",
		RunE: func(*cobra.Command, []string) error {
			return fmt.Errorf("not yet implemented")
		},
	}
}
