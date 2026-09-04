//go:build windows

// Package nopty exists only to fail the build with a clear message: ditty's
// PTY layer is Unix-only (linux, darwin, freebsd). It deliberately declares
// a different package name than the rest of internal/pty, so building ditty
// for GOOS=windows fails at compile time with an unambiguous
// "found packages pty (...) and nopty (unsupported_windows.go)" error
// rather than a confusing runtime failure.
package nopty
