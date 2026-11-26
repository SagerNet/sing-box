#!/usr/bin/env bash

set -euo pipefail

VERSION="1.27.4"
PATCH_COMMITS=(
  "042f409b154a743972239baefcfc400d5ff33de5"
  "d6ea9ffecf266da2d2793b635a07e7db60a3adaf"
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

# these patch URLs only work on golang1.27.x
# that means after golang1.28 release it must be changed
# see: https://github.com/SagerNet/go/commits/release-branch.go1.27/
# revert:
# 937368f84e: "crypto/x509: change how we retrieve chains on darwin"
# fixes:
# d6ea9ffecf: "cmd/link: lower default macOS version to 10.13"

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
