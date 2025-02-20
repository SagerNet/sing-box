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

| Type          | Format                        | Injectable       |
|---------------|-------------------------------|------------------|
| `direct`      | [Direct](./direct/)           | :material-close: |
| `mixed`       | [Mixed](./mixed/)             | TCP              |
| `socks`       | [SOCKS](./socks/)             | TCP              |
| `http`        | [HTTP](./http/)               | TCP              |
| `shadowsocks` | [Shadowsocks](./shadowsocks/) | TCP              |
| `vmess`       | [VMess](./vmess/)             | TCP              |
| `trojan`      | [Trojan](./trojan/)           | TCP              |
| `naive`       | [Naive](./naive/)             | :material-close: |
| `hysteria`    | [Hysteria](./hysteria/)       | :material-close: |
| `shadowtls`   | [ShadowTLS](./shadowtls/)     | TCP              |
| `tuic`        | [TUIC](./tuic/)               | :material-close: |
| `hysteria2`   | [Hysteria2](./hysteria2/)     | :material-close: |
| `vless`       | [VLESS](./vless/)             | TCP              |
| `anytls`      | [AnyTLS](./anytls/)           | TCP              |
| `tun`         | [Tun](./tun/)                 | :material-close: |
| `redirect`    | [Redirect](./redirect/)       | :material-close: |
| `tproxy`      | [TProxy](./tproxy/)           | :material-close: |

#### tag

The tag of the inbound.