---
icon: material/new-box
---

!!! question "Since sing-box 1.13.0"

### Structure

```json
{
  "type": "anytls2",
  "tag": "anytls2-out",

  "server": "127.0.0.1",
  "server_port": 1080,
  "password": "8JCsPssfgS8tiRwiMlhARg==",
  "idle_session_check_interval": "30s",
  "idle_session_timeout": "30s",
  "min_idle_session": 5,
  "tls": {},
  "transport": {},

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

#### password

==Required==

The AnyTLS password.

#### idle_session_check_interval

Interval checking for idle sessions. Default: 30s.

#### idle_session_timeout

In the check, close sessions that have been idle for longer than this. Default: 30s.

#### min_idle_session

In the check, at least the first `n` idle sessions are kept open. Default value: `n`=0

#### tls

==Required==

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

#### transport

V2Ray transport configuration, see [V2Ray Transport](/configuration/shared/v2ray-transport/).

Use this when AnyTLS needs HTTP path, WebSocket, gRPC, or HTTPUpgrade support to live behind an nginx-like reverse proxy and share the same public port with the main website.

#### multiplex

Not available.

AnyTLS already provides built-in session multiplexing, so multiple proxied connections share one outer connection natively. A separate mux layer is not needed or supported.

### Example (HTTP Path)

```json
{
  "type": "anytls2",
  "tag": "anytls2-out",
  "server": "example.org",
  "server_port": 443,
  "password": "password",
  "tls": {
    "enabled": true,
    "server_name": "example.org"
  },
  "transport": {
    "type": "ws",
    "path": "/anytls"
  }
}
```

### Example (HTTP/2 via nginx h2c)

Connects to nginx (port 443) over TLS+HTTP/2. nginx proxies to the AnyTLS2 backend via h2c.
See the matching inbound example for the nginx configuration.

```json
{
  "type": "anytls2",
  "tag": "anytls2-out",
  "server": "example.org",
  "server_port": 443,
  "password": "password",
  "tls": {
    "enabled": true,
    "server_name": "example.org"
  },
  "transport": {
    "type": "http",
    "path": "/anytls-http2"
  }
}
```

### Example (QUIC, direct)

Connects directly to the AnyTLS2 server over QUIC. No reverse proxy is involved.
QUIC provides per-stream delivery without TCP head-of-line blocking.
Requires sing-box built with the `with_quic` tag.

`quic` transport does not have a path field.

```json
{
  "type": "anytls2",
  "tag": "anytls2-out",
  "server": "example.org",
  "server_port": 443,
  "password": "password",
  "tls": {
    "enabled": true,
    "server_name": "example.org"
  },
  "transport": {
    "type": "quic"
  }
}
```

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.