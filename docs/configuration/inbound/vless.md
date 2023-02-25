### Structure

```json
{
  "type": "vless",
  "tag": "vless-in",

  ... // Listen Fields

  "users": [
    {
      "name": "sekai",
      "uuid": "bf000d23-0752-40b4-affe-68f7707a9661"
    }
  ],
  "tls": {},
  "transport": {}
}
```

### Listen Fields

See [Listen Fields](/configuration/shared/listen) for details.

### Fields

#### users

==Required==

VLESS users.

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#inbound).

#### transport

V2Ray Transport configuration, see [V2Ray Transport](/configuration/shared/v2ray-transport).
