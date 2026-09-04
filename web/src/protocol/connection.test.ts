import { describe, expect, it, vi } from "vitest";
import { DittyConnection, type WebSocketFactory, type WebSocketLike } from "./connection";
import { ServerOpcode } from "./opcodes";
import type { Hello } from "./types";

class MockSocket implements WebSocketLike {
  readyState = 0;
  binaryType = "";
  onopen: ((ev: unknown) => void) | null = null;
  onclose: ((ev: { code: number; reason: string }) => void) | null = null;
  onerror: ((ev: unknown) => void) | null = null;
  onmessage: ((ev: { data: ArrayBuffer }) => void) | null = null;
  sent: ArrayBuffer[] = [];
  closed = false;

  open() {
    this.readyState = 1;
    this.onopen?.({});
  }

  message(frame: ArrayBuffer) {
    this.onmessage?.({ data: frame });
  }

  emitClose(code: number, reason = "") {
    this.readyState = 3;
    this.onclose?.({ code, reason });
  }

  send(data: ArrayBuffer) {
    this.sent.push(data);
  }

  close() {
    this.closed = true;
    this.emitClose(1000, "client closed");
  }
}

function frame(opcode: number, payload?: unknown): ArrayBuffer {
  const body = payload === undefined ? new Uint8Array(0) : new TextEncoder().encode(JSON.stringify(payload));
  const buf = new Uint8Array(1 + body.length);
  buf[0] = opcode;
  buf.set(body, 1);
  return buf.buffer;
}

function rawFrame(opcode: number, bytes: number[]): ArrayBuffer {
  const buf = new Uint8Array(1 + bytes.length);
  buf[0] = opcode;
  buf.set(bytes, 1);
  return buf.buffer;
}

const helloPayload: Hello = {
  protocol: "ditty.v1",
  server: "ditty/dev",
  session: { id: "s1", name: "deploy", title: "deploy", cols: 80, rows: 24, state: "live", startedAt: "now" },
  client: { id: "c1", label: "you", writable: true, sizing: true },
  policy: { writable: true, reconnect: true, reconnectInterval: "1s", unloadWarning: false, maxClients: 8 },
};

function setup() {
  let created: MockSocket[] = [];
  const factory: WebSocketFactory = () => {
    const sock = new MockSocket();
    created.push(sock);
    return sock;
  };
  return { factory, sockets: () => created };
}

