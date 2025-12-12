---
icon: material/file-code
---

# Build from source

## :material-graph: Requirements

### sing-box 1.11

* Go 1.23.1 - ~

### sing-box 1.10

* Go 1.20.0 - ~

### sing-box 1.9

* Go 1.18.5 - 1.22.x
* Go 1.20.0 - 1.22.x with tag `with_quic`, or `with_utls` enabled

## :material-fast-forward: Simple Build

```bash
make
```

Or build and install binary to `$GOBIN`:

```bash
make install
```

## :material-cog: Custom Build

```bash
TAGS="tag_a tag_b" make
```

or

```bash
go build -tags "tag_a tag_b" ./cmd/sing-box
```

## :material-folder-settings: Build Tags

| Build Tag                          | Enabled by default   | Description                                                                                                                                                                                                                                                                                                                    |
|------------------------------------|----------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `with_quic`                        | :material-check:     | Build with QUIC support, see [QUIC and HTTP3 DNS transports](/configuration/dns/server/), [Naive inbound](/configuration/inbound/naive/), [Hysteria Inbound](/configuration/inbound/hysteria/), [Hysteria Outbound](/configuration/outbound/hysteria/) and [V2Ray Transport#QUIC](/configuration/shared/v2ray-transport#quic). |
| `with_grpc`                        | :material-close:️    | Build with standard gRPC support, see [V2Ray Transport#gRPC](/configuration/shared/v2ray-transport#grpc).                                                                                                                                                                                                                      |
| `with_dhcp`                        | :material-check:     | Build with DHCP support, see [DHCP DNS transport](/configuration/dns/server/).                                                                                                                                                                                                                                                 |
| `with_wireguard`                   | :material-check:     | Build with WireGuard support, see [WireGuard outbound](/configuration/outbound/wireguard/).                                                                                                                                                                                                                                    |
| `with_utls`                        | :material-check:     | Build with [uTLS](https://github.com/refraction-networking/utls) support for TLS outbound, see [TLS](/configuration/shared/tls#utls).                                                                                                                                                                                          |
| `with_acme`                        | :material-check:     | Build with ACME TLS certificate issuer support, see [TLS](/configuration/shared/tls/).                                                                                                                                                                                                                                         |
| `with_clash_api`                   | :material-check:     | Build with Clash API support, see [Experimental](/configuration/experimental#clash-api-fields).                                                                                                                                                                                                                                |
| `with_v2ray_api`                   | :material-close:️    | Build with V2Ray API support, see [Experimental](/configuration/experimental#v2ray-api-fields).                                                                                                                                                                                                                                |
| `with_gvisor`                      | :material-check:     | Build with gVisor support, see [Tun inbound](/configuration/inbound/tun#stack) and [WireGuard outbound](/configuration/outbound/wireguard#system_interface).                                                                                                                                                                   |
| `with_embedded_tor` (CGO required) | :material-close:️    | Build with embedded Tor support, see [Tor outbound](/configuration/outbound/tor/).                                                                                                                                                                                                                                             |
| `with_tailscale`                   | :material-check:     | Build with Tailscale support, see [Tailscale endpoint](/configuration/endpoint/tailscale)                                                                                                                                                                                                                                      |
| `with_naive_outbound`              | :material-close:️    | Build with NaiveProxy outbound support, see [NaiveProxy outbound](/configuration/outbound/naive/).                                                                                                                                                                                                                             |

It is not recommended to change the default build tag list unless you really know what you are adding.

## :material-layers: with_naive_outbound

NaiveProxy outbound requires special build configurations depending on your target platform.

### Supported Platforms

| Platform        | Architectures      | Mode   | Requirements                                                                                                                         |
|-----------------|--------------------|--------|--------------------------------------------------------------------------------------------------------------------------------------|
| Windows         | *                  | purego | None                                                                                                                                 |
| Linux           | amd64, arm64       | purego | Download libcronet from [cronet-go releases](https://github.com/sagernet/cronet-go/releases) to system library path or sing-box binary directory |
| Linux           | 386, amd64, arm, arm64 | CGO    | Chromium toolchain (see [cronet-go](https://github.com/sagernet/cronet-go))                                                          |
| Apple platforms | *                  | CGO    | Xcode                                                                                                                                |
| Android         | *                  | CGO    | Android NDK                                                                                                                          |

### Windows

Use `with_purego` tag.

### Linux (purego, amd64/arm64 only)

Download `libcronet.so` from [cronet-go releases](https://github.com/sagernet/cronet-go/releases) and install to system library path or the same directory as sing-box binary, then use `with_purego` tag.

### Linux (CGO)

See [cronet-go](https://github.com/sagernet/cronet-go#linux-build-instructions).

### Apple platforms / Android

See [cronet-go](https://github.com/sagernet/cronet-go).
