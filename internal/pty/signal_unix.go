//go:build linux || darwin || freebsd

package pty

import (
	"fmt"
	"os"
	"syscall"
)

// Signal sends sig to the Command's entire process group, not just its
// leader — a negative pid targets the group, so e.g. `bash -c 'sleep 100 &
// wait'` dies fully rather than orphaning the background sleep.
func (p *Process) Signal(sig os.Signal) error {
	select {
	case <-p.done:
		return errAlreadyDone
	default:
	}

	s, ok := sig.(syscall.Signal)
	if !ok {
		return fmt.Errorf("pty: unsupported signal type %T", sig)
	}
	if err := syscall.Kill(-p.pgid, s); err != nil {
		return fmt.Errorf("pty: signal group %d: %w", p.pgid, err)
	}
	return nil
}

// ParseSignal parses a signal name such as "SIGTERM" or "TERM" into an
// os.Signal, for validating flags like --kill-signal.
func ParseSignal(name string) (os.Signal, error) {
	switch name {
	case "SIGHUP", "HUP":
		return syscall.SIGHUP, nil
	case "SIGTERM", "TERM":
		return syscall.SIGTERM, nil
	case "SIGINT", "INT":
		return syscall.SIGINT, nil
	case "SIGKILL", "KILL":
		return syscall.SIGKILL, nil
	case "SIGQUIT", "QUIT":
		return syscall.SIGQUIT, nil
	case "SIGUSR1", "USR1":
		return syscall.SIGUSR1, nil
	case "SIGUSR2", "USR2":
		return syscall.SIGUSR2, nil
	default:
		return nil, fmt.Errorf("pty: unknown signal %q", name)
	}
}
