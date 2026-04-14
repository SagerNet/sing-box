---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.12.0"

    :material-plus: [server_ports](#server_ports)  
    :material-plus: [hop_interval](#hop_interval)

### Structure

```json
{
  "type": "hysteria",
  "tag": "hysteria-out",
  
  "server": "127.0.0.1",
  "server_port": 1080,
  "server_ports": [
    "2080:3000"
  ],
  "hop_interval": "",
  "up": "100 Mbps",
  "up_mbps": 100,
  "down": "100 Mbps",
  "down_mbps": 100,
  "obfs": "fuck me till the daylight",
  "auth": "",
  "auth_str": "password",
  "network": "",
  "tls": {},

  ... // QUIC Fields

  ... // Dial Fields

  // Deprecated

  "recv_window_conn": 0,
  "recv_window": 0,
  "disable_mtu_discovery": false
}
```

### Fields

#### server

==Required==

The server address.

#### server_port

==Required==

The server port.

#### server_ports

!!! question "Since sing-box 1.12.0"

Server port range list.

Conflicts with `server_port`.

#### hop_interval

!!! question "Since sing-box 1.12.0"

Port hopping interval.

`30s` is used by default.

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

#### auth

Authentication password, in base64.

#### auth_str

Authentication password.

#### network

Enabled network

One of `tcp` `udp`.

Both is enabled by default.

#### tls

==Required==

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

### QUIC Fields

See [QUIC Fields](/configuration/shared/quic/) for details.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.

### Deprecated Fields

#### recv_window_conn

!!! failure "Deprecated in sing-box 1.14.0"

    Use QUIC fields `stream_receive_window` instead.

#### recv_window

!!! failure "Deprecated in sing-box 1.14.0"

    Use QUIC fields `connection_receive_window` instead.

#### disable_mtu_discovery

!!! failure "Deprecated in sing-box 1.14.0"

    Use QUIC fields `disable_path_mtu_discovery` instead.
