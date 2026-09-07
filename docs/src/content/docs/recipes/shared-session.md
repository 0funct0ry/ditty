---
title: Shared interactive session
description: Multiple typists, one session, persistence across ditty restarts — all from tmux.
---

## The command

```sh
tmux new -s work
```

In another terminal, or later:

```sh
ditty -w tmux attach -t work
```

## What this gets you

Everyone who opens the URL attaches to the same `tmux` session. Multiple people can type — `tmux` itself handles the "whose keystrokes land where" problem, ditty just relays bytes. If ditty itself restarts or crashes, `tmux` and everything running inside it keeps going; the next `ditty -w tmux attach -t work` picks the same session back up.

## The risk

Every Client that connects has full write access to whatever the `tmux` session is running. There's no per-Client read-only/writable split within a single ditty invocation in v1.0.0 — if you need some viewers and some typists, run two ditty invocations: one `-w` against the `tmux` session for typists, one without `-w` for the rest, using [read-only broadcast](/recipes/read-only-broadcast/)'s approach against the same session via `tmux attach -r`.

## How it fails

- If the URL leaks, whoever has it can type into your `tmux` session exactly as if they were sitting at your keyboard.
- Closing the browser tab doesn't end the `tmux` session — you still need `tmux kill-session -t work` when you're actually done.
- If two people type at once, `tmux` doesn't arbitrate — output can interleave in a way that's confusing for both, the same as it would be for two people sharing a real terminal.
