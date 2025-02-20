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

| 类型            | 格式                            | 注入支持             |
|---------------|-------------------------------|------------------|
| `direct`      | [Direct](./direct/)           | :material-close: |
| `mixed`       | [Mixed](./mixed/)             | TCP              |
| `socks`       | [SOCKS](./socks/)             | TCP              |
| `http`        | [HTTP](./http/)               | TCP              |
| `shadowsocks` | [Shadowsocks](./shadowsocks/) | TCP              |
| `vmess`       | [VMess](./vmess/)             | TCP              |
| `trojan`      | [Trojan](./trojan/)           | TCP              |
| `naive`       | [Naive](./naive/)             | :material-close: |
| `hysteria`    | [Hysteria](./hysteria/)       | :material-close: |
| `shadowtls`   | [ShadowTLS](./shadowtls/)     | TCP              |
| `tuic`        | [TUIC](./tuic/)               | :material-close: |
| `hysteria2`   | [Hysteria2](./hysteria2/)     | :material-close: |
| `vless`       | [VLESS](./vless/)             | TCP              |
| `anytls`      | [AnyTLS](./anytls/)           | TCP              |
| `tun`         | [Tun](./tun/)                 | :material-close: |
| `redirect`    | [Redirect](./redirect/)       | :material-close: |
| `tproxy`      | [TProxy](./tproxy/)           | :material-close: |

#### tag

入站的标签。