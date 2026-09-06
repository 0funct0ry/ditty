#!/bin/sh
# Installs the latest ditty release for this machine's OS/architecture.
#
#   curl -fsSL https://github.com/0funct0ry/ditty/releases/latest/download/install.sh | sh
#
# This downloads and runs a script from the network, which you should not
# do without understanding what it does. The safer alternative is to fetch
# it first and read it before running:
#
#   curl -fsSL https://github.com/0funct0ry/ditty/releases/latest/download/install.sh -o install.sh
#   less install.sh
#   sh install.sh
#
# It downloads the matching release archive, verifies it against the
# published SHA256SUMS, and installs the ditty binary to a directory on
# your PATH (/usr/local/bin when writable and running as root, otherwise
# $HOME/.local/bin).
set -eu

REPO="0funct0ry/ditty"

os() {
  case "$(uname -s)" in
    Linux) echo linux ;;
    Darwin) echo darwin ;;
    FreeBSD) echo freebsd ;;
    *) echo "install.sh: unsupported OS $(uname -s)" >&2; exit 1 ;;
  esac
}

arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo amd64 ;;
    arm64|aarch64) echo arm64 ;;
    *) echo "install.sh: unsupported architecture $(uname -m)" >&2; exit 1 ;;
  esac
}

OS=$(os)
ARCH=$(arch)

LATEST_URL="https://github.com/${REPO}/releases/latest/download"
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo "install.sh: fetching SHA256SUMS"
curl -fsSL "${LATEST_URL}/SHA256SUMS" -o "${TMPDIR}/SHA256SUMS"

ARCHIVE_PATTERN="ditty_.*_${OS}_${ARCH}\.tar\.gz"
ARCHIVE_NAME=$(grep -oE "$ARCHIVE_PATTERN" "${TMPDIR}/SHA256SUMS" | head -1)
if [ -z "$ARCHIVE_NAME" ]; then
  echo "install.sh: no release archive found for ${OS}/${ARCH} in SHA256SUMS" >&2
  exit 1
fi

echo "install.sh: fetching ${ARCHIVE_NAME}"
curl -fsSL "${LATEST_URL}/${ARCHIVE_NAME}" -o "${TMPDIR}/${ARCHIVE_NAME}"

echo "install.sh: verifying checksum"
(
  cd "$TMPDIR"
  grep " ${ARCHIVE_NAME}\$" SHA256SUMS | sha256sum -c -
)

tar -xzf "${TMPDIR}/${ARCHIVE_NAME}" -C "$TMPDIR" ditty

DEST="/usr/local/bin"
if [ "$(id -u)" != "0" ] || [ ! -w "$DEST" ]; then
  DEST="${HOME}/.local/bin"
  mkdir -p "$DEST"
fi

install -m 0755 "${TMPDIR}/ditty" "${DEST}/ditty"
echo "install.sh: installed ${DEST}/ditty"

case ":${PATH}:" in
  *":${DEST}:"*) ;;
  *) echo "install.sh: ${DEST} is not on your PATH; add it to your shell profile" ;;
esac
