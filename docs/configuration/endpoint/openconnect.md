# OpenConnect Client

!!! question "Since sing-box 1.14.0"

==Client only==

## Structure

```json
{
  "type": "openconnect",
  "tag": "oc-client",

  "system": false,
  "name": "",

  ... // UDP NAT Fields

  "server": "vpn.example.com",
  "flavor": "anyconnect",
  "username": "",
  "password": "",
  "auth_group": "",
  "cookie": "",
  "token": {
    "mode": "",
    "secret": "",
    "secret_path": "",
    "pin": "",
    "password": "",
    "device_id": "",
    "counter": 0
  },
  "reported_os": "",
  "user_agent": "",
  "version": "",
  "local_hostname": "",
  "mobile": {
    "platform_version": "",
    "device_type": "",
    "device_unique_id": ""
  },
  "csd": {
    "wrapper_path": ""
  },
  "hip": {
    "wrapper_path": ""
  },
  "tncc": {
    "wrapper_path": "",
    "device_id": "",
    "user_agent": "",
    "machine_identification_enabled": false,
    "certificates": [
      {
        "certificate": [],
        "certificate_path": ""
      }
    ]
  },
  "fortinet_host_check": {
    "hostcheck": "",
    "check_virtual_desktop": ""
  },
  "no_udp": false,
  "dtls_local_port": 0,
  "compression_disabled": false,
  "compression_mode": "",
  "ipv6_disabled": false,
  "http_keepalive_disabled": false,
  "xml_post_disabled": false,
  "external_auth_disabled": false,
  "password_authentication_disabled": false,
  "tcp_keep_alive_enabled": false,
  "pfs": false,
  "mtu": 0,
  "base_mtu": 0,
  "dpd_interval": "",
  "reconnect_timeout": "",
  "trojan_interval": "",
  "queue_length": 0,
  "allow_insecure_crypto": false,
  "tls": {
    "insecure": false,
    "server_name": "",
    "peer_fingerprint": [],
    "system_trust_disabled": false,
    "certificate_authority": [],
    "certificate_authority_path": "",
    "client_certificate": [],
    "client_certificate_path": "",
    "client_key": [],
    "client_key_path": "",
    "client_key_password": "",
    "mca_certificate": [],
    "mca_certificate_path": "",
    "mca_key": [],
    "mca_key_path": "",
    "mca_key_password": ""
  },
  "form_entries": [
    {
      "form_id": "",
      "submission_key": "",
      "name": "",
      "value": "",
      "promote": false
    }
  ],

  ... // Dial Fields
}
```

!!! note ""

    You can ignore the JSON Array [] tag when the content is only one item.

## Fields

### system

Use a system interface.

Requires privilege and cannot conflict with existing system interfaces.

If disabled, sing-box uses the internal network stack.

### name

Custom interface name for the system interface.

An automatically generated `oc` interface name is used by default.

### server

==Required==

OpenConnect VPN server HTTPS URL.

The `https://` scheme is added if omitted. URL user information, queries, and fragments are not supported.

### flavor

OpenConnect protocol flavor, one of `anyconnect`, `gp`, `fortinet`, `f5`, `pulse`, or `nc`.

`anyconnect` is used by default.

### username

Username used to fill matching authentication form fields.

### password

Password used to fill matching authentication form fields.

### auth_group

Authentication group used to preselect a matching group, realm, domain, or gateway choice when supported by the selected flavor.

### cookie

Existing authentication session used to connect without first prompting for credentials.

The accepted format depends on `flavor`:

- `anyconnect`: A `webvpn` value, or a semicolon-separated cookie list containing `webvpn`.
- `gp`: The complete authenticated query string returned by GlobalProtect authentication.
- `nc`: A `DSID` value, or a semicolon-separated cookie list containing `DSID`.
- `pulse`: The raw Pulse authentication cookie value.
- `f5`: An `MRHSession` value, or a semicolon-separated cookie list containing `MRHSession` and optionally `F5_ST`.
- `fortinet`: An `SVPNCOOKIE` value, or a semicolon-separated cookie list containing `SVPNCOOKIE`.

If the server rejects the supplied session, normal authentication is attempted.

### token

Token configuration for automatically answering matching token fields or HTTP Bearer authentication.

One of `token.secret` or `token.secret_path` is required.

### token.mode

==Required==

Token mode, one of:

