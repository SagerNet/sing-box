---
icon: material/alert-decagram
---

!!! quote "sing-box 1.12.0 中的更改"

    :material-plus: [type](#type)

# DNS Server

### 结构

```json
{
  "dns": {
    "servers": [
      {
        "type": "",
        "tag": ""
      }
    ]
  }
}
```

#### type

DNS 服务器的类型。

| 类型              | 格式                        |
|-----------------|---------------------------|
| empty (default) | [Legacy](./legacy/)       |
| `local`         | [Local](./local/)         |
| `hosts`         | [Hosts](./hosts/)         |
| `tcp`           | [TCP](./tcp/)             |
| `udp`           | [UDP](./udp/)             |
| `tls`           | [TLS](./tls/)             |
| `quic`          | [QUIC](./quic/)           |
| `https`         | [HTTPS](./https/)         |
| `h3`            | [HTTP/3](./http3/)        |
| `dhcp`          | [DHCP](./dhcp/)           |
| `fakeip`        | [Fake IP](./fakeip/)      |
| `tailscale`     | [Tailscale](./tailscale/) |
| `resolved`      | [Resolved](./resolved/)   |

#### tag

DNS 服务器的标签。
