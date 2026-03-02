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
| `with_tailscale`                   | :material-check:  | 构建 Tailscale 支持，参阅 [Tailscale 端点](/configuration/endpoint/tailscale)。                                                                                                                                                                                                                                                         |
| `with_ccm`                         | :material-check:  | 构建 Claude Code Multiplexer 服务支持。                                                                                                                                                                                                                                                                                              |
| `with_ocm`                         | :material-check:  | 构建 OpenAI Codex Multiplexer 服务支持。                                                                                                                                                                                                                                                                                             |
| `with_naive_outbound`              | :material-check:  | 构建 NaiveProxy 出站支持，参阅 [NaiveProxy 出站](/configuration/outbound/naive/)。                                                                                                                                                                                                                                                         |
| `badlinkname`                      | :material-check:  | 启用 `go:linkname` 以访问标准库内部函数。Go 标准库未提供本项目需要的许多底层 API，且在外部重新实现不切实际。用于 kTLS（内核 TLS 卸载）和原始 TLS 记录操作。                                                                                                                                                                                                                           |
| `tfogo_checklinkname0`             | :material-check:  | `badlinkname` 的伴随标记。Go 1.23+ 链接器强制限制 `go:linkname` 使用；此标记表示构建使用 `-checklinkname=0` 以绕过该限制。                                                                                                                                                                                                                                |

除非您确实知道您正在启用什么，否则不建议更改默认构建标签列表。

## :material-wrench: 链接器标志

以下 `-ldflags` 在官方构建中使用：

| 标志                                                          | 说明                                                                                                                                                         |
|-------------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `-X 'internal/godebug.defaultGODEBUG=multipathtcp=0'`      | Go 1.24 默认为监听器启用 Multipath TCP（`multipathtcp=2`）。这可能在底层 socket 上导致错误，且 sing-box 有自己的 MPTCP 控制（`tcp_multi_path` 选项）。此标志禁用 Go 的默认行为。                             |
| `-checklinkname=0`                                          | Go 1.23+ 链接器拒绝未授权的 `go:linkname` 使用。此标志禁用该检查，需要与 `badlinkname` 构建标记一起使用。                                                                                   |

## :material-package-variant: 下游打包者

默认构建标签列表和链接器标志以文件形式存放在仓库中，供下游打包者直接引用：

| 文件 | 说明 |
|------|------|
| `release/DEFAULT_BUILD_TAGS` | Linux（常见架构）、Darwin 和 Android 的默认标签。 |
| `release/DEFAULT_BUILD_TAGS_WINDOWS` | Windows 的默认标签（包含 `with_purego`）。 |
| `release/DEFAULT_BUILD_TAGS_OTHERS` | 其他平台的默认标签（不含 `with_naive_outbound`）。 |
| `release/LDFLAGS` | 必需的链接器标志（参见上文）。 |

## :material-layers: with_naive_outbound

NaiveProxy 出站需要根据目标平台进行特殊的构建配置。

### 支持的平台

| 平台            | 架构                     | 模式     | 要求                             |
|---------------|------------------------|--------|--------------------------------|
| Linux         | amd64, arm64           | purego | 无（官方发布版本已包含库文件）                |
| Linux         | 386, amd64, arm, arm64 | CGO    | Chromium 工具链，运行时需要 glibc >= 2.31 |
| Linux (musl)  | 386, amd64, arm, arm64 | CGO    | Chromium 工具链                   |
| Windows       | amd64, arm64           | purego | 无（官方发布版本已包含库文件）                |
| Apple 平台      | *                      | CGO    | Xcode                          |
| Android       | *                      | CGO    | Android NDK                    |

### Windows

使用 `with_purego` 标记。

官方发布版本已包含 `libcronet.dll`。自行构建时，从 [cronet-go releases](https://github.com/sagernet/cronet-go/releases) 下载并放置在 `sing-box.exe` 相同目录或 `PATH` 中的任意目录。

### Linux (purego, 仅 amd64/arm64)

使用 `with_purego` 标记。

官方发布版本已包含 `libcronet.so`。自行构建时，从 [cronet-go releases](https://github.com/sagernet/cronet-go/releases) 下载并放置在 sing-box 二进制文件相同目录或系统库路径中。

### Linux (CGO)

参阅 [cronet-go](https://github.com/sagernet/cronet-go#linux-build-instructions)。

- **glibc 构建**：运行时需要 glibc >= 2.31
- **musl 构建**：使用 `with_musl` 标记，静态链接，无运行时要求

### Apple 平台 / Android

参阅 [cronet-go](https://github.com/sagernet/cronet-go)。
