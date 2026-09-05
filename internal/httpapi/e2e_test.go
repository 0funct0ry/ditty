package httpapi

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/0funct0ry/ditty/internal/pty"
	"github.com/0funct0ry/ditty/internal/session"
	"github.com/0funct0ry/ditty/internal/wire"

	"net/http/httptest"
)

// TestEndToEnd_RealBash exercises the full real stack — a real PTY-spawned
// bash, driven by a real internal/session Hub, served over a real
// WebSocket — the M10 acceptance criterion that the Phase 2 UI's transport
// works against a genuine Command, not a scripted double.
func TestEndToEnd_RealBash(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	process, err := pty.Spawn(ctx, pty.Command{
		Argv: []string{"bash", "--norc", "--noprofile"},
		Term: "xterm-256color",
		Cols: 80, Rows: 24,
	})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}

	hub := session.NewHub(session.Options{
		Process:       process,
		Name:          "e2e",
		Server:        "ditty-test",
		FlushInterval: time.Millisecond,
	})

	handler, err := NewRouter(Options{HubFactory: staticHub(NewSessionHub(hub, true)), Info: hub})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	conn := dial(t, "ws"+strings.TrimPrefix(server.URL, "http")+"/ws", []string{wire.Subprotocol})
	defer func() { _ = conn.Close() }()

	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf("Hello: %v", err)
	}

	input, err := wire.Encode(wire.OpInput, []byte("echo hello\n"))
	if err != nil {
		t.Fatalf("encode Input: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, input); err != nil {
		t.Fatalf("write Input: %v", err)
	}

	var sawHello bool
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && !sawHello {
		_, frame, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		op, payload, err := wire.Decode(frame)
		if err != nil {
			continue
		}
		if op == wire.OpOutput && strings.Contains(string(payload), "hello") {
			sawHello = true
		}
	}
	if !sawHello {
		t.Fatal("did not see 'hello' echoed back from the real Command")
	}

	exitInput, err := wire.Encode(wire.OpInput, []byte("exit\n"))
	if err != nil {
		t.Fatalf("encode Input: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, exitInput); err != nil {
		t.Fatalf("write Input: %v", err)
	}

	var exit *wire.Exit
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && exit == nil {
		_, frame, err := conn.ReadMessage()
		if err != nil {
			break
		}
		op, payload, err := wire.Decode(frame)
		if err != nil {
			continue
		}
		if op == wire.OpExit {
			e, err := wire.DecodeExit(payload)
			if err != nil {
				t.Fatalf("decode Exit: %v", err)
			}
			exit = &e
		}
	}
	if exit == nil {
		t.Fatal("did not receive an Exit frame after the Command exited")
	}
	if exit.Code != 0 {
		t.Fatalf("Exit.Code = %d, want 0", exit.Code)
	}
}
