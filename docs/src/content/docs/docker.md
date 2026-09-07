---
title: Docker
description: The two ditty container images, the entrypoint's env and secrets handling, and three compose examples.
---

## Two images, one purpose

```sh
docker pull ghcr.io/0funct0ry/ditty:latest          # distroless — no shell in the image
docker pull ghcr.io/0funct0ry/ditty:latest-alpine    # alpine — has a shell, for shell-based Commands
```

The default image is built `FROM gcr.io/distroless/static-debian12:nonroot` — there is no shell inside it at all, which is deliberate: ditty's job is to run *your* Command, not to carry one of its own around as an attack surface. Use `:alpine` only when the Command you want to share needs a shell present in the same image (for example, a Command that itself invokes `sh -c '...'`).

Both images:

- run as a non-root user by default
- expose port `7654`
- carry a `HEALTHCHECK` against `/healthz`
- are published multi-arch, amd64 and arm64

```sh
docker run --rm -p 7654:7654 ghcr.io/0funct0ry/ditty:latest --insecure-no-auth htop
```

## Environment and secrets

The entrypoint reads every `DITTY_*` environment variable ([Configuration](/configuration/) has the full mapping), plus a small set of `_FILE`-suffixed variables that point at a file instead — the standard Docker-secrets convention:

```yaml
environment:
  DITTY_BASIC_AUTH_FILE: /run/secrets/ditty_basic_auth
  DITTY_JWT_SECRET_FILE: /run/secrets/ditty_jwt_secret
```

The container refuses to start unauthenticated — the same [bind guard](/security/#the-three-invariants) applies here as anywhere else — unless `--insecure-no-auth` is passed explicitly. On startup it prints the resolved access mode, the same as running the binary directly.

## Three compose recipes

The repository's `docker-compose.yml` ships three profiles, each a demonstration rather than production hardening advice on its own:

```sh
docker compose --profile monitor up      # read-only broadcast
docker compose --profile shell-tls up    # authenticated shell behind TLS
docker compose --profile nginx up        # nginx-fronted, trusted-header auth
```

**`monitor`** — a read-only `htop` broadcast on `--insecure-no-auth`, appropriate only as a local demo (see [Read-only broadcast](/recipes/read-only-broadcast/) for the real recipe without `--insecure-no-auth`).

**`shell-tls`** — a writable `bash` session gated by HTTP basic auth, with the password supplied via a Docker secret (`DITTY_BASIC_AUTH_FILE`) rather than a plain environment variable, served over HTTPS from a mounted certificate pair.

**`nginx`** — ditty trusts an `X-Auth-User` header set by an nginx service in front of it (which would itself sit behind a real SSO layer in a genuine deployment), configured with `DITTY_TRUST_PROXY` restricted to nginx's address on the compose network — see [Behind nginx with SSO](/recipes/behind-nginx-sso/) for the full reasoning.

## Next

- [Deploying behind a proxy](/deploying-behind-a-proxy/) if you're fronting the container with your own reverse proxy instead of the bundled nginx example.
- [Security](/security/) before choosing `--insecure-no-auth` for anything but a local demo.
