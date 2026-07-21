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
  "mode": "tls",
  "network": "udp",
  "remote": "",
  "remote_port": 0,
  "max_clients": 1024,
  "address": [],
  "peer_address": "",
  "peer_address_ipv6": "",
  "topology": "subnet",
  "duplicate_cn": false,
  "users": [
    {
      "username": "",
      "password": ""
    }
  ],
  "static_key": [],
  "static_key_path": "",
  "key_direction": "",
  "tls": {
    "certificate": [],
    "certificate_path": "",
    "key": [],
    "key_path": "",
    "client_certificate": [],
    "client_certificate_path": "",
    "verify_client_certificate": "require",
    "client_name": "",
    "client_name_type": "name",
    "peer_fingerprint": [],
    "crl_path": "",
    "remote_certificate_ku": [],
    "remote_certificate_eku": "",
    "remote_certificate_tls": "",
    "certificate_profile": "",
    "ns_certificate_type": "",
    "version_min": "1.2",
    "version_max": "",
    "cipher": "",
    "groups": "",
    "control_wrap": {
      "type": "tls_crypt",
      "key": [],
      "key_path": "",
      "direction": "",
      "force_cookie": false
    }
  },
  "cipher": "",
  "data_ciphers": [],
  "data_ciphers_fallback": "",
  "auth": "",
  "mss_fix": 0,
  "mss_fix_disabled": false,
  "mss_fix_mode": "",
  "replay_window": 0,
  "replay_window_time": "",
  "push": {
    "routes": [],
    "dns": [],
    "dns_servers": [],
    "search_domains": [],
    "dhcp_options": [],
    "redirect_gateway": false,
    "redirect_gateway_flags": [],
    "block_outside_dns": false,
    "ping_interval": "",
    "ping_restart": ""
  },
  "ping_interval": "",
  "ping_restart": "",
  "renegotiate_interval": "",
  "renegotiate_disabled": false,
  "renegotiate_bytes": 0,
  "renegotiate_packets": 0,
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

The endpoint configures interface addresses and MTU but does not install
operating-system routes or DNS settings.

If disabled, sing-box uses the internal network stack.

### name

Custom interface name for system interface.

An automatically generated `ovpn` interface name is used by default.

### mtu

OpenVPN interface MTU.

`1500` will be used by default.

### mode

OpenVPN session mode, one of `tls` or `static_key`.

`tls` is used by default.

`static_key` serves one peer without a TLS control channel or forward secrecy.
It is retained as an explicit compatibility option for immutable deployments.
It does not use `tls`, `users`, push options, or TLS renegotiation options.

### network

OpenVPN transport network, one of `udp` or `tcp`.

`udp` will be used by default.

Only one transport network is served per endpoint; to serve both TCP and UDP,
configure two endpoints with separate `address` subnets,
matching upstream OpenVPN which requires two server processes.

### remote

Fixed remote peer address for a UDP `static_key` server.

Required with `remote_port` in UDP `static_key` mode. TCP servers accept the
single peer from the listening socket and do not use this field.

### remote_port

Fixed remote peer port for a UDP `static_key` server.

Required with `remote` in UDP `static_key` mode.

### max_clients

Maximum number of established and pending TLS client sessions.

`1024` is used by default. The value must be smaller than `16777216`, the size of the OpenVPN peer-id space.

`static_key` mode supports one peer, so this value must be `0` or `1`.

### address

==Required==

List of OpenVPN server address prefixes.

At most one IPv4 prefix and one IPv6 prefix are supported.

The prefix address is assigned to the server interface. The masked prefix is used as the client address pool and route.

The first IPv4 and IPv6 prefix addresses are used as the endpoint's local addresses.

In `static_key` mode these are the local tunnel prefixes rather than address pools.

### peer_address

IPv4 tunnel peer address.

Required when an IPv4 `address` is configured in `static_key` mode.

### peer_address_ipv6

IPv6 tunnel peer address.

