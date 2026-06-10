package sentry

import (
	"go/parser"
	"go/token"
	"os"
	"testing"
)

// TestRulesImportsArePure locks the Sentry correlation engine as a pure,
// I/O-free library (Phase 20, SENT-04). The rules layer MAY import net and time
// (it correlates IP destinations and time windows) but MUST NOT import os,
// net/http, io, sync, or context — any of those would let a future executor
// sneak side effects, blocking I/O, or shared mutable state into the engine the
// hook handler, gateway, and daemon all call synchronously.
//
// It scans both rules.go (the rule bodies) and types.go (the shared types).
func TestRulesImportsArePure(t *testing.T) {
	forbidden := map[string]bool{
		"os":       true,
		"net/http": true,
		"io":       true,
		"sync":     true,
		"context":  true,
	}

	for _, path := range []string{"rules.go", "types.go"} {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, src, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}
		for _, imp := range f.Imports {
			p := imp.Path.Value
			if len(p) >= 2 {
				p = p[1 : len(p)-1] // strip surrounding quotes
			}
			if forbidden[p] {
				t.Errorf("%s imports forbidden package %q — violates the pure-engine contract (allowed: net, time, stdlib helpers)", path, p)
			}
		}
	}
}
