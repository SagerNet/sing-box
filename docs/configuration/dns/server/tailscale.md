---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.14.0"

    :material-plus: [accept_search_domain](#accept_search_domain)

!!! question "Since sing-box 1.12.0"

# Tailscale

### Structure

```json
{
  "dns": {
    "servers": [
      {
        "type": "tailscale",
        "tag": "",

        "endpoint": "ts-ep",
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

The tag of the [Tailscale Endpoint](/configuration/endpoint/tailscale).

#### accept_default_resolvers

Indicates whether default DNS resolvers should be accepted for fallback queries in addition to MagicDNS。

if not enabled, `NXDOMAIN` will be returned for non-Tailscale domain queries.

#### accept_search_domain

!!! question "Since sing-box 1.14.0"

When enabled, single-label queries (e.g. `my-device`) are retried against each Tailscale search domain until one resolves.

Default resolvers are not consulted for single-label queries regardless of `accept_default_resolvers`.

### Examples

=== "MagicDNS only"

    === ":material-card-multiple: sing-box 1.14.0"

        ```json
        {
          "dns": {
            "servers": [
              {
                "type": "local",
                "tag": "local"
              },
              {
                "type": "tailscale",
                "tag": "ts",
                "endpoint": "ts-ep"
              }
            ],
            "rules": [
              {
                "action": "evaluate",
                "server": "ts"
              },
              {
                "match_response": true,
                "ip_accept_any": true,
                "action": "respond"
              }
            ]
          }
        }
        ```

    === ":material-card-remove: sing-box < 1.14.0"

        ```json
        {
          "dns": {
            "servers": [
              {
                "type": "local",
                "tag": "local"
              },
              {
                "type": "tailscale",
                "tag": "ts",
                "endpoint": "ts-ep"
              }
            ],
            "rules": [
              {
                "ip_accept_any": true,
                "server": "ts"
              }
            ]
          }
        }
        ```

=== "Use as global DNS"

    ```json
    {
      "dns": {
        "servers": [
          {
            "type": "tailscale",
            "endpoint": "ts-ep",
            "accept_default_resolvers": true
          }
        ]
      }
    }
    ```
