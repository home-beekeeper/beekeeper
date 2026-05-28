//go:build darwin

package darwin

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"text/template"
)

const LaunchdLabel = "com.mzansi.beekeeper.sentry"

var plistPath = "/Library/LaunchDaemons/com.mzansi.beekeeper.sentry.plist"

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{.Label}}</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.BinPath}}</string>
		<string>sentry</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardErrorPath</key>
	<string>/var/log/beekeeper-sentry.log</string>
	<key>StandardOutPath</key>
	<string>/var/log/beekeeper-sentry.log</string>
	<key>UserName</key>
	<string>root</string>
</dict>
</plist>
`

// WritePlist renders the launchd plist to plistPath and returns that path.
func WritePlist(binPath string) (string, error) {
	tmpl, err := template.New("plist").Parse(plistTemplate)
	if err != nil {
		return "", fmt.Errorf("parse plist template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct{ Label, BinPath string }{LaunchdLabel, binPath}); err != nil {
		return "", fmt.Errorf("render plist: %w", err)
	}
	if err := os.WriteFile(plistPath, buf.Bytes(), 0644); err != nil {
		return "", fmt.Errorf("write plist: %w", err)
	}
	return plistPath, nil
}

// LaunchctlLoad loads the launchd job at the given plist path.
func LaunchctlLoad(ctx context.Context, path string) error {
	out, err := exec.CommandContext(ctx, "launchctl", "load", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl load: %w: %s", err, out)
	}
	return nil
}

// LaunchctlUnload unloads the launchd job. Exit code 113 ("not loaded") is
// treated as success for idempotent uninstall.
func LaunchctlUnload(ctx context.Context, path string) error {
	out, err := exec.CommandContext(ctx, "launchctl", "unload", path).CombinedOutput()
	if err == nil {
		return nil
	}
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 113 {
		return nil
	}
	return fmt.Errorf("launchctl unload: %w: %s", err, out)
}

// LaunchctlList reports whether the job with the given label is loaded.
func LaunchctlList(ctx context.Context, label string) (bool, error) {
	err := exec.CommandContext(ctx, "launchctl", "list", label).Run()
	if err == nil {
		return true, nil
	}
	if _, ok := err.(*exec.ExitError); ok {
		return false, nil
	}
	return false, err
}

// CoverageGapNotes returns a description of eslogger observability limitations
// for use by 'beekeeper protect status' (SMAC-04).
func CoverageGapNotes() string {
	return `macOS Sentry coverage gaps (eslogger-based, SMAC-04):
  - Keychain / Security framework API access: not observable
  - In-memory Cocoa API operations: not observable
  - SIP-protected processes: limited visibility`
}
