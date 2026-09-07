#!/usr/bin/env bash
# Verifies the exact commands published in the §11 recipe pages against a
# built ditty binary (SPEC.md §12 M14 acceptance: "every recipe's command
# copy-pasted from the docs actually works").
#
# Recipes 1 (tmux), 2 (read-only broadcast), 4 (nginx+SSO — the ditty side:
# --socket, --trust-proxy, --trust-header) and 5 (one-shot support link) are
# started for real against DITTY_BIN and probed over /healthz. Recipe 3
# (docker sandbox) needs a container runtime that isn't assumed to be
# present in every CI runner, so it gets a dry-run check only. Recipe 4's
# nginx config snippet is additionally syntax-checked with `nginx -t` when
# nginx is installed, but ditty is never actually driven through a real
# nginx proxy here — see internal-docs/walkthroughs/walkthrough-M14.md for
# that gap and its one-time manual verification record.
set -euo pipefail

: "${DITTY_BIN:?set DITTY_BIN to a built ditty binary}"

PORT=17654
fail=0

log() { printf '[verify-recipes] %s\n' "$1"; }

wait_healthy() {
  for _ in $(seq 1 30); do
    if curl -fsS "http://127.0.0.1:$PORT/healthz" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.2
  done
  return 1
}

wait_healthy_socket() {
  local sock="$1" path="$2"
  for _ in $(seq 1 30); do
    if curl -fsS --unix-socket "$sock" "http://localhost${path}healthz" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.2
  done
  return 1
}

run_full() {
  local name="$1"; shift
  log "recipe: $name — $*"
  "$@" >"/tmp/verify-recipe-$name.log" 2>&1 &
  local pid=$!
  if wait_healthy; then
    log "recipe: $name — /healthz OK"
  else
    log "recipe: $name — FAILED (no healthy response)"
    cat "/tmp/verify-recipe-$name.log" || true
    fail=1
  fi
  kill "$pid" >/dev/null 2>&1 || true
  wait "$pid" 2>/dev/null || true
}

# --- Recipe 1: shared interactive session -----------------------------------
if command -v tmux >/dev/null 2>&1; then
  tmux new -d -s ditty-verify-work
  run_full shared-session "$DITTY_BIN" -p "$PORT" -a 127.0.0.1 --no-token -w tmux attach -t ditty-verify-work
  tmux kill-session -t ditty-verify-work >/dev/null 2>&1 || true
else
  log "recipe: shared-session — SKIPPED (tmux not installed)"
fi

# --- Recipe 2: read-only broadcast ------------------------------------------
run_full read-only-broadcast "$DITTY_BIN" -p "$PORT" -a 127.0.0.1 --no-token htop

# --- Recipe 3: sandboxed shell (syntax/dry-run only) ------------------------
if command -v docker >/dev/null 2>&1; then
  if docker run --rm --network none --memory 256m alpine sh -c 'exit 0'; then
    log "recipe: sandboxed-shell — dry-run OK (docker run --network none --memory 256m alpine sh)"
  else
    log "recipe: sandboxed-shell — FAILED dry-run"
    fail=1
  fi
else
  log "recipe: sandboxed-shell — SKIPPED (docker not installed); verified manually, see walkthrough-M14.md"
fi

# --- Recipe 4: behind nginx with SSO ----------------------------------------
# Runs the ditty side of the recipe for real (--socket, --base-path,
# --trust-proxy, --trust-header), probed directly over the Unix socket —
# this does not require nginx to be installed. The nginx config snippet
# itself is additionally syntax-checked with `nginx -t` when nginx is
# available, but no real nginx proxy is put in front of ditty here.
sock="/tmp/ditty-verify-nginx-sso.sock"
rm -f "$sock"
log "recipe: behind-nginx-sso — $DITTY_BIN --socket $sock --base-path /term --trust-proxy 127.0.0.1/32 --trust-header X-Auth-Request-Email --no-token --insecure-no-auth bash"
"$DITTY_BIN" --socket "$sock" --base-path /term --trust-proxy 127.0.0.1/32 \
  --trust-header X-Auth-Request-Email --no-token --insecure-no-auth bash \
  >"/tmp/verify-recipe-behind-nginx-sso.log" 2>&1 &
pid=$!
if wait_healthy_socket "$sock" "/term/"; then
  log "recipe: behind-nginx-sso — /healthz over the Unix socket OK"
else
  log "recipe: behind-nginx-sso — FAILED (socket not healthy)"
  cat "/tmp/verify-recipe-behind-nginx-sso.log" || true
  fail=1
fi
kill "$pid" >/dev/null 2>&1 || true
wait "$pid" 2>/dev/null || true
rm -f "$sock"

if command -v nginx >/dev/null 2>&1; then
  cfg=$(mktemp)
  cat >"$cfg" <<'EOF'
events {}
http {
  server {
    listen 8080;
    location /term/ {
      proxy_pass http://unix:/run/ditty.sock;
      proxy_http_version 1.1;
      proxy_set_header Upgrade $http_upgrade;
      proxy_set_header Connection "upgrade";
      proxy_set_header Host $host;
      proxy_set_header X-Auth-Request-Email $upstream_http_x_auth_request_email;
    }
  }
}
EOF
  if nginx -t -c "$cfg" 2>&1; then
    log "recipe: behind-nginx-sso — nginx -t OK"
  else
    log "recipe: behind-nginx-sso — FAILED nginx -t"
    fail=1
  fi
  rm -f "$cfg"
else
  log "recipe: behind-nginx-sso — nginx config syntax check SKIPPED (nginx not installed)"
fi

# --- Recipe 5: one-shot support link -----------------------------------------
run_full one-shot-support-link "$DITTY_BIN" -p "$PORT" -a 127.0.0.1 --no-token --once --detach-grace 5s --wait-for-client 5s -w bash

if [[ "$fail" -ne 0 ]]; then
  log "one or more recipes failed verification"
  exit 1
fi
log "all recipes verified"
