package fixture

import (
	"sync"
	"time"

	"github.com/0funct0ry/ditty/internal/wire"
)

// Hub plays one Scenario to every attached Client, reproducing the real
// Hub's join replay, roster, sizing and exit behaviour (SPEC.md §3-§5)
// against scripted data instead of a live PTY.
type Hub struct {
	mu        sync.Mutex
	scenario  Scenario
	scale     float64
	ring      *ring
	clients   map[string]*attachedClient
	order     []string // join order, for sizing handover
	sizingID  string
	state     string
	startedAt time.Time
	closed    bool
	exit      *wire.Exit

	done chan struct{}
}

type attachedClient struct {
	client           Client
	joinedAt         time.Time
	notifiedReadOnly bool
}

// NewHub starts scenario playing in real time and returns a Hub ready to
// accept Clients.
func NewHub(scenario Scenario) *Hub {
	return newHub(scenario, 1)
}

// NewHubScaled starts scenario playing with every timing multiplied by
// scale (e.g. 0.01 turns a 5s exit into 50ms). It exists so tests can
// exercise a full scenario without waiting on its real-time schedule; the
// production path always uses NewHub.
func NewHubScaled(scenario Scenario, scale float64) *Hub {
	return newHub(scenario, scale)
}

func newHub(scenario Scenario, scale float64) *Hub {
	h := &Hub{
		scenario:  scenario,
		scale:     scale,
		ring:      newRing(scrollbackBytes),
		clients:   make(map[string]*attachedClient),
		state:     wire.SessionStateLive,
		startedAt: time.Now(),
		done:      make(chan struct{}),
	}
	go h.play()
	return h
}

// Attach admits c: it sends Hello, then a ring replay if there is one, then
// (if the scenario has already ended) the terminal Exit frame, then
// broadcasts the updated Roster. It matches the real Hub's replay-on-join
// behaviour for both a first join and a reconnect.
func (h *Hub) Attach(c Client) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	joinedAt := time.Now()
	h.clients[c.ID()] = &attachedClient{client: c, joinedAt: joinedAt}
	h.order = append(h.order, c.ID())

	sizing := false
	if h.sizingID == "" {
		h.sizingID = c.ID()
		sizing = true
	}

	hello := wire.Hello{
		Protocol: wire.Subprotocol,
		Server:   "ditty-fixture",
		Session: wire.HelloSession{
			ID:        h.scenario.Name,
			Name:      h.scenario.Name,
			Title:     h.scenario.Name,
			Cols:      h.scenario.Cols,
			Rows:      h.scenario.Rows,
			State:     h.state,
			StartedAt: h.startedAt.Format(time.RFC3339),
		},
		Client: wire.HelloClient{ID: c.ID(), Label: c.Label(), Writable: false, Sizing: sizing},
		Policy: wire.HelloPolicy{Writable: false, Reconnect: true, ReconnectInterval: "1s"},
	}
	if err := h.sendJSON(c, wire.OpHello, hello); err != nil {
		return err
	}

	if snap := h.ring.Snapshot(); snap != nil {
		if err := h.sendRaw(c, wire.OpOutput, snap); err != nil {
			return err
		}
	}

	if h.closed && h.exit != nil {
		if err := h.sendJSON(c, wire.OpExit, *h.exit); err != nil {
			return err
		}
	}

	h.broadcastRosterLocked()
	return nil
}

// Detach removes id, handing the sizing role to the next Client by join
// order if id held it (SPEC.md §5.4).
func (h *Hub) Detach(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.clients, id)
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
			if ac, ok := h.clients[h.sizingID]; ok {
				_ = h.sendJSON(ac.client, wire.OpState, wire.State{
					Sizing: true,
					State:  h.state,
					Reason: "sizing handover",
				})
			}
		}
	}

	h.broadcastRosterLocked()
}

