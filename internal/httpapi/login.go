package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/0funct0ry/ditty/internal/security"
)

// loginRequest is /api/login's JSON body.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// loginResponse matches M7's AuthClient LoginResult exactly
// (web/src/protocol/auth.ts): {ok:true, role} or {ok:false, reason}.
type loginResponse struct {
	OK     bool   `json:"ok"`
	Role   string `json:"role,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// loginHandler answers POST /api/login (SPEC.md §6.2, §9): on success it
// sets the ditty_access and ditty_refresh cookies and reports the user's
// role; on failure (wrong credentials, disabled account, or a currently
// throttled username) it reports a reason, all indistinguishable to the
// caller.
func loginHandler(jg *security.JWTGrant) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req loginRequest
		if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" || req.Password == "" {
			c.JSON(http.StatusBadRequest, loginResponse{Reason: "username and password are required"})
			return
		}
		result, err := jg.Login(req.Username, req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, loginResponse{Reason: "internal error"})
			return
		}
		if !result.OK {
			c.JSON(http.StatusUnauthorized, loginResponse{Reason: result.Reason})
			return
		}
		http.SetCookie(c.Writer, jg.AccessCookie(result.AccessToken))
		http.SetCookie(c.Writer, jg.RefreshCookie(result.RefreshToken, result.Ceiling))
		c.JSON(http.StatusOK, loginResponse{OK: true, Role: result.Role})
	}
}

// refreshHandler answers POST /api/refresh: a valid, not-yet-expired
// ditty_refresh cookie is rotated for a new one and mints a fresh 15-minute
// access JWT (SPEC.md §6.2's "rotating refresh cookie").
func refreshHandler(jg *security.JWTGrant) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Request.Cookie(security.RefreshCookieName)
		if err != nil {
			c.Status(http.StatusUnauthorized)
			return
		}
		result, ok := jg.Refresh(cookie.Value)
		if !ok {
			c.Status(http.StatusUnauthorized)
			return
		}
		http.SetCookie(c.Writer, jg.AccessCookie(result.AccessToken))
		http.SetCookie(c.Writer, jg.RefreshCookie(result.RefreshToken, result.Ceiling))
		c.JSON(http.StatusOK, loginResponse{OK: true, Role: result.Role})
	}
}
