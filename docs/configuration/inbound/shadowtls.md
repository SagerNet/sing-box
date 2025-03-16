---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.12.0"

    :material-plus: [wildcard_sni](#wildcard_sni)

### Structure

```json
{
  "type": "shadowtls",
  "tag": "st-in",

  ... // Listen Fields

  "version": 3,
  "password": "fuck me till the daylight",
  "users": [
    {
      "name": "sekai",
      "password": "8JCsPssfgS8tiRwiMlhARg=="
    }
  ],
  "handshake": {
    "server": "google.com",
    "server_port": 443,
    
    ... // Dial Fields
  },
  "handshake_for_server_name": {
    "example.com": {
      "server": "example.com",
      "server_port": 443,

      ... // Dial Fields
    }
  },
  "strict_mode": false,
  "wildcard_sni": ""
}
```

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.

### Fields

#### version

ShadowTLS protocol version.

| Value         | Protocol Version                                                                        |
|---------------|-----------------------------------------------------------------------------------------|
| `1` (default) | [ShadowTLS v1](https://github.com/ihciah/shadow-tls/blob/master/docs/protocol-en.md#v1) |
| `2`           | [ShadowTLS v2](https://github.com/ihciah/shadow-tls/blob/master/docs/protocol-en.md#v2) |
| `3`           | [ShadowTLS v3](https://github.com/ihciah/shadow-tls/blob/master/docs/protocol-v3-en.md) |

#### password

ShadowTLS password.

Only available in the ShadowTLS protocol 2.

#### users

ShadowTLS users.

Only available in the ShadowTLS protocol 3.

#### handshake

==Required==

When `wildcard_sni` is configured to `all`, the server address is optional.

Handshake server address and [Dial Fields](/configuration/shared/dial/).

#### handshake_for_server_name

Handshake server address and [Dial Fields](/configuration/shared/dial/) for specific server name.

Only available in the ShadowTLS protocol 2/3.

#### strict_mode

ShadowTLS strict mode.

Only available in the ShadowTLS protocol 3.

#### wildcard_sni

!!! question "Since sing-box 1.12.0"

ShadowTLS wildcard SNI mode.

Available values are:

* `off`: (default) Disabled.
* `authed`: Authenticated connections will have their destination overwritten to `(servername):443`
* `all`: All connections will have their destination overwritten to `(servername):443`

Additionally, connections matching `handshake_for_server_name` are not affected.

Only available in the ShadowTLS protocol 3.
