### Structure

```json
{
  "type": "shadowtls",
  "tag": "st-out",
  
  "server": "127.0.0.1",
  "server_port": 1080,
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

#### tls

==Required==

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

### Dial Fields

See [Dial Fields](/configuration/shared/dial) for details.
