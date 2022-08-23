### Structure

```json
{
  "inbounds": [
    {
      "type": "hysteria",
      "tag": "hysteria-in",
      "listen": "::",
      "listen_port": 443,
      "sniff": false,
      "sniff_override_destination": false,
      "domain_strategy": "prefer_ipv6",
      "up": "100 Mbps",
      "up_mbps": 100,
      "down": "100 Mbps",
      "down_mbps": 100,
      "obfs": "fuck me till the daylight",
      "auth": "",
      "auth_str": "password",
      "recv_window_conn": 0,
      "recv_window_client": 0,
      "max_conn_client": 0,
      "disable_mtu_discovery": false,
      "tls": {}
    }
  ]
}
```

!!! warning ""

    QUIC, which is required by hysteria is not included by default, see [Installation](/#installation).

### Hysteria Fields

#### up, down

==Required==

Format: `[Integer] [Unit]` e.g. `100 Mbps, 640 KBps, 2 Gbps`

Supported units (case sensitive, b = bits, B = bytes, 8b=1B):

    bps (bits per second)
    Bps (bytes per second)
    Kbps (kilobits per second)
    KBps (kilobytes per second)
    Mbps (megabits per second)
    MBps (megabytes per second)
    Gbps (gigabits per second)
    GBps (gigabytes per second)
    Tbps (terabits per second)
    TBps (terabytes per`socks` inbound is a http server.
 second)

#### up_mbps, down_mbps

==Required==

`up, down` in Mbps.

#### obfs

Obfuscated password.

#### auth

Authentication password, in base64.

#### auth_str

Authentication password.

#### recv_window_conn

The QUIC stream-level flow control window for receiving data.

`15728640 (15 MB/s)` will be used if empty.

#### recv_window_client

The QUIC connection-level flow control window for receiving data.

`67108864 (64 MB/s)` will be used if empty.

#### max_conn_client

The maximum number of QUIC concurrent bidirectional streams that a peer is allowed to open.

`1024` will be used if empty.

#### disable_mtu_discovery

Disables Path MTU Discovery (RFC 8899). Packets will then be at most 1252 (IPv4) / 1232 (IPv6) bytes in size.

Force enabled on for systems other than Linux and Windows (according to upstream).

#### tls

==Required==

TLS configuration, see [TLS inbound structure](/configuration/shared/tls/#inbound-structure).

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
