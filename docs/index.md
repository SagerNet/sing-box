---
description: Welcome to the wiki page for the sing-box project.
---

# Home

Welcome to the wiki page for the sing-box project.

The universal proxy platform.

## Installation

sing-box requires Golang **1.18.5** or a higher version.

```bash
go install -v github.com/sagernet/sing-box/cmd/sing-box@latest
```

Install with options:

```bash
go install -v -tags with_clash_api github.com/sagernet/sing-box/cmd/sing-box@latest
```

| Build Tag                          | Description                                                                                                                                                                                                                                                                                                                               |
|------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `with_quic`                        | Build with QUIC support, see [QUIC and HTTP3 dns transports](./configuration/dns/server.md), [Naive inbound](./configuration/inbound/naive.md), [Hysteria Inbound](./configuration/inbound/hysteria.md), [Hysteria Outbound](./configuration/outbound/hysteria.md) and [V2Ray QUIC transport](./configuration/shared/v2ray-transport.md). |
| `with_grpc`                        | Build with standard gRPC support, see [V2Ray gRPC transport](./configuration/shared/v2ray-transport.md).                                                                                                                                                                                                                                  |
| `with_wireguard`                   | Build with WireGuard support, see [WireGuard outbound](./configuration/outbound/wireguard.md).                                                                                                                                                                                                                                            |
| `with_acme`                        | Build with ACME TLS certificate issuer support, see [TLS](./configuration/shared/tls.md).                                                                                                                                                                                                                                                 |
| `with_clash_api`                   | Build with Clash API support, see [Experimental](./configuration/experimental/index.md).                                                                                                                                                                                                                                                  |
| `no_gvisor`                        | Build without gVisor Tun stack support, see [Tun inbound](./configuration/inbound/tun.md).                                                                                                                                                                                                                                                |
| `with_embedded_tor` (CGO required) | Build with embedded Tor support, see [Tor outbound](./configuration/outbound/tor.md).                                                                                                                                                                                                                                                     |
| `with_lwip` (CGO required)         | Build with LWIP Tun stack support, see [Tun inbound](./configuration/inbound/tun.md).                                                                                                                                                                                                                                                     |

The binary is built under `$GOPATH/bin`

```bash
sing-box version
```

Systemd is recommended to use as sing-box management tool,
see [Linux server installation example](./examples/linux-server-installation).

## Contributors

[![](https://opencollective.com/sagernet/contributors.svg?width=740&button=false)](https://github.com/sagernet/sing-box/graphs/contributors)

## License

```
Copyright (C) 2022 by nekohasekai <contact-sagernet@sekai.icu>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
```

