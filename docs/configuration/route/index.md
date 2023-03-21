# Route

### Structure

```json
{
  "route": {
    "geoip": {},
    "geosite": {},
    "ip_rules": [],
    "rules": [],
    "final": "",
    "auto_detect_interface": false,
    "override_android_vpn": false,
    "default_interface": "en0",
    "default_mark": 233
  }
}
```

### Fields

| Key        | Format                             |
|------------|------------------------------------|
| `geoip`    | [GeoIP](./geoip)                   |
| `geosite`  | [Geosite](./geosite)               |
| `ip_rules` | List of [IP Route Rule](./ip-rule) |
| `rules`    | List of [Route Rule](./rule)       |

#### final

Default outbound tag. the first outbound will be used if empty.

#### auto_detect_interface

!!! error ""

    Only supported on Linux, Windows and macOS.

Bind outbound connections to the default NIC by default to prevent routing loops under tun.

Takes no effect if `outbound.bind_interface` is set.

#### override_android_vpn

!!! error ""

    Only supported on Android.

Accept Android VPN as upstream NIC when `auto_detect_interface` enabled.

#### default_interface

!!! error ""

    Only supported on Linux, Windows and macOS.

Bind outbound connections to the specified NIC by default to prevent routing loops under tun.

Takes no effect if `auto_detect_interface` is set.

#### default_mark

!!! error ""

    Only supported on Linux.

Set routing mark by default.

Takes no effect if `outbound.routing_mark` is set.