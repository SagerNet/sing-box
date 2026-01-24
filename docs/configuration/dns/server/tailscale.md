---
icon: material/new-box
---

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
        "accept_default_resolvers": false
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

### Examples

=== "MagicDNS only"

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
