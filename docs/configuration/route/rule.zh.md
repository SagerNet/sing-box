---
icon: material/alert-decagram
---

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
        "invert": false,
        "outbound": "direct"
      },
      {
        "type": "logical",
        "mode": "and",
        "rules": [],
        "invert": false,
        "outbound": "direct"
      }
    ]
  }
}

```

!!! note ""

    当内容只有一项时，可以忽略 JSON 数组 [] 标签。

### Default Fields

!!! note ""

    默认规则使用以下匹配逻辑:  
    (`domain` || `domain_suffix` || `domain_keyword` || `domain_regex` || `geosite` || `geoip` || `ip_cidr`) &&  
    (`port` || `port_range`) &&  
    (`source_geoip` || `source_ip_cidr`) &&  
    (`source_port` || `source_port_range`) &&  
    `other fields`

#### inbound

[入站](/zh/configuration/inbound) 标签。

#### ip_version

4 或 6。

默认不限制。

#### auth_user

认证用户名，参阅入站设置。

#### protocol

探测到的协议, 参阅 [协议探测](/zh/configuration/route/sniff/)。

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

#### geosite

!!! failure "已在 sing-box 1.8.0 废弃"

    Geosite 已废弃且可能在不久的将来移除，参阅 [迁移指南](/migration/#migrate-geosite-to-rule-sets)。

匹配 Geosite。

#### source_geoip

!!! failure "已在 sing-box 1.8.0 废弃"

    GeoIp 已废弃且可能在不久的将来移除，参阅 [迁移指南](/migration/#migrate-geoip-to-rule-sets)。

匹配源 GeoIP。

#### geoip

!!! failure "已在 sing-box 1.8.0 废弃"

    GeoIp 已废弃且可能在不久的将来移除，参阅 [迁移指南](/migration/#migrate-geoip-to-rule-sets)。

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

#### wifi_ssid

!!! quote ""

    仅在 Android 与 iOS 的图形客户端中支持。

匹配 WiFi SSID。

#### wifi_bssid

!!! quote ""

    仅在 Android 与 iOS 的图形客户端中支持。

匹配 WiFi BSSID。

#### rule_set

!!! question "自 sing-box 1.8.0 起"

匹配[规则集](/zh/configuration/route/#rule_set)。

#### rule_set_ipcidr_match_source

!!! question "自 sing-box 1.8.0 起"

使规则集中的 `ipcidr` 规则匹配源 IP。

#### invert

反选匹配结果。

#### outbound

==必填==

目标出站的标签。

### 逻辑字段

#### type

`logical`

#### mode

==必填==

`and` 或 `or`

#### rules

==必填==

包括的规则。