package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/0funct0ry/ditty/internal/wire"
)

// writeDeadline bounds every WebSocket write, per SPEC.md §5.5.
const writeDeadline = 5 * time.Second

// Client is the transport-facing side of a Hub attachment: what a Hub sends
// frames to and closes. wsClient below is the WebSocket-backed
// implementation NewWSHandler constructs for every upgrade.
//
// This is deliberately httpapi's own type rather than a re-export of
// internal/fixture's identically-shaped Client interface: httpapi must not
// import internal/fixture, or every build — including a release build
// without the "fixture" tag — would link it. The caller wiring a Hub
// implementation (cmd/run_fixture.go today, internal/session's transport
// wiring in M10) bridges the two structurally-identical interfaces with a
// small adapter; Go permits assigning between interface types whose method
// sets match without either package importing the other.
type Client interface {
	ID() string
	Label() string
	Send(frame []byte) error
	Close(code int, reason string)
}

// Hub is what NewWSHandler drives: attach/detach a Client, and route
// decoded Input/Resize frames to it. internal/fixture's Hub satisfies this
// shape today (via an adapter — see Client's doc comment); internal/
// session's real Hub (M9/M10) will too, so swapping one for the other only
// touches wiring, never this handler.
type Hub interface {
	Attach(c Client) error
	Detach(id string)
	Input(id string, data []byte)
	Resize(id string, cols, rows int)
}

var upgrader = websocket.Upgrader{
	Subprotocols: []string{wire.Subprotocol},
	// Origin enforcement (SPEC.md §6.3) arrives with the real router in
	// M10/M11; this dev-only transport only needs to be reachable.
	CheckOrigin: func(*http.Request) bool { return true },
}

// NewWSHandler upgrades to WebSocket, requiring the ditty.v1 subprotocol
// (closing with 1002 on mismatch), attaches a Client to hub, sends Hello
// first via that attach, then relays frames between the socket and hub
// until the connection closes.
func NewWSHandler(hub Hub) http.Handler {
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

		if err := hub.Attach(client); err != nil {
			_ = conn.Close()
			return
		}
		defer hub.Detach(id)

		for {
			msgType, frame, err := conn.ReadMessage()
			if err != nil {
				return
			}
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

// wsClient implements fixture.Client (and, later, whatever internal/session
// needs from a transport Client) over a single WebSocket connection.
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
