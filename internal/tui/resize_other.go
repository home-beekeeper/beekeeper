//go:build !windows

package tui

import tea "charm.land/bubbletea/v2"

// StartResizePoller is a no-op on platforms with native SIGWINCH support.
func StartResizePoller(p *tea.Program) {}
