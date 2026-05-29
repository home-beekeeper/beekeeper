package tui

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
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
func tailFrom(auditPath string, offset int64) ([]audit.AuditRecord, int64) {
	f, err := os.Open(auditPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, offset
		}
		return nil, offset
	}
	defer f.Close()

	if _, err := f.Seek(offset, 0); err != nil {
		return nil, offset
	}

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 1024*1024) // 1MB buffer
	scanner.Buffer(buf, len(buf))

	var records []audit.AuditRecord
	for scanner.Scan() {
		line := scanner.Bytes()
		var rec audit.AuditRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			// Skip malformed lines silently.
			continue
		}
		records = append(records, rec)
	}

	newOffset, _ := f.Seek(0, 1)
	if newOffset == 0 {
		// Seek to current position via whence=1 (current) with 0 offset
		info, err := f.Stat()
		if err == nil {
			newOffset = info.Size()
		} else {
			newOffset = offset
		}
	}
	return records, newOffset
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
