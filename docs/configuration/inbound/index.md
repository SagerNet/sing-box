# Inbound

### Structure

```json
{
  "inbounds": [
    {
      "type": "",
      "tag": ""
    }
  ]
}
```

### Fields

| Type          | Format                       | Injectable |
|---------------|------------------------------|------------|
| `direct`      | [Direct](./direct)           | X          |
| `mixed`       | [Mixed](./mixed)             | TCP        |
| `socks`       | [SOCKS](./socks)             | TCP        |
| `http`        | [HTTP](./http)               | TCP        |
| `shadowsocks` | [Shadowsocks](./shadowsocks) | TCP        |
| `vmess`       | [VMess](./vmess)             | TCP        |
| `trojan`      | [Trojan](./trojan)           | TCP        |
| `naive`       | [Naive](./naive)             | X          |
| `hysteria`    | [Hysteria](./hysteria)       | X          |
| `shadowtls`   | [ShadowTLS](./shadowtls)     | TCP        |
| `tuic`        | [TUIC](./tuic)               | X          |
| `hysteria2`   | [Hysteria2](./hysteria2)     | X          |
| `vless`       | [VLESS](./vless)             | TCP        |
| `tun`         | [Tun](./tun)                 | X          |
| `redirect`    | [Redirect](./redirect)       | X          |
| `tproxy`      | [TProxy](./tproxy)           | X          |

#### tag

The tag of the inbound.