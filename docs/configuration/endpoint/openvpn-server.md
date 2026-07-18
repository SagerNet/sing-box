# OpenVPN Server

!!! question "Since sing-box 1.14.0"

## Structure

```json
{
  "type": "openvpn-server",
  "tag": "ovpn-server",

  ... // Listen Fields

  "system": false,
  "name": "",
  "mtu": 1500,
  "network": "udp",
  "max_clients": 1024,
  "address": [],
  "topology": "subnet",
  "duplicate_cn": false,
  "users": [
    {
      "username": "",
      "password": ""
    }
  ],
  "tls": {
    "certificate": [],
    "certificate_path": "",
    "key": [],
    "key_path": "",
    "client_certificate": [],
    "client_certificate_path": "",
    "verify_client_certificate": "require",
    "control_wrap": {
      "type": "tls_crypt",
      "key": [],
      "key_path": "",
      "direction": "",
      "force_cookie": false
    }
  },
  "data_ciphers": [],
  "data_ciphers_fallback": "",
  "auth": "",
  "push": {
    "routes": [],
    "dns": [],
    "redirect_gateway": false,
    "redirect_gateway_flags": [],
    "block_outside_dns": false,
    "ping_interval": "",
    "ping_restart": ""
  },
  "ping_interval": "",
  "ping_restart": "",
  "renegotiate_interval": "",
  "handshake_window": "1m",

  ... // UDP NAT Fields
}
```

!!! note ""

    You can ignore the JSON Array [] tag when the content is only one item

## Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details. `udp_timeout` is part of the [UDP NAT Fields](#udp-nat-fields) below.

## Fields

### system

Use system interface.

Requires privilege and cannot conflict with existing system interfaces.

If disabled, sing-box uses the internal network stack.

### name

Custom interface name for system interface.

An automatically generated `ovpn` interface name is used by default.

### mtu

OpenVPN interface MTU.

`1500` will be used by default.

### network

OpenVPN transport network, one of `udp` or `tcp`.

`udp` will be used by default.

Only one transport network is served per endpoint; to serve both TCP and UDP,
configure two endpoints with separate `address` subnets,
matching upstream OpenVPN which requires two server processes.

### max_clients

Maximum number of established and pending TLS client sessions.

`1024` is used by default. The value must be smaller than `16777216`, the size of the OpenVPN peer-id space.

### address

==Required==

List of OpenVPN server address prefixes.

At most one IPv4 prefix and one IPv6 prefix are supported.

The prefix address is assigned to the server interface. The masked prefix is used as the client address pool and route.

The first IPv4 and IPv6 prefix addresses are used as the endpoint's local addresses.

### topology

OpenVPN topology pushed to clients, one of `subnet`, `p2p` or `net30`.

`subnet` will be used by default.

### duplicate_cn

Allow multiple active clients with the same authenticated certificate common name or username.

When disabled, a newly authenticated session replaces the existing session with the same identity and reuses its tunnel address when available.

Disabled by default.

### users

List of OpenVPN username/password users.

If set, clients must pass username/password authentication in addition to any certificate policy configured by `tls.verify_client_certificate`.

### users.username

Username.

### users.password

Password.

### tls

==Required==

OpenVPN control channel TLS configuration.

### tls.certificate

TLS server certificate content.

Either `tls.certificate` or `tls.certificate_path` is required.

Conflict with `tls.certificate_path`.

### tls.certificate_path

TLS server certificate path.

Either `tls.certificate` or `tls.certificate_path` is required.

Conflict with `tls.certificate`.

### tls.key

TLS server private key content.

Either `tls.key` or `tls.key_path` is required.

Conflict with `tls.key_path`.

### tls.key_path

TLS server private key path.

Either `tls.key` or `tls.key_path` is required.

Conflict with `tls.key`.

### tls.client_certificate

TLS CA certificate content, used to verify client certificates.

Either `tls.client_certificate` or `tls.client_certificate_path` is required.

Conflict with `tls.client_certificate_path`.

### tls.client_certificate_path

TLS CA certificate path, used to verify client certificates.

Either `tls.client_certificate` or `tls.client_certificate_path` is required.

Conflict with `tls.client_certificate`.

### tls.verify_client_certificate

