package session

import (
	"io"
	"os"
	"sync"

	"github.com/0funct0ry/ditty/internal/pty"
)

// fakeProcess is a Process double standing in for a real PTY: Read serves
// scripted output chunks fed via feed/close, and Write records every byte
// it is asked to write so read-only enforcement can be asserted at the PTY
// boundary itself (SPEC.md §6.1 I1).
type fakeProcess struct {
	// out and eof are never closed for concurrent-send safety; feed sends
	// on out, exitNow closes eof, and Read selects on both so a feed
	// racing an exit never sends on (or closes) a channel another
	// goroutine might close (or send on) concurrently.
	out chan []byte
	eof chan struct{}

	mu        sync.Mutex
	writes    [][]byte
	resizes   [][2]uint16
	signals   []os.Signal
	closed    bool
	exit      pty.ExitStatus
	waitDone  chan struct{}
	closeOnce sync.Once
}

func newFakeProcess() *fakeProcess {
	return &fakeProcess{
		out:      make(chan []byte, 64),
		eof:      make(chan struct{}),
		waitDone: make(chan struct{}),
	}
}

// feed makes p a []byte available to the next Read call. It is a no-op
// once the process has exited.
func (p *fakeProcess) feed(data []byte) {
	select {
	case p.out <- data:
	case <-p.eof:
	}
}

// exitNow makes Read return io.EOF and Wait return status, as if the
// Command had just exited.
func (p *fakeProcess) exitNow(status pty.ExitStatus) {
	p.closeOnce.Do(func() {
		p.mu.Lock()
		p.exit = status
		p.mu.Unlock()
		close(p.eof)
		close(p.waitDone)
	})
}

func (p *fakeProcess) Read(b []byte) (int, error) {
	select {
	case chunk := <-p.out:
		n := copy(b, chunk)
		return n, nil
	case <-p.eof:
		select {
		case chunk := <-p.out:
			n := copy(b, chunk)
			return n, nil
		default:
			return 0, io.EOF
		}
	}
}

func (p *fakeProcess) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	cp := make([]byte, len(b))
	copy(cp, b)
	p.writes = append(p.writes, cp)
	return len(b), nil
}

func (p *fakeProcess) Resize(cols, rows uint16) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.resizes = append(p.resizes, [2]uint16{cols, rows})
	return nil
}

func (p *fakeProcess) Signal(sig os.Signal) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.signals = append(p.signals, sig)
	return nil
}

func (p *fakeProcess) Close() error {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	p.exitNow(pty.ExitStatus{Signaled: true, Signal: "killed"})
	return nil
}

func (p *fakeProcess) Wait() (pty.ExitStatus, error) {
	<-p.waitDone
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.exit, nil
}

func (p *fakeProcess) writtenBytes() [][]byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([][]byte, len(p.writes))
	copy(out, p.writes)
	return out
}

func (p *fakeProcess) resizeCalls() [][2]uint16 {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([][2]uint16, len(p.resizes))
	copy(out, p.resizes)
	return out
}
