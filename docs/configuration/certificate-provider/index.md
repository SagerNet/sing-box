---
icon: material/new-box
---

!!! question "Since sing-box 1.14.0"

# Certificate Provider

### Structure

```json
{
  "certificate_providers": [
    {
      "type": "",
      "tag": ""
    }
  ]
}
```

### Fields

| Type   | Format           |
|--------|------------------|
| `acme` | [ACME](./acme)   |
| `tailscale` | [Tailscale](./tailscale) |

#### tag

The tag of the certificate provider.
