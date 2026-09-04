# Manual testing matrix — M4 terminal core

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
