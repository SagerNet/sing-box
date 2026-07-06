---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.14.0"

    :material-alert-decagram: [servers](#servers)

!!! question "Since sing-box 1.12.0"

# SSM API

SSM API service is a RESTful API server for managing multi-user inbounds.

See https://github.com/Shadowsocks-NET/shadowsocks-specs/blob/main/2023-1-shadowsocks-server-management-api-v1.md

### Structure

```json
{
  "type": "ssm-api",
  
  ... // Listen Fields
  
  "servers": {},
  "cache_path": "",
  "tls": {}
}
```

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.

### Fields

#### servers

==Required==

A mapping Object from HTTP endpoints to inbound tags.

Supported inbounds:

* [Shadowsocks](/configuration/inbound/shadowsocks) (multi-user)
* [HTTP](/configuration/inbound/http)
* [Mixed](/configuration/inbound/mixed)
* [SOCKS](/configuration/inbound/socks)
* [Naive](/configuration/inbound/naive)
* [Trojan](/configuration/inbound/trojan)
* [VMess](/configuration/inbound/vmess)
* [AnyTLS](/configuration/inbound/anytls)
* [Hysteria](/configuration/inbound/hysteria)
* [Hysteria2](/configuration/inbound/hysteria2)

Selected Shadowsocks inbounds must be configured with [managed](/configuration/inbound/shadowsocks#managed) enabled.

Since the SSM API user object carries a single credential, for VMess inbounds
the user password is the UUID, and managed users always use `alterId` `0`.

VLESS and TUIC inbounds are not supported, as their users cannot be
represented by a single credential (`flow` and `uuid` + `password` respectively).

Example:

```json
{
  "servers": {
    "/": "ss-in"
  }
}
```

#### cache_path

If set, when the server is about to stop, traffic and user state will be saved to the specified JSON file
to be restored on the next startup.

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#inbound).
