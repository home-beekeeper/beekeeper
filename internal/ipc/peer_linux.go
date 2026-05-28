//go:build linux

package ipc

import (
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

// verifyPeerUID uses SO_PEERCRED to confirm that the connecting process is
// running as expectedUID.
func verifyPeerUID(conn net.Conn, expectedUID uint32) error {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return fmt.Errorf("not a Unix connection")
	}

	rawConn, err := unixConn.SyscallConn()
	if err != nil {
		return fmt.Errorf("getting raw conn: %w", err)
	}

	var ucred *unix.Ucred
	var innerErr error
	ctrlErr := rawConn.Control(func(fd uintptr) {
		ucred, innerErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	})
	if ctrlErr != nil {
		return fmt.Errorf("control: %w", ctrlErr)
	}
	if innerErr != nil {
		return fmt.Errorf("getsockopt SO_PEERCRED: %w", innerErr)
	}
	if ucred.Uid != expectedUID {
		return fmt.Errorf("peer UID %d does not match expected %d", ucred.Uid, expectedUID)
	}
	return nil
}
