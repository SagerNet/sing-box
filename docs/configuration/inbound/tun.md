!!! error ""

    Only supported on Linux, Windows and macOS.

### Structure

```json
{
  "inbounds": [
    {
      "type": "tun",
      "tag": "tun-in",
      
      "interface_name": "tun0",
      "inet4_address": "172.19.0.1/30",
      "inet6_address": "fdfe:dcba:9876::1/128",
      "mtu": 1500,
      "auto_route": true,
      "endpoint_independent_nat": false,
      "udp_timeout": 300,
      "stack": "gvisor",
      "include_uid": [
        0
      ],
      "include_uid_range": [
        [
          1000,
          99999
        ]
      ],
      "exclude_uid": [
        1000
      ],
      "exclude_uid_range": [
        1000,
        99999
      ],
      "include_android_user": [
        0,
        10
      ],
      "include_package": [
        "com.android.chrome"
      ],
      "exclude_package": [
        "com.android.captiveportallogin"
      ],
      
      "sniff": true,
      "sniff_override_destination": false,
      "domain_strategy": "prefer_ipv4"
    }
  ]
}
```

!!! note ""

    You can ignore the JSON Array [] tag when the content is only one item

!!! warning ""

    If tun is running in non-privileged mode, addresses and MTU will not be configured automatically, please make sure the settings are accurate.

### Tun Fields

#### interface_name

Virtual device name, automatically selected if empty.

#### inet4_address

==Required==

IPv4 prefix for the tun interface.

#### inet6_address

IPv6 prefix for the tun interface.

#### mtu

The maximum transmission unit.

#### auto_route

Set the default route to the Tun.

!!! error ""

    To avoid traffic loopback, set `route.auto_detect_interface` or `route.default_interface` or `outbound.bind_interface`

#### endpoint_independent_nat

Enable endpoint-independent NAT.

Performance may degrade slightly, so it is not recommended to enable on when it is not needed.

#### udp_timeout

UDP NAT expiration time in seconds, default is 300 (5 minutes).

#### stack

TCP/IP stack.

| Stack            | Upstream                                                              | Status            |
|------------------|-----------------------------------------------------------------------|-------------------|
| gVisor (default) | [google/gvisor](https://github.com/google/gvisor)                     | recommended       |
| LWIP             | [eycorsican/go-tun2socks](https://github.com/eycorsican/go-tun2socks) | upstream archived |

!!! warning ""

    The LWIP stack is not included by default, see [Installation](/#Installation).

#### include_uid

!!! error ""

    UID rules are only supported on Linux and require auto_route.

Limit users in route. Not limited by default.

#### include_uid_range

Limit users in route, but in range.

#### exclude_uid

Exclude users in route.

#### exclude_uid_range

Exclude users in route, but in range.

#### include_android_user

!!! error ""

    Android user and package rules are only supported on Android and require auto_route.

Limit android users in route.

| Common user  | ID  |
|--------------|-----|
| Main         | 0   |
| Work Profile | 10  |

#### include_package

Limit android packages in route.

#### exclude_package

Exclude android packages in route.

### Listen Fields

#### sniff

Enable sniffing.

See [Sniff](/configuration/route/sniff/) for details.

#### sniff_override_destination

Override the connection destination address with the sniffed domain.

If the domain name is invalid (like tor), this will not work.

#### domain_strategy

One of `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`.

If set, the requested domain name will be resolved to IP before routing.

If `sniff_override_destination` is in effect, its value will be taken as a fallback.