//go:build darwin

package darwin

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/home-beekeeper/beekeeper/internal/sentry"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

func TestParseExecEvent(t *testing.T) {
	data := loadFixture(t, "exec_event.json")
	ev, err := parseEsloggerLine(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Kind != sentry.EventProcessCreate {
		t.Errorf("Kind = %v, want EventProcessCreate", ev.Kind)
	}
	if ev.PID != 12345 {
		t.Errorf("PID = %d, want 12345", ev.PID)
	}
	if ev.PPID != 6789 {
		t.Errorf("PPID = %d, want 6789", ev.PPID)
	}
	if ev.UID != 501 {
		t.Errorf("UID = %d, want 501", ev.UID)
	}
	if ev.Exe != "/usr/bin/git" {
		t.Errorf("Exe = %q, want /usr/bin/git", ev.Exe)
	}
	if ev.Cmdline != "git push" {
		t.Errorf("Cmdline = %q, want 'git push'", ev.Cmdline)
	}
	if ev.WallTime.IsZero() {
		t.Error("WallTime is zero")
	}
}

func TestParseOpenEvent(t *testing.T) {
	data := loadFixture(t, "open_event.json")
	ev, err := parseEsloggerLine(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Kind != sentry.EventFileAccess {
		t.Errorf("Kind = %v, want EventFileAccess", ev.Kind)
	}
	if ev.PID != 999 {
		t.Errorf("PID = %d, want 999", ev.PID)
	}
	if ev.FilePath != "/Users/me/.ssh/id_rsa" {
		t.Errorf("FilePath = %q, want /Users/me/.ssh/id_rsa", ev.FilePath)
	}
	if ev.Exe != "/bin/cat" {
		t.Errorf("Exe = %q, want /bin/cat", ev.Exe)
	}
}

func TestParseCreateEvent(t *testing.T) {
	data := loadFixture(t, "create_event.json")
	ev, err := parseEsloggerLine(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Kind != sentry.EventFileAccess {
		t.Errorf("Kind = %v, want EventFileAccess", ev.Kind)
	}
	if ev.FilePath != "/tmp/payload" {
		t.Errorf("FilePath = %q, want /tmp/payload", ev.FilePath)
	}
}

// TestParseCreateNewFileUnion proves the union fix: a new-file create whose path
// lives only in new_path.dir+filename is no longer dropped (FilePath non-empty).
func TestParseCreateNewFileUnion(t *testing.T) {
	data := loadFixture(t, "create_newfile_event.json")
	ev, err := parseEsloggerLine(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.FilePath != "/Users/me/.claude/settings.json" {
		t.Errorf("FilePath = %q, want /Users/me/.claude/settings.json (new_path union)", ev.FilePath)
	}
}

// TestParseWriteEvent proves a write event parses to EventFileWrite.
func TestParseWriteEvent(t *testing.T) {
	data := loadFixture(t, "write_event.json")
	ev, err := parseEsloggerLine(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Kind != sentry.EventFileWrite {
		t.Errorf("Kind = %v, want EventFileWrite", ev.Kind)
	}
	if ev.FilePath != "/Users/me/.vscode/tasks.json" {
		t.Errorf("FilePath = %q, want /Users/me/.vscode/tasks.json", ev.FilePath)
	}
}

// TestParseRenameEvent proves a rename event parses to EventFileWrite with the
// destination path (the write-temp-then-rename persistence pattern).
func TestParseRenameEvent(t *testing.T) {
	data := loadFixture(t, "rename_event.json")
	ev, err := parseEsloggerLine(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Kind != sentry.EventFileWrite {
		t.Errorf("Kind = %v, want EventFileWrite", ev.Kind)
	}
	if ev.FilePath != "/Users/me/Library/LaunchAgents/com.evil.plist" {
		t.Errorf("FilePath = %q, want the rename destination path", ev.FilePath)
	}
}

func TestParseNetworkEvent(t *testing.T) {
	data := loadFixture(t, "network_event.json")
	ev, err := parseEsloggerLine(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Kind != sentry.EventNetworkConnect {
		t.Errorf("Kind = %v, want EventNetworkConnect", ev.Kind)
	}
	if ev.PID != 4242 {
		t.Errorf("PID = %d, want 4242", ev.PID)
	}
	if ev.DstAddr == nil || ev.DstAddr.String() != "52.14.222.1" {
		t.Errorf("DstAddr = %v, want 52.14.222.1", ev.DstAddr)
	}
	if ev.DstPort != 443 {
		t.Errorf("DstPort = %d, want 443", ev.DstPort)
	}
}

func TestParseInvalidJSON(t *testing.T) {
	_, err := parseEsloggerLine([]byte("{not json"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ParseError) {
		t.Errorf("expected ParseError, got %v", err)
	}
}

func TestParseUnknownEventType(t *testing.T) {
	data := []byte(`{"event_type":"frobnicate","process":{"audit_token":{"pid":1,"uid":0}}}`)
	_, err := parseEsloggerLine(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ParseError) {
		t.Errorf("expected ParseError, got %v", err)
	}
}

func TestParseMissingTimeFallsBackToNow(t *testing.T) {
	data := []byte(`{"event_type":"exec","time":"","process":{"audit_token":{"pid":1,"uid":0},"ppid":0,"executable":{"path":"/bin/sh"}},"event":{"exec":{"target":{"audit_token":{"pid":1,"uid":0},"ppid":0,"executable":{"path":"/bin/sh"},"args":["sh"]}}}}`)
	ev, err := parseEsloggerLine(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if time.Since(ev.WallTime) > 5*time.Second {
		t.Errorf("WallTime %v is more than 5s in the past", ev.WallTime)
	}
}
