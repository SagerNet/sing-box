### Structure

```json
{
  "type": "vmess",
  "tag": "vmess-in",

  ... // Listen Fields

  "users": [
    {
      "name": "sekai",
      "uuid": "bf000d23-0752-40b4-affe-68f7707a9661",
      "alterId": 0
    }
  ],
  "tls": {},
  "multiplex": {},
  "transport": {}
}
```

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.

### Fields

#### users

==Required==

VMess users.

| Alter ID | Description             |
|----------|-------------------------|
| 0        | Disable legacy protocol |
| > 0      | Enable legacy protocol  |

!!! warning ""

    Legacy protocol support (VMess MD5 Authentication) is provided for compatibility purposes only, use of alterId > 1 is not recommended.

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#inbound).

#### multiplex

See [Multiplex](/configuration/shared/multiplex#inbound) for details.

#### transport

V2Ray Transport configuration, see [V2Ray Transport](/configuration/shared/v2ray-transport/).