- `totp`: Time-based One-Time Password.
- `hotp`: HMAC-based One-Time Password.
- `stoken`: RSA SecurID software token.
- `oidc`: OIDC access token used for HTTP Bearer authentication.

### token.secret

Software token secret.

For `totp` and `hotp`, this can be a Base32 secret, a `base32:`-prefixed secret, or an `otpauth://` URI of the matching type.

For `stoken`, this is the encoded RSA SecurID CTF token content.

For `oidc`, this is the access token value. It is sent only after the VPN server requests HTTP Bearer authentication.

Conflict with `token.secret_path`.

### token.secret_path

Path to the software token secret or OIDC access token.

Conflict with `token.secret`.

### token.pin

RSA SecurID PIN for `stoken` mode.

### token.password

Password for decrypting a password-protected RSA SecurID token in `stoken` mode.

### token.device_id

Device ID for decrypting a device-bound RSA SecurID token in `stoken` mode.

### token.counter

Initial counter for `hotp` mode.

If zero, the counter from an `otpauth://` URI is used when present; otherwise the counter starts at zero.

### reported_os

Operating system identity reported to the VPN server when supported by the selected flavor.

For `anyconnect`, `gp`, and `pulse`, the supported values are `linux`, `linux-64`, `win`, `mac-intel`, `android`, and `apple-ios`.

The default is selected from the system platform: `win` on Windows, `mac-intel` on macOS, `android` on Android, `apple-ios` on iOS, and `linux-64` or `linux` on other 64-bit or 32-bit systems.

### user_agent

User agent reported to the VPN server when supported by the selected flavor.

The default is flavor-specific. AnyConnect, Network Connect, Pulse, and F5 use `AnyConnect-compatible OpenConnect VPN Agent v9.21`; GlobalProtect uses `PAN GlobalProtect`; Fortinet uses `Mozilla/5.0 SV1`.

### version

Client version reported separately from `user_agent` when supported by the selected flavor.

`v9.21` is used by default. Currently used by AnyConnect XML authentication.

### local_hostname

Local hostname reported to the VPN server when supported by the selected flavor.

The system hostname is used by default, or `localhost` if it is unavailable.

### mobile

AnyConnect mobile client identity. When configured, all three fields are required and are reported during XML authentication and tunnel establishment.

### mobile.platform_version

Mobile operating system version reported to the AnyConnect server.

### mobile.device_type

Mobile device model or type reported to the AnyConnect server.

### mobile.device_unique_id

Mobile device identifier reported to the AnyConnect server.

### csd

AnyConnect CSD/host scan compliance options.

Built-in CSD handling is used by default when requested by the server.

### csd.wrapper_path

Path to an external AnyConnect CSD wrapper executable.

Built-in CSD handling is used if empty.

### hip

GlobalProtect HIP check and report options.

Built-in HIP reporting is used by default when requested by the server.

### hip.wrapper_path

Path to an external GlobalProtect HIP report wrapper executable.

Built-in HIP reporting is used if empty.

### tncc

Network Connect TNCC compliance options.

Built-in TNCC handling is used by default when requested by the server.

### tncc.wrapper_path

Path to an external Network Connect TNCC wrapper executable.

Built-in TNCC handling is used if empty.

Conflict with `tncc.device_id`, `tncc.user_agent`, `tncc.machine_identification_enabled`, and `tncc.certificates`.

### tncc.device_id

Device ID reported by the built-in TNCC handler.

Conflict with `tncc.wrapper_path`.

### tncc.user_agent

User agent used by the built-in TNCC handler.

`Neoteris HC Http` is used by default.

Conflict with `tncc.wrapper_path`.

### tncc.machine_identification_enabled

Enable built-in TNCC machine identification, including the platform, hostname, and observed MAC addresses.

Conflict with `tncc.wrapper_path`.

### tncc.certificates

Machine certificates used by the built-in TNCC handler to answer certificate requests.

Requires `tncc.machine_identification_enabled`.

Conflict with `tncc.wrapper_path`.

### tncc.certificates.certificate

TNCC machine certificate content in PEM format.

Conflict with `tncc.certificates.certificate_path`.

### tncc.certificates.certificate_path

TNCC machine certificate path in PEM format.

Conflict with `tncc.certificates.certificate`.

### fortinet_host_check

Fortinet hostcheck result override.

Hostcheck is disabled by default. It is enabled only when `fortinet_host_check.hostcheck` is non-empty. No operating system, security product, or network interface information is collected automatically.

