---
description: 欢迎来到该 sing-box 项目的文档页。
---

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

| 构建标志                         | 描述                                                                                                                                                                                                                                                                           |
|------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `with_quic`                  | 启用 QUIC 支持，参阅 [QUIC 和 HTTP3 DNS 传输层](./configuration/dns/server)，[Naive 入站](./configuration/inbound/naive)，[Hysteria 入站](./configuration/inbound/hysteria)，[Hysteria 出站](./configuration/outbound/hysteria) 和 [V2Ray 传输层#QUIC](./configuration/shared/v2ray-transport#quic)。 |
| `with_grpc`                  | 启用标准 gRPC 支持，参阅 [V2Ray 传输层#gRPC](./configuration/shared/v2ray-transport#grpc)。                                                                                                                                                                                               |
| `with_wireguard`             | 启用 WireGuard 支持，参阅 [WireGuard 出站](./configuration/outbound/wireguard)。                                                                                                                                                                                                       |
| `with_acme`                  | 启用 ACME TLS 证书签发支持，参阅 [TLS](./configuration/shared/tls)。                                                                                                                                                                                                                     |
| `with_clash_api`             | 启用 Clash api 支持，参阅 [实验性](./configuration/experimental#clash-api-fields)。                                                                                                                                                                                                     |
| `no_gvisor`                  | 禁用 gVisor Tun 栈支持，参阅 [Tun 入站](./configuration/inbound/tun#stack)。                                                                                                                                                                                                            |
| `with_embedded_tor` (需要 CGO) | 启用 嵌入式 Tor 支持，参阅 [Tor 出站](./configuration/outbound/tor)。                                                                                                                                                                                                                     |
| `with_lwip` (需要 CGO)         | 启用 LWIP Tun 栈支持，参阅 [Tun 入站](./configuration/inbound/tun#stack)。                                                                                                                                                                                                              |

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
along with this program. If not, see <http://www.gnu.org/licenses/>.
```
