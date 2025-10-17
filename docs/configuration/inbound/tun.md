---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.13.0"

    :material-plus: [exclude_mptcp](#exclude_mptcp)

!!! quote "Changes in sing-box 1.12.0"

    :material-plus: [loopback_address](#loopback_address)

!!! quote "Changes in sing-box 1.11.0"

    :material-delete-alert: [gso](#gso)  
    :material-alert-decagram: [route_address_set](#stack)  
    :material-alert-decagram: [route_exclude_address_set](#stack)

!!! quote "Changes in sing-box 1.10.0"

    :material-plus: [address](#address)  
    :material-delete-clock: [inet4_address](#inet4_address)  
    :material-delete-clock: [inet6_address](#inet6_address)  
    :material-plus: [route_address](#route_address)  
    :material-delete-clock: [inet4_route_address](#inet4_route_address)  
    :material-delete-clock: [inet6_route_address](#inet6_route_address)  
    :material-plus: [route_exclude_address](#route_address)  
    :material-delete-clock: [inet4_route_exclude_address](#inet4_route_exclude_address)  
    :material-delete-clock: [inet6_route_exclude_address](#inet6_route_exclude_address)  
    :material-plus: [iproute2_table_index](#iproute2_table_index)  
    :material-plus: [iproute2_rule_index](#iproute2_table_index)  
    :material-plus: [auto_redirect](#auto_redirect)  
    :material-plus: [auto_redirect_input_mark](#auto_redirect_input_mark)  
    :material-plus: [auto_redirect_output_mark](#auto_redirect_output_mark)  
    :material-plus: [route_address_set](#route_address_set)  
    :material-plus: [route_exclude_address_set](#route_address_set)

!!! quote "Changes in sing-box 1.9.0"

    :material-plus: [platform.http_proxy.bypass_domain](#platformhttp_proxybypass_domain)  
    :material-plus: [platform.http_proxy.match_domain](#platformhttp_proxymatch_domain)  

!!! quote "Changes in sing-box 1.8.0"

    :material-plus: [gso](#gso)  
    :material-alert-decagram: [stack](#stack)

!!! quote ""

    Only supported on Linux, Windows and macOS.

### Structure

```json
{
  "type": "tun",
  "tag": "tun-in",
  "interface_name": "tun0",
  "address": [
    "172.18.0.1/30",
    "fdfe:dcba:9876::1/126"
  ],
  "mtu": 9000,
  "auto_route": true,
  "iproute2_table_index": 2022,
  "iproute2_rule_index": 9000,
  "auto_redirect": true,
  "auto_redirect_input_mark": "0x2023",
  "auto_redirect_output_mark": "0x2024",
  "exclude_mptcp": false,
  "loopback_address": [
    "10.7.0.1"
  ],
  "strict_route": true,
  "route_address": [
    "0.0.0.0/1",
    "128.0.0.0/1",
    "::/1",
    "8000::/1"
  ],
  "route_exclude_address": [
    "192.168.0.0/16",
    "fc00::/7"
  ],
  "route_address_set": [
    "geoip-cloudflare"
  ],
  "route_exclude_address_set": [
    "geoip-cn"
  ],
  "endpoint_independent_nat": false,
  "udp_timeout": "5m",
  "stack": "system",
  "include_interface": [
    "lan0"
  ],
  "exclude_interface": [
    "lan1"
  ],
  "include_uid": [
    0
  ],
  "include_uid_range": [
    "1000:99999"
  ],
  "exclude_uid": [
    1000
  ],
  "exclude_uid_range": [
    "1000:99999"
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
  "platform": {
    "http_proxy": {
      "enabled": false,
      "server": "127.0.0.1",
      "server_port": 8080,
      "bypass_domain": [],
      "match_domain": []
    }
  },
  // Deprecated
  "gso": false,
  "inet4_address": [
    "172.19.0.1/30"
  ],
  "inet6_address": [
    "fdfe:dcba:9876::1/126"
  ],
  "inet4_route_address": [
    "0.0.0.0/1",
    "128.0.0.0/1"
  ],
  "inet6_route_address": [
    "::/1",
    "8000::/1"
  ],
  "inet4_route_exclude_address": [
    "192.168.0.0/16"
  ],
  "inet6_route_exclude_address": [
    "fc00::/7"
  ],
  ...
  // Listen Fields
}
```

!!! note ""

    You can ignore the JSON Array [] tag when the content is only one item

!!! warning ""

    If tun is running in non-privileged mode, addresses and MTU will not be configured automatically, please make sure the settings are accurate.

### Fields

#### interface_name

Virtual device name, automatically selected if empty.

#### address

!!! question "Since sing-box 1.10.0"

IPv4 and IPv6 prefix for the tun interface.

#### inet4_address

!!! failure "Deprecated in sing-box 1.10.0"

    `inet4_address` is merged to `address` and will be removed in sing-box 1.12.0.

IPv4 prefix for the tun interface.

#### inet6_address

!!! failure "Deprecated in sing-box 1.10.0"

    `inet6_address` is merged to `address` and will be removed in sing-box 1.12.0.

IPv6 prefix for the tun interface.

#### mtu

The maximum transmission unit.

#### gso

!!! failure "Deprecated in sing-box 1.11.0"

    GSO has no advantages for transparent proxy scenarios, is deprecated and no longer works, and will be removed in sing-box 1.12.0.

!!! question "Since sing-box 1.8.0"

!!! quote ""

    Only supported on Linux with `auto_route` enabled.

Enable generic segmentation offload.

#### auto_route

Set the default route to the Tun.

!!! quote ""

    To avoid traffic loopback, set `route.auto_detect_interface` or `route.default_interface` or `outbound.bind_interface`

!!! note "Use with Android VPN"

    By default, VPN takes precedence over tun. To make tun go through VPN, enable `route.override_android_vpn`.

!!! note "Also enable `auto_redirect`"

    `auto_redirect` is always recommended on Linux, it provides better routing, higher performance (better than tproxy), and avoids conflicts between TUN and Docker bridge networks.

#### iproute2_table_index

!!! question "Since sing-box 1.10.0"

Linux iproute2 table index generated by `auto_route`.

`2022` is used by default.

#### iproute2_rule_index

!!! question "Since sing-box 1.10.0"

Linux iproute2 rule start index generated by `auto_route`.

`9000` is used by default.

#### auto_redirect

!!! question "Since sing-box 1.10.0"

!!! quote ""

    Only supported on Linux with `auto_route` enabled.

Improve TUN routing and performance using nftables.

`auto_redirect` is always recommended on Linux, it provides better routing,
higher performance (better than tproxy),
and avoids conflicts between TUN and Docker bridge networks.

Note that `auto_redirect` also works on Android, 
but due to the lack of `nftables` and `ip6tables`,
only simple IPv4 TCP forwarding is performed.
To share your VPN connection over hotspot or repeater on Android,
use [VPNHotspot](https://github.com/Mygod/VPNHotspot).

`auto_redirect` also automatically inserts compatibility rules
into the OpenWrt fw4 table, i.e. 
it will work on routers without any extra configuration.

Conflict with `route.default_mark` and `[dialOptions].routing_mark`.

#### auto_redirect_input_mark

!!! question "Since sing-box 1.10.0"

Connection input mark used by `auto_redirect`.

`0x2023` is used by default.

#### auto_redirect_output_mark

!!! question "Since sing-box 1.10.0"

Connection output mark used by `auto_redirect`.

`0x2024` is used by default.

#### exclude_mptcp

!!! question "Since sing-box 1.13.0"

!!! quote ""

    Only supported on Linux with nftables and requires `auto_route` and `auto_redirect` enabled.

MPTCP cannot be transparently proxied due to protocol limitations.

Such traffic is usually created by Apple systems.

When enabled, MPTCP connections will bypass sing-box and connect directly, otherwise, will be rejected to avoid errors by default.

#### loopback_address

!!! question "Since sing-box 1.12.0"

Loopback addresses make TCP connections to the specified address connect to the source address.

Setting option value to `10.7.0.1` achieves the same behavior as SideStore/StosVPN.

When `auto_redirect` is enabled, the same behavior can be achieved for LAN devices (not just local) as a gateway.

#### strict_route

Enforce strict routing rules when `auto_route` is enabled:

*In Linux*:

* Let unsupported network unreachable
* For legacy reasons, when neither `strict_route` nor `auto_redirect` are enabled, all ICMP traffic will not go through TUN.

*In Windows*:

* Let unsupported network unreachable
* prevent DNS leak caused by
  Windows' [ordinary multihomed DNS resolution behavior](https://learn.microsoft.com/en-us/previous-versions/windows/it-pro/windows-server-2008-R2-and-2008/dd197552%28v%3Dws.10%29)

It may prevent some Windows applications (such as VirtualBox) from working properly in certain situations.

#### route_address

!!! question "Since sing-box 1.10.0"

Use custom routes instead of default when `auto_route` is enabled.

#### inet4_route_address

!!! failure "Deprecated in sing-box 1.10.0"

`inet4_route_address` is deprecated and will be removed in sing-box 1.12.0, please use [route_address](#route_address)
instead.

Use custom routes instead of default when `auto_route` is enabled.

#### inet6_route_address

!!! failure "Deprecated in sing-box 1.10.0"

`inet6_route_address` is deprecated and will be removed in sing-box 1.12.0, please use [route_address](#route_address)
instead.

Use custom routes instead of default when `auto_route` is enabled.

#### route_exclude_address

!!! question "Since sing-box 1.10.0"

Exclude custom routes when `auto_route` is enabled.

#### inet4_route_exclude_address

!!! failure "Deprecated in sing-box 1.10.0"

`inet4_route_exclude_address` is deprecated and will be removed in sing-box 1.12.0, please
use [route_exclude_address](#route_exclude_address) instead.

Exclude custom routes when `auto_route` is enabled.

#### inet6_route_exclude_address

!!! failure "Deprecated in sing-box 1.10.0"

`inet6_route_exclude_address` is deprecated and will be removed in sing-box 1.12.0, please
use [route_exclude_address](#route_exclude_address) instead.

Exclude custom routes when `auto_route` is enabled.

#### route_address_set

=== "With `auto_redirect` enabled"

    !!! question "Since sing-box 1.10.0"

    !!! quote ""
    
        Only supported on Linux with nftables and requires `auto_route` and `auto_redirect` enabled.
    
    Add the destination IP CIDR rules in the specified rule-sets to the firewall.
    Unmatched traffic will bypass the sing-box routes.
    
    Conflict with `route.default_mark` and `[dialOptions].routing_mark`.

=== "Without `auto_redirect` enabled"

    !!! question "Since sing-box 1.11.0"
    
    Add the destination IP CIDR rules in the specified rule-sets to routes, equivalent to adding to `route_address`.
    Unmatched traffic will bypass the sing-box routes.

    Note that it **doesn't work on the Android graphical client** due to
    the Android VpnService not being able to handle a large number of routes (DeadSystemException),
    but otherwise it works fine on all command line clients and Apple platforms.

#### route_exclude_address_set

=== "With `auto_redirect` enabled"

    !!! question "Since sing-box 1.10.0"

    !!! quote ""

    Only supported on Linux with nftables and requires `auto_route` and `auto_redirect` enabled.

    Add the destination IP CIDR rules in the specified rule-sets to the firewall.
    Matched traffic will bypass the sing-box routes.

=== "Without `auto_redirect` enabled"

    !!! question "Since sing-box 1.11.0"
    
    Add the destination IP CIDR rules in the specified rule-sets to routes, equivalent to adding to `route_exclude_address`.
    Matched traffic will bypass the sing-box routes.

    Note that it **doesn't work on the Android graphical client** due to
    the Android VpnService not being able to handle a large number of routes (DeadSystemException),
    but otherwise it works fine on all command line clients and Apple platforms.

#### endpoint_independent_nat

!!! info ""

    This item is only available on the gvisor stack, other stacks are endpoint-independent NAT by default.

Enable endpoint-independent NAT.

Performance may degrade slightly, so it is not recommended to enable on when it is not needed.

#### udp_timeout

UDP NAT expiration time.

`5m` will be used by default.

#### stack

!!! quote "Changes in sing-box 1.8.0"

    :material-delete-alert: The legacy LWIP stack has been deprecated and removed.

TCP/IP stack.

| Stack    | Description                                                                                           | 
|----------|-------------------------------------------------------------------------------------------------------|
| `system` | Perform L3 to L4 translation using the system network stack                                           |
| `gvisor` | Perform L3 to L4 translation using [gVisor](https://github.com/google/gvisor)'s virtual network stack |
| `mixed`  | Mixed `system` TCP stack and `gvisor` UDP stack                                                       |

Defaults to the `mixed` stack if the gVisor build tag is enabled, otherwise defaults to the `system` stack.

#### include_interface

!!! quote ""

    Interface rules are only supported on Linux and require auto_route.

Limit interfaces in route. Not limited by default.

Conflict with `exclude_interface`.

#### exclude_interface

!!! warning ""

    When `strict_route` enabled, return traffic to excluded interfaces will not be automatically excluded, so add them as well (example: `br-lan` and `pppoe-wan`).

Exclude interfaces in route.

Conflict with `include_interface`.

#### include_uid

!!! quote ""

    UID rules are only supported on Linux and require auto_route.

Limit users in route. Not limited by default.

#### include_uid_range

Limit users in route, but in range.

#### exclude_uid

Exclude users in route.

#### exclude_uid_range

Exclude users in route, but in range.

#### include_android_user

!!! quote ""

    Android user and package rules are only supported on Android and require auto_route.

Limit android users in route.

| Common user  | ID |
|--------------|----|
| Main         | 0  |
| Work Profile | 10 |

#### include_package

Limit android packages in route.

#### exclude_package

Exclude android packages in route.

#### platform

Platform-specific settings, provided by client applications.

#### platform.http_proxy

System HTTP proxy settings.

#### platform.http_proxy.enabled

Enable system HTTP proxy.

#### platform.http_proxy.server

==Required==

HTTP proxy server address.

#### platform.http_proxy.server_port

==Required==

HTTP proxy server port.

#### platform.http_proxy.bypass_domain

!!! note ""

    On Apple platforms, `bypass_domain` items matches hostname **suffixes**.

Hostnames that bypass the HTTP proxy.

#### platform.http_proxy.match_domain

!!! quote ""

    Only supported in graphical clients on Apple platforms.

Hostnames that use the HTTP proxy.

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.
