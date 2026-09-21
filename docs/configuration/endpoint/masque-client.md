# MASQUE Client

!!! question "Since sing-box 1.15.0"

`masque-client` endpoint is an IP proxying over HTTP ([RFC 9484](https://datatracker.ietf.org/doc/html/rfc9484), CONNECT-IP) client.

## Structure

```json
{
  "type": "masque-client",
  "tag": "masque-client",

  "server": "127.0.0.1",
  "server_port": 443,
  "username": "",
  "password": "",
  "path": "",
  "headers": {},
  "version": 0,
  "disable_version_fallback": false,
  "tls": {},
  "advertise_routes": [],
  "system": false,
  "name": "",
  "mtu": 1280,
  "on_demand": false,

  ... // HTTP2 Fields / QUIC Fields
  ... // UDP NAT Fields
  ... // Dial Fields
}
```

!!! note ""

    You can ignore the JSON Array [] tag when the content is only one item

## Fields

### server

==Required==

The server address.

### server_port

==Required==

The server port.

### username

Basic authorization username.

### password

Basic authorization password.

### path

URI template path of the IP proxying resource, may contain the `target` and `ipproto` variables.

`/.well-known/masque/ip/{target}/{ipproto}/` is used by default.

### headers

Extra headers of HTTP request.

### version

HTTP version.

Available values: `1`, `2`, `3`.

`3` is used by default.

When `1` or `2`, IP packets are carried in the TCP stream instead of QUIC datagrams.

When `2`, [QUIC Fields](#quic-fields) are replaced by [HTTP2 Fields](#http2-fields).

### disable_version_fallback

Disable automatic fallback to lower HTTP version.

### tls

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

Required for HTTP/3.

### advertise_routes

List of IP prefixes to advertise to the server.

The server will route traffic for these prefixes into this endpoint, where it is handled as inbound traffic.

### system

Use system interface.

Requires privilege and cannot conflict with existing system interfaces.

The endpoint configures interface addresses and MTU but does not install
operating-system routes or DNS settings.

If disabled, sing-box uses the internal network stack.

### name

Custom interface name for system interface.

An automatically generated `masque` interface name is used by default.

### mtu

Tunnel MTU.

`1280` will be used by default.

### on_demand

Allow the endpoint to be disconnected when necessary.

## HTTP2 Fields

When `version` is `2`.

See [HTTP2 Fields](/configuration/shared/http2/) for details.

`keep_alive_period` is `10s` by default.

## QUIC Fields

When `version` is `3` (default).

See [QUIC Fields](/configuration/shared/quic/) for details.

`keep_alive_period` is `10s` by default.

`initial_packet_size` is `mtu + 51` by default, so that IP packets up to the tunnel MTU fit into a QUIC datagram. QUIC packets cannot exceed 1452 bytes; with a larger `mtu`, IP packets that do not fit are answered with ICMP Packet Too Big. If the path cannot carry packets of that size, the QUIC handshake fails and the client falls back to a lower HTTP version.

## UDP NAT Fields

See [UDP NAT Fields](/configuration/shared/udp-nat/) for details.

## Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
