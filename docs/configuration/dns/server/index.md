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

| Type            | Format                                              |
|-----------------|-----------------------------------------------------|
| empty (default) | [Legacy](/configuration/dns/server/legacy/)         |
| `tcp`           | [TCP](/configuration/dns/server/tcp/)               |
| `udp`           | [UDP](/configuration/dns/server/udp/)               |
| `tls`           | [TLS](/configuration/dns/server/tls/)               |
| `https`         | [HTTPS](/configuration/dns/server/https/)           |
| `quic`          | [QUIC](/configuration/dns/server/quic/)             |
| `h3`            | [HTTP/3](/configuration/dns/server/http3/)          |
| `predefined`    | [Predefined](/configuration/dns/server/predefined/) |
| `dhcp`          | [DHCP](/configuration/dns/server/dhcp/)             |
| `fakeip`        | [Fake IP](/configuration/dns/server/fakeip/)        |


#### tag

The tag of the DNS server.
