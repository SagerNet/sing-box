### Structure

```json
{
  "type": "shadowtls",
  "tag": "st-in",

  ... // Listen Fields

  "version": 3,
  "password": "fuck me till the daylight",
  "handshake": {
    "server": "google.com",
    "server_port": 443,
    
    ... // Dial Fields
  }
}
```

### Listen Fields

See [Listen Fields](/configuration/shared/listen) for details.

### Fields

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

#### handshake

==Required==

Handshake server address and [Dial options](/configuration/shared/dial).