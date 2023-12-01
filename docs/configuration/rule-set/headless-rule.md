---
icon: material/new-box
---

### Structure

!!! question "Since sing-box 1.8.0"

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
      "package_name": [
        "com.termux"
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

    You can ignore the JSON Array [] tag when the content is only one item

### Default Fields

!!! note ""

    The default rule uses the following matching logic:  
    (`domain` || `domain_suffix` || `domain_keyword` || `domain_regex` || `ip_cidr`) &&  
    (`port` || `port_range`) &&  
    (`source_port` || `source_port_range`) &&  
    `other fields`

#### query_type

DNS query type. Values can be integers or type name strings.

#### network

`tcp` or `udp`.

#### domain

Match full domain.

#### domain_suffix

Match domain suffix.

#### domain_keyword

Match domain using keyword.

#### domain_regex

Match domain using regular expression.

#### source_ip_cidr

Match source IP CIDR.

#### ip_cidr

!!! info ""

    `ip_cidr` is an alias for `source_ip_cidr` when the Rule Set is used in DNS rules or `rule_set_ipcidr_match_source` enabled in route rules.

Match IP CIDR.

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

#### package_name

Match android package name.

#### wifi_ssid

!!! quote ""

    Only supported in graphical clients on Android and iOS.

Match WiFi SSID.

#### wifi_bssid

!!! quote ""

    Only supported in graphical clients on Android and iOS.

Match WiFi BSSID.

#### invert

Invert match result.

### Logical Fields

#### type

`logical`

#### mode

==Required==

`and` or `or`

#### rules

==Required==

Included rules.
