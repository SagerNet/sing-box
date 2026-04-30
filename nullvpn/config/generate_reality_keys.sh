#!/usr/bin/env bash
# generate_reality_keys.sh
# Generates a XTLS-Reality keypair for sing-box.
# Run ONCE per server deployment. Store output in Ansible vault / secrets manager.
# Never commit the output to git.

set -euo pipefail

SINGBOX_BIN="${SINGBOX_BIN:-sing-box}"

if ! command -v "$SINGBOX_BIN" &>/dev/null; then
  echo "ERROR: sing-box binary not found at '$SINGBOX_BIN'"
  echo "Install first: bash nullvpn/ansible/install_singbox.sh"
  exit 1
fi

echo "=== Generating Reality keypair ==="
KEYPAIR_JSON=$("$SINGBOX_BIN" generate reality-keypair)

PRIV=$(echo "$KEYPAIR_JSON" | grep -oP '(?<="PrivateKey":\s")[^"]+' || \
         echo "$KEYPAIR_JSON" | awk -F'"' '/PrivateKey/{print $4}')
PUB=$(echo  "$KEYPAIR_JSON" | grep -oP '(?<="PublicKey":\s")[^"]+' || \
        echo "$KEYPAIR_JSON" | awk -F'"' '/PublicKey/{print $4}')

echo ""
echo "REALITY_PRIVATE_KEY=$PRIV"
echo "REALITY_PUBLIC_KEY=$PUB"
echo ""
echo "=== Generating short_ids (8 values) ==="
for i in $(seq 1 8); do
  openssl rand -hex 4
done

echo ""
echo "Store these values in Ansible vault:"
echo "  ansible/group_vars/all/vault.yml"
echo "  var names: vault_reality_private_key, vault_reality_short_ids (list)"
