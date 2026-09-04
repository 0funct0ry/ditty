package pty

import "errors"

var (
	// errEmptyArgv is returned by Spawn when Command.Argv has no elements.
	errEmptyArgv = errors.New("pty: empty argv")
	// errNotRoot is returned by Spawn when UID or GID is set but ditty
	// itself is not running as root (SPEC.md §6.5).
	errNotRoot = errors.New("pty: --uid/--gid requires ditty to run as root")
	// errAlreadyDone is returned by Signal once the process has already
	// exited and been reaped.
	errAlreadyDone = errors.New("pty: process already exited")
)
