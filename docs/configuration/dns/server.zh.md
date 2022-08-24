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

dns 服务器的标签。

#### address

==必填==

dns 服务器的地址。

| 协议       | 格式                          |
|----------|-----------------------------|
| `System` | `local`                     |
| `TCP`    | `tcp://1.0.0.1`             |
| `UDP`    | `8.8.8.8` 或 `udp://8.8.4.4` |
| `TLS`    | `tls://dns.google`          |
| `HTTPS`  | `https://1.1.1.1/dns-query` |
| `QUIC`   | `quic://dns.adguard.com`    |
| `HTTP3`  | `h3://8.8.8.8/dns-query`    |
| `RCode`  | `rcode://refused`           |

!!! warning ""

    为了确保系统 DNS 生效，而不是 go 的内置默认解析器生效，请在编译时启用 CGO。

!!! warning ""

    默认不包含 QUIC 和 HTTP3 的传输方式， 详见 [Installation](/#installation)。

!!! info ""

    RCode 传输通常用于阻止查询，与规则和 `disable_cache` 规则选项一起使用。

| RCode             | 描述       | 
|-------------------|----------|
| `success`         | `无错误`    |
| `format_error`    | `格式错误`   |
| `server_failure`  | `服务器故障`  |
| `name_error`      | `不存在的域名` |
| `not_implemented` | `不可执行`   |

#### address_resolver

==如果地址包含域，则为必填==

使用另一个服务器的标签以解析地址中的域名。

#### address_strategy

解析地址中域名的域策略。

可选参数有：`prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`。

如果为空，将使用 `dns.strategy`。

#### strategy

解析域名的默认域策略。

可选参数有：`prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`。

如果被其他设置覆盖则无效。

#### detour

用于连接到 dns 服务器的出站标记。

如果为空，将使用默认出站。