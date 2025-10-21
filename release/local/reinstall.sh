#!/usr/bin/env bash

set -e -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/common.sh"

setup_environment

BUILD_TAGS=$(get_build_tags)

build_sing_box "$BUILD_TAGS"

stop_service
install_binary
start_service

echo ""
echo "Reinstallation complete!"
