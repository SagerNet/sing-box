### 结构

```json
{
  "outbounds": [
    {
      "type": "hysteria",
      "tag": "hysteria-out",
      
      "server": "127.0.0.1",
      "server_port": 1080,

      "up": "100 Mbps",
      "up_mbps": 100,
      "down": "100 Mbps",
      "down_mbps": 100,
      "obfs": "fuck me till the daylight",
      "auth": "",
      "auth_str": "password",
      "recv_window_conn": 0,
      "recv_window": 0,
      "disable_mtu_discovery": false,
      "network": "tcp",
      "tls": {},
      
      "detour": "upstream-out",
      "bind_interface": "en0",
      "bind_address": "0.0.0.0",
      "routing_mark": 1234,
      "reuse_addr": false,
      "connect_timeout": "5s",
      "domain_strategy": "prefer_ipv6",
      "fallback_delay": "300ms"
    }
  ]
}
```

!!! warning ""

    默认安装不包含被 Hysteria 依赖的 QUIC, 参阅 [安装](/zh/#installation).

### Hysteria 字段

#### server

==必填==

服务器地址

#### server_port

==必填==

服务器端口

#### up, down

==必填==

格式: `[Integer] [Unit]` e.g. `100 Mbps, 640 KBps, 2 Gbps`

支持的单位 (大小写敏感, b = bits, B = bytes, 8b=1B):

    bps (bits per second)
    Bps (bytes per second)
    Kbps (kilobits per second)
    KBps (kilobytes per second)
    Mbps (megabits per second)
    MBps (megabytes per second)
    Gbps (gigabits per second)
    GBps (gigabytes per second)
    Tbps (terabits per second)
    TBps (terabytes per second)

#### up_mbps, down_mbps

==必填==

以 Mbps 为单位的 `up, down`.

#### obfs

混淆密码

#### auth

base64 编码的认证密码

#### auth_str

认证密码

#### recv_window_conn

用于接收数据的 QUIC 流级流控制窗口。

如果为空，将使用 `15728640 (15 MB/s)`。

#### recv_window

用于接收数据的 QUIC 连接级流控制窗口。

如果为空，将使用 `67108864 (64 MB/s)`。

#### disable_mtu_discovery

禁用路径 MTU 发现 (RFC 8899)。 数据包的大小最多为 1252 (IPv4) / 1232 (IPv6) 字节。

强制为 Linux 和 Windows 以外的系统启用（根据上游）。

==必填==

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#outbound).

#### network

启用的网络协议

`tcp` 或 `udp`。

默认所有。

### 拨号字段

#### detour

上游出站的标签。

启用时，其他拨号字段将被忽略。

#### bind_interface

要绑定到的网络接口。

#### bind_address

要绑定的地址。

#### routing_mark

!!! error ""

    仅支持 Linux.

设置 netfilter 路由标记

#### reuse_addr

重用监听地址

#### connect_timeout

连接超时，采用 golang 的 Duration 格式。

持续时间字符串是一个可能有符号的序列十进制数，每个都有可选的分数和单位后缀， 例如 "300ms"、"-1.5h" 或 "2h45m"。
有效时间单位为 "ns"、"us"（或 "µs"）、"ms"、"s"、"m"、"h"。

#### domain_strategy

可选值：`prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`.

如果设置，服务器域名将在连接前解析为 IP。

如果为空，将使用 `dns.strategy`。

#### fallback_delay

在生成 RFC 6555 快速回退连接之前等待的时间长度。
也就是说，是在假设之前等待 IPv6 成功的时间量如果设置了 "prefer_ipv4"，则 IPv6 配置错误并回退到 IPv4。
如果为零，则使用 300 毫秒的默认延迟。

仅当 `domain_strategy` 为 `prefer_ipv4` 或 `prefer_ipv6` 时生效。
