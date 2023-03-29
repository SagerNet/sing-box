#!/usr/bin/env bash

set -e -o pipefail

go_version=$(curl -s https://raw.githubusercontent.com/actions/go-versions/main/versions-manifest.json | grep -oE '"version": "[0-9]{1}.[0-9]{1,}(.[0-9]{1,})?"' | head -1 | cut -d':' -f2 | sed 's/ //g; s/"//g')
curl -Lo go.tar.gz "https://go.dev/dl/go$go_version.linux-amd64.tar.gz"
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go.tar.gz
rm go.tar.gz