Required when an IPv6 `address` is configured in `static_key` mode.

### topology

OpenVPN topology pushed to clients, one of `subnet`, `p2p` or `net30`.

`subnet` is used by default in TLS mode. `p2p` is used by default in
`static_key` mode.

### duplicate_cn

Allow multiple active clients with the same authenticated certificate common name or username.

When disabled, a newly authenticated session replaces the existing session with the same identity and reuses its tunnel address when available.

Disabled by default.

Only available in TLS mode.

### users

List of OpenVPN username/password users.

If set, clients must pass username/password authentication in addition to any certificate policy configured by `tls.verify_client_certificate`.

Only available in TLS mode.

### users.username

Username.

### users.password

Password.

### static_key

OpenVPN static key content.

Required in `static_key` mode.

Conflict with `static_key_path`.

### static_key_path

OpenVPN static key path.

Required in `static_key` mode when `static_key` is not set.

Conflict with `static_key`.

### key_direction

Static key direction, one of `server` or `client`.

The key is used bidirectionally if empty. Conventionally the server uses
`server` and the peer uses `client`.

Only available in `static_key` mode.

### tls

Required in TLS mode.

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

One of `tls.client_certificate`, `tls.client_certificate_path`, or `tls.peer_fingerprint` is required when `tls.verify_client_certificate` is `require` or `optional`.

Conflict with `tls.client_certificate_path`.

### tls.client_certificate_path

TLS CA certificate path, used to verify client certificates.

One of `tls.client_certificate`, `tls.client_certificate_path`, or `tls.peer_fingerprint` is required when `tls.verify_client_certificate` is `require` or `optional`.

Conflict with `tls.client_certificate`.

### tls.verify_client_certificate

OpenVPN client certificate policy, one of `require`, `optional` or `none`.

`require` will be used by default.

If set to `optional`, a client certificate is verified when provided, but clients without a certificate are allowed.

If set to `none`, client certificates are not requested.

This field does not replace `users`; when `users` is set, username/password authentication is still required.

### tls.client_name

Expected client certificate name. Disabled when empty.

### tls.client_name_type

Certificate field matched by `tls.client_name`, one of `subject`, `name`, or `name-prefix`.

`name` is used by default when `tls.client_name` is configured.

### tls.peer_fingerprint

Allowed SHA-256 fingerprints of client leaf certificates. Fingerprint-only verification can be used without a client CA.

### tls.crl_path

Path to a certificate revocation list used to reject revoked client certificates.

### tls.remote_certificate_ku

Required client certificate key usage masks in OpenVPN `remote-cert-ku` format.

### tls.remote_certificate_eku

Required client certificate extended key usage. Conflict with an explicitly configured `tls.remote_certificate_tls`.

### tls.remote_certificate_tls

Client certificate purpose check, one of `server`, `client`, or `none`. `client` is used by default.

### tls.certificate_profile

Certificate profile, one of `insecure`, `legacy`, `preferred`, or `suiteb`.

`legacy` is used by default.

`insecure` accepts MD5- and SHA-1-signed certificate chains and smaller legacy
keys for compatibility with immutable peers. Use it only when the peer cannot
be upgraded. `legacy` accepts SHA-1 but rejects MD5 signatures; `preferred`
requires stronger signatures and keys.

When `suiteb` is selected and `tls.cipher` is empty, the TLS 1.2 cipher list defaults to the Suite B ECDHE-ECDSA AES-GCM suites. Explicit `tls.cipher` and `tls.groups` values are not restricted by the profile.

### tls.ns_certificate_type

Deprecated Netscape certificate type check, one of `server` or `client`.

### tls.version_min

Minimum TLS version. `1.2` is used by default.

### tls.version_max

Maximum TLS version. The maximum supported version is used by default.

### tls.cipher

Colon-separated OpenSSL cipher suite names allowed for TLS 1.2 and earlier.