// Input is always dropped: the fixture is read-only and has no PTY to
// write to (SPEC.md §0.3). The first drop per Client gets exactly one
// Notice, matching the real Hub's read-only enforcement.
func (h *Hub) Input(id string, _ []byte) {
	h.mu.Lock()
	ac, ok := h.clients[id]
	notify := ok && !ac.notifiedReadOnly
	if ok {
		ac.notifiedReadOnly = true
	}
	h.mu.Unlock()

	if notify {
		_ = h.sendJSON(ac.client, wire.OpNotice, wire.Notice{
			Level:   wire.NoticeLevelInfo,
			Message: "This session is read-only",
		})
	}
}

// Resize accepts a resize hint from id. The fixture has no PTY to resize,
// so this only exists to keep the wire contract satisfiable; it never
// errors.
func (h *Hub) Resize(_ string, _, _ int) {}

func (h *Hub) sendJSON(c Client, op wire.Opcode, payload any) error {
	frame, err := wire.Encode(op, payload)
	if err != nil {
		return err
	}
	return c.Send(frame)
}

func (h *Hub) sendRaw(c Client, op wire.Opcode, data []byte) error {
	frame, err := wire.Encode(op, data)
	if err != nil {
		return err
	}
	return c.Send(frame)
}

// broadcastRosterLocked must be called with h.mu held.
func (h *Hub) broadcastRosterLocked() {
	roster := wire.Roster{Count: len(h.order)}
	for _, id := range h.order {
		ac := h.clients[id]
		roster.Clients = append(roster.Clients, wire.RosterClient{
			ID:       id,
			Label:    ac.client.Label(),
			Writable: false,
			JoinedAt: ac.joinedAt.Format(time.RFC3339),
		})
	}
	frame, err := wire.Encode(wire.OpRoster, roster)
	if err != nil {
		return
	}
	for _, ac := range h.clients {
		_ = ac.client.Send(frame)
	}
}

// broadcastRawLocked must be called with h.mu held.
func (h *Hub) broadcastRawLocked(op wire.Opcode, data []byte) {
	frame, err := wire.Encode(op, data)
	if err != nil {
		return
	}
	for _, ac := range h.clients {
		_ = ac.client.Send(frame)
	}
}

func (h *Hub) at(d time.Duration) time.Duration {
	return time.Duration(float64(d) * h.scale)
}

func (h *Hub) sleepUntil(start time.Time, target time.Duration) bool {
	wait := target - time.Since(start)
	if wait <= 0 {
		return true
	}
	select {
	case <-time.After(wait):
		return true
	case <-h.done:
		return false
	}
}

// play runs scenario's schedule against real (or scaled) time: writing
// each Chunk into the ring and broadcasting it, dropping every Client at
// DropAt to simulate a network failure, then finishing per Ending.
func (h *Hub) play() {
	start := time.Now()

	for _, chunk := range h.scenario.Chunks {
		if !h.sleepUntil(start, h.at(chunk.At)) {
			return
		}
		h.mu.Lock()
		h.ring.Write(chunk.Data)
		h.broadcastRawLocked(wire.OpOutput, chunk.Data)
		h.mu.Unlock()
	}

	if h.scenario.DropAt > 0 {
		if !h.sleepUntil(start, h.at(h.scenario.DropAt)) {
			return
		}
		h.mu.Lock()
		for _, ac := range h.clients {
			ac.client.Close(1001, "connection dropped")
		}
		h.clients = make(map[string]*attachedClient)
		h.order = nil
		h.sizingID = ""
		h.mu.Unlock()
	}

	switch h.scenario.Ending.Kind {
	case EndingExit:
		if !h.sleepUntil(start, h.at(h.scenario.Ending.At)) {
			return
		}
		h.finish(wire.Exit{Code: h.scenario.Ending.Code})
	case EndingSignal:
		if !h.sleepUntil(start, h.at(h.scenario.Ending.At)) {
			return
		}
		h.finish(wire.Exit{Signal: h.scenario.Ending.Signal})
	case EndingNeverending:
	}
}

func (h *Hub) finish(exit wire.Exit) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	h.state = wire.SessionStateClosed
	h.exit = &exit
	for _, ac := range h.clients {
		_ = h.sendJSON(ac.client, wire.OpExit, exit)
	}
}
