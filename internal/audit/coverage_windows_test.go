//go:build windows

package audit

import (
	"errors"
	"testing"
)

// TestSyslogStubMethods constructs the Windows syslog stub directly and calls
// its Write/Close so the stub method bodies are covered on Windows hosts.
//
// This test lives in a //go:build windows file because syslogStub and
// ErrSyslogNotSupported are defined only in the Windows build of the package
// (syslog_stub.go); referencing them from a cross-platform _test.go fails to
// compile on Linux/macOS.
func TestSyslogStubMethods(t *testing.T) {
	s := &syslogStub{}
	if err := s.Write(AuditRecord{RecordID: "s"}); !errors.Is(err, ErrSyslogNotSupported) {
		t.Errorf("syslogStub.Write = %v, want ErrSyslogNotSupported", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("syslogStub.Close = %v, want nil", err)
	}
}
