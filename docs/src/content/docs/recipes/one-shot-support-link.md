---
title: One-shot support link
description: A link that works exactly once, then self-destructs.
---

## The command

```sh
ditty --once --detach-grace 30s --wait-for-client 5m -w bash
```

## What this gets you

`--wait-for-client 5m` gives you five minutes to actually send the link and have someone open it before ditty gives up and exits on its own. `--once` means the Session ends as soon as the one Client that connects detaches — no second visit to the same URL. `--detach-grace 30s` gives a brief window for a flaky connection to reconnect before treating a detach as final, so a dropped Wi-Fi packet doesn't end the whole session.

## The risk

`-w` means whoever you send this link to can run anything you can run, for as long as the session lasts. This recipe controls *how long the door stays open*, not *what someone can do while it's open* — treat the link itself like a one-time password: don't post it anywhere more public than the one person you're sending it to.

## How it fails

- If nobody connects within `--wait-for-client`, ditty exits and the link goes dead — regenerate a new one rather than trying to reuse the old URL.
- If the one Client detaches and doesn't reconnect within `--detach-grace`, the session closes — a second attempt at the same URL gets `rejected`, not a fresh session.
- This is still a single Command run once — it's not a durable support-ticket system with its own expiry tracking. For anything longer-lived than "someone is on a call with me right now", the SQLite auth backend's `grants` table is the better fit (see [Configuration](/configuration/)).
