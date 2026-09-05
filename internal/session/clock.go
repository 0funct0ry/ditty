package session

import "time"

// Clock abstracts wall-clock time and timers so the §3.1 lifecycle timers
// (--wait-for-client, --detach-grace) can be driven deterministically in
// tests instead of via time.Sleep.
type Clock interface {
	Now() time.Time
	// AfterFunc schedules f to run after d and returns a Timer that can
	// cancel it, mirroring time.AfterFunc.
	AfterFunc(d time.Duration, f func()) Timer
}

// Timer is the subset of *time.Timer that lifecycle code needs.
type Timer interface {
	// Stop prevents f from firing, if it hasn't already. It returns false
	// if the timer had already fired or been stopped.
	Stop() bool
}

// realClock is the production Clock, backed by the standard library.
type realClock struct{}

// NewRealClock returns the production Clock backed by package time.
func NewRealClock() Clock { return realClock{} }

func (realClock) Now() time.Time { return time.Now() }

func (realClock) AfterFunc(d time.Duration, f func()) Timer {
	return time.AfterFunc(d, f)
}
