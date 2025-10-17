---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.13.0"

    :material-plus: [kernel_tx](#kernel_tx)
    :material-plus: [kernel_rx](#kernel_rx)
    :material-plus: [curve_preferences](#curve_preferences)
    :material-plus: [certificate_public_key_sha256](#certificate_public_key_sha256)
    :material-plus: [client_certificate](#client_certificate)
    :material-plus: [client_certificate_path](#client_certificate_path)
    :material-plus: [client_key](#client_key)
    :material-plus: [client_key_path](#client_key_path)
    :material-plus: [client_authentication](#client_authentication)
    :material-plus: [client_certificate_public_key_sha256](#client_certificate_public_key_sha256)

!!! quote "Changes in sing-box 1.12.0"

    :material-plus: [fragment](#fragment)  
    :material-plus: [fragment_fallback_delay](#fragment_fallback_delay)  
    :material-plus: [record_fragment](#record_fragment)  
    :material-delete-clock: [ech.pq_signature_schemes_enabled](#pq_signature_schemes_enabled)  
    :material-delete-clock: [ech.dynamic_record_sizing_disabled](#dynamic_record_sizing_disabled)

!!! quote "Changes in sing-box 1.10.0"

    :material-alert-decagram: [utls](#utls)  

### Inbound

```json
{
  "enabled": true,
  "server_name": "",
  "alpn": [],
  "min_version": "",
  "max_version": "",
  "cipher_suites": [],
  "curve_preferences": [],
  "certificate": [],
  "certificate_path": "",
  "client_authentication": "",
  "client_certificate": [],
  "client_certificate_path": [],
  "client_certificate_public_key_sha256": [],
  "key": [],
  "key_path": "",
  "kernel_tx": false,
  "kernel_rx": false,
  "acme": {
    "domain": [],
    "data_directory": "",
    "default_server_name": "",
    "email": "",
    "provider": "",
    "disable_http_challenge": false,
    "disable_tls_alpn_challenge": false,
    "alternative_http_port": 0,
    "alternative_tls_port": 0,
    "external_account": {
      "key_id": "",
      "mac_key": ""
    },
    "dns01_challenge": {}
  },
  "ech": {
    "enabled": false,
    "key": [],
    "key_path": "",

    // Deprecated
    
    "pq_signature_schemes_enabled": false,
    "dynamic_record_sizing_disabled": false
  },
  "reality": {
    "enabled": false,
    "handshake": {
      "server": "google.com",
      "server_port": 443,

      ... // Dial Fields
    },
    "private_key": "UuMBgl7MXTPx9inmQp2UC7Jcnwc6XYbwDNebonM-FCc",
    "short_id": [
      "0123456789abcdef"
    ],
    "max_time_difference": "1m"
  }
}
```

### Outbound

```json
{
  "enabled": true,
  "disable_sni": false,
  "server_name": "",
  "insecure": false,
  "alpn": [],
  "min_version": "",
  "max_version": "",
  "cipher_suites": [],
  "curve_preferences": [],
  "certificate": "",
  "certificate_path": "",
  "certificate_public_key_sha256": [],
  "client_certificate": [],
  "client_certificate_path": "",
  "client_key": [],
  "client_key_path": "",
  "fragment": false,
  "fragment_fallback_delay": "",
  "record_fragment": false,
  "ech": {
    "enabled": false,
    "config": [],
    "config_path": "",

    // Deprecated
    "pq_signature_schemes_enabled": false,
    "dynamic_record_sizing_disabled": false
  },
  "utls": {
    "enabled": false,
    "fingerprint": ""
  },
  "reality": {
    "enabled": false,
    "public_key": "jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0",
    "short_id": "0123456789abcdef"
  }
}
```

TLS version values:

* `1.0`
* `1.1`
* `1.2`
* `1.3`

Cipher suite values:

* `TLS_RSA_WITH_AES_128_CBC_SHA`
* `TLS_RSA_WITH_AES_256_CBC_SHA`
* `TLS_RSA_WITH_AES_128_GCM_SHA256`
* `TLS_RSA_WITH_AES_256_GCM_SHA384`
* `TLS_AES_128_GCM_SHA256`
* `TLS_AES_256_GCM_SHA384`
* `TLS_CHACHA20_POLY1305_SHA256`
* `TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA`
* `TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA`
* `TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA`
* `TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA`
* `TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256`
* `TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384`
* `TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256`
* `TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384`
* `TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256`
* `TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256`

!!! note ""

    You can ignore the JSON Array [] tag when the content is only one item

### Fields

#### enabled

Enable TLS.

#### disable_sni

==Client only==

Do not send server name in ClientHello.

#### server_name

Used to verify the hostname on the returned certificates unless insecure is given.

It is also included in the client's handshake to support virtual hosting unless it is an IP address.

#### insecure

==Client only==

Accepts any server certificate.

#### alpn

List of supported application level protocols, in order of preference.

If both peers support ALPN, the selected protocol will be one from this list, and the connection will fail if there is
no mutually supported protocol.

See [Application-Layer Protocol Negotiation](https://en.wikipedia.org/wiki/Application-Layer_Protocol_Negotiation).

#### min_version

The minimum TLS version that is acceptable.

By default, TLS 1.2 is currently used as the minimum when acting as a
client, and TLS 1.0 when acting as a server.

#### max_version

The maximum TLS version that is acceptable.

By default, the maximum version is currently TLS 1.3.

#### cipher_suites

List of enabled TLS 1.0–1.2 cipher suites. The order of the list is ignored.
Note that TLS 1.3 cipher suites are not configurable.

If empty, a safe default list is used. The default cipher suites might change over time.

#### curve_preferences

!!! question "Since sing-box 1.13.0"

Set of supported key exchange mechanisms. The order of the list is ignored, and key exchange mechanisms are chosen
from this list using an internal preference order by Golang.

Available values, also the default list:

* `P256`
* `P384`
* `P521`
* `X25519`
* `X25519MLKEM768`

#### certificate

Server certificates chain line array, in PEM format.

#### certificate_path

!!! note ""

    Will be automatically reloaded if file modified.

The path to server certificate chain, in PEM format.


#### certificate_public_key_sha256

!!! question "Since sing-box 1.13.0"

==Client only==

List of SHA-256 hashes of server certificate public keys, in base64 format.

To generate the SHA-256 hash for a certificate's public key, use the following commands:

```bash
# For a certificate file
openssl x509 -in certificate.pem -pubkey -noout | openssl pkey -pubin -outform der | openssl dgst -sha256 -binary | openssl enc -base64

# For a certificate from a remote server
echo | openssl s_client -servername example.com -connect example.com:443 2>/dev/null | openssl x509 -pubkey -noout | openssl pkey -pubin -outform der | openssl dgst -sha256 -binary | openssl enc -base64
```

#### client_certificate

!!! question "Since sing-box 1.13.0"

==Client only==

Client certificate chain line array, in PEM format.

#### client_certificate_path

!!! question "Since sing-box 1.13.0"

==Client only==

The path to client certificate chain, in PEM format.

#### client_key

!!! question "Since sing-box 1.13.0"

==Client only==

Client private key line array, in PEM format.

#### client_key_path

!!! question "Since sing-box 1.13.0"

==Client only==

The path to client private key, in PEM format.

#### key

==Server only==

The server private key line array, in PEM format.

#### key_path

==Server only==

!!! note ""

    Will be automatically reloaded if file modified.

The path to the server private key, in PEM format.

#### client_authentication

!!! question "Since sing-box 1.13.0"

==Server only==

The type of client authentication to use.

Available values:

* `no` (default)
* `request`
* `require-any`
* `verify-if-given`
* `require-and-verify`

One of `client_certificate`, `client_certificate_path`, or `client_certificate_public_key_sha256` is required
if this option is set to `verify-if-given`, or `require-and-verify`.

#### client_certificate

!!! question "Since sing-box 1.13.0"

==Server only==

Client certificate chain line array, in PEM format.

#### client_certificate_path

!!! question "Since sing-box 1.13.0"

==Server only==

!!! note ""

    Will be automatically reloaded if file modified.

List of path to client certificate chain, in PEM format.

#### client_certificate_public_key_sha256

!!! question "Since sing-box 1.13.0"

==Server only==

List of SHA-256 hashes of client certificate public keys, in base64 format.

To generate the SHA-256 hash for a certificate's public key, use the following commands:

```bash
# For a certificate file
openssl x509 -in certificate.pem -pubkey -noout | openssl pkey -pubin -outform der | openssl dgst -sha256 -binary | openssl enc -base64

# For a certificate from a remote server
echo | openssl s_client -servername example.com -connect example.com:443 2>/dev/null | openssl x509 -pubkey -noout | openssl pkey -pubin -outform der | openssl dgst -sha256 -binary | openssl enc -base64
```

#### kernel_tx

!!! question "Since sing-box 1.13.0"

!!! quote ""

    Only supported on Linux 5.1+, use a newer kernel if possible.

!!! quote ""

    Only TLS 1.3 is supported.

!!! warning ""

    kTLS TX may only improve performance when `splice(2)` is available (both ends must be TCP or TLS without additional protocols after handshake); otherwise, it will definitely degrade performance.

Enable kernel TLS transmit support.

#### kernel_rx

!!! question "Since sing-box 1.13.0"

!!! quote ""

    Only supported on Linux 5.1+, use a newer kernel if possible.

!!! quote ""

    Only TLS 1.3 is supported.

!!! failure ""

    kTLS RX will definitely degrade performance even if `splice(2)` is in use, so enabling it is not recommended.

Enable kernel TLS receive support.

## Custom TLS support

!!! info "QUIC support"

    Only ECH is supported in QUIC.

#### utls

==Client only==

!!! failure ""
    
    There is no evidence that GFW detects and blocks servers based on TLS client fingerprinting, and using an imperfect emulation that has not been security reviewed could pose security risks.

uTLS is a fork of "crypto/tls", which provides ClientHello fingerprinting resistance.

Available fingerprint values:

!!! warning "Removed since sing-box 1.10.0"

    Some legacy chrome fingerprints have been removed and will fallback to chrome:

    :material-close: chrome_psk  
    :material-close: chrome_psk_shuffle  
    :material-close: chrome_padding_psk_shuffle  
    :material-close: chrome_pq  
    :material-close: chrome_pq_psk

* chrome
* firefox
* edge
* safari
* 360
* qq
* ios
* android
* random
* randomized

Chrome fingerprint will be used if empty.

### ECH Fields

ECH (Encrypted Client Hello) is a TLS extension that allows a client to encrypt the first part of its ClientHello
message.

The ECH key and configuration can be generated by `sing-box generate ech-keypair`.

#### pq_signature_schemes_enabled

!!! failure "Deprecated in sing-box 1.12.0"

    ECH support has been migrated to use stdlib in sing-box 1.12.0, which does not come with support for PQ signature schemes, so `pq_signature_schemes_enabled` has been deprecated and no longer works.

Enable support for post-quantum peer certificate signature schemes.

#### dynamic_record_sizing_disabled

!!! failure "Deprecated in sing-box 1.12.0"

    `dynamic_record_sizing_disabled` has nothing to do with ECH, was added by mistake, has been deprecated and no longer works.

Disables adaptive sizing of TLS records.

When true, the largest possible TLS record size is always used.  
When false, the size of TLS records may be adjusted in an attempt to improve latency.

#### key

==Server only==

ECH key line array, in PEM format.

#### key_path

==Server only==

!!! note ""

    Will be automatically reloaded if file modified.

The path to ECH key, in PEM format.

#### config

==Client only==

ECH configuration line array, in PEM format.

If empty, load from DNS will be attempted.

#### config_path

==Client only==

The path to ECH configuration, in PEM format.

If empty, load from DNS will be attempted.

#### fragment

!!! question "Since sing-box 1.12.0"

==Client only==

Fragment TLS handshakes to bypass firewalls.

This feature is intended to circumvent simple firewalls based on **plaintext packet matching**,
and should not be used to circumvent real censorship.

Due to poor performance, try `record_fragment` first, and only apply to server names known to be blocked.

On Linux, Apple platforms, (administrator privileges required) Windows,
the wait time can be automatically detected. Otherwise, it will fall back to
waiting for a fixed time specified by `fragment_fallback_delay`.

In addition, if the actual wait time is less than 20ms, it will also fall back to waiting for a fixed time,
because the target is considered to be local or behind a transparent proxy.

#### fragment_fallback_delay

!!! question "Since sing-box 1.12.0"

==Client only==

The fallback value used when TLS segmentation cannot automatically determine the wait time.

`500ms` is used by default.

#### record_fragment

!!! question "Since sing-box 1.12.0"

==Client only==

Fragment TLS handshake into multiple TLS records to bypass firewalls.

### ACME Fields

#### domain

List of domain.

ACME will be disabled if empty.

#### data_directory

The directory to store ACME data.

`$XDG_DATA_HOME/certmagic|$HOME/.local/share/certmagic` will be used if empty.

#### default_server_name

Server name to use when choosing a certificate if the ClientHello's ServerName field is empty.

#### email

The email address to use when creating or selecting an existing ACME server account

#### provider

The ACME CA provider to use.

| Value                   | Provider      |
|-------------------------|---------------|
| `letsencrypt (default)` | Let's Encrypt |
| `zerossl`               | ZeroSSL       |
| `https://...`           | Custom        |

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

External account bindings are "used to associate an ACME account with an existing account in a non-ACME system, such as
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

### Reality Fields

#### handshake

==Server only==

==Required==

Handshake server address and [Dial Fields](/configuration/shared/dial/).

#### private_key

==Server only==

==Required==

Private key, generated by `sing-box generate reality-keypair`.

#### public_key

==Client only==

==Required==

Public key, generated by `sing-box generate reality-keypair`.

#### short_id

==Required==

A hexadecimal string with zero to eight digits.

#### max_time_difference

==Server only==

The maximum time difference between the server and the client.

Check disabled if empty.