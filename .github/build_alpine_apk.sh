#!/usr/bin/env bash

set -e -o pipefail

ARCHITECTURE="$1"
VERSION="$2"
BINARY_PATH="$3"
OUTPUT_PATH="$4"

if [ -z "$ARCHITECTURE" ] || [ -z "$VERSION" ] || [ -z "$BINARY_PATH" ] || [ -z "$OUTPUT_PATH" ]; then
  echo "Usage: $0 <architecture> <version> <binary_path> <output_path>"
  exit 1
fi

PROJECT=$(cd "$(dirname "$0")/.."; pwd)

# Convert version to APK format:
#   1.13.0-beta.8  -> 1.13.0_beta8-r0
#   1.13.0-rc.3    -> 1.13.0_rc3-r0
#   1.13.0         -> 1.13.0-r0
APK_VERSION=$(echo "$VERSION" | sed -E 's/-([a-z]+)\.([0-9]+)/_\1\2/')
APK_VERSION="${APK_VERSION}-r0"

ROOT_DIR=$(mktemp -d)
trap 'rm -rf "$ROOT_DIR"' EXIT

# Binary
install -Dm755 "$BINARY_PATH" "$ROOT_DIR/usr/bin/sing-box"

# Config files
install -Dm644 "$PROJECT/release/config/config.json" "$ROOT_DIR/etc/sing-box/config.json"
install -Dm755 "$PROJECT/release/config/sing-box.initd" "$ROOT_DIR/etc/init.d/sing-box"
install -Dm644 "$PROJECT/release/config/sing-box.confd" "$ROOT_DIR/etc/conf.d/sing-box"

# Service files
install -Dm644 "$PROJECT/release/config/sing-box.service" "$ROOT_DIR/usr/lib/systemd/system/sing-box.service"
install -Dm644 "$PROJECT/release/config/sing-box@.service" "$ROOT_DIR/usr/lib/systemd/system/sing-box@.service"

# Completions
install -Dm644 "$PROJECT/release/completions/sing-box.bash" "$ROOT_DIR/usr/share/bash-completion/completions/sing-box.bash"
install -Dm644 "$PROJECT/release/completions/sing-box.fish" "$ROOT_DIR/usr/share/fish/vendor_completions.d/sing-box.fish"
install -Dm644 "$PROJECT/release/completions/sing-box.zsh" "$ROOT_DIR/usr/share/zsh/site-functions/_sing-box"

# License
install -Dm644 "$PROJECT/LICENSE" "$ROOT_DIR/usr/share/licenses/sing-box/LICENSE"

# APK metadata
PACKAGES_DIR="$ROOT_DIR/lib/apk/packages"
mkdir -p "$PACKAGES_DIR"

# .conffiles
cat > "$PACKAGES_DIR/.conffiles" <<'EOF'
/etc/conf.d/sing-box
/etc/init.d/sing-box
/etc/sing-box/config.json
EOF

# .conffiles_static (sha256 checksums)
while IFS= read -r conffile; do
  sha256=$(sha256sum "$ROOT_DIR$conffile" | cut -d' ' -f1)
  echo "$conffile $sha256"
done < "$PACKAGES_DIR/.conffiles" > "$PACKAGES_DIR/.conffiles_static"

# .list (all files, excluding lib/apk/packages/ metadata)
(cd "$ROOT_DIR" && find . -type f -o -type l) \
  | sed 's|^\./|/|' \
  | grep -v '^/lib/apk/packages/' \
  | sort > "$PACKAGES_DIR/.list"

# Build APK
apk mkpkg \
  --info "name:sing-box" \
  --info "version:${APK_VERSION}" \
  --info "description:The universal proxy platform." \
  --info "arch:${ARCHITECTURE}" \
  --info "license:GPL-3.0-or-later with name use or association addition" \
  --info "origin:sing-box" \
  --info "url:https://sing-box.sagernet.org/" \
  --info "maintainer:nekohasekai <contact-git@sekai.icu>" \
  --files "$ROOT_DIR" \
  --output "$OUTPUT_PATH"
