---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.11.0"

    :material-plus: [network_strategy](#network_strategy)  
    :material-alert: [fallback_delay](#fallback_delay)

### Structure

```json
{
  "detour": "upstream-out",
  "bind_interface": "en0",
  "inet4_bind_address": "0.0.0.0",
  "inet6_bind_address": "::",
  "routing_mark": 1234,
  "reuse_addr": false,
  "connect_timeout": "5s",
  "tcp_fast_open": false,
  "tcp_multi_path": false,
  "udp_fragment": false,
  "domain_strategy": "prefer_ipv6",
  "network_strategy": "default",
  "fallback_delay": "300ms"
}
```

### Fields

#### detour

The tag of the upstream outbound.

If enabled, all other fields will be ignored.

#### bind_interface

The network interface to bind to.

#### inet4_bind_address

The IPv4 address to bind to.

#### inet6_bind_address

The IPv6 address to bind to.

#### routing_mark

!!! quote ""

    Only supported on Linux.

Set netfilter routing mark.

#### reuse_addr

Reuse listener address.

#### tcp_fast_open

Enable TCP Fast Open.

#### tcp_multi_path

!!! warning ""

    Go 1.21 required.

Enable TCP Multi Path.

#### udp_fragment

Enable UDP fragmentation.

#### connect_timeout

Connect timeout, in golang's Duration format.

A duration string is a possibly signed sequence of
decimal numbers, each with optional fraction and a unit suffix,
such as "300ms", "-1.5h" or "2h45m".
Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".

#### domain_strategy

Available values: `prefer_ipv4`, `prefer_ipv6`, `ipv4_only`, `ipv6_only`.

If set, the requested domain name will be resolved to IP before connect.

| Outbound | Effected domains         | Fallback Value                            |
|----------|--------------------------|-------------------------------------------|
| `direct` | Domain in request        | Take `inbound.domain_strategy` if not set | 
| others   | Domain in server address | /                                         |

#### network_strategy

!!! question "Since sing-box 1.11.0"

!!! quote ""

    Only supported in graphical clients on Android and iOS with `auto_detect_interface` enabled.

Strategy for selecting network interfaces.

Available values:

- `default` (default): Connect to the default interface.
- `fallback`: Try all other interfaces when timeout.
- `hybrid`: Connect to all interfaces concurrently and choose the fastest one.
- `wifi`:  Prioritize WIFI, but try all other interfaces when unavailable or timeout.
- `cellular`: Prioritize Cellular, but try all other interfaces when unavailable or timeout.
- `ethernet`: Prioritize Ethernet, but try all other interfaces when unavailable or timeout.
- `wifi_only`: Connect to WIFI only.
- `cellular_only`: Connect to Cellular only.
- `ethernet_only`: Connect to Ethernet only.

For fallback strategies, when preferred interfaces fails or times out,
it will enter a 15s fast fallback state (upgraded to `hybrid`),
and exit immediately if recovers.

Conflicts with `bind_interface`, `inet4_bind_address` and `inet6_bind_address`.

#### fallback_delay

The length of time to wait before spawning a RFC 6555 Fast Fallback connection.

For `domain_strategy`, is the amount of time to wait for connection to succeed before assuming
that IPv4/IPv6 is misconfigured and falling back to other type of addresses.

For `network_strategy`, is the amount of time to wait for connection to succeed before falling
back to other interfaces.

Only take effect when `domain_strategy` or `network_strategy` is set.

`300ms` is used by default.
