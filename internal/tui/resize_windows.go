//go:build windows

package tui

import (
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/term"
)

// StartResizePoller polls the terminal size every 500ms on Windows,
// sending WindowSizeMsg to p on each tick. This works around the lack
// of SIGWINCH on Windows (TUI-10).
func StartResizePoller(p *tea.Program) {
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			w, h, err := term.GetSize(int(os.Stdout.Fd()))
			if err != nil {
				continue
			}
			p.Send(tea.WindowSizeMsg{Width: w, Height: h})
		}
	}()
}
