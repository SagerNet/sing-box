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

| Type          | Format                       |
|---------------|------------------------------|
| `direct`      | [Direct](./direct)           |
| `mixed`       | [Mixed](./mixed)             |
| `socks`       | [SOCKS](./socks)             |
| `http`        | [HTTP](./http)               |
| `shadowsocks` | [Shadowsocks](./shadowsocks) |
| `vmess`       | [VMess](./vmess)             |
| `trojan`      | [Trojan](./trojan)           |
| `naive`       | [Naive](./naive)             |
| `hysteria`    | [Hysteria](./hysteria)       |
| `tun`         | [Tun](./tun)                 |
| `redirect`    | [Redirect](./redirect)       |
| `tproxy`      | [TProxy](./tproxy)           |

#### tag

The tag of the inbound.