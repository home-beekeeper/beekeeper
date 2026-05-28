package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mzansi-agentive/beekeeper/internal/platform"
)

// Rotate rotates the audit log at auditPath when its size exceeds maxBytes.
// Archives are named beekeeper.ndjson.1, beekeeper.ndjson.2, … up to .999.
// Archives older than retentionDays are deleted before the shift.
//
// Rotate is NOT concurrent-safe on its own — the Writer mutex (added in Plan 03)
// serialises it. Rotate itself has no mutex.
func Rotate(auditPath string, maxBytes int64, retentionDays int) error {
	info, err := os.Stat(auditPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat audit log: %w", err)
	}
	if info.Size() <= maxBytes {
		return nil
	}

	// Discover the highest existing archive index (capped at 999).
	highest := 0
	for n := 1; n <= 999; n++ {
		archivePath := fmt.Sprintf("%s.%d", auditPath, n)
		if _, serr := os.Stat(archivePath); os.IsNotExist(serr) {
			break
		}
		highest = n
	}

	// Delete archives older than retentionDays and collect survivors.
	cutoff := time.Duration(retentionDays) * 24 * time.Hour
	survivors := make([]int, 0, highest)
	for n := 1; n <= highest; n++ {
		archivePath := fmt.Sprintf("%s.%d", auditPath, n)
		ainfo, serr := os.Stat(archivePath)
		if os.IsNotExist(serr) {
			continue
		}
		if serr != nil {
			survivors = append(survivors, n)
			continue
		}
		if retentionDays > 0 && time.Since(ainfo.ModTime()) > cutoff {
			_ = os.Remove(archivePath)
		} else {
			survivors = append(survivors, n)
		}
	}

	// Shift surviving archives upward in REVERSE order (highest N first) to
	// avoid clobbering: beekeeper.ndjson.N → beekeeper.ndjson.(N+1).
	for i := len(survivors) - 1; i >= 0; i-- {
		n := survivors[i]
		src := fmt.Sprintf("%s.%d", auditPath, n)
		dst := fmt.Sprintf("%s.%d", auditPath, n+1)
		if rerr := os.Rename(src, dst); rerr != nil {
			return fmt.Errorf("shift archive %s → %s: %w", filepath.Base(src), filepath.Base(dst), rerr)
		}
	}

	// Rename current log → .1
	if rerr := os.Rename(auditPath, auditPath+".1"); rerr != nil {
		return fmt.Errorf("archive current log: %w", rerr)
	}

	// Create a fresh empty log with owner-only permissions.
	f, err := os.OpenFile(auditPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("create new audit log: %w", err)
	}
	if cerr := f.Close(); cerr != nil {
		return fmt.Errorf("close new audit log: %w", cerr)
	}

	if err := platform.SetOwnerOnly(auditPath); err != nil {
		return fmt.Errorf("set owner-only on new audit log: %w", err)
	}

	return nil
}
