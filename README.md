# ditty

Share a terminal over the web from a single Go binary. One command, one PTY, many browsers. Read-only by default.

```bash
go install github.com/0funct0ry/ditty@latest
ditty htop
```

## Building from source

```bash
make build   # bin/ditty
make run ARGS="-w bash"
make check   # every gate CI enforces
```

See `make help` for the full target list. This project is pre-v1.0.0; see the milestone plan for what's implemented so far.
