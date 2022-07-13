!!! error ""

    Linux and Windows only

### Structure

```json
{
  "inbounds": [
    {
      "type": "tun",
      "tag": "tun-in",
      
      "inet4_address": "172.19.0.1/30",
      "inet6_address": "fdfe:dcba:9876::1/128",
      "mtu": 1500,
      "auto_route": true,
      "hijack_dns": true,
      
      "sniff": true,
      "sniff_override_destination": false,
      "domain_strategy": "prefer_ipv4"
    }
  ]
}
```

### Tun Fields

#### inet4_address

==Required==

IPv4 prefix for the tun interface.

#### inet6_address

IPv6 prefix for the tun interface.

#### mtu

The maximum transmission unit.

#### auto_route

Set the default route to the Tun.

!!! error ""

    To avoid traffic loopback, set `route.auto_delect_interface` or `outbound.bind_interface`

#### hijack_dns

Hijack TCP/UDP DNS requests to the built-in DNS adapter.

### Listen Fields

#### sniff

Enable sniffing.

Reads domain names for routing, supports HTTP TLS for TCP, QUIC for UDP.

This does not break zero copy, like splice.

#### sniff_override_destination

Override the connection destination address with the sniffed domain.

If the domain name is invalid (like tor), this will not work.

#### domain_strategy

One of `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`.

If set, the requested domain name will be resolved to IP before routing.

If `sniff_override_destination` is in effect, its value will be taken as a fallback.