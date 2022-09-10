# Experimental

### Structure

```json
{
  "experimental": {
    "clash_api": {
      "external_controller": "127.0.0.1:9090",
      "external_ui": "folder",
      "secret": "",
      "default_mode": "rule",
      "store_selected": false,
      "cache_file": "cache.db"
    }
  }
}
```

### Clash API Fields

!!! error ""

    Clash API is not included by default, see [Installation](/#installation).

!!! note ""

    Traffic statistics and connection management will disable TCP splice in linux and reduce performance, use at your own risk.

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

#### default_mode

Default mode in clash, `rule` will be used if empty.

This setting has no direct effect, but can be used in routing and DNS rules via the `clash_mode` rule item.

#### store_selected

!!! note ""

    The tag must be set for target outbounds.

Store selected outbound for the `Selector` outbound in cache file.

#### cache_file

Cache file path, `cache.db` will be used if empty.