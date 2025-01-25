---
icon: material/new-box
---

!!! question "Since sing-box 1.12.0"

# DHCP

### Structure

```json
{
  "dns": {
    "servers": [
      {
        "type": "dhcp",
        "tag": "",

        "interface": "",
        
        // Dial Fields
      }
    ]
  }
}
```

### Fields

#### interface

Interface name to listen on. 

Tge default interface will be used by default.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details. 
