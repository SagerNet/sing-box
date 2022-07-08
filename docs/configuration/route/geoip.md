# geoip

### Structure

```json
{
  "route": {
    "geoip": {
      "path": "",
      "download_url": "",
      "download_detour": ""
    }
  }
}
```

### Fields

#### path

The path to the sing-geoip database.

`geoip.db` will be used if empty.

#### download_url

The download URL of the sing-geoip database.

Default is `https://github.com/SagerNet/sing-geoip/releases/latest/download/geoip.db`.

#### download_detour

The ag of the outbound to download the database.

Default outbound will be used if empty.