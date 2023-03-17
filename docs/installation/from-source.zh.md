# 从源代码安装

sing-box 需要 Golang **1.18.5** 或更高版本。

```bash
go install -v github.com/sagernet/sing-box/cmd/sing-box@latest
```

自定义安装：

```bash
go install -v -tags with_clash_api github.com/sagernet/sing-box/cmd/sing-box@latest
```

| 构建标志                         | 描述                                                                                                                                                                                                                                                                      |
|------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `with_quic`                  | 启用 QUIC 支持，参阅 [QUIC 和 HTTP3 DNS 传输层](/configuration/dns/server)，[Naive 入站](/configuration/inbound/naive)，[Hysteria 入站](/configuration/inbound/hysteria)，[Hysteria 出站](/configuration/outbound/hysteria) 和 [V2Ray 传输层#QUIC](/configuration/shared/v2ray-transport#quic)。 |
| `with_grpc`                  | 启用标准 gRPC 支持，参阅 [V2Ray 传输层#gRPC](/configuration/shared/v2ray-transport#grpc)。                                                                                                                                                                                           |
| `with_dhcp`                  | 启用 DHCP 支持，参阅 [DHCP DNS 传输层](/configuration/dns/server)。                                                                                                                                                                                                                |
| `with_wireguard`             | 启用 WireGuard 支持，参阅 [WireGuard 出站](/configuration/outbound/wireguard)。                                                                                                                                                                                                   |
| `with_shadowsocksr`          | 启用 ShadowsocksR 支持，参阅 [ShadowsocksR 出站](/configuration/outbound/shadowsocksr)。                                                                                                                                                                                          |
| `with_ech`                   | 启用 TLS ECH 扩展支持，参阅 [TLS](/configuration/shared/tls#ech)。                                                                                                                                                                                                                |
| `with_utls`                  | 启用 [uTLS](https://github.com/refraction-networking/utls) 支持，参阅 [TLS](/configuration/shared/tls#utls)。                                                                                                                                                                   |
| `with_reality_server`        | 启用 reality TLS 服务器支持，参阅 [TLS](/configuration/shared/tls)。                                                                                                                                                                                                               |
| `with_acme`                  | 启用 ACME TLS 证书签发支持，参阅 [TLS](/configuration/shared/tls)。                                                                                                                                                                                                                 |
| `with_clash_api`             | 启用 Clash API 支持，参阅 [实验性](/configuration/experimental#clash-api-fields)。                                                                                                                                                                                                 |
| `with_v2ray_api`             | 启用 V2Ray API 支持，参阅 [实验性](/configuration/experimental#v2ray-api-fields)。                                                                                                                                                                                                 |
| `with_gvisor`                | 启用 gVisor 支持，参阅 [Tun 入站](/configuration/inbound/tun#stack) 和 [WireGuard 出站](/configuration/outbound/wireguard#system_interface)。                                                                                                                                        |
| `with_embedded_tor` (需要 CGO) | 启用 嵌入式 Tor 支持，参阅 [Tor 出站](/configuration/outbound/tor)。                                                                                                                                                                                                                 |
| `with_lwip` (需要 CGO)         | 启用 LWIP Tun 栈支持，参阅 [Tun 入站](/configuration/inbound/tun#stack)。                                                                                                                                                                                                          |

二进制文件将被构建在 `$GOPATH/bin` 下。

```bash
sing-box version
```

同时推荐使用 systemd 来管理 sing-box 服务器实例。
参阅 [Linux 服务器安装示例](/examples/linux-server-installation)。