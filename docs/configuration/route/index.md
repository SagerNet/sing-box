---
icon: material/alert-decagram
---

# Route

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
    "default_interface": "en0",
    "default_mark": 233
  }
}
```

### Fields

| Key       | Format               |
|-----------|----------------------|
| `geoip`   | [GeoIP](./geoip)     |
| `geosite` | [Geosite](./geosite) |

#### rules

List of [Route Rule](./rule)

#### rule_set

!!! question "Since sing-box 1.8.0"

List of [Rule Set](/configuration/rule-set)

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