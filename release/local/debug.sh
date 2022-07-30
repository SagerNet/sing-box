#!/usr/bin/env bash

set -e -o pipefail

DIR=$(dirname "$0")
PROJECT=$DIR/../..

pushd $PROJECT
git fetch
git reset FETCH_HEAD --hard
git clean -fdx
go install -v -trimpath -ldflags "-s -w -buildid=" -tags "no_gvisor,debug" ./cmd/sing-box
popd

sudo systemctl stop sing-box
sudo cp $(go env GOPATH)/bin/sing-box /usr/local/bin/
sudo systemctl start sing-box
sudo journalctl -u sing-box --output cat -f
