package security

import (
	"testing"
	"time"
)

// fixedClock returns a func() time.Time reading *seconds as an offset from
// a fixed epoch, so a test can advance the throttle's clock deterministically
// instead of sleeping.
func fixedClock(seconds *int64) func() time.Time {
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return func() time.Time { return epoch.Add(time.Duration(*seconds) * time.Second) }
}

func TestLoginThrottle_LocksAfterMaxFailures(t *testing.T) {
	th := newLoginThrottle(nil)

	for i := 0; i < loginThrottleMax-1; i++ {
		th.recordFailure("alice")
		if locked, _ := th.locked("alice"); locked {
			t.Fatalf("locked after only %d failures", i+1)
		}
	}
	th.recordFailure("alice")
	locked, remaining := th.locked("alice")
	if !locked {
		t.Fatal("expected locked after loginThrottleMax failures")
	}
	if remaining <= 0 {
		t.Fatalf("remaining = %v, want > 0", remaining)
	}
}

func TestLoginThrottle_ResetClearsLock(t *testing.T) {
	th := newLoginThrottle(nil)
	for i := 0; i < loginThrottleMax; i++ {
		th.recordFailure("alice")
	}
	if locked, _ := th.locked("alice"); !locked {
		t.Fatal("expected locked")
	}
	th.reset("alice")
	if locked, _ := th.locked("alice"); locked {
		t.Fatal("expected unlocked after reset")
	}
}

func TestLoginThrottle_UnlocksAfterWindow(t *testing.T) {
	var current int64
	th := newLoginThrottle(fixedClock(&current))

	for i := 0; i < loginThrottleMax; i++ {
		th.recordFailure("alice")
	}
	if locked, _ := th.locked("alice"); !locked {
		t.Fatal("expected locked")
	}
	current += int64(loginThrottleLock.Seconds()) + 1
	if locked, _ := th.locked("alice"); locked {
		t.Fatal("expected unlocked after the lock window elapses")
	}
}

func TestLoginThrottle_IndependentPerUsername(t *testing.T) {
	th := newLoginThrottle(nil)
	for i := 0; i < loginThrottleMax; i++ {
		th.recordFailure("alice")
	}
	if locked, _ := th.locked("bob"); locked {
		t.Fatal("bob should not be locked by alice's failures")
	}
}
