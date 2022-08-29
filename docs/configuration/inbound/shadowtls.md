### Structure

```json
{
  "inbounds": [
    {
      "type": "shadowtls",
      "tag": "st-in",
      "listen": "::",
      "listen_port": 443,
      "tcp_fast_open": false,
      "sniff": false,
      "sniff_override_destination": false,
      "domain_strategy": "prefer_ipv6",
      "udp_timeout": 300,
      "proxy_protocol": false,
      
      "handshake": {
        "server": "google.com",
        "server_port": 443,

        "detour": "upstream-out",
        "bind_interface": "en0",
        "bind_address": "0.0.0.0",
        "routing_mark": 1234,
        "reuse_addr": false,
        "connect_timeout": "5s",
        "tcp_fast_open": false,
        "fallback_delay": "300ms"
      }
    }
  ]
}
```

### ShadowTLS Fields

#### handshake

==Required==

Address and port of handshake destination.

##### Dial Fields

###### detour

The tag of the upstream outbound.

Other dial fields will be ignored when enabled.

###### bind_interface

The network interface to bind to.

###### bind_address

The address to bind to.

###### routing_mark

!!! error ""

    Only supported on Linux.

Set netfilter routing mark.

###### reuse_addr

Reuse listener address.

###### connect_timeout

Connect timeout, in golang's Duration format.

A duration string is a possibly signed sequence of
decimal numbers, each with optional fraction and a unit suffix,
such as "300ms", "-1.5h" or "2h45m".
Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".

###### domain_strategy

One of `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`.

If set, the server domain name will be resolved to IP before connecting.

`dns.strategy` will be used if empty.

###### fallback_delay

The length of time to wait before spawning a RFC 6555 Fast Fallback connection.
That is, is the amount of time to wait for IPv6 to succeed before assuming
that IPv6 is misconfigured and falling back to IPv4 if `prefer_ipv4` is set.
If zero, a default delay of 300ms is used.

Only take effect when `domain_strategy` is `prefer_ipv4` or `prefer_ipv6`.

### Listen Fields

#### listen

==Required==

Listen address.

#### listen_port

==Required==

Listen port.

#### tcp_fast_open

Enable tcp fast open for listener.

#### sniff

Enable sniffing.

See [Protocol Sniff](/configuration/route/sniff/) for details.

#### sniff_override_destination

Override the connection destination address with the sniffed domain.

If the domain name is invalid (like tor), this will not work.

#### domain_strategy

One of `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`.

If set, the requested domain name will be resolved to IP before routing.

If `sniff_override_destination` is in effect, its value will be taken as a fallback.

#### proxy_protocol

Parse [Proxy Protocol](https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt) in the connection header.