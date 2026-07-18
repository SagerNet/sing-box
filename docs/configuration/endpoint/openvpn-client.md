# OpenVPN Client

!!! question "Since sing-box 1.14.0"

## Structure

```json
{
  "type": "openvpn-client",
  "tag": "ovpn-client",

  "server": "127.0.0.1",
  "server_port": 1194,
  "servers": [
    {
      "server": "127.0.0.1",
      "server_port": 1194,
      "network": "udp"
    }
  ],
  "remote_random": false,
  "network": "udp",
  "username": "",
  "password": "",
  "auth_retry": "none",
  "static_challenge": "",
  "static_challenge_echo": false,
  "tls": {
    "server_name": "",
    "server_name_type": "name",
    "certificate": [],
    "certificate_path": "",
    "client_certificate": [],
    "client_certificate_path": "",
    "client_key": [],
    "client_key_path": "",
    "peer_fingerprint": [],
    "crl_path": "",
    "remote_certificate_ku": [],
    "remote_certificate_eku": "",
    "version_min": "1.2",
    "version_max": "",
    "cipher": "",
    "groups": "",
    "control_wrap": {
      "type": "",
      "key": [],
      "key_path": "",
      "direction": ""
    }
  },
  "data_ciphers": [],
  "data_ciphers_fallback": "",
  "auth": "",
  "mss_fix": 0,
  "fragment": 0,
  "compression": "",
  "compression_lzo": "",
  "allow_compression": "no",
  "route_no_pull": false,
  "pull_filters": [
    {
      "action": "ignore",
      "text": "route "
    }
  ],
  "routes": [],
  "route_gateway": "",
  "route_metric": 0,
  "redirect_gateway": false,
  "redirect_gateway_flags": [],
  "ping_interval": "",
  "ping_restart": "",
  "renegotiate_interval": "",
  "explicit_exit_notify": 0,
  "system": false,
  "name": "",
  "mtu": 1500,

  ... // UDP NAT Fields

  ... // Dial Fields
}
```

!!! note ""

    You can ignore the JSON Array [] tag when the content is only one item.

## Fields

### server

OpenVPN server address.

Either `server` or `servers` is required.

Conflict with `servers`.

### server_port

OpenVPN server port.

Required when `server` is set.

### servers

List of OpenVPN servers.

The client tries the servers in order and moves to the next server when a connection fails.

Either `server` or `servers` is required.

Conflict with `server`.

### servers.server

==Required==

OpenVPN server address.

### servers.server_port

==Required==

OpenVPN server port.

### servers.network

OpenVPN transport network for this server, one of `udp` or `tcp`.

The top-level `network` is used by default.

### remote_random

Randomize the `servers` order before connecting.

Disabled by default.

### network

Default OpenVPN transport network, one of `udp` or `tcp`.

`udp` is used by default.

This value applies to `server` and to `servers` entries without their own `network`.

### username

Username for OpenVPN username/password authentication.

### password

Password for OpenVPN username/password authentication.

### auth_retry

Behavior after username/password authentication fails, one of `none`, `nointeract`, or `interact`.

`none` is used by default and treats a permanent authentication failure as terminal.

`nointeract` and `interact` allow authentication retries.

### static_challenge

Static challenge text shown when requesting an authentication response.

### static_challenge_echo

Show the static challenge response as plain text.

### tls

==Required==

OpenVPN control channel TLS configuration.

### tls.server_name

Expected server certificate name.

Certificate name verification is disabled if empty. The certificate chain or fingerprint and server certificate usage are still verified.

### tls.server_name_type

Certificate field matched by `tls.server_name`, one of `subject`, `name`, or `name-prefix`.

`name` is used by default when `tls.server_name` is set.

`subject` matches the full certificate subject, `name` matches the common name exactly, and `name-prefix` matches a common name prefix.

### tls.certificate

Trusted CA certificate content.

One of `tls.certificate`, `tls.certificate_path`, or `tls.peer_fingerprint` is required.

