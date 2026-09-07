---
title: Troubleshooting
description: The specific symptoms people actually hit, and what causes them.
---

## "ditty: refusing to listen on ... without authentication"

This is the [bind guard](/security/#the-three-invariants) doing its job. You've bound to something other than loopback or a Unix socket without configuring a Grant. Pick one of the options the message lists — `--token` is usually the right one — or pass `--insecure-no-auth` if you genuinely mean it.

## Blank terminal behind a proxy

The page loads, the chrome bar appears, but the terminal itself never shows anything and stays in `connecting`. This almost always means the WebSocket upgrade never completed — the reverse proxy in front of ditty isn't forwarding the `Upgrade`/`Connection` headers. See [Deploying behind a proxy](/deploying-behind-a-proxy/#the-websocket-upgrade) for the exact nginx and Caddy configuration.

## `403` on the WebSocket upgrade

This is the origin check ([Security](/security/#transport-and-origin)) rejecting a cross-origin upgrade — the default, on-by-purpose defense against cross-site WebSocket hijacking. If you're intentionally serving the UI from one origin and connecting to ditty from another, widen `--origin-allow`; otherwise this usually means `Host` isn't being forwarded correctly by whatever sits in front of ditty.

## Garbled box-drawing characters / broken rendering

Usually a font or `TERM` mismatch rather than a ditty bug. Check that:

- the browser's font actually supports the box-drawing and Unicode ranges the Command is emitting (a system monospace font without full glyph coverage is a common cause)
- `--term` matches what the Command expects (`xterm-256color` is the default and correct for almost everything; a Command that assumes `screen-256color` or similar may need `--term` set explicitly)

## Keystrokes are ignored

You're in read-only mode. This is [I1](/security/#the-three-invariants) working as designed — `Input` frames are dropped server-side, not just hidden client-side. The chrome bar always states the current mode; if it says "read-only" and you need to type, restart with `-w`/`--writable`.

## Disconnects every 60 seconds

A proxy or load balancer's idle timeout is shorter than the interval ditty's own keepalive pings need to keep the connection warm from the proxy's perspective. See [Deploying behind a proxy](/deploying-behind-a-proxy/#the-idle-timeout) — either raise the proxy's timeout above `--ping-interval`, or lower `--ping-interval` well below the proxy's timeout.

## Still stuck?

Check `ditty config show --resolved` to confirm the flags you think are active actually are — a config file or an environment variable can silently override what you passed on the command line, and this shows you the origin of every value. Open an issue at [github.com/0funct0ry/ditty/issues](https://github.com/0funct0ry/ditty/issues) if none of the above matches what you're seeing.
