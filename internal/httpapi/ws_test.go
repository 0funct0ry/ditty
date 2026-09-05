package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
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

// staticHub is a HubFactory that always returns the same Hub, matching
// --shared mode — the shape most of this file's tests exercise.
func staticHub(hub Hub) HubFactory {
	return func([]string) (Hub, error) { return hub, nil }
}

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
	server := httptest.NewServer(NewWSHandler(staticHub(NewSessionHub(hub, true)), WSOptions{}))
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
	server := httptest.NewServer(NewWSHandler(staticHub(NewSessionHub(hub, true)), WSOptions{}))
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
	server := httptest.NewServer(NewWSHandler(staticHub(NewSessionHub(hub, true)), WSOptions{}))
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

func TestWSHandler_CrossOriginUpgradeRejected403(t *testing.T) {
	hub, p := newTestSessionHub("deploy")
	defer func() { _ = p.Close() }()
	server := httptest.NewServer(NewWSHandler(staticHub(NewSessionHub(hub, true)), WSOptions{}))
	defer server.Close()

	dialer := websocket.Dialer{Subprotocols: []string{wire.Subprotocol}, HandshakeTimeout: 5 * time.Second}
	header := http.Header{"Origin": {"http://evil.example"}}
	_, resp, err := dialer.Dial(wsURL(server), header)
	if err == nil {
		t.Fatal("expected the cross-origin upgrade to fail")
	}
	if resp == nil {
		t.Fatal("expected an HTTP response alongside the dial error")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestWSHandler_OriginAllowRegexOverride(t *testing.T) {
	hub, p := newTestSessionHub("deploy")
	defer func() { _ = p.Close() }()
	opts := WSOptions{OriginAllow: regexp.MustCompile(`^https?://evil\.example$`)}
	server := httptest.NewServer(NewWSHandler(staticHub(NewSessionHub(hub, true)), opts))
	defer server.Close()

	dialer := websocket.Dialer{Subprotocols: []string{wire.Subprotocol}, HandshakeTimeout: 5 * time.Second}
	header := http.Header{"Origin": {"http://evil.example"}}
	conn, resp, err := dialer.Dial(wsURL(server), header)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	defer func() { _ = conn.Close() }()
}

func TestWSHandler_MaxClientsRejects1013(t *testing.T) {
	p := newScriptedProcess()
	defer func() { _ = p.Close() }()
	hub := session.NewHub(session.Options{
		Process:       p,
		Name:          "deploy",
		Server:        "ditty-test",
		FlushInterval: time.Millisecond,
		MaxClients:    1,
	})
	server := httptest.NewServer(NewWSHandler(staticHub(NewSessionHub(hub, true)), WSOptions{}))
	defer server.Close()

	first := dial(t, wsURL(server), []string{wire.Subprotocol})
	defer func() { _ = first.Close() }()
	if _, _, err := first.ReadMessage(); err != nil {
		t.Fatalf("first Hello: %v", err)
	}

	second := dial(t, wsURL(server), []string{wire.Subprotocol})
	defer func() { _ = second.Close() }()
	_, _, err := second.ReadMessage()
	if err == nil {
		t.Fatal("expected the second Client to be rejected")
	}
	closeErr, ok := err.(*websocket.CloseError)
	if !ok {
		t.Fatalf("expected a close error, got %T: %v", err, err)
	}
	if closeErr.Code != 1013 {
		t.Fatalf("close code = %d, want 1013", closeErr.Code)
	}
}

// drainUntilClose reads and discards data frames on conn until it errors
// (a close, or the deadline already set on conn), returning that error —
// letting tests skip past incidental frames (Roster, and so on) that a
// fixed read count would fail on.
func drainUntilClose(conn *websocket.Conn) error {
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return err
		}
	}
}

func TestWSHandler_TextFrameCloses1003(t *testing.T) {
	hub, p := newTestSessionHub("deploy")
	defer func() { _ = p.Close() }()
	server := httptest.NewServer(NewWSHandler(staticHub(NewSessionHub(hub, true)), WSOptions{}))
	defer server.Close()

	conn := dial(t, wsURL(server), []string{wire.Subprotocol})
	defer func() { _ = conn.Close() }()
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf("Hello: %v", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, []byte("hello")); err != nil {
		t.Fatalf("write text frame: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	err := drainUntilClose(conn)
	closeErr, ok := err.(*websocket.CloseError)
	if !ok {
		t.Fatalf("expected a close error, got %T: %v", err, err)
	}
	if closeErr.Code != websocket.CloseUnsupportedData {
		t.Fatalf("close code = %d, want %d", closeErr.Code, websocket.CloseUnsupportedData)
	}
}

func TestWSHandler_PingKeepsConnectionAlive(t *testing.T) {
	hub, p := newTestSessionHub("deploy")
	defer func() { _ = p.Close() }()
	opts := WSOptions{PingInterval: 20 * time.Millisecond}
	server := httptest.NewServer(NewWSHandler(staticHub(NewSessionHub(hub, true)), opts))
	defer server.Close()

	conn := dial(t, wsURL(server), []string{wire.Subprotocol})
	defer func() { _ = conn.Close() }()
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf("Hello: %v", err)
	}

	var pings atomic.Int32
	conn.SetPingHandler(func(string) error {
		pings.Add(1)
		return conn.WriteControl(websocket.PongMessage, nil, time.Now().Add(time.Second))
	})

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		_ = drainUntilClose(conn) // unblocked by this test's own conn.Close()
	}()

	deadline := time.Now().Add(2 * time.Second)
	for pings.Load() < 3 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	_ = conn.Close()
	<-readDone

	if got := pings.Load(); got < 3 {
		t.Fatalf("pings received = %d, want >= 3 within 2s", got)
	}
}

func TestWSHandler_MissingPongTimesOut(t *testing.T) {
	hub, p := newTestSessionHub("deploy")
	defer func() { _ = p.Close() }()
	// A tiny PingInterval makes the derived read deadline (2*interval+5s)
	// small enough to observe within a test timeout, while still leaving
	// server-sent pings unanswered by this raw connection.
	opts := WSOptions{PingInterval: 5 * time.Millisecond}
	server := httptest.NewServer(NewWSHandler(staticHub(NewSessionHub(hub, true)), opts))
	defer server.Close()

	conn := dial(t, wsURL(server), []string{wire.Subprotocol})
	defer func() { _ = conn.Close() }()
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf("Hello: %v", err)
	}
	// Swallow pings without replying, simulating a peer that never pongs.
	conn.SetPingHandler(func(string) error { return nil })

	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if err := drainUntilClose(conn); err == nil {
		t.Fatal("expected the server to close a connection that never pongs")
	}
}
