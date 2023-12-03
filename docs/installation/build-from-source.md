---
icon: material/file-code
---

# Build from source

## :material-graph: Requirements

Before sing-box 1.4.0:

* Go 1.18.5 - 1.20.x

Since sing-box 1.4.0:

* Go 1.18.5 - ~
* Go 1.20.0 - ~ if `with_quic` tag enabled

You can download and install Go from: https://go.dev/doc/install, latest version is recommended.

## :material-fast-forward: Simple Build

```bash
make
```

Or build and install binary to `$GOBIN`:

```bash
make install
```

## :material-cog: Custom Build

```bash
TAGS="tag_a tag_b" make
```

or

```bash
go build -tags "tag_a tag_b" ./cmd/sing-box
```

## :material-folder-settings: Build Tags

| Build Tag                          | Enabled by default | Description                                                                                                                                                                                                                                                                                                                |
|------------------------------------|--------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `with_quic`                        | ✔                  | Build with QUIC support, see [QUIC and HTTP3 DNS transports](/configuration/dns/server), [Naive inbound](/configuration/inbound/naive), [Hysteria Inbound](/configuration/inbound/hysteria), [Hysteria Outbound](/configuration/outbound/hysteria) and [V2Ray Transport#QUIC](/configuration/shared/v2ray-transport#quic). |
| `with_grpc`                        | ✖️                 | Build with standard gRPC support, see [V2Ray Transport#gRPC](/configuration/shared/v2ray-transport#grpc).                                                                                                                                                                                                                  |
| `with_dhcp`                        | ✔                  | Build with DHCP support, see [DHCP DNS transport](/configuration/dns/server).                                                                                                                                                                                                                                              |
| `with_wireguard`                   | ✔                  | Build with WireGuard support, see [WireGuard outbound](/configuration/outbound/wireguard).                                                                                                                                                                                                                                 |
| `with_ech`                         | ✔                  | Build with TLS ECH extension support for TLS outbound, see [TLS](/configuration/shared/tls#ech).                                                                                                                                                                                                                           |
| `with_utls`                        | ✔                  | Build with [uTLS](https://github.com/refraction-networking/utls) support for TLS outbound, see [TLS](/configuration/shared/tls#utls).                                                                                                                                                                                      |
| `with_reality_server`              | ✔                  | Build with reality TLS server support,  see [TLS](/configuration/shared/tls).                                                                                                                                                                                                                                              |
| `with_acme`                        | ✔                  | Build with ACME TLS certificate issuer support, see [TLS](/configuration/shared/tls).                                                                                                                                                                                                                                      |
| `with_clash_api`                   | ✔                  | Build with Clash API support, see [Experimental](/configuration/experimental#clash-api-fields).                                                                                                                                                                                                                            |
| `with_v2ray_api`                   | ✖️                 | Build with V2Ray API support, see [Experimental](/configuration/experimental#v2ray-api-fields).                                                                                                                                                                                                                            |
| `with_gvisor`                      | ✔                  | Build with gVisor support, see [Tun inbound](/configuration/inbound/tun#stack) and [WireGuard outbound](/configuration/outbound/wireguard#system_interface).                                                                                                                                                               |
| `with_embedded_tor` (CGO required) | ✖️                 | Build with embedded Tor support, see [Tor outbound](/configuration/outbound/tor).                                                                                                                                                                                                                                          |
| `with_lwip` (CGO required)         | ✖️                 | Build with LWIP Tun stack support, see [Tun inbound](/configuration/inbound/tun#stack).                                                                                                                                                                                                                                    |


It is not recommended to change the default build tag list unless you really know what you are adding.
