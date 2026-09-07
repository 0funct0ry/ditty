---
title: Configuration
description: Precedence order, config file search path, environment variables, and the full flag surface.
---

## Precedence

Lowest to highest: **built-in defaults → config file → environment → command-line flags.** Every flag has an environment twin and a config file key, wired through Viper.

```sh
ditty config init             # write a fully commented config file
ditty config show --resolved  # print the effective config, with each value's origin
```

`config show --resolved` is the fastest way to answer "why is this flag set to that" — it tells you whether a value came from a default, the config file, the environment, or a flag.

## Config file search order

1. `--config <path>`, if given
2. `./ditty.yaml`
3. `$XDG_CONFIG_HOME/ditty/ditty.yaml`
4. `/etc/ditty/ditty.yaml`

## Environment variables

Prefix `DITTY_`, with dots replaced by underscores: `profile.fontSize` becomes `DITTY_PROFILE_FONTSIZE`.

## Flag surface

See the [CLI reference](/reference/cli/run/) for every flag, its default, and its short form — generated directly from the Cobra command tree, so it can't drift from what the binary actually accepts.

The flags are grouped, at a glance:

- **Server** — listen address, port, socket, TLS, base path, `--ping-interval`, `--max-clients`, `--open`.
- **Access** — `-w`/`--writable`, every [Grant](/security/#grant-types), `--origin-allow`, `--allow-iframe`, `--insecure-no-auth`.
- **Command and session** — working directory, environment, `--term`, `--uid`/`--gid`, naming and titling, sizing, scrollback, output coalescing, lifecycle (`--once`, `--exit-on-detach`, `--wait-for-client`, `--detach-grace`, `--close-on-exit`, `--kill-signal`), and the two command-injection surfaces (`--allow-url-args`, `--header-env`).
- **UI and logs** — index/favicon overrides, `--profile-*`, `--profile-lock`, `--focus`, reconnect behavior, logging verbosity and format.

## SQLite auth (`--auth-db`)

No SQLite file is created, opened, or required unless `--auth-db` is passed explicitly. With it:

```sh
ditty users add alice --role operator
ditty users list
ditty users passwd alice
ditty users disable alice
```

`role=operator` gets write capability when `-w` is set; `role=viewer` never gets write capability, even with `-w`. Passwords are always prompted interactively — `users add` never accepts one as a flag.

## Next

- [CLI reference](/reference/cli/run/) for the exhaustive flag list.
- [Security](/security/) for what each Grant actually protects against.
