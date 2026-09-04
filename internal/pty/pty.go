// Package pty is the only package that touches process and PTY syscalls.
// It knows nothing about Sessions, Clients, or HTTP (SPEC.md §2.1) and must
// not import any other internal/ package — internal/session (M9) wraps a
// *Process to build the real Hub, but pty itself stays a leaf.
package pty

import (
	"os"
	"os/exec"
	"sync"
)

// Command is the immutable description of what ditty runs, per SPEC.md §1:
// argv, cwd, env, TERM, uid/gid and kill signal, plus the initial PTY size.
type Command struct {
	// Argv is the program and its arguments. Argv[0] is resolved via PATH.
	Argv []string
	// Cwd is the child's working directory. Empty inherits ditty's own cwd.
	Cwd string
	// Env holds additional KEY=VAL entries layered onto the parent
	// environment; a key here overrides the same key inherited from the
	// parent. See buildEnv for the exact precedence and DITTY_* handling.
	Env []string
	// Term is the TERM value set for the child. Empty defaults to
	// "xterm-256color".
	Term string
	// UID and GID set the child's credentials via SysProcAttr.Credential.
	// Nil means "don't set". Setting either requires ditty itself to be
	// running as root (SPEC.md §6.5).
	UID, GID *uint32
	// KillSignal is sent to the process group on Close before the 5s
	// SIGKILL escalation. Nil defaults to syscall.SIGHUP (SPEC.md §3.1).
	KillSignal os.Signal
	// Cols and Rows are the initial PTY size. Zero means "not yet fixed by
	// the operator" — Spawn still gives the PTY a concrete default size
	// (80x24) since a PTY always has some size; the real size is expected
	// to arrive via Resize once a Client attaches.
	Cols, Rows uint16
	// SessionID becomes the DITTY_SESSION environment variable.
	SessionID string
}

// ExitStatus reports how a Command's process ended. Exactly one of Code
// (when !Signaled) or Signal (when Signaled) is meaningful. Field names
// deliberately echo the wire ditty.v1 Exit payload's "code"/"signal" keys
// (SPEC.md §5.2) so callers can map an ExitStatus onto that frame directly.
type ExitStatus struct {
	Code     int
	Signal   string
	Signaled bool
}

// Process is a handle to a spawned Command's PTY and process group. Read
// and Write operate on the PTY master; a closed or exited Process returns
// io.EOF (or a *fs.PathError wrapping it) from Read once the master fd is
// closed, which callers should treat as "the process is gone", not a real
// I/O error.
type Process struct {
	ptmx       *os.File
	cmd        *exec.Cmd
	pgid       int
	killSignal os.Signal

	done       chan struct{}
	exitStatus ExitStatus
	waitErr    error

	closeOnce sync.Once
}

// Read reads output from the PTY master.
func (p *Process) Read(b []byte) (int, error) { return p.ptmx.Read(b) }

// Write writes input to the PTY master.
func (p *Process) Write(b []byte) (int, error) { return p.ptmx.Write(b) }

// Wait blocks until the Command's process has exited and been reaped,
// returning its ExitStatus. It is safe to call Wait from multiple
// goroutines and any number of times; only Spawn's own background
// goroutine ever calls the underlying (*exec.Cmd).Wait.
func (p *Process) Wait() (ExitStatus, error) {
	<-p.done
	return p.exitStatus, p.waitErr
}
