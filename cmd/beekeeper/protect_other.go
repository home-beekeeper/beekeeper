//go:build !linux && !darwin

package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func runProtectInstall(cmd *cobra.Command, _ []string) error {
	fmt.Fprintln(cmd.OutOrStdout(), "beekeeper protect is not supported on this platform")
	return nil
}

func runProtectUninstall(cmd *cobra.Command, _ []string) error {
	fmt.Fprintln(cmd.OutOrStdout(), "beekeeper protect is not supported on this platform")
	return nil
}

func runProtectStatus(cmd *cobra.Command, _ []string) error {
	fmt.Fprintln(cmd.OutOrStdout(), "beekeeper protect is not supported on this platform")
	return nil
}

func runSentryDaemon(cmd *cobra.Command, _ []string) error {
	fmt.Fprintln(cmd.OutOrStdout(), "beekeeper sentry is only supported on Linux")
	return nil
}

func runSentryRulesList(cmd *cobra.Command, _ []string) error {
	fmt.Fprintln(cmd.OutOrStdout(), "beekeeper sentry is only supported on Linux")
	return nil
}

func runSentryRulesEnable(cmd *cobra.Command, args []string) error {
	_ = args
	fmt.Fprintln(cmd.OutOrStdout(), "beekeeper sentry is only supported on Linux")
	return nil
}

func runSentryRulesDisable(cmd *cobra.Command, args []string) error {
	_ = args
	fmt.Fprintln(cmd.OutOrStdout(), "beekeeper sentry is only supported on Linux")
	return nil
}
