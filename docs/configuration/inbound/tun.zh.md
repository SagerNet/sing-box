---
icon: material/alert-decagram
---

!!! quote "sing-box 1.8.0 中的更改"

    :material-plus: [gso](#gso)  
    :material-plus: [gso_max_size](#gso_max_size)  
    :material-alert-decagram: [stack](#stack)

!!! quote ""

    仅支持 Linux、Windows 和 macOS。

### 结构

```json
{
  "type": "tun",
  "tag": "tun-in",
  "interface_name": "tun0",
  "inet4_address": "172.19.0.1/30",
  "inet6_address": "fdfe:dcba:9876::1/126",
  "mtu": 9000,
  "gso": false,
  "gso_max_size": 65536,
  "auto_route": true,
  "strict_route": true,
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
  "endpoint_independent_nat": false,
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
      "server_port": 8080
    }
  },
  
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

#### inet4_address

==必填==

tun 接口的 IPv4 前缀。

#### inet6_address

tun 接口的 IPv6 前缀。

#### mtu

最大传输单元。

#### gso

!!! question "自 sing-box 1.8.0 起"

!!! quote ""

    仅支持 Linux。

启用通用分段卸载。

#### gso_max_size

!!! question "自 sing-box 1.8.0 起"

!!! quote ""

    仅支持 Linux。

通用分段卸载包的最大大小。

默认使用 `65536`。

#### auto_route

设置到 Tun 的默认路由。

!!! quote ""

    为避免流量环回，请设置 `route.auto_detect_interface` 或 `route.default_interface` 或 `outbound.bind_interface`。

!!! note "与 Android VPN 一起使用"

    VPN 默认优先于 tun。要使 tun 经过 VPN，启用 `route.override_android_vpn`。

#### strict_route

启用 `auto_route` 时执行严格的路由规则。

*在 Linux 中*:

* 让不支持的网络无法到达
* 将所有连接路由到 tun

它可以防止地址泄漏，并使 DNS 劫持在 Android 上工作，但你的设备将无法其他设备被访问。

*在 Windows 中*:

* 添加防火墙规则以阻止 Windows
  的 [普通多宿主 DNS 解析行为](https://learn.microsoft.com/en-us/previous-versions/windows/it-pro/windows-server-2008-R2-and-2008/dd197552%28v%3Dws.10%29)
  造成的 DNS 泄露

它可能会使某些应用程序（如 VirtualBox）在某些情况下无法正常工作。

#### inet4_route_address

启用 `auto_route` 时使用自定义路由而不是默认路由。

#### inet6_route_address

启用 `auto_route` 时使用自定义路由而不是默认路由。

#### inet4_route_exclude_address

启用 `auto_route` 时排除自定义路由。

#### inet6_route_exclude_address

启用 `auto_route` 时排除自定义路由。

#### endpoint_independent_nat

启用独立于端点的 NAT。

性能可能会略有下降，所以不建议在不需要的时候开启。

#### udp_timeout

UDP NAT 过期时间，以秒为单位，默认为 300（5 分钟）。

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

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。
