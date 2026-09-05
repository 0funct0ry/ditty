package session

import (
	"sync"
	"time"
)

// fakeClock is a manually-advanced Clock for lifecycle tests, so timer
// behaviour (--wait-for-client, --detach-grace) can be asserted without
// time.Sleep.
type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer
}

type fakeTimer struct {
	at      time.Time
	f       func()
	stopped bool
}

func (t *fakeTimer) Stop() bool {
	already := t.stopped
	t.stopped = true
	return !already
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Unix(0, 0)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) AfterFunc(d time.Duration, f func()) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{at: c.now.Add(d), f: f}
	c.timers = append(c.timers, t)
	return t
}

// Advance moves the clock forward by d and synchronously runs every timer
// due at or before the new time, in the order they were scheduled.
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	var due []func()
	remaining := c.timers[:0]
	for _, t := range c.timers {
		if t.stopped {
			continue
		}
		if !t.at.After(c.now) {
			due = append(due, t.f)
		} else {
			remaining = append(remaining, t)
		}
	}
	c.timers = remaining
	c.mu.Unlock()

	for _, f := range due {
		f()
	}
}
