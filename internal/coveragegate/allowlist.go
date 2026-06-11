package coveragegate

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// ReasonCode is a member of the CLOSED no-test allowlist taxonomy. Any code
// outside this set is rejected — the allowlist cannot be silently grown with an
// ad-hoc justification (VAL-08 / D-10).
type ReasonCode = string

// validReasons is the closed taxonomy. A path may only be allowlisted with one
// of these reason codes; anything else fails the parser closed.
var validReasons = map[ReasonCode]struct{}{
	"generated-bpf":  {}, // CI-generated eBPF bytecode (bpf_*_bpfel.go), never committed
	"platform-stub":  {}, // fail-closed per-OS shim with no logic (*_other.go / *_windows.go / *_unix.go)
	"type-only":      {}, // pure type/const/build-metadata, no behavior to test
	"exec-seam-stub": {}, // 1-line wrapper that exists only as a test seam (e.g. editorinit/lookup.go)
	"thin-delegator": {}, // delegates entirely to a tested function elsewhere (e.g. */sanity.go)
	"gen-directive":  {}, // go:generate directive carrier (e.g. sentry/linux/gen.go)
}

// ValidReasonCodes returns the closed taxonomy, sorted, for documentation and
// error messages.
func ValidReasonCodes() []string {
	codes := make([]string, 0, len(validReasons))
	for c := range validReasons {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	return codes
}

// Allowlist is the parsed coverage-allowlist.txt: a path -> reason-code map.
type Allowlist struct {
	entries map[string]string
}

// Has reports whether path (module-root-relative, forward-slash) is allowlisted.
func (a *Allowlist) Has(path string) bool {
	if a == nil {
		return false
	}
	_, ok := a.entries[path]
	return ok
}

// Reason returns the reason code for an allowlisted path, or "".
func (a *Allowlist) Reason(path string) string {
	if a == nil {
		return ""
	}
	return a.entries[path]
}

// Len returns the number of allowlist entries.
func (a *Allowlist) Len() int {
	if a == nil {
		return 0
	}
	return len(a.entries)
}

type lineKind int

const (
	lineSkip    lineKind = iota // blank or full-line comment
	lineEntry                   // valid "path<TAB># reason: <code>"
	lineInvalid                 // bare path, missing/empty reason, or out-of-taxonomy code
)

// classifyLine parses one allowlist line. The grammar is exactly:
//
//	<path><whitespace># reason: <code>
//
// A blank line or a line whose first non-space rune is '#' is lineSkip (header
// comments). Anything that is not a clean, in-taxonomy entry is lineInvalid —
// the parser fails closed on it rather than silently accounting the path.
func classifyLine(line string) (kind lineKind, path, reason string) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return lineSkip, "", ""
	}
	const marker = "# reason:"
	idx := strings.Index(line, marker)
	if idx < 0 {
		// A bare path with no reason code — the classic silent-weakening
		// attempt. Reject it.
		return lineInvalid, "", ""
	}
	path = strings.TrimSpace(line[:idx])
	reason = strings.TrimSpace(line[idx+len(marker):])
	if path == "" || reason == "" {
		return lineInvalid, "", ""
	}
	if _, ok := validReasons[reason]; !ok {
		return lineInvalid, "", ""
	}
	return lineEntry, path, reason
}

// ParseAllowlist reads reason-coded entries and FAILS CLOSED: if any line is a
// bare path, has an empty reason, or carries an out-of-taxonomy reason code,
// it returns an error and no usable allowlist. A tampered or sloppily-grown
// allowlist therefore breaks the gate loudly instead of silently lowering the
// coverage bar (VAL-08 / D-10).
func ParseAllowlist(r io.Reader) (*Allowlist, error) {
	entries := map[string]string{}
	sc := bufio.NewScanner(r)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		kind, path, reason := classifyLine(sc.Text())
		switch kind {
		case lineSkip:
			continue
		case lineEntry:
			if _, dup := entries[path]; dup {
				return nil, fmt.Errorf("coverage-allowlist: duplicate entry for %q (line %d)", path, lineNo)
			}
			entries[path] = reason
		case lineInvalid:
			return nil, fmt.Errorf("coverage-allowlist: line %d is not a valid 'path\\t# reason: <code>' entry with a code in %v: %q",
				lineNo, ValidReasonCodes(), sc.Text())
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return &Allowlist{entries: entries}, nil
}

// ParseAllowlistFile parses the allowlist at path. A missing file is an empty,
// valid allowlist (the gate then accounts purely by package-level linkage).
func ParseAllowlistFile(path string) (*Allowlist, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Allowlist{entries: map[string]string{}}, nil
		}
		return nil, err
	}
	defer f.Close()
	return ParseAllowlist(f)
}
