!!! error ""

    Only supported on Linux, Windows and macOS.

### Structure

```json
{
  "inbounds": [
    {
      "type": "tun",
      "tag": "tun-in",
      
      "interface_name": "tun0",
      "inet4_address": "172.19.0.1/30",
      "inet6_address": "fdfe:dcba:9876::1/128",
      "mtu": 1500,
      "auto_route": true,
      "endpoint_independent_nat": false,
      "udp_timeout": 300,
      "stack": "gvisor",
      
      "sniff": true,
      "sniff_override_destination": false,
      "domain_strategy": "prefer_ipv4"
    }
  ]
}
```

!!! warning ""

    If tun is running in non-privileged mode, the address and MTU will not be configured automatically, please make sure the settings are accurate.

### Tun Fields

#### interface_name

Virtual device name, automatically selected if empty.

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

Enable endpoint-independent NAT.

Performance may degrade slightly, so it is not recommended to enable on when it is not needed.

#### udp_timeout

UDP NAT expiration time in seconds, default is 300 (5 minutes).

#### stack

TCP/IP stack.

| Stack            | Upstream                                                              | Status            |
|------------------|-----------------------------------------------------------------------|-------------------|
| gVisor (default) | [google/gvisor](https://github.com/google/gvisor)                     | recommended       |
| LWIP             | [eycorsican/go-tun2socks](https://github.com/eycorsican/go-tun2socks) | upstream archived |

!!! warning ""

    The LWIP stack is not included by default, see [Installation](/#Installation).

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