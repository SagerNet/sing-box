---
icon: material/new-box
---

# Route

!!! quote "Changes in sing-box 1.11.0"

    :material-plus: [default_network_strategy](#default_network_strategy)  
    :material-alert: [default_fallback_delay](#default_fallback_delay)

!!! quote "Changes in sing-box 1.8.0"

    :material-plus: [rule_set](#rule_set)  
    :material-delete-clock: [geoip](#geoip)  
    :material-delete-clock: [geosite](#geosite)

### Structure

```json
{
  "route": {
    "geoip": {},
    "geosite": {},
    "rules": [],
    "rule_set": [],
    "final": "",
    "auto_detect_interface": false,
    "override_android_vpn": false,
    "default_interface": "",
    "default_mark": 0,
    "default_network_strategy": "",
    "default_fallback_delay": ""
  }
}
```

### Fields

| Key       | Format                |
|-----------|-----------------------|
| `geoip`   | [GeoIP](./geoip/)     |
| `geosite` | [Geosite](./geosite/) |

#### rules

List of [Route Rule](./rule/)

#### rule_set

!!! question "Since sing-box 1.8.0"

List of [rule-set](/configuration/rule-set/)

#### final

Default outbound tag. the first outbound will be used if empty.

#### auto_detect_interface

!!! quote ""

    Only supported on Linux, Windows and macOS.

Bind outbound connections to the default NIC by default to prevent routing loops under tun.

Takes no effect if `outbound.bind_interface` is set.

#### override_android_vpn

!!! quote ""

    Only supported on Android.

Accept Android VPN as upstream NIC when `auto_detect_interface` enabled.

#### default_interface

!!! quote ""

    Only supported on Linux, Windows and macOS.

Bind outbound connections to the specified NIC by default to prevent routing loops under tun.

Takes no effect if `auto_detect_interface` is set.

#### default_mark

!!! quote ""

    Only supported on Linux.

Set routing mark by default.

Takes no effect if `outbound.routing_mark` is set.

#### default_network_strategy

!!! quote ""

    Only supported in graphical clients on Android and iOS with `auto_detect_interface` enabled.

Strategy for selecting network interfaces.

Takes no effect if `outbound.bind_interface`, `outbound.inet4_bind_address` or `outbound.inet6_bind_address` is set.

Can be overrides by `outbound.network_strategy`.

Conflicts with `default_interface`.

See [Dial Fields](/configuration/shared/dial/#network_strategy) for available values.

#### default_fallback_delay

!!! quote ""

    Only supported in graphical clients on Android and iOS with `auto_detect_interface` enabled and `network_strategy` set.

See [Dial Fields](/configuration/shared/dial/#fallback_delay) for details.