//go:build darwin

package darwin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWritePlistRendersCorrectFields(t *testing.T) {
	orig := plistPath
	plistPath = filepath.Join(t.TempDir(), "test.plist")
	t.Cleanup(func() { plistPath = orig })

	path, err := WritePlist("/usr/local/bin/beekeeper")
	if err != nil {
		t.Fatalf("WritePlist error: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read plist: %v", err)
	}
	s := string(content)

	checks := []string{
		"<string>com.mzansi.beekeeper.sentry</string>",
		"<string>/usr/local/bin/beekeeper</string>",
		"<string>sentry</string>",
		"<true/>",
		"<string>root</string>",
	}
	for _, c := range checks {
		if !strings.Contains(s, c) {
			t.Errorf("plist missing %q", c)
		}
	}
}

func TestLaunchctlListMissingLabel(t *testing.T) {
	running, err := LaunchctlList(context.Background(), "com.never.installed.xyz123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if running {
		t.Error("expected not running for non-existent label")
	}
}

func TestCoverageGapNotesIncludesKeychainAndCocoa(t *testing.T) {
	notes := CoverageGapNotes()
	if !strings.Contains(notes, "Keychain") {
		t.Error("CoverageGapNotes missing 'Keychain'")
	}
	if !strings.Contains(notes, "Cocoa") {
		t.Error("CoverageGapNotes missing 'Cocoa'")
	}
}
