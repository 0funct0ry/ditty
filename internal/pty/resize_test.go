//go:build linux || darwin || freebsd

package pty

import (
	"bufio"
	"context"
	"strings"
	"testing"
	"time"
)

// TestResizeReflectedInStty spawns an interactive shell, resizes the PTY,
// then triggers `stty size` by writing it as a command line. The resize
// must happen before the triggering write — spawning `sh -c 'stty size'`
// directly races the shell exiting against the Resize call, so this test
// keeps the shell alive and drives it explicitly instead.
func TestResizeReflectedInStty(t *testing.T) {
	p, err := Spawn(context.Background(), Command{Argv: []string{"sh"}})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	defer func() { _ = p.Close() }()

	if err := p.Resize(100, 30); err != nil {
		t.Fatalf("Resize: %v", err)
	}

	// Give the shell a moment to finish starting up before driving it.
	time.Sleep(100 * time.Millisecond)

	if _, err := p.Write([]byte("stty size\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	found := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(p)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "30 100") {
				close(found)
				return
			}
		}
	}()

	select {
	case <-found:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for \"30 100\" in stty size output")
	}
}

func TestResizeRejectsZero(t *testing.T) {
	p, err := Spawn(context.Background(), Command{Argv: []string{"sh"}})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	defer func() { _ = p.Close() }()

	if err := p.Resize(0, 30); err == nil {
		t.Fatal("expected an error resizing to cols=0")
	}
	if err := p.Resize(30, 0); err == nil {
		t.Fatal("expected an error resizing to rows=0")
	}
}
