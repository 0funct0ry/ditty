import { useEffect, useRef, useState } from "react";
import { DittyConnection, type ConnectionState, type DittyConnectionOptions } from "../protocol/connection";
import type { Exit, Hello, Notice, Roster, State } from "../protocol/types";

export interface UseDittyConnectionOptions
  extends Pick<DittyConnectionOptions, "reconnectIntervalMs" | "maxReconnectIntervalMs" | "socketFactory"> {
  /** Called for every Output frame's raw bytes. Not part of React state — the
   * terminal writes these directly, so re-rendering on every chunk would be
   * wasteful and would race xterm's own buffering. */
  onOutput?: (data: Uint8Array) => void;
  onNotice?: (notice: Notice) => void;
}

export interface UseDittyConnectionResult {
  state: ConnectionState;
  hello: Hello | null;
  roster: Roster | null;
  sessionState: State | null;
  exit: Exit | null;
  writable: boolean;
  sizing: boolean;
  rejectReason: string | null;
  send: (data: Uint8Array) => void;
  resize: (cols: number, rows: number) => void;
  retryNow: () => void;
}

/**
 * Owns a DittyConnection for the lifetime of the component and mirrors its
 * state into React. The connection itself has no React dependency; this
 * hook is the thin adapter (SPEC.md §12 M4).
 */
export function useDittyConnection(url: string, options: UseDittyConnectionOptions = {}): UseDittyConnectionResult {
  const [state, setState] = useState<ConnectionState>("connecting");
  const [hello, setHello] = useState<Hello | null>(null);
  const [roster, setRoster] = useState<Roster | null>(null);
  const [sessionState, setSessionState] = useState<State | null>(null);
  const [exit, setExit] = useState<Exit | null>(null);

  const connRef = useRef<DittyConnection | null>(null);
  const optionsRef = useRef(options);
  optionsRef.current = options;

  useEffect(() => {
    const conn = new DittyConnection(url, {
      reconnectIntervalMs: options.reconnectIntervalMs,
      maxReconnectIntervalMs: options.maxReconnectIntervalMs,
      socketFactory: options.socketFactory,
      onStateChange: setState,
      onHello: setHello,
      onRoster: setRoster,
      onSessionState: setSessionState,
      onExit: setExit,
      onOutput: (data) => optionsRef.current.onOutput?.(data),
      onNotice: (notice) => optionsRef.current.onNotice?.(notice),
    });
    connRef.current = conn;
    setState(conn.state);

    return () => {
      conn.destroy();
      connRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- reconnect params/factory are read once per connection lifetime
  }, [url]);

  return {
    state,
    hello,
    roster,
    sessionState,
    exit,
    writable: connRef.current?.writable ?? false,
    sizing: connRef.current?.sizing ?? false,
    rejectReason: connRef.current?.rejectReason ?? null,
    send: (data) => connRef.current?.send(data),
    resize: (cols, rows) => connRef.current?.resize(cols, rows),
    retryNow: () => connRef.current?.retryNow(),
  };
}
