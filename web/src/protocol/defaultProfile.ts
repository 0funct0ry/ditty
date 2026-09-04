import type { Profile } from "./types";

// SPEC.md §7 defaults, used until a Hello.profile arrives (or if the
// backend omits it — the JSON field is optional).
export const defaultProfile: Profile = {
  theme: "ditty-dark",
  fontFamily: "JetBrains Mono, SF Mono, Menlo, monospace",
  fontSize: 14,
  lineHeight: 1.2,
  cursorStyle: "block",
  cursorBlink: true,
  scrollback: 5000,
  renderer: "webgl",
  bellStyle: "none",
  macOptionIsMeta: true,
  altSendsEscape: true,
  copyOnSelect: true,
  rightClickPaste: false,
  unicodeVersion: "11",
  eastAsianWidth: false,
  colors: {},
};
