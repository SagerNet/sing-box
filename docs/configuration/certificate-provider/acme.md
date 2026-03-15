---
icon: material/new-box
---

!!! question "Since sing-box 1.14.0"

# ACME

!!! quote ""

    `with_acme` build tag required.

!!! info ""

    This provider replaces deprecated inline `tls.acme` options.
    Fields without a version marker below are migrated from `tls.acme`;
    fields marked `Since sing-box 1.14.0` are newly added.

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
  "test_ca": "",
  "account_key": "",
  "trusted_roots_pem_files": [],
  "acme_timeout": "",
  "profile": "",
  "certificate_lifetime": "",
  "disable_http_challenge": false,
  "disable_tls_alpn_challenge": false,
  "alternative_http_port": 0,
  "alternative_tls_port": 0,
  "bind_host": "",
  "external_account": {
    "key_id": "",
    "mac_key": ""
  },
  "dns01_challenge": {},
  "preferred_chains": {
    "smallest": false,
    "root_common_name": [],
    "any_common_name": []
  },
  "key_type": "",
  "reuse_private_keys": false,
  "must_staple": false,
  "renewal_window_ratio": 0.0,
  "disable_ocsp_stapling": false,
  "ocsp_overrides": {}
}
```

### Fields

#### domain

List of domain.

ACME will be disabled if empty.

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

#### test_ca

!!! question "Since sing-box 1.14.0"

The test ACME directory URL to use for retries.

Must be `https://...` if set.

#### account_key

!!! question "Since sing-box 1.14.0"

The PEM-encoded private key of an existing ACME account.

#### trusted_roots_pem_files

!!! question "Since sing-box 1.14.0"

List of PEM files to trust when connecting to the ACME CA.

Useful for private ACME deployments or custom test CAs.

#### acme_timeout

!!! question "Since sing-box 1.14.0"

The maximum time allowed for an ACME obtain or renewal operation.

#### profile

!!! question "Since sing-box 1.14.0"

The ACME profile name to request from the CA.

Not all CAs support ACME profiles.

#### certificate_lifetime

!!! question "Since sing-box 1.14.0"

The certificate lifetime to request from the CA.

Not all CAs support custom certificate lifetimes.

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

#### bind_host

!!! question "Since sing-box 1.14.0"

The host to bind when starting HTTP-01 or TLS-ALPN challenge listeners.

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

#### dns01_challenge.ttl

!!! question "Since sing-box 1.14.0"

The TTL of the temporary TXT record used for the DNS challenge.

#### dns01_challenge.propagation_delay

!!! question "Since sing-box 1.14.0"

How long to wait after creating the challenge record before starting propagation checks.

#### dns01_challenge.propagation_timeout

!!! question "Since sing-box 1.14.0"

The maximum time to wait for the challenge record to propagate.

Set to `-1` to disable propagation checks.

#### dns01_challenge.resolvers

!!! question "Since sing-box 1.14.0"

Preferred DNS resolvers to use for DNS propagation checks.

#### dns01_challenge.override_domain

!!! question "Since sing-box 1.14.0"

Override the domain name used for the DNS challenge record.

Useful when `_acme-challenge` is delegated to a different zone.

For provider-specific fields, see [DNS01 Challenge Fields](/configuration/shared/dns01_challenge/).

#### preferred_chains

!!! question "Since sing-box 1.14.0"

Preferences for selecting alternate certificate chains offered by the CA.

At least one of `smallest`, `root_common_name`, or `any_common_name` must be set.

`root_common_name` and `any_common_name` are mutually exclusive.

#### preferred_chains.smallest

Prefer the smallest certificate chain.

#### preferred_chains.root_common_name

Prefer the first chain whose root issuer common name matches one of these values.

#### preferred_chains.any_common_name

Prefer the first chain whose issuer common name matches one of these values.

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

#### reuse_private_keys

!!! question "Since sing-box 1.14.0"

Reuse existing private keys from storage when renewing certificates.

#### must_staple

!!! question "Since sing-box 1.14.0"

Request certificates with the TLS Must-Staple extension.

#### renewal_window_ratio

!!! question "Since sing-box 1.14.0"

The fraction of certificate lifetime remaining when renewal should begin.

Must be greater than or equal to `0` and less than `1`.

If empty, certmagic's default renewal window is used.

#### disable_ocsp_stapling

!!! question "Since sing-box 1.14.0"

Disable automatic OCSP stapling.

#### ocsp_overrides

!!! question "Since sing-box 1.14.0"

A map of OCSP responder URLs to replacement URLs.

Set a replacement value to an empty string to disable querying that responder.
