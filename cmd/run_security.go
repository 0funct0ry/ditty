package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/0funct0ry/ditty/internal/config"
	"github.com/0funct0ry/ditty/internal/security"
	"github.com/0funct0ry/ditty/internal/store"
)

// Docker/Kubernetes secrets-file env vars (SPEC.md §15): a path in the env
// var, the actual secret in the file it names. These are a container
// convention, not a §8.1 flag with a Viper twin, so they are read via plain
// os.Getenv rather than through config.Resolver — resolveSecurityFlags
// still does no I/O beyond this and crypto/rand, so it can run before the
// logger (which needs secrets registered first) is built.
const (
	basicAuthFileEnv = "DITTY_BASIC_AUTH_FILE"
	jwtSecretFileEnv = "DITTY_JWT_SECRET_FILE"
)

// readSecretFile reads a Docker/Kubernetes secrets-style file, trimming a
// single trailing newline the way most secret-mounting tooling writes
// files.
func readSecretFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading secret file %q: %w", path, err)
	}
	return strings.TrimRight(string(b), "\r\n"), nil
}

// errSilent is returned by execRun when it has already printed the relevant
// message itself (the bind guard's exact SPEC.md §6.1 wording) and cobra
// must not additionally print its own "Error: ..." line.
var errSilent = errors.New("ditty: exiting")

// insecureNoAuthWarnInterval is how often the --insecure-no-auth reminder
// repeats for the life of the process (SPEC.md §6.1 I2).
const insecureNoAuthWarnInterval = 60 * time.Second

// warnInsecureNoAuth logs the --insecure-no-auth reminder every
// insecureNoAuthWarnInterval until ctx is done.
func warnInsecureNoAuth(ctx context.Context, logger *slog.Logger) {
	ticker := time.NewTicker(insecureNoAuthWarnInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			logger.Warn("running with --insecure-no-auth: this Session has no Grant and is not protected")
		case <-ctx.Done():
			return
		}
	}
}

// accessSummary renders SPEC.md §4.1's startup-banner access line.
// accessSummary renders SPEC.md §4.1's startup-banner access line: the
// write mode, every Grant actually active (never hardcoded — see
// grantNames), and the current Client count (always 0 at startup, since
// the banner prints before the server accepts its first connection).
// "read-write"/"read-only" matches the UI's own badge copy (ChromeBar.tsx)
// rather than the older "writable", so the two surfaces agree.
func accessSummary(writable bool, grants []string, clientCount int) string {
	mode := "read-only"
	if writable {
		mode = "read-write"
	}
	grantList := "none"
	if len(grants) > 0 {
		grantList = strings.Join(grants, "+")
	}
	noun := "clients"
	if clientCount == 1 {
		noun = "client"
	}
	return fmt.Sprintf("%s · %s · %d %s", mode, grantList, clientCount, noun)
}

// defaultTokenCookieTTL is the grant cookie's Max-Age (SPEC.md §4.1) until a
// dedicated --token-ttl flag exists (a link TTL is deferred to v1.1 M17 per
// SPEC.md §6.6's residual-risk table).
const defaultTokenCookieTTL = 24 * time.Hour

// securityConfig is every SPEC.md §6 flag resolved and validated, but not
// yet turned into Grant objects — resolveSecurityFlags does no I/O and
// never logs, so it can run before the logger (which needs its secrets
// registered first) is built.
type securityConfig struct {
	tokenEnabled bool
	token        string // "" when tokenEnabled but generated randomly below
	tokenLength  int

	basicAuth             string // "user:pass", "" if unset
	insecureBasicOverHTTP bool

	tlsCert, tlsKey, clientCA string

	trustHeader string
	trustProxy  []string

	insecureNoAuth bool

	allowURLArgs bool
	argPattern   string

	headerEnvMappings []security.HeaderEnvMapping

	authDBPath string
	jwtSecret  string // "" when --auth-db is unset; otherwise literal or randomly generated below
}

