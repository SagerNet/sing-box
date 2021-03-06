`socks` inbound is a http server.

### Structure

```json
{
  "inbounds": [
    {
      "type": "http",
      "tag": "http-in",
      
      "listen": "::",
      "listen_port": 2080,
      "tcp_fast_open": false,
      "sniff": false,
      "sniff_override_destination": false,
      "domain_strategy": "prefer_ipv6",

      "tls": {},
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

### HTTP Fields

#### tls

TLS configuration, see [TLS inbound structure](/configuration/shared/tls/#inbound-structure).

#### users

HTTP users.

No authentication required if empty.