package cmd

import "github.com/spf13/pflag"

// registerCommandFlags defines the SPEC.md §8.1 "Command & session" flags
// that internal/pty (M8) actually consumes, plus the §3.1 lifecycle flags
// that shape a Command's spawn/close behaviour. internal/session's own
// ring/output-pump/write-capability flags are registered separately by
// registerSessionFlags (M9); security-only flags (--allow-url-args,
// --arg-pattern, --header-env) are still deferred to M11. Newly claimed
// short letters: d E T U G N i y z Z x W g X K.
func registerCommandFlags(fs *pflag.FlagSet) {
	fs.StringP("cwd", "d", "", "working directory for the Command (default: ditty's own cwd)")
	fs.StringArrayP("env", "E", nil, "additional KEY=VAL environment variable for the Command (repeatable)")
	fs.StringP("term", "T", "xterm-256color", "TERM value set for the Command")
	fs.StringP("uid", "U", "", "run the Command as this numeric uid (requires ditty to run as root)")
	fs.StringP("gid", "G", "", "run the Command as this numeric gid (requires ditty to run as root)")
	fs.StringP("name", "N", "", "short Session name, used in URLs and titles")
	fs.StringP("title", "i", "{command} — {hostname}", "Session title template")
	fs.Uint16P("cols", "y", 0, "fixed PTY column count (0 = dynamic, sized by the first Client)")
	fs.Uint16P("rows", "z", 0, "fixed PTY row count (0 = dynamic, sized by the first Client)")

	fs.BoolP("once", "Z", false, "accept exactly one Client, ever; exit after it detaches")
	fs.BoolP("exit-on-detach", "x", false, "shorthand for --detach-grace 0s plus exiting the process")
	fs.DurationP("wait-for-client", "W", 0,
		"exit if no Client attaches within this duration (0 = forever)")
	fs.DurationP("detach-grace", "g", 0,
		"after the last Client detaches, wait this long before signalling the Command (0 = never)")
	fs.BoolP("close-on-exit", "X", true,
		"when the Command exits, show the closed state instead of offering reconnect")
	fs.StringP("kill-signal", "K", "SIGHUP",
		"signal sent to the process group on close (SIGKILL after a fixed 5s escalation)")
}
