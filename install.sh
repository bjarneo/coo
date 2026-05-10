#!/bin/sh
set -e

REPO="bjarneo/coo"

# Pick install dir. Honor $INSTALL_DIR if set, otherwise prefer ~/.local/bin
# when it's already on PATH (no sudo needed) and fall back to /usr/local/bin.
if [ -z "$INSTALL_DIR" ]; then
    LOCAL_BIN="$HOME/.local/bin"
    if echo "$PATH" | tr ':' '\n' | grep -qx "$LOCAL_BIN"; then
        mkdir -p "$LOCAL_BIN"
        INSTALL_DIR="$LOCAL_BIN"
    else
        INSTALL_DIR="/usr/local/bin"
    fi
fi

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

case "$OS" in
    linux|darwin) ;;
    mingw*|msys*|cygwin*) OS="windows" ;;
    *) echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

BINARY="coo-${OS}-${ARCH}"
if [ "$OS" = "windows" ]; then
    BINARY="${BINARY}.exe"
fi

fetch() {
    if command -v curl > /dev/null; then
        curl -fSL -o "$1" "$2"
    elif command -v wget > /dev/null; then
        wget -qO "$1" "$2"
    else
        echo "Error: curl or wget required" >&2
        exit 1
    fi
}

fetch_quiet() {
    if command -v curl > /dev/null; then
        curl -fSL -o "$1" "$2" 2>/dev/null
    elif command -v wget > /dev/null; then
        wget -qO "$1" "$2" 2>/dev/null
    fi
}

# Resolve the latest release tag, so the install is pinned to a specific
# tag rather than tracking whatever GitHub's /latest/ alias returns.
TAG_JSON=$(mktemp)
if ! fetch_quiet "$TAG_JSON" "https://api.github.com/repos/${REPO}/releases/latest"; then
    echo "Error: could not query GitHub releases API" >&2
    rm -f "$TAG_JSON"
    exit 1
fi
TAG=$(grep -o '"tag_name": *"[^"]*"' "$TAG_JSON" | head -n1 | sed 's/.*"\([^"]*\)"$/\1/')
rm -f "$TAG_JSON"
if [ -z "$TAG" ]; then
    echo "Error: could not parse latest release tag" >&2
    exit 1
fi

echo "Installing coo ${TAG} (${OS}/${ARCH})"

URL="https://github.com/${REPO}/releases/download/${TAG}/${BINARY}"
TMP=$(mktemp)
fetch "$TMP" "$URL"
chmod +x "$TMP"

CHECKSUM_URL="https://github.com/${REPO}/releases/download/${TAG}/checksums.txt"
CHECKSUMS=$(mktemp)
if fetch_quiet "$CHECKSUMS" "$CHECKSUM_URL"; then
    EXPECTED=$(grep "${BINARY}$" "$CHECKSUMS" | awk '{print $1}')
    if [ -n "$EXPECTED" ]; then
        if command -v sha256sum > /dev/null; then
            ACTUAL=$(sha256sum "$TMP" | awk '{print $1}')
        elif command -v shasum > /dev/null; then
            ACTUAL=$(shasum -a 256 "$TMP" | awk '{print $1}')
        else
            ACTUAL=""
        fi
        if [ -n "$ACTUAL" ] && [ "$ACTUAL" != "$EXPECTED" ]; then
            echo "Error: checksum mismatch" >&2
            echo "  expected: $EXPECTED" >&2
            echo "  got:      $ACTUAL" >&2
            rm -f "$TMP" "$CHECKSUMS"
            exit 1
        fi
        if [ -n "$ACTUAL" ]; then
            echo "Checksum verified."
        fi
    fi
fi
rm -f "$CHECKSUMS"

DEST="${INSTALL_DIR}/coo"
if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP" "$DEST"
else
    sudo mv "$TMP" "$DEST"
fi

echo "Installed coo to $DEST"

# Friendly nudge if the install dir isn't on PATH.
if ! echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
    echo
    echo "Note: $INSTALL_DIR is not on your PATH."
    echo "  Add this to your shell rc:"
    echo "    export PATH=\"$INSTALL_DIR:\$PATH\""
fi
