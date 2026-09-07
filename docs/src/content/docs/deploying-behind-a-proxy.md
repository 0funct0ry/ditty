---
title: Deploying behind a proxy
description: The two things a reverse proxy in front of ditty must get right — the WebSocket upgrade and the idle timeout.
---

Putting ditty behind nginx, Caddy, or a cloud load balancer works well, but two things have to be configured correctly or the terminal silently breaks.

## The WebSocket upgrade

`/ws` is a WebSocket upgrade, and most reverse proxies don't forward the headers that make an upgrade succeed unless you tell them to.

**nginx:**

```nginx
location /term/ {
    proxy_pass http://127.0.0.1:7654;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
}
```

**Caddy** does this automatically for any reverse_proxy block — no extra directive needed:

```txt
term.example.com {
    reverse_proxy 127.0.0.1:7654
}
```

Missing `Upgrade`/`Connection` headers is the single most common cause of "the terminal loads but stays blank" — see [Troubleshooting](/troubleshooting/).

## The idle timeout

ditty pings every `--ping-interval` (default 25s) to keep the connection alive, but proxies and load balancers often have their own idle timeout for WebSocket connections — commonly 60s. If the proxy's timeout is shorter than it takes ditty's pings to keep the connection warm from the proxy's point of view, you'll see disconnects on a regular cadence.

Set the proxy's timeout comfortably above `--ping-interval`, or lower `--ping-interval` to comfortably below the proxy's timeout. Either direction works; just make sure they're not close to each other.

```nginx
proxy_read_timeout 3600s;
```

## Origin and the trusted-header Grant

If you're terminating TLS and authentication at the proxy (SSO via oauth2-proxy, Cloudflare Access, an `auth_request` block), see the [behind nginx with SSO recipe](/recipes/behind-nginx-sso/) for the trusted-header Grant end to end, including why `--trust-proxy` has to be set explicitly.

`--origin-allow` defaults to same-host, which normally keeps working fine behind a reverse proxy as long as `Host` is forwarded correctly (`proxy_set_header Host $host;` above does this). If you're proxying from a different hostname than ditty itself expects, you may need to widen `--origin-allow` — see [Security](/security/#transport-and-origin) before you do.

## Next

- [Troubleshooting](/troubleshooting/) for the exact symptoms of getting any of this wrong.
- [Docker](/docker/) if the proxy is fronting a container instead of a bare process.
