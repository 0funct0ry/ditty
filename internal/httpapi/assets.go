package httpapi

import (
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/0funct0ry/ditty/web"
)

// assetsHandler serves the embedded UI's hashed, immutable JS/CSS/font
// files (SPEC.md §4) with a one-year cache lifetime — safe because Vite
// bakes a content hash into every filename under web/dist/assets. basePath
// is the already-normalized route group prefix (see normalizeBasePath), so
// the request path arriving here is "<basePath>/assets/<name>".
func assetsHandler(basePath string) gin.HandlerFunc {
	sub, err := fs.Sub(web.DistFS, "dist/assets")
	if err != nil {
		panic(err) // web/dist/assets is embedded at build time; this can't fail at runtime.
	}
	fileServer := http.StripPrefix(basePath+"/assets", http.FileServer(http.FS(sub)))

	return func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		fileServer.ServeHTTP(c.Writer, c.Request)
	}
}
