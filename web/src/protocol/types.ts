// Mirrors internal/wire/types.go field-for-field (same JSON shape, same
// names). Keep this file in sync with the Go package by hand — there is no
// codegen step for this milestone.

/** Client -> Server payload for ClientOpcode.Resize. */
export interface Resize {
  cols: number;
  rows: number;
}

/** Server -> Client payload for ServerOpcode.Hello, always the first frame. */
export interface Hello {
  protocol: string;
  server: string;
  session: HelloSession;
  client: HelloClient;
  policy: HelloPolicy;
  profile?: Profile;
}

export interface HelloSession {
  id: string;
  name: string;
  title: string;
  cols: number;
  rows: number;
  state: SessionState;
  startedAt: string;
}

export interface HelloClient {
  id: string;
  label: string;
  writable: boolean;
  sizing: boolean;
}

export interface HelloPolicy {
  writable: boolean;
  reconnect: boolean;
  reconnectInterval: string;
  unloadWarning: boolean;
  maxClients: number;
}

/** Server -> Client payload for ServerOpcode.Roster. */
export interface Roster {
  clients: RosterClient[];
  count: number;
}

export interface RosterClient {
  id: string;
  label: string;
  writable: boolean;
  joinedAt: string;
}

export type SessionState = "live" | "detached" | "closed";

/** Server -> Client payload for ServerOpcode.State. */
export interface State {
  writable: boolean;
  sizing: boolean;
  state: SessionState;
  reason: string;
}

/** Server -> Client payload for ServerOpcode.Exit. */
export interface Exit {
  code: number;
  signal: string;
  message: string;
}

export type NoticeLevel = "info" | "warn" | "error";

/** Server -> Client payload for ServerOpcode.Notice. */
export interface Notice {
  level: NoticeLevel;
  message: string;
}

// Profile (SPEC.md §7). Seeded server-side via Hello.profile, then merged
// with any localStorage-persisted client copy in M6.
export type CursorStyle = "block" | "underline" | "bar";
export type RendererKind = "webgl" | "canvas";
export type BellStyle = "none" | "sound" | "visual";

export interface Profile {
  theme: string;
  fontFamily: string;
  fontSize: number;
  lineHeight: number;
  cursorStyle: CursorStyle;
  cursorBlink: boolean;
  scrollback: number;
  renderer: RendererKind;
  bellStyle: BellStyle;
  macOptionIsMeta: boolean;
  altSendsEscape: boolean;
  copyOnSelect: boolean;
  rightClickPaste: boolean;
  unicodeVersion: string;
  eastAsianWidth: boolean;
  colors: Record<string, string>;
}
