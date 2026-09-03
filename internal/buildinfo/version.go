// Package buildinfo carries version metadata stamped in at link time via
// -ldflags (see the Makefile's BUILDINFO/LDFLAGS variables).
package buildinfo

// Version, Commit and Date default to "dev" for `go run`/`go build` without
// the Makefile's ldflags, and are overwritten at link time otherwise.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)
