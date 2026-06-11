//go:build darwin

package ipc

import (
	"net"
	"os"
	"testing"
)

// TestVerifyPeerUIDDarwin asserts LOCAL_PEERCRED-based peer-UID auth: a
// same-process (same-UID) peer passes, and a deliberately wrong expected UID is
// rejected.
func TestVerifyPeerUIDDarwin(t *testing.T) {
	sock := tempSock(t)
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		c, aerr := l.Accept()
		if aerr != nil {
			accepted <- nil
			return
		}
		accepted <- c
	}()

	client, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	serverConn := <-accepted
	if serverConn == nil {
		t.Fatal("accept failed")
	}
	defer serverConn.Close()

	uid := uint32(os.Getuid())
	if err := verifyPeerUID(serverConn, uid); err != nil {
		t.Errorf("verifyPeerUID(currentUID) = %v, want nil (same-process peer)", err)
	}
	if err := verifyPeerUID(serverConn, uid+1); err == nil {
		t.Error("verifyPeerUID(wrongUID) = nil, want rejection of a mismatched peer UID")
	}
}
