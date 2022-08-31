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
| `tun`         | [Tun](./tun)                 | X          |
| `redirect`    | [Redirect](./redirect)       | X          |
| `tproxy`      | [TProxy](./tproxy)           | X          |

#### tag

The tag of the inbound.