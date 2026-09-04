package fixture

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/0funct0ry/ditty/internal/wire"
)

// testScale compresses every scenario's real-time schedule so scenario
// tests run in well under a second instead of tracking the real 5s+
// schedules those scenarios are authored with for a live demo.
const testScale = 0.02

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

func TestHubDeploy_HelloThenOutput(t *testing.T) {
	scenario := Scenarios()["deploy"]
	h := NewHubScaled(scenario, testScale)
	c := newFakeClient("c1")
	if err := h.Attach(c); err != nil {
		t.Fatalf("Attach: %v", err)
	}

	waitUntil(t, 2*time.Second, func() bool { return len(c.snapshot()) >= 2 })

	frames := decodeAll(t, c.snapshot())
	if frames[0].Op != wire.OpHello {
		t.Fatalf("first frame = %s, want Hello", frames[0].Op)
	}
	hello, err := wire.DecodeHello(frames[0].Payload)
	if err != nil {
		t.Fatalf("decode Hello: %v", err)
	}
	if hello.Session.Name != "deploy" || !hello.Client.Sizing {
		t.Fatalf("unexpected Hello: %+v", hello)
	}

	// Collect every Output payload emitted and confirm it matches the
	// scenario's chunks, in order, once playback settles.
	waitUntil(t, 2*time.Second, func() bool {
		var total int
		for _, f := range c.snapshot() {
			op, payload, _ := wire.Decode(f)
			if op == wire.OpOutput {
				total += len(payload)
			}
		}
		var want int
		for _, ch := range scenario.Chunks {
			want += len(ch.Data)
		}
		return total >= want
	})

	var got bytes.Buffer
	for _, f := range decodeAll(t, c.snapshot()) {
		if f.Op == wire.OpOutput {
			got.Write(f.Payload)
		}
	}
	var want bytes.Buffer
	for _, ch := range scenario.Chunks {
		want.Write(ch.Data)
	}
	if got.String() != want.String() {
		t.Fatalf("output mismatch:\ngot:  %q\nwant: %q", got.String(), want.String())
	}
}

func TestHubSetProfile_SeedsHelloProfileAndLock(t *testing.T) {
	scenario := Scenarios()["deploy"]
	h := NewHubScaled(scenario, testScale)
	h.SetProfile(json.RawMessage(`{"theme":"nord"}`), true)

	c := newFakeClient("c1")
	if err := h.Attach(c); err != nil {
		t.Fatalf("Attach: %v", err)
	}

	waitUntil(t, 2*time.Second, func() bool { return len(c.snapshot()) >= 1 })

	frames := decodeAll(t, c.snapshot())
	hello, err := wire.DecodeHello(frames[0].Payload)
	if err != nil {
		t.Fatalf("decode Hello: %v", err)
	}
	if string(hello.Profile) != `{"theme":"nord"}` {
		t.Fatalf("Hello.Profile = %s, want {\"theme\":\"nord\"}", hello.Profile)
	}
	if !hello.Policy.ProfileLock {
		t.Fatal("Hello.Policy.ProfileLock = false, want true")
	}
}

func TestHubHtop_MultipleRedraws(t *testing.T) {
	scenario := Scenarios()["htop"]
	h := NewHubScaled(scenario, testScale)
	c := newFakeClient("c1")
	if err := h.Attach(c); err != nil {
		t.Fatalf("Attach: %v", err)
	}

	waitUntil(t, 2*time.Second, func() bool {
		n := 0
		for _, f := range decodeAll(t, c.snapshot()) {
			if f.Op == wire.OpOutput {
				n++
			}
		}
		return n >= 5
	})
}

func TestHubQuickexit_ExitFrame(t *testing.T) {
	scenario := Scenarios()["quickexit"]
	h := NewHubScaled(scenario, testScale)
	c := newFakeClient("c1")
	if err := h.Attach(c); err != nil {
		t.Fatalf("Attach: %v", err)
	}

	waitUntil(t, 2*time.Second, func() bool {
		for _, f := range decodeAll(t, c.snapshot()) {
			if f.Op == wire.OpExit {
				return true
			}
		}
		return false
	})

	var exit wire.Exit
	for _, f := range decodeAll(t, c.snapshot()) {
		if f.Op == wire.OpExit {
			var err error
			exit, err = wire.DecodeExit(f.Payload)
			if err != nil {
				t.Fatalf("decode Exit: %v", err)
			}
		}
	}
	if exit.Code != scenario.Ending.Code {
		t.Fatalf("exit code = %d, want %d", exit.Code, scenario.Ending.Code)
	}

	// A late joiner must see the same terminal Exit immediately.
	late := newFakeClient("late")
	if err := h.Attach(late); err != nil {
		t.Fatalf("Attach late: %v", err)
	}
	frames := decodeAll(t, late.snapshot())
	var sawExit bool
	for _, f := range frames {
		if f.Op == wire.OpExit {
			sawExit = true
		}
	}
	if !sawExit {
		t.Fatalf("late joiner did not receive Exit, frames: %+v", frames)
	}
}

