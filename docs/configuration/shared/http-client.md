---
icon: material/new-box
---

!!! question "Since sing-box 1.14.0"

### Structure

A string or an object.

When string, the tag of a shared [HTTP Client](/configuration/shared/http-client/) defined in top-level `http_clients`.

When object:

```json
{
  "engine": "",
  "version": 0,
  "disable_version_fallback": false,
  "headers": {},

  ... // HTTP2 Fields

  "tls": {},

  ... // Dial Fields
}
```

### Fields

#### engine

HTTP engine to use.

Values:

* `go` (default)
* `apple`

`apple` uses NSURLSession, only available on Apple platforms.

!!! warning ""

    Experimental only: due to the high memory overhead of both CGO and Network.framework,
    do not use in hot paths on iOS and tvOS.

Supported fields:

* `headers`
* `tls.server_name` (must match request host)
* `tls.insecure`
* `tls.min_version` / `tls.max_version`
* `tls.certificate` / `tls.certificate_path`
* `tls.certificate_public_key_sha256`
* Dial Fields

Unsupported fields:

* `version`
* `disable_version_fallback`
* HTTP2 Fields
* QUIC Fields
* `tls.engine`
* `tls.alpn`
* `tls.disable_sni`
* `tls.cipher_suites`
* `tls.curve_preferences`
* `tls.client_certificate` / `tls.client_certificate_path` / `tls.client_key` / `tls.client_key_path`
* `tls.fragment` / `tls.record_fragment`
* `tls.kernel_tx` / `tls.kernel_rx`
* `tls.ech`
* `tls.utls`
* `tls.reality`

#### version

HTTP version.

Available values: `1`, `2`, `3`.

`2` is used by default.

When `3`, [HTTP2 Fields](#http2-fields) are replaced by [QUIC Fields](#quic-fields).

#### disable_version_fallback

Disable automatic fallback to lower HTTP version.

#### headers

Custom HTTP headers.

`Host` header is used as request host.

### HTTP2 Fields

When `version` is `2` (default).

See [HTTP2 Fields](/configuration/shared/http2/) for details.

### QUIC Fields

When `version` is `3`.

See [QUIC Fields](/configuration/shared/quic/) for details.

### TLS Fields

See [TLS](/configuration/shared/tls/#outbound) for details.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
