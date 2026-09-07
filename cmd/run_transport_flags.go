package cmd

import (
	"time"

	"github.com/spf13/pflag"
)

// registerTransportFlags defines the SPEC.md §4/§5.5/§6.3 flags internal/
// httpapi's real router and WebSocket handler (M10) consume: server-side
// WebSocket keepalive, the WS Origin check override, and the CSP
// frame-ancestors override. None of these have a short form in SPEC.md
// §8.1's flag table, so they are long-only and claim no new letters.
func registerTransportFlags(fs *pflag.FlagSet) {
	fs.Duration("ping-interval", 25*time.Second,
		"interval between server-initiated WebSocket pings; the read deadline is 2x this plus 5s")
	fs.String("origin-allow", "",
		"regex the WebSocket Origin header must match (default: same host as the request)")
	fs.String("allow-iframe", "",
		"allow this session to be framed: omitted defaults to frame-ancestors 'none', "+
			"a value sets that CSP frame-ancestors source, and the bare flag with no "+
			"value omits frame-ancestors entirely (allows any framer)")
	fs.Lookup("allow-iframe").NoOptDefVal = "*"
	fs.String("socket", "",
		"listen on this Unix socket path instead of --address/--port (SPEC.md §8.1); mutually exclusive with both")
	fs.String("socket-mode", "0600", "file mode for the Unix socket, e.g. 0600 (--socket only)")
	fs.String("socket-owner", "", "user[:group] to chown the Unix socket to (--socket only)")
}
