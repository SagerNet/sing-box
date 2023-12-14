### Structure

```json
{
  "type": "shadowtls",
  "tag": "st-out",
  
  "server": "127.0.0.1",
  "server_port": 1080,
  "version": 3,
  "password": "fuck me till the daylight",
  "tls": {},

  ... // Dial Fields
}
```

### Fields

#### server

==Required==

The server address.

#### server_port

==Required==

The server port.

#### version

ShadowTLS protocol version.

| Value         | Protocol Version                                                                        |
|---------------|-----------------------------------------------------------------------------------------|
| `1` (default) | [ShadowTLS v1](https://github.com/ihciah/shadow-tls/blob/master/docs/protocol-en.md#v1) |
| `2`           | [ShadowTLS v2](https://github.com/ihciah/shadow-tls/blob/master/docs/protocol-en.md#v2) |
| `3`           | [ShadowTLS v3](https://github.com/ihciah/shadow-tls/blob/master/docs/protocol-v3-en.md) |

#### password

Set password.

Only available in the ShadowTLS v2/v3 protocol.

#### tls

==Required==

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
