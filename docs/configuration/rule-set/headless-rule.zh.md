---
icon: material/new-box
---

!!! quote "sing-box 1.14.0 中的更改"

    :material-plus: [package_name_regex](#package_name_regex)  
    :material-alert: [query_type](#query_type)

!!! quote "sing-box 1.13.0 中的更改"

    :material-plus: [network_interface_address](#network_interface_address)  
    :material-plus: [default_interface_address](#default_interface_address)

!!! quote "sing-box 1.11.0 中的更改"

    :material-plus: [network_type](#network_type)  
    :material-plus: [network_is_expensive](#network_is_expensive)  
    :material-plus: [network_is_constrained](#network_is_constrained)

### 结构

!!! question "自 sing-box 1.8.0 起"

```json
{
  "rules": [
    {
      "query_type": [
        "A",
        "HTTPS",
        32768
      ],
      "network": [
        "tcp"
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
      "ip_cidr": [
        "10.0.0.0/24",
        "192.168.0.1"
      ],
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
      "package_name_regex": [
        "^com\\.termux.*"
      ],
      "network_type": [
        "wifi"
      ],
      "network_is_expensive": false,
      "network_is_constrained": false,
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
      "invert": false
    },
    {
      "type": "logical",
      "mode": "and",
      "rules": [],
      "invert": false
    }
  ]
}
```

!!! note ""

    当内容只有一项时，可以忽略 JSON 数组 [] 标签。

### Default Fields

!!! note ""

    默认规则使用以下匹配逻辑:  
    (`domain` || `domain_suffix` || `domain_keyword` || `domain_regex` || `ip_cidr`) &&  
    (`port` || `port_range`) &&  
    (`source_port` || `source_port_range`) &&  
    `other fields`

#### query_type

!!! quote "sing-box 1.14.0 中的更改"

    当 DNS 规则引用此规则集时，此字段现在也会在 DNS 规则被未指定具体
    DNS 服务器的内部域名解析匹配时生效。此前只有来自客户端的 DNS 查询
    才会评估此字段。完整列表参阅
    [迁移指南](/zh/migration/#dns-规则中的-ip_version-和-query_type-行为更改)。

    当 DNS 规则引用了包含此字段的规则集时，该 DNS 规则在同一 DNS 配置中
    不能与旧版地址筛选字段 (DNS 规则)、旧版 DNS 规则动作 `strategy` 选项，
    或旧版 `rule_set_ip_cidr_accept_empty` DNS 规则项共存。

DNS 查询类型。值可以为整数或者类型名称字符串。

#### network

`tcp` 或 `udp`。

#### domain

匹配完整域名。

#### domain_suffix

匹配域名后缀。

#### domain_keyword

匹配域名关键字。

#### domain_regex

匹配域名正则表达式。

#### source_ip_cidr

匹配源 IP CIDR。

#### ip_cidr

匹配 IP CIDR。

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

#### package_name_regex

!!! question "自 sing-box 1.14.0 起"

使用正则表达式匹配 Android 应用包名。

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

#### invert

反选匹配结果。

### 逻辑字段

#### type

`logical`

#### mode

==必填==

`and` 或 `or`

#### rules

==必填==

包括的规则。