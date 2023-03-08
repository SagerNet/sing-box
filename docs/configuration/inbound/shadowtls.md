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
  "strict_mode": false
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

ShadowTLS password.

Only available in the ShadowTLS protocol 2.


#### users

ShadowTLS users.

Only available in the ShadowTLS protocol 3.

#### handshake

==Required==

Handshake server address and [Dial options](/configuration/shared/dial).

#### handshake_for_server_name

Handshake server address and [Dial options](/configuration/shared/dial) for specific server name.

Only available in the ShadowTLS protocol 2/3.

#### strict_mode

ShadowTLS strict mode.

Only available in the ShadowTLS protocol 3.
