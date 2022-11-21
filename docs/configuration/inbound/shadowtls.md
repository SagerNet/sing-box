### Structure

```json
{
  "type": "shadowtls",
  "tag": "st-in",

  ... // Listen Fields

  "version": 2,
  "password": "fuck me till the daylight",
  "fallback_after": 2,
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

#### password

Set password.

Only available in the ShadowTLS v2 protocol.


#### fallback_after

Packet count before perform fallback.

Default is 2.

Lowering this may prevent TLS 1.3 connections, but reduces the risk of being actively probed.

#### handshake

==Required==

Handshake server address and [Dial options](/configuration/shared/dial).