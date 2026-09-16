---
icon: material/new-box
---

!!! question "Since sing-box 1.15.0"

### Structure

```json
{
  "type": "tailcat",
  "tag": "tailcat-in",

  "private_key": "",
  "pre_shared_key": "",
  "users": [
    {
      "name": "",
      "public_key": ""
    }
  ],
  "derp_map_url": "",
  "derp_region": 0,
  "derp_servers": [],
  "http_client": {},

  ... // Dial Fields
}
```

### Fields

#### private_key

==Required==

Private key.

Generate with `sing-box generate tailcat-keypair`.

#### pre_shared_key

Pre-shared key.

#### users

Tailcat users.

Clients are not verified if empty.

To use a DERP server with client verification, set `verify_client_inbound` or `verify_client_key` in
[DERP service](/configuration/service/derp/#verify_client_inbound), and clients must use fixed
private keys.

#### users.public_key

==Required==

Client public key.

#### derp_map_url

URL of the [DERP map](https://pkg.go.dev/tailscale.com/tailcfg#DERPMap).

`https://tailcat.dev/derpmap.json` is used by default.

#### derp_region

DERP region ID in the DERP map.

Conflicts with `derp_servers`.

#### derp_servers

Custom DERP servers in [DERPNode](https://pkg.go.dev/tailscale.com/tailcfg#DERPNode) format, with
snake_case field names.

Conflicts with `derp_map_url` and `derp_region`.

Setting Array value to a string `__HOST__` is equivalent to configuring:

```json
{ "host": __HOST__ }
```

#### http_client

HTTP client used to fetch the DERP map.

See [HTTP Client](/configuration/shared/http-client/) for details.

### Dial Fields

!!! note

    Dial Fields in Tailcat inbounds only control how it connects to DERP servers and have nothing to do with actual connections.

See [Dial Fields](/configuration/shared/dial/) for details.
