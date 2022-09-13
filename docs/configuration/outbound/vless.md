### Structure

```json
{
  "type": "vless",
  "tag": "vless-out",
  
  "server": "127.0.0.1",
  "server_port": 1080,
  "uuid": "bf000d23-0752-40b4-affe-68f7707a9661",
  "network": "tcp",
  "tls": {},
  "packet_encoding": "",
  "transport": {},

  ... // Dial Fields
}
```

!!! warning ""

    The VLESS protocol is architecturally coupled to v2ray and is unmaintained. This outbound is provided for compatibility purposes only.

### Fields

#### server

==Required==

The server address.

#### server_port

==Required==

The server port.

#### uuid

==Required==

The VLESS user id.

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

#### transport

V2Ray Transport configuration, see [V2Ray Transport](/configuration/shared/v2ray-transport).

### Dial Fields

See [Dial Fields](/configuration/shared/dial) for details.
