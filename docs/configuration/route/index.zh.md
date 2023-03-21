# 路由

### 结构

```json
{
  "route": {
    "geoip": {},
    "geosite": {},
    "ip_rules": [],
    "rules": [],
    "final": "",
    "auto_detect_interface": false,
    "override_android_vpn": false,
    "default_interface": "en0",
    "default_mark": 233
  }
}
```

### 字段

| 键          | 格式                      |
|------------|-------------------------|
| `geoip`    | [GeoIP](./geoip)        |
| `geosite`  | [GeoSite](./geosite)    |
| `ip_rules` | 一组 [IP 路由规则](./ip-rule) |
| `rules`    | 一组 [路由规则](./rule)       |

#### final

默认出站标签。如果未空，将使用第一个可用于对应协议的出站。

#### auto_detect_interface

!!! error ""

    仅支持 Linux、Windows 和 macOS。

默认将出站连接绑定到默认网卡，以防止在 tun 下出现路由环路。

如果设置了 `outbound.bind_interface` 设置，则不生效。

#### override_android_vpn

!!! error ""

    仅支持 Android。

启用 `auto_detect_interface` 时接受 Android VPN 作为上游网卡。

#### default_interface

!!! error ""

    仅支持 Linux、Windows 和 macOS。

默认将出站连接绑定到指定网卡，以防止在 tun 下出现路由环路。

如果设置了 `auto_detect_interface` 设置，则不生效。

#### default_mark

!!! error ""

    仅支持 Linux。

默认为出站连接设置路由标记。

如果设置了 `outbound.routing_mark` 设置，则不生效。