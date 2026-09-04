import { ClientOpcode, ServerOpcode } from "./opcodes";
import type { Exit, Hello, Notice, Resize, Roster, State } from "./types";

/** Encodes a Resize frame: opcode byte + JSON payload. */
export function encodeResize(resize: Resize): ArrayBuffer {
  return encodeJSON(ClientOpcode.Resize, resize);
}

/** Encodes an Input frame: opcode byte + raw bytes verbatim. */
export function encodeInput(data: Uint8Array): ArrayBuffer {
  return encodeRaw(ClientOpcode.Input, data);
}

/** Encodes a Ping frame: opcode byte, empty payload. */
export function encodePing(): ArrayBuffer {
  return encodeRaw(ClientOpcode.Ping, new Uint8Array(0));
}

function encodeRaw(op: number, data: Uint8Array): ArrayBuffer {
  const frame = new Uint8Array(1 + data.length);
  frame[0] = op;
  frame.set(data, 1);
  return frame.buffer;
}

function encodeJSON(op: number, payload: unknown): ArrayBuffer {
  const body = new TextEncoder().encode(JSON.stringify(payload));
  return encodeRaw(op, body);
}

export type DecodedFrame =
  | { op: ServerOpcode.Output; data: Uint8Array }
  | { op: ServerOpcode.Hello; data: Hello }
  | { op: ServerOpcode.Roster; data: Roster }
  | { op: ServerOpcode.State; data: State }
  | { op: ServerOpcode.Exit; data: Exit }
  | { op: ServerOpcode.Notice; data: Notice }
  | { op: ServerOpcode.Pong }
  | { op: "unknown"; opcode: number };

/**
 * Decodes one Server -> Client frame. Never throws: a truncated or
 * malformed frame is reported as an error result rather than propagated,
 * and an opcode outside the known set decodes to "unknown" so the caller
 * can ignore it instead of treating it as fatal (SPEC.md §5.2).
 */
export function decodeFrame(buf: ArrayBuffer): DecodedFrame | { op: "error"; message: string } {
  if (buf.byteLength === 0) {
    return { op: "error", message: "ditty: empty frame" };
  }
  const bytes = new Uint8Array(buf);
  const opcode = bytes[0];
  const payload = bytes.subarray(1);

  switch (opcode) {
    case ServerOpcode.Output:
      return { op: ServerOpcode.Output, data: payload };
    case ServerOpcode.Pong:
      return { op: ServerOpcode.Pong };
    case ServerOpcode.Hello:
      return decodeJSON<ServerOpcode.Hello, Hello>(ServerOpcode.Hello, payload);
    case ServerOpcode.Roster:
      return decodeJSON<ServerOpcode.Roster, Roster>(ServerOpcode.Roster, payload);
    case ServerOpcode.State:
      return decodeJSON<ServerOpcode.State, State>(ServerOpcode.State, payload);
    case ServerOpcode.Exit:
      return decodeJSON<ServerOpcode.Exit, Exit>(ServerOpcode.Exit, payload);
    case ServerOpcode.Notice:
      return decodeJSON<ServerOpcode.Notice, Notice>(ServerOpcode.Notice, payload);
    default:
      return { op: "unknown", opcode };
  }
}

function decodeJSON<Op extends ServerOpcode, T>(
  op: Op,
  payload: Uint8Array,
): { op: Op; data: T } | { op: "error"; message: string } {
  try {
    const text = new TextDecoder().decode(payload);
    return { op, data: JSON.parse(text) as T };
  } catch (err) {
    return { op: "error", message: `ditty: malformed frame for opcode ${op}: ${String(err)}` };
  }
}
