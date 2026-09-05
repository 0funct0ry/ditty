package security

import (
	"fmt"
	"net/http"
	"strings"
)

// BasicGrant implements HTTP basic authentication (SPEC.md §6.2). Refusing
// to allow it over plaintext non-loopback (unless --insecure-basic-over-http)
// is enforced by the caller (cmd/run.go) via the bind guard / an explicit
// check before this Grant is ever constructed, not here.
type BasicGrant struct {
	user string
	pass string
}

// NewBasicGrant parses "user:pass" (--basic-auth / DITTY_BASIC_AUTH).
func NewBasicGrant(userPass string) (*BasicGrant, error) {
	user, pass, ok := strings.Cut(userPass, ":")
	if !ok || user == "" {
		return nil, fmt.Errorf("security: --basic-auth must be user:pass, got %q", userPass)
	}
	return &BasicGrant{user: user, pass: pass}, nil
}

// Authenticate checks the request's Authorization: Basic header in constant
// time against both the configured username and password.
func (g *BasicGrant) Authenticate(r *http.Request) (Identity, bool) {
	user, pass, ok := r.BasicAuth()
	if !ok {
		return Identity{}, false
	}
	userOK := constantTimeEqual(user, g.user)
	passOK := constantTimeEqual(pass, g.pass)
	if !userOK || !passOK {
		return Identity{}, false
	}
	return Identity{Label: user, Method: "basic"}, true
}
