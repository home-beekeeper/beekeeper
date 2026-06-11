package coveragegate

import (
	"strings"
	"testing"
)

// TestAllowlistFailsClosed is the VAL-08 self-defense meta-test: the allowlist
// parser must reject a bare path (no reason code) and a path carrying a reason
// code outside the closed taxonomy. Both attempts to silently weaken the gate
// are rejected — the parser fails closed.
func TestAllowlistFailsClosed(t *testing.T) {
	t.Run("bare path is invalid", func(t *testing.T) {
		if kind, _, _ := classifyLine("internal/foo/foo.go"); kind != lineInvalid {
			t.Errorf("bare path should be lineInvalid, got %v", kind)
		}
	})
	t.Run("unknown reason code is invalid", func(t *testing.T) {
		if kind, _, _ := classifyLine("internal/foo/foo.go\t# reason: just-because"); kind != lineInvalid {
			t.Errorf("out-of-taxonomy reason should be lineInvalid, got %v", kind)
		}
	})
	t.Run("ParseAllowlist rejects a bare path", func(t *testing.T) {
		if _, err := ParseAllowlist(strings.NewReader("internal/foo/foo.go\n")); err == nil {
			t.Error("ParseAllowlist must fail closed on a bare path, got nil error")
		}
	})
	t.Run("ParseAllowlist rejects an unknown reason code", func(t *testing.T) {
		if _, err := ParseAllowlist(strings.NewReader("internal/foo/foo.go\t# reason: bogus\n")); err == nil {
			t.Error("ParseAllowlist must fail closed on an unknown reason code, got nil error")
		}
	})
	t.Run("a valid entry parses", func(t *testing.T) {
		al, err := ParseAllowlist(strings.NewReader("internal/version/version.go\t# reason: type-only\n"))
		if err != nil {
			t.Fatalf("valid entry should parse, got %v", err)
		}
		if !al.Has("internal/version/version.go") || al.Reason("internal/version/version.go") != "type-only" {
			t.Errorf("valid entry not recorded: has=%v reason=%q", al.Has("internal/version/version.go"), al.Reason("internal/version/version.go"))
		}
	})
	t.Run("comments and blanks are skipped", func(t *testing.T) {
		al, err := ParseAllowlist(strings.NewReader("# header comment\n\n   # indented comment\ninternal/version/version.go\t# reason: type-only\n"))
		if err != nil {
			t.Fatalf("comments/blanks should be skipped, got %v", err)
		}
		if al.Len() != 1 {
			t.Errorf("expected 1 entry, got %d", al.Len())
		}
	})
	t.Run("duplicate entries fail closed", func(t *testing.T) {
		in := "internal/version/version.go\t# reason: type-only\ninternal/version/version.go\t# reason: type-only\n"
		if _, err := ParseAllowlist(strings.NewReader(in)); err == nil {
			t.Error("duplicate entry must fail closed, got nil error")
		}
	})
}

// TestAllowlistReasonTaxonomy proves each of the six closed reason codes is
// accepted and that an empty reason is rejected.
func TestAllowlistReasonTaxonomy(t *testing.T) {
	for _, code := range ValidReasonCodes() {
		line := "internal/pkg/file.go\t# reason: " + code
		if kind, _, reason := classifyLine(line); kind != lineEntry || reason != code {
			t.Errorf("reason code %q should be a valid entry, got kind=%v reason=%q", code, kind, reason)
		}
	}
	if len(ValidReasonCodes()) != 6 {
		t.Errorf("expected exactly 6 closed reason codes, got %d: %v", len(ValidReasonCodes()), ValidReasonCodes())
	}
	t.Run("empty reason is invalid", func(t *testing.T) {
		if kind, _, _ := classifyLine("internal/pkg/file.go\t# reason: "); kind != lineInvalid {
			t.Error("empty reason should be lineInvalid")
		}
		if kind, _, _ := classifyLine("internal/pkg/file.go\t# reason:"); kind != lineInvalid {
			t.Error("missing reason value should be lineInvalid")
		}
	})
}