func TestHubFlaky_DropsAndReplaysOnReconnect(t *testing.T) {
	scenario := Scenarios()["flaky"]
	h := NewHubScaled(scenario, testScale)
	c := newFakeClient("c1")
	if err := h.Attach(c); err != nil {
		t.Fatalf("Attach: %v", err)
	}

	waitUntil(t, 2*time.Second, func() bool { return c.isClosed() })

	// Reconnect: a fresh Client attaching after the drop must see
	// everything emitted so far via ring replay, including chunks
	// scheduled after the drop.
	waitUntil(t, 2*time.Second, func() bool {
		reconnect := newFakeClient("c1-reconnect")
		if err := h.Attach(reconnect); err != nil {
			t.Fatalf("Attach reconnect: %v", err)
		}
		for _, f := range decodeAll(t, reconnect.snapshot()) {
			if f.Op == wire.OpOutput && bytes.Contains(f.Payload, []byte("reconnected")) {
				return true
			}
		}
		return false
	})
}

func TestHubSizingHandover(t *testing.T) {
	scenario := Scenarios()["deploy"]
	h := NewHubScaled(scenario, testScale)

	c1 := newFakeClient("c1")
	c2 := newFakeClient("c2")
	if err := h.Attach(c1); err != nil {
		t.Fatalf("Attach c1: %v", err)
	}
	if err := h.Attach(c2); err != nil {
		t.Fatalf("Attach c2: %v", err)
	}

	hello1, err := wire.DecodeHello(decodeAll(t, c1.snapshot())[0].Payload)
	if err != nil {
		t.Fatalf("decode Hello c1: %v", err)
	}
	if !hello1.Client.Sizing {
		t.Fatalf("c1 should be the sizing client on first join")
	}

	h.Detach("c1")

	waitUntil(t, time.Second, func() bool {
		for _, f := range decodeAll(t, c2.snapshot()) {
			if f.Op == wire.OpState {
				state, err := wire.DecodeState(f.Payload)
				if err == nil && state.Sizing {
					return true
				}
			}
		}
		return false
	})
}

func TestHubMaxClients_RejectsOverLimit(t *testing.T) {
	scenario := Scenarios()["deploy"]
	h := NewHubScaled(scenario, testScale)
	h.SetMaxClients(1)

	c1 := newFakeClient("c1")
	if err := h.Attach(c1); err != nil {
		t.Fatalf("Attach c1: %v", err)
	}

	c2 := newFakeClient("c2")
	err := h.Attach(c2)
	if err == nil {
		t.Fatalf("Attach c2: want ErrMaxClients, got nil")
	}
	var maxErr *ErrMaxClients
	if !errors.As(err, &maxErr) {
		t.Fatalf("Attach c2 error = %v, want *ErrMaxClients", err)
	}
	code, reason := maxErr.CloseCode()
	if code != 1013 || reason == "" {
		t.Fatalf("CloseCode() = (%d, %q), want (1013, non-empty)", code, reason)
	}

	// Rejected clients must not appear in the roster or take a sizing role.
	if len(h.order) != 1 || h.order[0] != "c1" {
		t.Fatalf("order = %v, want [c1]", h.order)
	}
}

func TestHubReadOnly_DropsInputWithOneNotice(t *testing.T) {
	scenario := Scenarios()["deploy"]
	h := NewHubScaled(scenario, testScale)
	c := newFakeClient("c1")
	if err := h.Attach(c); err != nil {
		t.Fatalf("Attach: %v", err)
	}

	h.Input("c1", []byte("ls\n"))
	h.Input("c1", []byte("pwd\n"))

	var notices int
	for _, f := range decodeAll(t, c.snapshot()) {
		if f.Op == wire.OpNotice {
			notices++
		}
	}
	if notices != 1 {
		t.Fatalf("notices = %d, want exactly 1", notices)
	}
}
