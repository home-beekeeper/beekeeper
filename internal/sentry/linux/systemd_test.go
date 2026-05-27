//go:build linux

package linux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteUnitFile(t *testing.T) {
	tmpDir := t.TempDir()
	orig := unitFilePath
	unitFilePath = filepath.Join(tmpDir, "beekeeper-sentry.service")
	defer func() { unitFilePath = orig }()

	path, err := WriteUnitFile("/usr/local/bin/beekeeper")
	if err != nil {
		t.Fatalf("WriteUnitFile: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	s := string(content)
	for _, want := range []string{
		"ExecStart=/usr/local/bin/beekeeper sentry",
		"Type=notify",
		"CAP_BPF",
		"ProtectSystem=strict",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("unit file missing %q", want)
		}
	}
}

func TestIsSystemdRunningReturnsValue(t *testing.T) {
	result := IsSystemdRunning() // just verify no panic
	_ = result
}
