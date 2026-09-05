package httpapi

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/0funct0ry/ditty/internal/security"
	"github.com/0funct0ry/ditty/web"
)

// realAssetName returns the filename of some real, embedded, hashed asset
// under web/dist/assets — discovered at test time rather than hardcoded, so
// this test survives web/dist being rebuilt (Vite's content hash in every
// filename changes on every build).
func realAssetName(t *testing.T) string {
	t.Helper()
	entries, err := fs.ReadDir(web.DistFS, "dist/assets")
	if err != nil || len(entries) == 0 {
		t.Fatalf("read dist/assets: %v (entries=%d)", err, len(entries))
	}
	return entries[0].Name()
}

// fakeSessionInfo is a minimal SessionInfo double for router-level tests
// that don't need a real Hub.
type fakeSessionInfo struct{ state string }

func (f fakeSessionInfo) State() string { return f.state }

func newTestRouter(t *testing.T, basePath string) *httptest.Server {
	t.Helper()
	handler, err := NewRouter(Options{
		BasePath:     basePath,
		Info:         fakeSessionInfo{state: "live"},
		SessionID:    "sess1",
		SessionName:  "deploy",
		SessionTitle: "deploy — host",
		Server:       "ditty-test",
		Writable:     true,
		Profile:      []byte(`{"theme":"ditty-dark"}`),
	})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	return httptest.NewServer(handler)
}

func TestRouter_EveryRoute(t *testing.T) {
	for _, basePath := range []string{"/", "/term"} {
		t.Run("basePath="+basePath, func(t *testing.T) {
			server := newTestRouter(t, basePath)
			defer server.Close()

			prefix := strings.TrimSuffix(basePath, "/")

			t.Run("index", func(t *testing.T) {
				resp := get(t, server.URL+prefix+"/")
				defer func() { _ = resp.Body.Close() }()
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("status = %d, want 200", resp.StatusCode)
				}
				assertSecurityHeaders(t, resp)
				if basePath == "/term" {
					body := readBody(t, resp)
					if !strings.Contains(body, `src="/term/assets/`) {
						t.Fatalf("index.html was not rewritten for base path: %s", body)
					}
				}
			})

			t.Run("healthz", func(t *testing.T) {
				resp := get(t, server.URL+prefix+"/healthz")
				defer func() { _ = resp.Body.Close() }()
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("status = %d, want 200", resp.StatusCode)
				}
				var body map[string]string
				if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
					t.Fatalf("decode: %v", err)
				}
				if body["status"] != "ok" || body["state"] != "live" {
					t.Fatalf("body = %+v, want status=ok state=live", body)
				}
			})

			t.Run("api session", func(t *testing.T) {
				resp := get(t, server.URL+prefix+"/api/session")
				defer func() { _ = resp.Body.Close() }()
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("status = %d, want 200", resp.StatusCode)
				}
				var body map[string]any
				if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
					t.Fatalf("decode: %v", err)
				}
				if body["name"] != "deploy" || body["state"] != "live" {
					t.Fatalf("body = %+v", body)
				}
			})

			t.Run("api profile", func(t *testing.T) {
				resp := get(t, server.URL+prefix+"/api/profile")
				defer func() { _ = resp.Body.Close() }()
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("status = %d, want 200", resp.StatusCode)
				}
				body := readBody(t, resp)
				if !strings.Contains(body, "ditty-dark") {
					t.Fatalf("body = %s, want the seeded theme", body)
				}
			})

			t.Run("api logout", func(t *testing.T) {
				resp, err := http.Post(server.URL+prefix+"/api/logout", "", nil)
				if err != nil {
					t.Fatalf("POST: %v", err)
				}
				defer func() { _ = resp.Body.Close() }()
				if resp.StatusCode != http.StatusNoContent {
					t.Fatalf("status = %d, want 204", resp.StatusCode)
				}
			})

			t.Run("token exchange with no TokenGrant configured", func(t *testing.T) {
				resp := get(t, server.URL+prefix+"/t/sometoken")
				defer func() { _ = resp.Body.Close() }()
				if resp.StatusCode != http.StatusNotFound {
					t.Fatalf("status = %d, want 404", resp.StatusCode)
				}
			})

			t.Run("favicon missing", func(t *testing.T) {
				resp := get(t, server.URL+prefix+"/favicon.ico")
				defer func() { _ = resp.Body.Close() }()
				if resp.StatusCode != http.StatusNotFound {
					t.Fatalf("status = %d, want 404 (web/dist ships no favicon)", resp.StatusCode)
				}
			})

			t.Run("assets cache headers", func(t *testing.T) {
				resp := get(t, server.URL+prefix+"/assets/"+realAssetName(t))
				defer func() { _ = resp.Body.Close() }()
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("status = %d, want 200 for a real embedded asset", resp.StatusCode)
				}
				if got := resp.Header.Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
					t.Fatalf("Cache-Control = %q", got)
				}
			})

			t.Run("assets 404 for unknown file", func(t *testing.T) {
				resp := get(t, server.URL+prefix+"/assets/does-not-exist.js")
				defer func() { _ = resp.Body.Close() }()
				if resp.StatusCode != http.StatusNotFound {
					t.Fatalf("status = %d, want 404 for an unknown asset", resp.StatusCode)
				}
			})
		})
	}
}

