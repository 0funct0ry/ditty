package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// CookieName is the opaque grant cookie set by the token exchange (SPEC.md
// §4.1).
const CookieName = "ditty_grant"

// MinTokenLength is the minimum --token-length in bytes (SPEC.md §6.2).
const MinTokenLength = 8

// DefaultTokenLength is --token-length's default.
const DefaultTokenLength = 16

// handleLength is the opaque in-memory cookie handle's length in bytes
// (SPEC.md §4.1: "a random 32-byte opaque handle").
const handleLength = 32

// GenerateToken returns a random token of length bytes, base32-encoded
// without padding, refusing anything shorter than MinTokenLength.
func GenerateToken(length int) (string, error) {
	if length < MinTokenLength {
		return "", fmt.Errorf("security: --token-length must be at least %d bytes, got %d", MinTokenLength, length)
	}
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("security: generate token: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}

// TokenGrant implements the URL-token Grant (SPEC.md §4.1, §6.2): a single
// shared secret exchanged, once, for an opaque cookie handle held only in
// memory. Restarting the process discards every handle, invalidating every
// outstanding cookie.
type TokenGrant struct {
	token    string
	basePath string
	secure   bool
	ttl      time.Duration

	mu      sync.Mutex
	handles map[string]Identity
}

// NewTokenGrant constructs a TokenGrant for the given shared token. basePath
// is the cookie's Path attribute (--base-path); secure sets the cookie's
// Secure flag (true when TLS is in use); ttl is the cookie's Max-Age (the
// session TTL).
func NewTokenGrant(token, basePath string, secure bool, ttl time.Duration) *TokenGrant {
	if basePath == "" {
		basePath = "/"
	}
	return &TokenGrant{
		token:    token,
		basePath: basePath,
		secure:   secure,
		ttl:      ttl,
		handles:  make(map[string]Identity),
	}
}

// Token returns the shared token, for the startup banner/URL.
func (g *TokenGrant) Token() string { return g.token }

// Exchange validates candidate (the path segment from GET /t/:token) in
// constant time and, on success, mints a fresh opaque handle, sets the
// SPEC.md §4.1 cookie on w, and returns true. The caller is responsible for
// the 302 redirect (or 404 on false).
func (g *TokenGrant) Exchange(w http.ResponseWriter, candidate string) bool {
	if !constantTimeEqual(candidate, g.token) {
		return false
	}

	handle := make([]byte, handleLength)
	if _, err := rand.Read(handle); err != nil {
		return false
	}
	handleHex := hex.EncodeToString(handle)

	g.mu.Lock()
	g.handles[handleHex] = Identity{Label: "token", Method: "token"}
	g.mu.Unlock()

	cookie := &http.Cookie{
		Name:     CookieName,
		Value:    handleHex,
		Path:     g.basePath,
		HttpOnly: true,
		Secure:   g.secure,
		SameSite: http.SameSiteLaxMode,
	}
	if g.ttl > 0 {
		cookie.MaxAge = int(g.ttl.Seconds())
	}
	http.SetCookie(w, cookie)
	return true
}

// Authenticate admits a request carrying a valid grant cookie.
func (g *TokenGrant) Authenticate(r *http.Request) (Identity, bool) {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return Identity{}, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for handle, id := range g.handles {
		if constantTimeEqual(cookie.Value, handle) {
			return id, true
		}
	}
	return Identity{}, false
}

// constantTimeEqual compares a and b without leaking their contents via
// timing beyond the (public) fact that their lengths differ.
func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
