#!/usr/bin/env bash

set -e -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/common.sh"

echo "Uninstalling sing-box..."

if systemctl is-active --quiet sing-box 2>/dev/null; then
    echo "Stopping sing-box service..."
    sudo systemctl stop sing-box
fi

if systemctl is-enabled --quiet sing-box 2>/dev/null; then
    echo "Disabling sing-box service..."
    sudo systemctl disable sing-box
fi

echo "Removing files..."
sudo rm -rf "$INSTALL_DATA_PATH"
sudo rm -rf "$INSTALL_BIN_PATH/$BINARY_NAME"
sudo rm -rf "$INSTALL_CONFIG_PATH"
sudo rm -rf "$SYSTEMD_SERVICE_PATH/sing-box.service"

echo "Reloading systemd..."
sudo systemctl daemon-reload

echo ""
echo "Uninstallation complete!"
