# OpenVPN Client

!!! question "Since sing-box 1.14.0"

## Structure

```json
{
  "type": "openvpn-client",
  "tag": "ovpn-client",

  "mode": "tls",
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
  "address": [],
  "peer_address": "",
  "peer_address_ipv6": "",
  "topology": "",
  "username": "",
  "password": "",
  "auth_retry": "none",
  "static_challenge": "",
  "static_challenge_echo": false,
  "static_key": [],
  "static_key_path": "",
  "key_direction": "",
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
    "remote_certificate_tls": "",
    "certificate_profile": "",
    "ns_certificate_type": "",
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
  "cipher": "",
  "data_ciphers": [],
  "data_ciphers_fallback": "",
  "auth": "",
  "mss_fix": 0,
  "mss_fix_disabled": false,
  "mss_fix_mode": "",
  "fragment": 0,
  "replay_window": 0,
  "replay_window_time": "",
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
  "redirect_private": false,
  "block_ipv6": false,
  "ping_interval": "",
  "ping_restart": "",
  "ping_restart_disabled": false,
  "renegotiate_interval": "",
  "renegotiate_disabled": false,
  "renegotiate_bytes": 0,
  "renegotiate_packets": 0,
  "tls_timeout": "",
  "handshake_window": "",
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

### mode

OpenVPN session mode, one of `tls` or `static_key`.

`tls` is used by default.

`static_key` is a deprecated OpenVPN mode without a TLS control channel or
forward secrecy. It is retained as an explicit compatibility option for
immutable enterprise VPN servers. It does not use `tls`, username/password
authentication, pull options, or TLS renegotiation options.

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

### address

Local IPv4 and IPv6 tunnel prefixes.

At least one address is required in `static_key` mode. In TLS mode these
addresses are optional and can be replaced by addresses pulled from the
server.

### peer_address

IPv4 tunnel peer address and VPN gateway.

Required when an IPv4 `address` is configured in `static_key` mode.

### peer_address_ipv6

IPv6 tunnel peer address and VPN gateway.

Required when an IPv6 `address` is configured in `static_key` mode.

### topology

Tunnel topology, one of `net30`, `p2p`, or `subnet`.

The topology pulled from the server is used when empty in TLS mode.

### username

Username for OpenVPN username/password authentication.

Only available in TLS mode.

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

The key is used bidirectionally if empty. Only available in `static_key` mode.

### tls

Required in TLS mode.

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

The certificate must contain all bits from at least one configured mask.

Disabled by default.

### tls.remote_certificate_eku

Required server certificate extended key usage.

OpenSSL names, object identifiers, and the aliases `server` and `client` are accepted.

When set, this field replaces the default `tls.remote_certificate_tls` check.

Conflict with an explicitly configured `tls.remote_certificate_tls`.

### tls.remote_certificate_tls

Peer certificate purpose check, one of `server`, `client`, or `none`.

`server` is used by default.

`none` disables the certificate purpose check.

Conflict with `tls.remote_certificate_eku`.

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

Disabled by default. Prefer `tls.remote_certificate_tls`.

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

### cipher

Data-channel cipher used in `static_key` mode.

The upstream static-key default `BF-CBC` is used when empty. `BF-CBC` is a
legacy cipher with a 64-bit block size; configure the cipher required by the
server explicitly whenever possible. Static-key ciphers include `BF-CBC`,
`CAST5-CBC`, `DES-CBC`, `DES-EDE-CBC`, `DES-EDE3-CBC`, the AES-CBC,
ARIA-CBC, and Camellia-CBC families, `SEED-CBC`, `SM4-CBC`, and `NONE`.

Only available in `static_key` mode. `NONE` provides no confidentiality.

### data_ciphers

Allowed OpenVPN data channel ciphers.

Only available in TLS mode.

`AES-256-GCM`, `AES-128-GCM`, and `CHACHA20-POLY1305` are used by default.

The AES-GCM family includes `AES-192-GCM`. Retained ciphers include the CBC,
CFB, and OFB forms of AES, ARIA, Camellia, DES, Blowfish, and CAST5, the CBC,
CFB, and OFB forms of SEED and SM4, and `NONE`. CFB and OFB are available only
in TLS mode. Legacy ciphers provide weaker or no confidentiality and are not
enabled by default.

### data_ciphers_fallback

Data channel cipher for peers that do not support cipher negotiation.

Disabled by default.

Only available in TLS mode.

### auth

OpenVPN data channel authentication digest.

`SHA1` is used by default. It only applies to non-AEAD data ciphers and `tls_auth`.

Legacy digests including `MD5` and `RIPEMD160` remain available when explicitly
configured for compatibility.

### mss_fix

Maximum OpenVPN UDP packet size used to clamp the MSS of TCP connections sent through the tunnel.

This prevents TCP packets from exceeding the path MTU after OpenVPN encapsulation.

When empty, the upstream OpenVPN default is used: `fragment` when configured,
otherwise `1492` for the default tunnel MTU or the configured tunnel MTU.

### mss_fix_disabled

Disable MSS clamping, including the default clamp.

Conflict with `mss_fix` and `mss_fix_mode`.

### mss_fix_mode

OpenVPN MSS calculation mode for an explicit `mss_fix`, one of `mtu` or `fixed`.

An empty value uses the normal OpenVPN encapsulation-aware calculation. `mtu` also accounts for the outer IP and UDP/TCP transport headers. `fixed` treats `mss_fix` as an inner IPv4 packet size.

Requires `mss_fix`.

### fragment

Maximum OpenVPN UDP packet size used for OpenVPN data channel fragmentation.

Disabled when `0`. A non-zero value must be at least `68`.

Conflict with TCP transport.

### replay_window

UDP data-channel replay window size. `64` is used by default. The maximum is `65536`.

TCP always requires strictly consecutive packet IDs.

### replay_window_time

UDP data-channel replay window duration. `15s` is used by default and the maximum is `10m`.

The value must use whole seconds.

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

`no` is used by default and permits only compression stub framing. `asym` accepts compressed packets from the server but does not compress outgoing packets. For OpenVPN 2.7 compatibility, `yes` is accepted as a legacy alias for `asym`; the client never sends compressed packets.

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

IPv4 and IPv6 prefixes preferred by sing-box routing for this OpenVPN endpoint.

These routes are used in addition to routes accepted from the server.

They do not install operating-system routes. Select the endpoint through
sing-box route rules or its preferred-route behavior.

### route_gateway

IPv4 gateway for routes through the OpenVPN endpoint.

When empty, the VPN gateway received from the server is used.

The value is retained for OpenVPN configuration compatibility; endpoint route
preference is prefix-based and does not install a system gateway route.

### route_metric

Default metric for routes through the OpenVPN endpoint.

The platform default is used when `0`.

The value is retained for OpenVPN configuration compatibility and does not
install a system route.

### redirect_gateway

Prefer the OpenVPN endpoint for all IPv4 destinations in sing-box routing.

Disabled by default.

This does not install an operating-system default route.

### redirect_gateway_flags

OpenVPN `redirect-gateway` flags.

`!ipv4` disables IPv4 preference, `def1` represents it with two `/1`
prefixes, and `ipv6` also prefers the upstream-specific IPv6 prefixes. The
OpenVPN control connection always uses its configured outbound dialer rather
than endpoint routes, so `local` and `autolocal` require no system-route
exception. `bypass-dhcp` and `bypass-dns` are not applicable because sing-box
does not install pushed DHCP or DNS settings into the operating system.
`block-local` is unsupported because the endpoint has no cross-platform source
for the physical default gateway needed to preserve the gateway exception.

Empty by default.

### redirect_private

Accept `redirect_gateway_flags` without adding a default-route preference. Routes pushed or configured separately still affect the endpoint's preferred addresses, but no operating-system routes are installed.

Disabled by default.

### block_ipv6

Reject IPv6 traffic locally instead of sending it through the VPN.

Disabled by default.

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

### ping_restart_disabled

Disable the initial `120s` UDP pull timeout and any locally configured ping restart timeout.

Conflict with `ping_restart`.

### renegotiate_interval

OpenVPN TLS renegotiation interval.

When empty, the OpenVPN default `1h` is used.

### renegotiate_disabled

Disable time-based TLS renegotiation, including the default interval.

Conflict with `renegotiate_interval`.

### renegotiate_bytes

Renegotiate data-channel keys after this many bytes. `0` uses the cipher-dependent OpenVPN default.

### renegotiate_packets

Renegotiate data-channel keys after this many packets. `0` uses the cipher-dependent OpenVPN default.

### tls_timeout

Initial retransmission timeout for TLS control packets. The OpenVPN default `2s` is used when empty.

### handshake_window

Maximum time allowed for the initial TLS handshake and each renegotiation. The OpenVPN default `1m` is used when empty.

### explicit_exit_notify

Number of OpenVPN exit notifications sent when closing a UDP connection.

Notifications are sent one second apart. Disabled when `0`.

### system

Use a system interface.

Requires privilege and cannot conflict with existing system interfaces.

The endpoint configures interface addresses and MTU but does not install
operating-system routes or DNS settings.

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
