#!/usr/bin/env bash

set -euo pipefail

VERSION="1.26.5"
PATCH_COMMITS=(
  "a4ae550aa148b04c9d4890e98bee63aede5c4b53"
  "95b851f661584711faa8115b3234a461044f4510"
  "b0bf0a863cf218b9bb0d6c013903ab160ae0c39b"
  "7c7a2a0d68920f8764d9a3aeae7ad1067a17b3fa"
  "ba34356e82f3c12a7303d2231e6ea0e42bbbb3bf"
  "472a88edbc2a42feb53fa31d29ab54d33840dca6"
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
# these patch URLs only work on golang1.26.x
# that means after golang1.27 release it must be changed
# see: https://github.com/MetaCubeX/go/commits/release-branch.go1.26/
# revert:
# 693def151adff1af707d82d28f55dba81ceb08e1: "crypto/rand,runtime: switch RtlGenRandom for ProcessPrng"
# 7c1157f9544922e96945196b47b95664b1e39108: "net: remove sysSocket fallback for Windows 7"
# 48042aa09c2f878c4faa576948b07fe625c4707a: "syscall: remove Windows 7 console handle workaround"
# a17d959debdb04cd550016a3501dd09d50cd62e7: "runtime: always use LoadLibraryEx to load system libraries"
# fixes:
# bed309eff415bcb3c77dd4bc3277b682b89a388d: "Fix os.RemoveAll not working on Windows7"
# 34b899c2fb39b092db4fa67c4417e41dc046be4b: "Revert \"os: remove 5ms sleep on Windows in (*Process).Wait\""

for patch_commit in "${PATCH_COMMITS[@]}"; do
  curl "${CURL_ARGS[@]}" "https://github.com/MetaCubeX/go/commit/${patch_commit}.diff" | patch --verbose -p 1
done
