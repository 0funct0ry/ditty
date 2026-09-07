package cmd

import (
	"net"
	"os"
	"os/user"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/spf13/pflag"

	"github.com/0funct0ry/ditty/internal/config"
)

// newSocketResolver builds a *config.Resolver over the full `ditty run`
// flag surface (registerRunFlags), parsed from args, mirroring
// newTestResolver/newSecurityResolver's pattern — resolveSocketConfig needs
// both --socket and --address/--port to exist on the same flag set to test
// mutual exclusivity.
func newSocketResolver(t *testing.T, args []string) (*pflag.FlagSet, *config.Resolver) {
	t.Helper()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	registerRunFlags(fs)
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
	return fs, resolver
}

func TestResolveSocketConfig_EmptyWhenNotSet(t *testing.T) {
	fs, resolver := newSocketResolver(t, nil)
	cfg, err := resolveSocketConfig(fs, resolver)
	if err != nil {
		t.Fatalf("resolveSocketConfig: %v", err)
	}
	if cfg.path != "" {
		t.Errorf("path = %q, want empty when --socket is not set", cfg.path)
	}
}

func TestResolveSocketConfig_ParsesModeAndOwner(t *testing.T) {
	fs, resolver := newSocketResolver(t, []string{
		"--socket", "/run/ditty.sock",
		"--socket-mode", "0640",
		"--socket-owner", "alice:staff",
	})
	cfg, err := resolveSocketConfig(fs, resolver)
	if err != nil {
		t.Fatalf("resolveSocketConfig: %v", err)
	}
	if cfg.path != "/run/ditty.sock" {
		t.Errorf("path = %q, want /run/ditty.sock", cfg.path)
	}
	if cfg.mode != 0o640 {
		t.Errorf("mode = %o, want 0640", cfg.mode)
	}
	if cfg.owner != "alice:staff" {
		t.Errorf("owner = %q, want alice:staff", cfg.owner)
	}
}

func TestResolveSocketConfig_BadModeErrors(t *testing.T) {
	fs, resolver := newSocketResolver(t, []string{"--socket", "/run/ditty.sock", "--socket-mode", "not-octal"})
	if _, err := resolveSocketConfig(fs, resolver); err == nil {
		t.Fatal("expected an error for a non-octal --socket-mode")
	}
}

// TestResolveSocketConfig_MutuallyExclusiveWithAddress asserts SPEC.md
// §8.1's "mutually exclusive with address/port" — --socket combined with an
// explicitly-passed --address must be refused, not silently ignored.
func TestResolveSocketConfig_MutuallyExclusiveWithAddress(t *testing.T) {
	fs, resolver := newSocketResolver(t, []string{"--socket", "/run/ditty.sock", "--address", "0.0.0.0"})
	if _, err := resolveSocketConfig(fs, resolver); err == nil {
		t.Fatal("expected an error when --socket and --address are both given")
	}
}

func TestResolveSocketConfig_MutuallyExclusiveWithPort(t *testing.T) {
	fs, resolver := newSocketResolver(t, []string{"--socket", "/run/ditty.sock", "--port", "8080"})
	if _, err := resolveSocketConfig(fs, resolver); err == nil {
		t.Fatal("expected an error when --socket and --port are both given")
	}
}

// shortSocketDir returns a temp directory with a short path — Unix socket
// paths are capped at ~104 bytes on macOS/BSD, well under what t.TempDir()
// produces once it embeds the full test name.
func shortSocketDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "dsock")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func TestListenUnix_CreatesSocketWithModeAndRemovesStaleFile(t *testing.T) {
	dir := shortSocketDir(t)
	path := filepath.Join(dir, "ditty.sock")

	// A stale socket file, as an unclean previous exit would leave behind —
	// listenUnix must remove it rather than failing with "already in use".
	stale, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("create stale socket: %v", err)
	}
	_ = stale.Close()
	// net.UnixListener.Close() already unlinks path; recreate the file by
	// hand so the "stale, still-present file" case is actually exercised.
	if f, err := os.Create(path); err != nil {
		t.Fatalf("recreate stale file: %v", err)
	} else {
		_ = f.Close()
	}

	ln, err := listenUnix(socketConfig{path: path, mode: 0o600})
	if err != nil {
		t.Fatalf("listenUnix: %v", err)
	}
	defer func() { _ = ln.Close() }()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %o, want 0600", info.Mode().Perm())
	}
	if ln.Addr().String() != path {
		t.Errorf("Addr() = %q, want %q", ln.Addr().String(), path)
	}
}

