package httpapi

import (
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/0funct0ry/ditty/internal/pty"
	"github.com/0funct0ry/ditty/internal/session"
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

// scriptedProcess is a minimal session.Process double for exercising the
// real WebSocket transport end to end, without a real PTY: Read serves
// chunks fed via feed, Write/Resize/Signal are no-ops, and Close/Wait
// behave like a Command that has exited.
type scriptedProcess struct {
	out       chan []byte
	eof       chan struct{}
	waitDone  chan struct{}
	closeOnce sync.Once
}

func newScriptedProcess() *scriptedProcess {
	return &scriptedProcess{
		out:      make(chan []byte, 16),
		eof:      make(chan struct{}),
		waitDone: make(chan struct{}),
	}
}

func (p *scriptedProcess) feed(data []byte) {
	select {
	case p.out <- data:
	case <-p.eof:
	}
}

func (p *scriptedProcess) Read(b []byte) (int, error) {
	select {
	case chunk := <-p.out:
		return copy(b, chunk), nil
	case <-p.eof:
		select {
		case chunk := <-p.out:
			return copy(b, chunk), nil
		default:
			return 0, io.EOF
		}
	}
}

func (p *scriptedProcess) Write(b []byte) (int, error) { return len(b), nil }
func (p *scriptedProcess) Resize(uint16, uint16) error { return nil }
func (p *scriptedProcess) Signal(os.Signal) error      { return nil }

func (p *scriptedProcess) Close() error {
	p.closeOnce.Do(func() {
		close(p.eof)
		close(p.waitDone)
	})
	return nil
}

func (p *scriptedProcess) Wait() (pty.ExitStatus, error) {
	<-p.waitDone
	return pty.ExitStatus{}, nil
}

// sessionClientAdapter satisfies internal/session's Client interface over
// an httpapi.Client, hardcoding the write capability tests need — real
// write-capability plumbing (Grants, roles) arrives in M11/M12.
type sessionClientAdapter struct {
	httpClient Client
	writable   bool
}

func (a sessionClientAdapter) ID() string                    { return a.httpClient.ID() }
func (a sessionClientAdapter) Label() string                 { return a.httpClient.Label() }
func (a sessionClientAdapter) Writable() bool                { return a.writable }
func (a sessionClientAdapter) Send(frame []byte) error       { return a.httpClient.Send(frame) }
func (a sessionClientAdapter) Close(code int, reason string) { a.httpClient.Close(code, reason) }

// sessionHubAdapter bridges *session.Hub to httpapi.Hub for tests, the
// same way M10's real transport wiring will (see Client's doc comment in
// ws.go for why an adapter, not a shared type, is needed).
type sessionHubAdapter struct{ hub *session.Hub }

func (a sessionHubAdapter) Attach(c Client) error {
	return a.hub.Attach(sessionClientAdapter{httpClient: c})
}
func (a sessionHubAdapter) Detach(id string)                 { a.hub.Detach(id) }
func (a sessionHubAdapter) Input(id string, data []byte)     { a.hub.Input(id, data) }
func (a sessionHubAdapter) Resize(id string, cols, rows int) { a.hub.Resize(id, cols, rows) }

func newTestSessionHub(name string) (*session.Hub, *scriptedProcess) {
	p := newScriptedProcess()
	hub := session.NewHub(session.Options{
		Process:         p,
		Name:            name,
		Server:          "ditty-test",
		FlushInterval:   time.Millisecond,
		ScrollbackBytes: session.DefaultScrollbackBytes,
		// A long DetachGrace keeps a transient zero-Client moment (this
		// package's own reconnect test detaches and reattaches) from
		// closing the process before the test can reconnect; internal/
		// session's own tests cover --detach-grace's real timing.
		DetachGrace: time.Hour,
	})
	return hub, p
}

func TestWSHandler_HelloFirst(t *testing.T) {
	hub, p := newTestSessionHub("deploy")
	defer func() { _ = p.Close() }()
	server := httptest.NewServer(NewWSHandler(sessionHubAdapter{hub: hub}))
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
	hub, p := newTestSessionHub("deploy")
	defer func() { _ = p.Close() }()
	server := httptest.NewServer(NewWSHandler(sessionHubAdapter{hub: hub}))
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

func TestWSHandler_DisconnectReconnect_ReplaysOutput(t *testing.T) {
	hub, p := newTestSessionHub("flaky")
	defer func() { _ = p.Close() }()
	server := httptest.NewServer(NewWSHandler(sessionHubAdapter{hub: hub}))
	defer server.Close()

	p.feed([]byte("connected — streaming logs"))

	conn := dial(t, wsURL(server), []string{wire.Subprotocol})
	// Drain Hello + the first Output before simulating a network drop.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, frame, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read before drop: %v", err)
		}
		if op, _, _ := wire.Decode(frame); op == wire.OpOutput {
			break
		}
	}
	_ = conn.Close() // simulate a dropped connection

	// Give the server time to observe the read error and Detach, then
	// emit more output while no Client is attached.
	time.Sleep(50 * time.Millisecond)
	p.feed([]byte("reconnected — resuming"))
	time.Sleep(50 * time.Millisecond)

	reconnect := dial(t, wsURL(server), []string{wire.Subprotocol})
	defer func() { _ = reconnect.Close() }()

	var sawReconnected bool
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !sawReconnected {
		_, frame, err := reconnect.ReadMessage()
		if err != nil {
			break
		}
		op, payload, err := wire.Decode(frame)
		if err != nil {
			continue
		}
		if op == wire.OpOutput && strings.Contains(string(payload), "reconnected") {
			sawReconnected = true
		}
	}
	if !sawReconnected {
		t.Fatal("reconnect did not receive replayed output emitted during the drop")
	}
}
