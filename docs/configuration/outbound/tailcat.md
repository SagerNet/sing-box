---
icon: material/new-box
---

!!! question "Since sing-box 1.15.0"

### Structure

```json
{
  "type": "tailcat",
  "tag": "tailcat-out",

  "private_key": "",
  "server_public_key": "",
  "server_disco_key": "",
  "pre_shared_key": "",
  "derp_map_url": "",
  "derp_region": 0,
  "derp_servers": [],
  "http_client": {},

  ... // Dial Fields
}
```

### Fields

#### private_key

Private key.

A random key is used by default.

Required when the server verifies clients with `users`, or the DERP server verifies clients with
`verify_client_inbound` or `verify_client_key`.

#### server_public_key

==Required==

Server public key.

#### server_disco_key

==Required==

Server disco public key.

#### pre_shared_key

Pre-shared key.

#### derp_map_url

URL of the [DERP map](https://pkg.go.dev/tailscale.com/tailcfg#DERPMap).

`https://tailcat.dev/derpmap.json` is used by default.

#### derp_region

DERP region ID in the DERP map.

Conflicts with `derp_servers`.

#### derp_servers

Custom DERP servers, see [derp_servers](/configuration/inbound/tailcat/#derp_servers) in Tailcat inbound.

Conflicts with `derp_map_url` and `derp_region`.

#### http_client

HTTP client used to fetch the DERP map.

See [HTTP Client](/configuration/shared/http-client/) for details.

### Dial Fields

!!! note

    Dial Fields in Tailcat outbounds only control how it connects to DERP servers and have nothing to do with actual connections.

See [Dial Fields](/configuration/shared/dial/) for details.