Conflict with `tls.certificate_path`.

### tls.certificate_path

Trusted CA certificate path.

One of `tls.certificate`, `tls.certificate_path`, or `tls.peer_fingerprint` is required.

Conflict with `tls.certificate`.

### tls.client_certificate

Client certificate content.

Conflict with `tls.client_certificate_path`.

### tls.client_certificate_path

Client certificate path.

Conflict with `tls.client_certificate`.

### tls.client_key

Client private key content.

Conflict with `tls.client_key_path`.

### tls.client_key_path

Client private key path.

Conflict with `tls.client_key`.

The client certificate and key must both be set or both be empty.

### tls.peer_fingerprint

Allowed SHA-256 fingerprints of the server leaf certificate.

Each fingerprint must be 64 lowercase hexadecimal characters without separators.

When a trusted CA is also configured, both the certificate chain and fingerprint are verified. Without a trusted CA, the fingerprint, certificate validity period, configured name, and certificate usage are verified, but the certificate chain is not.

### tls.crl_path

Path to a PEM or DER certificate revocation list used to reject revoked server certificates.

The CRL signature and validity period are verified against the trusted certificate chain.

Disabled by default.

### tls.remote_certificate_ku

Required server certificate key usage masks, written as hexadecimal values in OpenVPN `remote-cert-ku` format.

Multiple values are combined, and all requested usages must be present.

Disabled by default.

### tls.remote_certificate_eku

Required server certificate extended key usage, one of `server` or `client`.

Disabled by default. The standard OpenVPN server certificate usage check still applies.

### tls.version_min

Minimum TLS version, one of `1.0`, `1.1`, `1.2`, or `1.3`.

`1.2` is used by default.

### tls.version_max

Maximum TLS version, one of `1.0`, `1.1`, `1.2`, or `1.3`.

The maximum supported version is used by default.

The value cannot be lower than `tls.version_min`.

### tls.cipher

Colon-separated OpenSSL cipher suite names allowed for TLS 1.2 and earlier.

The default TLS cipher suites are used when empty. TLS 1.3 cipher suites are not controlled by this field.

### tls.groups

Colon-separated TLS key exchange groups in preference order.

Supported groups are `X25519`, `SECP256R1`, `SECP384R1`, and `SECP521R1`, including their common OpenSSL and NIST aliases.

The default TLS groups are used when empty.

### tls.control_wrap

OpenVPN control channel wrapping.

Equivalent to OpenVPN `tls-auth`, `tls-crypt`, and `tls-crypt-v2`.

Disabled if empty.

### tls.control_wrap.type

Control channel wrapping type, one of `tls_auth`, `tls_crypt`, or `tls_crypt_v2`.

### tls.control_wrap.key

Control channel wrapping key content.

Conflict with `tls.control_wrap.key_path`.

### tls.control_wrap.key_path

Control channel wrapping key path.

Conflict with `tls.control_wrap.key`.

### tls.control_wrap.direction

`tls-auth` key direction, one of `server` or `client`.

Only available when `tls.control_wrap.type` is `tls_auth`. The key is used bidirectionally if empty.

### data_ciphers

Allowed OpenVPN data channel ciphers.

`AES-256-GCM`, `AES-128-GCM`, and `CHACHA20-POLY1305` are used by default.

### data_ciphers_fallback

Data channel cipher for peers that do not support cipher negotiation.

Disabled by default.

### auth

OpenVPN data channel authentication digest.

`SHA1` is used by default. It only applies to non-AEAD data ciphers and `tls_auth`.

### mss_fix

Maximum OpenVPN UDP packet size used to clamp the MSS of TCP connections sent through the tunnel.

This prevents TCP packets from exceeding the path MTU after OpenVPN encapsulation.

When empty, the upstream OpenVPN default is used: `fragment` when configured,
otherwise `1492` for the default tunnel MTU or the configured tunnel MTU.

### fragment

Maximum OpenVPN UDP packet size used for OpenVPN data channel fragmentation.

