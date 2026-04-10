---
icon: material/alert-decagram
---

!!! quote "sing-box 1.14.0 中的更改"

    :material-plus: [source_mac_address](#source_mac_address)  
    :material-plus: [source_hostname](#source_hostname)  
    :material-plus: [match_response](#match_response)  
    :material-delete-clock: [rule_set_ip_cidr_accept_empty](#rule_set_ip_cidr_accept_empty)  
    :material-plus: [response_rcode](#response_rcode)  
    :material-plus: [response_answer](#response_answer)  
    :material-plus: [response_ns](#response_ns)  
    :material-plus: [response_extra](#response_extra)

!!! quote "sing-box 1.13.0 中的更改"

    :material-plus: [interface_address](#interface_address)  
    :material-plus: [network_interface_address](#network_interface_address)  
    :material-plus: [default_interface_address](#default_interface_address)

!!! quote "sing-box 1.12.0 中的更改"

    :material-plus: [ip_accept_any](#ip_accept_any)  
    :material-delete-clock: [outbound](#outbound)

!!! quote "sing-box 1.11.0 中的更改"

    :material-plus: [action](#action)  
    :material-alert: [server](#server)  
    :material-alert: [disable_cache](#disable_cache)  
    :material-alert: [rewrite_ttl](#rewrite_ttl)  
    :material-alert: [client_subnet](#client_subnet)  
    :material-plus: [network_type](#network_type)  
    :material-plus: [network_is_expensive](#network_is_expensive)  
    :material-plus: [network_is_constrained](#network_is_constrained)

!!! quote "sing-box 1.10.0 中的更改"

    :material-delete-clock: [rule_set_ipcidr_match_source](#rule_set_ipcidr_match_source)  
    :material-plus: [rule_set_ip_cidr_match_source](#rule_set_ip_cidr_match_source)  
    :material-plus: [rule_set_ip_cidr_accept_empty](#rule_set_ip_cidr_accept_empty)  
    :material-plus: [process_path_regex](#process_path_regex)

!!! quote "sing-box 1.9.0 中的更改"

    :material-plus: [geoip](#geoip)  
    :material-plus: [ip_cidr](#ip_cidr)  
    :material-plus: [ip_is_private](#ip_is_private)  
    :material-plus: [client_subnet](#client_subnet)  
    :material-plus: [rule_set_ipcidr_match_source](#rule_set_ipcidr_match_source)

!!! quote "sing-box 1.8.0 中的更改"

    :material-plus: [rule_set](#rule_set)  
    :material-plus: [source_ip_is_private](#source_ip_is_private)  
    :material-delete-clock: [geoip](#geoip)  
    :material-delete-clock: [geosite](#geosite)

### 结构

```json
{
  "dns": {
    "rules": [
      {
        "inbound": [
          "mixed-in"
        ],
        "ip_version": 6,
        "query_type": [
          "A",
          "HTTPS",
          32768
        ],
        "network": "tcp",
        "auth_user": [
          "usera",
          "userb"
        ],
        "protocol": [
          "tls",
          "http",
          "quic"
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
        "source_ip_cidr": [
          "10.0.0.0/24",
          "192.168.0.1"
        ],
        "source_ip_is_private": false,
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
        "source_mac_address": [
          "00:11:22:33:44:55"
        ],
        "source_hostname": [
          "my-device"
        ],
        "wifi_ssid": [
          "My WIFI"
        ],
        "wifi_bssid": [
          "00:00:00:00:00:00"
        ],
        "rule_set": [
          "geoip-cn",
          "geosite-cn"
        ],
        "rule_set_ip_cidr_match_source": false,
        "match_response": false,
        "ip_cidr": [
          "10.0.0.0/24",
          "192.168.0.1"
        ],
        "ip_is_private": false,
        "ip_accept_any": false,
        "response_rcode": "",
        "response_answer": [],
        "response_ns": [],
        "response_extra": [],
        "invert": false,
        "outbound": [
          "direct"
        ],
        "action": "route",
        "server": "local",

        // 已弃用

        "rule_set_ip_cidr_accept_empty": false,
        "rule_set_ipcidr_match_source": false,
        "geosite": [
          "cn"
        ],
        "source_geoip": [
          "private"
        ],
        "geoip": [
          "cn"
        ]
      },
      {
        "type": "logical",
        "mode": "and",
        "rules": [],
        "action": "route",
        "server": "local"
      }
    ]
  }
}

```

!!! note ""

    当内容只有一项时，可以忽略 JSON 数组 [] 标签

### 默认字段

!!! note ""

    默认规则使用以下匹配逻辑:  
    (`domain` || `domain_suffix` || `domain_keyword` || `domain_regex` || `geosite`) &&  
    (`port` || `port_range`) &&  
    (`source_geoip` || `source_ip_cidr` || `source_ip_is_private`) &&  
    (`source_port` || `source_port_range`) &&  
    `other fields`

    另外，引用规则集中的每个分支都可视为与外层规则合并，不同分支之间仍保持 OR 语义。

#### inbound

[入站](/zh/configuration/inbound/) 标签.

#### ip_version

4 (A DNS 查询) 或 6 (AAAA DNS 查询)。

默认不限制。

#### query_type

DNS 查询类型。值可以为整数或者类型名称字符串。

#### network

`tcp` 或 `udp`。

#### auth_user

认证用户名，参阅入站设置。

#### protocol

探测到的协议, 参阅 [协议探测](/zh/configuration/route/sniff/)。

#### domain

匹配完整域名。

#### domain_suffix

匹配域名后缀。

#### domain_keyword

匹配域名关键字。

#### domain_regex

匹配域名正则表达式。

#### geosite

!!! failure "已在 sing-box 1.12.0 中被移除"

    GeoSite 已在 sing-box 1.8.0 废弃且在 sing-box 1.12.0 中被移除，参阅 [迁移指南](/zh/migration/#迁移-geosite-到规则集)。

匹配 Geosite。

#### source_geoip

!!! failure "已在 sing-box 1.12.0 中被移除"

    GeoIP 已在 sing-box 1.8.0 废弃且在 sing-box 1.12.0 中被移除，参阅 [迁移指南](/zh/migration/#迁移-geoip-到规则集)。

匹配源 GeoIP。

#### source_ip_cidr

匹配源 IP CIDR。

#### source_ip_is_private

!!! question "自 sing-box 1.8.0 起"

匹配非公开源 IP。

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

    仅支持 Linux、Windows 和 macOS.

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

    仅支持 Linux。

匹配用户名。

#### user_id

!!! quote ""

    仅支持 Linux。

匹配用户 ID。

#### clash_mode

匹配 Clash 模式。

#### network_type

!!! question "自 sing-box 1.11.0 起"

!!! quote ""

    仅在 Android 与 Apple 平台图形客户端中支持。

匹配网络类型。

Available values: `wifi`, `cellular`, `ethernet` and `other`.

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

#### source_mac_address

!!! question "自 sing-box 1.14.0 起"

!!! quote ""

    仅支持 Linux、macOS，或在 Android 和 macOS 图形客户端中支持。参阅 [邻居解析](/configuration/shared/neighbor/) 了解设置方法。

匹配源设备 MAC 地址。

#### source_hostname

!!! question "自 sing-box 1.14.0 起"

!!! quote ""

    仅支持 Linux、macOS，或在 Android 和 macOS 图形客户端中支持。参阅 [邻居解析](/configuration/shared/neighbor/) 了解设置方法。

匹配源设备从 DHCP 租约获取的主机名。

#### wifi_ssid

!!! quote ""

    仅在 Android 与 Apple 平台图形客户端和 Linux 中支持。

匹配 WiFi SSID。

#### wifi_bssid

!!! quote ""

    仅在 Android 与 Apple 平台图形客户端和 Linux 中支持。

匹配 WiFi BSSID。

#### rule_set

!!! question "自 sing-box 1.8.0 起"

匹配[规则集](/zh/configuration/route/#rule_set)。

#### rule_set_ipcidr_match_source

!!! question "自 sing-box 1.9.0 起"

!!! failure "已在 sing-box 1.10.0 废弃"

    `rule_set_ipcidr_match_source` 已重命名为 `rule_set_ip_cidr_match_source` 且将在 sing-box 1.11.0 中被移除。

使规则集中的 `ip_cidr` 规则匹配源 IP。

#### rule_set_ip_cidr_match_source

!!! question "自 sing-box 1.10.0 起"

使规则集中的 `ip_cidr` 规则匹配源 IP。

#### match_response

!!! question "自 sing-box 1.14.0 起"

启用响应匹配。启用后，此规则将匹配已评估的响应（由前序 [`evaluate`](/zh/configuration/dns/rule_action/#evaluate) 动作设置），而不仅是匹配原始查询。

该已评估的响应也可以被后续的 [`respond`](/zh/configuration/dns/rule_action/#respond) 动作直接返回。

响应匹配字段（`response_rcode`、`response_answer`、`response_ns`、`response_extra`）需要此选项。
当与 `evaluate` 或响应匹配字段一起使用时，`ip_cidr`、`ip_is_private` 和 `ip_accept_any` 也需要此选项。

#### ip_accept_any

!!! question "自 sing-box 1.12.0 起"

当 DNS 查询响应包含至少一个地址时匹配。

#### invert

反选匹配结果。

#### outbound

!!! failure "已在 sing-box 1.12.0 废弃"

    `outbound` 规则项已废弃且将在 sing-box 1.14.0 中被移除，参阅 [迁移指南](/zh/migration/#迁移-outbound-dns-规则项到域解析选项)。

匹配出站。

`any` 可作为值用于匹配任意出站。

#### action

==必填==

参阅 [规则动作](../rule_action/)。

#### server

!!! failure "已在 sing-box 1.11.0 废弃"

    已移动到 [DNS 规则动作](../rule_action#route).

#### disable_cache

!!! failure "已在 sing-box 1.11.0 废弃"

    已移动到 [DNS 规则动作](../rule_action#route).

#### rewrite_ttl

!!! failure "已在 sing-box 1.11.0 废弃"

    已移动到 [DNS 规则动作](../rule_action#route).

#### client_subnet

!!! failure "已在 sing-box 1.11.0 废弃"

    已移动到 [DNS 规则动作](../rule_action#route).

### 旧版地址筛选字段

!!! failure "已在 sing-box 1.14.0 废弃"

    旧版地址筛选字段已废弃，且将在 sing-box 1.16.0 中被移除，
    参阅[迁移指南](/zh/migration/#迁移地址筛选字段到响应匹配)。

仅对地址请求 (A/AAAA/HTTPS) 生效。 当查询结果与地址筛选规则项不匹配时，将跳过当前规则。

!!! info ""

    引用的规则集中的 `ip_cidr` 项也作为地址筛选字段生效。

!!! note ""

    启用 `experimental.cache_file.store_rdrc` 以缓存结果。

#### geoip

!!! failure "已在 sing-box 1.12.0 中被移除"

    GeoIP 已在 sing-box 1.8.0 废弃且在 sing-box 1.12.0 中被移除，参阅 [迁移指南](/zh/migration/#迁移-geoip-到规则集)。


与查询响应匹配 GeoIP。

#### ip_cidr

!!! question "自 sing-box 1.9.0 起"

与查询响应匹配 IP CIDR。

作为旧版地址筛选字段已废弃。请改为配合 `match_response` 使用，
参阅[迁移指南](/zh/migration/#迁移地址筛选字段到响应匹配)。

#### ip_is_private

!!! question "自 sing-box 1.9.0 起"

与查询响应匹配非公开 IP。

作为旧版地址筛选字段已废弃。请改为配合 `match_response` 使用，
参阅[迁移指南](/zh/migration/#迁移地址筛选字段到响应匹配)。

#### rule_set_ip_cidr_accept_empty

!!! question "自 sing-box 1.10.0 起"

!!! failure "已在 sing-box 1.14.0 废弃"

    `rule_set_ip_cidr_accept_empty` 已废弃且将在 sing-box 1.16.0 中被移除，
    参阅[迁移指南](/zh/migration/#迁移地址筛选字段到响应匹配)。

使规则集中的 `ip_cidr` 规则接受空查询响应。

### 响应匹配字段

!!! question "自 sing-box 1.14.0 起"

已评估的响应的匹配字段。需要将 `match_response` 设为 `true`，
且需要前序规则使用 [`evaluate`](/zh/configuration/dns/rule_action/#evaluate) 动作来填充响应。

该已评估的响应也可以被后续的 [`respond`](/zh/configuration/dns/rule_action/#respond) 动作直接返回。

#### response_rcode

匹配 DNS 响应码。

接受的值与 [predefined 动作 rcode](/zh/configuration/dns/rule_action/#rcode) 中相同。

#### response_answer

匹配 DNS 应答记录。

记录格式与 [predefined 动作 answer](/zh/configuration/dns/rule_action/#answer) 中相同。

#### response_ns

匹配 DNS 名称服务器记录。

记录格式与 [predefined 动作 ns](/zh/configuration/dns/rule_action/#ns) 中相同。

#### response_extra

匹配 DNS 额外记录。

记录格式与 [predefined 动作 extra](/zh/configuration/dns/rule_action/#extra) 中相同。

### 逻辑字段

#### type

`logical`

#### mode

==必填==

`and` 或 `or`

#### rules

==必填==

包括的规则。
