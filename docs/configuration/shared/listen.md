---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.12.0"

    :material-plus: [netns](#netns)  
    :material-plus: [bind_interface](#bind_interface)  
    :material-plus: [routing_mark](#routing_mark)  
    :material-plus: [reuse_addr](#reuse_addr)

!!! quote "Changes in sing-box 1.11.0"

    :material-delete-clock: [sniff](#sniff)  
    :material-delete-clock: [sniff_override_destination](#sniff_override_destination)  
    :material-delete-clock: [sniff_timeout](#sniff_timeout)  
    :material-delete-clock: [domain_strategy](#domain_strategy)  
    :material-delete-clock: [udp_disable_domain_unmapping](#udp_disable_domain_unmapping)

### Structure

```json
{
  "listen": "",
  "listen_port": 0,
  "bind_interface": "",
  "routing_mark": 0,
  "reuse_addr": false,
  "netns": "",
  "tcp_fast_open": false,
  "tcp_multi_path": false,
  "udp_fragment": false,
  "udp_timeout": "",
  "detour": "",

  // Deprecated
  
  "sniff": false,
  "sniff_override_destination": false,
  "sniff_timeout": "",
  "domain_strategy": "",
  "udp_disable_domain_unmapping": false
}
```

### Fields

#### listen

==Required==

Listen address.

#### listen_port

Listen port.

#### bind_interface

!!! question "Since sing-box 1.12.0"

The network interface to bind to.

#### routing_mark

!!! question "Since sing-box 1.12.0"

!!! quote ""

    Only supported on Linux.

Set netfilter routing mark.

Integers (e.g. `1234`) and string hexadecimals (e.g. `"0x1234"`) are supported.

#### reuse_addr

!!! question "Since sing-box 1.12.0"

Reuse listener address.

#### netns

!!! question "Since sing-box 1.12.0"

!!! quote ""

    Only supported on Linux.

Set network namespace, name or path.

#### tcp_fast_open

Enable TCP Fast Open.

#### tcp_multi_path

!!! warning ""

    Go 1.21 required.

Enable TCP Multi Path.

#### udp_fragment

Enable UDP fragmentation.

#### udp_timeout

UDP NAT expiration time.

`5m` will be used by default.

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
