#!/usr/bin/env bash

set -euo pipefail

VERSION="1.27.4"
PATCH_COMMITS=(
  "5bea096a8d38bcc2f7b29a423a2d7a677e7257b4"
  "2fd9d96c8897586872cf00e335dad7d0e2d32601"
  "51cbeef333241001a1106fbeeb7c7ef80054dac2"
  "4fd81ff5c9844620e836e8d2052e082ac26484f0"
  "2924464d68627505f876d4d275bf8432dbf81da5"
  "5c36850df445d11d557c3d995e15a3ac159749ea"
  "51c81072acbed32e4331026bc6ca308a3a495039"
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
wget "https://dl.google.com/go/go${VERSION}.linux-amd64.tar.gz"
tar -xzf "go${VERSION}.linux-amd64.tar.gz"
mv go go_win7
cd go_win7

# modify from https://github.com/restic/restic/issues/4636#issuecomment-1896455557
# these patch URLs only work on golang1.27.x
# that means after golang1.28 release it must be changed
# see: https://github.com/MetaCubeX/go/commits/release-branch.go1.27/
# revert:
# 341b5e2c0261cc059b157f1c7a2a2c4d1f417f0d: "cmd/link: raise minimum windows version to 10"
# 693def151adff1af707d82d28f55dba81ceb08e1: "crypto/rand,runtime: switch RtlGenRandom for ProcessPrng"
# 7c1157f9544922e96945196b47b95664b1e39108: "net: remove sysSocket fallback for Windows 7"
# 48042aa09c2f878c4faa576948b07fe625c4707a: "syscall: remove Windows 7 console handle workaround"
# a17d959debdb04cd550016a3501dd09d50cd62e7: "runtime: always use LoadLibraryEx to load system libraries"
# fixes:
# 5c36850df445d11d557c3d995e15a3ac159749ea: "Fix os.RemoveAll not working on Windows7"
# 51c81072acbed32e4331026bc6ca308a3a495039: "Revert \"os: remove 5ms sleep on Windows in (*Process).Wait\""

for patch_commit in "${PATCH_COMMITS[@]}"; do
  curl "${CURL_ARGS[@]}" "https://github.com/MetaCubeX/go/commit/${patch_commit}.diff" | patch --verbose -p 1
done

GOTOOLCHAIN=local ./bin/go install cmd/link
