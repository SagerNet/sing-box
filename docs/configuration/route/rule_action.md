---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.12.0"

    :material-plus: [tls_fragment](#tls_fragment)  
    :material-plus: [tls_fragment_fallback_delay](#tls_fragment_fallback_delay)  
    :material-plus: [resolve.disable_cache](#disable_cache)  
    :material-plus: [resolve.rewrite_ttl](#rewrite_ttl)  
    :material-plus: [resolve.client_subnet](#client_subnet)

## Final actions

### route

```json
{
  "action": "route", // default
  "outbound": "",
 
  ... // route-options Fields
}
```

!!! note ""

    You can ignore the JSON Array [] tag when the content is only one item

`route` inherits the classic rule behavior of routing connection to the specified outbound.

#### outbound

==Required==

Tag of target outbound.

#### route-options Fields

See `route-options` fields below.

### reject

```json
{
  "action": "reject",
  "method": "default", // default
  "no_drop": false
}
```

`reject` reject connections

The specified method is used for reject tun connections if `sniff` action has not been performed yet.

For non-tun connections and already established connections, will just be closed.

#### method

- `default`: Reply with TCP RST for TCP connections, and ICMP port unreachable for UDP packets.
- `drop`: Drop packets.

#### no_drop

If not enabled, `method` will be temporarily overwritten to `drop` after 50 triggers in 30s.

Not available when `method` is set to drop.

### hijack-dns

```json
{
  "action": "hijack-dns"
}
```

`hijack-dns` hijack DNS requests to the sing-box DNS module.

## Non-final actions

### route-options

```json
{
  "action": "route-options",
  "override_address": "",
  "override_port": 0,
  "network_strategy": "",
  "fallback_delay": "",
  "udp_disable_domain_unmapping": false,
  "udp_connect": false,
  "udp_timeout": "",
  "tls_fragment": false,
  "tls_fragment_fallback_delay": ""
}
```

`route-options` set options for routing.

#### override_address

Override the connection destination address.

#### override_port

Override the connection destination port.

#### network_strategy

See [Dial Fields](/configuration/shared/dial/#network_strategy) for details.

Only take effect if outbound is direct without `outbound.bind_interface`,
`outbound.inet4_bind_address` and `outbound.inet6_bind_address` set.

#### network_type

See [Dial Fields](/configuration/shared/dial/#network_type) for details.

#### fallback_network_type

See [Dial Fields](/configuration/shared/dial/#fallback_network_type) for details.

#### fallback_delay

See [Dial Fields](/configuration/shared/dial/#fallback_delay) for details.

#### udp_disable_domain_unmapping

If enabled, for UDP proxy requests addressed to a domain,
the original packet address will be sent in the response instead of the mapped domain.

This option is used for compatibility with clients that
do not support receiving UDP packets with domain addresses, such as Surge.

#### udp_connect

If enabled, attempts to connect UDP connection to the destination instead of listen.

#### udp_timeout

Timeout for UDP connections.

Setting a larger value than the UDP timeout in inbounds will have no effect.

Default value for protocol sniffed connections:

| Timeout | Protocol             |
|---------|----------------------|
| `10s`   | `dns`, `ntp`, `stun` |
| `30s`   | `quic`, `dtls`       |

If no protocol is sniffed, the following ports will be recognized as protocols by default:

| Port | Protocol |
|------|----------|
| 53   | `dns`    |
| 123  | `ntp`    |
| 443  | `quic`   |
| 3478 | `stun`   |

#### tls_fragment

!!! question "Since sing-box 1.12.0"

Fragment TLS handshakes to bypass firewalls.

This feature is intended to circumvent simple firewalls based on **plaintext packet matching**, and should not be used to circumvent real censorship.

Since it is not designed for performance, it should not be applied to all connections, but only to server names that are known to be blocked.

On Linux, Apple platforms, (administrator privileges required) Windows, the wait time can be automatically detected, otherwise it will fall back to waiting for a fixed time specified by `tls_fragment_fallback_delay`.

In addition, if the actual wait time is less than 20ms, it will also fall back to waiting for a fixed time, because the target is considered to be local or behind a transparent proxy.

#### tls_fragment_fallback_delay

!!! question "Since sing-box 1.12.0"

The fallback value used when TLS segmentation cannot automatically determine the wait time.

`500ms` is used by default.

### sniff

```json
{
  "action": "sniff",
  "sniffer": [],
  "timeout": ""
}
```

`sniff` performs protocol sniffing on connections.

For deprecated `inbound.sniff` options, it is considered to `sniff()` performed before routing.

#### sniffer

Enabled sniffers.

All sniffers enabled by default.

Available protocol values an be found on in [Protocol Sniff](../sniff/)

#### timeout

Timeout for sniffing.

`300ms` is used by default.

### resolve

```json
{
  "action": "resolve",
  "server": "",
  "strategy": "",
  "disable_cache": false,
  "rewrite_ttl": null,
  "client_subnet": null
}
```

`resolve` resolve request destination from domain to IP addresses.

#### server

Specifies DNS server tag to use instead of selecting through DNS routing.

#### strategy

DNS resolution strategy, available values are: `prefer_ipv4`, `prefer_ipv6`, `ipv4_only`, `ipv6_only`.

`dns.strategy` will be used by default.

#### disable_cache

!!! question "Since sing-box 1.12.0"

Disable cache and save cache in this query.

#### rewrite_ttl

!!! question "Since sing-box 1.12.0"

Rewrite TTL in DNS responses.

#### client_subnet

!!! question "Since sing-box 1.12.0"

Append a `edns0-subnet` OPT extra record with the specified IP prefix to every query by default.

If value is an IP address instead of prefix, `/32` or `/128` will be appended automatically.

Will overrides `dns.client_subnet`.
