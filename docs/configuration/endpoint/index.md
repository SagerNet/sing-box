---
icon: material/new-box
---

!!! question "Since sing-box 1.11.0"

# Endpoint

Endpoint is protocols that has both inbound and outbound behavior.

### Structure

```json
{
  "endpoints": [
    {
      "type": "",
      "tag": ""
    }
  ]
}
```

### Fields

| Type        | Format                    |
|-------------|---------------------------|
| `wireguard` | [WireGuard](./wireguard/) |

#### tag

The tag of the endpoint.
