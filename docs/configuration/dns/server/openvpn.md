---
icon: material/new-box
---

!!! question "Since sing-box 1.14.0"

# OpenVPN

### Structure

```json
{
  "dns": {
    "servers": [
      {
        "type": "openvpn",
        "tag": "",

        "endpoint": "ovpn-client",
        "accept_default_resolvers": false,
        "accept_search_domain": false
      }
    ]
  }
}
```

### Fields

#### endpoint

==Required==

The tag of the [OpenVPN Client Endpoint](/configuration/endpoint/openvpn-client).

DNS queries are sent through the endpoint to resolvers pushed by the OpenVPN server.

Both modern `dns` options and legacy `dhcp-option DNS`/`DNS6` and `DOMAIN-ROUTE` options are supported. `dnssec yes` is not supported.

#### accept_default_resolvers

Use pushed resolvers for queries that do not match a pushed `resolve-domains`, `DOMAIN-ROUTE`, or search-domain suffix.

When disabled, unmatched queries return `NXDOMAIN`.

#### accept_search_domain

When enabled and pushed search domains are available, single-label queries (for example, `intranet`) are retried with each search domain until one resolves.

If no search domain is available, the original single-label query follows normal default-resolver behavior.

### Example

```json
{
  "dns": {
    "servers": [
      {
        "type": "local",
        "tag": "local"
      },
      {
        "type": "openvpn",
        "tag": "ovpn-dns",
        "endpoint": "ovpn-client",
        "accept_default_resolvers": true,
        "accept_search_domain": true
      }
    ],
    "rules": [
      {
        "preferred_by": "ovpn-dns",
        "action": "route",
        "server": "ovpn-dns"
      }
    ],
    "final": "local"
  }
}
```
