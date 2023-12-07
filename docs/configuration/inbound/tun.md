---
icon: material/alert-decagram
---

!!! quote "Changes in sing-box 1.8.0"

    :material-plus: [gso](#gso)  
    :material-plus: [gso_max_size](#gso_max_size)  
    :material-alert-decagram: [stack](#stack)

!!! quote ""

    Only supported on Linux, Windows and macOS.

### Structure

```json
{
  "type": "tun",
  "tag": "tun-in",
  "interface_name": "tun0",
  "inet4_address": "172.19.0.1/30",
  "inet6_address": "fdfe:dcba:9876::1/126",
  "mtu": 9000,
  "gso": false,
  "gso_max_size": 65536,
  "auto_route": true,
  "strict_route": true,
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
  "endpoint_independent_nat": false,
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
    "1000-99999"
  ],
  "exclude_uid": [
    1000
  ],
  "exclude_uid_range": [
    "1000-99999"
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
      "server_port": 8080
    }
  },
  
  ... // Listen Fields
}
```

!!! note ""

    You can ignore the JSON Array [] tag when the content is only one item

!!! warning ""

    If tun is running in non-privileged mode, addresses and MTU will not be configured automatically, please make sure the settings are accurate.

### Fields

#### interface_name

Virtual device name, automatically selected if empty.

#### inet4_address

==Required==

IPv4 prefix for the tun interface.

#### inet6_address

IPv6 prefix for the tun interface.

#### mtu

The maximum transmission unit.

#### gso

!!! question "Since sing-box 1.8.0"

!!! quote ""

    Only supported on Linux.

Enable generic segmentation offload.

#### gso_max_size

!!! question "Since sing-box 1.8.0"

!!! quote ""

    Only supported on Linux.

Maximum GSO packet size.

`65536` is used by default.

#### auto_route

Set the default route to the Tun.

!!! quote ""

    To avoid traffic loopback, set `route.auto_detect_interface` or `route.default_interface` or `outbound.bind_interface`

!!! note "Use with Android VPN"

    By default, VPN takes precedence over tun. To make tun go through VPN, enable `route.override_android_vpn`.

#### strict_route

Enforce strict routing rules when `auto_route` is enabled:

*In Linux*:

* Let unsupported network unreachable
* Route all connections to tun

It prevents address leaks and makes DNS hijacking work on Android, but your device will not be accessible by others.

*In Windows*:

* Add firewall rules to prevent DNS leak caused by
  Windows' [ordinary multihomed DNS resolution behavior](https://learn.microsoft.com/en-us/previous-versions/windows/it-pro/windows-server-2008-R2-and-2008/dd197552%28v%3Dws.10%29)

It may prevent some applications (such as VirtualBox) from working properly in certain situations.

#### inet4_route_address

Use custom routes instead of default when `auto_route` is enabled.

#### inet6_route_address

Use custom routes instead of default when `auto_route` is enabled.

#### inet4_route_exclude_address

Exclude custom routes when `auto_route` is enabled.

#### inet6_route_exclude_address

Exclude custom routes when `auto_route` is enabled.

#### endpoint_independent_nat

!!! info ""

    This item is only available on the gvisor stack, other stacks are endpoint-independent NAT by default.

Enable endpoint-independent NAT.

Performance may degrade slightly, so it is not recommended to enable on when it is not needed.

#### udp_timeout

UDP NAT expiration time in seconds, default is 300 (5 minutes).

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

### Listen Fields

See [Listen Fields](/configuration/shared/listen) for details.
