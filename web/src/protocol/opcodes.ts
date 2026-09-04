// Mirrors internal/wire/opcodes.go. Byte 0 of every WebSocket binary frame
// is the opcode; Input/Output, Resize/Hello and Ping/Roster share numeric
// values across directions, so direction (not the byte value) disambiguates
// them — the two enums below are kept separate on purpose, matching the Go
// package.

// Client -> Server opcodes (SPEC.md §5.1).
export enum ClientOpcode {
  Input = 0x00,
  Resize = 0x01,
  Ping = 0x02,
}

// Server -> Client opcodes (SPEC.md §5.2).
export enum ServerOpcode {
  Output = 0x00,
  Hello = 0x01,
  Roster = 0x02,
  State = 0x03,
  Exit = 0x04,
  Notice = 0x05,
  Pong = 0x06,
}

export const SUBPROTOCOL = "ditty.v1";
