---
icon: material/new-box
---

!!! quote "sing-box 1.12.0 中的更改"

    :material-plus: [domain_resolver](#domain_resolver)  
    :material-delete-clock: [domain_strategy](#domain_strategy)  
    :material-plus: [netns](#netns)

!!! quote "sing-box 1.11.0 中的更改"

    :material-plus: [network_strategy](#network_strategy)  
    :material-alert: [fallback_delay](#fallback_delay)  
    :material-alert: [network_type](#network_type)  
    :material-alert: [fallback_network_type](#fallback_network_type)

### 结构

```json
{
  "detour": "",
  "bind_interface": "",
  "inet4_bind_address": "",
  "inet6_bind_address": "",
  "routing_mark": 0,
  "reuse_addr": false,
  "netns": "",
  "connect_timeout": "",
  "tcp_fast_open": false,
  "tcp_multi_path": false,
  "udp_fragment": false,
  "domain_resolver": "", // 或 {}
  "network_strategy": "",
  "network_type": [],
  "fallback_network_type": [],
  "fallback_delay": "",
  
  // 废弃的

  "domain_strategy": ""
}
```

!!! note ""

    当内容只有一项时，可以忽略 JSON 数组 [] 标签

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

支持数字 (如 `1234`) 和十六进制字符串 (如 `"0x1234"`)。

#### reuse_addr

重用监听地址。

#### netns

!!! question "自 sing-box 1.12.0 起"

!!! quote ""

    仅支持 Linux。

设置网络命名空间，名称或路径。

#### connect_timeout

连接超时，采用 golang 的 Duration 格式。

持续时间字符串是一个可能有符号的序列十进制数，每个都有可选的分数和单位后缀， 例如 "300ms"、"-1.5h" 或 "2h45m"。
有效时间单位为 "ns"、"us"（或 "µs"）、"ms"、"s"、"m"、"h"。

#### tcp_fast_open

启用 TCP Fast Open。

#### tcp_multi_path

!!! warning ""

    需要 Go 1.21。

启用 TCP Multi Path。

#### udp_fragment

启用 UDP 分段。

#### domain_resolver

!!! warning ""

    `outbound` DNS 规则项已弃用，且将在 sing-box 1.14.0 中被移除。因此，从 sing-box 1.14.0 版本开始，所有在服务器地址中使用域名的出站/端点均需配置此项。

!!! info ""

    当只有一个 DNS 服务器已配置时，`domain_resolver` 或 `route.default_domain_resolver` 是可选的。 

用于设置解析域名的域名解析器。

此选项的格式与 [路由 DNS 规则动作](/configuration/dns/rule_action/#route) 相同，但不包含 `action` 字段。  

若直接将此选项设置为字符串，则等同于设置该选项的 `server` 字段。

| 出站/端点       | 受影响的域名                |
|----------------|---------------------------|
| `direct`       | 请求中的域名                | 
| 其他类型        | 服务器地址中的域名           |

#### network_strategy

!!! question "自 sing-box 1.11.0 起"

!!! quote ""

    仅在 Android 与 iOS 平台图形客户端中支持，并且需要 `route.auto_detect_interface`。

用于选择网络接口的策略。

可用值：

- `default`（默认值）：按顺序连接默认网络或 `network_type` 中指定的网络。
- `hybrid`：同时连接所有网络或 `network_type` 中指定的网络。
- `fallback`：同时连接默认网络或 `network_type` 中指定的首选网络，当不可用或超时时尝试回退网络。

对于回退模式，当首选接口失败或超时时，
将进入15秒的快速回退状态（同时连接所有首选和回退网络），
如果首选网络恢复，则立即退出。

与 `bind_interface`, `bind_inet4_address` 和 `bind_inet6_address` 冲突。

#### network_type

!!! question "自 sing-box 1.11.0 起"

!!! quote ""

    仅在 Android 与 iOS 平台图形客户端中支持，并且需要 `route.auto_detect_interface`。

当使用 `default` 或 `hybrid` 网络策略时要使用的网络类型，或当使用 `fallback` 网络策略时要使用的首选网络类型。

可用值：`wifi`, `cellular`, `ethernet`, `other`。

默认使用设备默认网络。

#### fallback_network_type

!!! question "自 sing-box 1.11.0 起"

!!! quote ""

    仅在 Android 与 iOS 平台图形客户端中支持，并且需要 `route.auto_detect_interface`。

当使用 `fallback` 网络策略时，在首选网络不可用或超时的情况下要使用的回退网络类型。

默认使用除首选网络外的所有其他网络。

#### fallback_delay

在生成 RFC 6555 快速回退连接之前等待的时间长度。

对于 `domain_strategy`，是在假设之前等待 IPv6 成功的时间量如果设置了 "prefer_ipv4"，则 IPv6 配置错误并回退到 IPv4。

对于 `network_strategy`，对于 `network_strategy`，是在回退到其他接口之前等待连接成功的时间。

仅当 `domain_strategy` 或 `network_strategy` 已设置时生效。

默认使用 `300ms`。

#### domain_strategy

!!! failure "已在 sing-box 1.12.0 废弃"

    `domain_strategy` 已废弃且将在 sing-box 1.14.0 中被移除，参阅 [迁移指南](/migration/#migrate-outbound-domain-strategy-option-to-domain-resolver)。

可选值：`prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`。

如果设置，域名将在请求发出之前解析为 IP。

| 出站       | 受影响的域名    | 默认回退值                     |
|----------|-----------|---------------------------|
| `direct` | 请求中的域名    | `inbound.domain_strategy` | 
| others   | 服务器地址中的域名 | /                         |