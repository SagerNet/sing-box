!!! question "自 sing-box 1.11.0 起"

# 端点

端点是具有入站和出站行为的协议。

### 结构

```json
{
  "endpoints": [
    {
      "type": "",
      "tag": ""
    }
  ]
}
```

### 字段

| 类型               | 格式                                      |
|------------------|-----------------------------------------|
| `wireguard`      | [WireGuard](./wireguard/)               |
| `tailscale`      | [Tailscale](./tailscale/)               |
| `openconnect`    | [OpenConnect 客户端](./openconnect/)       |
| `openvpn-client` | [OpenVPN 客户端](./openvpn-client/)         |
| `openvpn-server` | [OpenVPN 服务器](./openvpn-server/)         |

#### tag

端点的标签。
