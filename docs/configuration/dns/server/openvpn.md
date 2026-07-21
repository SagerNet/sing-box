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

DNS queries are sent through the endpoint to resolvers pushed by the OpenVPN server. Modern OpenVPN `dns server` options support plain DNS, DNS over TLS, DNS over HTTPS, custom ports, SNI, and `resolve-domains`. Only the server group with the lowest priority number is active. Legacy `dhcp-option DNS`/`DNS6` and `DOMAIN-ROUTE` are used when no modern server group is present.

A modern server group overrides legacy DHCP DNS resolver and domain options. A standalone modern `dns search-domains` option does not remove legacy resolvers. Required DNSSEC validation (`dnssec yes`) is rejected because this transport does not provide DNSSEC validation.

Pushed DNS settings are not installed into the operating system.

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
