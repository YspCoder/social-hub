package musicbrainz

import (
	"context"
	"sync"
	"time"
)

// requestGate spaces reservations across every client created by one adapter.
type requestGate struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

func newRequestGate(interval time.Duration) *requestGate {
	return &requestGate{interval: interval}
}

func (g *requestGate) Wait(ctx context.Context) error {
	if g == nil || g.interval <= 0 {
		return nil
	}
	g.mu.Lock()
	now := time.Now()
	slot := now
	if g.next.After(now) {
		slot = g.next
	}
	g.next = slot.Add(g.interval)
	g.mu.Unlock()

	delay := time.Until(slot)
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
