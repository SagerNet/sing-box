---
icon: material/new-box
---

!!! question "Since sing-box 1.13.0"

### Structure

```json
{
  "type": "anytls2",
  "tag": "anytls2-in",

  ... // Listen Fields

  "users": [
    {
      "name": "sekai",
      "password": "8JCsPssfgS8tiRwiMlhARg=="
    }
  ],
  "padding_scheme": [],
  "tls": {},
  "transport": {}
}
```

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.

### Fields

#### users

==Required==

AnyTLS users.

#### padding_scheme

AnyTLS padding scheme line array.

Default padding scheme:

```json
[
  "stop=8",
  "0=30-30",
  "1=100-400",
  "2=400-500,c,500-1000,c,500-1000,c,500-1000,c,500-1000",
  "3=9-9,500-1000",
  "4=500-1000",
  "5=500-1000",
  "6=500-1000",
  "7=500-1000"
]
```

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#inbound).

#### transport

V2Ray transport configuration, see [V2Ray Transport](/configuration/shared/v2ray-transport/).

This adds HTTP-path-capable transport support to AnyTLS so it can sit behind an nginx-like reverse proxy and share the same public port with the main website. The reverse proxy continues to own normal website traffic with its own performance characteristics; sing-box only handles the AnyTLS2 path.

#### multiplex

Not available.

AnyTLS already provides built-in session multiplexing, so multiple proxied connections share one outer connection natively. A separate mux layer is not needed or supported.

### Example (HTTP Path)

Behind an nginx-like reverse proxy that already listens on `443`, bind AnyTLS2 to an internal port such as `8443`.

```json
{
  "type": "anytls2",
  "tag": "anytls2-in",
  "listen": "::",
  "listen_port": 8443,
  "users": [
    {
      "name": "example",
      "password": "password"
    }
  ],
  "transport": {
    "type": "ws",
    "path": "/anytls"
  }
}
```

> **Note:** When deployed behind nginx (or any reverse proxy), AnyTLS2 does not need to enable TLS. The proxy terminates TLS and forwards plain WebSocket traffic to the backend. If you run AnyTLS2 directly on a public port, you must enable TLS in its config.

#### Minimal nginx reverse proxy config

This example shows nginx terminating TLS and forwarding WebSocket traffic on `/anytls` to the AnyTLS2 backend. AnyTLS2 itself does not handle TLS.

```nginx
server {
    listen 443 ssl;
    server_name example.org;
    ssl_certificate     /path/to/certificate.pem;
    ssl_certificate_key /path/to/key.pem;

    location /anytls {
        proxy_pass http://127.0.0.1:8443;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        # Optionally tune timeouts for long-lived connections
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }
}
```

### Example (HTTP/2 via nginx h2c)

nginx terminates TLS and proxies to AnyTLS2 over h2c (cleartext HTTP/2). AnyTLS2 does not handle TLS.
HTTP/2 stream multiplexing means each AnyTLS2 session maps to an independent H2 stream,
avoiding TCP head-of-line blocking on the nginx→backend leg.

```json
{
  "type": "anytls2",
  "tag": "anytls2-in",
  "listen": "::",
  "listen_port": 8443,
  "users": [
    {
      "name": "example",
      "password": "password"
    }
  ],
  "transport": {
    "type": "http",
    "path": "/anytls-http2"
  }
}
```

Matching nginx config (`proxy_http_version 2` enables h2c upstream):

```nginx
server {
    listen 443 ssl;
    server_name example.org;
    ssl_certificate     /path/to/certificate.pem;
    ssl_certificate_key /path/to/key.pem;

    location /anytls-http2 {
        proxy_pass http://127.0.0.1:8443;
        proxy_http_version 2;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }
}
```

### Example (QUIC, direct)

AnyTLS2 with QUIC transport runs directly on a public UDP port — no reverse proxy is needed or possible.
QUIC eliminates TCP head-of-line blocking entirely: each AnyTLS2 session maps to a separate QUIC stream
with independent delivery, so a stalled stream never blocks others.
Requires sing-box built with the `with_quic` tag.

`quic` transport does not have a path field.

```json
{
  "type": "anytls2",
  "tag": "anytls2-in",
  "listen": "::",
  "listen_port": 443,
  "users": [
    {
      "name": "example",
      "password": "password"
    }
  ],
  "tls": {
    "enabled": true,
    "server_name": "example.org",
    "certificate_path": "/path/to/certificate.pem",
    "key_path": "/path/to/key.pem"
  },
  "transport": {
    "type": "quic"
  }
}
```

> **Note:** You can use HTTP/2 or QUIC for the main website, but the minimal config above is sufficient for WebSocket transport. Adjust paths and certificate locations as needed.