### Structure

```json
{
  "listen": "::",
  "listen_port": 5353,
  "tcp_fast_open": false,
  "tcp_multi_path": false,
  "udp_fragment": false,
  "sniff": false,
  "sniff_override_destination": false,
  "sniff_override_rules": [],
  "sniff_timeout": "300ms",
  "domain_strategy": "prefer_ipv6",
  "udp_timeout": 300,
  "proxy_protocol": false,
  "proxy_protocol_accept_no_header": false,
  "detour": "another-in"
}
```

### Fields

| Field                             | Available Context                                                 |
|-----------------------------------|-------------------------------------------------------------------|
| `listen`                          | Needs to listen on TCP or UDP.                                    |
| `listen_port`                     | Needs to listen on TCP or UDP.                                    |
| `tcp_fast_open`                   | Needs to listen on TCP.                                           |
| `tcp_multi_path`                  | Needs to listen on TCP.                                           |
| `udp_timeout`                     | Needs to assemble UDP connections, currently Tun and Shadowsocks. |
| `proxy_protocol`                  | Needs to listen on TCP.                                           |
| `proxy_protocol_accept_no_header` | When `proxy_protocol` enabled                                     |

#### listen

==Required==

Listen address.

#### listen_port

Listen port.

#### tcp_fast_open

Enable TCP Fast Open.

#### tcp_multi_path

!!! warning ""

    Go 1.21 required.

Enable TCP Multi Path.

#### udp_fragment

Enable UDP fragmentation.

#### sniff

Enable sniffing.

See [Protocol Sniff](/configuration/route/sniff/) for details.

#### sniff_override_destination

Override the connection destination address with the sniffed domain.

If the domain name is invalid (like tor), this will not work.

#### sniff_override_rules

Pick up the connection that will be overrided destination address with the sniffed domain by rules.

If the domain name is invalid (like tor), this will not work.

See [Sniff Override Rule](/configuration/shared/sniff_override_rules/) for details.

#### sniff_timeout

Timeout for sniffing.

300ms is used by default.

#### domain_strategy

One of `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`.

If set, the requested domain name will be resolved to IP before routing.

If `sniff_override_destination` is in effect, its value will be taken as a fallback.

#### udp_timeout

UDP NAT expiration time in seconds, default is 300 (5 minutes).

#### proxy_protocol

Parse [Proxy Protocol](https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt) in the connection header.

#### proxy_protocol_accept_no_header

Accept connections without Proxy Protocol header.

#### detour

If set, connections will be forwarded to the specified inbound.

Requires target inbound support, see [Injectable](/configuration/inbound/#fields).