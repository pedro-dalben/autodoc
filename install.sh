#!/bin/sh
set -eu
REPO="${AUTODOC_REPO:-pedro-dalben/autodoc}"
VERSION="${AUTODOC_VERSION:-latest}"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
esac
echo "==> autodoc install ($VERSION, $OS/$ARCH)"
echo "Releases are published via GoReleaser; source install always works:"
echo "  go install github.com/$REPO/cmd/autodoc@$VERSION"
if command -v go >/dev/null 2>&1; then
  GOBIN="${GOBIN:-$HOME/go/bin}"
  mkdir -p "$GOBIN"
  go install "github.com/$REPO/cmd/autodoc@$VERSION"
  echo "installed to $GOBIN/autodoc"
  echo "next: autodoc init && autodoc doctor"
else
  echo "Go not found; install Go 1.24+ first: https://go.dev/dl/" >&2
  exit 1
fi
