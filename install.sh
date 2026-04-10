#!/bin/sh
# csm installer — install, update, or uninstall csm.
#
# Install/update:  curl -fsSL https://raw.githubusercontent.com/ash0x0/csm/main/install.sh | sh
# Uninstall:       curl -fsSL https://raw.githubusercontent.com/ash0x0/csm/main/install.sh | sh -s -- uninstall
# Or locally:      ./install.sh [install|update|uninstall]

set -e

REPO="ash0x0/csm"
INSTALL_DIR="${CSM_INSTALL_DIR:-/usr/local/bin}"
BINARY="${INSTALL_DIR}/csm"

detect_platform() {
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  arch=$(uname -m)

  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) echo "Unsupported architecture: $arch" >&2; exit 1 ;;
  esac

  case "$os" in
    linux|darwin) ;;
    *) echo "Unsupported OS: $os" >&2; exit 1 ;;
  esac

  echo "${os}_${arch}"
}

get_latest_version() {
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
    grep '"tag_name"' | head -1 | sed 's/.*"v\([^"]*\)".*/\1/'
}

get_installed_version() {
  if [ -x "$BINARY" ]; then
    "$BINARY" version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo ""
  fi
}

do_install() {
  platform=$(detect_platform)
  latest=$(get_latest_version)

  if [ -z "$latest" ]; then
    echo "Error: could not determine latest version" >&2
    exit 1
  fi

  installed=$(get_installed_version)
  if [ -n "$installed" ] && [ "$installed" = "$latest" ]; then
    echo "csm v${installed} is already up to date."
    exit 0
  fi

  if [ -n "$installed" ]; then
    echo "Updating csm v${installed} → v${latest}..."
  else
    echo "Installing csm v${latest} for ${platform}..."
  fi

  url="https://github.com/${REPO}/releases/download/v${latest}/csm_${latest}_${platform}.tar.gz"

  tmpdir=$(mktemp -d)
  trap 'rm -rf "$tmpdir"' EXIT

  curl -fsSL "$url" -o "${tmpdir}/csm.tar.gz"
  tar -xzf "${tmpdir}/csm.tar.gz" -C "$tmpdir"

  if [ -w "$INSTALL_DIR" ]; then
    mv "${tmpdir}/csm" "${BINARY}"
  else
    echo "Writing to ${INSTALL_DIR} requires sudo..."
    sudo mv "${tmpdir}/csm" "${BINARY}"
  fi

  chmod +x "${BINARY}"

  if [ -n "$installed" ]; then
    echo "Updated to csm v${latest} at ${BINARY}"
  else
    echo "Installed csm v${latest} at ${BINARY}"
  fi
}

do_uninstall() {
  if [ ! -e "$BINARY" ]; then
    echo "csm is not installed at ${BINARY}"
    exit 0
  fi

  if [ -w "$BINARY" ]; then
    rm -f "${BINARY}"
  else
    echo "Removing ${BINARY} requires sudo..."
    sudo rm -f "${BINARY}"
  fi

  echo "Uninstalled csm from ${BINARY}"
}

cmd="${1:-install}"

case "$cmd" in
  install|update) do_install ;;
  uninstall)      do_uninstall ;;
  *)
    echo "Usage: $0 [install|update|uninstall]" >&2
    exit 1
    ;;
esac
