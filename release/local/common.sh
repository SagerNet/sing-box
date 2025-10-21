#!/usr/bin/env bash

set -e -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
BINARY_NAME="sing-box"

INSTALL_BIN_PATH="/usr/local/bin"
INSTALL_CONFIG_PATH="/usr/local/etc/sing-box"
INSTALL_DATA_PATH="/var/lib/sing-box"
SYSTEMD_SERVICE_PATH="/etc/systemd/system"

DEFAULT_BUILD_TAGS="with_gvisor,with_quic,with_dhcp,with_wireguard,with_utls,with_acme,with_clash_api,with_tailscale,with_ccm,badlinkname,tfogo_checklinkname0"

setup_environment() {
    if [ -d /usr/local/go ]; then
        export PATH="$PATH:/usr/local/go/bin"
    fi

    if ! command -v go &> /dev/null; then
        echo "Error: Go is not installed or not in PATH"
        echo "Run install_go.sh to install Go"
        exit 1
    fi
}

get_build_tags() {
    local extra_tags="$1"
    if [ -n "$extra_tags" ]; then
        echo "${DEFAULT_BUILD_TAGS},${extra_tags}"
    else
        echo "${DEFAULT_BUILD_TAGS}"
    fi
}

get_version() {
    cd "$PROJECT_DIR"
    GOHOSTOS=$(go env GOHOSTOS)
    GOHOSTARCH=$(go env GOHOSTARCH)
    CGO_ENABLED=0 GOOS=$GOHOSTOS GOARCH=$GOHOSTARCH go run github.com/sagernet/sing-box/cmd/internal/read_tag@latest
}

get_ldflags() {
    local version
    version=$(get_version)
    echo "-X 'github.com/sagernet/sing-box/constant.Version=${version}' -s -w -buildid= -checklinkname=0"
}

build_sing_box() {
    local tags="$1"
    local ldflags
    ldflags=$(get_ldflags)

    echo "Building sing-box with tags: $tags"
    cd "$PROJECT_DIR"
    export GOTOOLCHAIN=local
    go install -v -trimpath -ldflags "$ldflags" -tags "$tags" ./cmd/sing-box
}

install_binary() {
    local gopath
    gopath=$(go env GOPATH)
    echo "Installing binary to $INSTALL_BIN_PATH/$BINARY_NAME"
    sudo cp "${gopath}/bin/${BINARY_NAME}" "${INSTALL_BIN_PATH}/"
}

setup_config() {
    echo "Setting up configuration"
    sudo mkdir -p "$INSTALL_CONFIG_PATH"
    if [ ! -f "$INSTALL_CONFIG_PATH/config.json" ]; then
        sudo cp "$PROJECT_DIR/release/config/config.json" "$INSTALL_CONFIG_PATH/config.json"
        echo "Default config installed to $INSTALL_CONFIG_PATH/config.json"
    else
        echo "Config already exists at $INSTALL_CONFIG_PATH/config.json (not overwriting)"
    fi
}

setup_systemd() {
    echo "Setting up systemd service"
    sudo cp "$SCRIPT_DIR/sing-box.service" "$SYSTEMD_SERVICE_PATH/"
    sudo systemctl daemon-reload
}

stop_service() {
    if systemctl is-active --quiet sing-box; then
        echo "Stopping sing-box service"
        sudo systemctl stop sing-box
    fi
}

start_service() {
    echo "Starting sing-box service"
    sudo systemctl start sing-box
}

restart_service() {
    echo "Restarting sing-box service"
    sudo systemctl restart sing-box
}
