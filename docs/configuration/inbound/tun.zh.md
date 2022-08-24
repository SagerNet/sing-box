!!! error ""

    仅支持 Linux, Windows, 和 macOS.

### 结构

```json
{
  "inbounds": [
    {
      "type": "tun",
      "tag": "tun-in",
      "interface_name": "tun0",
      "inet4_address": "172.19.0.1/30",
      "inet6_address": "fdfe:dcba:9876::1/128",
      "mtu": 1500,
      "auto_route": true,
      "endpoint_independent_nat": false,
      "udp_timeout": 300,
      "stack": "gvisor",
      "include_uid": [
        0
      ],
      "include_uid_range": [
        [
          "1000-99999"
        ]
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
      "sniff": true,
      "sniff_override_destination": false,
      "domain_strategy": "prefer_ipv4"
    }
  ]
}
```

!!! note ""

    当内容只有一项时，可以忽略 JSON 数组 [] 标签

!!! warning ""

    如果 tun 在非特权模式下运行，地址和 MTU 将不会自动配置，请确保设置正确。

### Tun 字段

#### interface_name

虚拟设备名称，如果为空则自动选择。

#### inet4_address

==必填==

tun 接口的 IPv4 前缀。

#### inet6_address

tun 接口的 IPv6 前缀。

#### mtu

最大传输单元

#### auto_route

设置到 Tun 的默认路由。

!!! error ""

    为避免流量环回，请设置 `route.auto_detect_interface` 或 `route.default_interface` 或 `outbound.bind_interface`

#### endpoint_independent_nat

启用独立于端点的 NAT。

性能可能会略有下降，所以不建议在不需要的时候开启。

#### udp_timeout

UDP NAT 过期时间，以秒为单位，默认为 300（5 分钟）。

#### stack

TCP/IP 栈.

| 栈                | 上游                                                                    | 状态    |
|------------------|-----------------------------------------------------------------------|-------|
| gVisor (default) | [google/gvisor](https://github.com/google/gvisor)                     | 推荐    |
| LWIP             | [eycorsican/go-tun2socks](https://github.com/eycorsican/go-tun2socks) | 上游已存档 |

!!! warning ""

    默认安装不包含 LWIP 栈， 请参阅 [安装](/zh/#installation)。

#### include_uid

!!! error ""

    UID 规则仅在 Linux 下被支持，并且需要 `auto_route`.

限制被路由的的用户。 默认不限制。

#### include_uid_range

限制被路由的的用户范围。

#### exclude_uid

排除路由的的用户。

#### exclude_uid_range

排除路由的的用户范围。

#### include_android_user

!!! error ""

    Android 用户和应用规则仅在 Android 下被支持，并且需要 `auto_route`.

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

#### sniff

启用协议探测。

参阅 [协议探测](/zh/configuration/route/sniff/)

#### sniff_override_destination

用探测出的域名覆盖连接目标地址。

如果域名无效（如 Tor），将不生效。

#### domain_strategy

可选值： `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`.

如果设置，请求的域名将在路由之前解析为 IP。

如果 `sniff_override_destination` 生效，它的值将作为后备。
