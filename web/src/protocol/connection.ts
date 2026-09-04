import { SUBPROTOCOL } from "./opcodes";
import { decodeFrame, encodeInput, encodePing, encodeResize } from "./codec";
import { ServerOpcode } from "./opcodes";
import type { Exit, Hello, Notice, Roster, State } from "./types";

export type ConnectionState = "connecting" | "open" | "live" | "reconnecting" | "closed" | "rejected";

/** Minimal WebSocket surface the connection needs — lets tests inject a mock. */
export interface WebSocketLike {
  readonly readyState: number;
  binaryType: string;
  onopen: ((ev: unknown) => void) | null;
  onclose: ((ev: { code: number; reason: string }) => void) | null;
  onerror: ((ev: unknown) => void) | null;
  onmessage: ((ev: { data: ArrayBuffer }) => void) | null;
  send(data: ArrayBuffer): void;
  close(code?: number, reason?: string): void;
}

export type WebSocketFactory = (url: string, protocols: string[]) => WebSocketLike;

const defaultFactory: WebSocketFactory = (url, protocols) =>
  new WebSocket(url, protocols) as unknown as WebSocketLike;

// Close codes the server uses to reject a Client outright (SPEC.md §4, §5.5,
// §6): wrong/missing subprotocol, over --max-clients, or a slow-client
// eviction. None of these should trigger a reconnect loop.
const REJECT_CLOSE_CODES = new Set([1002, 1003, 1008, 1011, 1013]);

export interface DittyConnectionOptions {
  reconnectIntervalMs?: number;
  maxReconnectIntervalMs?: number;
  socketFactory?: WebSocketFactory;
  onStateChange?: (state: ConnectionState) => void;
  onOutput?: (data: Uint8Array) => void;
  onHello?: (hello: Hello) => void;
  onRoster?: (roster: Roster) => void;
  onSessionState?: (state: State) => void;
  onExit?: (exit: Exit) => void;
  onNotice?: (notice: Notice) => void;
}

/**
 * DittyConnection owns the ditty.v1 WebSocket state machine:
 * connecting -> open -> live -> reconnecting -> closed | rejected.
 * It has no DOM/React dependency beyond the injectable WebSocketLike, so it
 * can be driven directly in tests with a mock socket.
 */
export class DittyConnection {
  private url: string;
  private socket: WebSocketLike | null = null;
  private readonly socketFactory: WebSocketFactory;
  private readonly baseReconnectMs: number;
  private readonly maxReconnectMs: number;
  private reconnectDelayMs: number;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private manuallyClosed = false;
  private everLive = false;
  private destroyed = false;

  state: ConnectionState = "connecting";
  hello: Hello | null = null;
  roster: Roster | null = null;
  sessionState: State | null = null;
  exit: Exit | null = null;
  /** Close reason the server sent when it rejected this Client outright. */
  rejectReason: string | null = null;

  private readonly opts: DittyConnectionOptions;

  constructor(url: string, opts: DittyConnectionOptions = {}) {
    this.url = url;
    this.opts = opts;
    this.socketFactory = opts.socketFactory ?? defaultFactory;
    this.baseReconnectMs = opts.reconnectIntervalMs ?? 1000;
    this.maxReconnectMs = opts.maxReconnectIntervalMs ?? 30000;
    this.reconnectDelayMs = this.baseReconnectMs;
    this.connect();
  }

  private setState(state: ConnectionState) {
    if (this.state === state) return;
    this.state = state;
    this.opts.onStateChange?.(state);
  }

  private connect() {
    if (this.destroyed) return;
    this.setState(this.everLive ? "reconnecting" : "connecting");
    const socket = this.socketFactory(this.url, [SUBPROTOCOL]);
    socket.binaryType = "arraybuffer";
    socket.onopen = () => {
      if (this.destroyed) return;
      this.setState("open");
    };
    socket.onmessage = (ev) => this.handleMessage(ev.data);
    socket.onclose = (ev) => this.handleClose(ev.code, ev.reason);
    socket.onerror = () => {
      /* the close handler that follows carries the actionable code */
    };
    this.socket = socket;
  }

  private handleMessage(buf: ArrayBuffer) {
    const decoded = decodeFrame(buf);
    switch (decoded.op) {
      case ServerOpcode.Output:
        this.opts.onOutput?.(decoded.data);
        return;
      case ServerOpcode.Hello:
        this.hello = decoded.data;
        this.everLive = true;
        this.reconnectDelayMs = this.baseReconnectMs;
        this.setState("live");
        this.opts.onHello?.(decoded.data);
        return;
      case ServerOpcode.Roster:
        this.roster = decoded.data;
        this.opts.onRoster?.(decoded.data);
        return;
      case ServerOpcode.State:
        this.sessionState = decoded.data;
        this.opts.onSessionState?.(decoded.data);
        return;
      case ServerOpcode.Exit:
        this.exit = decoded.data;
        this.opts.onExit?.(decoded.data);
        this.manuallyClosed = true; // session ended; never reconnect
        this.setState("closed");
        return;
      case ServerOpcode.Notice:
        this.opts.onNotice?.(decoded.data);
        return;
      case ServerOpcode.Pong:
        return;
      case "unknown":
        return; // unknown opcodes are ignored, not fatal (SPEC.md §5.2)
      case "error":
        return; // malformed frame; drop and keep the connection alive
    }
  }

  private handleClose(code: number, reason: string) {
    this.socket = null;
    if (this.destroyed) return;

    if (this.manuallyClosed) {
      this.setState("closed");
      return;
    }
    if (!this.everLive && REJECT_CLOSE_CODES.has(code)) {
      this.rejectReason = reason || null;
      this.setState("rejected");
      return;
    }
    if (this.hello && this.hello.policy.reconnect === false) {
      this.setState("closed");
      return;
    }
    this.scheduleReconnect();
  }

  private scheduleReconnect() {
    this.setState("reconnecting");
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, this.reconnectDelayMs);
    this.reconnectDelayMs = Math.min(this.reconnectDelayMs * 2, this.maxReconnectMs);
  }

  /** Cancels any pending backoff and reconnects immediately. Also usable
   * from `rejected` (SPEC.md §10.1's "Try again") — that state has no
   * scheduled retry to cancel, just a fresh attempt to make. */
  retryNow() {
    if (this.state !== "reconnecting" && this.state !== "rejected") return;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.reconnectDelayMs = this.baseReconnectMs;
    this.rejectReason = null;
    this.connect();
  }

  get writable(): boolean {
    return this.sessionState?.writable ?? this.hello?.client.writable ?? false;
  }

  get sizing(): boolean {
    return this.sessionState?.sizing ?? this.hello?.client.sizing ?? false;
  }

  send(data: Uint8Array) {
    if (this.state !== "live" || !this.socket) return;
    this.socket.send(encodeInput(data));
  }

  resize(cols: number, rows: number) {
    if (this.state !== "live" || !this.socket) return;
    this.socket.send(encodeResize({ cols, rows }));
  }

  ping() {
    if (this.state !== "live" || !this.socket) return;
    this.socket.send(encodePing());
  }

  /** Closes the connection and disables reconnect. */
  close() {
    this.manuallyClosed = true;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.socket?.close(1000, "client closed");
    this.setState("closed");
  }

  /** Tears down without emitting further state changes; for unmount. */
  destroy() {
    this.destroyed = true;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.socket?.close();
    this.socket = null;
  }
}
