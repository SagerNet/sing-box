### Structure

```json
{
  "type": "hysteria2",
  "tag": "hy2-out",
  
  "server": "127.0.0.1",
  "server_port": 1080,
  "up_mbps": 100,
  "down_mbps": 100,
  "obfs": {
    "type": "salamander",
    "password": "cry_me_a_r1ver"
  },
  "password": "goofy_ahh_password",
  "network": "tcp",
  "tls": {},
  
  ... // Dial Fields
}
```

!!! warning ""

    QUIC, which is required by Hysteria2 is not included by default, see [Installation](/#installation).

### Fields

#### server

==Required==

The server address.

#### server_port

==Required==

The server port.

#### up_mbps, down_mbps

Max bandwidth, in Mbps.

If empty, the BBR congestion control algorithm will be used instead of Hysteria CC.

#### obfs.type

QUIC traffic obfuscator type, only available with `salamander`.

Disabled if empty.

#### obfs.password

QUIC traffic obfuscator password.

#### password

Authentication password.

#### network

Enabled network

One of `tcp` `udp`.

Both is enabled by default.

#### tls

==Required==

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

### Dial Fields

See [Dial Fields](/configuration/shared/dial) for details.
