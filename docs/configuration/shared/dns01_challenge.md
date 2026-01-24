---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.13.0"

    :material-plus: [alidns.security_token](#security_token)  
    :material-plus: [cloudflare.zone_token](#zone_token)  
    :material-plus: [acmedns](#acmedns)

### Structure

```json
{
  "provider": "",

  ... // Provider Fields
}
```

### Provider Fields

#### Alibaba Cloud DNS

```json
{
  "provider": "alidns",
  "access_key_id": "",
  "access_key_secret": "",
  "region_id": "",
  "security_token": ""
}
```

##### security_token

!!! question "Since sing-box 1.13.0"

The Security Token for STS temporary credentials.

#### Cloudflare

```json
{
  "provider": "cloudflare",
  "api_token": "",
  "zone_token": ""
}
```

##### zone_token

!!! question "Since sing-box 1.13.0"

Optional API token with `Zone:Read` permission.

When provided, allows `api_token` to be scoped to a single zone.

#### ACME-DNS

!!! question "Since sing-box 1.13.0"

```json
{
  "provider": "acmedns",
  "username": "",
  "password": "",
  "subdomain": "",
  "server_url": ""
}
```

See [ACME-DNS](https://github.com/joohoi/acme-dns) for details.
