#!/bin/sh
# Doss installer — builds from source (binary releases come later).
set -e

if ! command -v git >/dev/null 2>&1; then
  echo "doss needs git. install git first." >&2; exit 1
fi
if ! command -v go >/dev/null 2>&1; then
  echo "doss builds with Go (>= 1.22)." >&2
  echo "  macOS:  brew install go" >&2
  echo "  linux:  https://go.dev/dl/" >&2
  exit 1
fi

cd "$(dirname "$0")"
echo "building doss..."
go build -o doss ./cmd/doss

BIN_DIR="${DOSS_BIN_DIR:-$HOME/.local/bin}"
mkdir -p "$BIN_DIR"
mv doss "$BIN_DIR/doss"

echo "✓ installed: $BIN_DIR/doss"
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "  note: add $BIN_DIR to your PATH" ;;
esac
echo "next: doss init          (local only)"
echo "      doss init --github (with a private GitHub repo as the cloud copy)"
