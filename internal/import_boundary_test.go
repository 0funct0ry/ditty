// Package internal holds this repo-wide dependency-direction test: it has
// no production code of its own.
package internal

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// forbiddenImports names every import path internal/session, internal/pty
// and internal/wire must never carry (CLAUDE.md, SPEC.md §2.1): those
// packages sit below internal/httpapi in the dependency direction, and must
// stay free of HTTP/Gin so they can be reused as an embeddable library
// without pulling in a web framework.
var forbiddenImports = []string{
	"net/http",
	"github.com/gin-gonic/gin",
}

// guardedPackages are scanned relative to this file's own directory
// (internal/).
var guardedPackages = []string{"session", "pty", "wire"}

func TestDependencyDirection_NoHTTPBelowHTTPAPI(t *testing.T) {
	for _, pkg := range guardedPackages {
		t.Run(pkg, func(t *testing.T) {
			scanPackageImports(t, pkg, forbiddenImports)
		})
	}
}

// securityForbiddenImports are import paths internal/security (M11) must
// never carry: it sits below internal/httpapi in the dependency direction
// (SPEC.md §2.1 — only cmd and security may import internal/store, and
// security itself must never import httpapi or session/Gin), even though it
// legitimately imports net/http for the Grant interface's *http.Request.
var securityForbiddenImports = []string{
	"github.com/gin-gonic/gin",
	"github.com/0funct0ry/ditty/internal/httpapi",
	"github.com/0funct0ry/ditty/internal/session",
}

func TestDependencyDirection_SecurityStaysBelowHTTPAPI(t *testing.T) {
	scanPackageImports(t, "security", securityForbiddenImports)
}

// scanPackageImports fails the test if any .go file directly under
// internal/pkg imports one of forbidden.
func scanPackageImports(t *testing.T, pkg string, forbidden []string) {
	t.Helper()
	dir := filepath.Join(".", pkg)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbidden {
				if importPath == bad {
					t.Errorf("%s imports %q, which internal/%s must never import", path, importPath, pkg)
				}
			}
		}
	}
}
