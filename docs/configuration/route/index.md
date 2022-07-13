### Structure

```json
{
  "route": {
    "geoip": {},
    "geosite": {},
    "rules": [],
    "final": "",
    "auto_detect_interface": false
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