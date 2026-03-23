---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.14.0"

    :material-plus: [ttl](#ttl)  
    :material-plus: [propagation_delay](#propagation_delay)  
    :material-plus: [propagation_timeout](#propagation_timeout)  
    :material-plus: [resolvers](#resolvers)  
    :material-plus: [override_domain](#override_domain)

!!! quote "Changes in sing-box 1.13.0"

    :material-plus: [alidns.security_token](#security_token)  
    :material-plus: [cloudflare.zone_token](#zone_token)  
    :material-plus: [acmedns](#acmedns)

### Structure

```json
{
  "ttl": "",
  "propagation_delay": "",
  "propagation_timeout": "",
  "resolvers": [],
  "override_domain": "",
  "provider": "",

  ... // Provider Fields
}
```

### Fields

#### ttl

!!! question "Since sing-box 1.14.0"

The TTL of the temporary TXT record used for the DNS challenge.

#### propagation_delay

!!! question "Since sing-box 1.14.0"

How long to wait after creating the challenge record before starting propagation checks.

#### propagation_timeout

!!! question "Since sing-box 1.14.0"

The maximum time to wait for the challenge record to propagate.

Set to `-1` to disable propagation checks.

#### resolvers

!!! question "Since sing-box 1.14.0"

Preferred DNS resolvers to use for DNS propagation checks.

#### override_domain

!!! question "Since sing-box 1.14.0"

Override the domain name used for the DNS challenge record.

Useful when `_acme-challenge` is delegated to a different zone.

#### provider

The DNS provider. See below for provider-specific fields.

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
