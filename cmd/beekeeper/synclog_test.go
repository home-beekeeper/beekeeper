package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// TestOpenSyncLogCreatesAndAppends verifies a fresh sync.log is created and that
// a second open appends rather than truncates.
func TestOpenSyncLogCreatesAndAppends(t *testing.T) {
	t.Setenv("BEEKEEPER_HOME", t.TempDir())

	f, err := openSyncLog()
	if err != nil {
		t.Fatalf("openSyncLog() #1 error: %v", err)
	}
	if _, err := f.WriteString("first\n"); err != nil {
		t.Fatalf("write #1: %v", err)
	}
	f.Close()

	f2, err := openSyncLog()
	if err != nil {
		t.Fatalf("openSyncLog() #2 error: %v", err)
	}
	if _, err := f2.WriteString("second\n"); err != nil {
		t.Fatalf("write #2: %v", err)
	}
	path := f2.Name()
	f2.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sync.log: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "first") || !strings.Contains(got, "second") {
		t.Fatalf("append failed; sync.log = %q", got)
	}
}

// TestOpenSyncLogRotatesAtCap verifies an oversized sync.log is rotated to
// sync.log.1 and the new file starts fresh.
func TestOpenSyncLogRotatesAtCap(t *testing.T) {
	t.Setenv("BEEKEEPER_HOME", t.TempDir())

	path, err := syncLogPath()
	if err != nil {
		t.Fatalf("syncLogPath: %v", err)
	}
	if err := os.MkdirAll(strings.TrimSuffix(path, "sync.log"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Write a file at/over the cap so the next open rotates it.
	big := bytes.Repeat([]byte("x"), syncLogMaxBytes+16)
	if err := os.WriteFile(path, big, 0o600); err != nil {
		t.Fatalf("seed oversized log: %v", err)
	}

	f, err := openSyncLog()
	if err != nil {
		t.Fatalf("openSyncLog after oversize: %v", err)
	}
	f.WriteString("fresh\n")
	f.Close()

	if _, err := os.Stat(path + ".1"); err != nil {
		t.Errorf("expected rotated backup sync.log.1, stat err: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rotated sync.log: %v", err)
	}
	if len(data) >= syncLogMaxBytes {
		t.Errorf("sync.log was not rotated fresh; size=%d", len(data))
	}
	if !strings.Contains(string(data), "fresh") {
		t.Errorf("post-rotation sync.log missing new content: %q", string(data))
	}
}

// TestTeeWriterFansOut verifies teeWriter writes to all sinks, passes a single
// sink through unchanged, and discards with none.
func TestTeeWriterFansOut(t *testing.T) {
	var a, b bytes.Buffer
	w := teeWriter(&a, &b)
	if _, err := w.Write([]byte("hi")); err != nil {
		t.Fatalf("tee write: %v", err)
	}
	if a.String() != "hi" || b.String() != "hi" {
		t.Fatalf("tee fan-out failed: a=%q b=%q", a.String(), b.String())
	}

	// A single non-nil sink (the rest nil) is passed through unchanged.
	var c bytes.Buffer
	if got := teeWriter(&c, nil); got != &c {
		t.Fatal("single-sink tee should pass the sink through unchanged")
	}

	// Zero sinks degrade to io.Discard (never nil).
	if teeWriter() != io.Discard {
		t.Fatal("zero-sink tee should be io.Discard")
	}
}
