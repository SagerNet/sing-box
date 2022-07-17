`direct` inbound is a tunnel server.

### Structure

```json
{
  "inbounds": [
    {
      "type": "direct",
      "tag": "direct-in",
      
      "listen": "::",
      "listen_port": 5353,
      "tcp_fast_open": false,
      "sniff": false,
      "sniff_override_destination": false,
      "domain_strategy": "prefer_ipv6",
      "udp_timeout": 300,
      
      "network": "udp",
      "override_address": "1.0.0.1",
      "override_port": 53
    }
  ]
}
```

### Listen Fields

#### listen

==Required==

Listen address.

#### listen_port

==Required==

Listen port.

#### tcp_fast_open

Enable tcp fast open for listener.

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

#### udp_timeout

UDP NAT expiration time in seconds, default is 300 (5 minutes).

### Direct Fields

#### network

Listen network, one of `tcp` `udp`.

Both if empty.

#### override_address

Override the connection destination address.

#### override_port

Override the connection destination port.