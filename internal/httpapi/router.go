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

	"github.com/0funct0ry/ditty/internal/security"
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

	// Grants gates every route except /healthz and /t/:token (SPEC.md §6.2).
	// Empty means no Grant is configured (only reachable when the bind
	// guard has separately permitted no-auth), so every request is
	// admitted.
	Grants security.Grants
	// TokenGrant, when non-nil, backs /t/:token's path-to-cookie exchange
	// (SPEC.md §4.1) and /api/logout's cookie clearing.
	TokenGrant *security.TokenGrant
	// JWTGrant, when non-nil (--auth-db configured), backs /api/login and
	// /api/refresh (SPEC.md §6.2, §9) and is included in /api/logout's
	// teardown. Nil means --auth-db was not passed, so these routes are
	// never registered at all.
	JWTGrant *security.JWTGrant
	// HeaderEnv is the set of --header-env mappings (SPEC.md §6.4) resolved
	// against each WebSocket upgrade's request headers.
	HeaderEnv []security.HeaderEnvMapping
	// OnAttach and OnDetach, when non-nil, audit every WebSocket
	// attach/detach (SPEC.md §9). See WSOptions' matching fields.
	OnAttach func(identity security.Identity)
	OnDetach func(identity security.Identity)
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
	// /healthz and /t/:token are never Grant-gated: healthz is an
	// unauthenticated liveness probe, and the token exchange is itself how
	// a Client obtains a Grant in the first place.
	group.GET("/healthz", healthzHandler(opts.Info))
	rootPath := basePath
	if rootPath == "" {
		rootPath = "/"
	}
	group.GET("/t/:token", tokenExchangeHandler(opts.TokenGrant, rootPath))

	// The page itself (the embedded SPA and its assets) is gated only by
	// Grants that authenticate at the network/protocol layer before any
	// HTML is ever served — Token (via /t/:token), Basic (the browser's own
	// credential prompt), mTLS, TrustedHeader. JWTGrant is deliberately
	// excluded here: a username/password login has no such pre-HTML step,
	// so the SPA must be able to load unauthenticated in order to render
	// its own login screen and call /api/login. Gating "/" behind JWTGrant
	// too would make that screen unreachable — the exact bug this avoids.
	page := group.Group("")
	page.Use(grantMiddleware(nonJWTGrants(opts.Grants, opts.JWTGrant)))
	page.GET("/", indexHandler(index))
	page.GET("/favicon.ico", faviconHandler)
	page.GET("/assets/*filepath", assetsHandler(basePath))

	// /api/*, /ws stay gated by every configured Grant, JWT included: this
	// is the layer the SPA actually probes (a 401 from /api/session is how
	// it decides to show the login screen at all) and the layer that must
	// never be reachable without a real Grant.
	api := group.Group("")
	api.Use(grantMiddleware(opts.Grants))
	api.GET("/api/session", sessionHandler(opts))
	api.GET("/api/profile", profileHandler(opts.Profile))
	api.POST("/api/logout", logoutHandler(opts.TokenGrant, opts.JWTGrant))

	// /api/login and /api/refresh are unauthenticated by design (a request
	// carries no Grant yet when it logs in) and only exist at all when
	// --auth-db was passed — no route surface appears otherwise.
	if opts.JWTGrant != nil {
		group.POST("/api/login", loginHandler(opts.JWTGrant))
		group.POST("/api/refresh", refreshHandler(opts.JWTGrant))
	}

	if opts.HubFactory != nil {
		wsHandler := NewWSHandler(opts.HubFactory, WSOptions{
			OriginAllow:  opts.OriginAllow,
			PingInterval: opts.PingInterval,
			Grants:       opts.Grants,
			HeaderEnv:    opts.HeaderEnv,
			OnAttach:     opts.OnAttach,
			OnDetach:     opts.OnDetach,
		})
		api.GET("/ws", gin.WrapH(wsHandler))
	}

	return engine, nil
}

// nonJWTGrants returns grants with jwtGrant removed (a no-op copy when
// jwtGrant is nil or absent from grants) — see the page-vs-api route
// split in NewRouter for why the page-serving routes must never be gated
// by the JWT Grant specifically.
func nonJWTGrants(grants security.Grants, jwtGrant *security.JWTGrant) security.Grants {
	if jwtGrant == nil {
		return grants
	}
	out := make(security.Grants, 0, len(grants))
	for _, g := range grants {
		if g == security.Grant(jwtGrant) {
			continue
		}
		out = append(out, g)
	}
	return out
}

// grantMiddleware admits every request when no Grant is configured (the
// bind guard is what keeps that safe — SPEC.md §6.1 I2), and otherwise
// requires one of opts.Grants to authenticate the request.
func grantMiddleware(grants security.Grants) gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(grants) == 0 {
			c.Next()
			return
		}
		if _, ok := grants.Authenticate(c.Request); !ok {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Next()
	}
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
