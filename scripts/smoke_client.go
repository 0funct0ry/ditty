// Command smoke_client drives a single real ditty.v1 WebSocket round trip
// against a running ditty instance: attach, expect Hello, type a known
// string, and assert it echoes back through the Command's own output
// (SPEC.md §14's smoke-test requirement — a plain HTTP health check alone
// doesn't exercise the wire protocol). Run via `go run ./scripts/smoke_client.go`,
// invoked by scripts/smoke.sh, never built into the release binary.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/0funct0ry/ditty/internal/wire"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:7654", "host:port to dial")
	basePath := flag.String("base-path", "/", "URL base path")
	flag.Parse()

	if err := run(*addr, *basePath); err != nil {
		log.Fatalf("smoke_client: %v", err)
	}
	fmt.Println("smoke_client: ok")
}

func run(addr, basePath string) error {
	u := url.URL{Scheme: "ws", Host: addr, Path: strings.TrimSuffix(basePath, "/") + "/ws"}

	dialer := websocket.Dialer{
		Subprotocols:     []string{wire.Subprotocol},
		HandshakeTimeout: 5 * time.Second,
	}
	conn, resp, err := dialer.Dial(u.String(), nil)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("dial %s: %w (status %s)", u.String(), err, resp.Status)
		}
		return fmt.Errorf("dial %s: %w", u.String(), err)
	}
	defer func() { _ = conn.Close() }()

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}

	if err := expectHello(conn); err != nil {
		return err
	}

	const marker = "ditty-smoke-marker"
	input, err := wire.Encode(wire.OpInput, []byte(marker+"\n"))
	if err != nil {
		return fmt.Errorf("encode Input: %w", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, input); err != nil {
		return fmt.Errorf("write Input: %w", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := conn.SetReadDeadline(deadline); err != nil {
			return err
		}
		_, data, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
		op, payload, err := wire.Decode(data)
		if err != nil {
			continue
		}
		if op == wire.OpOutput && strings.Contains(string(payload), marker) {
			return nil
		}
	}
	return fmt.Errorf("did not see %q echoed back within the deadline", marker)
}

func expectHello(conn *websocket.Conn) error {
	_, data, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("read Hello: %w", err)
	}
	op, payload, err := wire.Decode(data)
	if err != nil {
		return fmt.Errorf("decode Hello: %w", err)
	}
	if op != wire.OpHello {
		return fmt.Errorf("first frame was %s, want Hello", op)
	}
	if _, err := wire.DecodeHello(payload); err != nil {
		return fmt.Errorf("decode Hello payload: %w", err)
	}
	return nil
}
