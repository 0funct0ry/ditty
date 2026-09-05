// Package httpapi is the app shell: it answers /healthz, serves the
// embedded UI under --base-path, and (since M3) can mount a minimal /ws
// upgrade handler in front of a Hub, such as internal/session's (M9). The
// full route table from SPEC.md §4 — security headers, CSP, /t/:token,
// /api/* — and the switch to Gin both arrive in M10; this is deliberately
// just enough transport for a browser to reach a Hub (SPEC.md §12 M1, M3).
package httpapi

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"

	"github.com/0funct0ry/ditty/web"
)

// NewHandler builds the app shell handler: healthzHandler at
// "<basePath>healthz", the embedded UI at basePath, and — when ws is
// non-nil — a WebSocket upgrade endpoint at "<basePath>ws". ws is nil on
// every path except the dev-only --fixture build (M3); the full-featured
// route table replaces this ws plumbing in M10.
func NewHandler(basePath string, ws http.Handler) (http.Handler, error) {
	sub, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		return nil, err
	}

	basePath = normalizeBasePath(basePath)

	mux := http.NewServeMux()
	mux.HandleFunc(basePath+"healthz", healthzHandler)
	if ws != nil {
		mux.Handle(basePath+"ws", ws)
	}

	fileServer := http.FileServer(http.FS(sub))
	uiHandler := fileServer
	if basePath != "/" {
		uiHandler = http.StripPrefix(strings.TrimSuffix(basePath, "/"), fileServer)
	}
	mux.Handle(basePath, uiHandler)

	return mux, nil
}

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// normalizeBasePath ensures basePath starts and ends with "/", collapsing
// "", "/", and any path missing a leading or trailing slash to that form.
func normalizeBasePath(basePath string) string {
	if basePath == "" {
		basePath = "/"
	}
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	if !strings.HasSuffix(basePath, "/") {
		basePath += "/"
	}
	return basePath
}
