### Structure

```json
{
  "experimental": {
    "clash_api": {
      "external_controller": "127.0.0.1:9090",
      "external_ui": "folder",
      "secret": ""
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

RESTful web API listening address. Disabled if empty.

#### external_ui

A relative path to the configuration directory or an absolute path to a
directory in which you put some static web resource. sing-box will then
serve it at `http://{{external-controller}}/ui`.

#### secret

Secret for the RESTful API (optional)
Authenticate by spedifying HTTP header `Authorization: Bearer ${secret}`
ALWAYS set a secret if RESTful API is listening on 0.0.0.0