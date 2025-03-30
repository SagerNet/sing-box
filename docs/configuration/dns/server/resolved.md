---
icon: material/new-box
---

!!! question "Since sing-box 1.12.0"

# Resolved

```json
{
  "dns": {
    "servers": [
      {
        "type": "resolved",
        "tag": "",

        "service": "resolved",
        "accept_default_resolvers": false
      }
    ]
  }
}
```


### Fields

#### service

==Required==

The tag of the [Resolved Service](/configuration/service/resolved).

#### accept_default_resolvers

Indicates whether the default DNS resolvers should be accepted for fallback queries in addition to matching domains.

Specifically, default DNS resolvers are DNS servers that have `SetLinkDefaultRoute` or `SetLinkDomains ~.` set.

If not enabled, `NXDOMAIN` will be returned for requests that do not match search or match domains.

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
            "type": "resolved",
            "tag": "resolved",
            "service": "resolved"
          }
        ],
        "rules": [
          {
            "ip_accept_any": true,
            "server": "resolved"
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
            "type": "resolved",
            "service": "resolved",
            "accept_default_resolvers": true
          }
        ]
      }
    }
    ```