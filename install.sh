#!/usr/bin/env bash
# aster install.sh -- one-curl install from the wildreason dl rail.
#
# Usage:
#   curl -fsSL https://artifacts.wildreason.ai/dl/aster/install.sh | bash
#
# Env overrides:
#   ASTER_ARCH         override auto-detected arch (e.g. darwin-arm64)
#   ASTER_DL_BASE      override download base (default artifacts.wildreason.ai/dl/aster)
#   ASTER_INSTALL_DIR  override install directory (default ~/.local/bin)
#
# The shape is openlap's install.sh, trimmed: fetch binary + sibling
# .sha256, verify, install, PATH hint. Refuses on checksum mismatch.

set -euo pipefail

DL_BASE="${ASTER_DL_BASE:-https://artifacts.wildreason.ai/dl/aster}"
INSTALL_DIR="${ASTER_INSTALL_DIR:-$HOME/.local/bin}"

# ---------- arch detect ----------
if [ -n "${ASTER_ARCH:-}" ]; then
  ARCH="$ASTER_ARCH"
else
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  m="$(uname -m)"
  case "$m" in
    arm64|aarch64) cpu="arm64" ;;
    x86_64|amd64)  cpu="amd64" ;;
    *) echo "aster: unsupported arch '$m' (set ASTER_ARCH to override)" >&2; exit 1 ;;
  esac
  case "$os" in
    darwin|linux) ;;
    *) echo "aster: unsupported OS '$os'" >&2; exit 1 ;;
  esac
  ARCH="$os-$cpu"
fi
echo "aster: installing for $ARCH"

# ---------- download + verify ----------
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

curl -fsSL "$DL_BASE/$ARCH/aster" -o "$TMP/aster"
curl -fsSL "$DL_BASE/$ARCH/aster.sha256" -o "$TMP/aster.sha256"

expected="$(awk '{print $1}' "$TMP/aster.sha256")"
if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$TMP/aster" | awk '{print $1}')"
else
  actual="$(shasum -a 256 "$TMP/aster" | awk '{print $1}')"
fi
if [ "$expected" != "$actual" ]; then
  echo "aster: sha256 mismatch (expected $expected, got $actual) -- refusing to install. Retry in a minute; a release may be mid-upload." >&2
  exit 1
fi

# ---------- install ----------
mkdir -p "$INSTALL_DIR"
install -m 0755 "$TMP/aster" "$INSTALL_DIR/aster"
echo "aster: installed to $INSTALL_DIR/aster ($("$INSTALL_DIR/aster" --version | head -1))"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "aster: NOTE -- $INSTALL_DIR is not on your PATH. Add:  export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
esac
