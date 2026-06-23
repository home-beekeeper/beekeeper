package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestRootWelcomeBanner verifies that bare `beekeeper` (no subcommand) prints the
// branded welcome banner followed by the usual command help, and exits cleanly.
// This is the first Beekeeper-authored output after install, so it must not
// regress to a silent or error-exit invocation.
func TestRootWelcomeBanner(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{})

	if err := root.Execute(); err != nil {
		t.Fatalf("bare `beekeeper` returned an error: %v", err)
	}

	out := buf.String()
	// Banner: the brand mark + name and the one-line purpose.
	for _, want := range []string{"BEEKEEPER", "autonomous coding agents"} {
		if !strings.Contains(out, want) {
			t.Errorf("welcome banner missing %q; got:\n%s", want, out)
		}
	}
	// Help still renders below the banner (the command list).
	if !strings.Contains(out, "Available Commands") {
		t.Errorf("help output missing the command list; got:\n%s", out)
	}
}

// TestPrintWelcomeIsAnsiFree guards the banner against ANSI escapes so it stays
// legible in pipes, logs, and captured output (the no-color contract).
func TestPrintWelcomeIsAnsiFree(t *testing.T) {
	var buf bytes.Buffer
	printWelcome(&buf)
	if strings.Contains(buf.String(), "\x1b[") {
		t.Errorf("welcome banner contains an ANSI escape; keep it plain text:\n%q", buf.String())
	}
}