func TestRouter_FrameAncestors(t *testing.T) {
	cases := []struct {
		name        string
		allowIframe string
		want        string
	}{
		{"default", "", "frame-ancestors 'none'"},
		{"specific origin", "https://example.com", "frame-ancestors https://example.com"},
		{"bare flag omits directive", "*", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, err := NewRouter(Options{
				Info:        fakeSessionInfo{state: "live"},
				AllowIframe: tc.allowIframe,
			})
			if err != nil {
				t.Fatalf("NewRouter: %v", err)
			}
			server := httptest.NewServer(handler)
			defer server.Close()

			resp := get(t, server.URL+"/healthz")
			defer func() { _ = resp.Body.Close() }()
			csp := resp.Header.Get("Content-Security-Policy")
			if tc.want == "" {
				if strings.Contains(csp, "frame-ancestors") {
					t.Fatalf("CSP = %q, want no frame-ancestors directive", csp)
				}
				return
			}
			if !strings.Contains(csp, tc.want) {
				t.Fatalf("CSP = %q, want it to contain %q", csp, tc.want)
			}
		})
	}
}

func TestRouter_GrantGatesProtectedRoutes(t *testing.T) {
	token, err := security.GenerateToken(security.DefaultTokenLength)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	tokenGrant := security.NewTokenGrant(token, "/", false, time.Hour)

	handler, err := NewRouter(Options{
		Info:       fakeSessionInfo{state: "live"},
		Grants:     security.Grants{tokenGrant},
		TokenGrant: tokenGrant,
	})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	// healthz stays open even with a Grant configured.
	resp := get(t, server.URL+"/healthz")
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", resp.StatusCode)
	}

	// A protected route with no cookie is rejected.
	unauth := get(t, server.URL+"/api/session")
	defer func() { _ = unauth.Body.Close() }()
	if unauth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("api/session with no Grant status = %d, want 401", unauth.StatusCode)
	}

	// The bad-token exchange 404s and sets no cookie.
	badExchange := get(t, server.URL+"/t/not-the-token")
	defer func() { _ = badExchange.Body.Close() }()
	if badExchange.StatusCode != http.StatusNotFound {
		t.Fatalf("bad token exchange status = %d, want 404", badExchange.StatusCode)
	}

	// The real token exchange sets the cookie and redirects, and the
	// cookie then admits the protected route.
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	exchangeResp, err := client.Get(server.URL + "/t/" + token)
	if err != nil {
		t.Fatalf("GET token exchange: %v", err)
	}
	defer func() { _ = exchangeResp.Body.Close() }()
	if exchangeResp.StatusCode != http.StatusFound {
		t.Fatalf("token exchange status = %d, want 302", exchangeResp.StatusCode)
	}
	cookies := exchangeResp.Cookies()
	if len(cookies) != 1 || cookies[0].Name != security.CookieName {
		t.Fatalf("token exchange cookies = %+v", cookies)
	}

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/session", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.AddCookie(cookies[0])
	authed, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET with cookie: %v", err)
	}
	defer func() { _ = authed.Body.Close() }()
	if authed.StatusCode != http.StatusOK {
		t.Fatalf("api/session with valid cookie status = %d, want 200", authed.StatusCode)
	}
}

func get(t *testing.T, url string) *http.Response {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		sb.Write(buf[:n])
		if err != nil {
			break
		}
	}
	return sb.String()
}

func assertSecurityHeaders(t *testing.T, resp *http.Response) {
	t.Helper()
	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := resp.Header.Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("Referrer-Policy = %q, want no-referrer", got)
	}
	if got := resp.Header.Get("Content-Security-Policy"); got == "" {
		t.Fatal("Content-Security-Policy header missing")
	}
}
