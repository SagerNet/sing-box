# MASQUE Server

!!! question "Since sing-box 1.15.0"

`masque-server` endpoint is an IP proxying over HTTP ([RFC 9484](https://datatracker.ietf.org/doc/html/rfc9484), CONNECT-IP) server.

## Structure

```json
{
  "type": "masque-server",
  "tag": "masque-server",

  ... // Listen Fields

  "version": [],
  "users": [
    {
      "username": "",
      "password": ""
    }
  ],
  "tls": {},
  "path": "",
  "address": [],
  "advertise_routes": [],
  "system": false,
  "name": "",
  "mtu": 1280,

  ... // HTTP2 Fields / QUIC Fields
  ... // UDP NAT Fields
}
```

!!! note ""

    You can ignore the JSON Array [] tag when the content is only one item

## Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details. `udp_timeout` is part of the [UDP NAT Fields](#udp-nat-fields) below.

## Fields

### version

List of HTTP versions to serve.

Available values: `1`, `2`, `3`.

All versions are used by default.

TLS is required for `3`.

### users

HTTP users, verified by the `Authorization` header.

No authentication required if empty.

### tls

TLS configuration, see [TLS](/configuration/shared/tls/#inbound).

IP proxying must be operated over TLS or QUIC. Leave it disabled only when the server is placed behind an HTTP intermediary that terminates TLS.

### path

URI template path of the IP proxying resource, may contain the `target` and `ipproto` variables.

`/.well-known/masque/ip/{target}/{ipproto}/` is used by default.

### address

==Required==

List of IP prefixes of the tunnel network, at most one for each IP version.

The address of the prefix is used by the server itself, other addresses in the prefix are assigned to clients.

### advertise_routes

List of IP prefixes to advertise to clients, in addition to the tunnel network.

Traffic from clients to other destinations is rejected.

All addresses are advertised by default.

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

## HTTP2 Fields

When `version` contains `2`.

See [HTTP2 Fields](/configuration/shared/http2/) for details.

## QUIC Fields

When `version` contains `3` (default), [HTTP2 Fields](#http2-fields) are replaced by QUIC Fields.

See [QUIC Fields](/configuration/shared/quic/) for details.

`initial_packet_size` is `mtu + 51` by default, so that IP packets up to the tunnel MTU fit into a QUIC datagram. QUIC packets cannot exceed 1452 bytes; with a larger `mtu`, IP packets that do not fit are answered with ICMP Packet Too Big.

## UDP NAT Fields

See [UDP NAT Fields](/configuration/shared/udp-nat/) for details.
