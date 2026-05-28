//go:build windows

package audit

import "errors"

// ErrSyslogNotSupported is returned by NewSyslogSink on Windows, where the
// standard library's log/syslog package is unavailable. Callers should treat
// this as a platform limitation, not a configuration error, and skip the syslog
// sink gracefully (see NewMultiSink).
var ErrSyslogNotSupported = errors.New("syslog: not supported on Windows")

type syslogStub struct{}

func (s *syslogStub) Write(_ AuditRecord) error { return ErrSyslogNotSupported }
func (s *syslogStub) Close() error              { return nil }

// NewSyslogSink always returns ErrSyslogNotSupported on Windows.
func NewSyslogSink(_ string) (*syslogStub, error) {
	return nil, ErrSyslogNotSupported
}
