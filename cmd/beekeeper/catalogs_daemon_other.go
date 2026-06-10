//go:build !linux && !darwin && !windows

package main

import (
	"fmt"
	"io"
)

// Unsupported-OS stubs for the unprivileged catalog-sync daemon. They return a
// clear error rather than silently no-op'ing (fail-closed surfacing).
func installCatalogDaemon(_ io.Writer, _ string) error {
	return fmt.Errorf("catalogs daemon is not supported on this platform")
}

func uninstallCatalogDaemon(_ io.Writer) error {
	return fmt.Errorf("catalogs daemon is not supported on this platform")
}

func catalogDaemonStatus() (bool, string, error) {
	return false, "unsupported platform", nil
}
