#!/bin/sh
set -e

LDFLAGS="-buildid="

build() {
    GOOS=$1 GOARCH=$2 go build -trimpath -ldflags="$LDFLAGS" -o "$3" .
}

build darwin  arm64        pullm-darwin-arm64
build linux   amd64        pullm-linux-amd64
build windows amd64        pullm-windows-amd64.exe