---
title: Themes
description: The six built-in terminal themes and how the Profile is resolved.
---

A Profile is resolved server-side (defaults ← config file ← flags), sent to the browser in the `Hello` frame, then optionally overridden per-browser and persisted in `localStorage` under `ditty.profile.v1`. The server value is a seed, not a lock — unless `--profile-lock` is set, which forces the server's values and hides the settings drawer entirely.

## The six themes

`--profile-theme` accepts:

- `ditty-dark` (default)
- `ditty-light`
- `nord`
- `dracula`
- `solarized-dark`
- `monokai`

They ship as Go structs in `internal/profile/themes.go` and are serialized into `Hello`, so switching themes in the browser never needs a network round trip.

## Other profile settings

```yaml
profile:
  theme: ditty-dark
  fontFamily: "JetBrains Mono, SF Mono, Menlo, monospace"
  fontSize: 14
  lineHeight: 1.2
  cursorStyle: block         # block | underline | bar
  cursorBlink: true
  scrollback: 5000           # browser-side lines, distinct from --scrollback-bytes
  renderer: webgl            # webgl | canvas | dom
  bellStyle: none            # none | sound | notification
  macOptionIsMeta: true
  altSendsEscape: true
  copyOnSelect: true
  rightClickPaste: false
  unicodeVersion: "11"
  eastAsianWidth: false
  colors: {}                 # optional 16-entry ANSI override
```

`renderer: webgl` falls back to `canvas` automatically if WebGL2 context creation fails, and a `Notice` tells you it happened.

## Keyboard policy

Browser chrome keeps working; the terminal still feels native:

- `Ctrl-C`: copies and clears an active selection; otherwise sends `0x03`.
- `Ctrl-V`: browser paste, then relayed as input. `Ctrl-Shift-V` always pastes.
- `Ctrl-Shift-C` always copies. `Cmd-C`/`Cmd-V` on macOS are always browser-native.
- `Alt-Tab`, `Alt-F4`, `Cmd-W`, `Ctrl-Shift-T`, `F5`, `F11`, `F12` are never captured by the terminal.
- Everything else goes to the PTY.
- `profile.keybindings` maps a chord to a byte sequence, e.g. `{ "ctrl+alt+l": "c" }` — up to ten entries.

## Locking the Profile

`--profile-lock` is for shared or public-facing sessions where you don't want every viewer picking their own theme and font size. It hides the settings drawer's button entirely, not just its contents.
