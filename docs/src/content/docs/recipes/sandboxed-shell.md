---
title: Sandboxed shell for someone you don't trust
description: Give an untrusted person a shell without giving them your shell.
---

## The command

```sh
ditty -w docker run --rm -it --network none --memory 256m alpine sh
```

## What this gets you

ditty has no sandbox of its own — see [Security](/security/#privilege) — so the sandbox comes from making the Command itself a throwaway container instead of a shell on your own machine. `--network none` denies the container any network access, `--memory 256m` caps what it can consume, `--rm` guarantees it's deleted the moment it exits, and `alpine sh` is a minimal, disposable shell with nothing of yours in it.

## The risk

This limits the blast radius to "inside a memory-capped, network-isolated container", not to zero. A container escape is still a container escape; `--network none` and `--memory` are resource and network controls, not a substitute for not trusting the person at all. Don't mount your filesystem into it (no `-v`), and don't relax `--network none` unless you've thought through why.

## How it fails

- If someone finds a container escape, they're now on the host machine that ran `docker run`, not just in the sandbox — Docker isolation is strong but not absolute.
- `--rm` only deletes the container on a clean exit; a killed Docker daemon or host crash can leave it behind — clean up with `docker ps -a` afterward if you're not sure.
- This recipe is still `-w` writable end to end: the person at the URL can do anything inside that container that its user can do, including anything the container image itself makes available (network tools if you didn't actually keep `--network none`, package managers, etc).
