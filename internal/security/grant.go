// Package security implements SPEC.md §4.1 and §6: the Grant types that
// admit a Client (Token, Basic, mTLS, TrustedHeader), the bind guard that
// refuses to start an unauthenticated server on anything but loopback, TLS
// setup, the header-env allowlist, the URL-arg gate, and log redaction. The
// fourth Grant — SQLite/JWT login (--auth-db) — is internal/store's and
// M12's job, not this package's.
package security

import "net/http"

// Identity is what a Grant reports for a request it admits: a human-facing
// label (SPEC.md §1's Client label) and the method that produced it, for
// logging and audit.
type Identity struct {
	Label  string
	Method string
}

// Grant authenticates an inbound request, admitting or rejecting it.
// Authenticate must never log or otherwise surface the credential it
// checked (SPEC.md §6, redact.go handles the log side).
type Grant interface {
	Authenticate(r *http.Request) (Identity, bool)
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
