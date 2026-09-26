---
icon: material/new-box
---

!!! question "Since sing-box 1.14.0"

# Hysteria Realm

Hysteria Realm is a rendezvous service for Hysteria2 NAT traversal.

Hysteria2 servers behind NAT register on the realm via the [`realm`](/configuration/inbound/hysteria2/#realm) inbound field, and clients connect to them through the realm via the [`realm`](/configuration/outbound/hysteria2/#realm) outbound field.

### Structure

```json
{
  "type": "hysteria-realm",

  ... // Listen Fields

  "tls": {},

  ... // HTTP2 Fields

  "users": [
    {
      "name": "",
      "token": "",
      "max_realms": 0
    }
  ]
}
```

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.

### HTTP2 Fields

See [HTTP2 Fields](/configuration/shared/http2/) for details.

### Fields

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#inbound).

When configured, the realm serves HTTP/2 over TLS; otherwise plain HTTP/1.1.

#### users

==Required==

Authorized users.

#### users.name

==Required==

Username, used in logs and as the quota key.

#### users.token

==Required==

Bearer token presented by Hysteria2 inbounds and outbounds via `Authorization: Bearer <token>`.

#### users.max_realms

Maximum number of realm slots this user may hold concurrently.
