// Package session implements the real Hub — internal/pty (M8) plus one
// Session lifecycle state machine, fanned out to any number of Clients
// (SPEC.md §1, §3). It is the real replacement for internal/fixture: it
// speaks the same ditty.v1 protocol, but is driven by a live PTY instead of
// a scripted Scenario.
//
// internal/session must never import internal/httpapi or Gin (CLAUDE.md);
// the transport that drives a Hub is injected as the Client interface
// below, the same shape internal/fixture's Client used and internal/
// httpapi's WebSocket handler (M10) will implement.
package session

// Client is the transport-facing interface a Hub sends frames to and
// closes. It is a superset of internal/fixture's Client: Writable reports
// whether this Client currently has write capability (SPEC.md §5.4, §6.1),
// since the real Hub — unlike the fixture — actually enforces it.
type Client interface {
	// ID returns this Client's unique identifier for the lifetime of the
	// attachment.
	ID() string
	// Label returns a short human-readable label (e.g. "Client 1").
	Label() string
	// Writable reports whether this Client may send Input frames the Hub
	// will forward to the PTY. Captured once at Attach time.
	Writable() bool
	// Send writes one already-encoded ditty.v1 frame. Implementations must
	// be safe to call concurrently with Close.
	Send(frame []byte) error
	// Close closes the underlying connection with a WebSocket close code
	// and reason.
	Close(code int, reason string)
}

// ErrMaxClients is returned by Attach when the Session is already at
// --max-clients capacity. It implements CloseCode so a transport can
// reject the Client with the right WebSocket close code instead of a
// generic close (SPEC.md §5.5/§6, matches internal/fixture's ErrMaxClients).
type ErrMaxClients struct{ Max int }

func (e *ErrMaxClients) Error() string { return "too many clients" }

// CloseCode reports the WebSocket close code and reason a transport should
// use to reject the Client (1013, "try again later").
func (e *ErrMaxClients) CloseCode() (int, string) { return 1013, "too many clients" }
