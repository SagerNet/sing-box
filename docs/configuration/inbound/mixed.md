`mixed` inbound is a socks4, socks4a, socks5 and http server.

### Structure

```json
{
  "inbounds": [
    {
      "type": "mixed",
      "tag": "mixed-in",
      
      "listen": "::",
      "listen_port": 2080,
      "tcp_fast_open": false,
      "sniff": false,
      "sniff_override_destination": false,
      "domain_strategy": "prefer_ipv6",
      
      "users": [
        {
          "username": "admin",
          "password": "admin"
        }
      ],
      
      "set_system_proxy": false
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

#### set_system_proxy

!!! error ""

    Windows only

Automatically set system proxy configuration when start and clean up when stop.

### Mixed Fields

#### users

Socks and HTTP users.

No authentication required if empty.