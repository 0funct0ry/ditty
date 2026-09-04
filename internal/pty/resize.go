//go:build linux || darwin || freebsd

package pty

import (
	"fmt"

	"github.com/creack/pty"
)

// Resize sets the PTY's window size. cols and rows must both be non-zero;
// clamping raw client-submitted values to the wire protocol's valid range
// (SPEC.md §5.1) is the caller's responsibility, not this package's.
func (p *Process) Resize(cols, rows uint16) error {
	if cols == 0 || rows == 0 {
		return fmt.Errorf("pty: resize: cols and rows must be > 0")
	}
	return pty.Setsize(p.ptmx, &pty.Winsize{Cols: cols, Rows: rows})
}
