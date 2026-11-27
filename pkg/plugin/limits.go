package plugin

import (
	"sync"
	"time"
)

// renderSlots limits how many previews and test emails render at once across
// the whole plugin. Templates in those requests are unsaved code from the
// caller; the render limits bound each one, and this bounds how many run
// together. Alert emails do not use slots, so previews cannot delay alerts.
var renderSlots = make(chan struct{}, 4)

func acquireRenderSlot() bool {
	select {
	case renderSlots <- struct{}{}:
		return true
	default:
		return false
	}
}

func releaseRenderSlot() { <-renderSlots }

// Test emails per organization per window. Package level so it survives the
// new App instance Grafana creates when settings are saved.
const (
	testEmailLimit  = 10
	testEmailWindow = time.Minute
)

var testLimiter = &windowLimiter{limit: testEmailLimit, window: testEmailWindow, hits: map[string][]time.Time{}}

// windowLimiter allows limit events per key in any sliding window.
type windowLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string][]time.Time
}

// allow records an event for key and reports whether it is within the limit,
// and if not, how long until the next one is.
func (l *windowLimiter) allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	recent := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if now.Sub(t) < l.window {
			recent = append(recent, t)
		}
	}
	if len(recent) >= l.limit {
		l.hits[key] = recent
		return false, l.window - now.Sub(recent[0])
	}
	l.hits[key] = append(recent, now)
	return true, 0
}
