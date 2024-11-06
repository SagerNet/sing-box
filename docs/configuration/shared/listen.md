---
icon: material/delete-clock
---

!!! quote "Changes in sing-box 1.11.0"

    :material-delete-clock: [sniff](#sniff)  
    :material-delete-clock: [sniff_override_destination](#sniff_override_destination)  
    :material-delete-clock: [sniff_timeout](#sniff_timeout)  
    :material-delete-clock: [domain_strategy](#domain_strategy)  
    :material-delete-clock: [udp_disable_domain_unmapping](#udp_disable_domain_unmapping)

### Structure

```json
{
  "listen": "::",
  "listen_port": 5353,
  "tcp_fast_open": false,
  "tcp_multi_path": false,
  "udp_fragment": false,
  "udp_timeout": "5m",
  "detour": "another-in",
  "sniff": false,
  "sniff_override_destination": false,
  "sniff_timeout": "300ms",
  "domain_strategy": "prefer_ipv6",
  "udp_disable_domain_unmapping": false
}
```

### Fields

| Field                          | Available Context                                       |
|--------------------------------|---------------------------------------------------------|
| `listen`                       | Needs to listen on TCP or UDP.                          |
| `listen_port`                  | Needs to listen on TCP or UDP.                          |
| `tcp_fast_open`                | Needs to listen on TCP.                                 |
| `tcp_multi_path`               | Needs to listen on TCP.                                 |
| `udp_timeout`                  | Needs to assemble UDP connections.                      |
| `udp_disable_domain_unmapping` | Needs to listen on UDP and accept domain UDP addresses. |

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

UDP NAT expiration time in seconds.

`5m` is used by default.

#### detour

If set, connections will be forwarded to the specified inbound.

Requires target inbound support, see [Injectable](/configuration/inbound/#fields).

#### sniff

!!! failure "Deprecated in sing-box 1.11.0"

    Inbound fields are deprecated and will be removed in sing-box 1.13.0, check [Migration](/migration/#migrate-legacy-inbound-fields-to-rule-actions).

Enable sniffing.

See [Protocol Sniff](/configuration/route/sniff/) for details.

#### sniff_override_destination

!!! failure "Deprecated in sing-box 1.11.0"

    Inbound fields are deprecated and will be removed in sing-box 1.13.0.

Override the connection destination address with the sniffed domain.

If the domain name is invalid (like tor), this will not work.

#### sniff_timeout

!!! failure "Deprecated in sing-box 1.11.0"

    Inbound fields are deprecated and will be removed in sing-box 1.13.0, check [Migration](/migration/#migrate-legacy-inbound-fields-to-rule-actions).

Timeout for sniffing.

`300ms` is used by default.

#### domain_strategy

!!! failure "Deprecated in sing-box 1.11.0"

    Inbound fields are deprecated and will be removed in sing-box 1.13.0, check [Migration](/migration/#migrate-legacy-inbound-fields-to-rule-actions).

One of `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`.

If set, the requested domain name will be resolved to IP before routing.

If `sniff_override_destination` is in effect, its value will be taken as a fallback.

#### udp_disable_domain_unmapping

!!! failure "Deprecated in sing-box 1.11.0"

    Inbound fields are deprecated and will be removed in sing-box 1.13.0, check [Migration](/migration/#migrate-legacy-inbound-fields-to-rule-actions).

If enabled, for UDP proxy requests addressed to a domain, 
the original packet address will be sent in the response instead of the mapped domain.

This option is used for compatibility with clients that 
do not support receiving UDP packets with domain addresses, such as Surge.
