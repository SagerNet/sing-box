#!/usr/bin/env bash

set -e -o pipefail

if [ -d /usr/local/go ]; then
  export PATH="$PATH:/usr/local/go/bin"
fi

DIR=$(dirname "$0")
PROJECT=$DIR/../..

pushd $PROJECT
go install -v -trimpath -ldflags "-s -w -buildid=" -tags no_gvisor,with_quic,with_wireguard,with_acme ./cmd/sing-box
popd

sudo systemctl stop sing-box
sudo cp $(go env GOPATH)/bin/sing-box /usr/local/bin/
sudo systemctl start sing-box
sudo journalctl -u sing-box --output cat -f
