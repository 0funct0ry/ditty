// Package fixture is a temporary stand-in for the real Session/Client Hub
// (internal/session, arriving in M9). It speaks ditty.v1 exactly like the
// real Hub — join replay, roster, sizing, resize acknowledgement, exit —
// but is driven by a scripted Scenario instead of a live PTY. It exists for
// exactly one milestone: once internal/session clears every line of
// PARITY.md against the real Hub, this package and the --fixture flag are
// deleted (SPEC.md §12 M9).
//
// internal/fixture must never import internal/pty or any HTTP/Gin package
// (CLAUDE.md); the transport that drives a Hub is injected as the Client
// interface below, the same shape internal/session will accept in M9/M10.
package fixture

// Client is the transport-facing interface a Hub sends frames to and
// closes. internal/httpapi's WebSocket handler implements this today;
// internal/session's real transport client will implement the same shape.
type Client interface {
	// ID returns this Client's unique identifier for the lifetime of the
	// attachment.
	ID() string
	// Label returns a short human-readable label (e.g. "Client 1").
	Label() string
	// Send writes one already-encoded ditty.v1 frame. Implementations must
	// be safe to call concurrently with Close.
	Send(frame []byte) error
	// Close closes the underlying connection with a WebSocket close code
	// and reason.
	Close(code int, reason string)
}
