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
| `tun`         | [Tun](./tun)                 |
| `redirect`    | [Redirect](./redirect)       |
| `tproxy`      | [TProxy](./tproxy)           |
| `dns`         | [DNS](./dns)                 |

#### tag

The tag of the inbound.