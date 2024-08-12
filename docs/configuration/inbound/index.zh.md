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
| `direct`      | [Direct](./direct/)           | N/A  |
| `mixed`       | [Mixed](./mixed/)             | TCP  |
| `socks`       | [SOCKS](./socks/)             | TCP  |
| `http`        | [HTTP](./http/)               | TCP  |
| `shadowsocks` | [Shadowsocks](./shadowsocks/) | TCP  |
| `vmess`       | [VMess](./vmess/)             | TCP  |
| `trojan`      | [Trojan](./trojan/)           | TCP  |
| `naive`       | [Naive](./naive/)             | N/A  |
| `hysteria`    | [Hysteria](./hysteria/)       | N/A  |
| `shadowtls`   | [ShadowTLS](./shadowtls/)     | TCP  |
| `tuic`        | [TUIC](./tuic/)               | N/A  |
| `hysteria2`   | [Hysteria2](./hysteria2/)     | N/A  |
| `vless`       | [VLESS](./vless/)             | TCP  |
| `tun`         | [Tun](./tun/)                 | N/A  |
| `redirect`    | [Redirect](./redirect/)       | N/A  |
| `tproxy`      | [TProxy](./tproxy/)           | N/A  |

#### tag

入站的标签。