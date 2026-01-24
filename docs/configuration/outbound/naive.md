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
  "quic": false,
  "quic_congestion_control": "",
  "tls": {},

  ... // Dial Fields
}
```

!!! warning "Platform Support"

    NaiveProxy outbound is only available on Apple platforms, Android, Windows and certain Linux builds.

    **Official Release Build Variants:**

    | Build Variant | Platforms | Description |
    |---------------|-----------|-------------|
    | (default)     | Linux amd64/arm64 | purego build with `libcronet.so` included |
    | `-glibc`      | Linux 386/amd64/arm/arm64 | CGO build dynamically linked with glibc, requires glibc >= 2.31 |
    | `-musl`       | Linux 386/amd64/arm/arm64 | CGO build statically linked with musl, no system requirements |
    | (default)     | Windows amd64/arm64 | purego build with `libcronet.dll` included |

    **Runtime Requirements:**

    - **Linux purego**: `libcronet.so` must be in the same directory as the sing-box binary or in system library path
    - **Windows**: `libcronet.dll` must be in the same directory as `sing-box.exe` or in a directory listed in `PATH`

    For self-built binaries, see [Build from source](/installation/build-from-source/#with_naive_outbound).

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

#### quic

Use QUIC instead of HTTP/2.

#### quic_congestion_control

QUIC congestion control algorithm.

| Algorithm | Description |
|-----------|-------------|
| `bbr` | BBR |
| `bbr2` | BBRv2 |
| `cubic` | CUBIC |
| `reno` | New Reno |

`bbr` is used by default (the default of QUICHE, used by Chromium which NaiveProxy is based on).

#### tls

==Required==

TLS configuration, see [TLS](/configuration/shared/tls/#outbound).

Only `server_name`, `certificate`, `certificate_path` and `ech` are supported.

Self-signed certificates change traffic behavior significantly, which defeats the purpose of NaiveProxy's design to resist traffic analysis, and should not be used in production.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
