//go:build windows

package platform

import "github.com/hectane/go-acl"

// SetOwnerOnly restricts the file at path to owner-only read/write (0600).
// On Windows, os.Chmod only manipulates the read-only attribute and does not
// restrict read access, so we apply a proper DACL via hectane/go-acl, which
// translates the 0600 mode into an owner-only access-control list.
func SetOwnerOnly(path string) error {
	return acl.Chmod(path, 0600)
}
