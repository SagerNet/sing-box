### Inbound Structure

```json
{
  "enabled": true,
  "server_name": "",
  "alpn": [],
  "min_version": "",
  "max_version": "",
  "cipher_suites": [],
  "certificate": "",
  "certificate_path": "",
  "key": "",
  "key_path": ""
}
```

### Outbound Structure

```json
{
  "enabled": true,
  "server_name": "",
  "insecure": false,
  "alpn": [],
  "min_version": "",
  "max_version": "",
  "cipher_suites": [],
  "disable_system_root": false,
  "certificate": "",
  "certificate_path": ""
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

### Fields

#### enabled

Enable TLS.

#### server_name

Used to verify the hostname on the returned certificates unless insecure is given.

It is also included in the client's handshake to support virtual hosting unless it is an IP address.

See [Server Name Indication](https://en.wikipedia.org/wiki/Server_Name_Indication).

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
client, and TLS 1.0 when acting as a server. TLS 1.0 is the minimum
supported by this package, both as a client and as a server.

The client-side default can temporarily be reverted to TLS 1.0 by
including the value "x509sha1=1" in the GODEBUG environment variable.
Note that this option will be removed in Go 1.19 (but it will still be
possible to set this field to VersionTLS10 explicitly).

#### max_version

The maximum TLS version that is acceptable.

By default, the maximum version supported by this package is used,
which is currently TLS 1.3.

#### cipher_suites

The elliptic curves that will be used in an ECDHE handshake, in preference order.

If empty, the default will be used. The client will use the first preference as the type for its key share in TLS 1.3.
This may change in the future.

#### certificate

The server certificate, in PEM format.

#### certificate_path

The path to the server certificate, in PEM format.

#### key

==Server only==

The server private key, in PEM format.

#### key_path

==Server only==

The path to the server private key, in PEM format.

### Reload

For server configuration, certificate and key will be automatically reloaded if modified.