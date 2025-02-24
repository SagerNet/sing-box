---
icon: material/alert-decagram
---

!!! quote "Changes in sing-box 1.12.0"

    :material-plus: [type](#type)

# DNS Server

### Structure

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

The type of the DNS server.

| Type            | Format                      |
|-----------------|-----------------------------|
| empty (default) | [Legacy](./legacy/)         |
| `tcp`           | [TCP](./tcp/)               |
| `udp`           | [UDP](./udp/)               |
| `tls`           | [TLS](./tls/)               |
| `https`         | [HTTPS](./https/)           |
| `quic`          | [QUIC](./quic/)             |
| `h3`            | [HTTP/3](./http3/)          |
| `predefined`    | [Predefined](./predefined/) |
| `dhcp`          | [DHCP](./dhcp/)             |
| `fakeip`        | [Fake IP](./fakeip/)        |
| `tailscale`     | [Tailscale](./tailscale/)   |

#### tag

The tag of the DNS server.
