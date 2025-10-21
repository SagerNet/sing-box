#!/usr/bin/env bash

set -e -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/common.sh"

setup_environment

echo "Updating sing-box from git repository..."
cd "$PROJECT_DIR"
git fetch
git reset FETCH_HEAD --hard
git clean -fdx

BUILD_TAGS=$(get_build_tags "debug")

build_sing_box "$BUILD_TAGS"

stop_service
install_binary
start_service

echo ""
echo "Following service logs (Ctrl+C to exit)..."
sudo journalctl -u sing-box --output cat -f
