// Package security implements SPEC.md §4.1 and §6: the Grant types that
// admit a Client (Token, Basic, mTLS, TrustedHeader, JWT), the bind guard
// that refuses to start an unauthenticated server on anything but loopback,
// TLS setup, the header-env allowlist, the URL-arg gate, and log redaction.
package security

import (
	"net/http"

	"github.com/0funct0ry/ditty/internal/store"
)

// Identity is what a Grant reports for a request it admits: a human-facing
// label (SPEC.md §1's Client label), the method that produced it (for
// logging and audit), and — for Grants with a role model (JWTGrant) — the
// Role that determines write capability (SPEC.md §9: operator may write
// when -w is set, viewer never writes). Role is empty for every Grant that
// carries no role (Token, Basic, mTLS, TrustedHeader); those fall back to
// the Session-wide -w flag deciding write capability alone.
type Identity struct {
	Label  string
	Method string
	Role   string
}

// Grant authenticates an inbound request, admitting or rejecting it.
// Authenticate must never log or otherwise surface the credential it
// checked (SPEC.md §6, redact.go handles the log side).
type Grant interface {
	Authenticate(r *http.Request) (Identity, bool)
}

// RoleAllowsWrite reports whether role permits write capability at all,
// independent of the Session-wide --writable flag (SPEC.md §9): operator
// may write when -w is set, viewer never writes even under -w — the actual
// write capability is writable && RoleAllowsWrite(role). A Grant that
// carries no role (Token, Basic, mTLS, TrustedHeader) reports role == "",
// which falls back to -w alone deciding write capability, matching every
// Grant that predates M12.
func RoleAllowsWrite(role string) bool {
	return role != store.RoleViewer
}

// Grants is an ordered set of active Grants (SPEC.md §6.2: "Multiple Grants
// can be active; any one admits"). A nil or empty Grants is only reachable
// when the bind guard has separately permitted no-auth (loopback, Unix
// socket, or --insecure-no-auth).
type Grants []Grant

// Authenticate returns the Identity from the first Grant that admits r.
func (g Grants) Authenticate(r *http.Request) (Identity, bool) {
	for _, grant := range g {
		if id, ok := grant.Authenticate(r); ok {
			return id, true
		}
	}
	return Identity{}, false
}
