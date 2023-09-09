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
| `shadowtls`   | [ShadowTLS](./shadowtls)     | TCP  |
| `tuic`        | [TUIC](./tuic)               | X    |
| `hysteria2`   | [Hysteria2](./hysteria2)     | X    |
| `vless`       | [VLESS](./vless)             | TCP  |
| `tun`         | [Tun](./tun)                 | X    |
| `redirect`    | [Redirect](./redirect)       | X    |
| `tproxy`      | [TProxy](./tproxy)           | X    |

#### tag

入站的标签。