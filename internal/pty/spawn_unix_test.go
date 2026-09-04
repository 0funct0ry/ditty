//go:build linux || darwin || freebsd

package pty

import (
	"bufio"
	"context"
	"strings"
	"testing"
	"time"
)

func TestSpawnEchoExitZero(t *testing.T) {
	p, err := Spawn(context.Background(), Command{Argv: []string{"echo", "hi"}})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	defer func() { _ = p.Close() }()

	scanner := bufio.NewScanner(p)
	if !scanner.Scan() {
		t.Fatalf("expected output line, scan error: %v", scanner.Err())
	}
	if got := strings.TrimSpace(scanner.Text()); got != "hi" {
		t.Fatalf("output = %q, want %q", got, "hi")
	}

	status, err := waitTimeout(t, p)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if status.Signaled || status.Code != 0 {
		t.Fatalf("status = %+v, want exit 0", status)
	}
}

func TestSpawnMissingBinary(t *testing.T) {
	_, err := Spawn(context.Background(), Command{Argv: []string{"ditty-does-not-exist-binary"}})
	if err == nil {
		t.Fatal("expected an error spawning a missing binary")
	}
}

func TestSpawnEmptyArgv(t *testing.T) {
	_, err := Spawn(context.Background(), Command{})
	if err != errEmptyArgv {
		t.Fatalf("err = %v, want errEmptyArgv", err)
	}
}

// waitTimeout calls p.Wait() but fails the test instead of hanging forever
// if the process never exits.
func waitTimeout(t *testing.T, p *Process) (ExitStatus, error) {
	t.Helper()
	type result struct {
		status ExitStatus
		err    error
	}
	ch := make(chan result, 1)
	go func() {
		status, err := p.Wait()
		ch <- result{status, err}
	}()
	select {
	case r := <-ch:
		return r.status, r.err
	case <-time.After(5 * time.Second):
		t.Fatal("Wait timed out")
		return ExitStatus{}, nil
	}
}
