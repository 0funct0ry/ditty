package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/0funct0ry/ditty/internal/security"
)

// healthzHandler answers SPEC.md §4's /healthz: never Grant-gated, and
// reporting the Session's real lifecycle state alongside the fixed "ok"
// status.
func healthzHandler(info SessionInfo) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "state": info.State()})
	}
}

// sessionHandler answers GET /api/session with the Session's identity and
// current state (SPEC.md §4). Grants are M11; every Client sees the same
// view for now.
//
// authRequired reports whether a JWTGrant is configured for this server at
// all — distinct from whether *this* request already carries a valid
// session. Reaching this handler at all means a request was admitted (by
// some Grant), so authRequired here is never about *this* request's own
// auth state; it's the one signal the frontend needs to tell "no JWT login
// exists here" apart from "JWT login exists and this browser already has a
// valid cookie for it" — both cases return 200, and without this field
// they'd be indistinguishable, which is exactly what made the sign-out
// button vanish on a page refresh (SPEC.md §12 M12: checkAuthRequired
// couldn't tell the two apart from a bare 200).
func sessionHandler(opts Options) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"id":           opts.SessionID,
			"name":         opts.SessionName,
			"title":        opts.SessionTitle,
			"server":       opts.Server,
			"state":        opts.Info.State(),
			"writable":     opts.Writable,
			"authRequired": opts.JWTGrant != nil,
		})
	}
}

// profileHandler answers GET /api/profile with the server-seeded Profile
// (SPEC.md §7). An empty Profile is reported as an empty JSON object rather
// than null.
func profileHandler(profile []byte) gin.HandlerFunc {
	body := json.RawMessage(profile)
	if len(body) == 0 {
		body = json.RawMessage("{}")
	}
	return func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json", body)
	}
}

// logoutHandler answers POST /api/logout: it clears the SPEC.md §4.1 grant
// cookie when a TokenGrant is configured, and clears both JWT cookies plus
// revokes the refresh token when a JWTGrant is configured (SPEC.md §6.2, §9)
// — one endpoint tears down whichever Grant(s) produced the session. Other
// Grant types (basic, mTLS, trust-header) carry no server-side session to
// clear, so this is a no-op for them.
func logoutHandler(tokenGrant *security.TokenGrant, jwtGrant *security.JWTGrant) gin.HandlerFunc {
	return func(c *gin.Context) {
		if tokenGrant != nil {
			c.SetCookie(security.CookieName, "", -1, "", "", false, true)
		}
		if jwtGrant != nil {
			if cookie, err := c.Request.Cookie(security.RefreshCookieName); err == nil {
				jwtGrant.Revoke(cookie.Value)
			}
			jwtGrant.ClearCookies(c.Writer)
		}
		c.Status(http.StatusNoContent)
	}
}

// tokenExchangeHandler answers GET /t/:token (SPEC.md §4.1): a valid token
// is exchanged, once, for the opaque grant cookie, then redirects to the
// base path; anything else — no TokenGrant configured, or a bad token — is
// a 404, revealing nothing about which case applies.
func tokenExchangeHandler(tokenGrant *security.TokenGrant, redirectTo string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if tokenGrant == nil || !tokenGrant.Exchange(c.Writer, c.Param("token")) {
			c.Status(http.StatusNotFound)
			return
		}
		c.Redirect(http.StatusFound, redirectTo)
	}
}
