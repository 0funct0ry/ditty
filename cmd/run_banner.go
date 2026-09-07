package cmd

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// bannerInfo is every piece of SPEC.md §4.1's required startup-banner
// information: the version, the Session's own id and Command, the
// complete shareable URL, and the access summary (accessSummary).
type bannerInfo struct {
	version   string
	sessionID string
	command   string
	url       string
	access    string
}

// bannerColors are the ANSI escape sequences the startup banner uses when
// color is enabled; every field is the empty string when it's not, so
// printBanner never needs to branch on whether color is on.
type bannerColors struct {
	reset, wordmark, dim, url string
}

func newBannerColors(enabled bool) bannerColors {
	if !enabled {
		return bannerColors{}
	}
	return bannerColors{
		reset:    "\x1b[0m",
		wordmark: "\x1b[1;32m",   // bold green
		dim:      "\x1b[2m",      // dim, for labels
		url:      "\x1b[1;4;36m", // bold, underlined, cyan — reads as a link
	}
}

// bannerColorEnabled reports whether the startup banner should color its
// output: only when w is an actual terminal — not a pipe, a redirected
// file, or the in-memory buffer cmd.SetOut swaps in during tests — and the
// widely recognized NO_COLOR convention (https://no-color.org) isn't set.
func bannerColorEnabled(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// printBanner prints ditty's startup banner (SPEC.md §4.1) to w. It is the
// only thing ditty prints before it starts accepting connections: every
// log line — "Command spawned", "ditty starting", every warning — is
// emitted after this, never before or interleaved with it, so an
// operator's terminal shows one clean block up front.
func printBanner(w io.Writer, info bannerInfo) {
	c := newBannerColors(bannerColorEnabled(w))
	_, _ = fmt.Fprintf(w, "%sditty%s %s%s%s\n\n", c.wordmark, c.reset, c.dim, info.version, c.reset)
	_, _ = fmt.Fprintf(w, "  %ssession%s   %s (%s)\n", c.dim, c.reset, info.sessionID, info.command)
	_, _ = fmt.Fprintf(w, "  %surl%s       %s%s%s\n", c.dim, c.reset, c.url, info.url, c.reset)
	_, _ = fmt.Fprintf(w, "  %saccess%s    %s\n\n", c.dim, c.reset, info.access)
}
