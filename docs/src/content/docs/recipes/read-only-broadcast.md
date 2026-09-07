---
title: Read-only broadcast
description: Show a log tail or a live dashboard to anyone with the URL, with no way for them to type into it.
---

## The command

```sh
ditty tail -f /var/log/app.log
```

or

```sh
ditty htop
```

## What this gets you

ditty's default mode — no `-w` — is exactly this: any Command's output relayed live to every browser that opens the URL, with `Input` frames dropped server-side before they reach the PTY. There's nothing extra to configure; the default read-only behavior is the entire recipe.

## The risk

Whatever the Command prints is visible to anyone with the URL — including anything sensitive that scrolls past in a log file or a process list. `--scrollback-bytes 0` disables the output ring if you don't want reconnecting Clients to be able to replay history they weren't watching live.

## How it fails

- A Client typing into the terminal sees a one-shot toast ("This session is read-only") and nothing is sent — this is enforced in the Hub, not hidden in the UI, so there's no client-side way around it.
- If the log or process being shown itself contains secrets (an access token in a log line, a password flashed in a process list), read-only mode doesn't redact it — it's still fully visible to every watcher.
- Log rotation or the tailed file disappearing ends the Command; ditty shows the exit state, it doesn't restart the tail for you.
