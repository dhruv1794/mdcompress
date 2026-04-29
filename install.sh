#!/bin/sh
# mdcompress installer.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/dhruv1794/mdcompress/main/install.sh | sh
#
# Environment overrides:
#   MDCOMPRESS_VERSION  Tag to install (default: latest GitHub release).
#   MDCOMPRESS_INSTALL_DIR  Directory to install into (default: /usr/local/bin).
#   GITHUB_TOKEN  Optional token to lift GitHub API rate limits.

set -eu

REPO="dhruv1794/mdcompress"
INSTALL_DIR="${MDCOMPRESS_INSTALL_DIR:-/usr/local/bin}"
BIN_NAME="mdcompress"

err() {
    printf 'install.sh: %s\n' "$1" >&2
    exit 1
}

require() {
    command -v "$1" >/dev/null 2>&1 || err "missing required tool: $1"
}

require uname
require tar
require mkdir
require mv
require rm

if command -v curl >/dev/null 2>&1; then
    DL="curl -fsSL"
    DL_OUT="-o"
elif command -v wget >/dev/null 2>&1; then
    DL="wget -q"
    DL_OUT="-O"
else
    err "need curl or wget"
fi

OS_RAW="$(uname -s)"
case "$OS_RAW" in
    Linux) OS=linux ;;
    Darwin) OS=darwin ;;
    MINGW*|MSYS*|CYGWIN*) OS=windows ;;
    *) err "unsupported OS: $OS_RAW" ;;
esac

ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
    x86_64|amd64) ARCH=amd64 ;;
    arm64|aarch64) ARCH=arm64 ;;
    *) err "unsupported architecture: $ARCH_RAW" ;;
esac

VERSION="${MDCOMPRESS_VERSION:-}"
if [ -z "$VERSION" ]; then
    API_URL="https://api.github.com/repos/${REPO}/releases/latest"
    if [ -n "${GITHUB_TOKEN:-}" ]; then
        AUTH_HDR="Authorization: Bearer ${GITHUB_TOKEN}"
    else
        AUTH_HDR=""
    fi
    if [ -n "$AUTH_HDR" ]; then
        RAW="$($DL -H "$AUTH_HDR" "$API_URL")"
    else
        RAW="$($DL "$API_URL")"
    fi
    VERSION="$(printf '%s\n' "$RAW" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)"
    [ -n "$VERSION" ] || err "could not detect latest release"
fi

# Strip leading v from version tag for the asset name.
VER_NUM="${VERSION#v}"

if [ "$OS" = "windows" ]; then
    ASSET="${BIN_NAME}_${VER_NUM}_${OS}_${ARCH}.zip"
else
    ASSET="${BIN_NAME}_${VER_NUM}_${OS}_${ARCH}.tar.gz"
fi

URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"

TMP="$(mktemp -d 2>/dev/null || mktemp -d -t mdcompress)"
trap 'rm -rf "$TMP"' EXIT

printf 'Downloading %s\n' "$URL" >&2
$DL "$URL" $DL_OUT "$TMP/$ASSET" || err "download failed: $URL"

cd "$TMP"
case "$ASSET" in
    *.tar.gz) tar -xzf "$ASSET" ;;
    *.zip) require unzip && unzip -q "$ASSET" ;;
esac

[ -f "$BIN_NAME" ] || [ -f "${BIN_NAME}.exe" ] || err "binary not found in archive"
EXTRACTED="$BIN_NAME"
[ -f "${BIN_NAME}.exe" ] && EXTRACTED="${BIN_NAME}.exe"

chmod +x "$EXTRACTED"

DEST="${INSTALL_DIR}/${EXTRACTED}"
if [ -w "$INSTALL_DIR" ] || mkdir -p "$INSTALL_DIR" 2>/dev/null; then
    mv "$EXTRACTED" "$DEST"
else
    require sudo
    sudo mkdir -p "$INSTALL_DIR"
    sudo mv "$EXTRACTED" "$DEST"
fi

printf 'Installed %s %s to %s\n' "$BIN_NAME" "$VERSION" "$DEST" >&2
