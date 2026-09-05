package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTrustedHeaderGrant_RequiresTrustedPeer(t *testing.T) {
	grant, err := NewTrustedHeaderGrant("X-Auth-User", []string{"10.0.0.0/8"})
	if err != nil {
		t.Fatalf("NewTrustedHeaderGrant: %v", err)
	}

	// Peer inside the trusted CIDR, with the header set: admitted.
	trusted := httptest.NewRequest(http.MethodGet, "/", nil)
	trusted.RemoteAddr = "10.1.2.3:54321"
	trusted.Header.Set("X-Auth-User", "alice")
	id, ok := grant.Authenticate(trusted)
	if !ok || id.Label != "alice" {
		t.Fatalf("Authenticate(trusted peer) = %+v, %v, want alice, true", id, ok)
	}

	// Peer outside the trusted CIDR: the header is never honoured, even
	// though it is present and well-formed (a forged header from an
	// untrusted peer must never authenticate).
	untrusted := httptest.NewRequest(http.MethodGet, "/", nil)
	untrusted.RemoteAddr = "203.0.113.9:54321"
	untrusted.Header.Set("X-Auth-User", "mallory")
	if _, ok := grant.Authenticate(untrusted); ok {
		t.Fatal("Authenticate(untrusted peer with forged header) = true, want false")
	}
}

func TestTrustedHeaderGrant_NoTrustProxyMeansNeverConstructed(t *testing.T) {
	// This documents the invariant cmd/run.go's wiring must uphold: with no
	// --trust-proxy CIDRs at all, callers must not construct a
	// TrustedHeaderGrant in the first place, so the header is ignored
	// unconditionally. NewTrustedHeaderGrant itself refuses an empty CIDR
	// list would-be caller from ending up with a Grant that trusts nothing
	// (and thus nothing that could be mistaken for "trusts everything").
	grant, err := NewTrustedHeaderGrant("X-Auth-User", nil)
	if err != nil {
		t.Fatalf("NewTrustedHeaderGrant(no cidrs): %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:54321"
	req.Header.Set("X-Auth-User", "alice")
	if _, ok := grant.Authenticate(req); ok {
		t.Fatal("Authenticate with no trusted CIDRs configured = true, want false")
	}
}
