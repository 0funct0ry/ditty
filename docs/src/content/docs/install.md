---
title: Install
description: Install ditty from Homebrew, a prebuilt binary, a container image, or go install.
---

ditty ships as a single static binary with no runtime dependencies. Pick whichever of these matches how you already manage tools.

## Homebrew (macOS and Linux)

```sh
brew install 0funct0ry/tap/ditty
```

## Prebuilt binary

Download the archive for your platform from the [latest release](https://github.com/0funct0ry/ditty/releases/latest), verify it against `SHA256SUMS`, and put the binary on your `PATH`:

```sh
curl -LO https://github.com/0funct0ry/ditty/releases/latest/download/ditty_linux_amd64.tar.gz
curl -LO https://github.com/0funct0ry/ditty/releases/latest/download/SHA256SUMS
sha256sum --ignore-missing -c SHA256SUMS
tar -xzf ditty_linux_amd64.tar.gz
sudo install ditty /usr/local/bin/ditty
```

Linux, macOS and FreeBSD are supported, on amd64 and arm64. There is no Windows build.

## Container image

```sh
docker pull ghcr.io/0funct0ry/ditty:latest
```

Two variants exist: the default is distroless (no shell in the image — ditty runs *your* Command, not one of its own), and `:latest-alpine` includes a shell for when the Command you want to share needs one. See [Docker](/docker/).

## go install

```sh
go install github.com/0funct0ry/ditty@latest
```

This builds from source with your local Go toolchain and puts `ditty` in `$GOBIN`.

## Verify

```sh
ditty version
```

## Next

Continue to [Quickstart](/quickstart/).
