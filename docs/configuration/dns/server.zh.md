### 结构

```json
{
  "dns": {
    "servers": [
      {
        "tag": "google",
        "address": "tls://dns.google",
        "address_resolver": "local",
        "address_strategy": "prefer_ipv4",
        "strategy": "ipv4_only",
        "detour": "direct"
      }
    ]
  }
}

```

### 字段

#### tag

DNS 服务器的标签。

#### address

==必填==

DNS 服务器的地址。

| 协议                 | 格式                           |
|--------------------|------------------------------|
| `System`           | `local`                      |
| `TCP`              | `tcp://1.0.0.1`              |
| `UDP`              | `8.8.8.8` `udp://8.8.4.4`    |
| `TLS`              | `tls://dns.google`           |
| `HTTPS`            | `https://1.1.1.1/dns-query`  |
| `QUIC`             | `quic://dns.adguard.com`     |
| `HTTP3`            | `h3://8.8.8.8/dns-query`     |
| `RCode`            | `rcode://refused`            |
| `DHCP`             | `dhcp://auto` 或 `dhcp://en0` |
| [FakeIP](./fakeip) | `fakeip`                     |

!!! warning ""

    为了确保系统 DNS 生效，而不是 Go 的内置默认解析器，请在编译时启用 CGO。

!!! warning ""

    默认安装不包含 QUIC 和 HTTP3 传输层，请参阅 [安装](/zh/#_2)。

!!! info ""

    RCode 传输层传输层常用于屏蔽请求. 与 DNS 规则和 `disable_cache` 规则选项一起使用。

!!! warning ""

    默认安装不包含 DHCP 传输层，请参阅 [安装](/zh/#_2)。

| RCode             | 描述       | 
|-------------------|----------|
| `success`         | `无错误`    |
| `format_error`    | `请求格式错误` |
| `server_failure`  | `服务器出错`  |
| `name_error`      | `域名不存在`  |
| `not_implemented` | `功能未实现`  |
| `refused`         | `请求被拒绝`  |

#### address_resolver

==如果服务器地址包括域名则必须==

用于解析本 DNS 服务器的域名的另一个 DNS 服务器的标签。

#### address_strategy

用于解析本 DNS 服务器的域名的策略。

可选项：`prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`。

默认使用 `dns.strategy`。

#### strategy

默认解析策略。

可选项：`prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`。

如果被其他设置覆盖则不生效。

#### detour

用于连接到 DNS 服务器的出站的标签。

如果为空，将使用默认出站。
