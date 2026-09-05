package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/0funct0ry/ditty/internal/security"
	"github.com/0funct0ry/ditty/internal/session"
	"github.com/0funct0ry/ditty/internal/store"
	"github.com/0funct0ry/ditty/internal/wire"
)

// writeRecordingProcess is a session.Process double that records every byte
// written to it (i.e. every byte the Hub forwarded from a writable Client),
// so a test can assert nothing at all reached "the PTY" for a denied write.
type writeRecordingProcess struct {
	*scriptedProcess
	writes [][]byte
}

func newWriteRecordingProcess() *writeRecordingProcess {
	return &writeRecordingProcess{scriptedProcess: newScriptedProcess()}
}

func (p *writeRecordingProcess) Write(b []byte) (int, error) {
	cp := make([]byte, len(b))
	copy(cp, b)
	p.writes = append(p.writes, cp)
	return len(b), nil
}

// newAuthTestServer wires a real Store, a real JWTGrant, and a real
// internal/session Hub behind a real HTTP server — role-based write
// capability is computed exactly as cmd/run.go computes it: writable &&
// security.RoleAllowsWrite(identity.Role).
func newAuthTestServer(t *testing.T, role string) (*httptest.Server, *security.JWTGrant, *store.Store, *writeRecordingProcess) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if _, err := st.CreateUser("alice", "hunter22ok", role); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	jg := security.NewJWTGrant([]byte("test-secret"), st, "/", false)
	proc := newWriteRecordingProcess()
	hub := session.NewHub(session.Options{
		Process:       proc,
		Name:          "authflow",
		Server:        "ditty-test",
		FlushInterval: time.Millisecond,
		OnWriteDenied: func(_, label string) { _ = st.RecordAudit(label, store.AuditWriteDenied, "") },
	})

	hubFactory := func(_ []string, identity security.Identity) (Hub, error) {
		return NewSessionHub(hub, security.RoleAllowsWrite(identity.Role)), nil
	}

	handler, err := NewRouter(Options{
		HubFactory: hubFactory,
		Info:       hub,
		Grants:     security.Grants{jg},
		JWTGrant:   jg,
		Writable:   true,
	})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server, jg, st, proc
}

func newCookieClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	return &http.Client{Jar: jar}
}

// doJSON POSTs body (JSON-encoded, or no body when nil) to url and returns
// the response's status code and decoded JSON body, closing the response
// body itself so call sites never need to.
func doJSON(t *testing.T, client *http.Client, url string, body any) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(http.MethodPost, url, reader)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

