`dns` inbound is a DNS server.

### Structure

```json
{
  "inbounds": [
    {
      "type": "dns",
      "tag": "dns-in",
      
      "listen": "::",
      "listen_port": 5353,
      "network": "udp"
    }
  ]
}
```

!!! note ""
    
    There are no outbound connections by the DNS inbound, all requests are handled internally.

### Listen Fields

#### listen

==Required==

Listen address.

#### listen_port

==Required==

Listen port.

### DNS Fields

#### network

Listen network, one of `tcp` `udp`.

Both if empty.