The default TLS cipher suites are used when empty. TLS 1.3 cipher suites are not controlled by this field.

### tls.groups

Colon-separated TLS key exchange groups in preference order.

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

### cipher

Data-channel cipher used in `static_key` mode.

The upstream static-key default `BF-CBC` is used when empty. Supported
static-key ciphers are the AES-CBC, ARIA-CBC, Camellia-CBC, DES-CBC,
Blowfish-CBC, CAST5-CBC families, `SEED-CBC`, `SM4-CBC`, and `NONE`.

Only available in `static_key` mode. `NONE` provides no confidentiality.

### data_ciphers

Allowed OpenVPN data channel ciphers.

`AES-256-GCM`, `AES-128-GCM` and `CHACHA20-POLY1305` are used by default.

The AES-GCM family includes `AES-192-GCM`. Retained ciphers include the CBC,
CFB, and OFB forms of AES, ARIA, Camellia, DES, Blowfish, and CAST5, the CBC,
CFB, and OFB forms of SEED and SM4, and `NONE`. CFB and OFB are available only
in TLS mode. Legacy ciphers provide weaker or no confidentiality and are not
enabled by default.

Only available in TLS mode.

### data_ciphers_fallback

OpenVPN data channel cipher for legacy clients that do not support cipher negotiation.

Equivalent to OpenVPN `data-ciphers-fallback`.

Disabled by default.

Only available in TLS mode.

### auth

OpenVPN data channel authentication digest.

`SHA1` will be used by default, matching the upstream default; it only applies to non-AEAD data ciphers and `tls_auth`.

Legacy digests including `MD5` and `RIPEMD160` remain available when explicitly
configured for compatibility.

### mss_fix

Maximum encapsulated packet size used to clamp TCP MSS. The upstream default calculation uses `1492` with the default MTU.

### mss_fix_disabled

Disable MSS clamping, including the default clamp.

### mss_fix_mode

Calculation mode for an explicit `mss_fix`, one of `mtu` or `fixed`. Requires `mss_fix`.

### replay_window

UDP data-channel replay window size. `64` is used by default; TCP packet IDs remain strictly consecutive.

### replay_window_time

UDP replay window duration. `15s` is used by default. The value must use whole seconds.

### push

Options pushed to clients.

### push.routes

Routes to push to clients.

IPv4 and IPv6 prefixes can be mixed.

### push.dns

DNS server addresses to push to clients.

Uses legacy `dhcp-option DNS`/`DNS6`. A pushed modern DNS server group overrides these addresses on compatible clients.

### push.dns_servers

Modern OpenVPN DNS server groups to push. Each entry contains `priority`, `addresses`, optional `resolve_domains`, `dnssec`, `transport`, and `sni`.

Addresses accept an IP address or `IP:port` (IPv6 ports use `[IPv6]:port`). `transport` is one of `plain`, `dot`, or `doh`; `dnssec` is one of `yes`, `optional`, or `no`. OpenVPN clients apply only the group with the lowest priority number.

### push.search_domains

Modern OpenVPN search domains to push.

### push.dhcp_options

Additional legacy `dhcp-option` values to push, without the `dhcp-option` prefix.

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

Only available in TLS mode.

### renegotiate_disabled

Disable time-based TLS renegotiation, including the default interval.

Only available in TLS mode.

### renegotiate_bytes

Renegotiate data-channel keys after this many bytes. `0` uses the cipher-dependent OpenVPN default.

Only available in TLS mode.

### renegotiate_packets

Renegotiate data-channel keys after this many packets. `0` uses the cipher-dependent OpenVPN default.

Only available in TLS mode.

### handshake_window

Maximum time allowed for the initial TLS handshake and each TLS renegotiation.

`1m` is used by default.

Only available in TLS mode.

## UDP NAT Fields

These fields configure UDP sessions for traffic through the OpenVPN interface.

See [UDP NAT Fields](/configuration/shared/udp-nat/) for details.