// TestAuthFlow_FullLifecycle exercises add -> login -> cookie -> attach ->
// refresh after expiry -> logout -> attach rejected (M12's acceptance
// criterion), against a real JWTGrant and Store, over a real HTTP server.
func TestAuthFlow_FullLifecycle(t *testing.T) {
	server, jg, _, _ := newAuthTestServer(t, store.RoleOperator)
	client := newCookieClient(t)

	status, body := doJSON(t, client, server.URL+"/api/login",
		map[string]string{"username": "alice", "password": "hunter22ok"})
	if status != http.StatusOK || body["ok"] != true || body["role"] != store.RoleOperator {
		t.Fatalf("login: status=%d body=%v", status, body)
	}

	resp, err := client.Get(server.URL + "/api/session")
	if err != nil {
		t.Fatalf("api/session: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("api/session after login: status=%d", resp.StatusCode)
	}

	// Force the access token to expire without touching the refresh token.
	var elapsed time.Duration
	base := time.Now()
	jg.SetNow(func() time.Time { return base.Add(elapsed) })
	elapsed = security.AccessTokenTTL + time.Second

	resp, err = client.Get(server.URL + "/api/session")
	if err != nil {
		t.Fatalf("api/session: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("api/session with expired access token: status=%d, want 401", resp.StatusCode)
	}

	status, body = doJSON(t, client, server.URL+"/api/refresh", nil)
	if status != http.StatusOK || body["ok"] != true {
		t.Fatalf("refresh: status=%d body=%v", status, body)
	}

	resp, err = client.Get(server.URL + "/api/session")
	if err != nil {
		t.Fatalf("api/session: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("api/session after refresh: status=%d, want 200", resp.StatusCode)
	}

	resp, err = client.Post(server.URL+"/api/logout", "application/json", nil)
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	_ = resp.Body.Close()

	resp, err = client.Get(server.URL + "/api/session")
	if err != nil {
		t.Fatalf("api/session: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("api/session after logout: status=%d, want 401", resp.StatusCode)
	}
}

// TestAuthFlow_IndexLoadsWithoutJWTCookie is a regression test: the page
// route ("/") must be reachable with no Grant at all when JWTGrant is the
// only configured Grant, so the embedded SPA can load and drive its own
// login screen against /api/login. Gating "/" behind the JWT Grant (as
// every other Grant type correctly does) makes the login screen
// unreachable — a browser hitting the bare URL gets a 401 instead of HTML.
func TestAuthFlow_IndexLoadsWithoutJWTCookie(t *testing.T) {
	server, _, _, _ := newAuthTestServer(t, store.RoleOperator)

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / with no cookie: status=%d, want 200 (the SPA must load unauthenticated)", resp.StatusCode)
	}

	resp, err = http.Get(server.URL + "/api/session")
	if err != nil {
		t.Fatalf("GET /api/session: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /api/session with no cookie: status=%d, want 401", resp.StatusCode)
	}
}

// TestAuthFlow_SessionReportsAuthRequired is a regression test: /api/session
// must let the frontend distinguish "no JWT configured" from "JWT
// configured and this browser already has a valid cookie" — both answer
// with a 200, and without the authRequired field the frontend can't tell
// them apart (the bug that made the sign-out button vanish on a page
// refresh after a successful login).
func TestAuthFlow_SessionReportsAuthRequired(t *testing.T) {
	server, _, _, _ := newAuthTestServer(t, store.RoleOperator)
	client := newCookieClient(t)

	_, body := doJSON(t, client, server.URL+"/api/login",
		map[string]string{"username": "alice", "password": "hunter22ok"})
	if body["ok"] != true {
		t.Fatalf("login failed: %v", body)
	}

	resp, err := client.Get(server.URL + "/api/session")
	if err != nil {
		t.Fatalf("GET /api/session: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/session with a valid cookie: status=%d, want 200", resp.StatusCode)
	}
	var sessionBody map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&sessionBody); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if sessionBody["authRequired"] != true {
		t.Fatalf(
			"authRequired = %v, want true (a page refresh with a valid cookie must still report a JWTGrant is configured, not just a bare 200)",
			sessionBody["authRequired"],
		)
	}
}

func TestAuthFlow_LoginRejectsWrongPassword(t *testing.T) {
	server, _, _, _ := newAuthTestServer(t, store.RoleViewer)
	client := newCookieClient(t)
	status, body := doJSON(t, client, server.URL+"/api/login",
		map[string]string{"username": "alice", "password": "wrong"})
	if status != http.StatusUnauthorized || body["ok"] == true || body["reason"] == "" {
		t.Fatalf("status=%d body=%v", status, body)
	}
}

// TestAuthFlow_LoginResponseShape asserts /api/login's JSON body matches
// M7's AuthClient LoginResult exactly (web/src/protocol/auth.ts):
// {ok:true, role} or {ok:false, reason}.
func TestAuthFlow_LoginResponseShape(t *testing.T) {
	server, _, _, _ := newAuthTestServer(t, store.RoleOperator)

	_, ok := doJSON(t, newCookieClient(t), server.URL+"/api/login",
		map[string]string{"username": "alice", "password": "hunter22ok"})
	if _, present := ok["reason"]; present {
		t.Fatalf("a successful login must not carry a reason field: %v", ok)
	}
	if ok["role"] != store.RoleOperator {
		t.Fatalf("role = %v", ok["role"])
	}

	_, fail := doJSON(t, newCookieClient(t), server.URL+"/api/login",
		map[string]string{"username": "alice", "password": "wrong"})
	if _, present := fail["role"]; present {
		t.Fatalf("a failed login must not carry a role field: %v", fail)
	}
	if _, present := fail["reason"]; !present {
		t.Fatalf("a failed login must carry a reason field: %v", fail)
	}
}

// TestAuthFlow_ViewerCannotWriteEvenWithWritable is M12's acceptance
// criterion "viewer cannot write with -w, asserted at the fake PTY": the
// Session is --writable, but a viewer Identity's WS attach must still never
// forward Input to the process, and a write_denied audit row must appear.
func TestAuthFlow_ViewerCannotWriteEvenWithWritable(t *testing.T) {
	server, _, st, proc := newAuthTestServer(t, store.RoleViewer)
	client := newCookieClient(t)

	_, body := doJSON(t, client, server.URL+"/api/login",
		map[string]string{"username": "alice", "password": "hunter22ok"})
	if body["ok"] != true {
		t.Fatalf("login failed: %v", body)
	}

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	header := http.Header{}
	for _, c := range client.Jar.Cookies(serverURL) {
		header.Add("Cookie", c.Name+"="+c.Value)
	}

	wsAddr := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	dialer := websocket.Dialer{Subprotocols: []string{wire.Subprotocol}}
	conn, dialResp, err := dialer.Dial(wsAddr, header)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = dialResp.Body.Close() }()
	defer func() { _ = conn.Close() }()

	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf("Hello: %v", err)
	}

	input, err := wire.Encode(wire.OpInput, []byte("echo denied\n"))
	if err != nil {
		t.Fatalf("encode Input: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, input); err != nil {
		t.Fatalf("write Input: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, frame, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if op, _, _ := wire.Decode(frame); op == wire.OpNotice {
			break
		}
	}

	if len(proc.writes) != 0 {
		t.Fatalf("expected no bytes forwarded to the process, got %v", proc.writes)
	}

	n, err := st.CountAuditAction("", store.AuditWriteDenied)
	if err != nil {
		t.Fatalf("CountAuditAction: %v", err)
	}
	if n == 0 {
		t.Fatal("expected a write_denied audit row")
	}
}

// TestAuthFlow_AuditNeverContainsSecrets extends the existing log-redaction
// test pattern (internal/security/redact_test.go) to the audit table: a
// full login (one failure, one success) must never leave the plaintext
// password anywhere in audit.detail.
func TestAuthFlow_AuditNeverContainsSecrets(t *testing.T) {
	server, _, st, _ := newAuthTestServer(t, store.RoleOperator)
	client := newCookieClient(t)

	const password = "hunter22ok"
	doJSON(t, client, server.URL+"/api/login",
		map[string]string{"username": "alice", "password": "wrong-password-x"})
	doJSON(t, client, server.URL+"/api/login",
		map[string]string{"username": "alice", "password": password})

	details, err := st.AllAuditDetails()
	if err != nil {
		t.Fatalf("AllAuditDetails: %v", err)
	}
	for _, d := range details {
		if strings.Contains(d, password) || strings.Contains(d, "wrong-password-x") {
			t.Fatalf("audit detail leaked a password: %q", d)
		}
	}
}
