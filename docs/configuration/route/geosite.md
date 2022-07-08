# geosite

### Structure

```json
{
  "route": {
    "geosite": {
      "path": "",
      "download_url": "",
      "download_detour": ""
    }
  }
}
```

### Fields

#### path

The path to the sing-geosite database.

`geosite.db` will be used if empty.

#### download_url

The download URL of the sing-geoip database.

Default is `https://github.com/SagerNet/sing-geosite/releases/latest/download/geosite.db`.

#### download_detour

The ag of the outbound to download the database.

Default outbound will be used if empty.