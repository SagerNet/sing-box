# NullVPN — sing-box Server Integration

This directory contains all NullVPN-specific server configuration, templates, and tooling for deploying sing-box as the TCP/443 transport layer over AmneziaWG.

## Architecture

```
[Android Client — sing-box-for-android]
         │
         │  TLS 1.3 (uTLS Chrome fingerprint)
         │  TCP :443, SNI: www.cloudflare.com
         ▼
[sing-box — VLESS + XTLS-Reality inbound]
         │  (nullvpn-backend generates per-user UUID + short_id)
         │  reality private_key rotated monthly via Ansible
         ▼
[AmneziaWG tunnel — awg0 — obfuscated UDP]
         │  Jc=4, Jmin=50, Jmax=1000, S1=0, S2=0, H1-H4=random
         ▼
[Internet — Instagram, Telegram, etc.]
```

## Files

| Path | Purpose |
|---|---|
| `config/server.json.template` | sing-box server config template (Jinja2-style `{{ }}` vars) |
| `config/generate_user_config.sh` | Generates per-user sing-box outbound JSON for provisioning |
| `config/generate_reality_keys.sh` | Generates Reality keypair (run once per server) |
| `ansible/roles/singbox/` | Ansible role: install, configure, systemd, certbot-bypass |
| `provisioning/push_config.py` | Backend helper: push provisioned config to nullvpn-registry |

## Deployment

```bash
# 1. Generate Reality keypair (server-side, once)
bash nullvpn/config/generate_reality_keys.sh

# 2. Deploy via Ansible
ansible-playbook -i inventory.yml nullvpn/ansible/singbox.yml

# 3. Verify sing-box is running
systemctl status sing-box
sing-box check -c /etc/sing-box/config.json
```

## Provisioning Integration

The backend calls `provisioning/push_config.py` after user registration to push a per-user sing-box outbound config to `nullvpnnet/nullvpn-registry/configs/{device_id}.json`.

See [nullvpnnet/nullvpn-registry](https://github.com/nullvpnnet/nullvpn-registry) for the GitHub Actions workflow that triggers provisioning.
