---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.13.0"

    :material-plus: [network_interface_address](#network_interface_address)  
    :material-plus: [default_interface_address](#default_interface_address)

!!! quote "Changes in sing-box 1.11.0"

    :material-plus: [network_type](#network_type)  
    :material-plus: [network_is_expensive](#network_is_expensive)  
    :material-plus: [network_is_constrained](#network_is_constrained)

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
      "process_path_regex": [
        "^/usr/bin/.+"
      ],
      "package_name": [
        "com.termux"
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

    `ip_cidr` is an alias for `source_ip_cidr` when `rule_set_ipcidr_match_source` enabled in route/DNS rules.

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

#### process_path_regex

!!! question "Since sing-box 1.10.0"

!!! quote ""

    Only supported on Linux, Windows, and macOS.

Match process path using regular expression.

#### package_name

Match android package name.

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
