// Package wire implements the binary frame codec for protocol ditty.v1.
//
// Every frame is a WebSocket binary message. Byte 0 is the opcode; the
// remaining bytes are the payload. Text frames are not part of this
// protocol and must be rejected by the transport with close code 1003.
// The WebSocket subprotocol string is Subprotocol; a client that does not
// negotiate it is rejected by the transport with close code 1002.
//
// Protocol changes are never made silently in place: a breaking change
// bumps the subprotocol to ditty.v2.
//
// # Client -> Server opcodes (§5.1)
//
//	0x00 Input   raw bytes            Dropped silently if the Client is read-only;
//	                                  the server sends one Notice on the first drop.
//	0x01 Resize  JSON {"cols":int,"rows":int}
//	                                  Ignored unless this Client is the sizing
//	                                  client. Clamped to cols in [10,1000],
//	                                  rows in [5,500].
//	0x02 Ping    empty                Optional application-level keepalive;
//	                                  the server replies Pong.
//
// # Server -> Client opcodes (§5.2)
//
//	0x00 Output  raw bytes            Raw PTY output.
//	0x01 Hello   JSON                 First frame after upgrade, always.
//	0x02 Roster  JSON {"clients":[{"id","label","writable","joinedAt"}],"count":int}
//	0x03 State   JSON {"writable":bool,"sizing":bool,"state":"live|detached|closed","reason":string}
//	0x04 Exit    JSON {"code":int,"signal":string,"message":string}
//	0x05 Notice  JSON {"level":"info|warn|error","message":string}
//	0x06 Pong    empty
//
// Input and Output share the numeric value 0x00; direction (which side sent
// the frame), not the opcode byte, disambiguates them. This package exposes
// them as distinct named constants with the same underlying byte value, and
// callers must decode using the opcode set appropriate to the direction
// they expect.
package wire

import "fmt"

// Opcode identifies the kind of frame. Its zero value is shared by Input
// and Output; see the package doc for how direction disambiguates them.
type Opcode byte

// Client -> Server opcodes.
const (
	OpInput  Opcode = 0x00
	OpResize Opcode = 0x01
	OpPing   Opcode = 0x02
)

// Server -> Client opcodes.
const (
	OpOutput Opcode = 0x00
	OpHello  Opcode = 0x01
	OpRoster Opcode = 0x02
	OpState  Opcode = 0x03
	OpExit   Opcode = 0x04
	OpNotice Opcode = 0x05
	OpPong   Opcode = 0x06
)

// Subprotocol is the WebSocket subprotocol identifying this wire format.
const Subprotocol = "ditty.v1"

// String returns a human-readable name for op. Byte values 0x00-0x02 are
// shared between the Client->Server and Server->Client tables (Input/
// Output, Resize/Hello, Ping/Roster), so String reports both names for
// those; the caller's direction context, not the byte value, is what
// actually disambiguates them.
func (op Opcode) String() string {
	switch byte(op) {
	case 0x00:
		return "Input/Output"
	case 0x01:
		return "Resize/Hello"
	case 0x02:
		return "Ping/Roster"
	case byte(OpState):
		return "State"
	case byte(OpExit):
		return "Exit"
	case byte(OpNotice):
		return "Notice"
	case byte(OpPong):
		return "Pong"
	default:
		return fmt.Sprintf("Opcode(0x%02x)", byte(op))
	}
}
