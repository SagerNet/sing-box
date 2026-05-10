---
icon: material/alert-decagram
---

!!! quote "Changes in sing-box 1.14.0"

    :material-plus: [bbr_profile](#bbr_profile)  
    :material-plus: [realm](#realm)

!!! quote "Changes in sing-box 1.11.0"

    :material-alert: [masquerade](#masquerade)  
    :material-alert: [ignore_client_bandwidth](#ignore_client_bandwidth)

### Structure

```json
{
  "type": "hysteria2",
  "tag": "hy2-in",
  
  ... // Listen Fields

  "up_mbps": 100,
  "down_mbps": 100,
  "obfs": {
    "type": "salamander",
    "password": "cry_me_a_r1ver"
  },
  "users": [
    {
      "name": "tobyxdd",
      "password": "goofy_ahh_password"
    }
  ],
  "ignore_client_bandwidth": false,
  "tls": {},

  ... // QUIC Fields

  "masquerade": "", // or {}
  "bbr_profile": "",
  "brutal_debug": false,
  "realm": {
    "server_url": "https://realm.example.com",
    "token": "",
    "realm_id": "",
    "stun_servers": [],
    "http_client": {}
  }
}
```

!!! warning "Difference from official Hysteria2"

    The official program supports an authentication method called **userpass**,
    which essentially uses a combination of `<username>:<password>` as the actual password,
    while sing-box does not provide this alias.
    To use sing-box with the official program, you need to fill in that combination as the actual password.

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.

### Fields

#### up_mbps, down_mbps

Max bandwidth, in Mbps.

Not limited if empty.

Conflict with `ignore_client_bandwidth`.

#### obfs.type

QUIC traffic obfuscator type, only available with `salamander`.

Disabled if empty.

#### obfs.password

QUIC traffic obfuscator password.

#### users

Hysteria2 users

#### users.password

Authentication password

#### ignore_client_bandwidth

*When `up_mbps` and `down_mbps` are not set*:

Commands clients to use the BBR CC instead of Hysteria CC.

*When `up_mbps` and `down_mbps` are set*:

Deny clients to use the BBR CC.

#### tls

==Required==

TLS configuration, see [TLS](/configuration/shared/tls/#inbound).

### QUIC Fields

See [QUIC Fields](/configuration/shared/quic/) for details.

#### masquerade

HTTP3 server behavior (URL string configuration) when authentication fails.

| Scheme       | Example                 | Description        |
|--------------|-------------------------|--------------------|
| `file`       | `file:///var/www`       | As a file server   |
| `http/https` | `http://127.0.0.1:8080` | As a reverse proxy |

Conflict with `masquerade.type`.

A 404 page will be returned if masquerade is not configured.

#### masquerade.type

HTTP3 server behavior (Object configuration) when authentication fails.

| Type     | Description                 | Fields                              |
|----------|-----------------------------|-------------------------------------|
| `file`   | As a file server            | `directory`                         |
| `proxy`  | As a reverse proxy          | `url`, `rewrite_host`               |
| `string` | Reply with a fixed response | `status_code`, `headers`, `content` |

Conflict with `masquerade`.

A 404 page will be returned if masquerade is not configured.

#### masquerade.directory

File server root directory.

#### masquerade.url

Reverse proxy target URL.

#### masquerade.rewrite_host

Rewrite the `Host` header to the target URL.

#### masquerade.status_code

Fixed response status code.

#### masquerade.headers

Fixed response headers.

#### masquerade.content

Fixed response content.

#### bbr_profile

!!! question "Since sing-box 1.14.0"

BBR congestion control algorithm profile, one of `conservative` `standard` `aggressive`.

`standard` is used by default.

#### brutal_debug

Enable debug information logging for Hysteria Brutal CC.

#### realm

!!! question "Since sing-box 1.14.0"

Register this inbound to a Hysteria Realm rendezvous service to enable NAT traversal.

The inbound discovers its public addresses via STUN, registers them on the realm, and uses UDP hole-punching to accept incoming clients without a publicly reachable listen address.

See [Hysteria Realm](/configuration/service/hysteria-realm/) for the rendezvous service.

#### realm.server_url

==Required==

Realm rendezvous service URL.

#### realm.token

Bearer token for the realm. Must match one of `users[].token` configured on the realm.

#### realm.realm_id

==Required==

Slot identifier on the realm.

1–64 characters, must match `^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`.

Outbounds must use the same `realm_id` to find this server.

#### realm.stun_servers

==Required==

List of STUN servers (`host` or `host:port`) used to discover public addresses.

Port defaults to `3478`.

#### realm.http_client

HTTP client used to talk to the realm.

See [HTTP Client](/configuration/shared/http-client/) for details.
