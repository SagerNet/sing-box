#!/usr/bin/env bash

sudo systemctl stop sing-box
sudo rm -rf /var/lib/sing-box
sudo rm -rf /usr/local/bin/sing-box
sudo rm -rf /usr/local/etc/sing-box
sudo rm -rf /etc/systemd/system/sing-box.service
sudo systemctl daemon-reload
