package tui

import "strings"

// sanitizeForTUI makes an audit-log string safe to render in the terminal.
//
// Audit records carry attacker-influenceable content: a malicious package name,
// a tool command, a crafted file path. The legacy raw-JSON audit view was safe
// only by accident, because json.Marshal escapes control bytes (an ESC becomes
// "", a newline "\n"). A structured view that prints fields directly loses
// that, and an embedded ANSI/OSC escape could spoof the dashboard (paint a BLOCK
// as ALLOWED, move the cursor, clear the screen) or, on some terminals, drive it
// (OSC 52 clipboard writes, title injection). Bidi-override and zero-width runes
// add a Trojan-Source style risk (visually reordering or hiding a package name).
//
// So every record-derived string MUST pass through here before it reaches
// lipgloss: drop the unsafe runes (replacing a run with a single space), collapse
// surrounding whitespace, and truncate to max runes with an ellipsis. This
// reproduces, explicitly, the neutralization json.Marshal used to give for free.
func sanitizeForTUI(s string, max int) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if isUnsafeRune(r) {
			// Collapse any run of unsafe/whitespace runes into one space.
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	out := strings.TrimSpace(b.String())

	if max > 1 {
		runes := []rune(out)
		if len(runes) > max {
			out = string(runes[:max-1]) + "…"
		}
	}
	return out
}

// isUnsafeRune reports whether r must never be rendered verbatim in the TUI:
// C0 controls (incl. TAB/CR/LF and ESC 0x1b), DEL, the C1 control range, the
// Unicode bidi embedding/override/isolate controls and zero-width/BOM runes
// (Trojan-Source class). Ordinary printable Unicode (accents, CJK, emoji) passes.
func isUnsafeRune(r rune) bool {
	switch {
	case r < 0x20: // C0 controls: NUL..US, includes \t \n \r and ESC
		return true
	case r == 0x7f: // DEL
		return true
	case r >= 0x80 && r <= 0x9f: // C1 controls
		return true
	case r >= 0x200b && r <= 0x200f: // zero-width space..joiners, LRM/RLM
		return true
	case r >= 0x202a && r <= 0x202e: // bidi embeddings + overrides (LRE..RLO/PDF)
		return true
	case r >= 0x2066 && r <= 0x2069: // bidi isolates (LRI/RLI/FSI/PDI)
		return true
	case r == 0xfeff: // BOM / zero-width no-break space
		return true
	default:
		return false
	}
}
