---
icon: material/new-box
---

!!! question "自 sing-box 1.13.0 起"

### 结构

```json
{
  "type": "naive",
  "tag": "naive-out",

  "server": "127.0.0.1",
  "server_port": 443,
  "username": "sekai",
  "password": "password",
  "insecure_concurrency": 0,
  "extra_headers": {},
  "udp_over_tcp": false | {},
  "quic": false,
  "quic_congestion_control": "",
  "tls": {},

  ... // 拨号字段
}
```

!!! warning "平台支持"

    NaiveProxy 出站仅在 Apple 平台、Android、Windows 和特定 Linux 构建上可用。

    **官方发布版本区别：**

    | 构建变体      | 平台                     | 说明                                       |
    |-----------|------------------------|------------------------------------------|
    | (默认)      | Linux amd64/arm64      | purego 构建，包含 `libcronet.so`              |
    | `-glibc`  | Linux 386/amd64/arm/arm64 | CGO 构建，动态链接 glibc，要求 glibc >= 2.31       |
    | `-musl`   | Linux 386/amd64/arm/arm64 | CGO 构建，静态链接 musl，无系统要求                   |
    | (默认)      | Windows amd64/arm64 | purego 构建，包含 `libcronet.dll`             |

    **运行时要求：**

    - **Linux purego**：`libcronet.so` 必须位于 sing-box 二进制文件相同目录或系统库路径中
    - **Windows**：`libcronet.dll` 必须位于 `sing-box.exe` 相同目录或 `PATH` 中的任意目录

    自行构建请参阅 [从源代码构建](/zh/installation/build-from-source/#with_naive_outbound)。

### 字段

#### server

==必填==

服务器地址。

#### server_port

==必填==

服务器端口。

#### username

认证用户名。

#### password

认证密码。

#### insecure_concurrency

并发隧道连接数。多连接使隧道更容易被流量分析检测，违背 NaiveProxy 抵抗流量分析的设计目的。

#### extra_headers

HTTP 请求中发送的额外头部。

#### udp_over_tcp

UDP over TCP 配置。

参阅 [UDP Over TCP](/zh/configuration/shared/udp-over-tcp/)。

#### quic

使用 QUIC 代替 HTTP/2。

#### quic_congestion_control

QUIC 拥塞控制算法。

| 算法 | 描述 |
|------|------|
| `bbr` | BBR |
| `bbr2` | BBRv2 |
| `cubic` | CUBIC |
| `reno` | New Reno |

默认使用 `bbr`（NaiveProxy 基于的 Chromium 使用的 QUICHE 的默认值）。

#### tls

==必填==

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#outbound)。

只有 `server_name`、`certificate`、`certificate_path`、`certificate_public_key_sha256` 和 `ech` 是被支持的。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
