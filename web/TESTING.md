# Manual testing matrix — M4 terminal core, M5 chrome & states, M6 settings & themes

Run against the real fixture Hub from M3, never against manually-toggled
mock state — that's the whole point of Phase 2 rehearsing against a real
`ditty.v1` server.

## Setup

```bash
make dev-fixture NAME=deploy   # builds with the fixture tag and starts ditty run --fixture deploy
```

In a second terminal:

```bash
cd web && DITTY_DEV_BACKEND=http://127.0.0.1:7654 npm run dev
```

Open the printed Vite URL. The dev server proxies `/ws` (and `/api`,
`/healthz`) to the fixture Hub, so the UI behaves exactly as it will once
embedded (SPEC.md §10.4 dev-mode proxying).

## Matrix

Repeat `make dev-fixture NAME=<scenario>` for each scenario below and
reload the UI against it.

| Scenario     | What to verify |
|--------------|----------------|
| `deploy`     | Long colourized build/rollout log renders correctly — ANSI colors, no dropped output, scrollback works. This is also the visual reference for chrome/type in M5. |
| `htop`       | Full-screen redraws (cursor positioning, screen clears) render without flicker or corruption — this exercises the renderer beyond simple append-only output. |
| `flaky`      | The connection actually drops on the scenario's schedule. Confirm: the terminal does **not** clear on disconnect (last frame stays frozen), a `reconnecting` overlay appears, and once the fixture reconnects, the ring-buffer replay repaints history correctly. Verify the state transition is `live -> reconnecting -> live`, not a simulated timeout. |
| `quickexit`  | The scenario exits non-zero a few seconds in. Confirm a real `Exit` frame arrives, the UI shows the exit code, and no reconnect is attempted afterward (state settles on `closed`). |

## Other checks

- **Resize**: resize the browser window; confirm a `Resize` frame is sent
  only when this client is the sizing client (check via `--max-clients`
  with two tabs open — the second tab should letterbox instead of
  resizing).
- **Read-only**: with a scenario that reports `writable: false` in `Hello`,
  type into the terminal — confirm a one-shot inline toast appears reading
  "This session is read-only" and no `Input` frame is sent (check the
  Network tab / a wire-level log).
- **Unknown/malformed frames**: not reachable from a real fixture; covered
  by the Vitest suite instead (`connection.test.ts`).
- **Bundle budget**: `make web-build && make bundle-check` — must stay
  under 400 KB gzipped (SPEC.md §10.4).

## M5 — chrome, states, accessibility

Same setup as above; the chrome bar and status bar are always on screen,
so every scenario in the matrix above also exercises them (dimensions,
protocol, sizing attribution, elapsed timer, the access badge).

