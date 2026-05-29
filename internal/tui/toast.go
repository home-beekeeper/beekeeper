package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

type toastKind string

const (
	toastOK   toastKind = "ok"
	toastWarn toastKind = "warn"
)

type toastHideMsg struct{}

// ToastModel is an ephemeral bottom-center notification.
type ToastModel struct {
	msg     string
	kind    toastKind
	visible bool
}

// Show arms the toast with a 2200ms auto-clear timer.
func (t ToastModel) Show(msg string, kind toastKind) (ToastModel, tea.Cmd) {
	t.msg = msg
	t.kind = kind
	t.visible = true
	cmd := tea.Tick(2200*time.Millisecond, func(_ time.Time) tea.Msg {
		return toastHideMsg{}
	})
	return t, cmd
}

// Update handles the hide message.
func (t ToastModel) Update(msg tea.Msg) (ToastModel, tea.Cmd) {
	if _, ok := msg.(toastHideMsg); ok {
		t.visible = false
	}
	return t, nil
}

// View renders the toast at a given terminal width.
func (t ToastModel) View(width int) string {
	if !t.visible {
		return ""
	}
	var icon string
	if t.kind == toastWarn {
		icon = styleAmber.Render("✓")
	} else {
		icon = styleGreen.Render("✓")
	}
	content := icon + " " + styleDim.Render(t.msg)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(0, 1).
		Render(content)
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, box)
}