func TestListenUnix_RemovesSocketOnClose(t *testing.T) {
	dir := shortSocketDir(t)
	path := filepath.Join(dir, "ditty.sock")

	ln, err := listenUnix(socketConfig{path: path, mode: 0o600})
	if err != nil {
		t.Fatalf("listenUnix: %v", err)
	}
	if err := ln.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("socket file still exists after Close: err=%v", err)
	}
}

func TestLookupOwner_UnknownUserErrors(t *testing.T) {
	if _, _, err := lookupOwner("no-such-user-ditty-test"); err == nil {
		t.Fatal("expected an error for an unknown user")
	}
}

// currentUserAndGroup resolves the running test's own user and primary
// group names, so owner-resolution tests work on whatever machine runs
// them rather than depending on a fixed uid/username existing everywhere.
func currentUserAndGroup(t *testing.T) (username, groupname string) {
	t.Helper()
	u, err := user.Current()
	if err != nil {
		t.Skipf("user.Current: %v", err)
	}
	g, err := user.LookupGroupId(u.Gid)
	if err != nil {
		t.Skipf("user.LookupGroupId(%s): %v", u.Gid, err)
	}
	return u.Username, g.Name
}

func TestLookupOwner_ValidUserNoGroup(t *testing.T) {
	username, _ := currentUserAndGroup(t)
	uid, gid, err := lookupOwner(username)
	if err != nil {
		t.Fatalf("lookupOwner(%q): %v", username, err)
	}
	if uid != os.Getuid() {
		t.Errorf("uid = %d, want %d", uid, os.Getuid())
	}
	if gid != -1 {
		t.Errorf("gid = %d, want -1 (unchanged) when no group is given", gid)
	}
}

func TestLookupOwner_ValidUserAndGroup(t *testing.T) {
	username, groupname := currentUserAndGroup(t)
	uid, gid, err := lookupOwner(username + ":" + groupname)
	if err != nil {
		t.Fatalf("lookupOwner(%q): %v", username+":"+groupname, err)
	}
	if uid != os.Getuid() {
		t.Errorf("uid = %d, want %d", uid, os.Getuid())
	}
	if gid != os.Getgid() {
		t.Errorf("gid = %d, want %d", gid, os.Getgid())
	}
}

func TestLookupOwner_UnknownGroupErrors(t *testing.T) {
	username, _ := currentUserAndGroup(t)
	if _, _, err := lookupOwner(username + ":no-such-group-ditty-test"); err == nil {
		t.Fatal("expected an error for an unknown group")
	}
}

// TestListenUnix_AppliesOwner exercises listenUnix's --socket-owner branch
// with the test's own user, the only chown that's guaranteed to succeed
// without root.
func TestListenUnix_AppliesOwner(t *testing.T) {
	username, _ := currentUserAndGroup(t)
	dir := shortSocketDir(t)
	path := filepath.Join(dir, "ditty.sock")

	ln, err := listenUnix(socketConfig{path: path, mode: 0o600, owner: username})
	if err != nil {
		t.Fatalf("listenUnix: %v", err)
	}
	defer func() { _ = ln.Close() }()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Skip("os.FileInfo.Sys() is not *syscall.Stat_t on this platform")
	}
	if int(stat.Uid) != os.Getuid() {
		t.Errorf("socket uid = %d, want %d", stat.Uid, os.Getuid())
	}
}

func TestListenUnix_UnknownOwnerErrors(t *testing.T) {
	dir := shortSocketDir(t)
	path := filepath.Join(dir, "ditty.sock")
	if _, err := listenUnix(socketConfig{path: path, mode: 0o600, owner: "no-such-user-ditty-test"}); err == nil {
		t.Fatal("expected an error for an unknown --socket-owner")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("socket file should be removed when applying --socket-owner fails")
	}
}
