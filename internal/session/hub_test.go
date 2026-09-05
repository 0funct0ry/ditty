package session

import (
	"bytes"
	"errors"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/0funct0ry/ditty/internal/pty"
	"github.com/0funct0ry/ditty/internal/wire"
)

func decodeAll(t *testing.T, frames [][]byte) []struct {
	Op      wire.Opcode
	Payload []byte
} {
	t.Helper()
	out := make([]struct {
		Op      wire.Opcode
		Payload []byte
	}, 0, len(frames))
	for _, f := range frames {
		op, payload, err := wire.Decode(f)
		if err != nil {
			t.Fatalf("decode frame: %v", err)
		}
		out = append(out, struct {
			Op      wire.Opcode
			Payload []byte
		}{op, payload})
	}
	return out
}

func waitUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	if !cond() {
		t.Fatalf("condition not met within %s", timeout)
	}
}

func newTestHub(t *testing.T, p *fakeProcess, opts Options) *Hub {
	t.Helper()
	opts.Process = p
	if opts.FlushInterval == 0 {
		opts.FlushInterval = time.Millisecond
	}
	if opts.Server == "" {
		opts.Server = "ditty-test"
	}
	if opts.ScrollbackBytes == 0 {
		opts.ScrollbackBytes = DefaultScrollbackBytes
	}
	return NewHub(opts)
}

