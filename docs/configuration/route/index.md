### Structure

```json
{
  "route": {
    "geoip": {},
    "geosite": {},
    "rules": [],
    "final": ""
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