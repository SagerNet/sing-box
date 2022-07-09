`shadowsocks` inbound is a shadowsocks server.

### Structure

```json
{
  "inbounds": [
    {
      "type": "shadowsocks",
      "tag": "ss-in",
      
      "listen": "::",
      "listen_port": 5353,
      "tcp_fast_open": false,
      "sniff": false,
      "sniff_override_destination": false,
      "domain_strategy": "prefer_ipv6",
      "udp_timeout": 300,
      "network": "udp",
      
      "method": "2022-blake3-aes-128-gcm",
      "password": "8JCsPssfgS8tiRwiMlhARg=="
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

Reads domain names for routing, supports HTTP TLS for TCP, QUIC for UDP.

This does not break zero copy, like splice.

#### sniff_override_destination

Override the connection destination address with the sniffed domain.

If the domain name is invalid (like tor), this will not work.

#### domain_strategy

One of `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`.

If set, the requested domain name will be resolved to IP before routing.

If `sniff_override_destination` is in effect, its value will be taken as a fallback.

#### udp_timeout

UDP NAT expiration time in seconds, default is 300 (5 minutes).

### Shadowsocks Fields

#### network

Listen network, one of `tcp` `udp`.

Both if empty.

#### method

==Required==

| Method                        | Key Length |
|-------------------------------|------------|
| 2022-blake3-aes-128-gcm       | 16         |
| 2022-blake3-aes-256-gcm       | 32         |
| 2022-blake3-chacha20-poly1305 | 32         |
| none                          | /          |
| aes-128-gcm                   | /          |
| aes-192-gcm                   | /          |
| aes-256-gcm                   | /          |
| chacha20-ietf-poly1305        | /          |
| xchacha20-ietf-poly1305       | /          |

#### password

==Required==

| Method        | Password Format                     |
|---------------|-------------------------------------|
| none          | /                                   |
| 2022 methods  | `openssl rand -base64 <Key Length>` |
| other methods | any string                          |

### Multi-User Structure

```json
{
  "inbounds": [
    {
      "type": "shadowsocks",
      "method": "2022-blake3-aes-128-gcm",
      "password": "8JCsPssfgS8tiRwiMlhARg==",
      "users": [
        {
          "name": "sekai",
          "password": "PCD2Z4o12bKUoFa3cC97Hw=="
        }
      ]
    }
  ]
}
```

### Relay Structure

```json
{
  "inbounds": [
    {
      "type": "shadowsocks",
      "method": "2022-blake3-aes-128-gcm",
      "password": "8JCsPssfgS8tiRwiMlhARg==",
      "destinations": [
        {
          "name": "test",
          "server": "example.com",
          "server_port": 8080,
          "password": "PCD2Z4o12bKUoFa3cC97Hw=="
        }
      ]
    }
  ]
}
```