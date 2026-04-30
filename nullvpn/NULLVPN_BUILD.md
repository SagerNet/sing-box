# NullVPN sing-box Build Configuration

## Purpose

This directory contains NullVPN-specific build configuration for the sing-box fork.
The goal is a **minimal binary** targeting TCP/443 with VLESS+XTLS-Reality transport
for DPI bypass (Instagram, Telegram unblocking on Russian/Iranian networks).

## Retained Protocols

| Protocol | Tag | Reason |
|----------|-----|--------|
| `vless` | `with_vless` | Primary client protocol |
| `tun` | `with_tun` | Android TUN inbound |
| `mixed` | `with_mixed` | Local SOCKS+HTTP inbound |
| `shadowsocks` | `with_shadowsocks` | Fallback transport |
| `direct` | _(always built)_ | DNS and bypass routing |
| `dns` | _(always built)_ | DNS inbound/outbound |

## Removed Protocols (not needed for NullVPN scope)

- `hysteria` / `hysteria2` — UDP-based, blocked by DPI targets anyway
- `naive` — HTTP/2 proxy, not part of NullVPN stack
- `tor` — not used
- `ssh` — not used
- `vmess` — legacy, not needed if VLESS is primary
- `trojan` — not needed
- `shadowtls` — redundant when using Reality
- `wireguard` — handled by VPNsd/amneziawg-go stack separately

## Build Command

```bash
# Build NullVPN-stripped binary (server)
make -f Makefile.nullvpn build-server

# Build NullVPN Android library
make -f Makefile.nullvpn build-android
```

## Config Entry Point

See `config/` directory in `nullvpnnet/sing-box-for-android` for the Android client template,
and `server-setup/singbox/` in `nullvpnnet/unified-scripts` for the server-side config.
