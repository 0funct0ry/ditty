package httpapi

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/0funct0ry/ditty/web"
)

// renderIndex reads the embedded UI's index.html once at startup and, when
// basePath is non-root, rewrites its root-absolute "/assets/..." references
// to "<basePath>/assets/..." — the two hardcoded asset paths Vite bakes in
// (SPEC.md §4's base-path rewriting requirement). basePath is already
// normalized (no trailing slash, "" for root) by normalizeBasePath.
func renderIndex(basePath string) ([]byte, error) {
	raw, err := fs.ReadFile(web.DistFS, "dist/index.html")
	if err != nil {
		return nil, err
	}
	if basePath == "" {
		return raw, nil
	}
	rewritten := strings.ReplaceAll(string(raw), `src="/assets/`, `src="`+basePath+`/assets/`)
	rewritten = strings.ReplaceAll(rewritten, `href="/assets/`, `href="`+basePath+`/assets/`)
	return []byte(rewritten), nil
}

// indexHandler serves the (possibly base-path-rewritten) embedded index.html.
func indexHandler(index []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	}
}

// faviconHandler serves the embedded favicon, or 404 when web/dist ships
// none — --favicon overrides are deferred past M10.
func faviconHandler(c *gin.Context) {
	data, err := fs.ReadFile(web.DistFS, "dist/favicon.ico")
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Data(http.StatusOK, "image/x-icon", data)
}
