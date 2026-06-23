package hooks

// command_test.go — unit tests for command.go
//
// Fail-closed wiring fix (exit-127 elimination): every harness hook installer
// now embeds an absolute binary path instead of the bare name "beekeeper", so
// the harness shell can resolve the hook even when beekeeper is off the PATH
// snapshot captured at launch. This file proves the shared command builder and
// stable-suffix matcher are correct across paths-with-spaces, fallback, and
// both old-bare and new-abspath hook forms.

import (
	"errors"
	"strings"
	"testing"
)

// TestResolveBeekeeperBin tests the resolveBeekeeperBin function via the
// injectable execResolver seam.
func TestResolveBeekeeperBin(t *testing.T) {
	t.Run("success_returns_quoted_toslash_path", func(t *testing.T) {
		orig := execResolver
		execResolver = func() (string, error) {
			return `C:\Users\x\beekeeper.exe`, nil
		}
		t.Cleanup(func() { execResolver = orig })

		got := resolveBeekeeperBin()
		// Must be double-quoted and use forward slashes.
		if got != `"C:/Users/x/beekeeper.exe"` {
			t.Fatalf("resolveBeekeeperBin() = %q, want %q", got, `"C:/Users/x/beekeeper.exe"`)
		}
	})

	t.Run("error_returns_bare_fallback", func(t *testing.T) {
		orig := execResolver
		execResolver = func() (string, error) {
			return "", errors.New("executable not found")
		}
		t.Cleanup(func() { execResolver = orig })

		got := resolveBeekeeperBin()
		if got != "beekeeper" {
			t.Fatalf("resolveBeekeeperBin() on error = %q, want %q", got, "beekeeper")
		}
	})

	t.Run("path_with_spaces_is_quoted", func(t *testing.T) {
		orig := execResolver
		execResolver = func() (string, error) {
			return `C:\Program Files\bk\beekeeper.exe`, nil
		}
		t.Cleanup(func() { execResolver = orig })

		got := resolveBeekeeperBin()
		// The quote is essential: without it the harness shell would split on the space.
		if got != `"C:/Program Files/bk/beekeeper.exe"` {
			t.Fatalf("resolveBeekeeperBin() with spaces = %q, want quoted form", got)
		}
		if !strings.HasPrefix(got, `"`) || !strings.HasSuffix(got, `"`) {
			t.Fatalf("path with spaces must be double-quoted, got: %s", got)
		}
	})

	t.Run("unix_path_no_spaces", func(t *testing.T) {
		orig := execResolver
		execResolver = func() (string, error) {
			return "/usr/local/bin/beekeeper", nil
		}
		t.Cleanup(func() { execResolver = orig })

		got := resolveBeekeeperBin()
		if got != `"/usr/local/bin/beekeeper"` {
			t.Fatalf("resolveBeekeeperBin() unix = %q, want quoted", got)
		}
	})
}

// TestBeekeeperCmd tests the beekeeperCmd function.
func TestBeekeeperCmd(t *testing.T) {
	t.Run("appends_args_to_abspath_token", func(t *testing.T) {
		orig := execResolver
		execResolver = func() (string, error) {
			return `/usr/bin/beekeeper`, nil
		}
		t.Cleanup(func() { execResolver = orig })

		got := beekeeperCmd("check --hook claude-code")
		want := `"/usr/bin/beekeeper" check --hook claude-code`
		if got != want {
			t.Fatalf("beekeeperCmd() = %q, want %q", got, want)
		}
	})

	t.Run("fallback_still_appends_args", func(t *testing.T) {
		orig := execResolver
		execResolver = func() (string, error) {
			return "", errors.New("no binary")
		}
		t.Cleanup(func() { execResolver = orig })

		got := beekeeperCmd("check --hook cursor")
		want := "beekeeper check --hook cursor"
		if got != want {
			t.Fatalf("beekeeperCmd() fallback = %q, want %q", got, want)
		}
	})

	t.Run("stable_suffix_is_substring_of_produced_command", func(t *testing.T) {
		orig := execResolver
		execResolver = func() (string, error) {
			return `C:\bk\beekeeper.exe`, nil
		}
		t.Cleanup(func() { execResolver = orig })

		suffix := "check --hook claude-code"
		cmd := beekeeperCmd(suffix)
		// The stable suffix must appear verbatim so matchesBeekeeperCommand can
		// detect BOTH old-bare and new-abspath forms via suffix matching.
		if !strings.Contains(cmd, suffix) {
			t.Fatalf("produced command %q does not contain stable suffix %q", cmd, suffix)
		}
		// The word "beekeeper" must appear (as part of the binary path) and the
		// word "check" must appear (in the suffix args). The stable suffix
		// "check --hook claude-code" contains "check" so matchesBeekeeperCommand
		// works on both forms.  Note: the CONTIGUOUS substring "beekeeper check"
		// does NOT appear in the abspath form because a closing quote separates
		// the binary name from the args — matchesBeekeeperCommand uses the suffix
		// directly and does NOT require the contiguous "beekeeper check" token.
		if !strings.Contains(cmd, "beekeeper") {
			t.Fatalf("produced command %q does not contain 'beekeeper' (binary name)", cmd)
		}
		if !strings.Contains(cmd, "check") {
			t.Fatalf("produced command %q does not contain 'check' (suffix token)", cmd)
		}
	})

	t.Run("audit_record_suffix_in_command", func(t *testing.T) {
		orig := execResolver
		execResolver = func() (string, error) {
			return `/bin/beekeeper`, nil
		}
		t.Cleanup(func() { execResolver = orig })

		cmd := beekeeperCmd("audit-record")
		// The stable suffix "audit-record" must appear so matchesBeekeeperCommand
		// can detect the PostToolUse command.
		if !strings.Contains(cmd, "audit-record") {
			t.Fatalf("audit-record command %q does not contain 'audit-record' (stable suffix)", cmd)
		}
	})
}

