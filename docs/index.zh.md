# 开始

欢迎来到该 sing-box 项目的文档页。

通用代理平台。

## 安装

sing-box 需要 Golang **1.18.5** 或更高版本。

```bash
go install -v github.com/sagernet/sing-box/cmd/sing-box@latest
```

自定义安装：

```bash
go install -v -tags with_clash_api github.com/sagernet/sing-box/cmd/sing-box@latest
```

| 构建标志                         | 描述                                                                                                                                                                                                                |
|------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `with_quic`                  | 启用 QUIC 支持，参阅 [QUIC 和 HTTP3 DNS 传输层](./configuration/dns/server)，[Naive 入站](./configuration/inbound/naive)，[Hysteria 入站](./configuration/inbound/hysteria) 和 [Hysteria 出站](./configuration/outbound/hysteria)。 |
| `with_grpc`                  | 启用 gRPC 支持，参阅 [V2Ray 传输层#gRPC](/configuration/shared/v2ray-transport#grpc)。                                                                                                                                      |
| `with_wireguard`             | 启用 WireGuard 支持，参阅 [WireGuard 出站](./configuration/outbound/wireguard)。                                                                                                                                           |
| `with_acme`                  | 启用 ACME TLS 证书签发支持，参阅 [TLS](./configuration/shared/tls)。                                                                                                                                                         |
| `with_clash_api`             | 启用 Clash api 支持，参阅 [实验性](./configuration/experimental#clash-api-fields)。                                                                                                                                         |
| `no_gvisor`                  | 禁用 gVisor Tun 栈支持，参阅 [Tun 入站](./configuration/inbound/tun#stack)。                                                                                                                                                |
| `with_embedded_tor` (需要 CGO) | 启用 嵌入式 Tor 支持，参阅 [Tor 出站](./configuration/outbound/tor)。                                                                                                                                                         |
| `with_lwip` (需要 CGO)         | 启用 LWIP Tun 栈支持，参阅 [Tun 入站](./configuration/inbound/tun#stack)。                                                                                                                                                  |

二进制文件将被构建在 `$GOPATH/bin` 下。

```bash
sing-box version
```

同时推荐使用 Systemd 来管理 sing-box 服务器实例。
参阅 [Linux 服务器安装示例](./examples/linux-server-installation)。

## 贡献者

[![](https://opencollective.com/sagernet/contributors.svg?width=740&button=false)](https://github.com/sagernet/sing-box/graphs/contributors)

## 授权

```
版权所有 (C) 2022 by nekohasekai <contact-sagernet@sekai.icu>

该程序是免费软件：您可以重新分发和 / 或修改根据 GNU 通用公共许可证的条款，由自由软件基金会，许可证的第 3 版，或（由您选择）任何更高版本。

分发这个程序是希望它有用，但没有任何保证； 甚至没有暗示的保证适销性或特定用途的适用性。 见 GNU 通用公共许可证以获取更多详细信息。

您应该已经收到一份 GNU 通用公共许可证的副本连同这个程序。 如果没有，请参阅 <http://www.gnu.org/licenses/>。
```
