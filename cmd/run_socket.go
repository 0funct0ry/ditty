package cmd

import (
	"fmt"
	"net"
	"os"
	"os/user"
	"strconv"
	"strings"

	"github.com/spf13/pflag"

	"github.com/0funct0ry/ditty/internal/config"
)

// socketConfig holds the resolved --socket/--socket-mode/--socket-owner
// flags (SPEC.md §8.1). path is empty when --socket was not passed, in
// which case ditty listens on --address/--port instead (cmd/run.go).
type socketConfig struct {
	path  string
	mode  os.FileMode
	owner string
}

// resolveSocketConfig parses and validates --socket/--socket-mode/
// --socket-owner, enforcing that --socket is mutually exclusive with
// --address/--port (SPEC.md §8.1: "Unix socket; mutually exclusive with
// address/port"). fs is needed only for Changed — config.Resolver doesn't
// expose whether a flag was explicitly set versus left at its default.
func resolveSocketConfig(fs *pflag.FlagSet, resolver *config.Resolver) (socketConfig, error) {
	path := resolver.String("socket")
	if path == "" {
		return socketConfig{}, nil
	}
	if fs.Changed("address") || fs.Changed("port") {
		return socketConfig{}, fmt.Errorf("--socket cannot be combined with --address or --port")
	}
	modeStr := resolver.String("socket-mode")
	mode, err := strconv.ParseUint(modeStr, 8, 32)
	if err != nil {
		return socketConfig{}, fmt.Errorf("--socket-mode %q: %w", modeStr, err)
	}
	return socketConfig{
		path:  path,
		mode:  os.FileMode(mode),
		owner: resolver.String("socket-owner"),
	}, nil
}

// listenUnix creates the Unix socket listener at cfg.path, removing any
// stale socket file an unclean previous exit left behind first (a fresh
// bind to an existing path otherwise fails with "address already in use"),
// then applies --socket-mode and --socket-owner.
func listenUnix(cfg socketConfig) (net.Listener, error) {
	if err := os.Remove(cfg.path); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("remove stale socket %s: %w", cfg.path, err)
	}
	ln, err := net.Listen("unix", cfg.path)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", cfg.path, err)
	}
	if err := os.Chmod(cfg.path, cfg.mode); err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("chmod %s: %w", cfg.path, err)
	}
	if cfg.owner != "" {
		uid, gid, err := lookupOwner(cfg.owner)
		if err != nil {
			_ = ln.Close()
			return nil, fmt.Errorf("--socket-owner %q: %w", cfg.owner, err)
		}
		if err := os.Chown(cfg.path, uid, gid); err != nil {
			_ = ln.Close()
			return nil, fmt.Errorf("chown %s: %w", cfg.path, err)
		}
	}
	return ln, nil
}

// lookupOwner parses "user" or "user:group" and resolves each to a numeric
// id. An omitted group leaves gid as -1, os.Chown's "leave unchanged" value.
func lookupOwner(spec string) (uid, gid int, err error) {
	userName, groupName, _ := strings.Cut(spec, ":")
	u, err := user.Lookup(userName)
	if err != nil {
		return 0, 0, err
	}
	uid, err = strconv.Atoi(u.Uid)
	if err != nil {
		return 0, 0, err
	}
	gid = -1
	if groupName != "" {
		g, err := user.LookupGroup(groupName)
		if err != nil {
			return 0, 0, err
		}
		gid, err = strconv.Atoi(g.Gid)
		if err != nil {
			return 0, 0, err
		}
	}
	return uid, gid, nil
}