When enabled and a successful Fortinet login response requests hostcheck, both configured values are submitted to the server before the VPN session is used. The values are sent unchanged as `application/x-www-form-urlencoded` fields.

Some Fortinet servers only request hostcheck from recognized FortiClient user agents. Configure `user_agent` when required by the server policy.

### fortinet_host_check.hostcheck

Fortinet hostcheck result string.

The conventional format is `<security-status>,<os-version>`, for example `0100,10.0.19042`. `security-status` contains four `0` or `1` characters representing, in order, third-party firewall, third-party antivirus, FortiClient firewall, and FortiClient antivirus.

An empty value disables Fortinet hostcheck, even if `fortinet_host_check.check_virtual_desktop` is configured.

### fortinet_host_check.check_virtual_desktop

Fortinet virtual desktop check result string.

FortiClient conventionally sends colon-separated MAC addresses joined by `|`, for example `74:78:27:4d:81:93|84:1b:77:3a:95:84`. An empty value is submitted as an empty field when hostcheck is enabled.

### no_udp

Disable the DTLS or ESP secondary data channel and use the TLS data channel only.

### dtls_local_port

Local UDP port used by the direct DTLS or ESP secondary data channel.

An automatically selected ephemeral port is used by default.

### compression_disabled

Disable AnyConnect compression negotiation.

By default, stateless `oc-lz4` and `lzs` compression is negotiated for CSTP and DTLS when supported by the server.

Compression can weaken traffic confidentiality when an attacker can influence plaintext sent through the VPN tunnel.

Conflict with `compression_mode` set to `all`.

### compression_mode

AnyConnect compression mode, one of:

- `stateless`: Advertise stateless `oc-lz4` and `lzs` compression.
- `all`: Additionally advertise stateful `deflate` compression for CSTP.

`stateless` is used by default. DTLS always uses stateless compression, including when `all` is selected.

Stateful compression has additional traffic confidentiality risks and should only be enabled when required by the VPN server.

### ipv6_disabled

Disable requesting and using IPv6 tunnel configuration.

### http_keepalive_disabled

Disable HTTP connection reuse during authentication and configuration requests.

### xml_post_disabled

Disable AnyConnect XML POST authentication and start authentication with the legacy GET flow.

### external_auth_disabled

Disable external browser authentication such as SSO and SAML for AnyConnect and GlobalProtect.

When enabled, external authentication is not advertised to the server and an unexpected external authentication request is rejected.

### password_authentication_disabled

Abort AnyConnect authentication if the server returns a non-success authentication form, matching OpenConnect `--no-passwd` behavior.

This does not affect the other flavors or a session supplied by `cookie`.

### tcp_keep_alive_enabled

Enable TCP keep alive for direct VPN server connections.

Disabled by default to match OpenConnect. Setting `tcp_keep_alive` or `tcp_keep_alive_interval` also enables it without requiring this field. When enabled without either duration, the operating system TCP keep alive timing is retained.

Conflict with `disable_tcp_keep_alive`.

### pfs

Require forward-secret TLS cipher suites for TLS 1.2 and earlier.

Disabled by default for compatibility with VPN servers that require RSA key exchange. This does not enable deprecated cipher suites; see `allow_insecure_crypto` for legacy crypto support.

### mtu

Preferred tunnel MTU.

The negotiated MTU is limited to this value for all flavors. For AnyConnect, this value is also sent to the server. GlobalProtect, F5, and Fortinet remove their protocol overhead before using it as the tunnel MTU.

Non-zero values below `576` are treated as `576`. The maximum value is `65535`.

### base_mtu

Base path MTU used to calculate the AnyConnect, GlobalProtect, F5, and Fortinet tunnel MTU after outer IP, transport, and protocol overhead.

`1406` is used by default.

These flavors treat values below `1280` as `1280`. The maximum value is `65535`.

### dpd_interval

Override the Dead Peer Detection interval.

The server-provided or flavor-specific interval is used by default.

Positive values below `2s` are treated as `2s`. The value must not be negative.

### reconnect_timeout

Maximum accumulated backoff time after failed reconnect attempts. The first reconnect attempt starts immediately, and this timeout does not cancel an attempt already in progress.

`300s` is used by default.

The value must not be negative.

### trojan_interval

Override the interval between GlobalProtect HIP reports or Network Connect TNCC checks.

The server-provided interval is used by default. GlobalProtect uses `1h` when the server does not provide one.

