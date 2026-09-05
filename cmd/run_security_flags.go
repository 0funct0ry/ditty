package cmd

import (
	"github.com/spf13/pflag"

	"github.com/0funct0ry/ditty/internal/security"
)

// registerSecurityFlags defines the SPEC.md §6 Grant, transport-hardening
// and untrusted-input-surface flags internal/security (M11) consumes: the
// four Grant types (token, basic, mTLS via TLS + --client-ca,
// trust-header), the bind guard's --insecure-no-auth escape hatch, and the
// two narrow, off-by-default command-injection surfaces (§6.4). Newly
// claimed short letters: none — every flag here is long-only, per §8.1's
// access flag table having no short forms of its own.
func registerSecurityFlags(fs *pflag.FlagSet) {
	fs.String("token", "", "require this URL token (SPEC.md §4.1); bare --token generates a random one")
	fs.Lookup("token").NoOptDefVal = "-"
	fs.Bool("no-token", false, "disable the default-on URL token (SPEC.md §6.1 I3)")
	fs.Int("token-length", 16, "random token length in bytes when --token generates its own (minimum 8)")

	fs.String("basic-auth", "", "require HTTP basic authentication as user:pass (also DITTY_BASIC_AUTH)")
	fs.Bool("insecure-basic-over-http", false,
		"allow --basic-auth over plaintext on a non-loopback address (credentials travel in the clear)")

	fs.String("tls-cert", "", "TLS certificate file; enables HTTPS")
	fs.String("tls-key", "", "TLS private key file; enables HTTPS")
	fs.String("client-ca", "", "CA certificate file; requires and verifies a client certificate (mutual TLS)")

	fs.String("trust-header", "", "trust this request header for the Client's identity, e.g. X-Auth-User")
	fs.StringSlice("trust-proxy", nil,
		"CIDR(s) whose peers may set --trust-header (required for --trust-header to have any effect)")

	fs.Bool("insecure-no-auth", false,
		"start without any Grant even on a non-loopback address (SPEC.md §6.1 I2); logs a WARN every 60s")

	fs.Bool("allow-url-args", false, "allow ?arg= query values to be appended to the Command's argv (SPEC.md §6.4)")
	fs.String("arg-pattern", security.DefaultArgPattern, "regex every --allow-url-args value must match")

	fs.StringArray("header-env", nil,
		"map a request header into the Command's environment as Header:TARGET (repeatable, SPEC.md §6.4)")
}