// resolveSecurityFlags parses and validates every §6 flag, generating the
// URL token now (SPEC.md §6.1 I3: on by default unless --no-token) since
// doing so needs no I/O beyond crypto/rand. It returns the resolved config
// alongside every secret value that must be registered with a
// security.Redactor before any logging happens.
func resolveSecurityFlags(resolver *config.Resolver) (securityConfig, []string, error) {
	var cfg securityConfig
	var secrets []string

	cfg.insecureBasicOverHTTP = resolver.Bool("insecure-basic-over-http")
	cfg.tlsCert = resolver.String("tls-cert")
	cfg.tlsKey = resolver.String("tls-key")
	cfg.clientCA = resolver.String("client-ca")
	cfg.trustHeader = resolver.String("trust-header")
	cfg.trustProxy = resolver.StringSlice("trust-proxy")
	cfg.insecureNoAuth = resolver.Bool("insecure-no-auth")
	cfg.allowURLArgs = resolver.Bool("allow-url-args")
	cfg.argPattern = resolver.String("arg-pattern")

	cfg.tokenEnabled = !resolver.Bool("no-token")
	if cfg.tokenEnabled {
		cfg.tokenLength = resolver.Int("token-length")
		literal := resolver.String("token")
		if literal == "-" { // bare --token (NoOptDefVal): generate one
			literal = ""
		}
		if literal == "" {
			generated, err := security.GenerateToken(cfg.tokenLength)
			if err != nil {
				return cfg, nil, fmt.Errorf("--token: %w", err)
			}
			literal = generated
		}
		cfg.token = literal
		secrets = append(secrets, cfg.token)
	}

	basicAuth := resolver.String("basic-auth")
	if basicAuth == "" {
		if path := os.Getenv(basicAuthFileEnv); path != "" {
			fileValue, err := readSecretFile(path)
			if err != nil {
				return cfg, nil, fmt.Errorf("%s: %w", basicAuthFileEnv, err)
			}
			basicAuth = fileValue
		}
	}
	if basicAuth != "" {
		user, pass, ok := strings.Cut(basicAuth, ":")
		if !ok || user == "" {
			return cfg, nil, fmt.Errorf("--basic-auth must be user:pass, got %q", basicAuth)
		}
		cfg.basicAuth = basicAuth
		secrets = append(secrets, pass)
	}

	for _, spec := range resolver.StringSlice("header-env") {
		m, err := security.ParseHeaderEnvMapping(spec)
		if err != nil {
			return cfg, nil, err
		}
		cfg.headerEnvMappings = append(cfg.headerEnvMappings, m)
	}

	// Validate --arg-pattern eagerly so a bad regex fails at startup, not
	// on the first request.
	if _, err := security.NewURLArgFilter(cfg.allowURLArgs, cfg.argPattern); err != nil {
		return cfg, nil, err
	}

	cfg.authDBPath = resolver.String("auth-db")
	if cfg.authDBPath != "" {
		jwtSecret := resolver.String("jwt-secret")
		if jwtSecret == "" {
			if path := os.Getenv(jwtSecretFileEnv); path != "" {
				fileValue, err := readSecretFile(path)
				if err != nil {
					return cfg, nil, fmt.Errorf("%s: %w", jwtSecretFileEnv, err)
				}
				jwtSecret = fileValue
			}
		}
		if jwtSecret == "" {
			// A random per-run secret (SPEC.md §6.2): restarting ditty
			// invalidates every JWT session, the same story --token already
			// tells for its own restart behaviour.
			b := make([]byte, 32)
			if _, err := rand.Read(b); err != nil {
				return cfg, nil, fmt.Errorf("--jwt-secret: %w", err)
			}
			jwtSecret = hex.EncodeToString(b)
		}
		cfg.jwtSecret = jwtSecret
		secrets = append(secrets, cfg.jwtSecret)
	}

	return cfg, secrets, nil
}

// deferredWarning is a log line buildGrants would otherwise emit
// immediately, held instead so the caller can flush it after the startup
// banner has printed — every log message appears after the banner, never
// interleaved above it (SPEC.md §4.1).
type deferredWarning struct {
	msg  string
	args []any
}

// buildGrants turns a resolved securityConfig into the active Grant set
// (SPEC.md §6.2: any one admits), the TokenGrant (nil when tokens are
// disabled, kept separate from Grants because /t/:token and /api/logout
// need its Exchange method specifically), and the JWTGrant (nil unless
// authStore is non-nil, i.e. --auth-db was passed). address is the address
// ditty is about to bind, used only to decide whether --basic-auth over
// plaintext is refused (SPEC.md §6.2). It does no logging itself — any
// warning is returned for the caller to log once the banner is up.
func buildGrants(cfg securityConfig, basePath, address string, authStore *store.Store) (
	security.Grants, *security.TokenGrant, *security.JWTGrant, []deferredWarning, error,
) {
	var grants security.Grants
	var tokenGrant *security.TokenGrant
	var jwtGrant *security.JWTGrant
	var warnings []deferredWarning

	if cfg.tokenEnabled {
		tokenGrant = security.NewTokenGrant(cfg.token, basePath, cfg.tlsCert != "", defaultTokenCookieTTL)
		grants = append(grants, tokenGrant)
	}

	if cfg.basicAuth != "" {
		if cfg.tlsCert == "" && !cfg.insecureBasicOverHTTP && !security.IsLoopbackAddr(address) {
			return nil, nil, nil, nil, fmt.Errorf(
				"--basic-auth over plaintext on a non-loopback address requires --insecure-basic-over-http (SPEC.md §6.2)")
		}
		basicGrant, err := security.NewBasicGrant(cfg.basicAuth)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		grants = append(grants, basicGrant)
	}

	if cfg.clientCA != "" {
		grants = append(grants, security.NewMTLSGrant())
	}

	if cfg.trustHeader != "" {
		if len(cfg.trustProxy) == 0 {
			warnings = append(warnings, deferredWarning{
				msg:  "--trust-header set without --trust-proxy; the header will never be honoured",
				args: []any{"header", cfg.trustHeader},
			})
		} else {
			thGrant, err := security.NewTrustedHeaderGrant(cfg.trustHeader, cfg.trustProxy)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			grants = append(grants, thGrant)
		}
	}

	if authStore != nil {
		jwtGrant = security.NewJWTGrant([]byte(cfg.jwtSecret), authStore, basePath, cfg.tlsCert != "")
		grants = append(grants, jwtGrant)
	}

	return grants, tokenGrant, jwtGrant, warnings, nil
}

// grantNames returns the short, human-readable name of every Grant
// actually active, in the order buildGrants constructs them, for the
// startup banner's access line. An empty result means no Grant is
// configured at all (only reachable when the bind guard has separately
// permitted no-auth: loopback, or --insecure-no-auth).
func grantNames(grants security.Grants) []string {
	names := make([]string, 0, len(grants))
	for _, g := range grants {
		switch g.(type) {
		case *security.TokenGrant:
			names = append(names, "token")
		case *security.BasicGrant:
			names = append(names, "basic-auth")
		case *security.MTLSGrant:
			names = append(names, "mTLS")
		case *security.TrustedHeaderGrant:
			names = append(names, "trust-header")
		case *security.JWTGrant:
			names = append(names, "auth-db")
		}
	}
	return names
}
