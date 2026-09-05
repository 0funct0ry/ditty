package security

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/0funct0ry/ditty/internal/store"
)

func newTestJWTGrant(t *testing.T) (*JWTGrant, *int64) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if _, err := st.CreateUser("alice", "hunter22ok", store.RoleOperator); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	g := NewJWTGrant([]byte("test-secret"), st, "/", false)
	var seconds int64
	g.now = fixedClock(&seconds)
	return g, &seconds
}

func requestWithCookies(cookies ...*http.Cookie) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	return r
}

func TestJWTGrant_LoginThenAuthenticate(t *testing.T) {
	g, _ := newTestJWTGrant(t)

	result, err := g.Login("alice", "hunter22ok")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if !result.OK || result.Role != store.RoleOperator {
		t.Fatalf("Login result = %+v, want OK with role operator", result)
	}

	id, ok := g.Authenticate(requestWithCookies(g.AccessCookie(result.AccessToken)))
	if !ok {
		t.Fatal("Authenticate should admit a request carrying a fresh access cookie")
	}
	if id.Role != store.RoleOperator || id.Label != "alice" {
		t.Fatalf("Identity = %+v", id)
	}
}

func TestJWTGrant_LoginWrongPassword(t *testing.T) {
	g, _ := newTestJWTGrant(t)
	result, err := g.Login("alice", "wrong")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if result.OK || result.Reason == "" {
		t.Fatalf("expected a failure with a reason, got %+v", result)
	}
}

func TestJWTGrant_LoginLocksAfterFailures(t *testing.T) {
	g, _ := newTestJWTGrant(t)
	for i := 0; i < loginThrottleMax; i++ {
		if _, err := g.Login("alice", "wrong"); err != nil {
			t.Fatalf("Login: %v", err)
		}
	}
	result, err := g.Login("alice", "hunter22ok") // correct password, but locked
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if result.OK {
		t.Fatal("expected the correct password to still be rejected while locked")
	}
}

func TestJWTGrant_AccessTokenExpires(t *testing.T) {
	g, seconds := newTestJWTGrant(t)
	result, err := g.Login("alice", "hunter22ok")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	cookie := g.AccessCookie(result.AccessToken)

	*seconds += int64(AccessTokenTTL.Seconds()) + 1
	if _, ok := g.Authenticate(requestWithCookies(cookie)); ok {
		t.Fatal("expected an expired access token to be rejected")
	}
}

func TestJWTGrant_RefreshRotatesAndReAuthenticates(t *testing.T) {
	g, seconds := newTestJWTGrant(t)
	login, err := g.Login("alice", "hunter22ok")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	*seconds += int64(AccessTokenTTL.Seconds()) + 1 // access expired, refresh still valid
	refreshed, ok := g.Refresh(login.RefreshToken)
	if !ok || !refreshed.OK {
		t.Fatalf("Refresh: ok=%v result=%+v", ok, refreshed)
	}
	if refreshed.RefreshToken == login.RefreshToken {
		t.Fatal("Refresh should rotate to a new refresh token, not reuse the old one")
	}

	if _, ok := g.Authenticate(requestWithCookies(g.AccessCookie(refreshed.AccessToken))); !ok {
		t.Fatal("the refreshed access token should authenticate")
	}
	if _, ok := g.Refresh(login.RefreshToken); ok {
		t.Fatal("the old refresh token must not be usable a second time")
	}
}

func TestJWTGrant_RefreshPastCeilingFails(t *testing.T) {
	g, seconds := newTestJWTGrant(t)
	login, err := g.Login("alice", "hunter22ok")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	*seconds += int64(RefreshCeiling.Seconds()) + 1
	if _, ok := g.Refresh(login.RefreshToken); ok {
		t.Fatal("a refresh token past its 12h ceiling must be rejected")
	}
}

func TestJWTGrant_RevokeInvalidatesRefresh(t *testing.T) {
	g, _ := newTestJWTGrant(t)
	login, err := g.Login("alice", "hunter22ok")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	g.Revoke(login.RefreshToken)
	if _, ok := g.Refresh(login.RefreshToken); ok {
		t.Fatal("a revoked refresh token must not work")
	}
}

func TestJWTGrant_Authenticate_NoCookie(t *testing.T) {
	g, _ := newTestJWTGrant(t)
	if _, ok := g.Authenticate(httptest.NewRequest(http.MethodGet, "/", nil)); ok {
		t.Fatal("a request without an access cookie must not authenticate")
	}
}

func TestJWTGrant_Authenticate_WrongSecretRejected(t *testing.T) {
	g, _ := newTestJWTGrant(t)
	result, err := g.Login("alice", "hunter22ok")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	other := NewJWTGrant([]byte("different-secret"), nil, "/", false)
	if _, ok := other.Authenticate(requestWithCookies(g.AccessCookie(result.AccessToken))); ok {
		t.Fatal("a token signed with a different secret must not authenticate")
	}
}
