package cmd

import (
	"time"

	"github.com/spf13/pflag"
)

// registerSessionFlags defines the SPEC.md §8.1 session-only flags
// internal/session (M9) consumes: the ring/output-pump tuning knobs and
// write capability. --max-clients was previously registered only inside
// the dev "fixture" build tag (M3); this is its real, always-present home.
// Security-only flags (--allow-url-args, --arg-pattern, --header-env) stay
// deferred to M11. Newly claimed short letters: S u m M w.
func registerSessionFlags(fs *pflag.FlagSet) {
	fs.IntP("scrollback-bytes", "S", 262144, "ring buffer capacity in bytes (0 disables it)")
	fs.IntP("chunk-bytes", "u", 32768, "PTY read buffer size in bytes, before coalescing")
	fs.DurationP("flush-interval", "m", 5*time.Millisecond, "coalesce PTY output for this long before fanning it out")
	fs.IntP("max-clients", "M", 0, "reject Clients past this many simultaneous attachments (0 = unlimited)")
	fs.BoolP("writable", "w", false, "allow attached Clients to type into the Command (SPEC.md §6.1 I1)")
}
