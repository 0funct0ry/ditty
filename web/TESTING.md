# Manual testing matrix — M4 terminal core, M5 chrome & states

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
