package cmd

import (
	"bytes"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"github.com/0funct0ry/ditty/internal/config"
	"github.com/0funct0ry/ditty/internal/store"
)

// newSecurityResolver builds a *config.Resolver over registerSecurityFlags'
// flag set, parsed from args, for exercising resolveSecurityFlags/
// buildGrants without a full execRun.
func newSecurityResolver(t *testing.T, args []string) *config.Resolver {
	t.Helper()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	registerSecurityFlags(fs)
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	resolver, err := config.New()
	if err != nil {
		t.Fatalf("config.New: %v", err)
	}
	if err := resolver.BindFlagSet(fs); err != nil {
		t.Fatalf("BindFlagSet: %v", err)
	}
	return resolver
}

func TestResolveSecurityFlags_TokenOnByDefault(t *testing.T) {
	resolver := newSecurityResolver(t, nil)
	cfg, secrets, err := resolveSecurityFlags(resolver)
	if err != nil {
		t.Fatalf("resolveSecurityFlags: %v", err)
	}
	if !cfg.tokenEnabled {
		t.Fatal("tokenEnabled = false, want true (SPEC.md §6.1 I3: on by default)")
	}
	if cfg.token == "" {
		t.Fatal("token was not generated")
	}
	if len(secrets) != 1 || secrets[0] != cfg.token {
		t.Fatalf("secrets = %v, want [token]", secrets)
	}
}

func TestResolveSecurityFlags_NoToken(t *testing.T) {
	resolver := newSecurityResolver(t, []string{"--no-token"})
	cfg, secrets, err := resolveSecurityFlags(resolver)
	if err != nil {
		t.Fatalf("resolveSecurityFlags: %v", err)
	}
	if cfg.tokenEnabled {
		t.Fatal("tokenEnabled = true, want false with --no-token")
	}
	if len(secrets) != 0 {
		t.Fatalf("secrets = %v, want none", secrets)
	}
}

func TestResolveSecurityFlags_ExplicitToken(t *testing.T) {
	resolver := newSecurityResolver(t, []string{"--token=mytoken123"})
	cfg, secrets, err := resolveSecurityFlags(resolver)
	if err != nil {
		t.Fatalf("resolveSecurityFlags: %v", err)
	}
	if cfg.token != "mytoken123" {
		t.Fatalf("token = %q, want mytoken123", cfg.token)
	}
	if len(secrets) != 1 || secrets[0] != "mytoken123" {
		t.Fatalf("secrets = %v", secrets)
	}
}

func TestResolveSecurityFlags_BasicAuth(t *testing.T) {
	resolver := newSecurityResolver(t, []string{"--basic-auth=alice:s3cret", "--no-token"})
	cfg, secrets, err := resolveSecurityFlags(resolver)
	if err != nil {
		t.Fatalf("resolveSecurityFlags: %v", err)
	}
	if cfg.basicAuth != "alice:s3cret" {
		t.Fatalf("basicAuth = %q", cfg.basicAuth)
	}
	if len(secrets) != 1 || secrets[0] != "s3cret" {
		t.Fatalf("secrets = %v, want [s3cret] (password only, never the username)", secrets)
	}
}

func TestResolveSecurityFlags_InvalidBasicAuth(t *testing.T) {
	resolver := newSecurityResolver(t, []string{"--basic-auth=nocolon", "--no-token"})
	if _, _, err := resolveSecurityFlags(resolver); err == nil {
		t.Fatal("resolveSecurityFlags(malformed --basic-auth) = nil error, want error")
	}
}

func TestResolveSecurityFlags_RejectsBlockedHeaderEnv(t *testing.T) {
	resolver := newSecurityResolver(t, []string{"--header-env=X-Foo:PATH", "--no-token"})
	if _, _, err := resolveSecurityFlags(resolver); err == nil {
		t.Fatal("resolveSecurityFlags(--header-env targeting PATH) = nil error, want error")
	}
}

