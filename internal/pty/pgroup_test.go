//go:build linux || darwin || freebsd

package pty

import (
	"bufio"
	"context"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestClosesGrandchild verifies Close signals the whole process group, not
// just the direct child: a background grandchild started inside the shell
// must also be gone afterward.
func TestClosesGrandchild(t *testing.T) {
	p, err := Spawn(context.Background(), Command{
		Argv: []string{"sh", "-c", "sleep 300 & echo $!; wait"},
	})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}

	var grandchildPID int
	scanner := bufio.NewScanner(p)
	found := make(chan struct{})
	go func() {
		if scanner.Scan() {
			if pid, err := strconv.Atoi(strings.TrimSpace(scanner.Text())); err == nil {
				grandchildPID = pid
				close(found)
			}
		}
	}()

	select {
	case <-found:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out reading grandchild pid")
	}
	if grandchildPID == 0 {
		t.Fatal("did not capture a grandchild pid")
	}

	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		err := syscall.Kill(grandchildPID, 0)
		if err == syscall.ESRCH {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("grandchild pid %d still alive after Close (kill -0 err: %v)", grandchildPID, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
