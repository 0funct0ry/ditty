package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
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

// logoutHandler answers POST /api/logout. There is no Grant/cookie
// machinery yet (M11), so this is a no-op that always succeeds.
func logoutHandler(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// tokenExchangeHandler answers GET /t/:token. Full token-to-cookie exchange
// is a Grant behaviour (SPEC.md §4.1) that arrives with internal/security
// in M11; until then the route exists (so its absence is never the reason
// a client fails) but is not implemented.
func tokenExchangeHandler(c *gin.Context) {
	c.Status(http.StatusNotImplemented)
}
