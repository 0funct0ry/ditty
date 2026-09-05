package security

import (
	"fmt"
	"net"
	"net/http"
)

// TrustedHeaderGrant delegates authentication to a reverse proxy (nginx,
// oauth2-proxy, Cloudflare Access) via a header it sets after doing its own
// auth. It is honoured only when the peer address is inside one of the
// configured --trust-proxy CIDRs (SPEC.md §6.2) — with no --trust-proxy
// given, this Grant must never be constructed at all, so the header is
// ignored unconditionally (cmd/run.go's wiring enforces that, logging one
// startup WARN).
type TrustedHeaderGrant struct {
	header string
	cidrs  []*net.IPNet
}

// NewTrustedHeaderGrant parses header (--trust-header) and cidrs
// (--trust-proxy, one or more CIDR strings). At least one valid CIDR is
// required — a TrustedHeaderGrant with no trusted peers would silently
// never authenticate anyone, which is the caller's job to avoid by not
// constructing this Grant at all in that case.
func NewTrustedHeaderGrant(header string, cidrs []string) (*TrustedHeaderGrant, error) {
	g := &TrustedHeaderGrant{header: header}
	for _, c := range cidrs {
		_, ipnet, err := net.ParseCIDR(c)
		if err != nil {
			return nil, fmt.Errorf("security: --trust-proxy %q: %w", c, err)
		}
		g.cidrs = append(g.cidrs, ipnet)
	}
	return g, nil
}

// Authenticate admits a request only when RemoteAddr is inside a trusted
// CIDR, and only then reads the trusted header at all.
func (g *TrustedHeaderGrant) Authenticate(r *http.Request) (Identity, bool) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil || !g.trusted(ip) {
		return Identity{}, false
	}

	user := r.Header.Get(g.header)
	if user == "" {
		return Identity{}, false
	}
	return Identity{Label: user, Method: "trust-header"}, true
}

func (g *TrustedHeaderGrant) trusted(ip net.IP) bool {
	for _, cidr := range g.cidrs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}
