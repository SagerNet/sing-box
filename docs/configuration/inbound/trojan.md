### Structure

```json
{
  "inbounds": [
    {
      "type": "trojan",
      "tag": "trojan-in",
      
      "listen": "::",
      "listen_port": 2080,
      "tcp_fast_open": false,
      "sniff": false,
      "sniff_override_destination": false,
      "domain_strategy": "prefer_ipv6",
      "proxy_protocol": false,

      "users": [
        {
          "name": "sekai",
          "password": "8JCsPssfgS8tiRwiMlhARg=="
        }
      ],
      "tls": {},
      "fallback": {
        "server": "127.0.0.1",
        "server_port": 8080
      },
      "fallback_for_alpn": {
        "http/1.1": {
          "server": "127.0.0.1",
          "server_port": 8081
        }
      },
      "transport": {}
    }
  ]
}
```

### Trojan Fields

#### users

==Required==

Trojan users.

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#inbound).

#### fallback

!!! error ""

    There is no evidence that GFW detects and blocks Trojan servers based on HTTP responses, and opening the standard http/s port on the server is a much bigger signature.

Fallback server configuration. Disabled if `fallback` and `fallback_for_alpn` are empty.

#### fallback_for_alpn

Fallback server configuration for specified ALPN.

If not empty, TLS fallback requests with ALPN not in this table will be rejected.

#### transport

V2Ray Transport configuration, see [V2Ray Transport](/configuration/shared/v2ray-transport).

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

See [Protocol Sniff](/configuration/route/sniff/) for details.

#### sniff_override_destination

Override the connection destination address with the sniffed domain.

If the domain name is invalid (like tor), this will not work.

#### domain_strategy

One of `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`.

If set, the requested domain name will be resolved to IP before routing.

If `sniff_override_destination` is in effect, its value will be taken as a fallback.

#### proxy_protocol

Parse [Proxy Protocol](https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt) in the connection header.