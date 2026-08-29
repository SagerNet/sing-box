#!/usr/bin/env bash

set -euo pipefail

VERSION="1.26.7"
PATCH_COMMITS=(
  "f080b0c6346eb690c0dc82497b35925f385b35ac"
  "2d9c12887c342fb9051d231aa5388743cb7e9cb6"
  "367663849c656612fc3c8875ef6e7f6b0c93dbd0"
)
CURL_ARGS=(
  -fL
  --silent
  --show-error
)

if [[ -n "${GITHUB_TOKEN:-}" ]]; then
  CURL_ARGS+=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
fi

mkdir -p "$HOME/go"
cd "$HOME/go"
wget "https://dl.google.com/go/go${VERSION}.darwin-arm64.tar.gz"
tar -xzf "go${VERSION}.darwin-arm64.tar.gz"
#cp -a go go_bootstrap
mv go go_osx
cd go_osx

# these patch URLs only work on golang1.26.x
# that means after golang1.27 release it must be changed
# see: https://github.com/SagerNet/go/commits/release-branch.go1.26/
# revert:
# 33d3f603c1: "cmd/link/internal/ld: use 12.0.0 OS/SDK versions for macOS linking"
# 937368f84e: "crypto/x509: change how we retrieve chains on darwin"
# d90a57ffe8: "cmd/link/internal/ld: unify OS/SDK versions for macOS linking"

for patch_commit in "${PATCH_COMMITS[@]}"; do
  curl "${CURL_ARGS[@]}" "https://github.com/SagerNet/go/commit/${patch_commit}.diff" | patch --verbose -p 1
done

# Rebuild is not needed: we build with CGO_ENABLED=1, so Apple's external
# linker handles LC_BUILD_VERSION via MACOSX_DEPLOYMENT_TARGET, and the
# stdlib (crypto/x509) is compiled from patched src automatically.
#cd src
#GOROOT_BOOTSTRAP="$HOME/go/go_bootstrap" ./make.bash
#cd ../..
#rm -rf go_bootstrap "go${VERSION}.darwin-arm64.tar.gz"
