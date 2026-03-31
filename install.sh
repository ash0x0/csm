#!/bin/sh
# csm installer — downloads the latest release binary for your platform.
# Usage: curl -fsSL https://raw.githubusercontent.com/ash0x0/csm/main/install.sh | sh

set -e

REPO="ash0x0/csm"
INSTALL_DIR="${CSM_INSTALL_DIR:-/usr/local/bin}"

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

main() {
  platform=$(detect_platform)
  version=$(get_latest_version)

  if [ -z "$version" ]; then
    echo "Error: could not determine latest version" >&2
    exit 1
  fi

  url="https://github.com/${REPO}/releases/download/v${version}/csm_${version}_${platform}.tar.gz"
  echo "Downloading csm v${version} for ${platform}..."

  tmpdir=$(mktemp -d)
  trap 'rm -rf "$tmpdir"' EXIT

  curl -fsSL "$url" -o "${tmpdir}/csm.tar.gz"
  tar -xzf "${tmpdir}/csm.tar.gz" -C "$tmpdir"

  if [ -w "$INSTALL_DIR" ]; then
    mv "${tmpdir}/csm" "${INSTALL_DIR}/csm"
  else
    echo "Installing to ${INSTALL_DIR} (requires sudo)..."
    sudo mv "${tmpdir}/csm" "${INSTALL_DIR}/csm"
  fi

  chmod +x "${INSTALL_DIR}/csm"
  echo "Installed csm v${version} to ${INSTALL_DIR}/csm"
}

main
