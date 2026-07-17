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
  "server": "vpn.example.com",
  "flavor": "anyconnect",
  "username": "",
  "password": "",
  "auth_group": "",
  "token": {
    "mode": "",
    "secret": "",
    "pin": "",
    "password": "",
    "device_id": "",
    "counter": 0
  },
  "reported_os": "",
  "user_agent": "",
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
  "no_udp": false,
  "allow_insecure_crypto": false,
  "tls": {
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

### token

Software token configuration for automatically answering matching token fields.

### token.mode

==Required==

Software token mode, one of:

- `totp`: Time-based One-Time Password.
- `hotp`: HMAC-based One-Time Password.
- `stoken`: RSA SecurID software token.

### token.secret

==Required==

Software token secret.

For `totp` and `hotp`, this can be a Base32 secret, a `base32:`-prefixed secret, or an `otpauth://` URI of the matching type.

For `stoken`, this is the encoded RSA SecurID CTF token content.

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

`anyconnect` uses `linux-64` by default. `gp` and `pulse` select a value based on the system platform by default.

### user_agent

User agent reported to the VPN server when supported by the selected flavor.

The default is flavor-specific.

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

### no_udp

Disable the DTLS or ESP secondary data channel and use the TLS data channel only.

### allow_insecure_crypto

Allow deprecated TLS and DTLS versions and cipher suites required by legacy VPN servers.

Disabled by default. This option does not disable server certificate verification.

### tls

OpenConnect TLS configuration.

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

## Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.

## Interactive authentication

Use `Tools` > `Endpoints` in the sing-box dashboard or any sing-box graphical client to authenticate and manage the endpoint.
