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
        "geosite": [
          "cn"
        ],
        "source_geoip": [
          "private"
        ],
        "source_ip_cidr": [
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
        "user": [
          "sekai"
        ],
        "user_id": [
          1000
        ],
        "clash_mode": "direct",
        "invert": false,
        "outbound": [
          "direct"
        ],
        "server": "local",
        "disable_cache": false,
        "rewrite_ttl": 100
      },
      {
        "type": "logical",
        "mode": "and",
        "rules": [],
        "server": "local",
        "disable_cache": false,
        "rewrite_ttl": 100
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
    (`source_geoip` || `source_ip_cidr`) &&  
    (`source_port` || `source_port_range`) &&  
    `other fields`

#### inbound

Tags of [Inbound](/configuration/inbound).

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

Match geosite.

#### source_geoip

Match source geoip.

#### source_ip_cidr

Match source ip cidr.

#### source_port

Match source port.

#### source_port_range

Match source port range.

#### port

Match port.

#### port_range

Match port range.

#### process_name

!!! error ""

    Only supported on Linux, Windows, and macOS.

Match process name.

#### process_path

!!! error ""

    Only supported on Linux, Windows, and macOS.

Match process path.

#### package_name

Match android package name.

#### user

!!! error ""

    Only supported on Linux.

Match user name.

#### user_id

!!! error ""

    Only supported on Linux.

Match user id.

#### clash_mode

Match Clash mode.

#### invert

Invert match result.

#### outbound

Match outbound.

`any` can be used as a value to match any outbound.

#### server

==Required==

Tag of the target dns server.

#### disable_cache

Disable cache and save cache in this query.

#### rewrite_ttl

Rewrite TTL in DNS responses.

### Logical Fields

#### type

`logical`

#### mode

`and` or `or`

#### rules

Included default rules.