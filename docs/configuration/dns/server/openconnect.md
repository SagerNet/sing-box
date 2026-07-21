---
icon: material/new-box
---

!!! question "Since sing-box 1.14.0"

# OpenConnect

### Structure

```json
{
  "dns": {
    "servers": [
      {
        "type": "openconnect",
        "tag": "",

        "endpoint": "oc-client",
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

The tag of the [OpenConnect Endpoint](/configuration/endpoint/openconnect).

DNS queries are sent to the resolvers pushed by the VPN server through the OpenConnect endpoint. Pushed split-DNS rules use their dedicated resolvers, while pushed split-DNS and search-domain suffixes use the general pushed resolvers. The most specific matching suffix takes precedence.

Pushed DNS settings are not installed into the operating system.

#### accept_default_resolvers

Accept the general resolvers pushed by the VPN server for unmatched queries.

When enabled, the general resolvers are used as the default only if the server requests all DNS through the tunnel, or if it does not provide split-DNS rules or suffixes. Otherwise, unmatched queries return `NXDOMAIN`.

#### accept_search_domain

When enabled and pushed search domains are available, single-label queries (for example, `intranet`) are retried with each search domain until one resolves.

If every search-domain expansion returns `NXDOMAIN`, the original unqualified name follows normal default-resolver behavior.

### Examples

=== "Split DNS only"

    ```json
    {
      "dns": {
        "servers": [
          {
            "type": "local",
            "tag": "local"
          },
          {
            "type": "openconnect",
            "tag": "oc",
            "endpoint": "oc-client"
          }
        ],
        "rules": [
          {
            "preferred_by": "oc",
            "action": "route",
            "server": "oc"
          }
        ],
        "final": "local"
      }
    }
    ```

=== "Accept pushed default resolvers"

    ```json
    {
      "dns": {
        "servers": [
          {
            "type": "openconnect",
            "endpoint": "oc-client",
            "accept_default_resolvers": true,
            "accept_search_domain": true
          }
        ]
      }
    }
    ```
