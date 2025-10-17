---
icon: material/new-box
---

!!! quote "sing-box 1.13.0 中的更改"

    :material-plus: [interface_address](#interface_address)  
    :material-plus: [network_interface_address](#network_interface_address)  
    :material-plus: [default_interface_address](#default_interface_address)  
    :material-plus: [preferred_by](#preferred_by)  
    :material-alert: [network](#network)

!!! quote "sing-box 1.11.0 中的更改"

    :material-plus: [action](#action)  
    :material-alert: [outbound](#outbound)  
    :material-plus: [network_type](#network_type)  
    :material-plus: [network_is_expensive](#network_is_expensive)  
    :material-plus: [network_is_constrained](#network_is_constrained)

!!! quote "sing-box 1.10.0 中的更改"

    :material-plus: [client](#client)  
    :material-delete-clock: [rule_set_ipcidr_match_source](#rule_set_ipcidr_match_source)  
    :material-plus: [process_path_regex](#process_path_regex)

!!! quote "sing-box 1.8.0 中的更改"

    :material-plus: [rule_set](#rule_set)  
    :material-plus: [rule_set_ipcidr_match_source](#rule_set_ipcidr_match_source)  
    :material-plus: [source_ip_is_private](#source_ip_is_private)  
    :material-plus: [ip_is_private](#ip_is_private)  
    :material-delete-clock: [source_geoip](#source_geoip)  
    :material-delete-clock: [geoip](#geoip)  
    :material-delete-clock: [geosite](#geosite)

### 结构

```json
{
  "route": {
    "rules": [
      {
        "inbound": [
          "mixed-in"
        ],
        "ip_version": 6,
        "network": [
          "tcp"
        ],
        "auth_user": [
          "usera",
          "userb"
        ],
        "protocol": [
          "tls",
          "http",
          "quic"
        ],
        "client": [
          "chromium",
          "safari",
          "firefox",
          "quic-go"
        ],
        "domain": [
          "test.com"
        ],
        "domain_suffix": [
          ".cn"
        ],
        "domain_keyword": [
          "test"
        ],
        "domain_regex": [
          "^stun\\..+"
        ],
        "geosite": [
          "cn"
        ],
        "source_geoip": [
          "private"
        ],
        "geoip": [
          "cn"
        ],
        "source_ip_cidr": [
          "10.0.0.0/24"
        ],
        "source_ip_is_private": false,
        "ip_cidr": [
          "10.0.0.0/24"
        ],
        "ip_is_private": false,
        "source_port": [
          12345
        ],
        "source_port_range": [
          "1000:2000",
          ":3000",
          "4000:"
        ],
        "port": [
          80,
          443
        ],
        "port_range": [
          "1000:2000",
          ":3000",
          "4000:"
        ],
        "process_name": [
          "curl"
        ],
        "process_path": [
          "/usr/bin/curl"
        ],
        "process_path_regex": [
          "^/usr/bin/.+"
        ],
        "package_name": [
          "com.termux"
        ],
        "user": [
          "sekai"
        ],
        "user_id": [
          1000
        ],
        "clash_mode": "direct",
        "network_type": [
          "wifi"
        ],
        "network_is_expensive": false,
        "network_is_constrained": false,
        "interface_address": {
          "en0": [
            "2000::/3"
          ]
        },
        "network_interface_address": {
          "wifi": [
            "2000::/3"
          ]
        },
        "default_interface_address": [
          "2000::/3"
        ],
        "wifi_ssid": [
          "My WIFI"
        ],
        "wifi_bssid": [
          "00:00:00:00:00:00"
        ],
        "preferred_by": [
          "tailscale",
          "wireguard"
        ],
        "rule_set": [
          "geoip-cn",
          "geosite-cn"
        ],
        // 已弃用
        "rule_set_ipcidr_match_source": false,
        "rule_set_ip_cidr_match_source": false,
        "invert": false,
        "action": "route",
        "outbound": "direct"
      },
      {
        "type": "logical",
        "mode": "and",
        "rules": [],
        "invert": false,
        "action": "route",
        "outbound": "direct"
      }
    ]
  }
}

```

!!! note ""

    当内容只有一项时，可以忽略 JSON 数组 [] 标签。

### 默认字段

!!! note ""

    默认规则使用以下匹配逻辑:  
    (`domain` || `domain_suffix` || `domain_keyword` || `domain_regex` || `geosite` || `geoip` || `ip_cidr` || `ip_is_private`) &&  
    (`port` || `port_range`) &&  
    (`source_geoip` || `source_ip_cidr` || `source_ip_is_private`) &&  
    (`source_port` || `source_port_range`) &&  
    `other fields`

    另外，引用的规则集可视为被合并，而不是作为一个单独的规则子项。

#### inbound

[入站](/zh/configuration/inbound/) 标签。

#### ip_version

4 或 6。

默认不限制。

#### auth_user

认证用户名，参阅入站设置。

#### protocol

探测到的协议, 参阅 [协议探测](/zh/configuration/route/sniff/)。

#### client

!!! question "自 sing-box 1.10.0 起"

探测到的客户端类型, 参阅 [协议探测](/zh/configuration/route/sniff/)。

#### network

!!! quote "sing-box 1.13.0 中的更改"

    自 sing-box 1.13.0 起，您可以通过新的 `icmp` 网络匹配 ICMP 回显（ping）请求。

    此类流量源自 `TUN`、`WireGuard` 和 `Tailscale` 入站，并可路由至 `Direct`、`WireGuard` 和 `Tailscale` 出站。

匹配网络类型。

`tcp`、`udp` 或 `icmp`。

#### domain

匹配完整域名。

#### domain_suffix

匹配域名后缀。

#### domain_keyword

匹配域名关键字。

#### domain_regex

匹配域名正则表达式。

#### geosite

!!! failure "已在 sing-box 1.8.0 废弃"

    Geosite 已废弃且可能在不久的将来移除，参阅 [迁移指南](/zh/migration/#geosite)。

匹配 Geosite。

#### source_geoip

!!! failure "已在 sing-box 1.8.0 废弃"

    GeoIP 已废弃且可能在不久的将来移除，参阅 [迁移指南](/zh/migration/#geoip)。

匹配源 GeoIP。

#### geoip

!!! failure "已在 sing-box 1.8.0 废弃"

    GeoIP 已废弃且可能在不久的将来移除，参阅 [迁移指南](/zh/migration/#geoip)。

匹配 GeoIP。

#### source_ip_cidr

匹配源 IP CIDR。

#### source_ip_is_private

!!! question "自 sing-box 1.8.0 起"

匹配非公开源 IP。

#### ip_cidr

匹配 IP CIDR。

#### ip_is_private

!!! question "自 sing-box 1.8.0 起"

匹配非公开 IP。

#### source_port

匹配源端口。

#### source_port_range

匹配源端口范围。

#### port

匹配端口。

#### port_range

匹配端口范围。

#### process_name

!!! quote ""

    仅支持 Linux、Windows 和 macOS。

匹配进程名称。

#### process_path

!!! quote ""

    仅支持 Linux、Windows 和 macOS.

匹配进程路径。

#### process_path_regex

!!! question "自 sing-box 1.10.0 起"

!!! quote ""

    仅支持 Linux、Windows 和 macOS.

使用正则表达式匹配进程路径。

#### package_name

匹配 Android 应用包名。

#### user

!!! quote ""

    仅支持 Linux.

匹配用户名。

#### user_id

!!! quote ""

    仅支持 Linux.

匹配用户 ID。

#### clash_mode

匹配 Clash 模式。

#### network_type

!!! question "自 sing-box 1.11.0 起"

!!! quote ""

    仅在 Android 与 Apple 平台图形客户端中支持。

匹配网络类型。

可用值: `wifi`, `cellular`, `ethernet` and `other`.

#### network_is_expensive

!!! question "自 sing-box 1.11.0 起"

!!! quote ""

    仅在 Android 与 Apple 平台图形客户端中支持。

匹配如果网络被视为计费 (在 Android) 或被视为昂贵，
像蜂窝网络或个人热点 (在 Apple 平台)。

#### network_is_constrained

!!! question "自 sing-box 1.11.0 起"

!!! quote ""

    仅在 Apple 平台图形客户端中支持。

匹配如果网络在低数据模式下。

#### interface_address

!!! question "自 sing-box 1.13.0 起"

!!! quote ""

    仅支持 Linux、Windows 和 macOS.

匹配接口地址。

#### network_interface_address

!!! question "自 sing-box 1.13.0 起"

!!! quote ""

    仅在 Android 与 Apple 平台图形客户端中支持。

匹配网络接口（可用值同 `network_type`）地址。

#### default_interface_address

!!! question "自 sing-box 1.13.0 起"

!!! quote ""

    仅支持 Linux、Windows 和 macOS.

匹配默认接口地址。

#### wifi_ssid

!!! quote ""

    仅在 Android 与 Apple 平台图形客户端中支持。

匹配 WiFi SSID。

#### wifi_bssid

!!! quote ""

    仅在 Android 与 Apple 平台图形客户端中支持。

匹配 WiFi BSSID。

#### preferred_by

!!! question "自 sing-box 1.13.0 起"

匹配制定出站的首选路由。

| 类型          | 匹配                             |
|-------------|--------------------------------|
| `tailscale` | 匹配 MagicDNS 域名和对端的 allowed IPs |
| `wireguard` | 匹配对端的 allowed IPs              |

#### rule_set

!!! question "自 sing-box 1.8.0 起"

匹配[规则集](/zh/configuration/route/#rule_set)。

#### rule_set_ipcidr_match_source

!!! question "自 sing-box 1.8.0 起"

!!! failure "已在 sing-box 1.10.0 废弃"

    `rule_set_ipcidr_match_source` 已重命名为 `rule_set_ip_cidr_match_source` 且将在 sing-box 1.11.0 中被移除。

使规则集中的 `ip_cidr` 规则匹配源 IP。

#### rule_set_ip_cidr_match_source

!!! question "自 sing-box 1.10.0 起"

使规则集中的 `ip_cidr` 规则匹配源 IP。

#### invert

反选匹配结果。

#### action

==必填==

参阅 [规则动作](../rule_action/)。

#### outbound

!!! failure "已在 sing-box 1.11.0 废弃"

    已移动到 [规则动作](../rule_action#route).

### 逻辑字段

#### type

`logical`

#### mode

==必填==

`and` 或 `or`

#### rules

==必填==

包括的规则。