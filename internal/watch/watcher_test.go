package watch

import (
	"context"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

// countHandler counts HandleNewExtension invocations atomically.
type countHandler struct {
	count atomic.Int64
}

func (h *countHandler) HandleNewExtension(_ context.Context, _ string) {
	h.count.Add(1)
}

func TestWatchNonExistentDir(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately so the loop exits without waiting.
	cancel()

	err := Watch(ctx, []string{filepath.Join(t.TempDir(), "does-not-exist")}, WatchConfig{
		DebounceWindow: 10 * time.Millisecond,
		RetryInterval:  10 * time.Millisecond,
	}, &countHandler{})
	if err != nil {
		t.Fatalf("Watch returned unexpected error: %v", err)
	}
}

func TestWatchWindowsFilter(t *testing.T) {
	// Create synthetic events.
	writeEvent := fsnotify.Event{Name: "/some/path", Op: fsnotify.Write}
	createEvent := fsnotify.Event{Name: "/some/path", Op: fsnotify.Create}
	renameEvent := fsnotify.Event{Name: "/some/path", Op: fsnotify.Rename}

	if shouldProcess(writeEvent) {
		t.Error("Write event should not be processed on any platform")
	}

	if !shouldProcess(createEvent) {
		t.Error("Create event should be processed on all platforms")
	}

	// Rename: only processed on non-Windows.
	if runtime.GOOS == "windows" {
		if shouldProcess(renameEvent) {
			t.Error("Rename event should not be processed on Windows")
		}
	} else {
		if !shouldProcess(renameEvent) {
			t.Error("Rename event should be processed on non-Windows")
		}
	}
}

func TestWatchDebounce(t *testing.T) {
	ctx := context.Background()
	handler := &countHandler{}
	window := 50 * time.Millisecond
	debounce := make(map[string]*time.Timer)

	const path = "/fake/ext/my-extension"

	// Fire 10 events for the same path within the debounce window.
	for i := 0; i < 10; i++ {
		processEvent(ctx, path, window, debounce, handler)
	}

	// Wait for the debounce timer to fire plus a small buffer.
	time.Sleep(window + 100*time.Millisecond)

	got := handler.count.Load()
	if got != 1 {
		t.Errorf("handler called %d times, want exactly 1", got)
	}
}
