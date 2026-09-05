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
	fs.IntP("max-clients", "M", 0, "reject Clients past this many simultaneous attachments (0 = unlimited); --shared only")
	fs.BoolP("writable", "w", false, "allow attached Clients to type into the Command (SPEC.md §6.1 I1)")
	fs.Bool("shared", false,
		"share one Command/PTY across every Client, fanning out its output (ditty's pre-M10 default). "+
			"Without this flag (the default), every Client gets its own fresh Command, spawned on connect "+
			"and torn down when that Client disconnects — one Client's Command exiting never affects any "+
			"other Client, and a page refresh always starts a brand-new Command, with no reconnect/replay. "+
			"--max-clients, --wait-for-client, --detach-grace and --exit-on-detach only apply in --shared mode.")
}
