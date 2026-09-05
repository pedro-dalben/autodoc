#!/bin/sh
# AutoDoc installer: downloads a versioned binary from GitHub Releases,
# verifies its SHA-256 checksum, and installs it. No Go toolchain needed.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/pedro-dalben/autodoc/main/install.sh | sh
#   curl -fsSL ... | sh -s -- --version v0.1.0
#   AUTODOC_VERSION=v0.1.0 sh install.sh
#   AUTODOC_INSTALL_DIR=$HOME/bin sh install.sh
#
# Env:
#   AUTODOC_VERSION      release tag, e.g. v0.1.0 (default: latest)
#   AUTODOC_INSTALL_DIR  install directory (default: $HOME/.local/bin)
#   AUTODOC_REPO         owner/repo (default: pedro-dalben/autodoc)
#   AUTODOC_RELEASE_BASE override for tests (default: https://github.com)
set -eu

REPO="${AUTODOC_REPO:-pedro-dalben/autodoc}"
VERSION="${AUTODOC_VERSION:-latest}"
INSTALL_DIR="${AUTODOC_INSTALL_DIR:-$HOME/.local/bin}"
RELEASE_BASE="${AUTODOC_RELEASE_BASE:-https://github.com}"
usage() {
  echo "usage: install.sh [--version vX.Y.Z] [--dir PATH]" >&2
}

while [ $# -gt 0 ]; do
  case "$1" in
    --version) VERSION="${2:?missing value for --version}"; shift 2 ;;
    --dir) INSTALL_DIR="${2:?missing value for --dir}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage; exit 1 ;;
  esac
done

fail() { echo "error: $1" >&2; exit 1; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "$1 not found (needed to download and verify the release)"
}

need_cmd uname
need_cmd mkdir
need_cmd mktemp
if command -v curl >/dev/null 2>&1; then
  download() { curl -fsSL -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
  download() { wget -q -O "$2" "$1"; }
else
  fail "need curl or wget to download the release"
fi

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  linux|darwin) ;;
  mingw*|msys*|cygwin*) OS="windows" ;;
  *) fail "unsupported OS: $(uname -s) (supported: Linux, macOS, Windows via install.ps1)" ;;
esac

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) fail "unsupported architecture: $(uname -m) (supported: x86_64/amd64, arm64)" ;;
esac

if [ "$OS" = "windows" ]; then
  fail "on Windows use install.ps1 instead (see docs/install.md)"
fi

if [ "$VERSION" = "latest" ]; then
  # GitHub's /releases/latest endpoint only serves stable releases, never
  # pre-releases. While AutoDoc has no stable release yet, install the
  # release candidate explicitly instead of relying on "latest".
  NO_STABLE="no stable AutoDoc release is available yet (only pre-releases). Install the release candidate explicitly: AUTODOC_VERSION=<tag> sh install.sh (see https://github.com/$REPO/releases and docs/install.md)"
  command -v grep >/dev/null 2>&1 || fail "grep not found (needed to resolve the latest version)"
  if command -v curl >/dev/null 2>&1; then
    TAG_JSON="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest")" || \
      fail "$NO_STABLE"
  else
    TAG_JSON="$(wget -q -O - "https://api.github.com/repos/$REPO/releases/latest")" || \
      fail "$NO_STABLE"
  fi
  VERSION="$(printf '%s' "$TAG_JSON" | grep '"tag_name":' | head -1 | cut -d'"' -f4)"
  [ -n "$VERSION" ] || fail "could not parse latest release tag ($NO_STABLE)"
fi
case "$VERSION" in
  v*) ;;
  *) VERSION="v$VERSION" ;;
esac

ASSET_VER="${VERSION#v}"
ASSET="autodoc_${ASSET_VER}_${OS}_${ARCH}.tar.gz"
BASE="$RELEASE_BASE/$REPO/releases/download/$VERSION"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT INT TERM

echo "==> autodoc $VERSION ($OS/$ARCH)"
download "$BASE/$ASSET" "$TMP/autodoc.tar.gz" || \
  fail "download failed: $BASE/$ASSET not found (bad version or unsupported platform?)"
download "$BASE/checksums.txt" "$TMP/checksums.txt" || \
  fail "download failed: checksums.txt missing for $VERSION"

WANT="$(grep " $ASSET\$" "$TMP/checksums.txt" | cut -d' ' -f1)"
[ -n "$WANT" ] || fail "checksum for $ASSET missing in checksums.txt (aborting)"

if command -v sha256sum >/dev/null 2>&1; then
  GOT="$(sha256sum "$TMP/autodoc.tar.gz" | cut -d' ' -f1)"
elif command -v shasum >/dev/null 2>&1; then
  GOT="$(shasum -a 256 "$TMP/autodoc.tar.gz" | cut -d' ' -f1)"
else
  fail "need sha256sum or shasum to verify the download"
fi
[ "$GOT" = "$WANT" ] || fail "checksum mismatch for $ASSET (expected $WANT, got $GOT; aborting)"

tar -xzf "$TMP/autodoc.tar.gz" -C "$TMP" || fail "could not extract $ASSET"
BIN="$TMP/autodoc"
[ -x "$BIN" ] || BIN="$(find "$TMP" -name autodoc -type f | head -1)"
[ -n "$BIN" ] && [ -f "$BIN" ] || fail "autodoc binary not found inside $ASSET"

mkdir -p "$INSTALL_DIR"
cp "$BIN" "$INSTALL_DIR/autodoc"
chmod +x "$INSTALL_DIR/autodoc"
"$INSTALL_DIR/autodoc" version >/dev/null 2>&1 || \
  fail "installed binary failed to run ($INSTALL_DIR/autodoc version)"

echo "installed $INSTALL_DIR/autodoc"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "note: $INSTALL_DIR is not on PATH. Add this line to your shell rc:"; echo "  export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
esac
echo "next: autodoc init && autodoc doctor"
