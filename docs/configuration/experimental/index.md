# Experimental

### Structure

```json
{
  "experimental": {
    "clash_api": {
      "external_controller": "127.0.0.1:9090",
      "external_ui": "",
      "external_ui_download_url": "",
      "external_ui_download_detour": "",
      "secret": "",
      "default_mode": "",
      "store_mode": false,
      "store_selected": false,
      "store_fakeip": false,
      "cache_file": "",
      "cache_id": ""
    },
    "v2ray_api": {
      "listen": "127.0.0.1:8080",
      "stats": {
        "enabled": true,
        "inbounds": [
          "socks-in"
        ],
        "outbounds": [
          "proxy",
          "direct"
        ],
        "users": [
          "sekai"
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

#### external_ui_download_url

ZIP download URL for the external UI, will be used if the specified `external_ui` directory is empty.

`https://github.com/MetaCubeX/Yacd-meta/archive/gh-pages.zip` will be used if empty.

#### external_ui_download_detour

The tag of the outbound to download the external UI.

Default outbound will be used if empty.

#### secret

Secret for the RESTful API (optional)
Authenticate by spedifying HTTP header `Authorization: Bearer ${secret}`
ALWAYS set a secret if RESTful API is listening on 0.0.0.0

#### default_mode

Default mode in clash, `rule` will be used if empty.

This setting has no direct effect, but can be used in routing and DNS rules via the `clash_mode` rule item.

#### store_mode

Store Clash mode in cache file.

#### store_selected

!!! note ""

    The tag must be set for target outbounds.

Store selected outbound for the `Selector` outbound in cache file.

#### store_fakeip

Store fakeip in cache file.

#### cache_file

Cache file path, `cache.db` will be used if empty.

#### cache_id

Cache ID.

If not empty, `store_selected` will use a separate store keyed by it.

### V2Ray API Fields

!!! error ""

    V2Ray API is not included by default, see [Installation](/#installation).

#### listen

gRPC API listening address. V2Ray API will be disabled if empty.

#### stats

Traffic statistics service settings.

#### stats.enabled

Enable statistics service.

#### stats.inbounds

Inbound list to count traffic.

#### stats.outbounds

Outbound list to count traffic.

#### stats.users

User list to count traffic.