//go:build !windows

package platform

import "os"

// SetOwnerOnly restricts the file at path to owner-only read/write (0600).
// On Unix this maps directly to os.Chmod with mode 0600.
func SetOwnerOnly(path string) error {
	return os.Chmod(path, 0600)
}
