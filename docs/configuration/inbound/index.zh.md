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

| 类型            | 格式                           | 注入支持 |
|---------------|------------------------------|------|
| `direct`      | [Direct](./direct)           | X    |
| `mixed`       | [Mixed](./mixed)             | TCP  |
| `socks`       | [SOCKS](./socks)             | TCP  |
| `http`        | [HTTP](./http)               | TCP  |
| `shadowsocks` | [Shadowsocks](./shadowsocks) | TCP  |
| `vmess`       | [VMess](./vmess)             | TCP  |
| `trojan`      | [Trojan](./trojan)           | TCP  |
| `naive`       | [Naive](./naive)             | X    |
| `hysteria`    | [Hysteria](./hysteria)       | X    |
| `tun`         | [Tun](./tun)                 | X    |
| `redirect`    | [Redirect](./redirect)       | X    |
| `tproxy`      | [TProxy](./tproxy)           | X    |

#### tag

入站的标签。