# Install from source

sing-box requires Golang **1.18.5** or a higher version.

```bash
go install -v github.com/sagernet/sing-box/cmd/sing-box@latest
```

Install with options:

```bash
go install -v -tags with_clash_api github.com/sagernet/sing-box/cmd/sing-box@latest
```

| Build Tag                          | Description                                                                                                                                                                                                                                                                                                                |
|------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `with_quic`                        | Build with QUIC support, see [QUIC and HTTP3 DNS transports](/configuration/dns/server), [Naive inbound](/configuration/inbound/naive), [Hysteria Inbound](/configuration/inbound/hysteria), [Hysteria Outbound](/configuration/outbound/hysteria) and [V2Ray Transport#QUIC](/configuration/shared/v2ray-transport#quic). |
| `with_grpc`                        | Build with standard gRPC support, see [V2Ray Transport#gRPC](/configuration/shared/v2ray-transport#grpc).                                                                                                                                                                                                                  |
| `with_dhcp`                        | Build with DHCP support, see [DHCP DNS transport](/configuration/dns/server).                                                                                                                                                                                                                                              |
| `with_wireguard`                   | Build with WireGuard support, see [WireGuard outbound](/configuration/outbound/wireguard).                                                                                                                                                                                                                                 |
| `with_shadowsocksr`                | Build with ShadowsocksR support, see [ShadowsocksR outbound](/configuration/outbound/shadowsocksr).                                                                                                                                                                                                                        |
| `with_ech`                         | Build with TLS ECH extension support for TLS outbound, see [TLS](/configuration/shared/tls#ech).                                                                                                                                                                                                                           |
| `with_utls`                        | Build with [uTLS](https://github.com/refraction-networking/utls) support for TLS outbound, see [TLS](/configuration/shared/tls#utls).                                                                                                                                                                                      |
| `with_reality_server`              | Build with reality TLS server support,  see [TLS](/configuration/shared/tls).                                                                                                                                                                                                                                              |
| `with_acme`                        | Build with ACME TLS certificate issuer support, see [TLS](/configuration/shared/tls).                                                                                                                                                                                                                                      |
| `with_clash_api`                   | Build with Clash API support, see [Experimental](/configuration/experimental#clash-api-fields).                                                                                                                                                                                                                            |
| `with_v2ray_api`                   | Build with V2Ray API support, see [Experimental](/configuration/experimental#v2ray-api-fields).                                                                                                                                                                                                                            |
| `with_gvisor`                      | Build with gVisor support, see [Tun inbound](/configuration/inbound/tun#stack) and [WireGuard outbound](/configuration/outbound/wireguard#system_interface).                                                                                                                                                               |
| `with_embedded_tor` (CGO required) | Build with embedded Tor support, see [Tor outbound](/configuration/outbound/tor).                                                                                                                                                                                                                                          |
| `with_lwip` (CGO required)         | Build with LWIP Tun stack support, see [Tun inbound](/configuration/inbound/tun#stack).                                                                                                                                                                                                                                    |

The binary is built under `$GOBIN` (or `$GOPATH/bin` if not set):

```bash
sing-box version
```

It is also recommended to use systemd to manage sing-box service,
see [Linux server installation example](/examples/linux-server-installation).

Also, there is an unofficial community-maintained build with all tags enabled, which builds the latest (stable, or unstable) tag on schedule that is available here: <https://github.com/z4x7k/sing-box-all>.
