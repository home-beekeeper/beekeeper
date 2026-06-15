package audit

import (
	"errors"
	"fmt"
	"os"

	"github.com/bantuson/beekeeper/internal/config"
)

// Sink is the common interface for audit output targets. Every implementation
// must be safe for concurrent use (the Writer mutex guards calls from the file
// sink; remote sinks must manage their own concurrency).
type Sink interface {
	Write(rec AuditRecord) error
	Close() error
}

// WriterSink wraps an existing *Writer so it satisfies the Sink interface.
type WriterSink struct{ w *Writer }

// NewWriterSink returns a Sink that delegates to w.
func NewWriterSink(w *Writer) *WriterSink { return &WriterSink{w: w} }

// Write delegates to the underlying Writer.
func (s *WriterSink) Write(rec AuditRecord) error { return s.w.Write(rec) }

// Close closes the underlying Writer.
func (s *WriterSink) Close() error { return s.w.Close() }

// MultiSink fans out every Write and Close call to all registered sinks.
// Errors from individual sinks are not short-circuited: every sink always
// receives the call. The last non-nil error is returned.
type MultiSink struct{ sinks []Sink }

// NewMultiSinkFromSinks returns a MultiSink that owns sinks.
func NewMultiSinkFromSinks(sinks []Sink) *MultiSink { return &MultiSink{sinks: sinks} }

// Write delivers rec to every sink; returns the last non-nil error.
func (m *MultiSink) Write(rec AuditRecord) error {
	var lastErr error
	for _, s := range m.sinks {
		if err := s.Write(rec); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// Close closes every sink; returns the last non-nil error.
func (m *MultiSink) Close() error {
	var lastErr error
	for _, s := range m.sinks {
		if err := s.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// NewMultiSink constructs the full sink graph for auditPath and the provided
// config.AuditConfig. A file-backed WriterSink is always created as the first
// sink. Additional sinks (syslog, otlp, https) are appended based on cfg.Sinks.
//
// If NewSyslogSink returns ErrSyslogNotSupported the syslog sink is skipped
// with a warning printed to stderr; any other syslog error is returned to the
// caller. Remote sinks (OTLP, HTTPS) print a machine-readable warning so
// operators know audit data is leaving the local host.
func NewMultiSink(auditPath string, cfg config.AuditConfig) (Sink, error) {
	w, err := NewWriter(auditPath)
	if err != nil {
		return nil, fmt.Errorf("audit file sink: %w", err)
	}

	sinks := []Sink{NewWriterSink(w)}
	var remoteNames []string

	if containsString(cfg.Sinks, "syslog") && cfg.SyslogAddress != "" {
		ss, serr := NewSyslogSink(cfg.SyslogAddress)
		if serr != nil {
			if errors.Is(serr, ErrSyslogNotSupported) {
				fmt.Fprintf(os.Stderr, "beekeeper audit: syslog not supported on this platform — skipping\n")
			} else {
				_ = w.Close()
				return nil, fmt.Errorf("syslog sink: %w", serr)
			}
		} else {
			sinks = append(sinks, ss)
			remoteNames = append(remoteNames, "syslog")
		}
	}

	if containsString(cfg.Sinks, "otlp") && cfg.OTLPEndpoint != "" {
		// Finding #12 (MEDIUM): fail closed on a non-https or SSRF endpoint so a
		// misconfigured collector cannot exfiltrate audit records (which carry
		// the full record JSON) over cleartext or to instance-metadata services.
		if verr := ValidateRemoteSinkEndpoint(cfg.OTLPEndpoint, true); verr != nil {
			_ = w.Close()
			return nil, fmt.Errorf("otlp sink: %w", verr)
		}
		sinks = append(sinks, NewOTLPSink(cfg.OTLPEndpoint))
		remoteNames = append(remoteNames, "otlp")
	}

	if containsString(cfg.Sinks, "https") && cfg.HTTPSEndpoint != "" {
		// Finding #12 (MEDIUM): the "https" sink must actually require https and
		// reject SSRF targets — see ValidateRemoteSinkEndpoint.
		if verr := ValidateRemoteSinkEndpoint(cfg.HTTPSEndpoint, true); verr != nil {
			_ = w.Close()
			return nil, fmt.Errorf("https sink: %w", verr)
		}
		sinks = append(sinks, NewHTTPSink(cfg.HTTPSEndpoint))
		remoteNames = append(remoteNames, "https")
	}

	if len(remoteNames) > 0 {
		fmt.Fprintf(os.Stderr,
			"WARNING: audit data will leave this machine via %v sink(s). "+
				"Disable with audit.sinks in ~/.beekeeper/config.json.\n",
			remoteNames,
		)
	}

	return NewMultiSinkFromSinks(sinks), nil
}

// NewMultiSinkWithCorpus is identical to NewMultiSink but appends an additional
// caller-supplied corpus sink to the fan-out graph.
//
// Design (OQ-1 resolution, Option C — least invasive):
//   - NewMultiSink is UNCHANGED. Existing call sites are unaffected.
//   - The corpus sink is passed as an audit.Sink INTERFACE (not as a concrete
//     *corpus.StoreSink) to avoid an audit→corpus import cycle. The caller (cmd/beekeeper)
//     constructs the corpus.StoreSink and passes it in as audit.Sink.
//   - Fan-out semantics match MultiSink: every Write delivers to all sinks; errors
//     from individual sinks are accumulated and the last non-nil error is returned.
//     A corpus sink error does NOT prevent the file sink from receiving the record.
//
// corpusSink may be nil (e.g. when cfg.Corpus.Enabled is false); in that case
// NewMultiSinkWithCorpus behaves identically to NewMultiSink.
func NewMultiSinkWithCorpus(auditPath string, auditCfg config.AuditConfig, corpusSink Sink) (Sink, error) {
	base, err := NewMultiSink(auditPath, auditCfg)
	if err != nil {
		return nil, err
	}
	if corpusSink == nil {
		return base, nil
	}
	// Extract the existing sinks from the base MultiSink and append corpusSink.
	// NewMultiSink always returns a *MultiSink; use type assertion to access sinks.
	ms, ok := base.(*MultiSink)
	if !ok {
		// Defensive: if base is not a *MultiSink (should not happen), compose as a
		// two-sink MultiSink (base + corpus).
		return NewMultiSinkFromSinks([]Sink{base, corpusSink}), nil
	}
	// IN-05: copy explicitly so we never mutate ms.sinks's backing array in place
	// (nondeterministic aliasing if it has spare capacity).
	allSinks := append(append([]Sink{}, ms.sinks...), corpusSink)
	return NewMultiSinkFromSinks(allSinks), nil
}

// containsString reports whether slice contains s.
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
