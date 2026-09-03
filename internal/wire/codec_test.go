package wire

import (
	"bytes"
	"math/rand"
	"testing"
)

func TestRoundTripEveryOpcode(t *testing.T) {
	cases := []struct {
		name    string
		op      Opcode
		payload any
		decode  func([]byte) error
	}{
		{"Input", OpInput, []byte("hello"), nil},
		{"Output", OpOutput, []byte("\x1b[2Jworld"), nil},
		{"Resize", OpResize, Resize{Cols: 80, Rows: 24}, func(b []byte) error { _, err := DecodeResize(b); return err }},
		{"Ping", OpPing, nil, nil},
		{"Pong", OpPong, nil, nil},
		{
			"Hello", OpHello,
			Hello{Protocol: Subprotocol, Server: "1.0.0",
				Session: HelloSession{ID: "s1", Name: "deploy", Title: "bash", Cols: 80, Rows: 24, State: "live", StartedAt: "now"},
				Client:  HelloClient{ID: "c1", Label: "Client 1", Writable: true, Sizing: true},
				Policy:  HelloPolicy{Writable: true, Reconnect: true, ReconnectInterval: "3s", UnloadWarning: true, MaxClients: 0}},
			func(b []byte) error { _, err := DecodeHello(b); return err },
		},
		{
			"Roster", OpRoster,
			Roster{Clients: []RosterClient{{ID: "c1", Label: "Client 1", Writable: true, JoinedAt: "now"}}, Count: 1},
			func(b []byte) error { _, err := DecodeRoster(b); return err },
		},
		{
			"State", OpState,
			State{Writable: true, Sizing: true, State: SessionStateLive, Reason: ""},
			func(b []byte) error { _, err := DecodeState(b); return err },
		},
		{
			"Exit", OpExit,
			Exit{Code: 0, Signal: "", Message: "exited"},
			func(b []byte) error { _, err := DecodeExit(b); return err },
		},
		{
			"Notice", OpNotice,
			Notice{Level: NoticeLevelInfo, Message: "sized by Client 1"},
			func(b []byte) error { _, err := DecodeNotice(b); return err },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			frame, err := Encode(tc.op, tc.payload)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			if frame[0] != byte(tc.op) {
				t.Fatalf("byte 0 = 0x%02x, want 0x%02x", frame[0], tc.op)
			}

			gotOp, payload, err := Decode(frame)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if gotOp != tc.op {
				t.Fatalf("Decode opcode = %v, want %v", gotOp, tc.op)
			}

			if raw, ok := tc.payload.([]byte); ok {
				if !bytes.Equal(payload, raw) {
					t.Fatalf("payload = %q, want %q", payload, raw)
				}
			}

			if tc.decode != nil {
				if err := tc.decode(payload); err != nil {
					t.Fatalf("typed decode: %v", err)
				}
			}
		})
	}
}

func TestRoundTripRandomizedPayloads(t *testing.T) {
	rng := rand.New(rand.NewSource(1))

	for i := 0; i < 200; i++ {
		raw := make([]byte, rng.Intn(256))
		rng.Read(raw)

		for _, op := range []Opcode{OpInput, OpOutput} {
			frame, err := Encode(op, raw)
			if err != nil {
				t.Fatalf("Encode %s: %v", op, err)
			}
			gotOp, payload, err := Decode(frame)
			if err != nil {
				t.Fatalf("Decode %s: %v", op, err)
			}
			if gotOp != op || !bytes.Equal(payload, raw) {
				t.Fatalf("round trip mismatch for %s", op)
			}
		}

		resize := Resize{Cols: rng.Intn(2000) - 500, Rows: rng.Intn(1000) - 250}
		frame, err := Encode(OpResize, resize)
		if err != nil {
			t.Fatalf("Encode Resize: %v", err)
		}
		_, payload, err := Decode(frame)
		if err != nil {
			t.Fatalf("Decode Resize: %v", err)
		}
		got, err := DecodeResize(payload)
		if err != nil {
			t.Fatalf("DecodeResize: %v", err)
		}
		if got.Cols < MinCols || got.Cols > MaxCols || got.Rows < MinRows || got.Rows > MaxRows {
			t.Fatalf("clamp failed: %+v", got)
		}
	}
}

func TestEncodeToDecodeFrom(t *testing.T) {
	var buf bytes.Buffer
	if err := EncodeTo(&buf, OpOutput, []byte("hi")); err != nil {
		t.Fatalf("EncodeTo: %v", err)
	}
	op, payload, err := DecodeFrom(&buf)
	if err != nil {
		t.Fatalf("DecodeFrom: %v", err)
	}
	if op != OpOutput || string(payload) != "hi" {
		t.Fatalf("got op=%v payload=%q", op, payload)
	}
}

func TestOpcodeString(t *testing.T) {
	cases := []struct {
		op   Opcode
		want string
	}{
		{Opcode(0x00), "Input/Output"},
		{OpResize, "Resize/Hello"},
		{OpHello, "Resize/Hello"},
		{OpPing, "Ping/Roster"},
		{OpRoster, "Ping/Roster"},
		{OpState, "State"},
		{OpExit, "Exit"},
		{OpNotice, "Notice"},
		{OpPong, "Pong"},
		{Opcode(0xFF), "Opcode(0xff)"},
	}
	for _, tc := range cases {
		if got := tc.op.String(); got != tc.want {
			t.Errorf("Opcode(0x%02x).String() = %q, want %q", byte(tc.op), got, tc.want)
		}
	}
}

func TestEncodeRawWrongType(t *testing.T) {
	if _, err := Encode(OpInput, "not bytes"); err == nil {
		t.Fatal("expected error encoding non-[]byte payload for raw opcode")
	}
}

func TestEncodeJSONMarshalError(t *testing.T) {
	if _, err := Encode(OpHello, make(chan int)); err == nil {
		t.Fatal("expected error marshalling unsupported type")
	}
}
