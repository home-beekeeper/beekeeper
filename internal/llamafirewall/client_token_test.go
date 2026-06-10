// Cross-platform (no build tag) tests for the loopback-TCP client transport and
// its per-launch bearer token (Phase 20, LLMF). These run on Windows — the
// primary dev platform — where the old unix-socket supervisor_test.go never did,
// proving the single TCP transport works on every OS and that the token restores
// the access control the old 0600 unix socket gave.
package llamafirewall

import (
	"net"
	"testing"
	"time"
)

// startTokenCheckingSidecar mimics the Python sidecar's bearer-token check: for
// each request it returns ResultClean iff req.Token equals want, otherwise a
// fail-closed ResultError ("unauthorized"). It proves the Go client stamps the
// per-launch token onto every request and that a wrong/absent token is rejected.
func startTokenCheckingSidecar(t *testing.T, want string) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return // listener closed
			}
			go func(c net.Conn) {
				defer c.Close()
				var req ScanRequest
				if err := Decode(c, &req); err != nil {
					return
				}
				resp := ScanResponse{RequestID: req.RequestID, Result: ResultClean}
				if req.Token != want {
					resp = ScanResponse{
						RequestID: req.RequestID,
						Result:    ResultError,
						Error:     "unauthorized: token mismatch",
					}
				}
				_ = Encode(c, resp)
			}(conn)
		}
	}()
	return ln.Addr().String(), func() {
		ln.Close()
		<-done
	}
}

// TestClientSendsTokenAccepted verifies a request carrying the matching token is
// accepted (the client stamps c.token onto every ScanRequest).
func TestClientSendsTokenAccepted(t *testing.T) {
	addr, stop := startTokenCheckingSidecar(t, "secret-token")
	defer stop()

	c, err := Dial(addr, "secret-token", time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	resp, err := c.Scan(ScanRequest{Kind: ScanPrompt, Content: "hi", RequestID: "r1"})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if resp.Result != ResultClean {
		t.Fatalf("correct token: result = %q, want clean", resp.Result)
	}
}

// TestClientWrongTokenRejected verifies a request with the wrong token is
// rejected with a fail-closed error result — restoring the access control the
// old 0600 unix socket provided now that the transport is loopback TCP.
func TestClientWrongTokenRejected(t *testing.T) {
	addr, stop := startTokenCheckingSidecar(t, "secret-token")
	defer stop()

	c, err := Dial(addr, "WRONG", time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	resp, err := c.Scan(ScanRequest{Kind: ScanPrompt, Content: "hi", RequestID: "r2"})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if resp.Result != ResultError {
		t.Fatalf("wrong token: result = %q, want error (rejected)", resp.Result)
	}
	if resp.Error == "" {
		t.Fatal("wrong token: expected a non-empty error field")
	}
}
