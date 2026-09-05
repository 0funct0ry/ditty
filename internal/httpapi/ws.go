package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"
	"regexp"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/0funct0ry/ditty/internal/wire"
)

// writeDeadline bounds every WebSocket write, per SPEC.md §5.5.
const writeDeadline = 5 * time.Second

// DefaultPingInterval is used when WSOptions.PingInterval is zero.
const DefaultPingInterval = 25 * time.Second

// Client is the transport-facing side of a Hub attachment: what a Hub sends
// frames to and closes. wsClient below is the WebSocket-backed
// implementation NewWSHandler constructs for every upgrade.
//
// This is deliberately httpapi's own type rather than a re-export of
// internal/session's identically-shaped (plus Writable) Client interface:
// internal/session must never import internal/httpapi or Gin. hub.go
// bridges the two structurally-identical interfaces with a small adapter;
// Go permits assigning between interface types whose method sets match
// without either package importing the other.
type Client interface {
	ID() string
	Label() string
	Send(frame []byte) error
	Close(code int, reason string)
}

// Hub is what NewWSHandler drives: attach/detach a Client, and route
// decoded Input/Resize frames to it. hub.go's sessionHubAdapter is the
// production implementation, bridging to internal/session's real Hub (M9).
type Hub interface {
	Attach(c Client) error
	Detach(id string)
	Input(id string, data []byte)
	Resize(id string, cols, rows int)
}

// HubFactory produces the Hub a single WebSocket connection attaches to.
// In shared mode (--shared) it always returns the same Hub; in ditty's
// default per-Client mode it spawns a brand-new Command/Session for every
// connection, so one Client's Command exiting never affects any other
// Client (SPEC.md §1's Session is scoped to one Client's connection, not to
// the whole `ditty run` invocation, in this mode). An error rejects the
// upgrade — e.g. --once refusing a second connection ever.
type HubFactory func() (Hub, error)

// WSOptions configures NewWSHandler's origin check and keepalive.
type WSOptions struct {
	// OriginAllow overrides the default same-host Origin check (SPEC.md
	// §6.3, --origin-allow): when set, an upgrade is admitted only if the
	// full Origin header matches this regex. Nil keeps the same-host
	// default.
	OriginAllow *regexp.Regexp
	// PingInterval is how often the server sends a WebSocket ping to each
	// attached Client (SPEC.md §5.5, --ping-interval). Zero uses
	// DefaultPingInterval.
	PingInterval time.Duration
}

// checkOrigin implements SPEC.md §6.3: no Origin header (a non-browser
// client) is always allowed; otherwise the default is same-host-as-request,
// overridden by opts.OriginAllow when set.
func checkOrigin(opts WSOptions) func(*http.Request) bool {
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		if opts.OriginAllow != nil {
			return opts.OriginAllow.MatchString(origin)
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		return u.Host == r.Host
	}
}

// NewWSHandler upgrades to WebSocket, requiring the ditty.v1 subprotocol
// (closing with 1002 on mismatch) and an admissible Origin (SPEC.md §6.3;
// gorilla/websocket answers a rejected origin with an HTTP 403 itself, from
// inside Upgrade). It calls newHub once per connection to get the Hub this
// Client attaches to — which sends Hello first — then relays frames
// between the socket and that Hub, sending a keepalive ping every
// opts.PingInterval, until the connection closes.
func NewWSHandler(newHub HubFactory, opts WSOptions) http.Handler {
	pingInterval := opts.PingInterval
	if pingInterval <= 0 {
		pingInterval = DefaultPingInterval
	}
	readDeadline := 2*pingInterval + 5*time.Second

	upgrader := websocket.Upgrader{
		Subprotocols: []string{wire.Subprotocol},
		CheckOrigin:  checkOrigin(opts),
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		if conn.Subprotocol() != wire.Subprotocol {
			closeConn(conn, websocket.CloseProtocolError, "subprotocol required: "+wire.Subprotocol)
			return
		}

		id, err := randomID()
		if err != nil {
			_ = conn.Close()
			return
		}
		client := &wsClient{id: id, label: "Client " + id[:6], conn: conn}

		hub, err := newHub()
		if err != nil {
			closeConn(conn, websocket.CloseTryAgainLater, err.Error())
			return
		}

		if err := hub.Attach(client); err != nil {
			code, reason := websocket.CloseTryAgainLater, "rejected"
			if cc, ok := err.(interface{ CloseCode() (int, string) }); ok {
				code, reason = cc.CloseCode()
			}
			closeConn(conn, code, reason)
			return
		}
		defer hub.Detach(id)

		_ = conn.SetReadDeadline(time.Now().Add(readDeadline))
		conn.SetPongHandler(func(string) error {
			return conn.SetReadDeadline(time.Now().Add(readDeadline))
		})

		done := make(chan struct{})
		defer close(done)
		go runPingLoop(conn, pingInterval, done)

		for {
			msgType, frame, err := conn.ReadMessage()
			if err != nil {
				// A read deadline expiring without any liveness from the
				// peer (SPEC.md §5.5) leaves the TCP connection otherwise
				// intact; close it explicitly rather than leaving it to the
				// peer to notice.
				_ = conn.Close()
				return
			}
			_ = conn.SetReadDeadline(time.Now().Add(readDeadline))
			if msgType != websocket.BinaryMessage {
				closeConn(conn, websocket.CloseUnsupportedData, "binary frames only")
				return
			}

			op, payload, err := wire.Decode(frame)
			if err != nil {
				continue
			}
			switch op {
			case wire.OpInput:
				hub.Input(id, payload)
			case wire.OpResize:
				resize, err := wire.DecodeResize(payload)
				if err != nil {
					continue
				}
				hub.Resize(id, resize.Cols, resize.Rows)
			case wire.OpPing:
				_ = client.Send([]byte{byte(wire.OpPong)})
			}
		}
	})
}

// runPingLoop sends a WebSocket-protocol ping every interval until done is
// closed (SPEC.md §5.5). A blocked/dead peer that never pongs is caught by
// the read deadline in NewWSHandler's caller, not here.
func runPingLoop(conn *websocket.Conn, interval time.Duration, done <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeDeadline)); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}

func closeConn(conn *websocket.Conn, code int, reason string) {
	msg := websocket.FormatCloseMessage(code, reason)
	_ = conn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(time.Second))
	_ = conn.Close()
}

func randomID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// wsClient implements httpapi.Client over a single WebSocket connection.
type wsClient struct {
	id    string
	label string
	conn  *websocket.Conn
	mu    sync.Mutex
}

func (c *wsClient) ID() string    { return c.id }
func (c *wsClient) Label() string { return c.label }

func (c *wsClient) Send(frame []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(writeDeadline))
	return c.conn.WriteMessage(websocket.BinaryMessage, frame)
}

func (c *wsClient) Close(code int, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	msg := websocket.FormatCloseMessage(code, reason)
	_ = c.conn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(time.Second))
	_ = c.conn.Close()
}
