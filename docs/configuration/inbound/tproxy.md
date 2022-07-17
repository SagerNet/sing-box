`tproxy` inbound is a linux TProxy server.

### Structure

```json
{
  "inbounds": [
    {
      "type": "tproxy",
      "tag": "tproxy-in",
      
      "listen": "::",
      "listen_port": 5353,
      "sniff": false,
      "sniff_override_destination": false,
      "domain_strategy": "prefer_ipv6",
      "udp_timeout": 300,
      
      "network": "udp"
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

### TProxy Fields

#### network

Listen network, one of `tcp` `udp`.

Both if empty.