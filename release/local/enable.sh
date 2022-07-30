#!/usr/bin/env bash

set -e -o pipefail

sudo systemctl enable sing-box
sudo systemctl start sing-box
sudo journalctl -u sing-box --output cat -f
