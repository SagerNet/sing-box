---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.14.0"

    :material-plus: [account_key](#account_key)  
    :material-plus: [key_type](#key_type)  
    :material-plus: [detour](#detour)

# ACME

!!! quote ""

    `with_acme` build tag required.

### Structure

```json
{
  "type": "acme",
  "tag": "",

  "domain": [],
  "data_directory": "",
  "default_server_name": "",
  "email": "",
  "provider": "",
  "account_key": "",
  "disable_http_challenge": false,
  "disable_tls_alpn_challenge": false,
  "alternative_http_port": 0,
  "alternative_tls_port": 0,
  "external_account": {
    "key_id": "",
    "mac_key": ""
  },
  "dns01_challenge": {},
  "key_type": "",
  "detour": ""
}
```

### Fields

#### domain

==Required==

List of domains.

#### data_directory

The directory to store ACME data.

`$XDG_DATA_HOME/certmagic|$HOME/.local/share/certmagic` will be used if empty.

#### default_server_name

Server name to use when choosing a certificate if the ClientHello's ServerName field is empty.

#### email

The email address to use when creating or selecting an existing ACME server account.

#### provider

The ACME CA provider to use.

| Value                   | Provider      |
|-------------------------|---------------|
| `letsencrypt (default)` | Let's Encrypt |
| `zerossl`               | ZeroSSL       |
| `https://...`           | Custom        |

When `provider` is `zerossl`, sing-box will automatically request ZeroSSL EAB credentials if `email` is set and
`external_account` is empty.

When `provider` is `zerossl`, at least one of `external_account`, `email`, or `account_key` is required.

#### account_key

!!! question "Since sing-box 1.14.0"

The PEM-encoded private key of an existing ACME account.

#### disable_http_challenge

Disable all HTTP challenges.

#### disable_tls_alpn_challenge

Disable all TLS-ALPN challenges

#### alternative_http_port

The alternate port to use for the ACME HTTP challenge; if non-empty, this port will be used instead of 80 to spin up a
listener for the HTTP challenge.

#### alternative_tls_port

The alternate port to use for the ACME TLS-ALPN challenge; the system must forward 443 to this port for challenge to
succeed.

#### external_account

EAB (External Account Binding) contains information necessary to bind or map an ACME account to some other account known
by the CA.

External account bindings are used to associate an ACME account with an existing account in a non-ACME system, such as
a CA customer database.

To enable ACME account binding, the CA operating the ACME server needs to provide the ACME client with a MAC key and a
key identifier, using some mechanism outside of ACME. §7.3.4

#### external_account.key_id

The key identifier.

#### external_account.mac_key

The MAC key.

#### dns01_challenge

ACME DNS01 challenge field. If configured, other challenge methods will be disabled.

See [DNS01 Challenge Fields](/configuration/shared/dns01_challenge/) for details.

#### key_type

!!! question "Since sing-box 1.14.0"

The private key type to generate for new certificates.

| Value      | Type    |
|------------|---------|
| `ed25519`  | Ed25519 |
| `p256`     | P-256   |
| `p384`     | P-384   |
| `rsa2048`  | RSA     |
| `rsa4096`  | RSA     |

#### detour

!!! question "Since sing-box 1.14.0"

The tag of the upstream outbound.

All provider HTTP requests will use this outbound.
