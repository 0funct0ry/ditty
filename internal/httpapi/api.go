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
func sessionHandler(opts Options) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"id":       opts.SessionID,
			"name":     opts.SessionName,
			"title":    opts.SessionTitle,
			"server":   opts.Server,
			"state":    opts.Info.State(),
			"writable": opts.Writable,
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
// cookie, when a TokenGrant is configured. Other Grant types (basic, mTLS,
// trust-header) carry no server-side session to clear, so this is a no-op
// for them.
func logoutHandler(tokenGrant *security.TokenGrant) gin.HandlerFunc {
	return func(c *gin.Context) {
		if tokenGrant != nil {
			c.SetCookie(security.CookieName, "", -1, "", "", false, true)
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
