package wire

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// ErrEmptyFrame is returned by Decode when given a zero-length frame.
var ErrEmptyFrame = errors.New("wire: empty frame")

// rawOpcodes are the opcodes whose payload is written verbatim rather than
// JSON-marshalled.
func isRawOpcode(op Opcode) bool {
	return op == OpInput || op == OpOutput
}

// Encode writes op as byte 0 followed by payload. For OpInput and OpOutput,
// payload must be a []byte and is written verbatim. Every other opcode's
// payload is JSON-marshalled; pass nil for the empty-payload opcodes
// (OpPing, OpPong).
func Encode(op Opcode, payload any) ([]byte, error) {
	if isRawOpcode(op) {
		raw, ok := payload.([]byte)
		if !ok {
			return nil, fmt.Errorf("wire: encode %s: payload must be []byte, got %T", op, payload)
		}
		frame := make([]byte, 1+len(raw))
		frame[0] = byte(op)
		copy(frame[1:], raw)
		return frame, nil
	}

	if payload == nil {
		return []byte{byte(op)}, nil
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("wire: encode %s: %w", op, err)
	}
	frame := make([]byte, 1+len(body))
	frame[0] = byte(op)
	copy(frame[1:], body)
	return frame, nil
}

// Decode splits frame into its opcode and remaining payload bytes. It
// performs no JSON parsing beyond validating, for known JSON opcodes, that
// the payload (when non-empty) is syntactically valid JSON. Decode never
// panics.
func Decode(frame []byte) (Opcode, []byte, error) {
	if len(frame) == 0 {
		return 0, nil, ErrEmptyFrame
	}

	op := Opcode(frame[0])
	payload := frame[1:]

	if !knownOpcode(op) {
		return 0, nil, fmt.Errorf("wire: unknown opcode 0x%02x", frame[0])
	}

	if isRawOpcode(op) {
		return op, payload, nil
	}

	if len(payload) > 0 && !json.Valid(payload) {
		return 0, nil, fmt.Errorf("wire: decode %s: invalid JSON payload", op)
	}

	return op, payload, nil
}

// knownOpcode reports whether b falls within the valid opcode byte range
// (0x00-0x06) shared by both the Client->Server and Server->Client tables.
// It cannot tell the two tables apart on the byte value alone — see the
// Opcode.String doc comment — so callers that need direction-specific
// validation must check against the specific opcode set they expect.
func knownOpcode(op Opcode) bool {
	return byte(op) <= byte(OpPong)
}

// EncodeTo encodes op/payload and writes the resulting frame to w.
func EncodeTo(w io.Writer, op Opcode, payload any) error {
	frame, err := Encode(op, payload)
	if err != nil {
		return err
	}
	_, err = w.Write(frame)
	return err
}

// DecodeFrom reads all of r and decodes it as a single frame. It is
// intended for tests and tooling that hand it exactly one message's worth
// of bytes; it is not used on the live WebSocket path, where the transport
// already delivers one frame per message.
func DecodeFrom(r io.Reader) (Opcode, []byte, error) {
	frame, err := io.ReadAll(r)
	if err != nil {
		return 0, nil, fmt.Errorf("wire: decode from reader: %w", err)
	}
	return Decode(frame)
}

// DecodeResize decodes a Resize payload and clamps it into range.
func DecodeResize(payload []byte) (Resize, error) {
	var r Resize
	if err := json.Unmarshal(payload, &r); err != nil {
		return Resize{}, fmt.Errorf("wire: decode Resize: %w", err)
	}
	return r.Clamp(), nil
}

// DecodeHello decodes a Hello payload.
func DecodeHello(payload []byte) (Hello, error) {
	var h Hello
	if err := json.Unmarshal(payload, &h); err != nil {
		return Hello{}, fmt.Errorf("wire: decode Hello: %w", err)
	}
	return h, nil
}

// DecodeRoster decodes a Roster payload.
func DecodeRoster(payload []byte) (Roster, error) {
	var r Roster
	if err := json.Unmarshal(payload, &r); err != nil {
		return Roster{}, fmt.Errorf("wire: decode Roster: %w", err)
	}
	return r, nil
}

// DecodeState decodes a State payload and validates State.State against
// the live/detached/closed enum.
func DecodeState(payload []byte) (State, error) {
	var s State
	if err := json.Unmarshal(payload, &s); err != nil {
		return State{}, fmt.Errorf("wire: decode State: %w", err)
	}
	switch s.State {
	case SessionStateLive, SessionStateDetached, SessionStateClosed:
	default:
		return State{}, fmt.Errorf("wire: decode State: invalid state %q", s.State)
	}
	return s, nil
}

// DecodeExit decodes an Exit payload.
func DecodeExit(payload []byte) (Exit, error) {
	var e Exit
	if err := json.Unmarshal(payload, &e); err != nil {
		return Exit{}, fmt.Errorf("wire: decode Exit: %w", err)
	}
	return e, nil
}

// DecodeNotice decodes a Notice payload and validates Notice.Level against
// the info/warn/error enum.
func DecodeNotice(payload []byte) (Notice, error) {
	var n Notice
	if err := json.Unmarshal(payload, &n); err != nil {
		return Notice{}, fmt.Errorf("wire: decode Notice: %w", err)
	}
	switch n.Level {
	case NoticeLevelInfo, NoticeLevelWarn, NoticeLevelError:
	default:
		return Notice{}, fmt.Errorf("wire: decode Notice: invalid level %q", n.Level)
	}
	return n, nil
}
