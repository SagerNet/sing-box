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
| `socks`       | [Socks](./socks)             |
| `http`        | [HTTP](./http)               |
| `shadowsocks` | [Shadowsocks](./shadowsocks) |
| `vmess`       | [VMess](./vmess)             |
| `tun`         | [Tun](./tun)                 |
| `redirect`    | [Redirect](./redirect)       |
| `tproxy`      | [TProxy](./tproxy)           |

#### tag

The tag of the inbound.