func outputBytes(t *testing.T, frames [][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	for _, f := range decodeAll(t, frames) {
		if f.Op == wire.OpOutput {
			buf.Write(f.Payload)
		}
	}
	return buf.Bytes()
}

// --- internal/fixture/PARITY.md, reproduced against the real Hub ---

func TestHub_ScrollbackZero_DisablesRing(t *testing.T) {
	p := newFakeProcess()
	// newTestHub's ScrollbackBytes default only fills in the zero value,
	// which is indistinguishable from an explicit --scrollback-bytes 0, so
	// this test constructs the Hub directly instead.
	h := NewHub(Options{Process: p, Name: "deploy", ScrollbackBytes: 0, FlushInterval: time.Millisecond, Server: "ditty-test"})
	p.feed([]byte("hello"))

	c := newFakeClient("c1")
	time.Sleep(20 * time.Millisecond) // let the pump flush before attaching
	if err := h.Attach(c); err != nil {
		t.Fatalf("Attach: %v", err)
	}
	out := outputBytes(t, c.snapshot())
	if bytes.Contains(out, []byte("hello")) {
		t.Fatalf("--scrollback-bytes 0 should retain nothing, replay contained %q", out)
	}
}

func TestHub_JoinReplay_FirstAttach(t *testing.T) {
	p := newFakeProcess()
	h := newTestHub(t, p, Options{Name: "deploy"})
	p.feed([]byte("hello"))

	c := newFakeClient("c1")
	if err := h.Attach(c); err != nil {
		t.Fatalf("Attach: %v", err)
	}

	waitUntil(t, time.Second, func() bool {
		frames := decodeAll(t, c.snapshot())
		return len(frames) > 0 && frames[0].Op == wire.OpHello
	})
	waitUntil(t, time.Second, func() bool {
		return bytes.Contains(outputBytes(t, c.snapshot()), []byte("hello"))
	})
}

func TestHub_MidSessionJoin(t *testing.T) {
	p := newFakeProcess()
	h := newTestHub(t, p, Options{Name: "deploy"})
	p.feed([]byte("chunk one"))
	p.feed([]byte("chunk two"))

	waitUntil(t, time.Second, func() bool {
		h.mu.Lock()
		defer h.mu.Unlock()
		return bytes.Contains(h.ring.Snapshot(), []byte("chunk two"))
	})

	c := newFakeClient("late")
	if err := h.Attach(c); err != nil {
		t.Fatalf("Attach: %v", err)
	}
	waitUntil(t, time.Second, func() bool {
		out := outputBytes(t, c.snapshot())
		return bytes.Contains(out, []byte("chunk one")) && bytes.Contains(out, []byte("chunk two"))
	})
}

func TestHub_Resize_OnlySizingClientAccepted(t *testing.T) {
	p := newFakeProcess()
	h := newTestHub(t, p, Options{Name: "deploy"})

	c1 := newFakeClient("c1")
	c2 := newFakeClient("c2")
	_ = h.Attach(c1)
	_ = h.Attach(c2)

	h.Resize("c2", 100, 40) // not sizing; ignored
	if len(p.resizeCalls()) != 0 {
		t.Fatalf("non-sizing Resize should be ignored, got %v", p.resizeCalls())
	}

	h.Resize("c1", 100, 40) // sizing; accepted and clamped
	calls := p.resizeCalls()
	if len(calls) != 1 || calls[0] != [2]uint16{100, 40} {
		t.Fatalf("sizing Resize = %v, want one call {100,40}", calls)
	}
}

func TestHub_DisconnectReconnect_ReplaysEverything(t *testing.T) {
	p := newFakeProcess()
	h := newTestHub(t, p, Options{Name: "flaky"})
	p.feed([]byte("before drop"))
	waitUntil(t, time.Second, func() bool {
		h.mu.Lock()
		defer h.mu.Unlock()
		return bytes.Contains(h.ring.Snapshot(), []byte("before drop"))
	})

	c := newFakeClient("c1")
	_ = h.Attach(c)
	h.Detach("c1") // simulate a dropped connection

	p.feed([]byte("during drop"))
	waitUntil(t, time.Second, func() bool {
		h.mu.Lock()
		defer h.mu.Unlock()
		return bytes.Contains(h.ring.Snapshot(), []byte("during drop"))
	})

	reconnect := newFakeClient("c1")
	if err := h.Attach(reconnect); err != nil {
		t.Fatalf("Attach reconnect: %v", err)
	}
	waitUntil(t, time.Second, func() bool {
		out := outputBytes(t, reconnect.snapshot())
		return bytes.Contains(out, []byte("before drop")) && bytes.Contains(out, []byte("during drop"))
	})
}

func TestHub_ExitCode(t *testing.T) {
	p := newFakeProcess()
	h := newTestHub(t, p, Options{Name: "quickexit"})
	c := newFakeClient("c1")
	_ = h.Attach(c)

	p.exitNow(pty.ExitStatus{Code: 1})
	<-h.Done()

	var exit wire.Exit
	waitUntil(t, time.Second, func() bool {
		for _, f := range decodeAll(t, c.snapshot()) {
			if f.Op == wire.OpExit {
				var err error
				exit, err = wire.DecodeExit(f.Payload)
				if err != nil {
					t.Fatalf("decode Exit: %v", err)
				}
				return true
			}
		}
		return false
	})
	if exit.Code != 1 {
		t.Fatalf("Exit.Code = %d, want 1", exit.Code)
	}
	if h.State() != StateClosed {
		t.Fatalf("State() = %q, want closed", h.State())
	}

	late := newFakeClient("late")
	if err := h.Attach(late); err != nil {
		t.Fatalf("Attach late: %v", err)
	}
	waitUntil(t, time.Second, func() bool {
		for _, f := range decodeAll(t, late.snapshot()) {
			if f.Op == wire.OpExit {
				return true
			}
		}
		return false
	})
}

func TestHub_ExitSignal(t *testing.T) {
	p := newFakeProcess()
	h := newTestHub(t, p, Options{Name: "signalled"})
	c := newFakeClient("c1")
	_ = h.Attach(c)

	p.exitNow(pty.ExitStatus{Signaled: true, Signal: "terminated"})
	<-h.Done()

	var exit wire.Exit
	waitUntil(t, time.Second, func() bool {
		for _, f := range decodeAll(t, c.snapshot()) {
			if f.Op == wire.OpExit {
				exit, _ = wire.DecodeExit(f.Payload)
				return true
			}
		}
		return false
	})
	if exit.Signal != "terminated" {
		t.Fatalf("Exit.Signal = %q, want terminated", exit.Signal)
	}
}

func TestHub_RosterJoinLeave(t *testing.T) {
	p := newFakeProcess()
	h := newTestHub(t, p, Options{Name: "deploy"})
	c1 := newFakeClient("c1")
	c2 := newFakeClient("c2")
	_ = h.Attach(c1)
	_ = h.Attach(c2)

	waitUntil(t, time.Second, func() bool {
		for _, f := range decodeAll(t, c1.snapshot()) {
			if f.Op == wire.OpRoster {
				r, _ := wire.DecodeRoster(f.Payload)
				if r.Count == 2 {
					return true
				}
			}
		}
		return false
	})

	h.Detach("c2")
	waitUntil(t, time.Second, func() bool {
		return h.Roster().Count == 1
	})
}

func TestHub_SizingHandover(t *testing.T) {
	p := newFakeProcess()
	h := newTestHub(t, p, Options{Name: "deploy"})
	c1 := newFakeClient("c1")
	c2 := newFakeClient("c2")
	c3 := newFakeClient("c3")
	_ = h.Attach(c1)
	_ = h.Attach(c2)
	_ = h.Attach(c3)

	hello1, err := wire.DecodeHello(decodeAll(t, c1.snapshot())[0].Payload)
	if err != nil {
		t.Fatalf("decode Hello c1: %v", err)
	}
	if !hello1.Client.Sizing {
		t.Fatal("c1 should be sizing client on first join")
	}

	h.Detach("c1")

	waitUntil(t, time.Second, func() bool {
		for _, f := range decodeAll(t, c2.snapshot()) {
			if f.Op == wire.OpState {
				st, err := wire.DecodeState(f.Payload)
				if err == nil && st.Sizing {
					return true
				}
			}
		}
		return false
	})
}

func TestHub_ReadOnly_DropsInputWithOneNotice(t *testing.T) {
	p := newFakeProcess()
	h := newTestHub(t, p, Options{Name: "deploy"})
	c := newFakeClient("c1") // not writable
	_ = h.Attach(c)

	h.Input("c1", []byte("ls\n"))
	h.Input("c1", []byte("pwd\n"))

	waitUntil(t, time.Second, func() bool {
		n := 0
		for _, f := range decodeAll(t, c.snapshot()) {
			if f.Op == wire.OpNotice {
				n++
			}
		}
		return n == 1
	})
	if len(p.writtenBytes()) != 0 {
		t.Fatalf("read-only Input reached the PTY: %v", p.writtenBytes())
	}
}

func TestHub_ReadOnly_EnforcedAtPTYUnderConcurrentInput(t *testing.T) {
	p := newFakeProcess()
	h := newTestHub(t, p, Options{Name: "deploy"})
	ro := newFakeClient("ro")
	rw := newWritableFakeClient("rw")
	_ = h.Attach(ro)
	_ = h.Attach(rw)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); h.Input("ro", []byte("x")) }()
		go func() { defer wg.Done(); h.Input("rw", []byte("y")) }()
	}
	wg.Wait()

	for _, w := range p.writtenBytes() {
		if bytes.ContainsRune(w, 'x') {
			t.Fatalf("read-only Client's bytes reached the PTY: %q", w)
		}
	}
}

