#!/bin/sh
# Doss installer.
#   curl -fsSL https://raw.githubusercontent.com/Kordi-AI/doss/main/install.sh | sh
# Downloads a prebuilt binary for your OS/arch; falls back to building from
# source (needs Go) when no matching release asset is available.
set -e

REPO="Kordi-AI/doss"
BIN_DIR="${DOSS_BIN_DIR:-$HOME/.local/bin}"

need() { command -v "$1" >/dev/null 2>&1; }

detect() {
  os=$(uname -s); arch=$(uname -m)
  case "$os" in
    Darwin) os=darwin ;;
    Linux)  os=linux ;;
    *) echo "unsupported OS: $os (build from source: https://github.com/$REPO)" >&2; exit 1 ;;
  esac
  case "$arch" in
    x86_64|amd64) arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *) echo "unsupported arch: $arch" >&2; exit 1 ;;
  esac
  echo "${os}_${arch}"
}

build_from_source() {
  need git || { echo "no prebuilt binary and git is missing." >&2; exit 1; }
  need go  || { echo "no prebuilt binary for your platform, and Go isn't installed to build from source." >&2
                echo "  macOS: brew install go   linux: https://go.dev/dl/" >&2; exit 1; }
  tmp=$(mktemp -d)
  echo "building doss from source..."
  if [ -f "./cmd/doss/main.go" ]; then
    go build -o "$tmp/doss" ./cmd/doss
  else
    git clone --depth 1 "https://github.com/$REPO" "$tmp/src" >/dev/null 2>&1
    ( cd "$tmp/src" && go build -o "$tmp/doss" ./cmd/doss )
  fi
  mkdir -p "$BIN_DIR"; mv "$tmp/doss" "$BIN_DIR/doss"; rm -rf "$tmp"
}

verify_checksum() {
  plat="$1"
  bin="$2"
  dir="$3"
  sums="$dir/checksums.txt"
  url="https://github.com/$REPO/releases/latest/download/checksums.txt"
  if need curl; then
    curl -fsSL "$url" -o "$sums" 2>/dev/null || return 1
  elif need wget; then
    wget -qO "$sums" "$url" 2>/dev/null || return 1
  else
    return 1
  fi
  expected=$(awk -v f="doss_${plat}" '$2 == f {print $1}' "$sums")
  [ -n "$expected" ] || return 1
  if need sha256sum; then
    actual=$(sha256sum "$bin" | awk '{print $1}')
  elif need shasum; then
    actual=$(shasum -a 256 "$bin" | awk '{print $1}')
  else
    return 1
  fi
  if [ "$actual" != "$expected" ]; then
    echo "checksum mismatch for doss_${plat}" >&2
    echo "  expected: $expected" >&2
    echo "  actual:   $actual" >&2
    exit 1
  fi
}

install_prebuilt() {
  plat=$(detect)
  url="https://github.com/$REPO/releases/latest/download/doss_${plat}"
  tmp=$(mktemp -d)
  echo "downloading doss for ${plat}..."
  if need curl; then
    curl -fsSL "$url" -o "$tmp/doss" 2>/dev/null || return 1
  elif need wget; then
    wget -qO "$tmp/doss" "$url" 2>/dev/null || return 1
  else
    return 1
  fi
  [ -s "$tmp/doss" ] || return 1
  verify_checksum "$plat" "$tmp/doss" "$tmp" || return 1
  chmod +x "$tmp/doss"
  mkdir -p "$BIN_DIR"; mv "$tmp/doss" "$BIN_DIR/doss"; rm -rf "$tmp"
}

if ! install_prebuilt; then
  echo "no prebuilt binary available — falling back to source build"
  build_from_source
fi

echo "✓ installed: $BIN_DIR/doss"
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "  note: add $BIN_DIR to your PATH" ;;
esac
echo "next: doss init          (guided setup)"
echo "      doss init --github  (with a private GitHub repo as the cloud copy)"
