// Package httpapi is the app shell: the Gin router implementing every
// route in SPEC.md §4, security headers and CSP on every response, the
// ditty.v1 WebSocket upgrade in front of a Hub (internal/session's, since
// M10), and the embedded UI under --base-path.
package httpapi

import (
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
)

// SessionInfo is the read-only view of a Session's Hub that /api/session
// and /healthz report — narrower than httpapi.Hub, since it never needs to
// accept input.
type SessionInfo interface {
	State() string
}

// Options configures NewRouter.
type Options struct {
	// BasePath is the URL path prefix the UI, API and WebSocket are mounted
	// under (--base-path). "" is treated as "/".
	BasePath string
	// HubFactory produces the Hub each new WebSocket connection attaches
	// to, driving /ws. Nil disables /ws entirely.
	HubFactory HubFactory
	// Info reports the Session's lifecycle state for /api/session and
	// /healthz. Required.
	Info SessionInfo
	// PingInterval is the WebSocket keepalive interval (--ping-interval).
	// Zero uses DefaultPingInterval.
	PingInterval time.Duration
	// OriginAllow overrides the default same-host WebSocket Origin check
	// (--origin-allow). Nil keeps the same-host default.
	OriginAllow *regexp.Regexp
	// AllowIframe sets the CSP frame-ancestors source (--allow-iframe).
	// Empty means frame-ancestors 'none'. The literal value "*" omits the
	// frame-ancestors directive entirely (allows any framer).
	AllowIframe string

	// SessionName, SessionTitle, SessionID and Server feed /api/session's
	// JSON.
	SessionID, SessionName, SessionTitle, Server string
	// Writable is the Session-wide write capability (--writable), reported
	// on /api/session.
	Writable bool
	// Profile seeds /api/profile's JSON body (SPEC.md §7).
	Profile []byte
}

// NewRouter builds the full SPEC.md §4 route table under opts.BasePath.
func NewRouter(opts Options) (http.Handler, error) {
	basePath := normalizeBasePath(opts.BasePath)

	index, err := renderIndex(basePath)
	if err != nil {
		return nil, err
	}

	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(securityHeaders(opts.AllowIframe))

	group := engine.Group(basePath)
	group.GET("/healthz", healthzHandler(opts.Info))
	group.GET("/", indexHandler(index))
	group.GET("/favicon.ico", faviconHandler)
	group.GET("/assets/*filepath", assetsHandler(basePath))
	group.GET("/t/:token", tokenExchangeHandler)
	group.GET("/api/session", sessionHandler(opts))
	group.GET("/api/profile", profileHandler(opts.Profile))
	group.POST("/api/logout", logoutHandler)

	if opts.HubFactory != nil {
		wsHandler := NewWSHandler(opts.HubFactory, WSOptions{
			OriginAllow:  opts.OriginAllow,
			PingInterval: opts.PingInterval,
		})
		group.GET("/ws", gin.WrapH(wsHandler))
	}

	return engine, nil
}

// securityHeaders sets SPEC.md §4's headers on every response: nosniff,
// no-referrer, and a CSP whose frame-ancestors defaults to 'none'
// (--allow-iframe), unless allowIframe is the sentinel "*" (bare
// --allow-iframe, no origin given), in which case frame-ancestors is
// omitted entirely.
func securityHeaders(allowIframe string) gin.HandlerFunc {
	frameAncestors := "frame-ancestors 'none'; "
	switch allowIframe {
	case "":
		// default, set above
	case "*":
		frameAncestors = ""
	default:
		frameAncestors = "frame-ancestors " + allowIframe + "; "
	}
	csp := frameAncestors + "default-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self' ws: wss:"

	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Content-Security-Policy", csp)
		c.Next()
	}
}

// normalizeBasePath ensures basePath starts with "/" and has no trailing
// slash, so it can be used directly as a Gin route group prefix ("" means
// root, matching Gin's own convention).
func normalizeBasePath(basePath string) string {
	if basePath == "" || basePath == "/" {
		return ""
	}
	if basePath[0] != '/' {
		basePath = "/" + basePath
	}
	if basePath[len(basePath)-1] == '/' {
		basePath = basePath[:len(basePath)-1]
	}
	return basePath
}
