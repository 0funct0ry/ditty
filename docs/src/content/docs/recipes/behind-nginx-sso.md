---
title: Behind nginx with SSO
description: Delegate authentication to a reverse proxy that already handles SSO, over a Unix socket.
---

## The command

```sh
ditty --socket /run/ditty.sock --base-path /term \
  --trust-proxy 127.0.0.1/32 --trust-header X-Auth-Request-Email \
  bash
```

## The proxy snippet

```nginx
location /term/ {
    proxy_pass http://unix:/run/ditty.sock;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Auth-Request-Email $upstream_http_x_auth_request_email;
}
```

(Adjust `X-Auth-Request-Email` to whatever header your SSO layer — oauth2-proxy, Cloudflare Access, an auth_request block — actually sets after it authenticates the request.)

## What this gets you

ditty listens only on a Unix socket, not a network address, so the bind guard never even applies at the ditty layer — nginx is the only thing that can reach it. `--trust-proxy 127.0.0.1/32` tells ditty to honor `--trust-header` only for connections that arrive from nginx itself, so the header can't be forged by talking to ditty directly (there's nothing to talk to directly; it's a socket file). The Client's label comes from whatever your SSO puts in that header.

## The risk

Everything hinges on nginx being the only thing that can write to the socket and set that header. If `/run/ditty.sock` has loose permissions, or if something else on the box can reach it, the trust-header check is bypassed entirely — `--trust-proxy` only restricts by peer address, and a Unix socket connection's peer address is meaningless for that check the way a TCP one is, so socket file permissions are doing real work here (`--socket-mode`/`--socket-owner`).

## How it fails

- Without the `Upgrade`/`Connection` headers in the nginx block, the WebSocket upgrade fails and the terminal never connects — see [Troubleshooting](/troubleshooting/) for exactly this symptom.
- If nginx's `auth_request` (or equivalent) isn't actually wired up ahead of this location block, `X-Auth-Request-Email` is empty or absent, and ditty treats the request as having no identity from that Grant — combine this with at least one other Grant, or a bind guard override you actually mean, rather than assuming the header check alone gates access.
- Forgetting `--trust-proxy` (leaving it at its empty default) makes `--trust-header` completely inert — ditty logs a `WARN` once at startup precisely for this case.
