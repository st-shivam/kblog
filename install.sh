#!/usr/bin/env bash
# kblog installer
# Usage: curl -fsSL https://raw.githubusercontent.com/st-tripathi/kblog/main/install.sh | bash
set -euo pipefail

REPO="st-tripathi/kblog"
BINARY="kblog"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# ── detect OS and arch ────────────────────────────────────────────────────────
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac
case "$OS" in
  linux|darwin) ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# ── resolve latest version ────────────────────────────────────────────────────
if [ -z "${VERSION:-}" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
fi
[ -z "$VERSION" ] && { echo "Could not determine latest version"; exit 1; }

# ── download and install binary ───────────────────────────────────────────────
ARCHIVE="kblog_${VERSION#v}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/$VERSION/$ARCHIVE"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "Installing kblog $VERSION ($OS/$ARCH)..."
curl -fsSL "$URL" -o "$TMP/$ARCHIVE"
tar -xzf "$TMP/$ARCHIVE" -C "$TMP"

if [ -w "$INSTALL_DIR" ]; then
  cp "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
else
  sudo cp "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
fi
chmod +x "$INSTALL_DIR/$BINARY"
echo "  Binary installed to $INSTALL_DIR/$BINARY"

# ── install k9s plugin ────────────────────────────────────────────────────────
install_k9s_plugin() {
  local plugin_src="$TMP/plugin/plugins.yaml"
  [ -f "$plugin_src" ] || return 0

  local k9s_dir=""
  if [ "$OS" = "darwin" ] && [ -d "$HOME/Library/Application Support/k9s" ]; then
    k9s_dir="$HOME/Library/Application Support/k9s"
  elif [ -d "${XDG_CONFIG_HOME:-$HOME/.config}/k9s" ]; then
    k9s_dir="${XDG_CONFIG_HOME:-$HOME/.config}/k9s"
  elif command -v k9s &>/dev/null; then
    # k9s is installed but config dir doesn't exist yet — create it
    k9s_dir="${XDG_CONFIG_HOME:-$HOME/.config}/k9s"
    mkdir -p "$k9s_dir"
  fi

  [ -z "$k9s_dir" ] && return 0

  local plugin_dst="$k9s_dir/plugins.yaml"
  if [ -f "$plugin_dst" ]; then
    if grep -q "kblog-pod" "$plugin_dst"; then
      echo "  k9s plugin already present — skipping"
    else
      grep -v '^plugins:' "$plugin_src" >> "$plugin_dst"
      echo "  k9s plugin merged into $plugin_dst"
    fi
  else
    cp "$plugin_src" "$plugin_dst"
    echo "  k9s plugin installed to $plugin_dst"
  fi
}

install_k9s_plugin

echo ""
echo "kblog $VERSION installed successfully."
echo "Run 'kblog --help' to get started."
[ -n "$(command -v k9s 2>/dev/null)" ] && echo "In k9s: highlight any pod or deployment and press Shift-L."
