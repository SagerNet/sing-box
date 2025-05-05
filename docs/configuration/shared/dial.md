---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.12.0"

    :material-plus: [domain_resolver](#domain_resolver)  
    :material-delete-clock: [domain_strategy](#domain_strategy)  
    :material-plus: [netns](#netns)

!!! quote "Changes in sing-box 1.11.0"

    :material-plus: [network_strategy](#network_strategy)  
    :material-alert: [fallback_delay](#fallback_delay)  
    :material-alert: [network_type](#network_type)  
    :material-alert: [fallback_network_type](#fallback_network_type)

### Structure

```json
{
  "detour": "",
  "bind_interface": "",
  "inet4_bind_address": "",
  "inet6_bind_address": "",
  "routing_mark": 0,
  "reuse_addr": false,
  "netns": "",
  "connect_timeout": "",
  "tcp_fast_open": false,
  "tcp_multi_path": false,
  "udp_fragment": false,
  
  "domain_resolver": "", // or {}
  "network_strategy": "",
  "network_type": [],
  "fallback_network_type": [],
  "fallback_delay": "",

  // Deprecated
  
  "domain_strategy": ""
}
```

!!! note ""

    You can ignore the JSON Array [] tag when the content is only one item

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

Integers (e.g. `1234`) and string hexadecimals (e.g. `"0x1234"`) are supported.

#### reuse_addr

Reuse listener address.

#### netns

!!! question "Since sing-box 1.12.0"

!!! quote ""

    Only supported on Linux.

Set network namespace, name or path.

#### connect_timeout

Connect timeout, in golang's Duration format.

A duration string is a possibly signed sequence of
decimal numbers, each with optional fraction and a unit suffix,
such as "300ms", "-1.5h" or "2h45m".
Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".

#### tcp_fast_open

Enable TCP Fast Open.

#### tcp_multi_path

!!! warning ""

    Go 1.21 required.

Enable TCP Multi Path.

#### udp_fragment

Enable UDP fragmentation.

#### domain_resolver

!!! warning ""

    `outbound` DNS rule items are deprecated and will be removed in sing-box 1.14.0, so this item will be required for outbound/endpoints using domain name in server address since sing-box 1.14.0.

!!! info ""

    `domain_resolver` or `route.default_domain_resolver` is optional when only one DNS server is configured.

Set domain resolver to use for resolving domain names.

This option uses the same format as the [route DNS rule action](/configuration/dns/rule_action/#route) without the `action` field.

Setting this option directly to a string is equivalent to setting `server` of this options.

| Outbound/Endpoints | Effected domains         |
|--------------------|--------------------------|
| `direct`           | Domain in request        | 
| others             | Domain in server address |

#### network_strategy

!!! question "Since sing-box 1.11.0"

!!! quote ""

    Only supported in graphical clients on Android and Apple platforms with `auto_detect_interface` enabled.

Strategy for selecting network interfaces.

Available values:

- `default` (default): Connect to default network or networks specified in `network_type` sequentially.
- `hybrid`: Connect to all networks or networks specified in `network_type` concurrently.
- `fallback`: Connect to default network or preferred networks specified in `network_type` concurrently, and try fallback networks when unavailable or timeout.

For fallback, when preferred interfaces fails or times out,
it will enter a 15s fast fallback state (Connect to all preferred and fallback networks concurrently),
and exit immediately if preferred networks recover.

Conflicts with `bind_interface`, `inet4_bind_address` and `inet6_bind_address`.

#### network_type

!!! question "Since sing-box 1.11.0"

!!! quote ""

    Only supported in graphical clients on Android and Apple platforms with `auto_detect_interface` enabled.

Network types to use when using `default` or `hybrid` network strategy or
preferred network types to use when using `fallback` network strategy.

Available values: `wifi`, `cellular`, `ethernet`, `other`.

Device's default network is used by default.

#### fallback_network_type

!!! question "Since sing-box 1.11.0"

!!! quote ""

    Only supported in graphical clients on Android and Apple platforms with `auto_detect_interface` enabled.

Fallback network types when preferred networks are unavailable or timeout when using `fallback` network strategy.

All other networks expect preferred are used by default.

#### fallback_delay

!!! question "Since sing-box 1.11.0"

!!! quote ""

    Only supported in graphical clients on Android and Apple platforms with `auto_detect_interface` enabled.

The length of time to wait before spawning a RFC 6555 Fast Fallback connection.

For `domain_strategy`, is the amount of time to wait for connection to succeed before assuming
that IPv4/IPv6 is misconfigured and falling back to other type of addresses.

For `network_strategy`, is the amount of time to wait for connection to succeed before falling
back to other interfaces.

Only take effect when `domain_strategy` or `network_strategy` is set.

`300ms` is used by default.

#### domain_strategy

!!! failure "Deprecated in sing-box 1.12.0"

    `domain_strategy` is deprecated and will be removed in sing-box 1.14.0, check [Migration](/migration/#migrate-outbound-domain-strategy-option-to-domain-resolver).

Available values: `prefer_ipv4`, `prefer_ipv6`, `ipv4_only`, `ipv6_only`.

If set, the requested domain name will be resolved to IP before connect.

| Outbound | Effected domains         | Fallback Value                            |
|----------|--------------------------|-------------------------------------------|
| `direct` | Domain in request        | Take `inbound.domain_strategy` if not set | 
| others   | Domain in server address | /                                         |

