package session

import (
	"os"

	"github.com/0funct0ry/ditty/internal/pty"
)

// Process is the subset of *pty.Process a Hub drives. Declaring it here,
// rather than depending on *pty.Process directly, lets tests inject a fake
// PTY that records every byte written — the read-only enforcement test
// (SPEC.md §6.1 I1) asserts at this boundary, not at the wire layer.
type Process interface {
	Read(b []byte) (int, error)
	Write(b []byte) (int, error)
	Resize(cols, rows uint16) error
	Signal(sig os.Signal) error
	Close() error
	Wait() (pty.ExitStatus, error)
}

var _ Process = (*pty.Process)(nil)
