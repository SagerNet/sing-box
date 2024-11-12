---
icon: material/new-box
---

!!! quote "sing-box 1.11.0 中的更改"

    :material-plus: [network_strategy](#network_strategy)  
    :material-alert: [fallback_delay](#fallback_delay)

### 结构

```json
{
  "detour": "upstream-out",
  "bind_interface": "en0",
  "inet4_bind_address": "0.0.0.0",
  "inet6_bind_address": "::",
  "routing_mark": 1234,
  "reuse_addr": false,
  "connect_timeout": "5s",
  "tcp_fast_open": false,
  "tcp_multi_path": false,
  "udp_fragment": false,
  "domain_strategy": "prefer_ipv6",
  "network_strategy": "",
  "fallback_delay": "300ms"
}
```

### 字段

#### detour

上游出站的标签。

启用时，其他拨号字段将被忽略。

#### bind_interface

要绑定到的网络接口。

#### inet4_bind_address

要绑定的 IPv4 地址。

#### inet6_bind_address

要绑定的 IPv6 地址。

#### routing_mark

!!! quote ""

    仅支持 Linux。

设置 netfilter 路由标记。

#### reuse_addr

重用监听地址。

#### tcp_fast_open

启用 TCP Fast Open。

#### tcp_multi_path

!!! warning ""

    需要 Go 1.21。

启用 TCP Multi Path。

#### udp_fragment

启用 UDP 分段。

#### connect_timeout

连接超时，采用 golang 的 Duration 格式。

持续时间字符串是一个可能有符号的序列十进制数，每个都有可选的分数和单位后缀， 例如 "300ms"、"-1.5h" 或 "2h45m"。
有效时间单位为 "ns"、"us"（或 "µs"）、"ms"、"s"、"m"、"h"。

#### domain_strategy

可选值：`prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`。

如果设置，域名将在请求发出之前解析为 IP。

| 出站       | 受影响的域名    | 默认回退值                     |
|----------|-----------|---------------------------|
| `direct` | 请求中的域名    | `inbound.domain_strategy` | 
| others   | 服务器地址中的域名 | /                         |

#### network_strategy

!!! question "自 sing-box 1.11.0 起"

!!! quote ""

    仅在 Android 与 iOS 平台图形客户端中支持。

用于选择网络接口的策略。

可用值：

- `default` (默认): 连接到默认接口，
- `fallback`: 如果超时，尝试所有剩余接口。
- `hybrid`: 同时尝试所有接口，选择最快的一个。
- `wifi`:  优先使用 WIFI，但在不可用或超时时尝试所有其他接口。
- `cellular`: 优先使用蜂窝数据，但在不可用或超时时尝试所有其他接口。
- `ethernet`: 优先使用以太网，但在不可用或超时时尝试所有其他接口。
- `wifi_only`: 仅连接到 WIFI。
- `cellular_only`: 仅连接到蜂窝数据。
- `ethernet_only`: 仅连接到以太网。

对于回退策略, 当优先使用的接口发生故障或超时时， 将进入 15 秒的快速回退状态（升级为 `hybrid`）， 且恢复后立即退出。

与 `bind_interface`, `bind_inet4_address` 和 `bind_inet6_address` 冲突。

#### fallback_delay

在生成 RFC 6555 快速回退连接之前等待的时间长度。

对于 `domain_strategy`，是在假设之前等待 IPv6 成功的时间量如果设置了 "prefer_ipv4"，则 IPv6 配置错误并回退到 IPv4。

对于 `network_strategy`，对于 `network_strategy`，是在回退到其他接口之前等待连接成功的时间。

仅当 `domain_strategy` 或 `network_strategy` 已设置时生效。

默认使用 `300ms`。
