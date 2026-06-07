package check

import "testing"

// TestExpandShellEnvVars covers the $VAR / ${VAR} canonicalization hardening:
// resolved vars expand, unresolved vars stay literal (fail-closed), and non-var
// '$' usages are preserved.
func TestExpandShellEnvVars(t *testing.T) {
	t.Setenv("MYVAR", "/resolved/path")

	cases := []struct{ in, want string }{
		{"$MYVAR/x", "/resolved/path/x"},
		{"${MYVAR}/x", "/resolved/path/x"},
		{"prefix/$MYVAR", "prefix//resolved/path"},
		{"no vars here", "no vars here"},
		{"$UNSET_VAR_XYZ/x", "$UNSET_VAR_XYZ/x"},        // fail-closed: literal kept
		{"${UNSET_VAR_XYZ}/x", "${UNSET_VAR_XYZ}/x"},    // fail-closed: literal kept
		{"price$5", "price$5"},                          // '$' not a name start
		{"trailing$", "trailing$"},                      // bare trailing '$'
		{"${unclosed/x", "${unclosed/x"},                // no closing brace -> literal
		{"a$MYVAR$MYVAR", "a/resolved/path/resolved/path"},
		{"", ""},
	}
	for _, c := range cases {
		if got := expandShellEnvVars(c.in); got != c.want {
			t.Errorf("expandShellEnvVars(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestExpandShellEnvVarsLowerCaseFallback verifies the upper-case Getenv fallback
// (matching expandWinEnvVars behavior).
func TestExpandShellEnvVarsLowerCaseFallback(t *testing.T) {
	t.Setenv("HOMEDRIVE_TEST", "C:")
	if got := expandShellEnvVars("$homedrive_test/x"); got != "C:/x" {
		t.Errorf("expected upper-case env fallback, got %q", got)
	}
}
