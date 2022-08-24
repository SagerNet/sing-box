# 出站

### 结构

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

### 字段

| 类型            | 格式                           |
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

出站的标签。