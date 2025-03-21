#!/usr/bin/env bash

set -e -o pipefail

release/local/install_minimal.sh
sudo cp .github/goreleaser/config.json /usr/local/etc/sing-box/config.json
sudo mkdir -p /var/lib/sing-box/.github/goreleaser
sudo cp .github/goreleaser/response.json /var/lib/sing-box/.github/goreleaser/response.json
go run -v ./cmd/sing-box tools install-ca .github/goreleaser/ca.crt
sudo systemctl start sing-box
sleep 5
