// Package history keeps a log of the emails the plugin sent or failed to
// send, so users can confirm delivery, see errors and resend.
//
// The log is in memory and shared by every app instance in the plugin
// process (Grafana creates a new instance when settings change). It holds
// the most recent entries only and is cleared when Grafana restarts. When
// Grafana runs on several servers, each server has its own log.
package history

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/MiteshDhangar/beautify-alert/pkg/delivery"
)

// Capacity is how many entries the log keeps.
const Capacity = 500

// maxStoredBody is the largest HTML body kept for viewing and resending.
const maxStoredBody = 256 << 10

// Sources of an entry.
const (
	SourceAlert  = "alert"
	SourceTest   = "test"
	SourceResend = "resend"
)

// Entry is one send attempt.
type Entry struct {
	ID          string    `json:"id"`
	Time        time.Time `json:"time"`
	Source      string    `json:"source"`
	Template    string    `json:"template"`
	To          []string  `json:"to"`
	Subject     string    `json:"subject"`
	AlertStatus string    `json:"alertStatus,omitempty"`
	Alerts      int       `json:"alerts"`
	OK          bool      `json:"ok"`
	Error       string    `json:"error,omitempty"`
	// Message is kept for viewing and resending; nil when the body was too large.
	Message *delivery.Message `json:"message,omitempty"`
}

// Summary returns e without its message body, for listing.
func (e Entry) Summary() Entry {
	e.Message = nil
	return e
}

// Log is a fixed-size, concurrency-safe log of entries, newest first.
type Log struct {
	mu      sync.RWMutex
	entries []Entry // ring buffer
	next    int
	full    bool
}

// New returns an empty log.
func New() *Log { return &Log{entries: make([]Entry, Capacity)} }

// Default is the process-wide log.
var Default = New()

// Add records e, filling in its ID and time when empty, and returns it.
func (l *Log) Add(e Entry) Entry {
	if e.ID == "" {
		e.ID = newID()
	}
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}
	if e.Message != nil && len(e.Message.HTML) > maxStoredBody {
		e.Message = nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries[l.next] = e
	l.next = (l.next + 1) % len(l.entries)
	if l.next == 0 {
		l.full = true
	}
	return e
}

// List returns up to limit entries, newest first, without message bodies.
func (l *Log) List(limit int) []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	n := l.next
	if l.full {
		n = len(l.entries)
	}
	if limit <= 0 || limit > n {
		limit = n
	}
	out := make([]Entry, 0, limit)
	for i := 0; i < limit; i++ {
		idx := (l.next - 1 - i + len(l.entries)) % len(l.entries)
		out = append(out, l.entries[idx].Summary())
	}
	return out
}

// Get returns the entry with id, including its message.
func (l *Log) Get(id string) (Entry, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for _, e := range l.entries {
		if e.ID == id && id != "" {
			return e, true
		}
	}
	return Entry{}, false
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
