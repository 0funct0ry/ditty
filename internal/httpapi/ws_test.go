package httpapi

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/0funct0ry/ditty/internal/fixture"
	"github.com/0funct0ry/ditty/internal/wire"
)

func dial(t *testing.T, url string, subprotocols []string) *websocket.Conn {
	t.Helper()
	dialer := websocket.Dialer{Subprotocols: subprotocols, HandshakeTimeout: 5 * time.Second}
	conn, resp, err := dialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	return conn
}

func wsURL(server *httptest.Server) string {
	return "ws" + strings.TrimPrefix(server.URL, "http") + "/"
}

// fixtureHubAdapter bridges *fixture.Hub to httpapi.Hub for tests, the same
// way cmd/run_fixture.go does for the real dev build (see Client's doc
// comment in ws.go for why an adapter, not a shared type, is needed).
type fixtureHubAdapter struct{ hub *fixture.Hub }

func (a fixtureHubAdapter) Attach(c Client) error            { return a.hub.Attach(c) }
func (a fixtureHubAdapter) Detach(id string)                 { a.hub.Detach(id) }
func (a fixtureHubAdapter) Input(id string, data []byte)     { a.hub.Input(id, data) }
func (a fixtureHubAdapter) Resize(id string, cols, rows int) { a.hub.Resize(id, cols, rows) }

func TestWSHandler_HelloFirst(t *testing.T) {
	hub := fixture.NewHubScaled(fixture.Scenarios()["deploy"], 0.02)
	server := httptest.NewServer(NewWSHandler(fixtureHubAdapter{hub: hub}))
	defer server.Close()

	conn := dial(t, wsURL(server), []string{wire.Subprotocol})
	defer func() { _ = conn.Close() }()

	_, frame, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	op, payload, err := wire.Decode(frame)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if op != wire.OpHello {
		t.Fatalf("first frame = %s, want Hello", op)
	}
	hello, err := wire.DecodeHello(payload)
	if err != nil {
		t.Fatalf("decode Hello: %v", err)
	}
	if hello.Session.Name != "deploy" {
		t.Fatalf("Hello.Session.Name = %q, want deploy", hello.Session.Name)
	}
}

func TestWSHandler_WrongSubprotocolCloses(t *testing.T) {
	hub := fixture.NewHubScaled(fixture.Scenarios()["deploy"], 0.02)
	server := httptest.NewServer(NewWSHandler(fixtureHubAdapter{hub: hub}))
	defer server.Close()

	conn := dial(t, wsURL(server), []string{"not-ditty"})
	defer func() { _ = conn.Close() }()

	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Fatal("expected the connection to close on subprotocol mismatch")
	}
	closeErr, ok := err.(*websocket.CloseError)
	if !ok {
		t.Fatalf("expected a close error, got %T: %v", err, err)
	}
	if closeErr.Code != websocket.CloseProtocolError {
		t.Fatalf("close code = %d, want %d", closeErr.Code, websocket.CloseProtocolError)
	}
}

func TestWSHandler_FlakyDropsAndReconnects(t *testing.T) {
	hub := fixture.NewHubScaled(fixture.Scenarios()["flaky"], 0.05)
	server := httptest.NewServer(NewWSHandler(fixtureHubAdapter{hub: hub}))
	defer server.Close()

	conn := dial(t, wsURL(server), []string{wire.Subprotocol})
	defer func() { _ = conn.Close() }()

	// Drain until the drop closes the connection.
	var closed bool
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !closed {
		if _, _, err := conn.ReadMessage(); err != nil {
			closed = true
		}
	}
	if !closed {
		t.Fatal("expected flaky to drop the connection")
	}

	// Reconnect and confirm replay includes output emitted before the drop.
	reconnect := dial(t, wsURL(server), []string{wire.Subprotocol})
	defer func() { _ = reconnect.Close() }()

	var sawOutput bool
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !sawOutput {
		_, frame, err := reconnect.ReadMessage()
		if err != nil {
			break
		}
		op, payload, err := wire.Decode(frame)
		if err != nil {
			continue
		}
		if op == wire.OpOutput && strings.Contains(string(payload), "connected") {
			sawOutput = true
		}
	}
	if !sawOutput {
		t.Fatal("reconnect did not receive replayed output")
	}
}
