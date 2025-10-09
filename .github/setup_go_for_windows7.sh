#!/usr/bin/env bash

VERSION="1.25.3"

mkdir -p $HOME/go
cd $HOME/go
wget "https://dl.google.com/go/go${VERSION}.linux-amd64.tar.gz"
tar -xzf "go${VERSION}.linux-amd64.tar.gz"
mv go go_win7
cd go_win7

# modify from https://github.com/restic/restic/issues/4636#issuecomment-1896455557
# this patch file only works on golang1.25.x
# that means after golang1.26 release it must be changed
# see: https://github.com/MetaCubeX/go/commits/release-branch.go1.25/
# revert:
# 693def151adff1af707d82d28f55dba81ceb08e1: "crypto/rand,runtime: switch RtlGenRandom for ProcessPrng"
# 7c1157f9544922e96945196b47b95664b1e39108: "net: remove sysSocket fallback for Windows 7"
# 48042aa09c2f878c4faa576948b07fe625c4707a: "syscall: remove Windows 7 console handle workaround"
# a17d959debdb04cd550016a3501dd09d50cd62e7: "runtime: always use LoadLibraryEx to load system libraries"

alias curl='curl -H "Authorization: Bearer ${{ secrets.GITHUB_TOKEN }}"'

curl https://github.com/MetaCubeX/go/commit/8cb5472d94c34b88733a81091bd328e70ee565a4.diff | patch --verbose -p 1
curl https://github.com/MetaCubeX/go/commit/6788c4c6f9fafb56729bad6b660f7ee2272d699f.diff | patch --verbose -p 1
curl https://github.com/MetaCubeX/go/commit/a5b2168bb836ed9d6601c626f95e56c07923f906.diff | patch --verbose -p 1
curl https://github.com/MetaCubeX/go/commit/f56f1e23507e646c85243a71bde7b9629b2f970c.diff | patch --verbose -p 1
