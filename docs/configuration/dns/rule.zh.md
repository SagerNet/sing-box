---
icon: material/alert-decagram
---

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
        "geosite": [
          "cn"
        ],
        "source_geoip": [
          "private"
        ],
        "source_ip_cidr": [
          "10.0.0.0/24"
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
        "outbound": [
          "direct"
        ],
        "server": "local",
        "disable_cache": false
      },
      {
        "type": "logical",
        "mode": "and",
        "rules": [],
        "server": "local",
        "disable_cache": false
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
    (`source_geoip` || `source_ip_cidr`) &&  
    (`source_port` || `source_port_range`) &&  
    `other fields`

#### inbound

[入站](/zh/configuration/inbound) 标签.

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

!!! failure "已在 sing-box 1.8.0 废弃"

    Geosite 已废弃且可能在不久的将来移除，参阅 [迁移指南](/migration/#migrate-geosite-to-rule-sets)。

匹配 Geosite。

#### source_geoip

!!! failure "已在 sing-box 1.8.0 废弃"

    GeoIp 已废弃且可能在不久的将来移除，参阅 [迁移指南](/migration/#migrate-geoip-to-rule-sets)。

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

#### invert

反选匹配结果。

#### outbound

匹配出站。

`any` 可作为值用于匹配任意出站。

#### server

==必填==

目标 DNS 服务器的标签。

#### disable_cache

在此查询中禁用缓存。

#### rewrite_ttl

重写 DNS 回应中的 TTL。

### 逻辑字段

#### type

`logical`

#### mode

`and` 或 `or`

#### rules

包括的规则。