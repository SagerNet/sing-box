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
  "masquerade": "",
  "tls": {}
}
```

!!! warning ""

    QUIC, which is required by Hysteria2 is not included by default, see [Installation](/#installation).

### Listen Fields

See [Listen Fields](/configuration/shared/listen) for details.

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

Commands the client to use the BBR flow control algorithm instead of Hysteria CC.

Conflict with `up_mbps` and `down_mbps`.

#### masquerade

HTTP3 server behavior when authentication fails.

| Scheme       | Example                 | Description        |
|--------------|-------------------------|--------------------|
| `file`       | `file:///var/www`       | As a file server   |
| `http/https` | `http://127.0.0.1:8080` | As a reverse proxy |

A 404 page will be returned if empty.

#### tls

==Required==

TLS configuration, see [TLS](/configuration/shared/tls/#inbound).