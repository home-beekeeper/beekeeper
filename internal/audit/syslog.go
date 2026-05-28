//go:build linux || darwin

package audit

import (
	"encoding/json"
	"fmt"
	"log/syslog"
	"os"
	"strings"
	"time"
)

// ErrSyslogNotSupported is returned on platforms that do not support syslog.
// It is defined here (linux/darwin build) as a nil sentinel; the windows stub
// defines the real error value. Both files share the same exported symbol name
// so callers can use errors.Is regardless of OS.
//
// Note: the actual non-nil ErrSyslogNotSupported lives in syslog_stub.go
// (windows build). On linux/darwin we do not need it, but callers in sink.go
// reference it unconditionally, so we provide a nil var here for completeness.
// The errors.Is(err, ErrSyslogNotSupported) check in sink.go is safe: when
// NewSyslogSink succeeds on linux/darwin the var is nil and the check is never
// reached on the success path.
var ErrSyslogNotSupported error // nil on linux/darwin

// SyslogSink delivers AuditRecords to a remote or local syslog daemon using
// RFC 5424 framing. The priority is LOG_LOCAL0|LOG_INFO (facility 16, severity 6).
type SyslogSink struct{ writer *syslog.Writer }

// NewSyslogSink dials the syslog daemon at address.
// Address format: "udp:host:port", "tcp:host:port", or "host:port" (UDP default).
func NewSyslogSink(address string) (*SyslogSink, error) {
	network, addr := parseSyslogAddress(address)
	w, err := syslog.Dial(network, addr, syslog.LOG_LOCAL0|syslog.LOG_INFO, "beekeeper")
	if err != nil {
		return nil, fmt.Errorf("syslog dial %q: %w", address, err)
	}
	return &SyslogSink{writer: w}, nil
}

// parseSyslogAddress splits an address of the form "proto:rest" where proto is
// "udp" or "tcp". All other formats are treated as plain addresses using UDP.
func parseSyslogAddress(address string) (string, string) {
	parts := strings.SplitN(address, ":", 2)
	if len(parts) == 2 && (parts[0] == "udp" || parts[0] == "tcp") {
		return parts[0], parts[1]
	}
	return "udp", address
}

// Write sends rec as an RFC 5424 syslog message. The MSG part is the full JSON
// representation of the record so no information is lost.
func (s *SyslogSink) Write(rec AuditRecord) error {
	hostname, _ := os.Hostname()
	ts := rec.Timestamp
	if ts == "" {
		ts = time.Now().UTC().Format(time.RFC3339)
	}
	pid := os.Getpid()

	data, _ := json.Marshal(rec)
	// RFC 5424: <PRI>VERSION TIMESTAMP HOSTNAME APP-NAME PROCID MSGID STRUCTURED-DATA MSG
	// PRI = (16*8)+6 = 134 (LOCAL0, INFO)
	msg := fmt.Sprintf(
		`<134>1 %s %s beekeeper %d policy_decision [beekeeper@0 decision="%s" tool="%s" agent="%s"] %s`,
		ts, hostname, pid, rec.Decision, rec.ToolName, rec.AgentName, string(data),
	)
	_, err := fmt.Fprint(s.writer, msg)
	return err
}

// Close closes the underlying syslog connection.
func (s *SyslogSink) Close() error { return s.writer.Close() }
