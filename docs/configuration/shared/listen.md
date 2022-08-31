### Structure

```json
{
  "listen": "::",
  "listen_port": 5353,
  "tcp_fast_open": false,
  "sniff": false,
  "sniff_override_destination": false,
  "domain_strategy": "prefer_ipv6",
  "udp_timeout": 300,
  "proxy_protocol": false,
  "detour": "another-in"
}
```

### Fields

| Field            | Available Context                                                 |
|------------------|-------------------------------------------------------------------|
| `listen`         | Needs to listen on TCP or UDP.                                    |
| `listen_port`    | Needs to listen on TCP or UDP.                                    |
| `tcp_fast_open`  | Needs to listen on TCP.                                           |
| `udp_timeout`    | Needs to assemble UDP connections, currently Tun and Shadowsocks. |
| `proxy_protocol` | Needs to listen on TCP.                                           |

#### listen

==Required==

Listen address.

#### listen_port

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

#### udp_timeout

UDP NAT expiration time in seconds, default is 300 (5 minutes).

#### proxy_protocol

Parse [Proxy Protocol](https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt) in the connection header.

#### detour

If set, connections will be forwarded to the specified inbound.

Requires target inbound support, see [Injectable](/configuration/inbound/#fields).