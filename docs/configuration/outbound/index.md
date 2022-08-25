# Outbound

### Structure

```json
{
  "outbounds": [
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
| `block`       | [Block](./block)             |
| `socks`       | [SOCKS](./socks)             |
| `http`        | [HTTP](./http)               |
| `shadowsocks` | [Shadowsocks](./shadowsocks) |
| `vmess`       | [VMess](./vmess)             |
| `trojan`      | [Trojan](./trojan)           |
| `wireguard`   | [Wireguard](./wireguard)     |
| `hysteria`    | [Hysteria](./hysteria)       |
| `tor`         | [Tor](./tor)                 |
| `ssh`         | [SSH](./ssh)                 |
| `dns`         | [DNS](./dns)                 |
| `selector`    | [Selector](./selector)       |

#### tag

The tag of the outbound.