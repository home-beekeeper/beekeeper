//go:build !windows

package main

// HideConsoleWindow is a no-op on non-Windows platforms: launchd (macOS) and
// systemd --user (Linux) run the scheduled sync detached with no console window,
// so there is nothing to hide. The `catalogs sync --background` path calls it
// unconditionally so the cross-platform code compiles and behaves identically
// everywhere; only Windows actually hides a window.
func HideConsoleWindow() {}
