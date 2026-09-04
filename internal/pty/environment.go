package pty

import (
	"os"
	"strings"
)

// buildEnv constructs the child's environment: the parent environment with
// every DITTY_* entry dropped (so the child doesn't inherit ditty's own
// configuration), then Command.Env layered on top, then TERM and
// DITTY_SESSION set unconditionally.
//
// Entries are deduplicated by key rather than blindly appended: a POSIX
// envp array is not guaranteed last-wins on a duplicate key (common libc
// getenv implementations return the first match), so simply appending an
// override after the parent's own value would not reliably take effect.
func buildEnv(c Command) []string {
	index := make(map[string]int)
	var out []string

	set := func(kv string) {
		key, _, ok := strings.Cut(kv, "=")
		if !ok {
			return
		}
		if i, exists := index[key]; exists {
			out[i] = kv
			return
		}
		index[key] = len(out)
		out = append(out, kv)
	}

	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "DITTY_") {
			continue
		}
		set(kv)
	}
	for _, kv := range c.Env {
		set(kv)
	}

	// TERM: an explicit Command.Term (the --term flag, which carries its
	// own "xterm-256color" default) always wins. Otherwise leave whatever
	// the parent env or --env already produced; only fall back to the
	// default if TERM is missing entirely.
	if c.Term != "" {
		set("TERM=" + c.Term)
	} else if _, ok := index["TERM"]; !ok {
		set("TERM=xterm-256color")
	}
	set("DITTY_SESSION=" + c.SessionID)

	return out
}
