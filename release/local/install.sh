#!/usr/bin/env bash

set -e -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/common.sh"

setup_environment

BUILD_TAGS=$(get_build_tags)

build_sing_box "$BUILD_TAGS"
install_binary
setup_config
setup_systemd

echo ""
echo "Installation complete!"
echo "To enable and start the service, run: $SCRIPT_DIR/enable.sh"
