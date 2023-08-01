### Structure

```json
{
  "type": "tuic",
  "tag": "tuic-out",
  
  "server": "127.0.0.1",
  "server_port": 1080,
  "uuid": "2DD61D93-75D8-4DA4-AC0E-6AECE7EAC365",
  "password": "hello",
  "congestion_control": "cubic",
  "udp_relay_mode": "native",
  "zero_rtt_handshake": false,
  "heartbeat": "10s",
  "network": "tcp",
  "tls": {},
  
  ... // Dial Fields
}
```

!!! warning ""

    QUIC, which is required by TUIC is not included by default, see [Installation](/#installation).

### Fields

#### server

==Required==

The server address.

#### server_port

==Required==

The server port.

#### uuid

==Required==

TUIC user uuid

#### password

TUIC user password

#### congestion_control

QUIC congestion control algorithm

One of: `cubic`, `new_reno`, `bbr`

`cubic` is used by default.

#### udp_relay_mode

UDP packet relay mode

| Mode   | Description                                                              |
|:-------|:-------------------------------------------------------------------------|
| native | native UDP characteristics                                               |
| quic   | lossless UDP relay using QUIC streams, additional overhead is introduced |

`native` is used by default.

#### network

Enabled network

One of `tcp` `udp`.

Both is enabled by default.

#### tls

==Required==

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

### Dial Fields

See [Dial Fields](/configuration/shared/dial) for details.
