---
icon: material/new-box
---

!!! question "Since sing-box 1.12.0"

# SSM API

SSM API service is a RESTful API server for managing Shadowsocks servers.

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

A mapping Object from HTTP endpoints to [Shadowsocks Inbound](/configuration/inbound/shadowsocks) tags.

Selected Shadowsocks inbounds must be configured with [managed](/configuration/inbound/shadowsocks#managed) enabled.

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
