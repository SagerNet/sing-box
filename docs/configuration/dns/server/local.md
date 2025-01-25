---
icon: material/new-box
---

!!! question "Since sing-box 1.12.0"

# Local

### Structure

```json
{
  "dns": {
    "servers": [
      {
        "type": "local",
        "tag": "",

        // Dial Fields
      }
    ]
  }
}
```

!!! info "Difference from legacy local server"
    
    * The old legacy local server only handles IP requests; the new one handles all types of requests and supports concurrent for IP requests.
    * The old local server uses default outbound by default unless detour is specified; the new one uses dialer just like outbound, which is equivalent to using an empty direct outbound by default.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
