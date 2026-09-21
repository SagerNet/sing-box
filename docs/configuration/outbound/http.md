`http` outbound is a HTTP CONNECT proxy client.

### Structure

```json
{
  "type": "http",
  "tag": "http-out",
  
  "server": "127.0.0.1",
  "server_port": 1080,
  "username": "sekai",
  "password": "admin",
  "path": "",
  "headers": {},
  "version": 0,
  "disable_version_fallback": false,
  "tls": {},

  ... // HTTP2 Fields / QUIC Fields
  ... // Dial Fields
}
```

### Fields

#### server

==Required==

The server address.

#### server_port

==Required==

The server port.

#### username

Basic authorization username.

#### password

Basic authorization password.

#### path

Path of HTTP request.

#### headers

Extra headers of HTTP request.

#### version

!!! question "Since sing-box 1.15.0"

HTTP version.

Available values: `1`, `2`, `3`.

`2` is used by default, or `1` if `path` or the `Host` header is set.

`path` and the `Host` header are only available for `1`.

When `3`, [HTTP2 Fields](#http2-fields) are replaced by [QUIC Fields](#quic-fields).

#### disable_version_fallback

!!! question "Since sing-box 1.15.0"

Disable automatic fallback to lower HTTP version.

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

### HTTP2 Fields

!!! question "Since sing-box 1.15.0"

When `version` is `2` (default).

See [HTTP2 Fields](/configuration/shared/http2/) for details.

### QUIC Fields

!!! question "Since sing-box 1.15.0"

When `version` is `3`.

See [QUIC Fields](/configuration/shared/quic/) for details.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
