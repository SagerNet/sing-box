### Structure

```json
{
  "listen": "::",
  "listen_port": 5353,
  "tcp_fast_open": false,
  "tcp_multi_path": false,
  "udp_fragment": false,
  "udp_timeout": 300,
  "detour": "another-in",
  "sniff": false,
  "sniff_override_destination": false,
  "sniff_timeout": "300ms",
  "domain_strategy": "prefer_ipv6",
  "udp_disable_domain_unmapping": false
}
```

### Fields

| Field                          | Available Context                                                 |
|--------------------------------|-------------------------------------------------------------------|
| `listen`                       | Needs to listen on TCP or UDP.                                    |
| `listen_port`                  | Needs to listen on TCP or UDP.                                    |
| `tcp_fast_open`                | Needs to listen on TCP.                                           |
| `tcp_multi_path`               | Needs to listen on TCP.                                           |
| `udp_timeout`                  | Needs to assemble UDP connections, currently Tun and Shadowsocks. |
| `udp_disable_domain_unmapping` | Needs to listen on UDP and accept domain UDP addresses.           |

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

#### udp_timeout

UDP NAT expiration time in seconds, default is 300 (5 minutes).

#### detour

If set, connections will be forwarded to the specified inbound.

Requires target inbound support, see [Injectable](/configuration/inbound/#fields).

#### sniff

Enable sniffing.

See [Protocol Sniff](/configuration/route/sniff/) for details.

#### sniff_override_destination

Override the connection destination address with the sniffed domain.

If the domain name is invalid (like tor), this will not work.

#### sniff_timeout

Timeout for sniffing.

300ms is used by default.

#### domain_strategy

One of `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`.

If set, the requested domain name will be resolved to IP before routing.

If `sniff_override_destination` is in effect, its value will be taken as a fallback.

#### udp_disable_domain_unmapping

If enabled, for UDP proxy requests addressed to a domain, 
the original packet address will be sent in the response instead of the mapped domain.

This option is used for compatibility with clients that 
do not support receiving UDP packets with domain addresses, such as Surge.
