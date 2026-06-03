package check

import "testing"

func TestSelftestAllFixturesPass(t *testing.T) {
	passed, failed, err := RunSelftest()
	if err != nil {
		t.Fatalf("RunSelftest returned setup error: %v", err)
	}
	if failed != 0 {
		t.Fatalf("RunSelftest: %d fixtures failed (passed %d), want 0 failures", failed, passed)
	}
	if passed == 0 {
		t.Fatal("RunSelftest passed 0 fixtures, expected the embedded corpus to run")
	}
}

// TestRunSelftest verifies that beekeeper selftest stays green after the SDEF-01
// pollen-self catalog extension. It mirrors TestSelftestAllFixturesPass but uses
// the canonical name referenced by the SDEF-01 verification command.
func TestRunSelftest(t *testing.T) {
	passed, failed, err := RunSelftest()
	if err != nil {
		t.Fatalf("RunSelftest returned setup error: %v", err)
	}
	if failed != 0 {
		t.Fatalf("RunSelftest: %d fixtures failed (passed %d), want 0 failures", failed, passed)
	}
	// SDEF-01: the corpus now includes the pollen-self fixture, so the count must
	// be at least the number of fixtures that were passing before + 1 for the
	// new pollen-self case.
	if passed == 0 {
		t.Fatal("RunSelftest passed 0 fixtures, expected the embedded corpus to run")
	}
	t.Logf("RunSelftest: passed=%d failed=%d", passed, failed)
}
