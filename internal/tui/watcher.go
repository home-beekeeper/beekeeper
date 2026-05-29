package tui

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	tea "charm.land/bubbletea/v2"

	audit "github.com/mzansi-agentive/beekeeper/internal/audit"
)

// newRecordsMsg carries newly-appended audit records from the watcher goroutine.
type newRecordsMsg []audit.AuditRecord

// stateTick is sent every 5 seconds to refresh panel state.
type stateTick time.Time

// healthTick is sent every 10 seconds to refresh health pips.
type healthTick time.Time

// clockMsg is sent every second to update the live clock.
type clockMsg time.Time

func stateTickCmd() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg { return stateTick(t) })
}

func healthTickCmd() tea.Cmd {
	return tea.Tick(10*time.Second, func(t time.Time) tea.Msg { return healthTick(t) })
}

func clockCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return clockMsg(t) })
}

// tailFrom reads NDJSON lines from auditPath starting at offset.
// Returns new records and the new offset. Non-existent files return nil, offset.
//
// Only complete, newline-terminated lines advance the offset. A partial trailing
// line (record mid-write with no trailing newline yet) is NOT consumed — the
// returned offset stays before it so the next call re-reads and emits it once its
// newline has been written. Malformed-but-complete lines still advance the offset
// (they are complete; do not re-read them forever) but are silently skipped.
func tailFrom(auditPath string, offset int64) ([]audit.AuditRecord, int64) {
	f, err := os.Open(auditPath)
	if err != nil {
		return nil, offset
	}
	defer f.Close()

	if _, err := f.Seek(offset, 0); err != nil {
		return nil, offset
	}

	r := bufio.NewReader(f)
	var records []audit.AuditRecord
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			// Partial trailing line (no newline yet) or EOF with no data.
			// Do NOT advance the offset — re-read this fragment next tick.
			break
		}
		// Only complete (newline-terminated) lines advance the offset.
		offset += int64(len(line))
		var rec audit.AuditRecord
		if json.Unmarshal([]byte(strings.TrimRight(line, "\n")), &rec) == nil {
			records = append(records, rec)
		}
		// Malformed complete lines: offset already advanced; skip silently.
	}
	return records, offset
}

// watchAuditLog watches auditPath for new records and sends newRecordsMsg to p.
// Must be started as a goroutine before p.Run() is called.
func watchAuditLog(p *tea.Program, auditPath string) {
	var offset int64

	watcher, err := fsnotify.NewWatcher()
	if err == nil {
		defer watcher.Close()
		// Watch the parent directory — not the file itself (fsnotify requirement on some platforms).
		if watchErr := watcher.Add(filepath.Dir(auditPath)); watchErr != nil {
			// Fall through to ticker-only mode.
			watcher.Close()
			watcher = nil
		}
	} else {
		watcher = nil
	}

	fallback := time.NewTicker(time.Second)
	defer fallback.Stop()

	sendIfNew := func() {
		records, newOffset := tailFrom(auditPath, offset)
		offset = newOffset
		if len(records) > 0 {
			p.Send(newRecordsMsg(records))
		}
	}

	if watcher != nil {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) && event.Name == auditPath {
					sendIfNew()
				} else if event.Has(fsnotify.Create) && event.Name == auditPath {
					offset = 0
					sendIfNew()
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			case <-fallback.C:
				sendIfNew()
			}
		}
	} else {
		// Ticker-only fallback.
		for range fallback.C {
			sendIfNew()
		}
	}
}
