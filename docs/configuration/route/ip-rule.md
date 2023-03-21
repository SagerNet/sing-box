### Structure

```json
{
  "route": {
    "ip_rules": [
      {
        "inbound": [
          "mixed-in"
        ],
        "ip_version": 6,
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
        "invert": false,
        "action": "direct",
        "outbound": "wireguard"
      },
      {
        "type": "logical",
        "mode": "and",
        "rules": [],
        "invert": false,
        "action": "direct",
        "outbound": "wireguard"
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
    (`domain` || `domain_suffix` || `domain_keyword` || `domain_regex` || `geosite` || `geoip` || `ip_cidr`) &&  
    (`port` || `port_range`) &&  
    (`source_geoip` || `source_ip_cidr`) &&  
    (`source_port` || `source_port_range`) &&  
    `other fields`

#### inbound

Tags of [Inbound](/configuration/inbound).

#### ip_version

4 or 6.

Not limited if empty.

#### network

Match network protocol.

Available values:

* `tcp`
* `udp`
* `icmpv4`
* `icmpv6`

#### domain

Match full domain.

#### domain_suffix

Match domain suffix.

#### domain_keyword

Match domain using keyword.

#### domain_regex

Match domain using regular expression.

#### geosite

Match geosite.

#### source_geoip

Match source geoip.

#### geoip

Match geoip.

#### source_ip_cidr

Match source ip cidr.

#### ip_cidr

Match ip cidr.

#### source_port

Match source port.

#### source_port_range

Match source port range.

#### port

Match port.

#### port_range

Match port range.

#### invert

Invert match result.

#### action

==Required==

| Action | Description                                                        |
|--------|--------------------------------------------------------------------|
| return | Stop IP routing and assemble the connection to the transport layer |
| block  | Block the connection                                               |
| direct | Directly forward the connection                                    |

#### outbound

==Required if action is direct==

Tag of the target outbound.

Only outbound which supports IP connection can be used, see [Outbounds that support IP connection](/configuration/outbound/#outbounds-that-support-ip-connection).

### Logical Fields

#### type

`logical`

#### mode

==Required==

`and` or `or`

#### rules

==Required==

Included default rules.