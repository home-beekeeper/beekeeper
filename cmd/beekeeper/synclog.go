package main

import (
	"io"
	"os"
	"path/filepath"

	"github.com/home-beekeeper/beekeeper/internal/platform"
)

// syncLogMaxBytes is the size at which sync.log is rotated to sync.log.1 (a
// single backup) before a fresh file is opened. Keeps the background sync log
// bounded without a full rotation framework.
const syncLogMaxBytes = 1 << 20 // 1 MiB

// syncLogPath returns the background-sync log path under the platform log dir.
func syncLogPath() (string, error) {
	dir, err := platform.LogDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sync.log"), nil
}

// openSyncLog opens (creating + rotating as needed) the background sync log for
// append. If the existing file is at/over syncLogMaxBytes it is rotated to
// sync.log.1 (single backup) before a fresh file is opened. The file is
// owner-only (0o600) under an owner-only (0o700) directory.
//
// Failures are returned to the caller, which treats them as NON-FATAL: a
// background sync that cannot open its log still runs, writing only to its
// original Out/Err. A rotation rename failure is swallowed (we fall through and
// append to the existing file) so logging is never blocked by a stuck backup.
func openSyncLog() (*os.File, error) {
	path, err := syncLogPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if info, statErr := os.Stat(path); statErr == nil && info.Size() >= syncLogMaxBytes {
		_ = os.Rename(path, path+".1") // best-effort; never blocks logging
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
}

// teeWriter returns a writer that fans out to every non-nil sink. With one sink
// it returns that sink unchanged; with none it returns io.Discard. Used to send
// a `catalogs sync --background` run's output to BOTH the original cobra Out/Err
// and the persistent sync.log.
func teeWriter(sinks ...io.Writer) io.Writer {
	nonNil := make([]io.Writer, 0, len(sinks))
	for _, w := range sinks {
		if w != nil {
			nonNil = append(nonNil, w)
		}
	}
	switch len(nonNil) {
	case 0:
		return io.Discard
	case 1:
		return nonNil[0]
	default:
		return io.MultiWriter(nonNil...)
	}
}
