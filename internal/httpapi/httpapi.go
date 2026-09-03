// Package httpapi is the M1 app shell: it answers /healthz and serves the
// embedded UI under --base-path. There is no Session, no Grant, no PTY and
// no WebSocket here yet — those arrive with the fixture Hub (M3) and the
// real transport (M10). Gin is not used until M10 either; net/http alone
// covers this milestone's scope (SPEC.md §12 M1).
package httpapi

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"

	"github.com/0funct0ry/ditty/web"
)

// NewHandler builds the M1 shell handler: healthzHandler at
// "<basePath>healthz" and the embedded UI at basePath.
func NewHandler(basePath string) (http.Handler, error) {
	sub, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		return nil, err
	}

	basePath = normalizeBasePath(basePath)

	mux := http.NewServeMux()
	mux.HandleFunc(basePath+"healthz", healthzHandler)

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
