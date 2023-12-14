### Structure

```json
{
  "type": "hysteria",
  "tag": "hysteria-in",
  
  ... // Listen Fields

  "up": "100 Mbps",
  "up_mbps": 100,
  "down": "100 Mbps",
  "down_mbps": 100,
  "obfs": "fuck me till the daylight",

  "users": [
    {
      "name": "sekai",
      "auth": "",
      "auth_str": "password"
    }
  ],
  
  "recv_window_conn": 0,
  "recv_window_client": 0,
  "max_conn_client": 0,
  "disable_mtu_discovery": false,
  "tls": {}
}
```

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.

### Fields

#### up, down

==Required==

Format: `[Integer] [Unit]` e.g. `100 Mbps, 640 KBps, 2 Gbps`

Supported units (case sensitive, b = bits, B = bytes, 8b=1B):

    bps (bits per second)
    Bps (bytes per second)
    Kbps (kilobits per second)
    KBps (kilobytes per second)
    Mbps (megabits per second)
    MBps (megabytes per second)
    Gbps (gigabits per second)
    GBps (gigabytes per second)
    Tbps (terabits per second)
    TBps (terabytes per second)

#### up_mbps, down_mbps

==Required==

`up, down` in Mbps.

#### obfs

Obfuscated password.

#### users

Hysteria users

#### users.auth

Authentication password, in base64.

#### users.auth_str

Authentication password.

#### recv_window_conn

The QUIC stream-level flow control window for receiving data.

`15728640 (15 MB/s)` will be used if empty.

#### recv_window_client

The QUIC connection-level flow control window for receiving data.

`67108864 (64 MB/s)` will be used if empty.

#### max_conn_client

The maximum number of QUIC concurrent bidirectional streams that a peer is allowed to open.

`1024` will be used if empty.

#### disable_mtu_discovery

Disables Path MTU Discovery (RFC 8899). Packets will then be at most 1252 (IPv4) / 1232 (IPv6) bytes in size.

Force enabled on for systems other than Linux and Windows (according to upstream).

#### tls

==Required==

TLS configuration, see [TLS](/configuration/shared/tls/#inbound).