Disabled when `0`. A non-zero value must be at least `68`.

Conflict with TCP transport.

### compression

OpenVPN `compress` framing mode, one of `none`, `no`, `lz4`, `lz4-v2`, `stub`, `stub-v2`, `disabled`, or `off`.

Disabled by default.

Compression can weaken traffic confidentiality. Prefer `stub` or `stub-v2` only when framing compatibility is required.

### compression_lzo

OpenVPN `comp-lzo` mode, one of `none`, `no`, `yes`, `adaptive`, `asym`, `disabled`, or `off`.

Disabled by default.

Compression can weaken traffic confidentiality. Enable it only when required by the server.

### allow_compression

Policy for compression pushed by the server, one of `no`, `asym`, or `yes`.

`no` is used by default and permits only compression stub framing. `asym` accepts compressed packets from the server but does not compress outgoing packets. `yes` permits compression in both directions.

Conflict with non-stub compression enabled by `compression` or `compression_lzo` when set to `no`.

### route_no_pull

Ignore routes, DNS and DHCP settings, route metrics, `redirect-gateway`,
`redirect-private`, `block-ipv6`, and `block-outside-dns` pushed by the server.

Interface configuration, topology, tunnel MTU, `route-gateway`, and locally configured routes are still used.

Disabled by default.

### pull_filters

Ordered filters for options pushed by the server.

The first filter whose `text` is a case-sensitive prefix of the complete pushed option is applied. Options that match no filter are accepted.

### pull_filters.action

==Required==

Filter action, one of `accept`, `ignore`, or `reject`.

`accept` applies the option, `ignore` discards it, and `reject` terminates the connection.

### pull_filters.text

==Required==

Case-sensitive prefix to match against the pushed option name and value.

For example, `route ` matches pushed IPv4 route options without matching `route-gateway`.

### routes

IPv4 and IPv6 route prefixes routed through the OpenVPN endpoint.

These routes are used in addition to routes accepted from the server.

### route_gateway

IPv4 gateway for routes through the OpenVPN endpoint.

When empty, the VPN gateway received from the server is used.

### route_metric

Default metric for routes through the OpenVPN endpoint.

The platform default is used when `0`.

### redirect_gateway

Route all IPv4 traffic through the OpenVPN endpoint.

Disabled by default.

### redirect_gateway_flags

OpenVPN `redirect-gateway` flags.

`!ipv4` disables the IPv4 default route, and `ipv6` also routes all IPv6 traffic through the endpoint. Other OpenVPN flags are accepted for compatibility but do not change endpoint routing.

Empty by default.

### ping_interval

Interval after which the client sends a data-channel ping when no packet has been sent to the server.

A server-pushed OpenVPN `ping` value overrides this value.

The value must use whole seconds.

Disabled by default.

### ping_restart

Time without receiving a packet after which the client reconnects to the server.

A server-pushed OpenVPN `ping-restart` value overrides this value.

The value must use whole seconds.

When empty, `120s` is used for UDP connections with pull enabled until the
server pushes another value. No default receive timeout is used for TCP.

### renegotiate_interval

OpenVPN TLS renegotiation interval.

When empty, the OpenVPN default `1h` is used.

### explicit_exit_notify

Number of OpenVPN exit notifications sent when closing a UDP connection.

Notifications are sent one second apart. Disabled when `0`.

### system

Use a system interface.

Requires privilege and cannot conflict with existing system interfaces.

If disabled, sing-box uses the internal network stack.

### name

Custom interface name for the system interface.

An automatically generated `ovpn` interface name is used by default.

### mtu

OpenVPN interface MTU.

When empty, `1500` is used until a server-pushed MTU is received.

## UDP NAT Fields

See [UDP NAT Fields](/configuration/shared/udp-nat/) for details.

## Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.

## Interactive authentication

Use `Tools` > `Endpoints` in the sing-box dashboard or any sing-box graphical client to authenticate and manage the endpoint.
