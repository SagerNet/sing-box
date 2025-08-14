---
icon: material/alert-decagram
---

!!! quote "Changes in sing-box 1.13.0"

    :material-plus: [interface_address](#interface_address)  
    :material-plus: [network_interface_address](#network_interface_address)  
    :material-plus: [default_interface_address](#default_interface_address)

!!! quote "Changes in sing-box 1.12.0"

    :material-plus: [ip_accept_any](#ip_accept_any)  
    :material-delete-clock: [outbound](#outbound)

!!! quote "Changes in sing-box 1.11.0"

    :material-plus: [action](#action)  
    :material-alert: [server](#server)  
    :material-alert: [disable_cache](#disable_cache)  
    :material-alert: [rewrite_ttl](#rewrite_ttl)  
    :material-alert: [client_subnet](#client_subnet)  
    :material-plus: [network_type](#network_type)  
    :material-plus: [network_is_expensive](#network_is_expensive)  
    :material-plus: [network_is_constrained](#network_is_constrained)

!!! quote "Changes in sing-box 1.10.0"

    :material-delete-clock: [rule_set_ipcidr_match_source](#rule_set_ipcidr_match_source)  
    :material-plus: [rule_set_ip_cidr_match_source](#rule_set_ip_cidr_match_source)  
    :material-plus: [rule_set_ip_cidr_accept_empty](#rule_set_ip_cidr_accept_empty)  
    :material-plus: [process_path_regex](#process_path_regex)

!!! quote "Changes in sing-box 1.9.0"

    :material-plus: [geoip](#geoip)  
    :material-plus: [ip_cidr](#ip_cidr)  
    :material-plus: [ip_is_private](#ip_is_private)  
    :material-plus: [client_subnet](#client_subnet)  
    :material-plus: [rule_set_ipcidr_match_source](#rule_set_ipcidr_match_source)

!!! quote "Changes in sing-box 1.8.0"

    :material-plus: [rule_set](#rule_set)  
    :material-plus: [source_ip_is_private](#source_ip_is_private)  
    :material-delete-clock: [geoip](#geoip)  
    :material-delete-clock: [geosite](#geosite)

### Structure

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
        "ip_cidr": [
          "10.0.0.0/24",
          "192.168.0.1"
        ],
        "ip_is_private": false,
        "ip_accept_any": false,
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
        "rule_set": [
          "geoip-cn",
          "geosite-cn"
        ],
        "rule_set_ip_cidr_match_source": false,
        "rule_set_ip_cidr_accept_empty": false,
        "invert": false,
        "outbound": [
          "direct"
        ],
        "action": "route",
        "server": "local",

        // Deprecated
        
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

    You can ignore the JSON Array [] tag when the content is only one item

### Default Fields

!!! note ""

    The default rule uses the following matching logic:  
    (`domain` || `domain_suffix` || `domain_keyword` || `domain_regex` || `geosite`) &&  
    (`port` || `port_range`) &&  
    (`source_geoip` || `source_ip_cidr` ｜｜ `source_ip_is_private`) &&  
    (`source_port` || `source_port_range`) &&  
    `other fields`

    Additionally, included rule-sets can be considered merged rather than as a single rule sub-item.

#### inbound

Tags of [Inbound](/configuration/inbound/).

#### ip_version

4 (A DNS query) or 6 (AAAA DNS query).

Not limited if empty.

#### query_type

DNS query type. Values can be integers or type name strings.

#### network

`tcp` or `udp`.

#### auth_user

Username, see each inbound for details.

#### protocol

Sniffed protocol, see [Sniff](/configuration/route/sniff/) for details.

#### domain

Match full domain.

#### domain_suffix

Match domain suffix.

#### domain_keyword

Match domain using keyword.

#### domain_regex

Match domain using regular expression.

#### geosite

!!! failure "Deprecated in sing-box 1.8.0"

    Geosite is deprecated and will be removed in sing-box 1.12.0, check [Migration](/migration/#migrate-geosite-to-rule-sets).

Match geosite.

#### source_geoip

!!! failure "Deprecated in sing-box 1.8.0"

    GeoIP is deprecated and will be removed in sing-box 1.12.0, check [Migration](/migration/#migrate-geoip-to-rule-sets).

Match source geoip.

#### source_ip_cidr

Match source IP CIDR.

#### source_ip_is_private

!!! question "Since sing-box 1.8.0"

Match non-public source IP.

#### source_port

Match source port.

#### source_port_range

Match source port range.

#### port

Match port.

#### port_range

Match port range.

#### process_name

!!! quote ""

    Only supported on Linux, Windows, and macOS.

Match process name.

#### process_path

!!! quote ""

    Only supported on Linux, Windows, and macOS.

Match process path.

#### process_path_regex

!!! question "Since sing-box 1.10.0"

!!! quote ""

    Only supported on Linux, Windows, and macOS.

Match process path using regular expression.

#### package_name

Match android package name.

#### user

!!! quote ""

    Only supported on Linux.

Match user name.

#### user_id

!!! quote ""

    Only supported on Linux.

Match user id.

#### clash_mode

Match Clash mode.

#### network_type

!!! question "Since sing-box 1.11.0"

!!! quote ""

    Only supported in graphical clients on Android and Apple platforms.

Match network type.

Available values: `wifi`, `cellular`, `ethernet` and `other`.

#### network_is_expensive

!!! question "Since sing-box 1.11.0"

!!! quote ""

    Only supported in graphical clients on Android and Apple platforms.

Match if network is considered Metered (on Android) or considered expensive,
such as Cellular or a Personal Hotspot (on Apple platforms).

#### network_is_constrained

!!! question "Since sing-box 1.11.0"

!!! quote ""

    Only supported in graphical clients on Apple platforms.

Match if network is in Low Data Mode.

#### interface_address

!!! question "Since sing-box 1.13.0"

!!! quote ""

    Only supported on Linux, Windows, and macOS.

Match interface address.

#### network_interface_address

!!! question "Since sing-box 1.13.0"

!!! quote ""

    Only supported in graphical clients on Android and Apple platforms.

Matches network interface (same values as `network_type`) address.

#### default_interface_address

!!! question "Since sing-box 1.13.0"

!!! quote ""

    Only supported on Linux, Windows, and macOS.

Match default interface address.

#### wifi_ssid

!!! quote ""

    Only supported in graphical clients on Android and Apple platforms.

Match WiFi SSID.

#### wifi_bssid

!!! quote ""

    Only supported in graphical clients on Android and Apple platforms.

Match WiFi BSSID.

#### rule_set

!!! question "Since sing-box 1.8.0"

Match [rule-set](/configuration/route/#rule_set).

#### rule_set_ipcidr_match_source

!!! question "Since sing-box 1.9.0"

!!! failure "Deprecated in sing-box 1.10.0"
    
    `rule_set_ipcidr_match_source` is renamed to `rule_set_ip_cidr_match_source` and will be remove in sing-box 1.11.0.

Make `ip_cidr` rule items in rule-sets match the source IP.

#### rule_set_ip_cidr_match_source

!!! question "Since sing-box 1.10.0"

Make `ip_cidr` rule items in rule-sets match the source IP.

#### invert

Invert match result.

#### outbound

!!! failure "Deprecated in sing-box 1.12.0"

    `outbound` rule items are deprecated and will be removed in sing-box 1.14.0, check [Migration](/migration/#migrate-outbound-dns-rule-items-to-domain-resolver). 

Match outbound.

`any` can be used as a value to match any outbound.

#### action

==Required==

See [DNS Rule Actions](../rule_action/) for details.

#### server

!!! failure "Deprecated in sing-box 1.11.0"

    Moved to [DNS Rule Action](../rule_action#route).

#### disable_cache

!!! failure "Deprecated in sing-box 1.11.0"

    Moved to [DNS Rule Action](../rule_action#route).

#### rewrite_ttl

!!! failure "Deprecated in sing-box 1.11.0"

    Moved to [DNS Rule Action](../rule_action#route).

#### client_subnet

!!! failure "Deprecated in sing-box 1.11.0"

    Moved to [DNS Rule Action](../rule_action#route).

### Address Filter Fields

Only takes effect for address requests (A/AAAA/HTTPS). When the query results do not match the address filtering rule items, the current rule will be skipped.

!!! info ""

    `ip_cidr` items in included rule-sets also takes effect as an address filtering field.

!!! note ""

    Enable `experimental.cache_file.store_rdrc` to cache results.

#### geoip

!!! failure "Removed in sing-box 1.12.0"

    GeoIP is deprecated in sing-box 1.8.0 and removed in sing-box 1.12.0, check [Migration](/migration/#migrate-geoip-to-rule-sets).

Match GeoIP with query response.

#### ip_cidr

!!! question "Since sing-box 1.9.0"

Match IP CIDR with query response.

#### ip_is_private

!!! question "Since sing-box 1.9.0"

Match private IP with query response.

#### rule_set_ip_cidr_accept_empty

!!! question "Since sing-box 1.10.0"

Make `ip_cidr` rules in rule-sets accept empty query response.

#### ip_accept_any

!!! question "Since sing-box 1.12.0"

Match any IP with query response.

### Logical Fields

#### type

`logical`

#### mode

`and` or `or`

#### rules

Included rules.