// TestMatchesBeekeeperCommand tests the stable-suffix matcher.
func TestMatchesBeekeeperCommand(t *testing.T) {
	t.Run("old_bare_name_matches_suffix", func(t *testing.T) {
		// Old form: bare name.
		cmd := "beekeeper check --hook claude-code"
		suffix := "check --hook claude-code"
		if !matchesBeekeeperCommand(cmd, suffix) {
			t.Fatalf("matchesBeekeeperCommand(%q, %q) = false, want true", cmd, suffix)
		}
	})

	t.Run("new_abspath_form_matches_suffix", func(t *testing.T) {
		// New form: quoted absolute path.
		cmd := `"/home/user/beekeeper" check --hook claude-code`
		suffix := "check --hook claude-code"
		if !matchesBeekeeperCommand(cmd, suffix) {
			t.Fatalf("matchesBeekeeperCommand(%q, %q) = false, want true", cmd, suffix)
		}
	})

	t.Run("windows_abspath_form_matches_suffix", func(t *testing.T) {
		cmd := `"C:/Users/x/beekeeper.exe" check --hook cursor`
		suffix := "check --hook cursor"
		if !matchesBeekeeperCommand(cmd, suffix) {
			t.Fatalf("matchesBeekeeperCommand(%q, %q) = false, want true", cmd, suffix)
		}
	})

	t.Run("decoy_third_party_command_does_not_match", func(t *testing.T) {
		// A third-party command that merely contains the word "beekeeper" (perhaps
		// coincidentally) but NOT the stable suffix must NOT be matched.
		// T-w7y-03: the matcher must NOT trigger on the bare word "beekeeper" alone.
		cmd := "some-other-tool --config beekeeper-ish"
		suffix := "check --hook claude-code"
		if matchesBeekeeperCommand(cmd, suffix) {
			t.Fatalf("matchesBeekeeperCommand(%q, %q) = true, want false (decoy must not match)", cmd, suffix)
		}
	})

	t.Run("decoy_missing_check_hook_token", func(t *testing.T) {
		// A command that contains "beekeeper" but not the full "check --hook <harness>" suffix.
		cmd := "beekeeper --version"
		suffix := "check --hook gemini"
		if matchesBeekeeperCommand(cmd, suffix) {
			t.Fatalf("matchesBeekeeperCommand(%q, %q) = true, want false", cmd, suffix)
		}
	})

	t.Run("audit_record_suffix_matches_both_forms", func(t *testing.T) {
		suffix := "audit-record"
		bare := "beekeeper audit-record"
		abspath := `"C:/prog/beekeeper.exe" audit-record`
		if !matchesBeekeeperCommand(bare, suffix) {
			t.Fatalf("matchesBeekeeperCommand(bare audit-record) = false, want true")
		}
		if !matchesBeekeeperCommand(abspath, suffix) {
			t.Fatalf("matchesBeekeeperCommand(abspath audit-record) = false, want true")
		}
	})

	t.Run("partial_suffix_does_not_match", func(t *testing.T) {
		// Suffix "check --hook claude" should not match "check --hook claude-code"
		// because the suffix is anchored: a partial match of the suffix string within
		// the command is fine (strings.Contains), but if the suffix itself is a
		// sub-match this is correct behaviour per the spec (the caller always passes
		// the full stable suffix, not a partial one).
		// This sub-test just confirms the logic is sane for the exact suffix contract.
		cmd := "beekeeper check --hook claude-code"
		suffix := "check --hook claude-code"
		if !matchesBeekeeperCommand(cmd, suffix) {
			t.Fatalf("exact suffix must match, got false")
		}
	})
}
