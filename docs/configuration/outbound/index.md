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
| `socks`       | [Socks](./socks)             |
| `http`        | [HTTP](./http)               |
| `shadowsocks` | [Shadowsocks](./shadowsocks) |
| `dns`         | [DNS](./dns)                 |
| `selector`    | [Selector](./selector)       |
| `urltest`     | [URLTest](./urltest)         |

#### tag

The tag of the outbound.