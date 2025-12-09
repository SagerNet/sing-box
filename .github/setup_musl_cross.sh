#!/bin/bash
set -xeuo pipefail

TARGET="$1"

# Download musl-cross toolchain from musl.cc
cd "$HOME"
wget -q "https://musl.cc/${TARGET}-cross.tgz"
mkdir -p musl-cross
tar -xf "${TARGET}-cross.tgz" -C musl-cross --strip-components=1
rm "${TARGET}-cross.tgz"
