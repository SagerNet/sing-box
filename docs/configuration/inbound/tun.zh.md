!!! error ""

    仅支持 Linux、Windows 和 macOS。

### 结构

```json
{
  "type": "tun",
  "tag": "tun-in",

  "interface_name": "tun0",
  "inet4_address": "172.19.0.1/30",
  "inet6_address": "fdfe:dcba:9876::1/128",
  "mtu": 1500,
  "auto_route": true,
  "strict_route": true,
  "endpoint_independent_nat": false,
  "stack": "gvisor",
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

#### auto_route

设置到 Tun 的默认路由。

!!! error ""

    为避免流量环回，请设置 `route.auto_detect_interface` 或 `route.default_interface` 或 `outbound.bind_interface`。

#### strict_route

在 Linux 中启用 `auto_route` 时执行严格的路由规则。

* 让不支持的网络无法到达
* 将所有连接路由到 tun

它可以防止地址泄漏，并使 DNS 劫持在 Android 和使用 systemd-resolved 的 Linux 上工作，但你的设备将无法其他设备被访问。

#### endpoint_independent_nat

启用独立于端点的 NAT。

性能可能会略有下降，所以不建议在不需要的时候开启。

#### udp_timeout

UDP NAT 过期时间，以秒为单位，默认为 300（5 分钟）。

#### stack

TCP/IP 栈。

| 栈                | 上游                                                                    | 状态    |
|------------------|-----------------------------------------------------------------------|-------|
| gVisor (default) | [google/gvisor](https://github.com/google/gvisor)                     | 推荐    |
| LWIP             | [eycorsican/go-tun2socks](https://github.com/eycorsican/go-tun2socks) | 上游已存档 |

!!! warning ""

    默认安装不包含 LWIP 栈，请参阅 [安装](/zh/#_2)。

#### include_uid

!!! error ""

    UID 规则仅在 Linux 下被支持，并且需要 `auto_route`。

限制被路由的的用户。默认不限制。

#### include_uid_range

限制被路由的的用户范围。

#### exclude_uid

排除路由的的用户。

#### exclude_uid_range

排除路由的的用户范围。

#### include_android_user

!!! error ""

    Android 用户和应用规则仅在 Android 下被支持，并且需要 `auto_route`。

限制被路由的 Android 用户。

| 常用用户 | ID  |
|--|-----|
| 您 | 0   |
| 工作资料 | 10  |

#### include_package

限制被路由的 Android 应用包名。

#### exclude_package

排除路由的 Android 应用包名。

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。