# syntax=docker/dockerfile:1
#
# Two final images share this file: the default (last FROM, distroless, no
# shell — ditty runs *your* Command, not a shell of its own) and `alpine`
# (named exactly "alpine" so `make docker-alpine`'s --target finds it), for
# cases that want a shell present in the image itself. See SPEC.md §15 and
# CLAUDE.md; built only via `make docker` / `make docker-alpine`, never a
# bare `docker build` by hand.

# ---- web build --------------------------------------------------------------
FROM node:20-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ---- go build -----------------------------------------------------------------
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags "-s -w \
      -X github.com/0funct0ry/ditty/internal/buildinfo.Version=${VERSION} \
      -X github.com/0funct0ry/ditty/internal/buildinfo.Commit=${COMMIT} \
      -X github.com/0funct0ry/ditty/internal/buildinfo.Date=${DATE}" \
      -o /out/ditty .

# ---- alpine (shell) final -----------------------------------------------------
FROM alpine:3.20 AS alpine
RUN apk add --no-cache ca-certificates && \
    addgroup -S ditty && adduser -S ditty -G ditty
COPY --from=build /out/ditty /usr/local/bin/ditty
USER ditty
ENV DITTY_ADDRESS=0.0.0.0
EXPOSE 7654
# The healthcheck only probes a plaintext-reachable /healthz on 127.0.0.1;
# a TLS-only listener (--tls-cert/--tls-key) is not probed generically.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
  CMD ["/usr/local/bin/ditty", "__healthcheck"]
ENTRYPOINT ["/usr/local/bin/ditty"]

# ---- distroless (default, no shell) final -------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot AS distroless
COPY --from=build /out/ditty /usr/local/bin/ditty
USER nonroot:nonroot
ENV DITTY_ADDRESS=0.0.0.0
EXPOSE 7654
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
  CMD ["/usr/local/bin/ditty", "__healthcheck"]
ENTRYPOINT ["/usr/local/bin/ditty"]
