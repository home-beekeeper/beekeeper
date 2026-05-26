// Package notify provides a best-effort desktop notification wrapper over beeep.
// Notifications are fire-and-forget: errors are swallowed, and a missing display
// environment (headless Linux) short-circuits without any error propagation.
package notify

import (
	"os"
	"runtime"

	"github.com/gen2brain/beeep"
)

// Config controls whether desktop notifications are sent.
type Config struct {
	Enabled bool
}

// notifyFunc is the notification back-end. It is package-level so tests can
// replace it with a stub without build tags or exported interfaces.
var notifyFunc func(title, message string, icon any) error = beeep.Notify

// Notify sends a desktop notification with the given title and message if
// notifications are enabled and the environment supports them.
//
// It is unconditionally best-effort: errors are always swallowed. The caller
// must not rely on a notification having been delivered.
func Notify(cfg Config, title, message string) {
	if !cfg.Enabled {
		return
	}

	// On Linux, skip if neither X11 nor Wayland display is available (headless CI,
	// SSH sessions without a forwarded display, etc.). beeep calls dbus-send or
	// notify-send; those commands hang or error in headless environments.
	if runtime.GOOS == "linux" {
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return
		}
	}

	_ = notifyFunc(title, message, "")
}
