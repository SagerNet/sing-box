### Structure

```json
{
  "outbounds": [
    {
      "type": "hysteria",
      "tag": "hysteria-out",
      
      "server": "127.0.0.1",
      "server_port": 1080,

      "up": "100 Mbps",
      "up_mbps": 100,
      "down": "100 Mbps",
      "down_mbps": 100,
      "obfs": "fuck me till the daylight",
      "auth": "",
      "auth_str": "password",
      "recv_window_conn": 0,
      "recv_window": 0,
      "disable_mtu_discovery": false,
      "network": "tcp",
      "tls": {},
      
      "detour": "upstream-out",
      "bind_interface": "en0",
      "bind_address": "0.0.0.0",
      "routing_mark": 1234,
      "reuse_addr": false,
      "connect_timeout": "5s",
      "domain_strategy": "prefer_ipv6",
      "fallback_delay": "300ms"
    }
  ]
}
```

!!! warning ""

    QUIC, which is required by hysteria is not included by default, see [Installation](/#installation).

### Hysteria Fields

#### server

==Required==

The server address.

#### server_port

==Required==

The server port.

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

#### recv_window_conn

The QUIC stream-level flow control window for receiving data.

`15728640 (15 MB/s)` will be used if empty.

#### recv_window

The QUIC connection-level flow control window for receiving data.

`67108864 (64 MB/s)` will be used if empty.

#### disable_mtu_discovery

Disables Path MTU Discovery (RFC 8899). Packets will then be at most 1252 (IPv4) / 1232 (IPv6) bytes in size.

Force enabled on for systems other than Linux and Windows (according to upstream).

#### tls

==Required==

TLS configuration, see [TLS outbound structure](/configuration/shared/tls/#outbound-structure).

#### network

Enabled network

One of `tcp` `udp`.

Both is enabled by default.

### Dial Fields

#### detour

The tag of the upstream outbound.

Other dial fields will be ignored when enabled.

#### bind_interface

The network interface to bind to.

#### bind_address

The address to bind to.

#### routing_mark

!!! error ""

    Linux only

The iptables routing mark.

#### reuse_addr

Reuse listener address.

#### connect_timeout

Connect timeout, in golang's Duration format.

A duration string is a possibly signed sequence of
decimal numbers, each with optional fraction and a unit suffix,
such as "300ms", "-1.5h" or "2h45m".
Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".

#### domain_strategy

One of `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`.

If set, the server domain name will be resolved to IP before connecting.

`dns.strategy` will be used if empty.

#### fallback_delay

The length of time to wait before spawning a RFC 6555 Fast Fallback connection.
That is, is the amount of time to wait for IPv6 to succeed before assuming
that IPv6 is misconfigured and falling back to IPv4 if `prefer_ipv4` is set.
If zero, a default delay of 300ms is used.

Only take effect when `domain_strategy` is `prefer_ipv4` or `prefer_ipv6`.