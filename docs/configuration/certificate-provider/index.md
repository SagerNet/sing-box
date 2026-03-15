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
| `cloudflare-origin-ca` | [Cloudflare Origin CA](./cloudflare-origin-ca) |

The `acme` type migrates deprecated inline `tls.acme` options.
See [ACME](./acme) for fields newly added in sing-box 1.14.0.

#### tag

The tag of the certificate provider.
