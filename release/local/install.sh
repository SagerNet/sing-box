#!/usr/bin/env bash

set -e -o pipefail

DIR=$(dirname "$0")
PROJECT=$DIR/../..

pushd $PROJECT
go install -v -trimpath -ldflags "-s -w -buildid=" -tags "no_gvisor" ./cmd/sing-box
popd

sudo cp $(go env GOPATH)/bin/sing-box /usr/local/bin/
sudo mkdir -p /usr/local/etc/sing-box
sudo cp $DIR/config.json /usr/local/etc/sing-box/config.json
sudo cp $DIR/sing-box.service /etc/systemd/system
sudo systemctl daemon-reload