func TestHub_MaxClients_RejectsOverLimit(t *testing.T) {
	p := newFakeProcess()
	h := newTestHub(t, p, Options{Name: "deploy", MaxClients: 1})
	c1 := newFakeClient("c1")
	if err := h.Attach(c1); err != nil {
		t.Fatalf("Attach c1: %v", err)
	}
	c2 := newFakeClient("c2")
	err := h.Attach(c2)
	var maxErr *ErrMaxClients
	if !errors.As(err, &maxErr) {
		t.Fatalf("Attach c2 error = %v, want *ErrMaxClients", err)
	}
	code, reason := maxErr.CloseCode()
	if code != 1013 || reason == "" {
		t.Fatalf("CloseCode() = (%d, %q), want (1013, non-empty)", code, reason)
	}
}

func TestHub_SlowClient_EvictedWithoutDelayingOthers(t *testing.T) {
	p := newFakeProcess()
	// ChunkBytes: 1 forces every single-byte read to flush as its own
	// Output frame immediately, instead of coalescing into one frame over
	// --flush-interval — this test needs one frame per feed to overflow
	// the 256-frame outbox.
	h := newTestHub(t, p, Options{Name: "deploy", ChunkBytes: 1})
	slow := newFakeClient("slow")
	fast := newFakeClient("fast")
	_ = h.Attach(slow)
	_ = h.Attach(fast)
	slow.block()
	defer slow.unblock()

	for i := 0; i < outboxSize+10; i++ {
		p.feed([]byte("x"))
	}

	start := time.Now()
	waitUntil(t, time.Second, func() bool { return slow.isClosed() })
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("slow client eviction took %s, want well under 500ms", elapsed)
	}

	waitUntil(t, 100*time.Millisecond, func() bool {
		return bytes.Contains(outputBytes(t, fast.snapshot()), []byte("x"))
	})
}

func TestHub_RateLimit_SustainedBreachCloses(t *testing.T) {
	clock := newFakeClock()
	p := newFakeProcess()
	h := newTestHub(t, p, Options{Name: "deploy", Clock: clock})
	c := newWritableFakeClient("c1")
	_ = h.Attach(c)

	big := bytes.Repeat([]byte("a"), inputRateLimitBytesPerSecond*2)
	h.Input("c1", big) // first breach: notice, no close yet
	if c.isClosed() {
		t.Fatal("closed on first breach, want a warning first")
	}

	clock.Advance(rateLimitSustainedBreach + time.Millisecond)
	h.Input("c1", big) // still over budget after the sustained window
	waitUntil(t, time.Second, func() bool { return c.isClosed() })
}

