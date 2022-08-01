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
      "endpoint_independent_nat": false,
      "udp_timeout": 300,
      
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

    To avoid traffic loopback, set `route.auto_detect_interface` or `route.default_interface` or `outbound.bind_interface`

#### endpoint_independent_nat

Enabled endpoint-independent NAT.

Performance may degrade slightly, so it is not recommended to enable on when it is not needed.

#### udp_timeout

UDP NAT expiration time in seconds, default is 300 (5 minutes).

### Listen Fields

#### sniff

Enable sniffing.

See [Sniff](/configuration/route/sniff/) for details.

#### sniff_override_destination

Override the connection destination address with the sniffed domain.

If the domain name is invalid (like tor), this will not work.

#### domain_strategy

One of `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`.

If set, the requested domain name will be resolved to IP before routing.

If `sniff_override_destination` is in effect, its value will be taken as a fallback.