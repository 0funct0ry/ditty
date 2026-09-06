#!/usr/bin/env bash
# Integration smoke test (SPEC.md §14, M13): start the built binary, wait
# for /healthz, confirm the embedded UI is served, then drive one real
# ditty.v1 WebSocket round trip via scripts/smoke_client.go. Invoked by
# `make smoke` with DITTY_BIN set to the built binary.
set -euo pipefail

: "${DITTY_BIN:?set DITTY_BIN to the built ditty binary}"

PORT=$(( (RANDOM % 20000) + 20000 ))
TMPDIR=$(mktemp -d)
PID=""

cleanup() {
  if [[ -n "$PID" ]]; then
    kill "$PID" 2>/dev/null || true
    wait "$PID" 2>/dev/null || true
  fi
  rm -rf "$TMPDIR"
}
trap cleanup EXIT

"$DITTY_BIN" --insecure-no-auth -w -a 127.0.0.1 -p "$PORT" --no-token cat >"$TMPDIR/log" 2>&1 &
PID=$!

echo "smoke: waiting for /healthz on 127.0.0.1:${PORT}"
ok=0
for _ in $(seq 1 50); do
  if curl -fsS "http://127.0.0.1:${PORT}/healthz" >/dev/null 2>&1; then
    ok=1
    break
  fi
  sleep 0.1
done
if [[ "$ok" -ne 1 ]]; then
  echo "smoke: /healthz never became reachable" >&2
  cat "$TMPDIR/log" >&2 || true
  exit 1
fi

echo "smoke: checking the UI is served at /"
if ! curl -fsS "http://127.0.0.1:${PORT}/" | grep -qi "<title"; then
  echo "smoke: / did not serve the expected HTML" >&2
  exit 1
fi

echo "smoke: driving one ditty.v1 WebSocket round trip"
go run ./scripts/smoke_client.go -addr "127.0.0.1:${PORT}" -base-path "/"

echo "smoke: ok"
