---
title: ditty run
description: CLI reference for ditty run, generated from the Cobra command tree.
---

## ditty run

Start a Session and share it over the web

```
ditty run [flags] -- <command> [args...]
```

### Options

```
  -a, --address string                address to bind (default "127.0.0.1")
      --allow-iframe string[="*"]     allow this session to be framed: omitted defaults to frame-ancestors 'none', a value sets that CSP frame-ancestors source, and the bare flag with no value omits frame-ancestors entirely (allows any framer)
      --allow-url-args                allow ?arg= query values to be appended to the Command's argv (SPEC.md §6.4)
      --arg-pattern string            regex every --allow-url-args value must match (default "^[A-Za-z0-9._/-]{1,64}$")
      --auth-db ditty users           path to a SQLite file for username/password login (SPEC.md §9); manage users with ditty users
  -b, --base-path string              URL path prefix to mount the UI and API under (default "/")
      --basic-auth string             require HTTP basic authentication as user:pass (also DITTY_BASIC_AUTH)
  -u, --chunk-bytes int               PTY read buffer size in bytes, before coalescing (default 32768)
      --client-ca string              CA certificate file; requires and verifies a client certificate (mutual TLS)
  -X, --close-on-exit                 when the Command exits, show the closed state instead of offering reconnect (default true)
  -y, --cols uint16                   fixed PTY column count (0 = dynamic, sized by the first Client)
  -d, --cwd string                    working directory for the Command (default: ditty's own cwd)
  -g, --detach-grace duration         after the last Client detaches, wait this long before signalling the Command (0 = never)
  -E, --env stringArray               additional KEY=VAL environment variable for the Command (repeatable)
  -x, --exit-on-detach                shorthand for --detach-grace 0s plus exiting the process
  -m, --flush-interval duration       coalesce PTY output for this long before fanning it out (default 5ms)
  -j, --focus                         hide the chrome bar and status bar entirely, leaving only the terminal
  -G, --gid string                    run the Command as this numeric gid (requires ditty to run as root)
      --header-env stringArray        map a request header into the Command's environment as Header:TARGET (repeatable, SPEC.md §6.4)
  -h, --help                          help for run
      --insecure-basic-over-http      allow --basic-auth over plaintext on a non-loopback address (credentials travel in the clear)
      --insecure-no-auth              start without any Grant even on a non-loopback address (SPEC.md §6.1 I2); logs a WARN every 60s
      --jwt-secret string             HS256 signing secret for --auth-db sessions; a random per-run secret is used when unset, invalidating every session on restart
  -K, --kill-signal string            signal sent to the process group on close (SIGKILL after a fixed 5s escalation) (default "SIGHUP")
  -L, --log-file string               write logs to this file instead of stderr
  -f, --log-format string             log output format: text or json (default "text")
  -M, --max-clients int               reject Clients past this many simultaneous attachments (0 = unlimited); --shared only
  -N, --name string                   short Session name, used in URLs and titles
      --no-token                      disable the default-on URL token (SPEC.md §6.1 I3)
  -Z, --once                          accept exactly one Client, ever, across the whole run; ditty itself exits once that Client's Command ends (matches gotty's --once)
  -o, --open                          open the default browser once the server starts
      --origin-allow string           regex the WebSocket Origin header must match (default: same host as the request)
      --ping-interval duration        interval between server-initiated WebSocket pings; the read deadline is 2x this plus 5s (default 25s)
  -p, --port int                      port to listen on (0 = random, printed at startup) (default 7654)
  -e, --profile-bell string           bell style: none, sound, visual (default "none")
  -C, --profile-copy-on-select        copy selected text to the clipboard automatically (default true)
  -k, --profile-cursor-blink          whether the cursor blinks (default true)
  -c, --profile-cursor-style string   cursor style: block, underline, bar (default "block")
  -F, --profile-font-family string    terminal font family (default "JetBrains Mono, SF Mono, Menlo, monospace")
  -s, --profile-font-size int         terminal font size in pixels (default 14)
  -l, --profile-lock                  force these Profile values and hide the settings drawer entirely
  -R, --profile-renderer string       terminal renderer: webgl, canvas (default "webgl")
  -t, --profile-theme string          terminal theme: ditty-dark, ditty-light, nord, dracula, solarized-dark, monokai (default "ditty-dark")
  -q, --quiet                         suppress all but warning/error logs
  -z, --rows uint16                   fixed PTY row count (0 = dynamic, sized by the first Client)
  -S, --scrollback-bytes int          ring buffer capacity in bytes (0 disables it) (default 262144)
      --shared                        share one Command/PTY across every Client, fanning out its output (ditty's pre-M10 default). Without this flag (the default), every Client gets its own fresh Command, spawned on connect and torn down when that Client disconnects — one Client's Command exiting never affects any other Client, and a page refresh always starts a brand-new Command, with no reconnect/replay. --max-clients, --wait-for-client, --detach-grace and --exit-on-detach only apply in --shared mode.
      --socket string                 listen on this Unix socket path instead of --address/--port (SPEC.md §8.1); mutually exclusive with both
      --socket-mode string            file mode for the Unix socket, e.g. 0600 (--socket only) (default "0600")
      --socket-owner string           user[:group] to chown the Unix socket to (--socket only)
  -T, --term string                   TERM value set for the Command (default "xterm-256color")
  -i, --title string                  Session title template (default "{command} — {hostname}")
      --tls-cert string               TLS certificate file; enables HTTPS
      --tls-key string                TLS private key file; enables HTTPS
      --token string[="-"]            require this URL token (SPEC.md §4.1); bare --token generates a random one
      --token-length int              random token length in bytes when --token generates its own (minimum 8) (default 16)
      --trust-header string           trust this request header for the Client's identity, e.g. X-Auth-User
      --trust-proxy strings           CIDR(s) whose peers may set --trust-header (required for --trust-header to have any effect)
  -U, --uid string                    run the Command as this numeric uid (requires ditty to run as root)
  -v, --verbose count                 increase log verbosity (repeatable)
  -W, --wait-for-client duration      exit if no Client attaches within this duration (0 = forever)
  -w, --writable                      allow attached Clients to type into the Command (SPEC.md §6.1 I1)
```

### SEE ALSO

* [ditty](/reference/cli/ditty/)	 - Share a terminal over the web from a single binary

###### Auto generated by spf13/cobra on 7-Sep-2026
