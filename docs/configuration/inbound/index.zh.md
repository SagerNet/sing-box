# 入站

### 结构

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

### 字段

| 类型            | 格式                           |
|---------------|------------------------------|
| `direct`      | [Direct](./direct)           |
| `mixed`       | [Mixed](./mixed)             |
| `socks`       | [Socks](./socks)             |
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

入站的标签。