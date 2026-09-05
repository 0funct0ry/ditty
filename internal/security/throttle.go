package security

import (
	"sync"
	"time"
)

// loginThrottleWindow and loginThrottleMax implement SPEC.md §9's login
// throttle: 5 failures per username per minute, then a 30s lock. This is
// not part of §9's schema (which defines no throttle table) — it is an
// in-memory, per-JWTGrant interpretation, matching the M7 fixture responder
// (web/src/protocol/fixtureAuthClient.ts) so the real login screen's
// lockout copy fires under the same conditions it was built against.
const (
	loginThrottleWindow = time.Minute
	loginThrottleMax    = 5
	loginThrottleLock   = 30 * time.Second
)

// loginThrottle tracks recent login failures per username, in memory only —
// a restart clears every lock, consistent with a JWTGrant's random-per-run
// secret already invalidating every session on restart.
type loginThrottle struct {
	mu          sync.Mutex
	failures    map[string][]time.Time
	lockedUntil map[string]time.Time
	now         func() time.Time
}

func newLoginThrottle(now func() time.Time) *loginThrottle {
	if now == nil {
		now = time.Now
	}
	return &loginThrottle{
		failures:    make(map[string][]time.Time),
		lockedUntil: make(map[string]time.Time),
		now:         now,
	}
}

// locked reports whether username is currently locked out, and for how much
// longer.
func (t *loginThrottle) locked(username string) (bool, time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	until, ok := t.lockedUntil[username]
	if !ok {
		return false, 0
	}
	now := t.now()
	if now.After(until) {
		delete(t.lockedUntil, username)
		return false, 0
	}
	return true, until.Sub(now)
}

// recordFailure adds one failure for username, locking it out once
// loginThrottleMax failures have landed within loginThrottleWindow. A
// failure recorded while already locked does not extend the lock.
func (t *loginThrottle) recordFailure(username string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, locked := t.lockedUntil[username]; locked {
		return
	}
	now := t.now()
	cutoff := now.Add(-loginThrottleWindow)
	recent := t.failures[username][:0]
	for _, ts := range t.failures[username] {
		if ts.After(cutoff) {
			recent = append(recent, ts)
		}
	}
	recent = append(recent, now)
	t.failures[username] = recent
	if len(recent) >= loginThrottleMax {
		t.lockedUntil[username] = now.Add(loginThrottleLock)
		delete(t.failures, username)
	}
}

// reset clears every failure/lock for username, called on a successful
// login.
func (t *loginThrottle) reset(username string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.failures, username)
	delete(t.lockedUntil, username)
}
