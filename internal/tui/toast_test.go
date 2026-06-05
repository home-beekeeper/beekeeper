package tui

import (
	"strings"
	"testing"
)

// TestToastShowArmsAndRenders verifies that Show() arms the toast (visible,
// message + kind set, a non-nil auto-clear command returned) and that View()
// renders the message for each alert type.
func TestToastShowArmsAndRenders(t *testing.T) {
	cases := []struct {
		name     string
		kind     toastKind
		msg      string
		wantIcon string
	}{
		{"ok", toastOK, "item sent to quarantine", "✓"},
		{"warn", toastWarn, "policy source degraded", "⚠"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, cmd := ToastModel{}.Show(tc.msg, tc.kind)
			if !m.visible {
				t.Fatalf("Show(%s) did not set visible=true", tc.name)
			}
			if m.kind != tc.kind {
				t.Fatalf("Show(%s) kind = %q, want %q", tc.name, m.kind, tc.kind)
			}
			if cmd == nil {
				t.Fatalf("Show(%s) returned nil auto-clear cmd", tc.name)
			}
			view := m.View(80)
			if !strings.Contains(view, tc.msg) {
				t.Errorf("View(%s) does not contain message %q\ngot: %q", tc.name, tc.msg, view)
			}
			if !strings.Contains(view, tc.wantIcon) {
				t.Errorf("View(%s) does not contain icon %q\ngot: %q", tc.name, tc.wantIcon, view)
			}
		})
	}
}

// TestToastKindsRenderDistinctly is the core "different alert types" assertion:
// the ok and warn toasts must be visually distinguishable by GLYPH (not just
// color, which a non-TTY profile strips). ok uses ✓, warn uses ⚠ — each must
// carry its own glyph and NOT the other's.
func TestToastKindsRenderDistinctly(t *testing.T) {
	ok, _ := ToastModel{}.Show("done", toastOK)
	warn, _ := ToastModel{}.Show("heads up", toastWarn)

	okView := ok.View(80)
	warnView := warn.View(80)

	if strings.Contains(okView, "⚠") {
		t.Errorf("ok toast must not show the warn glyph ⚠\ngot: %q", okView)
	}
	if !strings.Contains(okView, "✓") {
		t.Errorf("ok toast must show ✓\ngot: %q", okView)
	}
	if strings.Contains(warnView, "✓") {
		t.Errorf("warn toast must not show the ok glyph ✓\ngot: %q", warnView)
	}
	if !strings.Contains(warnView, "⚠") {
		t.Errorf("warn toast must show ⚠\ngot: %q", warnView)
	}
	if okView == warnView {
		t.Errorf("ok and warn toasts rendered identically — alert types are not distinct")
	}
}

// TestToastHideClearsView verifies the lifecycle: an armed toast renders, the
// auto-clear toastHideMsg makes it invisible, and an invisible toast renders "".
func TestToastHideClearsView(t *testing.T) {
	m, _ := ToastModel{}.Show("ephemeral", toastOK)
	if m.View(80) == "" {
		t.Fatal("armed toast rendered empty")
	}

	m, _ = m.Update(toastHideMsg{})
	if m.visible {
		t.Fatal("toastHideMsg did not clear visible")
	}
	if got := m.View(80); got != "" {
		t.Errorf("hidden toast View = %q, want empty", got)
	}
}

// TestToastZeroValueInvisible verifies a never-shown toast renders nothing.
func TestToastZeroValueInvisible(t *testing.T) {
	var m ToastModel
	if got := m.View(80); got != "" {
		t.Errorf("zero-value ToastModel View = %q, want empty", got)
	}
}
