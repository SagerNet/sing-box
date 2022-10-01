# Experimental

### Structure

```json
{
  "experimental": {
    "clash_api": {
      "external_controller": "127.0.0.1:9090",
      "external_ui": "folder",
      "secret": "",
      "direct_io": false,
      "default_mode": "rule",
      "store_selected": false,
      "cache_file": "cache.db"
    },
    "v2ray_api": {
      "listen": "127.0.0.1:8080",
      "stats": {
        "enabled": true,
        "direct_io": false,
        "inbounds": [
          "socks-in"
        ],
        "outbounds": [
          "proxy",
          "direct"
        ]
      }
    }
  }
}
```

!!! note ""

    Traffic statistics and connection management can degrade performance.

### Clash API Fields

!!! error ""

    Clash API is not included by default, see [Installation](/#installation).

#### external_controller

RESTful web API listening address. Clash API will be disabled if empty.

#### external_ui

A relative path to the configuration directory or an absolute path to a
directory in which you put some static web resource. sing-box will then
serve it at `http://{{external-controller}}/ui`.

#### secret

Secret for the RESTful API (optional)
Authenticate by spedifying HTTP header `Authorization: Bearer ${secret}`
ALWAYS set a secret if RESTful API is listening on 0.0.0.0

#### direct_io

Allows lossless relays like splice without real-time traffic reporting.

#### default_mode

Default mode in clash, `rule` will be used if empty.

This setting has no direct effect, but can be used in routing and DNS rules via the `clash_mode` rule item.

#### store_selected

!!! note ""

    The tag must be set for target outbounds.

Store selected outbound for the `Selector` outbound in cache file.

#### cache_file

Cache file path, `cache.db` will be used if empty.

### V2Ray API Fields

!!! error ""

    V2Ray API is not included by default, see [Installation](/#installation).

#### listen

gRPC API listening address. V2Ray API will be disabled if empty.

#### stats

Traffic statistics service settings.

#### stats.enabled

Enable statistics service.

#### stats.direct_io

Allows lossless relays like splice without real-time traffic reporting.

#### stats.inbounds

Inbound list to count traffic.

#### stats.outbounds

Outbound list to count traffic.