func TestHub_Once_ClosesAfterSingleDetach(t *testing.T) {
	p := newFakeProcess()
	h := newTestHub(t, p, Options{Name: "deploy", Once: true})
	c := newFakeClient("c1")
	_ = h.Attach(c)
	h.Detach("c1")

	waitUntil(t, time.Second, func() bool {
		p.mu.Lock()
		defer p.mu.Unlock()
		return p.closed
	})
}

func TestHub_LifecycleTimers_FakeClock(t *testing.T) {
	t.Run("wait-for-client expiry closes with no attach", func(t *testing.T) {
		clock := newFakeClock()
		p := newFakeProcess()
		h := newTestHub(t, p, Options{Name: "deploy", WaitForClient: 5 * time.Second, Clock: clock})
		if h.State() != StateAwaiting {
			t.Fatalf("State() = %q, want awaiting", h.State())
		}
		clock.Advance(5 * time.Second)
		waitUntil(t, time.Second, func() bool {
			p.mu.Lock()
			defer p.mu.Unlock()
			return p.closed
		})
	})

	t.Run("attach before wait-for-client cancels the timer", func(t *testing.T) {
		clock := newFakeClock()
		p := newFakeProcess()
		h := newTestHub(t, p, Options{Name: "deploy", WaitForClient: 5 * time.Second, Clock: clock})
		_ = h.Attach(newFakeClient("c1"))
		clock.Advance(10 * time.Second)
		time.Sleep(20 * time.Millisecond)
		p.mu.Lock()
		closed := p.closed
		p.mu.Unlock()
		if closed {
			t.Fatal("process closed despite a Client attaching before the deadline")
		}
	})

	t.Run("detach-grace signals after the last Client leaves", func(t *testing.T) {
		clock := newFakeClock()
		p := newFakeProcess()
		h := newTestHub(t, p, Options{Name: "deploy", DetachGrace: 10 * time.Second, Clock: clock})
		_ = h.Attach(newFakeClient("c1"))
		h.Detach("c1")
		if h.State() != StateDetached {
			t.Fatalf("State() = %q, want detached", h.State())
		}
		clock.Advance(10 * time.Second)
		waitUntil(t, time.Second, func() bool {
			p.mu.Lock()
			defer p.mu.Unlock()
			return p.closed
		})
	})

	t.Run("re-attach during detach-grace cancels the timer", func(t *testing.T) {
		clock := newFakeClock()
		p := newFakeProcess()
		h := newTestHub(t, p, Options{Name: "deploy", DetachGrace: 10 * time.Second, Clock: clock})
		_ = h.Attach(newFakeClient("c1"))
		h.Detach("c1")
		_ = h.Attach(newFakeClient("c1"))
		clock.Advance(20 * time.Second)
		time.Sleep(20 * time.Millisecond)
		p.mu.Lock()
		closed := p.closed
		p.mu.Unlock()
		if closed {
			t.Fatal("process closed despite a re-attach during detach-grace")
		}
	})
}

// TestHub20ClientsRace attaches, types, resizes and detaches 20 Clients
// concurrently for a short window under -race: no races, no deadlocks, and
// every goroutine this test starts winds down.
func TestHub20ClientsRace(t *testing.T) {
	p := newFakeProcess()
	// A long DetachGrace keeps a transient all-clients-detached moment
	// (routine during this test's random churn) from closing the process
	// mid-run; lifecycle timer behaviour has its own dedicated tests.
	h := newTestHub(t, p, Options{Name: "deploy", DetachGrace: time.Hour})

	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				p.feed([]byte("tick"))
				time.Sleep(time.Millisecond)
			}
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := "race-client"
			r := rand.New(rand.NewSource(int64(n)))
			c := newWritableFakeClient(id + string(rune('a'+n)))
			deadline := time.Now().Add(300 * time.Millisecond)
			for time.Now().Before(deadline) {
				switch r.Intn(4) {
				case 0:
					_ = h.Attach(c)
				case 1:
					h.Input(c.ID(), []byte("x"))
				case 2:
					h.Resize(c.ID(), 80+r.Intn(20), 24+r.Intn(10))
				case 3:
					h.Detach(c.ID())
				}
			}
			h.Detach(c.ID())
		}(i)
	}
	wg.Wait()
	close(stop)
}
