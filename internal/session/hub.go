package session

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/0funct0ry/ditty/internal/wire"
)

// Session lifecycle states (SPEC.md §3). wire.SessionStateLive/Detached/
// Closed cover the states a Client can observe over the wire; starting and
// awaiting only ever exist before any Client has attached.
const (
	StateStarting = "starting"
	StateAwaiting = "awaiting"
	StateLive     = wire.SessionStateLive
	StateDetached = wire.SessionStateDetached
	StateClosed   = wire.SessionStateClosed
)

// outboxSize is the per-Client buffered send queue (SPEC.md §5.5): a Client
// whose Send falls behind fills this queue and is evicted rather than
// blocking the PTY output pump.
const outboxSize = 256

// evictCloseCode is the WebSocket close code used to evict a slow Client
// (SPEC.md §5.5).
const evictCloseCode = 1011

// rateLimitCloseCode is used to close a Client whose Input has sustained a
// rate-limit breach (SPEC.md §6.6).
const rateLimitCloseCode = 1008

// Options configures a new Hub/Session.
type Options struct {
	// Process is the already-spawned Command this Session drives. Required.
	Process Process

	// ID, Name, Title describe the Session for Hello (SPEC.md §1).
	ID, Name, Title string
	// Cols, Rows are the Session's initial PTY size, as reported in Hello.
	Cols, Rows int
	// Server is the Hello.server value.
	Server string

	// ScrollbackBytes is the ring capacity; 0 disables the ring entirely
	// (--scrollback-bytes). Negative is treated as the default.
	ScrollbackBytes int
	// ChunkBytes is the PTY read buffer size (--chunk-bytes). <=0 uses a
	// sensible default.
	ChunkBytes int
	// FlushInterval coalesces PTY output before fanning it out
	// (--flush-interval). <=0 uses a sensible default.
	FlushInterval time.Duration

	// MaxClients caps simultaneous attachments; 0 means unlimited.
	MaxClients int
	// Once accepts exactly one Client, ever; the Session closes after it
	// detaches (SPEC.md §3.1).
	Once bool
	// WaitForClient closes the Session if no Client ever attaches within
	// this duration; 0 waits forever.
	WaitForClient time.Duration
	// DetachGrace is how long, after the last Client detaches, the Session
	// waits before signalling the Command; 0 signals immediately.
	DetachGrace time.Duration
	// ExitOnDetach is a shorthand for DetachGrace of 0 (SPEC.md §3.1).
	ExitOnDetach bool

	// Profile seeds every Hello.profile; ProfileLock seeds Hello.policy.
	// profileLock (SPEC.md §7, §10.2).
	Profile     json.RawMessage
	ProfileLock bool

	// Clock is the time source for every lifecycle timer. Defaults to
	// NewRealClock() when nil.
	Clock Clock
}

// defaultChunkBytes and defaultFlushInterval match SPEC.md §5.5's defaults.
const (
	defaultChunkBytes    = 32 * 1024
	defaultFlushInterval = 5 * time.Millisecond
)

// Hub owns one PTY-backed Session: the §3 lifecycle state machine, the
// ring buffer, and every attached Client. All Client-visible state is
// mutated only while holding mu; Client.Send is never called while mu is
// held — frames are handed to a per-Client outbox instead, so one slow
// Client can never delay another Client or the PTY output pump.
type Hub struct {
	process Process
	clock   Clock

	id, name, title string
	server          string
	scrollback      int
	chunkBytes      int
	flushInterval   time.Duration
	maxClients      int
	once            bool
	waitForClient   time.Duration
	detachGrace     time.Duration
	exitOnDetach    bool

	mu           sync.Mutex
	ring         *ring
	clients      map[string]*attachedClient
	order        []string // join order, for sizing handover
	sizingID     string
	state        string
	startedAt    time.Time
	cols, rows   int
	closed       bool
	exit         *wire.Exit
	everAttached bool
	profile      json.RawMessage
	profileLock  bool
	waitTimer    Timer
	detachTimer  Timer

	done chan struct{}
}

type attachedClient struct {
	client           Client
	joinedAt         time.Time
	notifiedReadOnly bool
	limiter          *tokenBucket
	outbox           chan []byte
	stopWriter       chan struct{}
}

