#!/usr/bin/env bash

set -e -o pipefail
curl -o go.tar.gz https://go.dev/dl/go1.19.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go.tar.gz
rm go.tar.gz