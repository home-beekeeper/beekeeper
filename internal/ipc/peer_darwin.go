//go:build darwin

package ipc

import (
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

// verifyPeerUID uses GetsockoptXucred(SOL_LOCAL, LOCAL_PEERCRED) to confirm
// that the connecting process is running as expectedUID. On macOS,
// SO_PEERCRED is not available; LOCAL_PEERCRED provides equivalent UID
// authentication for Unix domain sockets via the Xucred structure.
func verifyPeerUID(conn net.Conn, expectedUID uint32) error {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return fmt.Errorf("not a Unix connection")
	}

	rawConn, err := unixConn.SyscallConn()
	if err != nil {
		return fmt.Errorf("getting raw conn: %w", err)
	}

	var xucred *unix.Xucred
	var innerErr error
	ctrlErr := rawConn.Control(func(fd uintptr) {
		xucred, innerErr = unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
	})
	if ctrlErr != nil {
		return fmt.Errorf("control: %w", ctrlErr)
	}
	if innerErr != nil {
		return fmt.Errorf("getsockopt LOCAL_PEERCRED: %w", innerErr)
	}
	if xucred.Uid != expectedUID {
		return fmt.Errorf("peer UID %d does not match expected %d", xucred.Uid, expectedUID)
	}
	return nil
}
