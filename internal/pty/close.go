//go:build linux || darwin || freebsd

package pty

import (
	"syscall"
	"time"
)

// closeGrace is how long Close waits after sending the configured kill
// signal before escalating to SIGKILL (SPEC.md §3.1).
const closeGrace = 5 * time.Second

// Close signals the Command's process group to exit — first with the
// configured kill signal, then, after closeGrace, with SIGKILL — and
// closes the PTY master. It always reaps: Close never returns while the
// process is still alive. Calling Close more than once is safe; only the
// first call has any effect.
func (p *Process) Close() error {
	var closeErr error
	p.closeOnce.Do(func() {
		select {
		case <-p.done:
		default:
			_ = p.Signal(p.killSignal)

			timer := time.NewTimer(closeGrace)
			defer timer.Stop()

			select {
			case <-p.done:
			case <-timer.C:
				_ = syscall.Kill(-p.pgid, syscall.SIGKILL)
				<-p.done
			}
		}
		closeErr = p.ptmx.Close()
	})
	return closeErr
}
