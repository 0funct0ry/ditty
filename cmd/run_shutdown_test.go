package cmd

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/0funct0ry/ditty/internal/pty"
	"github.com/0funct0ry/ditty/internal/session"
)

// TestGracefulShutdown_ClosesCommandAndWaitsForSession exercises the M10
// graceful-shutdown path: closing the listener, then the real Command,
// drives internal/session's existing exit broadcast without a separate
// Notice-then-close API (see gracefulShutdown's doc comment).
func TestGracefulShutdown_ClosesCommandAndWaitsForSession(t *testing.T) {
	process, err := pty.Spawn(context.Background(), pty.Command{
		Argv: []string{"sleep", "30"},
	})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}

	hub := session.NewHub(session.Options{
		Process:       process,
		Name:          "shutdown-test",
		FlushInterval: time.Millisecond,
	})

	server := &http.Server{Handler: http.NotFoundHandler()}

	done := make(chan error, 1)
	go func() { done <- gracefulShutdown(server, process, hub) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("gracefulShutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("gracefulShutdown did not return within 5s")
	}

	select {
	case <-hub.Done():
	default:
		t.Fatal("Hub.Done() was not closed after gracefulShutdown returned")
	}
}
