//go:build fuzz

package sentry

import (
	"testing"
	"time"
)

// FuzzEvaluateEvent fuzzes the pure Sentry rule evaluator (VAL-04). The
// evaluator is locked pure/I/O-free by TestRulesImportsArePure, which makes it
// an ideal fuzz target. Contract:
//
//   - it must NEVER panic — a panic in the correlation engine is a fail-open DoS
//     for the runtime monitor (every hook/gateway/daemon path calls it
//     synchronously);
//   - every returned alert's Severity must be "critical" or "high" — the two
//     documented values; anything else breaks the audit/SIEM contract.
//
// Modeled on internal/policy/fuzz_test.go::FuzzEvaluate. Seeded with one event
// of each kind plus an out-of-range kind. BaselineState{} zero-value is safe to
// pass (DurationDays == 0 -> baseline inactive).
func FuzzEvaluateEvent(f *testing.F) {
	// kind, pid, ppid, exe, path
	f.Add(uint8(1), uint32(1000), uint32(1), "code", "/home/u/.aws/credentials")                   // FileAccess cred read
	f.Add(uint8(0), uint32(1001), uint32(1), "gh", "")                                              // ProcessCreate cred CLI
	f.Add(uint8(2), uint32(1002), uint32(1), "claude", "")                                          // NetworkConnect
	f.Add(uint8(3), uint32(1003), uint32(1), "node", "/home/u/.config/systemd/user/x.service")      // FileWrite persistence
	f.Add(uint8(4), uint32(1004), uint32(1), "curl", "exfil.example.com")                           // DNSQuery (no-op rule)
	f.Add(uint8(255), uint32(0), uint32(0), "", "")                                                 // out-of-range kind / zero

	f.Fuzz(func(t *testing.T, kind uint8, pid, ppid uint32, exe, path string) {
		ev := SentryEvent{
			Kind:     EventKind(kind),
			PID:      pid,
			PPID:     ppid,
			Exe:      exe,
			FilePath: path,
		}
		tree := map[uint32]ProcessNode{pid: {PID: pid, PPID: ppid, Exe: exe}}

		alerts := EvaluateEvent(ev, NewRuleState(), tree, InventorySnapshot{}, RuleConfig{}, BaselineState{}, time.Now())

		for _, a := range alerts {
			switch a.Severity {
			case "critical", "high":
				// valid
			default:
				t.Errorf("invalid alert severity %q from kind=%d exe=%q path=%q (rule %s)", a.Severity, kind, exe, path, a.RuleID)
			}
		}
	})
}
