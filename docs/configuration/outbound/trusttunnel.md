---
icon: material/new-box
---

!!! question "Since sing-box 1.14.0"

### Structure

```json
{
  "type": "trusttunnel",
  "tag": "trusttunnel-out",

  "server": "127.0.0.1",
  "server_port": 443,
  "username": "trust",
  "password": "tunnel",
  "health_check": true,
  "quic": false,
  "quic_congestion_control": "bbr",
  "tls": {},

  ... // Dial Fields
}
```

### Fields

#### server

==Required==

The server address.

#### server_port

==Required==

The server port.

#### username

==Required==

Authentication username.

#### password

Authentication password.

#### health_check

Enable periodic health check.

#### quic

Use QUIC transport.

- `false`: Use HTTP/2 over TCP.
- `true`: Use HTTP/3 over UDP.

#### quic_congestion_control

QUIC congestion control algorithm.

| Algorithm | Description |
|-----------|-------------|
| `bbr` | BBR |
| `bbr_standard` | BBR (Standard version) |
| `bbr2` | BBRv2 |
| `bbr_variant` | BBRv2 (An experimental variant) |
| `cubic` | CUBIC |
| `reno` | New Reno |

`bbr` is used by default.

#### tls

==Required==

Outbound TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
