import type { ITheme } from "@xterm/xterm";
import type { Profile } from "./types";

export type ThemeName = "ditty-dark" | "ditty-light" | "nord" | "dracula" | "solarized-dark" | "monokai";

/** Chrome tint that keeps the chrome bar / status bar visually agreeing
 * with the terminal's theme (CLAUDE.md/tailwind.config.ts design-token
 * note). Applied as CSS custom properties, not new Tailwind classes, so no
 * build step is needed per theme. */
export interface ChromeTint {
  bg: string;
  border: string;
  accent: string;
}

export interface ThemeDefinition {
  label: string;
  swatch: [string, string, string];
  xterm: ITheme;
  chrome: ChromeTint;
}

// The six themes from SPEC.md §7. Shipped in the bundle so the browser
// never needs to fetch a palette — only the theme *name* is seeded from the
// server (internal/profile/themes.go mirrors just the name list).
export const THEMES: Record<ThemeName, ThemeDefinition> = {
  "ditty-dark": {
    label: "ditty dark",
    swatch: ["#0E1116", "#3D9C87", "#E0A32E"],
    xterm: {
      background: "#0E1116",
      foreground: "#E4E7EC",
      cursor: "#3D9C87",
      cursorAccent: "#0E1116",
      selectionBackground: "rgba(61, 156, 135, 0.35)",
      black: "#171B24",
      red: "#C4634C",
      green: "#3D9C87",
      yellow: "#E0A32E",
      blue: "#4C8BC4",
      magenta: "#9C7CE0",
      cyan: "#2C7A6B",
      white: "#A6ADB9",
      brightBlack: "#2C3342",
      brightRed: "#D97B65",
      brightGreen: "#56C2A8",
      brightYellow: "#EDBB55",
      brightBlue: "#6FA6D9",
      brightMagenta: "#B49AEA",
      brightCyan: "#3D9C87",
      brightWhite: "#ECEDE8",
    },
    chrome: { bg: "#171B24", border: "#2C3342", accent: "#3D9C87" },
  },
  "ditty-light": {
    label: "ditty light",
    swatch: ["#F6F6F2", "#2C7A6B", "#BC5341"],
    xterm: {
      background: "#F6F6F2",
      foreground: "#171B24",
      cursor: "#2C7A6B",
      cursorAccent: "#F6F6F2",
      selectionBackground: "rgba(44, 122, 107, 0.25)",
      black: "#ECEDE8",
      red: "#BC5341",
      green: "#2C7A6B",
      yellow: "#B9861F",
      blue: "#3E6FA0",
      magenta: "#7C5AC4",
      cyan: "#2C7A6B",
      white: "#171B24",
      brightBlack: "#7C8492",
      brightRed: "#C4634C",
      brightGreen: "#3D9C87",
      brightYellow: "#E0A32E",
      brightBlue: "#4C8BC4",
      brightMagenta: "#9C7CE0",
      brightCyan: "#3D9C87",
      brightWhite: "#0E1116",
    },
    chrome: { bg: "#ECEDE8", border: "#D5D6CF", accent: "#2C7A6B" },
  },
  nord: {
    label: "nord",
    swatch: ["#2E3440", "#88C0D0", "#EBCB8B"],
    xterm: {
      background: "#2E3440",
      foreground: "#D8DEE9",
      cursor: "#D8DEE9",
      cursorAccent: "#2E3440",
      selectionBackground: "rgba(136, 192, 208, 0.3)",
      black: "#3B4252",
      red: "#BF616A",
      green: "#A3BE8C",
      yellow: "#EBCB8B",
      blue: "#81A1C1",
      magenta: "#B48EAD",
      cyan: "#88C0D0",
      white: "#E5E9F0",
      brightBlack: "#4C566A",
      brightRed: "#BF616A",
      brightGreen: "#A3BE8C",
      brightYellow: "#EBCB8B",
      brightBlue: "#81A1C1",
      brightMagenta: "#B48EAD",
      brightCyan: "#8FBCBB",
      brightWhite: "#ECEFF4",
    },
    chrome: { bg: "#2E3440", border: "#3B4252", accent: "#88C0D0" },
  },
  dracula: {
    label: "dracula",
    swatch: ["#282A36", "#BD93F9", "#50FA7B"],
    xterm: {
      background: "#282A36",
      foreground: "#F8F8F2",
      cursor: "#F8F8F2",
      cursorAccent: "#282A36",
      selectionBackground: "rgba(189, 147, 249, 0.3)",
      black: "#21222C",
      red: "#FF5555",
      green: "#50FA7B",
      yellow: "#F1FA8C",
      blue: "#BD93F9",
      magenta: "#FF79C6",
      cyan: "#8BE9FD",
      white: "#F8F8F2",
      brightBlack: "#6272A4",
      brightRed: "#FF6E6E",
      brightGreen: "#69FF94",
      brightYellow: "#FFFFA5",
      brightBlue: "#D6ACFF",
      brightMagenta: "#FF92DF",
      brightCyan: "#A4FFFF",
      brightWhite: "#FFFFFF",
    },
    chrome: { bg: "#282A36", border: "#44475A", accent: "#BD93F9" },
  },
  "solarized-dark": {
    label: "solarized",
    swatch: ["#002B36", "#268BD2", "#B58900"],
    xterm: {
      background: "#002B36",
      foreground: "#839496",
      cursor: "#93A1A1",
      cursorAccent: "#002B36",
      selectionBackground: "rgba(38, 139, 210, 0.3)",
      black: "#073642",
      red: "#DC322F",
      green: "#859900",
      yellow: "#B58900",
      blue: "#268BD2",
      magenta: "#D33682",
      cyan: "#2AA198",
      white: "#EEE8D5",
      brightBlack: "#002B36",
      brightRed: "#CB4B16",
      brightGreen: "#586E75",
      brightYellow: "#657B83",
      brightBlue: "#839496",
      brightMagenta: "#6C71C4",
      brightCyan: "#93A1A1",
      brightWhite: "#FDF6E3",
    },
    chrome: { bg: "#073642", border: "#0A4B5C", accent: "#268BD2" },
  },
  monokai: {
    label: "monokai",
    swatch: ["#272822", "#A6E22E", "#FD971F"],
    xterm: {
      background: "#272822",
      foreground: "#F8F8F2",
      cursor: "#F8F8F0",
      cursorAccent: "#272822",
      selectionBackground: "rgba(166, 226, 46, 0.25)",
      black: "#272822",
      red: "#F92672",
      green: "#A6E22E",
      yellow: "#F4BF75",
      blue: "#66D9EF",
      magenta: "#AE81FF",
      cyan: "#A1EFE4",
      white: "#F8F8F2",
      brightBlack: "#75715E",
      brightRed: "#F92672",
      brightGreen: "#A6E22E",
      brightYellow: "#F4BF75",
      brightBlue: "#66D9EF",
      brightMagenta: "#AE81FF",
      brightCyan: "#A1EFE4",
      brightWhite: "#F9F8F5",
    },
    chrome: { bg: "#272822", border: "#3E3D32", accent: "#A6E22E" },
  },
};

export const THEME_NAMES = Object.keys(THEMES) as ThemeName[];

function isThemeName(value: string): value is ThemeName {
  return Object.prototype.hasOwnProperty.call(THEMES, value);
}

/** Looks up profile.theme (falling back to ditty-dark for an unknown or
 * missing name), then applies profile.colors as a final override on top —
 * the same override path Terminal.tsx already supported pre-M6. */
export function resolveTheme(profile: Profile): ITheme {
  const base = THEMES[isThemeName(profile.theme) ? profile.theme : "ditty-dark"].xterm;
  if (Object.keys(profile.colors).length === 0) return base;
  return { ...base, ...profile.colors };
}

export function resolveChromeTint(profile: Profile): ChromeTint {
  return THEMES[isThemeName(profile.theme) ? profile.theme : "ditty-dark"].chrome;
}