// NewHub spawns the output pump and lifecycle timers for a freshly
// constructed Session and returns a Hub ready to accept Clients.
func NewHub(opts Options) *Hub {
	clock := opts.Clock
	if clock == nil {
		clock = NewRealClock()
	}

	scrollback := opts.ScrollbackBytes
	if scrollback < 0 {
		scrollback = DefaultScrollbackBytes
	}
	chunkBytes := opts.ChunkBytes
	if chunkBytes <= 0 {
		chunkBytes = defaultChunkBytes
	}
	flushInterval := opts.FlushInterval
	if flushInterval <= 0 {
		flushInterval = defaultFlushInterval
	}

	h := &Hub{
		process:       opts.Process,
		clock:         clock,
		id:            opts.ID,
		name:          opts.Name,
		title:         opts.Title,
		server:        opts.Server,
		scrollback:    scrollback,
		chunkBytes:    chunkBytes,
		flushInterval: flushInterval,
		maxClients:    opts.MaxClients,
		once:          opts.Once,
		waitForClient: opts.WaitForClient,
		detachGrace:   opts.DetachGrace,
		exitOnDetach:  opts.ExitOnDetach,

		ring:        newRing(scrollback),
		clients:     make(map[string]*attachedClient),
		state:       StateStarting,
		startedAt:   clock.Now(),
		cols:        opts.Cols,
		rows:        opts.Rows,
		profile:     opts.Profile,
		profileLock: opts.ProfileLock,
		done:        make(chan struct{}),
	}

	if h.waitForClient > 0 {
		h.state = StateAwaiting
		h.waitTimer = clock.AfterFunc(h.waitForClient, h.onWaitForClientExpired)
	}

	go h.runOutputPump()
	return h
}

// onWaitForClientExpired closes the Session if --wait-for-client elapsed
// with no Client ever attaching.
func (h *Hub) onWaitForClientExpired() {
	h.mu.Lock()
	if h.everAttached || h.closed {
		h.mu.Unlock()
		return
	}
	h.mu.Unlock()
	_ = h.process.Close()
}

// Attach admits c: it sends Hello, then a ring replay if there is one, then
// (if the Session has already closed) the terminal Exit frame, then
// broadcasts the updated Roster. It matches the fixture Hub's replay-on-
// join behaviour for both a first join and a reconnect.
func (h *Hub) Attach(c Client) error {
	h.mu.Lock()

	if h.maxClients > 0 && len(h.clients) >= h.maxClients {
		h.mu.Unlock()
		return &ErrMaxClients{Max: h.maxClients}
	}
	if _, exists := h.clients[c.ID()]; exists {
		// Already attached under this ID: a no-op rather than a duplicate
		// join-order entry, which would desynchronize h.order from
		// h.clients on the next Detach.
		h.mu.Unlock()
		return nil
	}

	joinedAt := h.clock.Now()
	ac := &attachedClient{
		client:     c,
		joinedAt:   joinedAt,
		limiter:    newTokenBucket(inputRateLimitBytesPerSecond, h.clock),
		outbox:     make(chan []byte, outboxSize),
		stopWriter: make(chan struct{}),
	}
	h.clients[c.ID()] = ac
	h.order = append(h.order, c.ID())
	go h.runClientWriter(c.ID(), ac)

	if h.waitTimer != nil {
		h.waitTimer.Stop()
		h.waitTimer = nil
	}
	if h.detachTimer != nil {
		h.detachTimer.Stop()
		h.detachTimer = nil
	}
	h.everAttached = true
	if !h.closed {
		h.state = StateLive
	}

	sizing := h.assignSizingLocked(c)

	hello := wire.Hello{
		Protocol: wire.Subprotocol,
		Server:   h.server,
		Session: wire.HelloSession{
			ID:        h.id,
			Name:      h.name,
			Title:     h.title,
			Cols:      h.cols,
			Rows:      h.rows,
			State:     h.state,
			StartedAt: h.startedAt.Format(time.RFC3339),
		},
		Client: wire.HelloClient{ID: c.ID(), Label: c.Label(), Writable: c.Writable(), Sizing: sizing},
		Policy: wire.HelloPolicy{
			Writable:    c.Writable(),
			Reconnect:   true,
			MaxClients:  h.maxClients,
			ProfileLock: h.profileLock,
		},
		Profile: h.profile,
	}
	h.sendToLocked(ac, wire.OpHello, hello, false)

	if snap := h.ring.Snapshot(); snap != nil {
		h.sendToLocked(ac, wire.OpOutput, snap, true)
	}

	if h.closed && h.exit != nil {
		h.sendToLocked(ac, wire.OpExit, *h.exit, false)
	}

	evict := h.broadcastRosterLocked()
	h.mu.Unlock()

	h.evictAll(evict)
	return nil
}

// assignSizingLocked assigns the sizing role per SPEC.md §5.4: the first
// Client ever attached sizes by default, but a writable Client pre-empts a
// non-writable incumbent. Must be called with h.mu held; c must already be
// registered in h.clients.
func (h *Hub) assignSizingLocked(c Client) bool {
	if h.sizingID == "" {
		h.sizingID = c.ID()
		return true
	}
	if !c.Writable() {
		return false
	}
	if cur, ok := h.clients[h.sizingID]; ok && !cur.client.Writable() {
		h.sizingID = c.ID()
		h.enqueueLocked(cur, encodeOrNil(wire.OpState, wire.State{
			Sizing: false, State: h.state, Reason: "sizing handover",
		}))
		return true
	}
	return false
}

