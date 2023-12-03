### Structure

```json
{
  "type": "naive",
  "tag": "naive-in",
  "network": "udp",

  ... // Listen Fields

  "users": [
    {
      "username": "sekai",
      "password": "password"
    }
  ],
  "tls": {}
}
```

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.

### Fields

#### network

Listen network, one of `tcp` `udp`.

Both if empty.

#### users

==Required==

Naive users.

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#inbound).