---
icon: material/new-box
---

!!! question "Since sing-box 1.12.0"

# DERP

DERP service is a Tailscale DERP server, similar to [derper](https://pkg.go.dev/tailscale.com/cmd/derper).

### Structure

```json
{
  "type": "derp",
  
  ... // Listen Fields

  "tls": {},
  "config_path": "",
  "verify_client_endpoint": [],
  "verify_client_url": [],
  "home": "",
  "mesh_with": [],
  "mesh_psk": "",
  "mesh_psk_file": "",
  "stun": {}
}
```

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.

### Fields

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#inbound).

#### config_path

==Required==

Derper configuration file path.

Example: `derper.key`

#### verify_client_endpoint

Tailscale endpoints tags to verify clients.

#### verify_client_url

URL to verify clients.

Object format:

```json
{
  "url": "https://my-headscale.com/verify",
  
  ... // Dial Fields
}
```

Setting Array value to a string `__URL__` is equivalent to configuring:

```json
{ "url": __URL__ }
```

#### home

What to serve at the root path. It may be left empty (the default, for a default homepage), `blank` for a blank page, or a URL to redirect to

#### mesh_with

Mesh with other DERP servers.

Object format:

```json
{
  "server": "",
  "server_port": "",
  "host": "",
  "tls": {},
  
  ... // Dial Fields
}
```

Object fields:

- `server`: **Required** DERP server address.
- `server_port`: **Required** DERP server port.
- `host`: Custom DERP hostname.
- `tls`: [TLS](/configuration/shared/tls/#outbound)
- `Dial Fields`: [Dial Fields](/configuration/shared/dial/)

#### mesh_psk

Pre-shared key for DERP mesh.

#### mesh_psk_file

Pre-shared key file for DERP mesh.

#### stun

STUN server listen options.

Object format:

```json
{
  "enabled": true,
  
  ... // Listen Fields
}
```

Object fields:

- `enabled`: **Required** Enable STUN server.
- `listen`: **Required** STUN server listen address, default to `::`.
- `listen_port`: **Required** STUN server listen port, default to `3478`.
- `other Listen Fields`: [Listen Fields](/configuration/shared/listen/)

Setting `stun` value to a number `__PORT__` is equivalent to configuring:

```json
{ "enabled": true, "listen_port": __PORT__ }
```
