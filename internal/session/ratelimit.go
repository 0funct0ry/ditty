package session

import (
	"sync"
	"time"
)

// inputRateLimitBytesPerSecond is the sustained per-Client Input budget
// (SPEC.md §6.6): a Client that keeps exceeding it is disconnected.
const inputRateLimitBytesPerSecond = 64 * 1024

// rateLimitSustainedBreach is how long a Client must stay continuously over
// budget before the Hub closes it, rather than reacting to a single burst.
const rateLimitSustainedBreach = time.Second

// tokenBucket is a per-Client input rate limiter: bytes/sec sustained, with
// a one-second burst allowance.
type tokenBucket struct {
	mu    sync.Mutex
	clock Clock

	rate   float64 // bytes/sec
	burst  float64
	tokens float64
	last   time.Time
	overAt time.Time // zero if currently within budget
}

func newTokenBucket(ratePerSecond float64, clock Clock) *tokenBucket {
	return &tokenBucket{
		clock:  clock,
		rate:   ratePerSecond,
		burst:  ratePerSecond,
		tokens: ratePerSecond,
		last:   clock.Now(),
	}
}

// Allow reports whether n bytes may be admitted now, consuming tokens if
// so. A false return leaves the bucket's "over budget" clock running (see
// Sustained) until a subsequent Allow succeeds.
func (b *tokenBucket) Allow(n int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.clock.Now()
	if elapsed := now.Sub(b.last).Seconds(); elapsed > 0 {
		b.tokens += elapsed * b.rate
		if b.tokens > b.burst {
			b.tokens = b.burst
		}
		b.last = now
	}

	if b.tokens < float64(n) {
		if b.overAt.IsZero() {
			b.overAt = now
		}
		return false
	}
	b.tokens -= float64(n)
	b.overAt = time.Time{}
	return true
}

// Sustained reports whether the bucket has been continuously over budget
// for at least d.
func (b *tokenBucket) Sustained(d time.Duration) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return !b.overAt.IsZero() && b.clock.Now().Sub(b.overAt) >= d
}