The value must not be negative.

### queue_length

Inbound and outbound packet queue length between the VPN transport and the tunnel interface.

`32` is used by default. A full queue applies backpressure until its consumer makes room; queued packets are not discarded.

### allow_insecure_crypto

Enable weak TLS and DTLS cipher suites and TLS 1.0 compatibility required by legacy VPN servers.

Disabled by default; TLS versions below 1.2 are otherwise rejected. This option does not disable server certificate verification.

### tls

OpenConnect TLS configuration.

### tls.insecure

Disable verification of the VPN server certificate and hostname.

Disabled by default. Enabling this permits an active attacker to impersonate the VPN server. Prefer `tls.certificate_authority` or `tls.peer_fingerprint` when possible.

### tls.server_name

Server name used for TLS SNI and certificate hostname verification.

The hostname from `server` is used by default.

### tls.peer_fingerprint

Allowed server certificate fingerprints. A single string or a list can be specified.

Supported formats:

- An unprefixed SHA-1 certificate fingerprint compatible with OpenConnect `--servercert`.
- `sha1:<hex>`: SHA-1 SPKI fingerprint.
- `sha256:<hex>`: SHA-256 SPKI fingerprint.
- `pin-sha256:<base64>`: Base64-encoded SHA-256 SPKI pin.

The encoded fingerprint in every format can be abbreviated to a prefix of at least four characters. When configured, the peer certificate must match one of these fingerprints; a match can authorize a certificate that is not otherwise trusted.

### tls.system_trust_disabled

Disable the system CA certificate pool.

Use `tls.certificate_authority` or `tls.peer_fingerprint` to establish trust when enabled.

### tls.certificate_authority

Additional trusted CA certificate content in PEM format.

The certificates are added to the system certificate pool.

Conflict with `tls.certificate_authority_path`.

### tls.certificate_authority_path

Path to additional trusted CA certificates in PEM format.

The certificates are added to the system certificate pool.

Conflict with `tls.certificate_authority`.

### tls.client_certificate

Client certificate chain content in PEM format.

Conflict with `tls.client_certificate_path`.

### tls.client_certificate_path

Client certificate chain path in PEM format.

Conflict with `tls.client_certificate`.

### tls.client_key

Client private key content in PEM format.

Conflict with `tls.client_key_path`.

### tls.client_key_path

Client private key path in PEM format.

Conflict with `tls.client_key`.

The client certificate and key must both be set or both be empty.

### tls.client_key_password

Password for the encrypted client private key.

### tls.mca_certificate

AnyConnect multiple-certificate authentication (MCA) certificate chain content in PEM format.

Conflict with `tls.mca_certificate_path`.

### tls.mca_certificate_path

AnyConnect multiple-certificate authentication (MCA) certificate chain path in PEM format.

Conflict with `tls.mca_certificate`.

### tls.mca_key

AnyConnect multiple-certificate authentication (MCA) private key content in PEM format.

Conflict with `tls.mca_key_path`.

### tls.mca_key_path

AnyConnect multiple-certificate authentication (MCA) private key path in PEM format.

Conflict with `tls.mca_key`.

The MCA certificate and key must both be set or both be empty.

### tls.mca_key_password

Password for the encrypted MCA private key.

### form_entries

Authentication form field overrides.

An entry matches by `submission_key` when set, or by the combination of `form_id` and `name`. Later matching entries take precedence.

### form_entries.form_id

Authentication form identifier used with `form_entries.name` when `form_entries.submission_key` is empty.

### form_entries.submission_key

Authentication field submission key.

Either `form_entries.submission_key` or both `form_entries.form_id` and `form_entries.name` are required.

### form_entries.name

Authentication field name used with `form_entries.form_id` when `form_entries.submission_key` is empty.

### form_entries.value

Value supplied automatically for the matching authentication field.

Conflict with `form_entries.promote`.

### form_entries.promote

Ask for the matching authentication field interactively instead of supplying an automatic value.

Conflict with `form_entries.value`.

## UDP NAT Fields

See [UDP NAT Fields](/configuration/shared/udp-nat/) for details.

## Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.

## Interactive authentication

Use `Tools` > `Endpoints` in the sing-box dashboard or any sing-box graphical client to authenticate and manage the endpoint.

## DNS

Pushed DNS settings are not installed into the operating system. Configure an [OpenConnect DNS server](/configuration/dns/server/openconnect/) to use them through sing-box.