OpenVPN client certificate policy, one of `require`, `optional` or `none`.

`require` will be used by default.

If set to `optional`, a client certificate is verified when provided, but clients without a certificate are allowed.

If set to `none`, client certificates are not requested.

This field does not replace `users`; when `users` is set, username/password authentication is still required.

### tls.control_wrap

OpenVPN control channel wrapping.

Equivalent to OpenVPN `tls-auth`, `tls-crypt` and `tls-crypt-v2`.

Disabled by default.

### tls.control_wrap.type

==Required==

Control channel wrapping type, one of `tls_auth`, `tls_crypt` or `tls_crypt_v2`.

For `tls_crypt_v2`, the key is the server key.

### tls.control_wrap.key

Control channel wrapping key content.

Either `tls.control_wrap.key` or `tls.control_wrap.key_path` is required.

Conflict with `tls.control_wrap.key_path`.

### tls.control_wrap.key_path

Control channel wrapping key path.

Either `tls.control_wrap.key` or `tls.control_wrap.key_path` is required.

Conflict with `tls.control_wrap.key`.

### tls.control_wrap.direction

OpenVPN `tls-auth` key direction, one of `server` or `client`.

Only available when `tls.control_wrap.type` is `tls_auth`.

`server` maps to OpenVPN key direction `0`, and `client` maps to `1`; by convention servers use `0` and clients use `1`.

If empty, the key is used bidirectionally, matching an omitted `key-direction` on both peers.

### tls.control_wrap.force_cookie

Require `tls-crypt-v2` clients over UDP to support stateless session cookies.

Only available when `tls.control_wrap.type` is `tls_crypt_v2`. When disabled,
clients without cookie support are accepted using the upstream `allow-noncookie` behavior.

Disabled by default.

### data_ciphers

Allowed OpenVPN data channel ciphers.

`AES-256-GCM`, `AES-128-GCM` and `CHACHA20-POLY1305` are used by default.

### data_ciphers_fallback

OpenVPN data channel cipher for legacy clients that do not support cipher negotiation.

Equivalent to OpenVPN `data-ciphers-fallback`.

Disabled by default.

### auth

OpenVPN data channel authentication digest.

`SHA1` will be used by default, matching the upstream default; it only applies to non-AEAD data ciphers and `tls_auth`.

### push

Options pushed to clients.

### push.routes

Routes to push to clients.

IPv4 and IPv6 prefixes can be mixed.

### push.dns

DNS server addresses to push to clients.

### push.redirect_gateway

Push `redirect-gateway` to clients, which routes client traffic through the VPN according to `push.redirect_gateway_flags`.

When `push.redirect_gateway_flags` is empty, `def1` is used by default.

### push.redirect_gateway_flags

OpenVPN `redirect-gateway` flags to push to clients.

Only available when `push.redirect_gateway` is enabled.

`def1` is used by default.

### push.block_outside_dns

Push `block-outside-dns` to clients, which blocks DNS queries outside the VPN on Windows clients.

### push.ping_interval

OpenVPN `ping` interval pushed to clients.

After the interval passes without sending a packet, the client sends a data-channel ping to the server.

The value must use whole seconds.

Disabled by default.

### push.ping_restart

OpenVPN `ping-restart` timeout pushed to clients.

After the timeout passes without receiving a packet, the client reconnects to the server.

The value must use whole seconds.

Disabled by default.

### ping_interval

Interval after which the server sends a data-channel ping when no packet has been sent to a client.

This value applies to the server. Use `push.ping_interval` to configure clients.

The value must use whole seconds.

Disabled by default.

### ping_restart

Time without receiving a packet after which the server closes the client session.

This value applies to the server. Use `push.ping_restart` to configure clients.

The server timeout should be longer than the client timeout so the client can reconnect before the server discards its session.

The value must use whole seconds.

Disabled by default.

### renegotiate_interval

OpenVPN TLS renegotiation interval.

When empty, the OpenVPN default `1h` is used.

### handshake_window

Maximum time allowed for the initial TLS handshake and each TLS renegotiation.

`1m` is used by default.

## UDP NAT Fields

These fields configure UDP sessions for traffic through the OpenVPN interface.

See [UDP NAT Fields](/configuration/shared/udp-nat/) for details.
