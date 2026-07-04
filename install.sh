#!/bin/sh
# Dossier installer — builds from source (binary releases come later).
set -e

if ! command -v git >/dev/null 2>&1; then
  echo "dossier needs git. install git first." >&2; exit 1
fi
if ! command -v go >/dev/null 2>&1; then
  echo "dossier builds with Go (>= 1.22)." >&2
  echo "  macOS:  brew install go" >&2
  echo "  linux:  https://go.dev/dl/" >&2
  exit 1
fi

cd "$(dirname "$0")"
echo "building dossier..."
go build -o dossier ./cmd/dossier

BIN_DIR="${DOSSIER_BIN_DIR:-$HOME/.local/bin}"
mkdir -p "$BIN_DIR"
mv dossier "$BIN_DIR/dossier"

echo "✓ installed: $BIN_DIR/dossier"
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "  note: add $BIN_DIR to your PATH" ;;
esac
echo "next: dossier init          (local only)"
echo "      dossier init --github (with a private GitHub repo as the cloud copy)"
