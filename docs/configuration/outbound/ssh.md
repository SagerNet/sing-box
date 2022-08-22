### Structure

```json
{
  "outbounds": [
    {
      "type": "ssh",
      "tag": "ssh-out",
     
      "server": "127.0.0.1",
      "server_port": 22,
      "user": "root",
      "password": "admin",
      "private_key": "",
      "private_key_path": "$HOME/.ssh/id_rsa",
      "private_key_passphrase": "",
      "host_key_algorithms": [],
      "client_version": "SSH-2.0-OpenSSH_7.4p1",
      
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

### SSH Fields

#### server

==Required==

Server address.

#### server_port

Server port. 22 will be used if empty.

#### user

SSH user, root will be used if empty.

#### password

Password.

#### private_key

Private key content.

#### private_key_path

Private key path.

#### private_key_passphrase

Private key passphrase.

#### host_key_algorithms

Host key algorithms.

#### client_version

Client version. Random version will be used if empty.

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
