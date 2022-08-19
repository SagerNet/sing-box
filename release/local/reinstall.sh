#!/usr/bin/env bash

set -e -o pipefail

DIR=$(dirname "$0")
PROJECT=$DIR/../..

pushd $PROJECT
go install -v -trimpath -ldflags "-s -w -buildid=" -tags no_gvisor,with_quic,with_acme ./cmd/sing-box
popd

sudo systemctl stop sing-box
sudo cp $(go env GOPATH)/bin/sing-box /usr/local/bin/
sudo systemctl start sing-box
sudo journalctl -u sing-box --output cat -f