func TestBuildGrants_TokenAndBasic(t *testing.T) {
	resolver := newSecurityResolver(t, []string{"--basic-auth=alice:s3cret", "--insecure-basic-over-http"})
	cfg, _, err := resolveSecurityFlags(resolver)
	if err != nil {
		t.Fatalf("resolveSecurityFlags: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	grants, tokenGrant, _, err := buildGrants(cfg, "/", "0.0.0.0:7654", logger, nil)
	if err != nil {
		t.Fatalf("buildGrants: %v", err)
	}
	if tokenGrant == nil {
		t.Fatal("tokenGrant = nil, want non-nil (token is on by default)")
	}
	if len(grants) != 2 {
		t.Fatalf("grants = %d, want 2 (token + basic)", len(grants))
	}
}

func TestBuildGrants_BasicOverPlaintextRefusedOnLAN(t *testing.T) {
	resolver := newSecurityResolver(t, []string{"--basic-auth=alice:s3cret", "--no-token"})
	cfg, _, err := resolveSecurityFlags(resolver)
	if err != nil {
		t.Fatalf("resolveSecurityFlags: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	if _, _, _, err := buildGrants(cfg, "/", "192.168.1.24:7654", logger, nil); err == nil {
		t.Fatal("buildGrants(--basic-auth over plaintext LAN, no override) = nil error, want error")
	}
}

func TestBuildGrants_TrustHeaderWithoutTrustProxyWarnsAndSkips(t *testing.T) {
	resolver := newSecurityResolver(t, []string{"--trust-header=X-Auth-User", "--no-token"})
	cfg, _, err := resolveSecurityFlags(resolver)
	if err != nil {
		t.Fatalf("resolveSecurityFlags: %v", err)
	}
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	grants, _, _, err := buildGrants(cfg, "/", "127.0.0.1:7654", logger, nil)
	if err != nil {
		t.Fatalf("buildGrants: %v", err)
	}
	if len(grants) != 0 {
		t.Fatalf("grants = %d, want 0 (trust-header without trust-proxy must never be constructed)", len(grants))
	}
	if !bytes.Contains(buf.Bytes(), []byte("trust-proxy")) {
		t.Fatal("expected a startup WARN mentioning trust-proxy")
	}
}

func TestBuildGrants_TrustHeaderWithTrustProxy(t *testing.T) {
	resolver := newSecurityResolver(t, []string{
		"--trust-header=X-Auth-User", "--trust-proxy=10.0.0.0/8", "--no-token",
	})
	cfg, _, err := resolveSecurityFlags(resolver)
	if err != nil {
		t.Fatalf("resolveSecurityFlags: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	grants, _, _, err := buildGrants(cfg, "/", "127.0.0.1:7654", logger, nil)
	if err != nil {
		t.Fatalf("buildGrants: %v", err)
	}
	if len(grants) != 1 {
		t.Fatalf("grants = %d, want 1", len(grants))
	}
}

func TestBuildGrants_ClientCAAddsMTLSGrant(t *testing.T) {
	resolver := newSecurityResolver(t, []string{"--client-ca=/dev/null", "--no-token"})
	cfg, _, err := resolveSecurityFlags(resolver)
	if err != nil {
		t.Fatalf("resolveSecurityFlags: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	grants, _, _, err := buildGrants(cfg, "/", "127.0.0.1:7654", logger, nil)
	if err != nil {
		t.Fatalf("buildGrants: %v", err)
	}
	if len(grants) != 1 {
		t.Fatalf("grants = %d, want 1 (mTLS)", len(grants))
	}
}

func TestResolveSecurityFlags_NoAuthDBByDefault(t *testing.T) {
	resolver := newSecurityResolver(t, []string{"--no-token"})
	cfg, _, err := resolveSecurityFlags(resolver)
	if err != nil {
		t.Fatalf("resolveSecurityFlags: %v", err)
	}
	if cfg.authDBPath != "" {
		t.Fatalf("authDBPath = %q, want empty when --auth-db is not passed", cfg.authDBPath)
	}
}

// TestAuthDBGuard_NoFileWithoutFlag reproduces execRun's exact guard around
// store.Open (cmd/run.go: "if secCfg.authDBPath != \"\" { store.Open(...) }")
// and asserts no *.db* file appears anywhere under a scratch directory when
// --auth-db is never passed (M12's acceptance criterion).
func TestAuthDBGuard_NoFileWithoutFlag(t *testing.T) {
	dir := t.TempDir()
	resolver := newSecurityResolver(t, []string{"--no-token"})
	cfg, _, err := resolveSecurityFlags(resolver)
	if err != nil {
		t.Fatalf("resolveSecurityFlags: %v", err)
	}

	if cfg.authDBPath != "" {
		s, err := store.Open(cfg.authDBPath)
		if err != nil {
			t.Fatalf("store.Open: %v", err)
		}
		defer func() { _ = s.Close() }()
	}

	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if strings.Contains(d.Name(), ".db") {
			t.Fatalf("unexpected database file %s created without --auth-db", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir: %v", err)
	}
}

func TestReadSecretFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	if err := os.WriteFile(path, []byte("s3cret\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := readSecretFile(path)
	if err != nil {
		t.Fatalf("readSecretFile: %v", err)
	}
	if got != "s3cret" {
		t.Fatalf("readSecretFile = %q, want %q (trailing newline trimmed)", got, "s3cret")
	}

	if _, err := readSecretFile(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("readSecretFile(missing file) = nil error, want error")
	}
}

func TestResolveSecurityFlags_BasicAuthFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "basic-auth")
	if err := os.WriteFile(path, []byte("alice:s3cret\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv(basicAuthFileEnv, path)

	resolver := newSecurityResolver(t, []string{"--no-token"})
	cfg, secrets, err := resolveSecurityFlags(resolver)
	if err != nil {
		t.Fatalf("resolveSecurityFlags: %v", err)
	}
	if cfg.basicAuth != "alice:s3cret" {
		t.Fatalf("basicAuth = %q, want alice:s3cret (from %s)", cfg.basicAuth, basicAuthFileEnv)
	}
	if len(secrets) != 1 || secrets[0] != "s3cret" {
		t.Fatalf("secrets = %v, want [s3cret]", secrets)
	}
}

func TestResolveSecurityFlags_BasicAuthFlagWinsOverFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "basic-auth")
	if err := os.WriteFile(path, []byte("fromfile:pw\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv(basicAuthFileEnv, path)

	resolver := newSecurityResolver(t, []string{"--basic-auth=alice:s3cret", "--no-token"})
	cfg, _, err := resolveSecurityFlags(resolver)
	if err != nil {
		t.Fatalf("resolveSecurityFlags: %v", err)
	}
	if cfg.basicAuth != "alice:s3cret" {
		t.Fatalf("basicAuth = %q, want the flag value to win over %s", cfg.basicAuth, basicAuthFileEnv)
	}
}

func TestResolveSecurityFlags_BasicAuthFileMissing(t *testing.T) {
	t.Setenv(basicAuthFileEnv, filepath.Join(t.TempDir(), "does-not-exist"))
	resolver := newSecurityResolver(t, []string{"--no-token"})
	if _, _, err := resolveSecurityFlags(resolver); err == nil {
		t.Fatal("resolveSecurityFlags(missing basic-auth file) = nil error, want error")
	}
}

func TestResolveSecurityFlags_JWTSecretFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jwt-secret")
	if err := os.WriteFile(path, []byte("supersecretvalue\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv(jwtSecretFileEnv, path)

	resolver := newSecurityResolver(t, []string{"--no-token", "--auth-db=" + filepath.Join(dir, "auth.db")})
	cfg, secrets, err := resolveSecurityFlags(resolver)
	if err != nil {
		t.Fatalf("resolveSecurityFlags: %v", err)
	}
	if cfg.jwtSecret != "supersecretvalue" {
		t.Fatalf("jwtSecret = %q, want supersecretvalue (from %s)", cfg.jwtSecret, jwtSecretFileEnv)
	}
	if len(secrets) != 1 || secrets[0] != "supersecretvalue" {
		t.Fatalf("secrets = %v, want [supersecretvalue]", secrets)
	}
}

func TestResolveSecurityFlags_JWTSecretFlagWinsOverFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jwt-secret")
	if err := os.WriteFile(path, []byte("fromfile"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv(jwtSecretFileEnv, path)

	resolver := newSecurityResolver(t, []string{
		"--no-token", "--auth-db=" + filepath.Join(dir, "auth.db"), "--jwt-secret=explicitsecret",
	})
	cfg, _, err := resolveSecurityFlags(resolver)
	if err != nil {
		t.Fatalf("resolveSecurityFlags: %v", err)
	}
	if cfg.jwtSecret != "explicitsecret" {
		t.Fatalf("jwtSecret = %q, want the flag value to win over %s", cfg.jwtSecret, jwtSecretFileEnv)
	}
}

func TestResolveSecurityFlags_JWTSecretFileMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(jwtSecretFileEnv, filepath.Join(dir, "does-not-exist"))
	resolver := newSecurityResolver(t, []string{"--no-token", "--auth-db=" + filepath.Join(dir, "auth.db")})
	if _, _, err := resolveSecurityFlags(resolver); err == nil {
		t.Fatal("resolveSecurityFlags(missing jwt-secret file) = nil error, want error")
	}
}

func TestAccessSummary(t *testing.T) {
	if got := accessSummary(false); got != "read-only · token" {
		t.Fatalf("accessSummary(false) = %q", got)
	}
	if got := accessSummary(true); got != "writable · token" {
		t.Fatalf("accessSummary(true) = %q", got)
	}
}
