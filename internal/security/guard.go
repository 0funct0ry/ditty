package security

import (
	"fmt"
	"net"
	"strings"
)

// bindGuardMessage is SPEC.md §6.1 I2's refusal message, word for word.
// addr is substituted into the first line; the rest is fixed.
const bindGuardMessage = `ditty: refusing to listen on %s without authentication.

This would expose %s to your entire network.

Pick one:
  --token                  random URL token (printed at startup)
  --basic-auth user:pass   HTTP basic authentication
  --auth-db ditty.db       SQLite users + JWT login page
  --client-ca ca.pem       mutual TLS
  --trust-header X-User    delegate to your reverse proxy
  --insecure-no-auth       I understand; do it anyway`

// BindGuard implements SPEC.md §6.1 I2: binding to anything other than
// loopback or a Unix socket requires a Grant. addr is the network address
// ditty is about to listen on (host:port, or a socket path when isUnixSocket
// is true); command is the Command's own name/argv, for the message's
// second line. It returns nil when it is safe to proceed: addr is loopback,
// addr is a Unix socket, at least one Grant is configured, or
// insecureNoAuth is set. Otherwise it returns an error whose Error() is the
// exact §6.1 message.
func BindGuard(addr string, isUnixSocket bool, command string, grantsConfigured, insecureNoAuth bool) error {
	if isUnixSocket || isLoopback(addr) || grantsConfigured || insecureNoAuth {
		return nil
	}
	return fmt.Errorf(bindGuardMessage, addr, command)
}

// IsLoopbackAddr reports whether addr (host:port, or a bare host) resolves
// to a loopback address. Exported for cmd/run.go's --insecure-basic-over-http
// check (SPEC.md §6.2), which needs the same loopback test BindGuard uses.
func IsLoopbackAddr(addr string) bool { return isLoopback(addr) }

// isLoopback reports whether addr (host:port, or a bare host) resolves to a
// loopback address.
func isLoopback(addr string) bool {
	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
