package watch

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ExtensionHandler is implemented by types that process newly-detected
// extension directories. HandleNewExtension is called from a goroutine; it
// must be safe for concurrent invocation.
type ExtensionHandler interface {
	HandleNewExtension(ctx context.Context, path string)
}

// WatchConfig controls debounce and retry behaviour.
type WatchConfig struct {
	// DebounceWindow is how long to wait after the last event for a given path
	// before invoking the handler. Defaults to 500 ms when zero.
	DebounceWindow time.Duration
	// RetryInterval is how frequently pending (non-existent) directories are
	// re-added to the watcher. Defaults to 30 s when zero.
	RetryInterval time.Duration
}

// Watch starts the fsnotify watcher loop. It monitors each directory in dirs,
// retrying non-existent ones every RetryInterval. When a qualifying event
// arrives the handler is invoked after DebounceWindow.
//
// Watch blocks until ctx is cancelled, then returns nil.
func Watch(ctx context.Context, dirs []string, cfg WatchConfig, handler ExtensionHandler) error {
	if cfg.DebounceWindow == 0 {
		cfg.DebounceWindow = 500 * time.Millisecond
	}
	if cfg.RetryInterval == 0 {
		cfg.RetryInterval = 30 * time.Second
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	// Expand home dirs and attempt initial Add; track failures for retry.
	var pending []string
	for _, d := range dirs {
		expanded := expandHome(d)
		if err := w.Add(expanded); err != nil {
			log.Printf("beekeeper watch: cannot watch %q (will retry): %v", expanded, err)
			pending = append(pending, expanded)
		}
	}

	retryTicker := time.NewTicker(cfg.RetryInterval)
	defer retryTicker.Stop()

	// debounce maps path → active timer. Access is confined to this goroutine.
	debounce := make(map[string]*time.Timer)

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-retryTicker.C:
			var stillPending []string
			for _, d := range pending {
				if err := w.Add(d); err != nil {
					stillPending = append(stillPending, d)
				}
			}
			pending = stillPending

		case event, ok := <-w.Events:
			if !ok {
				return nil
			}
			if !shouldProcess(event) {
				continue
			}
			processEvent(ctx, event.Name, cfg.DebounceWindow, debounce, handler)

		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			log.Printf("beekeeper watch: watcher error: %v", err)
		}
	}
}

// processEvent applies the debounce logic for a single path event. It is
// factored out of the Watch loop so tests can drive it directly without a
// real fsnotify watcher.
func processEvent(ctx context.Context, path string, window time.Duration, debounce map[string]*time.Timer, handler ExtensionHandler) {
	if t, exists := debounce[path]; exists {
		t.Stop()
	}
	p := path // capture for closure
	debounce[p] = time.AfterFunc(window, func() {
		go handler.HandleNewExtension(ctx, p)
	})
}

// shouldProcess reports whether the fsnotify event is one that Beekeeper should
// act on. On Windows only Create events are relevant (Rename is not fired for
// new installs). On Linux/macOS both Create and Rename qualify.
func shouldProcess(event fsnotify.Event) bool {
	if runtime.GOOS == "windows" {
		return event.Has(fsnotify.Create)
	}
	return event.Has(fsnotify.Create) || event.Has(fsnotify.Rename)
}

// expandHome replaces a leading "~" with the current user's home directory.
// If os.UserHomeDir returns an error the original path is returned unchanged.
func expandHome(dir string) string {
	if len(dir) == 0 || dir[0] != '~' {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return dir
	}
	return filepath.Join(home, dir[1:])
}