| Check | How |
|-------|-----|
| `connecting` | Reload the page against any scenario; catch the dimmed terminal + "Connecting to `<session>`" overlay before `Hello` arrives. |
| `live` | Any scenario once `Hello` lands — full opacity, dot pulses once then holds still, badge reads `read-only` (fixture never grants write). |
| `reconnecting` | `make dev-fixture NAME=flaky` — attach *before* the scripted drop (a few seconds after starting the fixture). Confirm the frame freezes (not cleared), the overlay shows a live countdown ("Reconnecting in Ns · press Enter to retry now"), and both Enter and the "Retry now" button reconnect immediately. |
| `closed` | `make dev-fixture NAME=quickexit` — confirm the real `Exit` frame drives "`<title>` exited (code N)" with the actual duration, "Copy transcript" copies the terminal's text, and there is no reconnect attempt. |
| `rejected` | `make dev-fixture NAME=deploy ARGS="--max-clients 1"`, open one tab (attaches), then a second tab — confirm the second gets "This session is full." with a "Try again" button, driven by a real `1013` close code, not a simulated state. |
| roster note | With two tabs attached to the same scenario, close one — confirm the other's status bar shows a transient "Client N left" note that fades on its own (no toast stack). |
| Keyboard-only | Tab through the chrome bar (dot is not focusable, settings and share buttons are) — every control must be reachable and show a visible focus ring. |
| `prefers-reduced-motion` | Emulate `reduce` (OS setting, or your browser devtools' rendering-emulation panel) and reload — the dot must not pulse and the roster note must not fade; confirm via `index.css`'s reduced-motion override if the emulation isn't available in your browser tooling. |
| 380px layout | Narrow the window to 380px — chrome collapses to icon-only (title and Client count hide), the terminal keeps full height, nothing overflows horizontally. |
| Lighthouse a11y | Run a Lighthouse audit against the `live` state; target ≥ 95. |
| Screenshots | Capture each state (`connecting`, `live`, `reconnecting`, `closed`, `rejected`) at 1440px and 380px into `docs/public/screens/` — these double as docs/marketing assets later (SPEC.md §12 M5). |

## M6 — settings drawer & themes

Same setup as above (`make dev-fixture NAME=deploy` is enough for most of
this; a few rows need extra fixture flags as noted).

| Check | How |
|-------|-----|
| Theme swatches | Open the drawer (gear icon), click each of the six swatches. Confirm the terminal background/foreground/ANSI colors and the chrome bar's tint change together, instantly, with no reload and no `Resize` frame sent (fixture has no PTY to resize — check the Network tab shows nothing new). |
| Font family / size | Change the font `<select>` and the size stepper — confirm the terminal re-renders live at the new font/size. |
| Cursor style / blink | Change cursor style and toggle blink — confirm the terminal cursor updates live. |
| Renderer | Switch WebGL ⇄ Canvas — confirm no visible resize/reflow of the terminal; if WebGL is unavailable in your browser, confirm the "renderer: canvas (WebGL unavailable)" badge still shows and the renderer select doesn't fight it. |
| Copy on select | Toggle on, select text in the terminal, confirm it's copied to the clipboard; toggle off, confirm selecting no longer copies. |
| Bell | `make dev-fixture NAME=deploy ARGS="--profile-bell visual"` (or toggle "Bell notification" in the drawer) then trigger a BEL byte from the scenario/PTY — confirm a brief flash; with the toggle off, confirm nothing happens. |
| Persistence | Change several settings, reload the page — confirm they survive via `localStorage['ditty.profile.v1']`. |
| Reset to server defaults | After changing settings, click "Reset to server defaults" — confirm the Profile returns exactly to what `Hello.profile` seeded, and `localStorage` is cleared. |
| `--profile-lock` | `make dev-fixture NAME=deploy ARGS="--profile-lock --profile-theme monokai"` — confirm the settings gear button is entirely **absent** from the chrome bar (not just disabled), and the seeded theme/flags apply regardless of any stale `localStorage` override from a previous session. |
| No reflow | With the drawer open, confirm the terminal's bounding box (and its `cols`×`rows` in the status bar) does not change — the drawer overlays, it never resizes the PTY viewport. |
| Share sheet | Click the share icon — confirm it shows the current URL, a working Copy button, and one honest sentence about access (`They cannot type` for a read-only fixture session). |
| Keyboard-only | Tab into the drawer/share sheet; every control (swatches, selects, stepper, switches, reset, close) must be reachable with a visible focus ring; `Escape` closes either. |
| `prefers-reduced-motion` | Emulate `reduce` — the drawer's slide-in must not animate (see `index.css`'s `.settings-drawer` reduced-motion override). |
| 380px layout | Narrow the window to 380px — the drawer becomes full-width, still overlays rather than reflowing the terminal. |
| Screenshots | Capture the drawer open and the share sheet open at 1440px and 380px into `docs/public/screens/`. |
| Bundle budget | `make web-build` — must stay under 400 KB gzipped; the six themes and the drawer are pure data/markup, so this shouldn't move much (SPEC.md §10.4). |
