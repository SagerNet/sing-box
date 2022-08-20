### Structure

```json
{
  "outbounds": [
    {
      "type": "tor",
      "tag": "tor-out",
      
      "executable_path": "/usr/bin/tor",
      "extra_args": [],
      "data_directory": "$HOME/.cache/tor",
      "torrc": {
        "ClientOnly": 1
      },
      
      "detour": "upstream-out",
      "bind_interface": "en0",
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

!!! info ""

    Embedded tor is not included by default, see [Installation](/#Installation).

### Tor Fields

#### executable_path

The path to the Tor executable.

Embedded Tor will be ignored if set.

#### extra_args

List of extra arguments passed to the Tor instance when started.

#### data_directory

==Recommended==

The data directory of Tor.

Each start will be very slow if not specified.

#### torrc

Map of torrc options.

See [tor(1)](https://linux.die.net/man/1/tor)

### Dial Fields

#### detour

The tag of the upstream outbound.

Other dial fields will be ignored when enabled.

#### bind_interface

The network interface to bind to.

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