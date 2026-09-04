//go:build linux || darwin || freebsd

package pty

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"
)

// nextFD opens and immediately closes a throwaway file, returning the fd
// number the OS assigned it. Comparing this before and after a spawn/close
// loop detects a leaked file descriptor portably across linux, darwin and
// freebsd without relying on /proc/self/fd, which darwin and freebsd don't
// mount by default.
func nextFD(t *testing.T) int {
	t.Helper()
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer func() { _ = f.Close() }()
	return int(f.Fd())
}

func TestSpawnCloseLeakFree(t *testing.T) {
	const cycles = 100
	const tolerance = 5

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseGoroutines := runtime.NumGoroutine()
	baseFD := nextFD(t)

	for i := 0; i < cycles; i++ {
		p, err := Spawn(context.Background(), Command{Argv: []string{"echo", "hi"}})
		if err != nil {
			t.Fatalf("cycle %d: Spawn: %v", i, err)
		}
		if _, err := waitTimeout(t, p); err != nil {
			t.Fatalf("cycle %d: Wait: %v", i, err)
		}
		if err := p.Close(); err != nil {
			t.Fatalf("cycle %d: Close: %v", i, err)
		}
	}

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	endGoroutines := runtime.NumGoroutine()
	endFD := nextFD(t)

	if diff := endGoroutines - baseGoroutines; diff > tolerance {
		t.Errorf("goroutine count grew by %d (base %d, end %d), want <= %d", diff, baseGoroutines, endGoroutines, tolerance)
	}
	if diff := endFD - baseFD; diff > tolerance {
		t.Errorf("next fd number grew by %d (base %d, end %d), want <= %d — likely an fd leak", diff, baseFD, endFD, tolerance)
	}
}
