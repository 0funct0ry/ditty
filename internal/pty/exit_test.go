//go:build linux || darwin || freebsd

package pty

import (
	"context"
	"syscall"
	"testing"
)

func TestExitCodeThree(t *testing.T) {
	p, err := Spawn(context.Background(), Command{Argv: []string{"sh", "-c", "exit 3"}})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	defer func() { _ = p.Close() }()

	status, err := waitTimeout(t, p)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if status.Signaled {
		t.Fatalf("status = %+v, want Signaled=false", status)
	}
	if status.Code != 3 {
		t.Fatalf("status.Code = %d, want 3", status.Code)
	}
}

func TestSignalTermReported(t *testing.T) {
	p, err := Spawn(context.Background(), Command{Argv: []string{"sh", "-c", "sleep 30"}})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	defer func() { _ = p.Close() }()

	if err := p.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("Signal: %v", err)
	}

	status, err := waitTimeout(t, p)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if !status.Signaled {
		t.Fatalf("status = %+v, want Signaled=true", status)
	}
	if status.Signal != syscall.SIGTERM.String() {
		t.Fatalf("status.Signal = %q, want %q", status.Signal, syscall.SIGTERM.String())
	}
}

func TestSignalAfterExitReturnsErr(t *testing.T) {
	p, err := Spawn(context.Background(), Command{Argv: []string{"true"}})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	defer func() { _ = p.Close() }()

	if _, err := waitTimeout(t, p); err != nil {
		t.Fatalf("Wait: %v", err)
	}

	if err := p.Signal(syscall.SIGTERM); err != errAlreadyDone {
		t.Fatalf("Signal after exit = %v, want errAlreadyDone", err)
	}
}
