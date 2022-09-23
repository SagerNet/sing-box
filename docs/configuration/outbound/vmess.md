### Structure

```json
{
  "type": "vmess",
  "tag": "vmess-out",
  
  "server": "127.0.0.1",
  "server_port": 1080,
  "uuid": "bf000d23-0752-40b4-affe-68f7707a9661",
  "security": "auto",
  "alter_id": 0,
  "global_padding": false,
  "authenticated_length": true,
  "network": "tcp",
  "tls": {},
  "packet_encoding": "",
  "multiplex": {},
  "transport": {},

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

#### uuid

==Required==

The VMess user id.

#### security

Encryption methods:

* `auto`
* `none`
* `zero`
* `aes-128-gcm`
* `chancha20-poly1305`

Legacy encryption methods:

* `aes-128-ctr`

#### alter_id

| Alter ID | Description         |
|----------|---------------------|
| 0        | Use AEAD protocol   |
| 1        | Use legacy protocol |
| > 1      | Unused, same as 1   |

#### global_padding

Protocol parameter. Will waste traffic randomly if enabled (enabled by default in v2ray and cannot be disabled).

#### authenticated_length

Protocol parameter. Enable length block encryption.

#### network

Enabled network

One of `tcp` `udp`.

Both is enabled by default.

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

#### packet_encoding

| Encoding   | Description           |
|------------|-----------------------|
| (none)     | Disabled              |
| packetaddr | Supported by v2ray 5+ |
| xudp       | Supported by xray     |

#### multiplex

Multiplex configuration, see [Multiplex](/configuration/shared/multiplex).

#### transport

V2Ray Transport configuration, see [V2Ray Transport](/configuration/shared/v2ray-transport).

### Dial Fields

See [Dial Fields](/configuration/shared/dial) for details.