// Detach removes id, handing the sizing role to the next Client by join
// order if id held it (SPEC.md §5.4), and arms the detach-grace timer once
// the last Client leaves.
func (h *Hub) Detach(id string) {
	h.mu.Lock()

	ac, ok := h.clients[id]
	if !ok {
		h.mu.Unlock()
		return
	}
	delete(h.clients, id)
	close(ac.stopWriter)
	for i, oid := range h.order {
		if oid == id {
			h.order = append(h.order[:i], h.order[i+1:]...)
			break
		}
	}

	if h.sizingID == id {
		h.sizingID = ""
		if len(h.order) > 0 {
			h.sizingID = h.order[0]
			if next, ok := h.clients[h.sizingID]; ok {
				h.enqueueLocked(next, encodeOrNil(wire.OpState, wire.State{
					Sizing: true, State: h.state, Reason: "sizing handover",
				}))
			}
		}
	}

	var shouldClose bool
	if len(h.clients) == 0 && !h.closed {
		h.state = StateDetached
		if h.once {
			shouldClose = true
		} else {
			grace := h.detachGrace
			if h.exitOnDetach {
				grace = 0
			}
			h.detachTimer = h.clock.AfterFunc(grace, h.onDetachGraceExpired)
		}
	}

	evict := h.broadcastRosterLocked()
	h.mu.Unlock()

	h.evictAll(evict)
	if shouldClose {
		_ = h.process.Close()
	}
}

// onDetachGraceExpired signals the Command once --detach-grace has elapsed
// with no Client re-attached.
func (h *Hub) onDetachGraceExpired() {
	h.mu.Lock()
	stillEmpty := len(h.clients) == 0 && !h.closed
	h.mu.Unlock()
	if stillEmpty {
		_ = h.process.Close()
	}
}

// Input forwards data from Client id to the PTY, unless id is not writable
// (SPEC.md §6.1 I1: dropped here, before the PTY, not in the transport or
// UI) or has exceeded its input rate limit (SPEC.md §6.6).
func (h *Hub) Input(id string, data []byte) {
	h.mu.Lock()
	ac, ok := h.clients[id]
	if !ok {
		h.mu.Unlock()
		return
	}

	if !ac.client.Writable() {
		notify := !ac.notifiedReadOnly
		ac.notifiedReadOnly = true
		if notify {
			h.enqueueLocked(ac, encodeOrNil(wire.OpNotice, wire.Notice{
				Level: wire.NoticeLevelInfo, Message: "This session is read-only",
			}))
		}
		h.mu.Unlock()
		return
	}

	if !ac.limiter.Allow(len(data)) {
		sustained := ac.limiter.Sustained(rateLimitSustainedBreach)
		h.enqueueLocked(ac, encodeOrNil(wire.OpNotice, wire.Notice{
			Level: wire.NoticeLevelWarn, Message: "Input rate limit exceeded",
		}))
		h.mu.Unlock()
		if sustained {
			ac.client.Close(rateLimitCloseCode, "input rate limit exceeded")
			h.Detach(id)
		}
		return
	}
	h.mu.Unlock()

	_, _ = h.process.Write(data)
}

// Resize applies a resize from Client id, clamped into the wire protocol's
// valid range, but only when id is the current sizing client (SPEC.md
// §5.4); otherwise it is ignored.
func (h *Hub) Resize(id string, cols, rows int) {
	h.mu.Lock()
	if id != h.sizingID {
		h.mu.Unlock()
		return
	}
	clamped := wire.Resize{Cols: cols, Rows: rows}.Clamp()
	h.cols, h.rows = clamped.Cols, clamped.Rows
	h.mu.Unlock()

	_ = h.process.Resize(uint16(clamped.Cols), uint16(clamped.Rows))
}

// Roster returns a snapshot of the currently attached Clients, in join
// order.
func (h *Hub) Roster() wire.Roster {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.rosterLocked()
}

func (h *Hub) rosterLocked() wire.Roster {
	roster := wire.Roster{Count: len(h.order)}
	for _, id := range h.order {
		ac := h.clients[id]
		roster.Clients = append(roster.Clients, wire.RosterClient{
			ID:       id,
			Label:    ac.client.Label(),
			Writable: ac.client.Writable(),
			JoinedAt: ac.joinedAt.Format(time.RFC3339),
		})
	}
	return roster
}

// State returns the Session's current lifecycle state (SPEC.md §3).
func (h *Hub) State() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.state
}

