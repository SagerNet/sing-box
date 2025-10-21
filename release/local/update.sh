#!/usr/bin/env bash

set -e -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/common.sh"

echo "Updating sing-box from git repository..."
cd "$PROJECT_DIR"
git fetch
git reset FETCH_HEAD --hard
git clean -fdx

echo ""
echo "Running reinstall..."
exec "$SCRIPT_DIR/reinstall.sh"