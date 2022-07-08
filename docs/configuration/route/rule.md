### Structure

```json
{
  "route": {
    "rules": [
      {
        "inbound": [
          "mixed-in"
        ],
        "network": "tcp",
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
        "ip_cidr": [
          "10.0.0.0/24"
        ],
        "source_port": [
          12345
        ],
        "port": [
          80,
          443
        ],
        "outbound": "direct"
      },
      {
        "type": "logical",
        "mode": "and",
        "rules": [],
        "outbound": "direct"
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
    (`source_geoip` || `source_ip_cidr`) &&  
    `other fields`  

#### inbound

Tags of [inbound](../inbound).

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

#### port

Match port.

#### outbound

Tag of the target outbound.

### Logical Fields

#### type

`logical`

#### mode

`and` or `or`

#### rules

Included default rules.

#### outbound

Tag of the target outbound.
