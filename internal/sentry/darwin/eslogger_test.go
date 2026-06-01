//go:build darwin

package darwin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bantuson/beekeeper/internal/sentry"
)

func TestDefaultEsloggerEventsIncludesOpenAndCreate(t *testing.T) {
	hasOpen, hasCreate := false, false
	for _, e := range DefaultEsloggerEvents {
		if e == "open" {
			hasOpen = true
		}
		if e == "create" {
			hasCreate = true
		}
	}
	if !hasOpen {
		t.Error("DefaultEsloggerEvents missing 'open'")
	}
	if !hasCreate {
		t.Error("DefaultEsloggerEvents missing 'create'")
	}
}

func TestEsloggerCommandShape(t *testing.T) {
	cmd := EsloggerCommand(context.Background(), []string{"exec", "open"})
	if filepath.Base(cmd.Path) != "eslogger" && !strings.HasSuffix(cmd.Path, "eslogger") {
		// cmd.Path may be empty if eslogger not in PATH on CI; check Args instead
		if len(cmd.Args) == 0 || cmd.Args[0] != "eslogger" {
			t.Errorf("expected eslogger command, got Path=%q Args=%v", cmd.Path, cmd.Args)
		}
	}
	if len(cmd.Args) < 3 {
		t.Fatalf("expected at least 3 args (eslogger exec open), got %v", cmd.Args)
	}
	if cmd.Args[1] != "exec" || cmd.Args[2] != "open" {
		t.Errorf("expected args [exec open], got %v", cmd.Args[1:])
	}
}

func TestDrainEsloggerHappyPath(t *testing.T) {
	fixtures := []string{"exec_event.json", "open_event.json", "network_event.json"}
	var lines []string
	for _, f := range fixtures {
		data, err := os.ReadFile(filepath.Join("testdata", f))
		if err != nil {
			t.Fatalf("read fixture %s: %v", f, err)
		}
		lines = append(lines, strings.TrimSpace(string(data)))
	}
	input := strings.NewReader(strings.Join(lines, "\n") + "\n")
	ch := make(chan sentry.SentryEvent, 10)
	if err := drainEslogger(input, ch); err != nil {
		t.Fatalf("drainEslogger error: %v", err)
	}
	if len(ch) != 3 {
		t.Errorf("expected 3 events, got %d", len(ch))
	}
}

func TestDrainEsloggerSkipsMalformed(t *testing.T) {
	execData, err := os.ReadFile(filepath.Join("testdata", "exec_event.json"))
	if err != nil {
		t.Fatal(err)
	}
	input := strings.NewReader("{bad json\n" + strings.TrimSpace(string(execData)) + "\n{also bad\n")
	ch := make(chan sentry.SentryEvent, 10)
	if err := drainEslogger(input, ch); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ch) != 1 {
		t.Errorf("expected 1 event, got %d", len(ch))
	}
}

func TestDrainEsloggerDropsOnFullChannel(t *testing.T) {
	before := atomic.LoadUint64(&EventsDropped)
	fixtures := []string{"exec_event.json", "open_event.json", "network_event.json"}
	var lines []string
	for _, f := range fixtures {
		data, err := os.ReadFile(filepath.Join("testdata", f))
		if err != nil {
			t.Fatalf("read fixture %s: %v", f, err)
		}
		lines = append(lines, strings.TrimSpace(string(data)))
	}
	input := strings.NewReader(strings.Join(lines, "\n") + "\n")
	ch := make(chan sentry.SentryEvent, 1)
	if err := drainEslogger(input, ch); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	after := atomic.LoadUint64(&EventsDropped)
	if after-before < 2 {
		t.Errorf("expected >= 2 drops, got %d", after-before)
	}
	// restore counter
	atomic.StoreUint64(&EventsDropped, before)
}
