//go:build !windows

package quarantine

import "os"

// isReparsePoint reports whether path is a symlink. On POSIX there is no
// junction/reparse-point concept, so the ModeSymlink check via os.Lstat is the
// complete test. os.Lstat does not dereference the final component.
//
// F-2: the quarantine move/restore source must not be a symlink — a symlinked
// source would redirect the rename to an operator-unexpected location.
func isReparsePoint(path string) (bool, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	return fi.Mode()&os.ModeSymlink != 0, nil
}
