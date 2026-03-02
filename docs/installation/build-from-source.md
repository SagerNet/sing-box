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
| `with_tailscale`                   | :material-check:     | Build with Tailscale support, see [Tailscale endpoint](/configuration/endpoint/tailscale).                                                                                                                                                                                                                                     |
| `with_ccm`                         | :material-check:     | Build with Claude Code Multiplexer service support.                                                                                                                                                                                                                                                                            |
| `with_ocm`                         | :material-check:     | Build with OpenAI Codex Multiplexer service support.                                                                                                                                                                                                                                                                           |
| `with_naive_outbound`              | :material-check:     | Build with NaiveProxy outbound support, see [NaiveProxy outbound](/configuration/outbound/naive/).                                                                                                                                                                                                                             |
| `badlinkname`                      | :material-check:     | Enable `go:linkname` access to internal standard library functions. Required because the Go standard library does not expose many low-level APIs needed by this project, and reimplementing them externally is impractical. Used for kTLS (kernel TLS offload) and raw TLS record manipulation.                                 |
| `tfogo_checklinkname0`             | :material-check:     | Companion to `badlinkname`. Go 1.23+ enforces `go:linkname` restrictions via the linker; this tag signals the build uses `-checklinkname=0` to bypass that enforcement.                                                                                                                                                       |

It is not recommended to change the default build tag list unless you really know what you are adding.

## :material-wrench: Linker Flags

The following `-ldflags` are used in official builds:

| Flag                                                        | Description                                                                                                                                                                                                             |
|-------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `-X 'internal/godebug.defaultGODEBUG=multipathtcp=0'`      | Go 1.24 enabled Multipath TCP for listeners by default (`multipathtcp=2`). This may cause errors on low-level sockets, and sing-box has its own MPTCP control (`tcp_multi_path` option). This flag disables the Go default. |
| `-checklinkname=0`                                          | Go 1.23+ linker rejects unauthorized `go:linkname` usage. This flag disables the check, required together with the `badlinkname` build tag.                                                                            |

## :material-package-variant: For Downstream Packagers

The default build tag lists and linker flags are available as files in the repository for downstream packagers to reference directly:

| File | Description |
|------|-------------|
| `release/DEFAULT_BUILD_TAGS` | Default for Linux (common architectures), Darwin, and Android. |
| `release/DEFAULT_BUILD_TAGS_WINDOWS` | Default for Windows (includes `with_purego`). |
| `release/DEFAULT_BUILD_TAGS_OTHERS` | Default for other platforms (no `with_naive_outbound`). |
| `release/LDFLAGS` | Required linker flags (see above). |

## :material-layers: with_naive_outbound

NaiveProxy outbound requires special build configurations depending on your target platform.

### Supported Platforms

| Platform        | Architectures          | Mode   | Requirements                                      |
|-----------------|------------------------|--------|---------------------------------------------------|
| Linux           | amd64, arm64           | purego | None (library included in official releases)      |
| Linux           | 386, amd64, arm, arm64 | CGO    | Chromium toolchain, glibc >= 2.31 at runtime      |
| Linux (musl)    | 386, amd64, arm, arm64 | CGO    | Chromium toolchain                                |
| Windows         | amd64, arm64           | purego | None (library included in official releases)      |
| Apple platforms | *                      | CGO    | Xcode                                             |
| Android         | *                      | CGO    | Android NDK                                       |

### Windows

Use `with_purego` tag.

For official releases, `libcronet.dll` is included in the archive. For self-built binaries, download from [cronet-go releases](https://github.com/sagernet/cronet-go/releases) and place in the same directory as `sing-box.exe` or in a directory listed in `PATH`.

### Linux (purego, amd64/arm64 only)

Use `with_purego` tag.

For official releases, `libcronet.so` is included in the archive. For self-built binaries, download from [cronet-go releases](https://github.com/sagernet/cronet-go/releases) and place in the same directory as sing-box binary or in system library path.

### Linux (CGO)

See [cronet-go](https://github.com/sagernet/cronet-go#linux-build-instructions).

- **glibc build**: Requires glibc >= 2.31 at runtime
- **musl build**: Use `with_musl` tag, statically linked, no runtime requirements

### Apple platforms / Android

See [cronet-go](https://github.com/sagernet/cronet-go).
