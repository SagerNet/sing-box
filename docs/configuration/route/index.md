### Structure

```json
{
  "route": {
    "geoip": {},
    "geosite": {},
    "rules": [],
    "final": "",
    "auto_detect_interface": false,
    "default_interface": "en0",
    "default_mark": 233
  }
}
```

### Fields

| Key       | Format                       |
|-----------|------------------------------|
| `geoip`   | [GeoIP](./geoip)             |
| `geosite` | [Geosite](./geosite)         |
| `rules`   | List of [Route Rule](./rule) |

#### final

Default outbound tag. the first outbound will be used if empty.

#### auto_detect_interface

!!! error ""

    Linux and Windows only

Bind outbound connections to the default NIC by default to prevent routing loops under Tun.

Takes no effect if `outbound.bind_interface` is set.

#### default_interface

!!! error ""

    Linux and Windows only

Bind outbound connections to the specified NIC by default to prevent routing loops under Tun.

Takes no effect if `auto_detect_interface` is set.

#### default_mark

!!! error ""

    Linux only

Set iptables routing mark by default.

Takes no effect if `outbound.routing_mark` is set.