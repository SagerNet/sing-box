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

| 类型             | 格式                             |
|----------------|--------------------------------|
| `direct`       | [Direct](./direct)             |
| `block`        | [Block](./block)               |
| `socks`        | [SOCKS](./socks)               |
| `http`         | [HTTP](./http)                 |
| `shadowsocks`  | [Shadowsocks](./shadowsocks)   |
| `vmess`        | [VMess](./vmess)               |
| `trojan`       | [Trojan](./trojan)             |
| `wireguard`    | [Wireguard](./wireguard)       |
| `hysteria`     | [Hysteria](./hysteria)         |
| `shadowsocksr` | [ShadowsocksR](./shadowsocksr) |
| `vless`        | [VLESS](./vless)               |
| `shadowtls`    | [ShadowTLS](./shadowtls)       |
| `tuic`         | [TUIC](./tuic)                 |
| `hysteria2`    | [Hysteria2](./hysteria2)       |
| `tor`          | [Tor](./tor)                   |
| `ssh`          | [SSH](./ssh)                   |
| `dns`          | [DNS](./dns)                   |
| `selector`     | [Selector](./selector)         |
| `urltest`      | [URLTest](./urltest)           |

#### tag

出站的标签。

### 特性

#### 支持 IP 连接的出站

* `WireGuard`
