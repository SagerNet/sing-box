!!! error ""

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
  "endpoint_independent_nat": false,
  "stack": "system",
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

!!! note "Use with Android VPN"

    By default, VPN takes precedence over tun. To make tun go through VPN, enable `route.override_android_vpn`.

#### strict_route

*In Linux*:

Enforce strict routing rules when `auto_route` is enabled:

* Let unsupported network unreachable
* Route all connections to tun

It prevents address leaks and makes DNS hijacking work on Android and Linux with systemd-resolved, but your device will
not be accessible by others.

*In Windows*:

Use segmented `auto_route` routing settings, which may help if you're using a dial-up network.

#### inet4_route_address

Use custom routes instead of default when `auto_route` is enabled.

#### inet6_route_address

Use custom routes instead of default when `auto_route` is enabled.

#### endpoint_independent_nat

!!! info ""

    This item is only available on the gvisor stack, other stacks are endpoint-independent NAT by default.

Enable endpoint-independent NAT.

Performance may degrade slightly, so it is not recommended to enable on when it is not needed.

#### udp_timeout

UDP NAT expiration time in seconds, default is 300 (5 minutes).

#### stack

TCP/IP stack.

| Stack            | Description                                                                      | Status            |
|------------------|----------------------------------------------------------------------------------|-------------------|
| system (default) | Sometimes better performance                                                     | recommended       |
| gVisor           | Better compatibility, based on [google/gvisor](https://github.com/google/gvisor) | recommended       |
| LWIP             | Based on [eycorsican/go-tun2socks](https://github.com/eycorsican/go-tun2socks)   | upstream archived |

!!! warning ""

    gVisor and LWIP stacks is not included by default, see [Installation](/#installation).

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

See [Listen Fields](/configuration/shared/listen) for details.
