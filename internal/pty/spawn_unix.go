//go:build linux || darwin || freebsd

package pty

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

// defaultCols and defaultRows are used when Command.Cols/Rows are zero — a
// PTY always has some size; 0/0 just means "not yet fixed by the operator"
// and the real size is expected to arrive via Resize.
const (
	defaultCols = 80
	defaultRows = 24
)

// Spawn starts Command in a new PTY, in its own session and process group
// (so the whole group, not just the leader, can later be signaled), and
// returns a handle to it. The returned Process is ready to Read/Write
// immediately; a single background goroutine reaps the process and makes
// its ExitStatus available via Wait.
func Spawn(ctx context.Context, c Command) (*Process, error) {
	if len(c.Argv) == 0 {
		return nil, errEmptyArgv
	}

	if (c.UID != nil || c.GID != nil) && os.Geteuid() != 0 {
		return nil, errNotRoot
	}
	if os.Geteuid() == 0 {
		slog.Warn("ditty is running as root; there is no sandbox — consider running the Command inside docker run instead (SPEC.md §6.5)")
	}

	cmd := exec.CommandContext(ctx, c.Argv[0], c.Argv[1:]...)
	cmd.Dir = c.Cwd
	cmd.Env = buildEnv(c)
	// creack/pty's StartWithSize sets Setsid itself (new session, new
	// controlling terminal). setsid(2) already makes the child the leader
	// of a new process group with pgid == pid, so Setpgid must NOT also be
	// requested here — a session leader calling setpgid on itself fails
	// with EPERM. The child is its own process group leader regardless.
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	if c.UID != nil || c.GID != nil {
		cmd.SysProcAttr.Credential = credential(c.UID, c.GID)
	}

	cols, rows := c.Cols, c.Rows
	if cols == 0 || rows == 0 {
		cols, rows = defaultCols, defaultRows
	}
	size := &pty.Winsize{Cols: cols, Rows: rows}

	ptmx, err := pty.StartWithSize(cmd, size)
	if err != nil {
		return nil, fmt.Errorf("pty: spawn %q: %w", c.Argv[0], err)
	}

	// setsid(2) (done by StartWithSize above) makes the child the leader of
	// a new session and, as a POSIX consequence, of a new process group
	// whose pgid equals its own pid — this holds the instant the child is
	// created and is not something the parent needs to (re)query. Calling
	// getpgid(2) here to confirm it is not just redundant but actively
	// racy for a fast-exiting child (e.g. `echo hi`): once the child has
	// exited, some platforms (observed on darwin) can answer getpgid with
	// ESRCH for a small window before the parent has even had a chance to
	// reap it, well before that's a real leak or error condition.
	pgid := cmd.Process.Pid

	killSignal := c.KillSignal
	if killSignal == nil {
		killSignal = syscall.SIGHUP
	}

	p := &Process{
		ptmx:       ptmx,
		cmd:        cmd,
		pgid:       pgid,
		killSignal: killSignal,
		done:       make(chan struct{}),
	}
	go p.reap()
	return p, nil
}

// reap is the sole call site for (*exec.Cmd).Wait: os/exec forbids calling
// Wait more than once, so every other Process method only ever observes
// p.done rather than waiting itself.
func (p *Process) reap() {
	err := p.cmd.Wait()
	state := p.cmd.ProcessState
	if state == nil {
		p.waitErr = err
		close(p.done)
		return
	}

	status, ok := state.Sys().(syscall.WaitStatus)
	if !ok {
		p.exitStatus = ExitStatus{Code: state.ExitCode()}
		close(p.done)
		return
	}
	if status.Signaled() {
		p.exitStatus = ExitStatus{Signaled: true, Signal: status.Signal().String()}
	} else {
		p.exitStatus = ExitStatus{Code: status.ExitStatus()}
	}
	close(p.done)
}