// broadcastRosterLocked must be called with h.mu held. It returns the IDs
// of any Clients whose outbox was full and must be evicted once the caller
// releases h.mu.
func (h *Hub) broadcastRosterLocked() []string {
	frame := encodeOrNil(wire.OpRoster, h.rosterLocked())
	return h.fanOutLocked(frame)
}

// fanOutLocked enqueues frame onto every attached Client's outbox and
// returns the IDs of any whose outbox was already full.
func (h *Hub) fanOutLocked(frame []byte) []string {
	if frame == nil {
		return nil
	}
	var evict []string
	for id, ac := range h.clients {
		if !h.enqueueLocked(ac, frame) {
			evict = append(evict, id)
		}
	}
	return evict
}

// sendToLocked enqueues a single frame to one Client. If raw is true,
// payload is written verbatim (OpOutput); otherwise it is JSON-encoded.
func (h *Hub) sendToLocked(ac *attachedClient, op wire.Opcode, payload any, raw bool) {
	var frame []byte
	if raw {
		frame = encodeOrNil(op, payload.([]byte))
	} else {
		frame = encodeOrNil(op, payload)
	}
	h.enqueueLocked(ac, frame)
}

// enqueueLocked places frame on ac's outbox without blocking. It reports
// false (and leaves frame undelivered) if the outbox is already full,
// meaning ac is a slow Client that must be evicted.
func (h *Hub) enqueueLocked(ac *attachedClient, frame []byte) bool {
	if frame == nil {
		return true
	}
	select {
	case ac.outbox <- frame:
		return true
	default:
		return false
	}
}

// evictAll closes and detaches every Client in ids (SPEC.md §5.5: a slow
// Client is evicted with close code 1011). Must be called without h.mu
// held.
func (h *Hub) evictAll(ids []string) {
	for _, id := range ids {
		h.mu.Lock()
		ac, ok := h.clients[id]
		h.mu.Unlock()
		if !ok {
			continue
		}
		ac.client.Close(evictCloseCode, "slow client")
		h.Detach(id)
	}
}

// runClientWriter is the sole goroutine that ever calls ac.client.Send, so
// a Client whose Send blocks forever only ever blocks its own writer, never
// the Hub or any other Client.
func (h *Hub) runClientWriter(id string, ac *attachedClient) {
	for {
		select {
		case frame := <-ac.outbox:
			if err := ac.client.Send(frame); err != nil {
				h.Detach(id)
				return
			}
		case <-ac.stopWriter:
			return
		}
	}
}

// runOutputPump reads from the PTY, coalesces output over flushInterval
// (or chunkBytes, whichever comes first), writes it to the ring, and fans
// it out to every attached Client. It runs until the PTY is gone, at which
// point it reports the Session's exit.
func (h *Hub) runOutputPump() {
	reads := make(chan []byte, 16)
	go func() {
		buf := make([]byte, h.chunkBytes)
		for {
			n, err := h.process.Read(buf)
			if n > 0 {
				cp := make([]byte, n)
				copy(cp, buf[:n])
				reads <- cp
			}
			if err != nil {
				close(reads)
				return
			}
		}
	}()

	ticker := time.NewTicker(h.flushInterval)
	defer ticker.Stop()

	var pending []byte
	flush := func() {
		if len(pending) == 0 {
			return
		}
		data := pending
		pending = nil
		h.mu.Lock()
		h.ring.Write(data)
		evict := h.fanOutLocked(encodeOrNil(wire.OpOutput, data))
		h.mu.Unlock()
		h.evictAll(evict)
	}

	for {
		select {
		case data, ok := <-reads:
			if !ok {
				flush()
				h.handleProcessExit()
				return
			}
			pending = append(pending, data...)
			if len(pending) >= h.chunkBytes {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// handleProcessExit runs once the PTY reports EOF: it waits for the
// Command's real exit status, transitions the Session to closed, and
// broadcasts the terminal Exit frame.
func (h *Hub) handleProcessExit() {
	status, _ := h.process.Wait()
	exit := wire.Exit{Code: status.Code, Signal: status.Signal}

	h.mu.Lock()
	h.closed = true
	h.state = StateClosed
	h.exit = &exit
	if h.waitTimer != nil {
		h.waitTimer.Stop()
		h.waitTimer = nil
	}
	if h.detachTimer != nil {
		h.detachTimer.Stop()
		h.detachTimer = nil
	}
	evict := h.fanOutLocked(encodeOrNil(wire.OpExit, exit))
	h.mu.Unlock()

	h.evictAll(evict)
	close(h.done)
}

// Done is closed once the Session's Command has exited and its terminal
// Exit frame has been broadcast.
func (h *Hub) Done() <-chan struct{} { return h.done }

func encodeOrNil(op wire.Opcode, payload any) []byte {
	frame, err := wire.Encode(op, payload)
	if err != nil {
		return nil
	}
	return frame
}
