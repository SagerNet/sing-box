---
icon: material/alert-decagram
---

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
  "masquerade": "", // or {}
  "brutal_debug": false
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

#### brutal_debug

Enable debug information logging for Hysteria Brutal CC.
