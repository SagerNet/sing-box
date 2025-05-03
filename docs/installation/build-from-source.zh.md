---
icon: material/file-code
---

# 从源代码构建

## :material-graph: 要求

### sing-box 1.11

* Go 1.23.1 - ~

### sing-box 1.10

* Go 1.20.0 - ~
* Go 1.21.0 - ~ with tag `with_ech` enabled

### sing-box 1.9

* Go 1.18.5 - 1.22.x
* Go 1.20.0 - 1.22.x with tag `with_quic`, or `with_utls` enabled
* Go 1.21.0 - 1.22.x with tag `with_ech` enabled

您可以从 https://go.dev/doc/install 下载并安装 Go，推荐使用最新版本。

## :material-fast-forward: 快速开始

```bash
make
```

或者构建二进制文件并将其安装到 `$GOBIN`：

```bash
make install
```

## :material-cog: 自定义构建

```bash
TAGS="tag_a tag_b" make
```

or

```bash
go build -tags "tag_a tag_b" ./cmd/sing-box
```

## :material-folder-settings: 构建标记

| 构建标记                               | 默认启动              | 说明                                                                                                                                                                                                                                                                                                                             |
|------------------------------------|-------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `with_quic`                        | :material-check:  | Build with QUIC support, see [QUIC and HTTP3 DNS transports](/configuration/dns/server/), [Naive inbound](/configuration/inbound/naive/), [Hysteria Inbound](/configuration/inbound/hysteria/), [Hysteria Outbound](/configuration/outbound/hysteria/) and [V2Ray Transport#QUIC](/configuration/shared/v2ray-transport#quic). |
| `with_grpc`                        | :material-close:️ | Build with standard gRPC support, see [V2Ray Transport#gRPC](/configuration/shared/v2ray-transport#grpc).                                                                                                                                                                                                                      |
| `with_dhcp`                        | :material-check:  | Build with DHCP support, see [DHCP DNS transport](/configuration/dns/server/).                                                                                                                                                                                                                                                 |
| `with_wireguard`                   | :material-check:  | Build with WireGuard support, see [WireGuard outbound](/configuration/outbound/wireguard/).                                                                                                                                                                                                                                    |
| `with_utls`                        | :material-check:  | Build with [uTLS](https://github.com/refraction-networking/utls) support for TLS outbound, see [TLS](/configuration/shared/tls#utls).                                                                                                                                                                                          |
| `with_acme`                        | :material-check:  | Build with ACME TLS certificate issuer support, see [TLS](/configuration/shared/tls/).                                                                                                                                                                                                                                         |
| `with_clash_api`                   | :material-check:  | Build with Clash API support, see [Experimental](/configuration/experimental#clash-api-fields).                                                                                                                                                                                                                                |
| `with_v2ray_api`                   | :material-close:️ | Build with V2Ray API support, see [Experimental](/configuration/experimental#v2ray-api-fields).                                                                                                                                                                                                                                |
| `with_gvisor`                      | :material-check:  | Build with gVisor support, see [Tun inbound](/configuration/inbound/tun#stack) and [WireGuard outbound](/configuration/outbound/wireguard#system_interface).                                                                                                                                                                   |
| `with_embedded_tor` (CGO required) | :material-close:️ | Build with embedded Tor support, see [Tor outbound](/configuration/outbound/tor/).                                                                                                                                                                                                                                             |
| `with_tailscale`                   | :material-check:  | Build with Tailscale support, see [Tailscale endpoint](/configuration/endpoint/tailscale)                                                                                                                                                                                                                                      |

除非您确实知道您正在启用什么，否则不建议更改默认构建标签列表。
