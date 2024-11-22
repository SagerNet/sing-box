---
icon: material/alert-decagram
---

!!! quote "sing-box 1.11.0 中的更改"

    :material-delete-alert: [gso](#gso)

!!! quote "sing-box 1.10.0 中的更改"

    :material-plus: [address](#address)  
    :material-delete-clock: [inet4_address](#inet4_address)  
    :material-delete-clock: [inet6_address](#inet6_address)  
    :material-plus: [route_address](#route_address)  
    :material-delete-clock: [inet4_route_address](#inet4_route_address)  
    :material-delete-clock: [inet6_route_address](#inet6_route_address)  
    :material-plus: [route_exclude_address](#route_address)  
    :material-delete-clock: [inet4_route_exclude_address](#inet4_route_exclude_address)  
    :material-delete-clock: [inet6_route_exclude_address](#inet6_route_exclude_address)   
    :material-plus: [iproute2_table_index](#iproute2_table_index)  
    :material-plus: [iproute2_rule_index](#iproute2_table_index)  
    :material-plus: [auto_redirect](#auto_redirect)  
    :material-plus: [auto_redirect_input_mark](#auto_redirect_input_mark)  
    :material-plus: [auto_redirect_output_mark](#auto_redirect_output_mark)  
    :material-plus: [route_address_set](#route_address_set)  
    :material-plus: [route_exclude_address_set](#route_address_set)

!!! quote "sing-box 1.9.0 中的更改"

    :material-plus: [platform.http_proxy.bypass_domain](#platformhttp_proxybypass_domain)  
    :material-plus: [platform.http_proxy.match_domain](#platformhttp_proxymatch_domain)  

!!! quote "sing-box 1.8.0 中的更改"

    :material-plus: [gso](#gso)  
    :material-alert-decagram: [stack](#stack)

!!! quote ""

    仅支持 Linux、Windows 和 macOS。

### 结构

```json
{
  "type": "tun",
  "tag": "tun-in",
  "interface_name": "tun0",
  "address": [
    "172.18.0.1/30",
    "fdfe:dcba:9876::1/126"
  ],
  "mtu": 9000,
  "auto_route": true,
  "iproute2_table_index": 2022,
  "iproute2_rule_index": 9000,
  "auto_redirect": false,
  "auto_redirect_input_mark": "0x2023",
  "auto_redirect_output_mark": "0x2024",
  "strict_route": true,
  "route_address": [
    "0.0.0.0/1",
    "128.0.0.0/1",
    "::/1",
    "8000::/1"
  ],

  "route_exclude_address": [
    "192.168.0.0/16",
    "fc00::/7"
  ],
  "route_address_set": [
    "geoip-cloudflare"
  ],
  "route_exclude_address_set": [
    "geoip-cn"
  ],
  "endpoint_independent_nat": false,
  "udp_timeout": "5m",
  "stack": "system",
  "include_interface": [
    "lan0"
  ],
  "exclude_interface": [
    "lan1"
  ],
  "include_uid": [
    0
  ],
  "include_uid_range": [
    "1000-99999"
  ],
  "exclude_uid": [
    1000
  ],
  "exclude_uid_range": [
    "1000-99999"
  ],
  "include_android_user": [
    0,
    10
  ],
  "include_package": [
    "com.android.chrome"
  ],
  "exclude_package": [
    "com.android.captiveportallogin"
  ],
  "platform": {
    "http_proxy": {
      "enabled": false,
      "server": "127.0.0.1",
      "server_port": 8080,
      "bypass_domain": [],
      "match_domain": []
    }
  },

  // 已弃用
  "gso": false,
  "inet4_address": [
    "172.19.0.1/30"
  ],
  "inet6_address": [
    "fdfe:dcba:9876::1/126"
  ],
  "inet4_route_address": [
    "0.0.0.0/1",
    "128.0.0.0/1"
  ],
  "inet6_route_address": [
    "::/1",
    "8000::/1"
  ],
  "inet4_route_exclude_address": [
    "192.168.0.0/16"
  ],
  "inet6_route_exclude_address": [
    "fc00::/7"
  ],
  
  ... // 监听字段
}
```

!!! note ""

    当内容只有一项时，可以忽略 JSON 数组 [] 标签。

!!! warning ""

    如果 tun 在非特权模式下运行，地址和 MTU 将不会自动配置，请确保设置正确。

### Tun 字段

#### interface_name

虚拟设备名称，默认自动选择。

#### address

!!! question "自 sing-box 1.10.0 起"

==必填==

tun 接口的 IPv4 和 IPv6 前缀。

#### inet4_address

!!! failure "已在 sing-box 1.10.0 废弃"

    `inet4_address` 已合并到 `address` 且将在 sing-box 1.12.0 中被移除。

==必填==

tun 接口的 IPv4 前缀。

#### inet6_address

!!! failure "已在 sing-box 1.10.0 废弃"

    `inet6_address` 已合并到 `address` 且将在 sing-box 1.12.0 中被移除。

tun 接口的 IPv6 前缀。

#### mtu

最大传输单元。

#### gso

!!! failure "已在 sing-box 1.11.0 废弃"

    GSO 对于透明代理场景没有优势，已废弃和不再生效，且将在 sing-box 1.12.0 中被移除。

!!! question "自 sing-box 1.8.0 起"

!!! quote ""

    仅支持 Linux。

启用通用分段卸载。

#### auto_route

设置到 Tun 的默认路由。

!!! quote ""

    为避免流量环回，请设置 `route.auto_detect_interface` 或 `route.default_interface` 或 `outbound.bind_interface`。

!!! note "与 Android VPN 一起使用"

    VPN 默认优先于 tun。要使 tun 经过 VPN，启用 `route.override_android_vpn`。

#### iproute2_table_index

!!! question "自 sing-box 1.10.0 起"

`auto_route` 生成的 iproute2 路由表索引。

默认使用 `2022`。

#### iproute2_rule_index

!!! question "自 sing-box 1.10.0 起"

`auto_route` 生成的 iproute2 规则起始索引。

默认使用 `9000`。

#### auto_redirect

!!! question "自 sing-box 1.10.0 起"

!!! quote ""

    仅支持 Linux，且需要 `auto_route` 已启用。 

自动配置 iptables/nftables 以重定向连接。

*在 Android 中*：

仅转发本地 IPv4 连接。 要通过热点或中继共享您的 VPN 连接，请使用 [VPNHotspot](https://github.com/Mygod/VPNHotspot)。

*在 Linux 中*:

带有 `auto_redirect `的 `auto_route` 可以在路由器上按预期工作，**无需干预**。

#### auto_redirect_input_mark

!!! question "自 sing-box 1.10.0 起"

`route_address_set` 和 `route_exclude_address_set` 使用的连接输入标记。

默认使用 `0x2023`。

#### auto_redirect_output_mark

!!! question "自 sing-box 1.10.0 起"

`route_address_set` 和 `route_exclude_address_set` 使用的连接输出标记。

默认使用 `0x2024`。

#### strict_route

启用 `auto_route` 时执行严格的路由规则。

*在 Linux 中*:

* 让不支持的网络无法到达
* 使 ICMP 流量路由到 tun 而不是上游接口
* 将所有连接路由到 tun

它可以防止 IP 地址泄漏，并使 DNS 劫持在 Android 上工作。

*在 Windows 中*:

* 添加防火墙规则以阻止 Windows
  的 [普通多宿主 DNS 解析行为](https://learn.microsoft.com/en-us/previous-versions/windows/it-pro/windows-server-2008-R2-and-2008/dd197552%28v%3Dws.10%29)
  造成的 DNS 泄露

它可能会使某些应用程序（如 VirtualBox）在某些情况下无法正常工作。

#### route_address

!!! question "自 sing-box 1.10.0 起"

设置到 Tun 的自定义路由。

#### inet4_route_address

!!! failure "已在 sing-box 1.10.0 废弃"

    `inet4_route_address` 已合并到 `route_address` 且将在 sing-box 1.12.0 中被移除。

启用 `auto_route` 时使用自定义路由而不是默认路由。

#### inet6_route_address

!!! failure "已在 sing-box 1.10.0 废弃"

    `inet6_route_address` 已合并到 `route_address` 且将在 sing-box 1.12.0 中被移除。

启用 `auto_route` 时使用自定义路由而不是默认路由。

#### route_exclude_address

!!! question "自 sing-box 1.10.0 起"

设置到 Tun 的排除自定义路由。

#### inet4_route_exclude_address

!!! failure "已在 sing-box 1.10.0 废弃"

    `inet4_route_exclude_address` 已合并到 `route_exclude_address` 且将在 sing-box 1.12.0 中被移除。

启用 `auto_route` 时排除自定义路由。

#### inet6_route_exclude_address

!!! failure "已在 sing-box 1.10.0 废弃"

    `inet6_route_exclude_address` 已合并到 `route_exclude_address` 且将在 sing-box 1.12.0 中被移除。

启用 `auto_route` 时排除自定义路由。

#### route_address_set

!!! question "自 sing-box 1.10.0 起"

!!! quote ""

    仅支持 Linux，且需要 nftables，`auto_route` 和 `auto_redirect` 已启用。 

将指定规则集中的目标 IP CIDR 规则添加到防火墙。
不匹配的流量将绕过 sing-box 路由。

与 `route.default_mark` 和 `[dialOptions].routing_mark` 冲突。

#### route_exclude_address_set

!!! question "自 sing-box 1.10.0 起"

!!! quote ""

    仅支持 Linux，且需要 nftables，`auto_route` 和 `auto_redirect` 已启用。

将指定规则集中的目标 IP CIDR 规则添加到防火墙。
匹配的流量将绕过 sing-box 路由。

与 `route.default_mark` 和 `[dialOptions].routing_mark` 冲突。

#### endpoint_independent_nat

启用独立于端点的 NAT。

性能可能会略有下降，所以不建议在不需要的时候开启。

#### udp_timeout

UDP NAT 过期时间。

默认使用 `5m`。

#### stack

!!! quote "sing-box 1.8.0 中的更改"

    :material-delete-alert: 旧的 LWIP 栈已被弃用并移除。

TCP/IP 栈。

| 栈      | 描述                                                               |
|--------|------------------------------------------------------------------|
| system | 基于系统网络栈执行 L3 到 L4 转换                                             |
| gVisor | 基于 [gVisor](https://github.com/google/gvisor) 虚拟网络栈执行 L3 到 L4 转换 |
| mixed  | 混合 `system` TCP 栈与 `gvisor` UDP 栈                                |

默认使用 `mixed` 栈如果 gVisor 构建标记已启用，否则默认使用 `system` 栈。

#### include_interface

!!! quote ""

    接口规则仅在 Linux 下被支持，并且需要 `auto_route`。

限制被路由的接口。默认不限制。

与 `exclude_interface` 冲突。

#### exclude_interface

!!! warning ""

    当 `strict_route` 启用，到被排除接口的回程流量将不会被自动排除，因此也要添加它们（例：`br-lan` 与 `pppoe-wan`）。

排除路由的接口。

与 `include_interface` 冲突。

#### include_uid

!!! quote ""

    UID 规则仅在 Linux 下被支持，并且需要 `auto_route`。

限制被路由的用户。默认不限制。

#### include_uid_range

限制被路由的用户范围。

#### exclude_uid

排除路由的用户。

#### exclude_uid_range

排除路由的用户范围。

#### include_android_user

!!! quote ""

    Android 用户和应用规则仅在 Android 下被支持，并且需要 `auto_route`。

限制被路由的 Android 用户。

| 常用用户 | ID |
|------|----|
| 您    | 0  |
| 工作资料 | 10 |

#### include_package

限制被路由的 Android 应用包名。

#### exclude_package

排除路由的 Android 应用包名。

#### platform

平台特定的设置，由客户端应用提供。

#### platform.http_proxy

系统 HTTP 代理设置。

##### platform.http_proxy.enabled

启用系统 HTTP 代理。

##### platform.http_proxy.server

==必填==

系统 HTTP 代理服务器地址。

##### platform.http_proxy.server_port

==必填==

系统 HTTP 代理服务器端口。

##### platform.http_proxy.bypass_domain

!!! note ""

    在 Apple 平台，`bypass_domain` 项匹配主机名 **后缀**.

绕过代理的主机名列表。

##### platform.http_proxy.match_domain

!!! quote ""

    仅在 Apple 平台图形客户端中支持。

代理的主机名列表。

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。
