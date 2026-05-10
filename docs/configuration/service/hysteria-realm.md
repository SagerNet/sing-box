---
icon: material/new-box
---

!!! question "Since sing-box 1.14.0"

# Hysteria Realm

Hysteria Realm is a rendezvous service for Hysteria2 NAT traversal.

A Hysteria2 server behind NAT registers its STUN-discovered public addresses to a stable realm endpoint; clients query the realm to learn the server's current addresses and perform UDP hole-punching to establish a direct QUIC connection.

The realm only carries control-plane signaling. Once hole-punching succeeds, all proxy traffic flows directly between client and server.

### Structure

```json
{
  "type": "hysteria-realm",

  ... // Listen Fields

  "tls": {},
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
