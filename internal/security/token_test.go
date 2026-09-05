package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTokenGrant_ExchangeAndAuthenticate(t *testing.T) {
	token, err := GenerateToken(DefaultTokenLength)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	grant := NewTokenGrant(token, "/", false, 24*time.Hour)

	// Bad token never sets a cookie.
	badRec := httptest.NewRecorder()
	if grant.Exchange(badRec, "not-the-token") {
		t.Fatal("Exchange(bad token) = true, want false")
	}
	if len(badRec.Result().Cookies()) != 0 {
		t.Fatal("Exchange(bad token) set a cookie")
	}

	// Good token sets the cookie.
	goodRec := httptest.NewRecorder()
	if !grant.Exchange(goodRec, token) {
		t.Fatal("Exchange(good token) = false, want true")
	}
	cookies := goodRec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != CookieName {
		t.Fatalf("Exchange(good token) cookies = %+v, want one %s cookie", cookies, CookieName)
	}
	handle := cookies[0].Value

	// The cookie alone then authenticates.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: handle})
	if _, ok := grant.Authenticate(req); !ok {
		t.Fatal("Authenticate(valid cookie) = false, want true")
	}

	// An unrelated cookie value never authenticates.
	reqBad := httptest.NewRequest(http.MethodGet, "/", nil)
	reqBad.AddCookie(&http.Cookie{Name: CookieName, Value: "forged"})
	if _, ok := grant.Authenticate(reqBad); ok {
		t.Fatal("Authenticate(forged cookie) = true, want false")
	}

	// A no-cookie request never authenticates.
	reqNone := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, ok := grant.Authenticate(reqNone); ok {
		t.Fatal("Authenticate(no cookie) = true, want false")
	}
}

func TestTokenGrant_RestartInvalidatesCookies(t *testing.T) {
	token, _ := GenerateToken(DefaultTokenLength)
	grant := NewTokenGrant(token, "/", false, time.Hour)

	rec := httptest.NewRecorder()
	grant.Exchange(rec, token)
	handle := rec.Result().Cookies()[0].Value

	// A "restart" is a fresh TokenGrant with an empty handle map.
	restarted := NewTokenGrant(token, "/", false, time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: handle})
	if _, ok := restarted.Authenticate(req); ok {
		t.Fatal("Authenticate on a restarted TokenGrant admitted an old handle")
	}
}

func TestGenerateToken_MinLength(t *testing.T) {
	if _, err := GenerateToken(MinTokenLength - 1); err == nil {
		t.Fatal("GenerateToken(below minimum) = nil error, want error")
	}
	if _, err := GenerateToken(MinTokenLength); err != nil {
		t.Fatalf("GenerateToken(minimum) = %v, want nil", err)
	}
}
