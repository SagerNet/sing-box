---
icon: material/new-box
---

!!! question "Since sing-box 1.13.0"

### Structure

```json
{
  "type": "naive",
  "tag": "naive-out",

  "server": "127.0.0.1",
  "server_port": 443,
  "username": "sekai",
  "password": "password",
  "insecure_concurrency": 0,
  "extra_headers": {},
  "udp_over_tcp": false | {},
  "tls": {},

  ... // Dial Fields
}
```

!!! warning ""

    NaiveProxy outbound is only available on Apple platforms, Android, Windows and some Linux architectures, see [Build from source](/installation/build-from-source/#with_naive_outbound).

### Fields

#### server

==Required==

The server address.

#### server_port

==Required==

The server port.

#### username

Authentication username.

#### password

Authentication password.

#### insecure_concurrency

Number of concurrent tunnel connections. Multiple connections make the tunneling easier to detect through traffic analysis, which defeats the purpose of NaiveProxy's design to resist traffic analysis.

#### extra_headers

Extra headers to send in HTTP requests.

#### udp_over_tcp

UDP over TCP protocol settings.

See [UDP Over TCP](/configuration/shared/udp-over-tcp/) for details.

#### tls

==Required==

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

Only `server_name`, `certificate`, `certificate_path` and `certificate_public_key_sha256` are supported.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
