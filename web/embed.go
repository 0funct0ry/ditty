// Package web embeds the built UI (web/dist) into the ditty binary.
//
// Until the real UI lands in M4 (SPEC.md §12), `make build` writes a
// placeholder web/dist/index.html so embed.FS has something to embed at
// compile time — see the Makefile's $(WEB_DIST) rule.
package web

import "embed"

//go:embed all:dist
var DistFS embed.FS
