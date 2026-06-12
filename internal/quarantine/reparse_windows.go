//go:build windows

package quarantine

import (
	"os"
	"syscall"
)

// isReparsePoint reports whether path is a symlink OR a Windows reparse point
// (which includes junctions / mount points created via `mklink /J`). os.Lstat
// does not dereference the final component, so a symlinked or junctioned source
// is detected here rather than silently followed by the subsequent os.Rename.
//
// F-2: the quarantine move/restore source must not be a reparse point — a
// junction can redirect the rename target to an operator-unexpected location.
func isReparsePoint(path string) (bool, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	// Cross-platform symlink bit (Go sets ModeSymlink for name-surrogate reparse
	// points it understands).
	if fi.Mode()&os.ModeSymlink != 0 {
		return true, nil
	}
	// Windows junctions / mount points are reparse points that Go does NOT flag
	// as ModeSymlink. Inspect the raw FILE_ATTRIBUTE_REPARSE_POINT bit.
	if data, ok := fi.Sys().(*syscall.Win32FileAttributeData); ok {
		if data.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return true, nil
		}
	}
	return false, nil
}
