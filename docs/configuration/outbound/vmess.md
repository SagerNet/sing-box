### Structure

```json
{
  "outbounds": [
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
      "multiplex": {},
      "transport": {},

      "detour": "upstream-out",
      "bind_interface": "en0",
      "bind_address": "0.0.0.0",
      "routing_mark": 1234,
      "reuse_addr": false,
      "connect_timeout": "5s",
      "tcp_fast_open": false,
      "domain_strategy": "prefer_ipv6",
      "fallback_delay": "300ms"
    }
  ]
}
```

### VMess Fields

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

TLS configuration, see [TLS outbound structure](/configuration/shared/tls/#outbound).

#### multiplex

Multiplex configuration, see [Multiplex structure](/configuration/shared/multiplex).

#### transport

V2Ray Transport configuration, see [V2Ray Transport](/configuration/shared/v2ray-transport).

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