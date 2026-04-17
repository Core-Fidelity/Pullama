#!/bin/sh
set -e

LDFLAGS="-buildid="

build() {
    GOOS=$1 GOARCH=$2 go build -trimpath -ldflags="$LDFLAGS" -o "$3" .
}

build darwin  arm64        pullama-darwin-arm64
build linux   amd64        pullama-linux-amd64
build windows amd64        pullama-windows-amd64.exe

install() {
    go build -trimpath -ldflags="$LDFLAGS" -o pullama .
    cp pullama /usr/local/bin/pullama
    echo "Installed to /usr/local/bin/pullama"
}

case "${1:-all}" in
    install) install ;;
    all)     build darwin arm64 pullama-darwin-arm64; build linux amd64 pullama-linux-amd64; build windows amd64 pullama-windows-amd64.exe ;;
    *)       echo "Usage: $0 [all|install]" >&2; exit 1 ;;
esac