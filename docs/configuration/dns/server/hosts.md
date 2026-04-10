---
icon: material/new-box
---

!!! question "Since sing-box 1.12.0"

# Hosts

### Structure

```json
{
  "dns": {
    "servers": [
      {
        "type": "hosts",
        "tag": "",

        "path": [],
        "predefined": {}
      }
    ]
  }
}
```

!!! note ""

    You can ignore the JSON Array [] tag when the content is only one item

### Fields

#### path

List of paths to hosts files.

`/etc/hosts` is used by default.

`C:\Windows\System32\Drivers\etc\hosts` is used by default on Windows.

Example:

```json
{
  // "path": "/etc/hosts"
  
  "path": [
    "/etc/hosts",
    "$HOME/.hosts"
  ]
}
```

#### predefined

Predefined hosts.

Example:

```json
{
  "predefined": {
    "www.google.com": "127.0.0.1",
    "localhost": [
      "127.0.0.1",
      "::1"
    ]
  }
}
```

### Examples

=== "Use hosts if available"

    === ":material-card-multiple: sing-box 1.14.0"

        ```json
        {
          "dns": {
            "servers": [
              {
                ...
              },
              {
                "type": "hosts",
                "tag": "hosts"
              }
            ],
            "rules": [
              {
                "action": "evaluate",
                "server": "hosts"
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
                ...
              },
              {
                "type": "hosts",
                "tag": "hosts"
              }
            ],
            "rules": [
              {
                "ip_accept_any": true,
                "server": "hosts"
              }
            ]
          }
        }
        ```