package security

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/0funct0ry/ditty/internal/store"
)

// AccessCookieName and RefreshCookieName are the two cookies a JWTGrant
// login sets (SPEC.md §6.2, §9): a short-lived HS256 access token and an
// opaque, rotating refresh handle.
const (
	AccessCookieName  = "ditty_access"
	RefreshCookieName = "ditty_refresh"
)

// AccessTokenTTL is the access JWT's lifetime (SPEC.md §6.2: "15-min access
// cookie").
const AccessTokenTTL = 15 * time.Minute

// RefreshCeiling is how long a refresh token may keep renewing an access
// token, measured from the first login, not a sliding window (SPEC.md
// §6.2: "rotating refresh, 12-hour ceiling").
const RefreshCeiling = 12 * time.Hour

// refreshHandleLength matches TokenGrant's opaque handle length.
const refreshHandleLength = 32

// accessClaims is the access JWT's payload (SPEC.md §9's role model).
type accessClaims struct {
	Username string `json:"usr"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// refreshRecord is what a refresh handle maps to: the username it refreshes
// on behalf of, and the absolute time past which it may no longer renew.
type refreshRecord struct {
	username string
	ceiling  time.Time
}

// LoginResult is what Login/Refresh report: on success OK is true and
// AccessToken/RefreshToken/Role are set; on failure OK is false and Reason
// is a human-readable message — this is the exact shape M7's AuthClient
// LoginResult expects (web/src/protocol/auth.ts), which internal/httpapi's
// login handler serializes almost verbatim.
type LoginResult struct {
	OK           bool
	Role         string
	Reason       string
	AccessToken  string
	RefreshToken string
	Ceiling      time.Time
}

// JWTGrant implements the SQLite/JWT login Grant (SPEC.md §6.2, §9): it
// authenticates ditty_access cookies for every other route, and separately
// exposes Login/Refresh/Logout for internal/httpapi's /api/login,
// /api/refresh and /api/logout handlers. It calls into store.Store for
// credentials and audit, but never touches database/sql itself.
type JWTGrant struct {
	secret   []byte
	store    *store.Store
	basePath string
	secure   bool
	now      func() time.Time
	throttle *loginThrottle

	mu      sync.Mutex
	refresh map[string]refreshRecord
}

// NewJWTGrant constructs a JWTGrant. secret signs/verifies the access JWT
// (random per-run unless --jwt-secret is given: restarting invalidates
// every session, same story as TokenGrant). basePath and secure set the
// cookies' Path and Secure attributes, matching TokenGrant's cookie policy.
func NewJWTGrant(secret []byte, st *store.Store, basePath string, secure bool) *JWTGrant {
	if basePath == "" {
		basePath = "/"
	}
	return &JWTGrant{
		secret:   secret,
		store:    st,
		basePath: basePath,
		secure:   secure,
		now:      time.Now,
		throttle: newLoginThrottle(nil),
		refresh:  make(map[string]refreshRecord),
	}
}

// SetNow overrides JWTGrant's time source (real time by default). It exists
// so tests can exercise access-token expiry and the refresh ceiling
// deterministically, without sleeping.
func (g *JWTGrant) SetNow(now func() time.Time) {
	g.now = now
}

// Authenticate admits a request carrying a valid, unexpired ditty_access
// cookie.
func (g *JWTGrant) Authenticate(r *http.Request) (Identity, bool) {
	cookie, err := r.Cookie(AccessCookieName)
	if err != nil {
		return Identity{}, false
	}
	claims := &accessClaims{}
	token, err := jwt.ParseWithClaims(cookie.Value, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("security: unexpected signing method %v", t.Header["alg"])
		}
		return g.secret, nil
	}, jwt.WithTimeFunc(g.now))
	if err != nil || !token.Valid {
		return Identity{}, false
	}
	return Identity{Label: claims.Username, Method: "jwt", Role: claims.Role}, true
}

// Login checks username/password against the Store, honouring the login
// throttle (SPEC.md §9's interpretation: 5 failures/60s -> 30s lock). A
// wrong password, a disabled account, an unknown user, and a currently
// locked-out username are all reported the same way — OK=false with a
// human-readable Reason — so a caller can serialize LoginResult straight
// into M7's AuthClient response shape.
func (g *JWTGrant) Login(username, password string) (LoginResult, error) {
	if locked, remaining := g.throttle.locked(username); locked {
		return LoginResult{Reason: fmt.Sprintf(
			"too many failed attempts; try again in %d seconds", int(remaining.Round(time.Second).Seconds()),
		)}, nil
	}

	user, ok, err := g.store.VerifyPassword(username, password)
	if err != nil {
		return LoginResult{}, err
	}
	if !ok {
		g.throttle.recordFailure(username)
		_ = g.store.RecordAudit(username, store.AuditLoginFailed, "")
		return LoginResult{Reason: "incorrect username or password"}, nil
	}
	g.throttle.reset(username)
	_ = g.store.TouchLastLogin(username)
	_ = g.store.RecordAudit(username, store.AuditLogin, fmt.Sprintf(`{"role":%q}`, user.Role))

	now := g.now()
	ceiling := now.Add(RefreshCeiling)
	access, err := g.signAccess(user.Username, user.Role, now)
	if err != nil {
		return LoginResult{}, err
	}
	refreshToken, err := g.mintRefresh(user.Username, ceiling)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{OK: true, Role: user.Role, AccessToken: access, RefreshToken: refreshToken, Ceiling: ceiling}, nil
}

// Refresh rotates a still-valid refresh token: the old one is deleted, a
// new opaque token is minted carrying the same ceiling forward (so a
// refresh can never push the session past 12h from its original login),
// and a fresh 15-minute access JWT is issued. ok is false for an unknown,
// already-used, or past-ceiling refresh token.
func (g *JWTGrant) Refresh(oldRefreshToken string) (result LoginResult, ok bool) {
	g.mu.Lock()
	rec, found := g.refresh[oldRefreshToken]
	if found {
		delete(g.refresh, oldRefreshToken)
	}
	g.mu.Unlock()
	if !found {
		return LoginResult{}, false
	}

	now := g.now()
	if now.After(rec.ceiling) {
		return LoginResult{}, false
	}

	user, exists, err := g.store.GetUserByUsername(rec.username)
	if err != nil || !exists || user.Disabled {
		return LoginResult{}, false
	}

	access, err := g.signAccess(user.Username, user.Role, now)
	if err != nil {
		return LoginResult{}, false
	}
	newRefreshToken, err := g.mintRefresh(rec.username, rec.ceiling)
	if err != nil {
		return LoginResult{}, false
	}
	return LoginResult{OK: true, Role: user.Role, AccessToken: access, RefreshToken: newRefreshToken, Ceiling: rec.ceiling}, true
}

// Revoke deletes refreshToken, so it can never be used again (SPEC.md §6.2:
// logout tears down the rotating refresh session).
func (g *JWTGrant) Revoke(refreshToken string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.refresh, refreshToken)
}

// AccessCookie builds the ditty_access cookie for the given signed JWT.
func (g *JWTGrant) AccessCookie(accessToken string) *http.Cookie {
	return &http.Cookie{
		Name:     AccessCookieName,
		Value:    accessToken,
		Path:     g.basePath,
		HttpOnly: true,
		Secure:   g.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(AccessTokenTTL.Seconds()),
	}
}

// RefreshCookie builds the ditty_refresh cookie for the given opaque token,
// its Max-Age tracking the absolute ceiling rather than always 12h.
func (g *JWTGrant) RefreshCookie(refreshToken string, ceiling time.Time) *http.Cookie {
	maxAge := int(time.Until(ceiling).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	return &http.Cookie{
		Name:     RefreshCookieName,
		Value:    refreshToken,
		Path:     g.basePath,
		HttpOnly: true,
		Secure:   g.secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   maxAge,
	}
}

// ClearCookies expires both JWT cookies (SPEC.md §4: /api/logout).
func (g *JWTGrant) ClearCookies(w http.ResponseWriter) {
	for _, name := range []string{AccessCookieName, RefreshCookieName} {
		http.SetCookie(w, &http.Cookie{
			Name: name, Value: "", Path: g.basePath, MaxAge: -1, HttpOnly: true, Secure: g.secure,
		})
	}
}

func (g *JWTGrant) signAccess(username, role string, now time.Time) (string, error) {
	claims := accessClaims{
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(g.secret)
}

func (g *JWTGrant) mintRefresh(username string, ceiling time.Time) (string, error) {
	b := make([]byte, refreshHandleLength)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("security: mint refresh token: %w", err)
	}
	token := hex.EncodeToString(b)
	g.mu.Lock()
	g.refresh[token] = refreshRecord{username: username, ceiling: ceiling}
	g.mu.Unlock()
	return token, nil
}
