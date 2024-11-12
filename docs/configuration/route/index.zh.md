# 路由

!!! quote "sing-box 1.8.0 中的更改"

    :material-plus: [rule_set](#rule_set)  
    :material-delete-clock: [geoip](#geoip)  
    :material-delete-clock: [geosite](#geosite)

### 结构

```json
{
  "route": {
    "geoip": {},
    "geosite": {},
    "rules": [],
    "rule_set": [],
    "final": "",
    "auto_detect_interface": false,
    "override_android_vpn": false,
    "default_interface": "",
    "default_mark": 0,
    "default_network_strategy": "",
    "default_fallback_delay": ""
  }
}
```

### 字段

| 键         | 格式                    |
|-----------|-----------------------|
| `geoip`   | [GeoIP](./geoip/)     |
| `geosite` | [Geosite](./geosite/) |

#### rule

一组 [路由规则](./rule/)    。

#### rule_set

!!! question "自 sing-box 1.8.0 起"

一组 [规则集](/configuration/rule-set/)。

#### final

默认出站标签。如果为空，将使用第一个可用于对应协议的出站。

#### auto_detect_interface

!!! quote ""

    仅支持 Linux、Windows 和 macOS。

默认将出站连接绑定到默认网卡，以防止在 tun 下出现路由环路。

如果设置了 `outbound.bind_interface` 设置，则不生效。

#### override_android_vpn

!!! quote ""

    仅支持 Android。

启用 `auto_detect_interface` 时接受 Android VPN 作为上游网卡。

#### default_interface

!!! quote ""

    仅支持 Linux、Windows 和 macOS。

默认将出站连接绑定到指定网卡，以防止在 tun 下出现路由环路。

如果设置了 `auto_detect_interface` 设置，则不生效。

#### default_mark

!!! quote ""

    仅支持 Linux。

默认为出站连接设置路由标记。

如果设置了 `outbound.routing_mark` 设置，则不生效。

#### network_strategy

!!! quote ""

    仅在 Android 与 Apple 平台图形客户端中支持，并且需要 `auto_detect_interface`。

选择网络接口的策略。

当 `outbound.bind_interface`, `outbound.inet4_bind_address` 或 `outbound.inet6_bind_address` 已设置时不生效。

可以被 `outbound.network_strategy` 覆盖。

与 `default_interface` 冲突。

可用值请参阅 [拨号字段](/configuration/shared/dial/#network_strategy)。

#### fallback_delay

!!! quote ""

    仅在 Android 与 Apple 平台图形客户端中支持，并且需要 `auto_detect_interface` 且 `network_strategy` 已设置。

详情请参阅 [拨号字段](/configuration/shared/dial/#fallback_delay)。
