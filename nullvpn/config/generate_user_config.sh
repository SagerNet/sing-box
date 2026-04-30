#!/usr/bin/env bash
# generate_user_config.sh
# Generates a per-user sing-box outbound JSON config for NullVPN provisioning.
# Called by nullvpn-backend after device registration to build the config
# that gets pushed to nullvpn-registry/configs/{device_id}.json
#
# Usage:
#   bash generate_user_config.sh <device_id> <user_uuid> [premium]
#
# Output: JSON to stdout — pipe to the provisioning script.

set -euo pipefail

DEVICE_ID="${1:?device_id required}"
USER_UUID="${2:?user_uuid required}"
TIER="${3:-trial}"

# Load from environment (set by nullvpn-backend or Ansible)
SERVER_ENDPOINT="${SERVER_ENDPOINT:?SERVER_ENDPOINT env var required}"
SERVER_PORT="${SERVER_PORT:-443}"
REALITY_PUBLIC_KEY="${REALITY_PUBLIC_KEY:?REALITY_PUBLIC_KEY env var required}"
REALITY_SHORT_ID="${REALITY_SHORT_ID:?REALITY_SHORT_ID env var required}"
REALITY_SNI="${REALITY_SNI:-www.cloudflare.com}"

# Premium uses a different server endpoint if configured
if [ "$TIER" = "premium" ] && [ -n "${PREMIUM_SERVER_ENDPOINT:-}" ]; then
  SERVER_ENDPOINT="$PREMIUM_SERVER_ENDPOINT"
  SERVER_PORT="${PREMIUM_SERVER_PORT:-443}"
fi

cat <<EOF
{
  "device_id": "$DEVICE_ID",
  "tier": "$TIER",
  "provisioned_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "outbound": {
    "type": "vless",
    "tag": "nullvpn-out",
    "server": "$SERVER_ENDPOINT",
    "server_port": $SERVER_PORT,
    "uuid": "$USER_UUID",
    "flow": "xtls-rprx-vision",
    "tls": {
      "enabled": true,
      "server_name": "$REALITY_SNI",
      "utls": {
        "enabled": true,
        "fingerprint": "chrome"
      },
      "reality": {
        "enabled": true,
        "public_key": "$REALITY_PUBLIC_KEY",
        "short_id": "$REALITY_SHORT_ID"
      }
    }
  }
}
EOF