describe("DittyConnection", () => {
  it("goes connecting -> open -> live on Hello", () => {
    const { factory, sockets } = setup();
    const states: string[] = [];
    const conn = new DittyConnection("ws://x/ws", { socketFactory: factory, onStateChange: (s) => states.push(s) });
    expect(conn.state).toBe("connecting");

    sockets()[0].open();
    expect(conn.state).toBe("open");

    sockets()[0].message(frame(ServerOpcode.Hello, helloPayload));
    expect(conn.state).toBe("live");
    expect(conn.hello?.session.id).toBe("s1");
    expect(states).toEqual(["open", "live"]);
  });

  it("routes Output frames to onOutput without touching React-shaped state", () => {
    const { factory, sockets } = setup();
    const chunks: Uint8Array[] = [];
    new DittyConnection("ws://x/ws", { socketFactory: factory, onOutput: (d) => chunks.push(d) });
    sockets()[0].open();
    sockets()[0].message(rawFrame(ServerOpcode.Output, [104, 105])); // "hi"
    expect(chunks).toHaveLength(1);
    expect(new TextDecoder().decode(chunks[0])).toBe("hi");
  });

  it("handles Roster, State, Notice frames", () => {
    const { factory, sockets } = setup();
    const rosters: unknown[] = [];
    const notices: unknown[] = [];
    const states: unknown[] = [];
    const conn = new DittyConnection("ws://x/ws", {
      socketFactory: factory,
      onRoster: (r) => rosters.push(r),
      onNotice: (n) => notices.push(n),
      onSessionState: (s) => states.push(s),
    });
    sockets()[0].open();
    sockets()[0].message(frame(ServerOpcode.Roster, { clients: [], count: 0 }));
    sockets()[0].message(frame(ServerOpcode.Notice, { level: "info", message: "hi" }));
    sockets()[0].message(frame(ServerOpcode.State, { writable: false, sizing: false, state: "live", reason: "" }));
    expect(rosters).toHaveLength(1);
    expect(notices).toHaveLength(1);
    expect(states).toHaveLength(1);
    expect(conn.writable).toBe(false); // State overrides Hello once received
  });

  it("goes closed on Exit and does not reconnect", () => {
    const { factory, sockets } = setup();
    const conn = new DittyConnection("ws://x/ws", { socketFactory: factory });
    sockets()[0].open();
    sockets()[0].message(frame(ServerOpcode.Hello, helloPayload));
    sockets()[0].message(frame(ServerOpcode.Exit, { code: 0, signal: "", message: "" }));
    expect(conn.state).toBe("closed");
    expect(conn.exit?.code).toBe(0);

    sockets()[0].emitClose(1006);
    expect(conn.state).toBe("closed");
    expect(sockets()).toHaveLength(1); // no reconnect attempt
  });

  it("reconnects with backoff after an unexpected close while live", () => {
    vi.useFakeTimers();
    const { factory, sockets } = setup();
    const conn = new DittyConnection("ws://x/ws", {
      socketFactory: factory,
      reconnectIntervalMs: 100,
      maxReconnectIntervalMs: 1000,
    });
    sockets()[0].open();
    sockets()[0].message(frame(ServerOpcode.Hello, helloPayload));

    sockets()[0].emitClose(1006); // abnormal closure
    expect(conn.state).toBe("reconnecting");

    vi.advanceTimersByTime(100);
    expect(sockets()).toHaveLength(2);
    sockets()[1].open();
    expect(conn.state).toBe("open");
    vi.useRealTimers();
  });

  it("goes rejected on a first-connection protocol-error close, no retry", () => {
    const { factory, sockets } = setup();
    const conn = new DittyConnection("ws://x/ws", { socketFactory: factory });
    sockets()[0].emitClose(1002, "missing subprotocol");
    expect(conn.state).toBe("rejected");
    expect(sockets()).toHaveLength(1);
  });

  it("retryNow cancels backoff and reconnects immediately", () => {
    vi.useFakeTimers();
    const { factory, sockets } = setup();
    const conn = new DittyConnection("ws://x/ws", { socketFactory: factory, reconnectIntervalMs: 10_000 });
    sockets()[0].open();
    sockets()[0].message(frame(ServerOpcode.Hello, helloPayload));
    sockets()[0].emitClose(1006);
    expect(conn.state).toBe("reconnecting");

    conn.retryNow();
    expect(sockets()).toHaveLength(2);
    vi.useRealTimers();
  });

  it("ignores unknown opcodes and malformed JSON without breaking the connection", () => {
    const { factory, sockets } = setup();
    const conn = new DittyConnection("ws://x/ws", { socketFactory: factory });
    sockets()[0].open();
    sockets()[0].message(rawFrame(99, [1, 2, 3])); // unknown opcode
    expect(conn.state).toBe("open"); // unaffected, referenced below to keep the variable live

    sockets()[0].message(rawFrame(ServerOpcode.Hello, [123, 110, 111, 116, 106, 115, 111, 110])); // "{notjson"
    expect(conn.state).toBe("open"); // malformed Hello dropped, not fatal

    sockets()[0].message(new ArrayBuffer(0)); // empty frame
    expect(conn.state).toBe("open");
  });

  it("send/resize/ping are no-ops until live", () => {
    const { factory, sockets } = setup();
    const conn = new DittyConnection("ws://x/ws", { socketFactory: factory });
    conn.send(new TextEncoder().encode("x"));
    conn.resize(80, 24);
    conn.ping();
    sockets()[0].open();
    expect(sockets()[0].sent).toHaveLength(0);

    sockets()[0].message(frame(ServerOpcode.Hello, helloPayload));
    conn.send(new TextEncoder().encode("x"));
    conn.resize(80, 24);
    expect(sockets()[0].sent).toHaveLength(2);
  });

  it("manual close() disables reconnect", () => {
    const { factory, sockets } = setup();
    const conn = new DittyConnection("ws://x/ws", { socketFactory: factory });
    sockets()[0].open();
    sockets()[0].message(frame(ServerOpcode.Hello, helloPayload));
    conn.close();
    expect(conn.state).toBe("closed");
    expect(sockets()).toHaveLength(1);
  });
});
